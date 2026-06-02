package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicClient calls the Anthropic Messages API at /v1/messages.
type AnthropicClient struct {
	BaseURL          string
	APIKey           string
	DefaultModel     string
	AnthropicVersion string
	HTTPClient       *http.Client
}

// anthropicMessage uses content blocks rather than a plain string so we can
// embed tool_use / tool_result entries in the conversation history.
type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

// anthropicBlock is a discriminated union covering the block variants we use:
//   - text:        plain assistant or user text
//   - tool_use:    model-side tool call (assistant turn)
//   - tool_result: local tool execution result (user turn)
type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type anthropicRequest struct {
	Model       string               `json:"model"`
	System      string               `json:"system,omitempty"`
	Messages    []anthropicMessage   `json:"messages"`
	Temperature *float64             `json:"temperature,omitempty"`
	MaxTokens   int                  `json:"max_tokens"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
}

type anthropicResponse struct {
	Model      string           `json:"model"`
	StopReason string           `json:"stop_reason"`
	Content    []anthropicBlock `json:"content"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Chat implements the unified Chat interface against Anthropic Messages API.
func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, opts Options) (*Result, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(c.DefaultModel)
	}
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	system, wireMessages, err := toAnthropicMessages(messages)
	if err != nil {
		return nil, err
	}
	if len(wireMessages) == 0 {
		return nil, fmt.Errorf("anthropic chat requires at least one user message")
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body := anthropicRequest{
		Model:       model,
		System:      system,
		Messages:    wireMessages,
		Temperature: opts.Temperature,
		MaxTokens:   maxTokens,
	}
	if len(opts.Tools) > 0 {
		body.Tools = toAnthropicTools(opts.Tools)
		if tc := normaliseAnthropicToolChoice(opts.ToolChoice); tc != nil {
			body.ToolChoice = tc
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	version := c.AnthropicVersion
	if version == "" {
		version = "2023-06-01"
	}
	req.Header.Set("anthropic-version", version)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call anthropic: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, truncateError(raw))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w (body=%s)", err, truncateError(raw))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("anthropic error: %s", parsed.Error.Message)
	}

	var sb strings.Builder
	var toolCalls []ToolCall
	for _, block := range parsed.Content {
		switch block.Type {
		case "", "text":
			sb.WriteString(block.Text)
		case "tool_use":
			args := block.Input
			if len(strings.TrimSpace(string(args))) == 0 {
				args = json.RawMessage("{}")
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	if sb.Len() == 0 && len(toolCalls) == 0 {
		return nil, ErrEmptyResponse
	}

	return &Result{
		Model:        firstNonEmpty(parsed.Model, model),
		Text:         sb.String(),
		FinishReason: parsed.StopReason,
		ToolCalls:    toolCalls,
	}, nil
}

// ChatStream issues a streaming Messages call. Anthropic streams events
// rather than chunked JSON, so we maintain a small state machine that
// tracks the active content block per index and emits StreamEvents as
// blocks complete.
func (c *AnthropicClient) ChatStream(ctx context.Context, messages []Message, opts Options, onEvent StreamCallback) error {
	if c.BaseURL == "" {
		return fmt.Errorf("base url is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("anthropic api key is required")
	}
	if onEvent == nil {
		return errors.New("anthropic: onEvent callback is required")
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(c.DefaultModel)
	}
	if model == "" {
		return fmt.Errorf("model is required")
	}

	system, wireMessages, err := toAnthropicMessages(messages)
	if err != nil {
		return err
	}
	if len(wireMessages) == 0 {
		return fmt.Errorf("anthropic chat requires at least one user message")
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body := struct {
		anthropicRequest
		Stream bool `json:"stream"`
	}{
		anthropicRequest: anthropicRequest{
			Model:       model,
			System:      system,
			Messages:    wireMessages,
			Temperature: opts.Temperature,
			MaxTokens:   maxTokens,
		},
		Stream: true,
	}
	if len(opts.Tools) > 0 {
		body.Tools = toAnthropicTools(opts.Tools)
		if tc := normaliseAnthropicToolChoice(opts.ToolChoice); tc != nil {
			body.ToolChoice = tc
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal anthropic stream request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build anthropic stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", c.APIKey)
	version := c.AnthropicVersion
	if version == "" {
		version = "2023-06-01"
	}
	req.Header.Set("anthropic-version", version)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = streamHTTPClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic http %d: %s", resp.StatusCode, truncateError(raw))
	}

	return parseAnthropicStream(resp.Body, model, onEvent)
}

// parseAnthropicStream consumes Server-Sent Events from Anthropic's
// Messages API. The event types we care about are:
//   - message_start          (captures model name)
//   - content_block_start    (records block type/index, captures tool_use id+name)
//   - content_block_delta    (forwards text_delta, accumulates input_json_delta)
//   - content_block_stop     (flushes tool_use block if applicable)
//   - message_delta          (captures stop_reason)
//   - message_stop           (terminates stream)
//   - error                  (terminates stream with an error)
func parseAnthropicStream(r io.Reader, fallbackModel string, onEvent StreamCallback) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type blockState struct {
		Kind      string // "text" / "tool_use"
		ToolID    string
		ToolName  string
		ToolInput strings.Builder
	}
	blocks := map[int]*blockState{}
	model := fallbackModel
	finish := ""
	var usageAcc TokenUsage

	emitDone := func(err error) error {
		done := StreamEvent{Kind: StreamEventDone, Model: model, Finish: finish}
		if !usageAcc.IsZero() {
			u := usageAcc
			done.Usage = &u
		}
		_ = onEvent(done)
		return err
	}

	flushBlock := func(idx int) error {
		st, ok := blocks[idx]
		if !ok || st == nil {
			return nil
		}
		if st.Kind == "tool_use" {
			args := strings.TrimSpace(st.ToolInput.String())
			if args == "" || !json.Valid([]byte(args)) {
				args = "{}"
			}
			tc := ToolCall{ID: st.ToolID, Name: st.ToolName, Arguments: json.RawMessage(args)}
			if err := onEvent(StreamEvent{Kind: StreamEventToolCall, ToolCall: &tc}); err != nil {
				return err
			}
		}
		delete(blocks, idx)
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var frame struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
			ContentBlock struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			continue
		}
		switch frame.Type {
		case "message_start":
			if strings.TrimSpace(frame.Message.Model) != "" {
				model = frame.Message.Model
			}
		case "content_block_start":
			st := &blockState{Kind: frame.ContentBlock.Type}
			if frame.ContentBlock.Type == "tool_use" {
				st.ToolID = frame.ContentBlock.ID
				st.ToolName = frame.ContentBlock.Name
				if input := strings.TrimSpace(string(frame.ContentBlock.Input)); input != "" && input != "{}" {
					st.ToolInput.WriteString(input)
				}
			}
			blocks[frame.Index] = st
		case "content_block_delta":
			st, ok := blocks[frame.Index]
			if !ok {
				continue
			}
			switch frame.Delta.Type {
			case "text_delta":
				if frame.Delta.Text != "" {
					if err := onEvent(StreamEvent{Kind: StreamEventText, Text: frame.Delta.Text}); err != nil {
						return emitDone(err)
					}
				}
			case "input_json_delta":
				st.ToolInput.WriteString(frame.Delta.PartialJSON)
			}
		case "content_block_stop":
			if err := flushBlock(frame.Index); err != nil {
				return emitDone(err)
			}
		case "message_delta":
			if frame.Delta.StopReason != "" {
				finish = frame.Delta.StopReason
			}
			if len(frame.Usage) > 0 {
				usageAcc = usageAcc.MergePreferNonZero(ParseAnthropicUsage(frame.Usage))
			}
		case "message_stop":
			return emitDone(nil)
		case "error":
			msg := ""
			if frame.Error != nil {
				msg = frame.Error.Message
			}
			return emitDone(fmt.Errorf("anthropic stream error: %s", msg))
		}
	}
	if err := scanner.Err(); err != nil {
		return emitDone(fmt.Errorf("anthropic stream read: %w", err))
	}
	return emitDone(nil)
}

// toAnthropicMessages converts our neutral message list into Anthropic's
// content-block wire format. Multiple consecutive "tool" messages (which
// happen when the model issues parallel tool calls in one turn) are merged
// into a single user message whose content holds one tool_result block per
// call — that's the only shape Anthropic accepts.
func toAnthropicMessages(messages []Message) (string, []anthropicMessage, error) {
	system := ""
	out := []anthropicMessage{}
	for i, m := range messages {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case "user":
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: []anthropicBlock{{Type: "text", Text: m.Content}},
			})
		case "assistant":
			blocks := []anthropicBlock{}
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				args := tc.Arguments
				if len(strings.TrimSpace(string(args))) == 0 {
					args = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: args,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			out = append(out, anthropicMessage{Role: "assistant", Content: blocks})
		case "tool":
			if m.ToolCallID == "" {
				return "", nil, fmt.Errorf("anthropic: message[%d] has role=tool but empty ToolCallID", i)
			}
			block := anthropicBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			// Merge into the previous user turn when applicable, otherwise
			// open a new one. Anthropic disallows two consecutive user
			// turns, so this merge is functional, not just an optimisation.
			if len(out) > 0 && out[len(out)-1].Role == "user" {
				out[len(out)-1].Content = append(out[len(out)-1].Content, block)
			} else {
				out = append(out, anthropicMessage{Role: "user", Content: []anthropicBlock{block}})
			}
		default:
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: []anthropicBlock{{Type: "text", Text: m.Content}},
			})
		}
	}
	return system, out, nil
}

func toAnthropicTools(tools []ToolDefinition) []anthropicTool {
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		schema := t.Parameters
		if len(strings.TrimSpace(string(schema))) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return out
}

func normaliseAnthropicToolChoice(choice string) *anthropicToolChoice {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "auto":
		return &anthropicToolChoice{Type: "auto"}
	case "required":
		return &anthropicToolChoice{Type: "any"}
	case "none":
		// Anthropic 没有原生 "none"，最稳的退路是不传 tools；但若调用方
		// 显式指定 none 而我们仍传 tools，至少阻止模型调用：限定 auto
		// 并在系统提示里要求。这里返回 nil 让 Anthropic 走默认（auto）。
		return nil
	default:
		return &anthropicToolChoice{Type: "tool", Name: choice}
	}
}
