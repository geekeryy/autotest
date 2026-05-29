package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// EmbeddingClient is an optional capability for OpenAI-compatible providers
// that expose POST /v1/embeddings.
type EmbeddingClient interface {
	Embed(ctx context.Context, model string, inputs []string) ([][]float32, error)
}

type oaEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type oaEmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed calls the OpenAI-compatible embeddings endpoint. Empty model falls
// back to DefaultModel on the client.
func (c *OpenAICompatibleClient) Embed(ctx context.Context, model string, inputs []string) ([][]float32, error) {
	if c == nil {
		return nil, fmt.Errorf("embedding client is nil")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(c.DefaultModel)
	}
	if model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	clean := make([]string, 0, len(inputs))
	for _, in := range inputs {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		clean = append(clean, in)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("embedding input is empty")
	}

	body, err := json.Marshal(oaEmbeddingRequest{Model: model, Input: clean})
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	header := "Authorization"
	scheme := "Bearer "
	if c.AuthHeader != "" {
		header = c.AuthHeader
	}
	if c.AuthScheme != "" {
		scheme = c.AuthScheme
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
		return nil, fmt.Errorf("call embedding provider: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embedding provider http %d: %s", resp.StatusCode, friendlyOpenAIHTTPError(resp.StatusCode, raw))
	}

	var parsed oaEmbeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("embedding provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return nil, ErrEmptyResponse
	}

	sort.Slice(parsed.Data, func(i, j int) bool {
		return parsed.Data[i].Index < parsed.Data[j].Index
	})
	out := make([][]float32, len(parsed.Data))
	for i, row := range parsed.Data {
		if len(row.Embedding) == 0 {
			return nil, fmt.Errorf("embedding row %d is empty", row.Index)
		}
		out[i] = row.Embedding
	}
	return out, nil
}
