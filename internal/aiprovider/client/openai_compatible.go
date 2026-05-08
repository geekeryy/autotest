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

// OpenAICompatibleClient targets any service exposing the OpenAI /v1/chat/completions API,
// including DeepSeek, Kimi, Xiaomi MiLM gateway, OpenAI itself, and Ollama (which mirrors
// the schema under /v1/chat/completions when started normally).
type OpenAICompatibleClient struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
	// AuthHeader controls how the API key is sent. Defaults to "Authorization: Bearer".
	AuthHeader string
	// AuthScheme is the prefix in the Authorization header (e.g. "Bearer "). Empty for none.
	AuthScheme string
	// ExtraHeaders are added to every request (used for org IDs, custom routing, etc).
	ExtraHeaders map[string]string
}

type oaChatRequest struct {
	Model       string      `json:"model"`
	Messages    []Message   `json:"messages"`
	Temperature *float64    `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
	Format      interface{} `json:"response_format,omitempty"`
}

type oaChatResponse struct {
	Choices []struct {
		Index        int     `json:"index"`
		FinishReason string  `json:"finish_reason"`
		Message      Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Model string `json:"model"`
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

	body := oaChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	if opts.JSONOnly {
		body.Format = map[string]string{"type": "json_object"}
	}

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
		return nil, fmt.Errorf("ai provider http %d: %s", resp.StatusCode, truncateError(raw))
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

	out := &Result{
		Model:        firstNonEmpty(parsed.Model, model),
		Text:         parsed.Choices[0].Message.Content,
		FinishReason: parsed.Choices[0].FinishReason,
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncateError(raw []byte) string {
	const limit = 600
	s := strings.TrimSpace(string(raw))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
