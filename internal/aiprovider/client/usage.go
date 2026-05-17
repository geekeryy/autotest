package client

import (
	"encoding/json"
)

// TokenUsage is a provider-neutral view of one LLM completion's token accounting.
// Fields are populated when upstream returns usage metadata (OpenAI
// stream_options.include_usage, Anthropic message_delta.usage, etc.).
type TokenUsage struct {
	PromptTokens             int
	CompletionTokens         int
	TotalTokens              int
	CachedPromptTokens       int // OpenAI prompt_tokens_details.cached_tokens
	PromptCacheHitTokens     int // DeepSeek prompt_cache_hit_tokens
	PromptCacheMissTokens    int // DeepSeek prompt_cache_miss_tokens
	CacheReadInputTokens     int // Anthropic cache_read_input_tokens
	CacheCreationInputTokens int // Anthropic cache_creation_input_tokens
	ReasoningTokens          int // OpenAI completion_tokens_details.reasoning_tokens
	ProviderRaw              json.RawMessage
}

func (u TokenUsage) IsZero() bool {
	return u.PromptTokens == 0 &&
		u.CompletionTokens == 0 &&
		u.TotalTokens == 0 &&
		u.CachedPromptTokens == 0 &&
		u.PromptCacheHitTokens == 0 &&
		u.PromptCacheMissTokens == 0 &&
		u.CacheReadInputTokens == 0 &&
		u.CacheCreationInputTokens == 0 &&
		u.ReasoningTokens == 0 &&
		len(u.ProviderRaw) == 0
}

// MergePreferNonZero keeps the latest non-zero counters. Streaming providers may
// emit usage on multiple chunks; the final chunk is authoritative.
func (u TokenUsage) MergePreferNonZero(next TokenUsage) TokenUsage {
	out := u
	if next.PromptTokens > 0 {
		out.PromptTokens = next.PromptTokens
	}
	if next.CompletionTokens > 0 {
		out.CompletionTokens = next.CompletionTokens
	}
	if next.TotalTokens > 0 {
		out.TotalTokens = next.TotalTokens
	}
	if next.CachedPromptTokens > 0 {
		out.CachedPromptTokens = next.CachedPromptTokens
	}
	if next.PromptCacheHitTokens > 0 {
		out.PromptCacheHitTokens = next.PromptCacheHitTokens
	}
	if next.PromptCacheMissTokens > 0 {
		out.PromptCacheMissTokens = next.PromptCacheMissTokens
	}
	if next.CacheReadInputTokens > 0 {
		out.CacheReadInputTokens = next.CacheReadInputTokens
	}
	if next.CacheCreationInputTokens > 0 {
		out.CacheCreationInputTokens = next.CacheCreationInputTokens
	}
	if next.ReasoningTokens > 0 {
		out.ReasoningTokens = next.ReasoningTokens
	}
	if len(next.ProviderRaw) > 0 {
		out.ProviderRaw = next.ProviderRaw
	}
	return out
}

// ParseOpenAICompatibleUsage parses OpenAI Chat Completions usage objects and
// DeepSeek-compatible extensions (prompt_cache_hit_tokens, etc.).
func ParseOpenAICompatibleUsage(raw json.RawMessage) TokenUsage {
	if len(raw) == 0 {
		return TokenUsage{}
	}
	var body struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return TokenUsage{ProviderRaw: raw}
	}
	return TokenUsage{
		PromptTokens:             body.PromptTokens,
		CompletionTokens:         body.CompletionTokens,
		TotalTokens:              body.TotalTokens,
		CachedPromptTokens:       body.PromptTokensDetails.CachedTokens,
		PromptCacheHitTokens:     body.PromptCacheHitTokens,
		PromptCacheMissTokens:    body.PromptCacheMissTokens,
		ReasoningTokens:          body.CompletionTokensDetails.ReasoningTokens,
		ProviderRaw:              raw,
	}
}

// ParseAnthropicUsage parses Anthropic Messages API usage objects.
func ParseAnthropicUsage(raw json.RawMessage) TokenUsage {
	if len(raw) == 0 {
		return TokenUsage{}
	}
	var body struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return TokenUsage{ProviderRaw: raw}
	}
	total := body.InputTokens + body.OutputTokens
	return TokenUsage{
		PromptTokens:             body.InputTokens,
		CompletionTokens:         body.OutputTokens,
		TotalTokens:              total,
		CacheReadInputTokens:     body.CacheReadInputTokens,
		CacheCreationInputTokens: body.CacheCreationInputTokens,
		ProviderRaw:              raw,
	}
}
