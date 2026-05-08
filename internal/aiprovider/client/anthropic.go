package client

import (
	"bytes"
	"context"
	"encoding/json"
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

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
}

type anthropicResponse struct {
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
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

	system := ""
	userMessages := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case "user", "assistant":
			userMessages = append(userMessages, anthropicMessage{Role: m.Role, Content: m.Content})
		default:
			userMessages = append(userMessages, anthropicMessage{Role: "user", Content: m.Content})
		}
	}
	if len(userMessages) == 0 {
		return nil, fmt.Errorf("anthropic chat requires at least one user message")
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body := anthropicRequest{
		Model:       model,
		System:      system,
		Messages:    userMessages,
		Temperature: opts.Temperature,
		MaxTokens:   maxTokens,
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
	for _, block := range parsed.Content {
		if block.Type == "text" || block.Type == "" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return nil, ErrEmptyResponse
	}

	return &Result{
		Model:        firstNonEmpty(parsed.Model, model),
		Text:         sb.String(),
		FinishReason: parsed.StopReason,
	}, nil
}
