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
	"sort"
	"strings"
)

// OpenAICompatibleClient targets any service exposing the OpenAI /v1/chat/completions API,
// including DeepSeek, Kimi, Xiaomi MiLM gateway, OpenAI itself, and Ollama (which mirrors
// the schema under /v1/chat/completions when started normally).
type OpenAICompatibleClient struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
	Thinking     *OpenAIThinkingConfig
	// AuthHeader controls how the API key is sent. Defaults to "Authorization: Bearer".
	AuthHeader string
	// AuthScheme is the prefix in the Authorization header (e.g. "Bearer "). Empty for none.
	AuthScheme string
	// ExtraHeaders are added to every request (used for org IDs, custom routing, etc).
	ExtraHeaders map[string]string
}

// OpenAIThinkingConfig captures provider-specific reasoning controls for
// OpenAI-compatible gateways that expose thinking mode with different field
// names. Supported dialects today are "deepseek" and "xiaomi".
type OpenAIThinkingConfig struct {
	Enabled bool
	Dialect string
	Effort  string
	Summary string
}

// oaMessage mirrors the wire format used by OpenAI-compatible servers. Content
// is a pointer so we can emit the JSON literal `null` for assistant turns that
// only carry tool_calls — a number of upstream gateways reject the request if
// `content` is missing entirely.
type oaMessage struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Name             string          `json:"name,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	ToolCalls        []oaToolCall    `json:"tool_calls,omitempty"`
}

type oaToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function oaToolCallFn `json:"function"`
}

type oaToolCallFn struct {
	Name string `json:"name"`
	// Arguments is a JSON-encoded string per the OpenAI spec, not a nested
	// object. We store the raw JSON string here and convert from/to
	// json.RawMessage at the boundary.
	Arguments string `json:"arguments"`
}

type oaTool struct {
	Type     string    `json:"type"`
	Function oaToolDef `json:"function"`
}

type oaToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type oaChatRequest struct {
	Model           string      `json:"model"`
	Messages        []oaMessage `json:"messages"`
	Temperature     *float64    `json:"temperature,omitempty"`
	MaxTokens       int         `json:"max_tokens,omitempty"`
	Stream          bool        `json:"stream"`
	StreamOptions   *oaStreamOptions `json:"stream_options,omitempty"`
	Format          interface{} `json:"response_format,omitempty"`
	Tools           []oaTool    `json:"tools,omitempty"`
	ToolChoice      any         `json:"tool_choice,omitempty"`
	Thinking        any         `json:"thinking,omitempty"`
	ReasoningEffort string      `json:"reasoning_effort,omitempty"`
	Reasoning       any         `json:"reasoning,omitempty"`
}

type oaStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type oaChatResponse struct {
	Choices []struct {
		Index        int       `json:"index"`
		FinishReason string    `json:"finish_reason"`
		Message      oaMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Model string `json:"model"`
}

// oaStreamChunk is the shape of a single SSE chunk emitted by OpenAI-style
// chat-completion streams. We only model the fields we care about.
type oaStreamChunk struct {
	Choices []struct {
		Index        int           `json:"index"`
		Delta        oaStreamDelta `json:"delta"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Model string          `json:"model"`
	Usage json.RawMessage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type oaStreamDelta struct {
	Role             string             `json:"role"`
	Content          *string            `json:"content"`
	ReasoningContent *string            `json:"reasoning_content"`
	ToolCalls        []oaStreamToolCall `json:"tool_calls"`
}

// oaStreamToolCall mirrors OpenAI's incremental tool-call payload. Note
// that `Index` is the stable identifier inside a single response — `id`
// only appears on the first chunk, and `arguments` arrives in fragments.
type oaStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

// Chat implements the unified Chat interface.
func (c *OpenAICompatibleClient) Chat(ctx context.Context, messages []Message, opts Options) (*Result, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(c.DefaultModel)
	}
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	wireMessages, err := toOAMessages(messages)
	if err != nil {
		return nil, err
	}

	body := oaChatRequest{
		Model:       model,
		Messages:    wireMessages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	// json_object 与 tools 同时存在时部分提供商会报错；优先 tools。
	if len(opts.Tools) > 0 {
		body.Tools = toOATools(opts.Tools)
		body.ToolChoice = normaliseToolChoice(opts.ToolChoice)
	} else if opts.JSONOnly {
		body.Format = map[string]string{"type": "json_object"}
	}
	applyOpenAIThinking(&body, mergeOpenAIThinking(c.Thinking, opts.Thinking))

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	header := c.AuthHeader
	if header == "" {
		header = "Authorization"
	}
	scheme := c.AuthScheme
	if header == "Authorization" && scheme == "" {
		scheme = "Bearer "
	}
	if c.APIKey != "" {
		req.Header.Set(header, scheme+c.APIKey)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai provider: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ai provider response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ai provider http %d: %s", resp.StatusCode, friendlyOpenAIHTTPError(resp.StatusCode, raw))
	}

	var parsed oaChatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode ai provider response: %w (body=%s)", err, truncateError(raw))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("ai provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, ErrEmptyResponse
	}

	choice := parsed.Choices[0]
	out := &Result{
		Model:        firstNonEmpty(parsed.Model, model),
		FinishReason: choice.FinishReason,
		ToolCalls:    fromOAToolCalls(choice.Message.ToolCalls),
	}
	if len(choice.Message.Content) > 0 {
		var text string
		if err := json.Unmarshal(choice.Message.Content, &text); err == nil {
			out.Text = text
		}
	}
	out.ReasoningContent = choice.Message.ReasoningContent
	if out.Text == "" && out.ReasoningContent == "" && len(out.ToolCalls) == 0 {
		return nil, ErrEmptyResponse
	}
	return out, nil
}

// ChatStream issues a streaming chat completion. Text content is forwarded
// to onEvent as it arrives; tool calls are buffered until their argument
// streams complete and then emitted once each. A final StreamEventDone is
// always sent (even when the upstream errors mid-stream — the error is
// returned from this method, but the caller still gets the chance to close
// out their SSE writer in onEvent).
func (c *OpenAICompatibleClient) ChatStream(ctx context.Context, messages []Message, opts Options, onEvent StreamCallback) error {
	if c.BaseURL == "" {
		return fmt.Errorf("base url is required")
	}
	if onEvent == nil {
		return errors.New("openai: onEvent callback is required")
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(c.DefaultModel)
	}
	if model == "" {
		return fmt.Errorf("model is required")
	}

	wireMessages, err := toOAMessages(messages)
	if err != nil {
		return err
	}

	body := oaChatRequest{
		Model:       model,
		Messages:    wireMessages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stream:      true,
	}
	if opts.CollectUsage {
		body.StreamOptions = &oaStreamOptions{IncludeUsage: true}
	}
	if len(opts.Tools) > 0 {
		body.Tools = toOATools(opts.Tools)
		body.ToolChoice = normaliseToolChoice(opts.ToolChoice)
	} else if opts.JSONOnly {
		body.Format = map[string]string{"type": "json_object"}
	}
	applyOpenAIThinking(&body, mergeOpenAIThinking(c.Thinking, opts.Thinking))

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal openai stream request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build openai stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	header := c.AuthHeader
	if header == "" {
		header = "Authorization"
	}
	scheme := c.AuthScheme
	if header == "Authorization" && scheme == "" {
		scheme = "Bearer "
	}
	if c.APIKey != "" {
		req.Header.Set(header, scheme+c.APIKey)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = streamHTTPClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call ai provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ai provider http %d: %s", resp.StatusCode, friendlyOpenAIHTTPError(resp.StatusCode, raw))
	}

	streamErr := parseOpenAIStream(resp.Body, model, onEvent)
	return streamErr
}

// parseOpenAIStream reads SSE lines and dispatches StreamEvents. It returns
// early on context cancellation or on callback errors but always tries to
// emit a final StreamEventDone so the SSE handler can close cleanly.
func parseOpenAIStream(r io.Reader, fallbackModel string, onEvent StreamCallback) error {
	scanner := bufio.NewScanner(r)
	// SSE payloads can be large for long-tool-call argument blobs; bump
	// the buffer to avoid bufio.ErrTooLong on heavy tool-arguments JSON.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type partialCall struct {
		ID        string
		Name      string
		Arguments strings.Builder
	}
	calls := map[int]*partialCall{}
	model := fallbackModel
	finish := ""
	var usageAcc TokenUsage

	flush := func(err error) error {
		// Emit completed tool calls in stable index order so downstream
		// consumers can rely on deterministic ordering across providers.
		indexes := make([]int, 0, len(calls))
		for idx := range calls {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)
		for _, idx := range indexes {
			pc := calls[idx]
			if pc == nil {
				continue
			}
			args := strings.TrimSpace(pc.Arguments.String())
			if args == "" || !json.Valid([]byte(args)) {
				args = "{}"
			}
			tc := ToolCall{ID: pc.ID, Name: pc.Name, Arguments: json.RawMessage(args)}
			if cbErr := onEvent(StreamEvent{Kind: StreamEventToolCall, ToolCall: &tc}); cbErr != nil && err == nil {
				err = cbErr
			}
		}
		done := StreamEvent{Kind: StreamEventDone, Model: model, Finish: finish}
		if !usageAcc.IsZero() {
			u := usageAcc
			done.Usage = &u
		}
		_ = onEvent(done)
		return err
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
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			return flush(nil)
		}
		var chunk oaStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 单帧解析失败不应中断整条流——记录并继续。
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return flush(fmt.Errorf("ai provider stream error: %s", chunk.Error.Message))
		}
		if strings.TrimSpace(chunk.Model) != "" {
			model = chunk.Model
		}
		if len(chunk.Usage) > 0 {
			usageAcc = usageAcc.MergePreferNonZero(ParseOpenAICompatibleUsage(chunk.Usage))
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != nil {
				text := *choice.Delta.Content
				if text != "" {
					if err := onEvent(StreamEvent{Kind: StreamEventText, Text: text}); err != nil {
						return flush(err)
					}
				}
			}
			if choice.Delta.ReasoningContent != nil {
				text := *choice.Delta.ReasoningContent
				if text != "" {
					if err := onEvent(StreamEvent{Kind: StreamEventReasoning, ReasoningContent: text}); err != nil {
						return flush(err)
					}
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				pc, ok := calls[tc.Index]
				if !ok {
					pc = &partialCall{}
					calls[tc.Index] = pc
				}
				if tc.ID != "" {
					pc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					pc.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					pc.Arguments.WriteString(tc.Function.Arguments)
				}
			}
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return flush(fmt.Errorf("openai stream read: %w", err))
	}
	return flush(nil)
}

// toOAMessages converts neutral messages into the OpenAI wire format. The
// only non-trivial mapping is for "tool" responses (which must carry
// tool_call_id) and assistant turns with tool_calls (which must have content
// = null on the wire).
func toOAMessages(messages []Message) ([]oaMessage, error) {
	out := make([]oaMessage, 0, len(messages))
	for i, m := range messages {
		oa := oaMessage{Role: m.Role, ReasoningContent: m.ReasoningContent, Name: m.Name}
		switch m.Role {
		case "assistant":
			if len(m.ToolCalls) > 0 {
				oa.ToolCalls = make([]oaToolCall, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					args := strings.TrimSpace(string(tc.Arguments))
					if args == "" {
						args = "{}"
					}
					oa.ToolCalls = append(oa.ToolCalls, oaToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: oaToolCallFn{
							Name:      tc.Name,
							Arguments: args,
						},
					})
				}
				if m.Content != "" {
					encoded, err := encodeOAContent(m.Content, nil)
					if err != nil {
						return nil, fmt.Errorf("openai: message[%d] content: %w", i, err)
					}
					oa.Content = encoded
				} else {
					oa.Content = json.RawMessage("null")
				}
			} else {
				encoded, err := encodeOAContent(m.Content, nil)
				if err != nil {
					return nil, fmt.Errorf("openai: message[%d] content: %w", i, err)
				}
				oa.Content = encoded
			}
		case "tool":
			if m.ToolCallID == "" {
				return nil, fmt.Errorf("openai: message[%d] has role=tool but empty ToolCallID", i)
			}
			oa.ToolCallID = m.ToolCallID
			encoded, err := encodeOAContent(m.Content, nil)
			if err != nil {
				return nil, fmt.Errorf("openai: message[%d] content: %w", i, err)
			}
			oa.Content = encoded
		default:
			encoded, err := encodeOAContent(m.Content, m.Images)
			if err != nil {
				return nil, fmt.Errorf("openai: message[%d] content: %w", i, err)
			}
			oa.Content = encoded
		}
		out = append(out, oa)
	}
	return out, nil
}

func toOATools(tools []ToolDefinition) []oaTool {
	out := make([]oaTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, oaTool{
			Type: "function",
			Function: oaToolDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

func fromOAToolCalls(calls []oaToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(calls))
	for _, c := range calls {
		args := strings.TrimSpace(c.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		// 模型偶尔会返回非法 JSON 参数；这里仍传给上层，由 tool runner 自行校验。
		out = append(out, ToolCall{
			ID:        c.ID,
			Name:      c.Function.Name,
			Arguments: json.RawMessage(args),
		})
	}
	return out
}

// normaliseToolChoice maps our neutral string into OpenAI's polymorphic
// tool_choice field. Empty string defaults to "auto" because we already
// populated Tools.
func normaliseToolChoice(choice string) any {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "auto":
		return "auto"
	case "none":
		return "none"
	case "required":
		return "required"
	default:
		return choice
	}
}

func applyOpenAIThinking(body *oaChatRequest, cfg *OpenAIThinkingConfig) {
	if body == nil || cfg == nil || !cfg.Enabled {
		return
	}
	dialect := strings.ToLower(strings.TrimSpace(cfg.Dialect))
	effort := strings.TrimSpace(cfg.Effort)
	summary := strings.TrimSpace(cfg.Summary)

	switch dialect {
	case "deepseek":
		body.Thinking = map[string]string{"type": "enabled"}
		if effort != "" {
			body.ReasoningEffort = effort
		}
	case "xiaomi":
		reasoning := map[string]string{}
		if effort != "" {
			reasoning["effort"] = effort
		}
		if summary != "" {
			reasoning["summary"] = summary
		}
		if len(reasoning) == 0 {
			reasoning["effort"] = "medium"
		}
		body.Reasoning = reasoning
	}
}

func mergeOpenAIThinking(base, override *OpenAIThinkingConfig) *OpenAIThinkingConfig {
	if override == nil {
		return base
	}
	cfg := *override
	if base != nil {
		if strings.TrimSpace(cfg.Dialect) == "" {
			cfg.Dialect = base.Dialect
		}
		if strings.TrimSpace(cfg.Effort) == "" {
			cfg.Effort = base.Effort
		}
		if strings.TrimSpace(cfg.Summary) == "" {
			cfg.Summary = base.Summary
		}
	}
	return &cfg
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func friendlyOpenAIHTTPError(status int, raw []byte) string {
	body := truncateError(raw)
	lower := strings.ToLower(body)
	if status == http.StatusNotFound && strings.Contains(lower, "image input") {
		return "当前模型不支持图片输入；含图片时应由平台自动使用 MiMo-V2.5，请确认网关已开通该型号"
	}
	return body
}

func truncateError(raw []byte) string {
	const limit = 600
	s := strings.TrimSpace(string(raw))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
