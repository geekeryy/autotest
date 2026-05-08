package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Spec describes how to build a Client. It is constructed by the service from a stored Provider.
type Spec struct {
	Type         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	ExtraConfig  map[string]any
}

// New constructs a Client from a Spec.
func New(spec Spec) (Client, error) {
	switch strings.ToLower(strings.TrimSpace(spec.Type)) {
	case "deepseek", "openai", "kimi", "xiaomi":
		return openAICompatibleFromSpec(spec), nil
	case "ollama":
		return openAICompatibleFromSpec(spec), nil
	case "anthropic":
		return anthropicFromSpec(spec), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, spec.Type)
	}
}

// SpecFromExtra parses provider extra_config JSON for client construction.
func SpecFromExtra(extra []byte) (map[string]any, error) {
	if len(extra) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(extra, &out); err != nil {
		return nil, fmt.Errorf("decode provider extraConfig: %w", err)
	}
	return out, nil
}

func openAICompatibleFromSpec(spec Spec) *OpenAICompatibleClient {
	c := &OpenAICompatibleClient{
		BaseURL:      spec.BaseURL,
		APIKey:       spec.APIKey,
		DefaultModel: spec.DefaultModel,
	}
	if spec.ExtraConfig == nil {
		return c
	}
	if v, ok := spec.ExtraConfig["organization"].(string); ok && v != "" {
		c.ExtraHeaders = map[string]string{"OpenAI-Organization": v}
	}
	if v, ok := spec.ExtraConfig["headers"].(map[string]any); ok {
		if c.ExtraHeaders == nil {
			c.ExtraHeaders = map[string]string{}
		}
		for k, val := range v {
			if s, ok := val.(string); ok {
				c.ExtraHeaders[k] = s
			}
		}
	}
	return c
}

func anthropicFromSpec(spec Spec) *AnthropicClient {
	c := &AnthropicClient{
		BaseURL:      spec.BaseURL,
		APIKey:       spec.APIKey,
		DefaultModel: spec.DefaultModel,
	}
	if spec.ExtraConfig != nil {
		if v, ok := spec.ExtraConfig["anthropicVersion"].(string); ok && v != "" {
			c.AnthropicVersion = v
		}
	}
	return c
}
