package client

import (
	"encoding/json"
	"testing"
)

func TestParseOpenAICompatibleUsage(t *testing.T) {
	raw := json.RawMessage(`{
		"prompt_tokens": 120,
		"completion_tokens": 40,
		"total_tokens": 160,
		"prompt_tokens_details": {"cached_tokens": 80},
		"completion_tokens_details": {"reasoning_tokens": 12}
	}`)
	u := ParseOpenAICompatibleUsage(raw)
	if u.PromptTokens != 120 || u.CompletionTokens != 40 || u.TotalTokens != 160 {
		t.Fatalf("tokens: %+v", u)
	}
	if u.CachedPromptTokens != 80 || u.ReasoningTokens != 12 {
		t.Fatalf("details: %+v", u)
	}
}

func TestParseOpenAICompatibleUsageDeepSeekCache(t *testing.T) {
	raw := json.RawMessage(`{
		"prompt_tokens": 100,
		"completion_tokens": 20,
		"total_tokens": 120,
		"prompt_cache_hit_tokens": 60,
		"prompt_cache_miss_tokens": 40
	}`)
	u := ParseOpenAICompatibleUsage(raw)
	if u.PromptCacheHitTokens != 60 || u.PromptCacheMissTokens != 40 {
		t.Fatalf("cache: %+v", u)
	}
}

func TestParseAnthropicUsage(t *testing.T) {
	raw := json.RawMessage(`{
		"input_tokens": 90,
		"output_tokens": 30,
		"cache_read_input_tokens": 50,
		"cache_creation_input_tokens": 10
	}`)
	u := ParseAnthropicUsage(raw)
	if u.PromptTokens != 90 || u.CompletionTokens != 30 || u.TotalTokens != 120 {
		t.Fatalf("tokens: %+v", u)
	}
	if u.CacheReadInputTokens != 50 || u.CacheCreationInputTokens != 10 {
		t.Fatalf("cache: %+v", u)
	}
}

func TestTokenUsageMergePreferNonZero(t *testing.T) {
	a := TokenUsage{PromptTokens: 1}
	b := TokenUsage{PromptTokens: 99, CompletionTokens: 2}
	m := a.MergePreferNonZero(b)
	if m.PromptTokens != 99 || m.CompletionTokens != 2 {
		t.Fatalf("merge: %+v", m)
	}
}
