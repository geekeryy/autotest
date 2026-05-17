package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ModelInfo is a single model entry returned to the UI.
type ModelInfo struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"displayName,omitempty"`
	OwnedBy      string   `json:"ownedBy,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// ListModels queries the upstream provider for available model IDs.
// OpenAI-compatible providers use GET {baseURL}/models; Anthropic uses
// GET {baseURL}/models; Ollama falls back to native GET /api/tags when
// the OpenAI-compatible listing is empty or unavailable.
func ListModels(ctx context.Context, spec Spec) ([]ModelInfo, error) {
	providerType := strings.ToLower(strings.TrimSpace(spec.Type))
	switch providerType {
	case "anthropic":
		return listAnthropicModels(ctx, anthropicFromSpec(spec))
	case "ollama":
		return listOllamaModels(ctx, openAICompatibleFromSpec(spec))
	default:
		return listOpenAIModels(ctx, openAICompatibleFromSpec(spec))
	}
}

func listOpenAIModels(ctx context.Context, c *OpenAICompatibleClient) ([]ModelInfo, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}
	applyOpenAIAuth(req, c)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = modelsHTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read models response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list models http %d: %s", resp.StatusCode, truncateError(raw))
	}

	out, err := parseOpenAIModelsPayload(raw)
	if err != nil {
		return nil, err
	}
	return filterAndSortModels(out), nil
}

func parseOpenAIModelsPayload(raw []byte) ([]ModelInfo, error) {
	var payload struct {
		Data   []json.RawMessage `json:"data"`
		Models []json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	rows := payload.Data
	if len(rows) == 0 {
		rows = payload.Models
	}
	out := make([]ModelInfo, 0, len(rows))
	for _, rowRaw := range rows {
		var item openAIModelRow
		if err := json.Unmarshal(rowRaw, &item); err != nil {
			continue
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.Name)
		}
		if id == "" {
			continue
		}
		caps := InferModelCapabilities(id, ModelCapabilitiesFromMetadata(rowRaw))
		out = append(out, ModelInfo{
			ID:           id,
			OwnedBy:      strings.TrimSpace(item.OwnedBy),
			Capabilities: caps,
		})
	}
	return out, nil
}

type openAIModelRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func listAnthropicModels(ctx context.Context, c *AnthropicClient) ([]ModelInfo, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build anthropic models request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	version := c.AnthropicVersion
	if version == "" {
		// Models list requires a recent API version; chat still defaults in Chat().
		version = "2023-06-01"
	}
	req.Header.Set("anthropic-version", version)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = modelsHTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list anthropic models: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic models response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list anthropic models http %d: %s", resp.StatusCode, truncateError(raw))
	}

	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode anthropic models response: %w", err)
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return nil, fmt.Errorf("anthropic models api: %s", payload.Error.Message)
	}

	out := make([]ModelInfo, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		out = append(out, ModelInfo{
			ID:          id,
			DisplayName: strings.TrimSpace(item.DisplayName),
		})
	}
	return filterAndSortModels(out), nil
}

func listOllamaModels(ctx context.Context, c *OpenAICompatibleClient) ([]ModelInfo, error) {
	models, err := listOpenAIModels(ctx, c)
	if err == nil && len(models) > 0 {
		return models, nil
	}
	native, nativeErr := listOllamaNativeTags(ctx, c)
	if nativeErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, nativeErr
	}
	return native, nil
}

func listOllamaNativeTags(ctx context.Context, c *OpenAICompatibleClient) ([]ModelInfo, error) {
	base := ollamaNativeBase(c.BaseURL)
	if base == "" {
		return nil, fmt.Errorf("base url is required")
	}
	url := strings.TrimRight(base, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build ollama tags request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = modelsHTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list ollama models: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama tags response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list ollama models http %d: %s", resp.StatusCode, truncateError(raw))
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode ollama tags response: %w", err)
	}
	out := make([]ModelInfo, 0, len(payload.Models))
	for _, item := range payload.Models {
		id := strings.TrimSpace(item.Name)
		if id == "" {
			continue
		}
		out = append(out, ModelInfo{ID: id})
	}
	return filterAndSortModels(out), nil
}

func ollamaNativeBase(openAIBase string) string {
	s := strings.TrimRight(strings.TrimSpace(openAIBase), "/")
	if strings.HasSuffix(s, "/v1") {
		return strings.TrimSuffix(s, "/v1")
	}
	return s
}

func applyOpenAIAuth(req *http.Request, c *OpenAICompatibleClient) {
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
}

// filterAndSortModels drops obvious non-chat entries and sorts by id.
func filterAndSortModels(models []ModelInfo) []ModelInfo {
	if len(models) == 0 {
		return models
	}
	out := make([]ModelInfo, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, m := range models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		lower := strings.ToLower(id)
		if strings.Contains(lower, "embed") ||
			strings.Contains(lower, "dall-e") ||
			strings.Contains(lower, "moderation") {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		m.ID = id
		if len(m.Capabilities) == 0 {
			m.Capabilities = InferModelCapabilities(id, nil)
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// ModelIDs extracts sorted model id strings from ModelInfo slice.
func ModelIDs(models []ModelInfo) []string {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

var modelsHTTPClient = &http.Client{Timeout: 20 * time.Second}
