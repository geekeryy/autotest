package aiprovider

import (
	"encoding/json"

	"autotest/internal/aiprovider/client"
)

// AssistantUsageDetail is persisted on assistant messages. It is streamed over
// SSE only when debug mode is enabled (live per-hop UI in the chat panel).
type AssistantUsageDetail struct {
	Hop                      int             `json:"hop,omitempty"`
	ProviderType             string          `json:"providerType,omitempty"`
	Model                    string          `json:"model,omitempty"`
	ElapsedMillis            int             `json:"elapsedMillis,omitempty"`
	PromptTokens             int             `json:"promptTokens,omitempty"`
	CompletionTokens         int             `json:"completionTokens,omitempty"`
	TotalTokens              int             `json:"totalTokens,omitempty"`
	CachedPromptTokens       int             `json:"cachedPromptTokens,omitempty"`
	PromptCacheHitTokens     int             `json:"promptCacheHitTokens,omitempty"`
	PromptCacheMissTokens    int             `json:"promptCacheMissTokens,omitempty"`
	CacheReadInputTokens     int             `json:"cacheReadInputTokens,omitempty"`
	CacheCreationInputTokens int             `json:"cacheCreationInputTokens,omitempty"`
	ReasoningTokens          int             `json:"reasoningTokens,omitempty"`
	ProviderRaw              json.RawMessage `json:"providerRaw,omitempty"`
}

func usageDetailFromClient(
	providerType, model string,
	hop, elapsedMillis int,
	usage client.TokenUsage,
) *AssistantUsageDetail {
	if usage.IsZero() {
		return nil
	}
	return &AssistantUsageDetail{
		Hop:                      hop,
		ProviderType:             providerType,
		Model:                    model,
		ElapsedMillis:            elapsedMillis,
		PromptTokens:             usage.PromptTokens,
		CompletionTokens:         usage.CompletionTokens,
		TotalTokens:              usage.TotalTokens,
		CachedPromptTokens:       usage.CachedPromptTokens,
		PromptCacheHitTokens:     usage.PromptCacheHitTokens,
		PromptCacheMissTokens:    usage.PromptCacheMissTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		ReasoningTokens:          usage.ReasoningTokens,
		ProviderRaw:              usage.ProviderRaw,
	}
}

func marshalUsageDetail(detail *AssistantUsageDetail) json.RawMessage {
	if detail == nil {
		return nil
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	return body
}
