package aisession

import (
	"encoding/json"
	"testing"
)

func TestAccumulateUsageDetail(t *testing.T) {
	var out SessionTokenUsageStats
	accumulateUsageDetail(&out, messageUsageDetail{
		PromptTokens:         100,
		CompletionTokens:     20,
		TotalTokens:          120,
		CachedPromptTokens:   10,
		PromptCacheHitTokens: 5,
	})
	if !out.HasUsageData || out.PromptTokens != 100 || out.TotalTokens != 120 {
		t.Fatalf("first: %+v", out)
	}
	accumulateUsageDetail(&out, messageUsageDetail{
		PromptTokens:     50,
		CompletionTokens: 10,
	})
	if out.RoundsWithUsage != 2 || out.PromptTokens != 150 {
		t.Fatalf("second: %+v", out)
	}
}

func TestBuildTokenUsageStatsUsesAccumulate(t *testing.T) {
	usage, _ := json.Marshal(messageUsageDetail{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	stats := buildTokenUsageStats([]Message{
		{Role: "assistant", UsageDetails: usage},
	})
	if !stats.HasUsageData || stats.TotalTokens != 3 {
		t.Fatalf("stats: %+v", stats)
	}
}
