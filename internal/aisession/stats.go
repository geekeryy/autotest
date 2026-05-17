package aisession

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// messageUsageDetail mirrors aiprovider.AssistantUsageDetail JSON persisted in
// usage_details without importing aiprovider (avoids package cycles).
type messageUsageDetail struct {
	Hop                      int `json:"hop"`
	PromptTokens             int `json:"promptTokens"`
	CompletionTokens         int `json:"completionTokens"`
	TotalTokens              int `json:"totalTokens"`
	CachedPromptTokens       int `json:"cachedPromptTokens"`
	PromptCacheHitTokens     int `json:"promptCacheHitTokens"`
	PromptCacheMissTokens    int `json:"promptCacheMissTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
	ReasoningTokens          int `json:"reasoningTokens"`
	ElapsedMillis            int `json:"elapsedMillis"`
	Model                    string `json:"model"`
}

// SessionConversationStats summarizes non-system message counts and timing.
type SessionConversationStats struct {
	TotalMessages          int        `json:"totalMessages"`
	UserMessages           int        `json:"userMessages"`
	AssistantMessages      int        `json:"assistantMessages"`
	ToolMessages           int        `json:"toolMessages"`
	PendingConfirmMessages int        `json:"pendingConfirmMessages"`
	RejectedMessages       int        `json:"rejectedMessages"`
	AssistantWithToolCalls int        `json:"assistantWithToolCalls"`
	ToolCallCount          int        `json:"toolCallCount"`
	TotalElapsedMillis     int        `json:"totalElapsedMillis"`
	FirstMessageAt         *time.Time `json:"firstMessageAt,omitempty"`
	LastMessageAt          *time.Time `json:"lastMessageAt,omitempty"`
	ModelsUsed             []string   `json:"modelsUsed,omitempty"`
}

// SessionTokenUsageStats aggregates usage_details across assistant turns.
type SessionTokenUsageStats struct {
	RoundsWithUsage          int    `json:"roundsWithUsage"`
	PromptTokens             int    `json:"promptTokens"`
	CompletionTokens         int    `json:"completionTokens"`
	TotalTokens              int    `json:"totalTokens"`
	CachedPromptTokens       int    `json:"cachedPromptTokens"`
	PromptCacheHitTokens     int    `json:"promptCacheHitTokens"`
	PromptCacheMissTokens    int    `json:"promptCacheMissTokens"`
	CacheReadInputTokens     int    `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int    `json:"cacheCreationInputTokens"`
	ReasoningTokens          int    `json:"reasoningTokens"`
	UsageElapsedMillis       int    `json:"usageElapsedMillis"`
	HasUsageData             bool   `json:"hasUsageData"`
	Hint                     string `json:"hint,omitempty"`
}

// SessionDetail bundles session metadata with computed statistics.
type SessionDetail struct {
	Session      Session                  `json:"session"`
	Conversation SessionConversationStats `json:"conversation"`
	TokenUsage   SessionTokenUsageStats   `json:"tokenUsage"`
}

// BuildSessionDetail computes conversation and token statistics for a session.
func BuildSessionDetail(session Session, messages []Message) SessionDetail {
	return SessionDetail{
		Session:      session,
		Conversation: buildConversationStats(messages),
		TokenUsage:   buildTokenUsageStats(messages),
	}
}

func buildConversationStats(messages []Message) SessionConversationStats {
	out := SessionConversationStats{}
	modelSet := map[string]struct{}{}
	var first, last time.Time
	hasTime := false

	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		out.TotalMessages++
		switch m.Role {
		case "user":
			out.UserMessages++
		case "assistant":
			out.AssistantMessages++
			if m.ElapsedMillis > 0 {
				out.TotalElapsedMillis += m.ElapsedMillis
			}
			if m.Model != "" {
				modelSet[m.Model] = struct{}{}
			}
			switch m.Status {
			case StatusPendingConfirm:
				out.PendingConfirmMessages++
			case StatusRejected:
				out.RejectedMessages++
			}
			n := countToolCalls(m.ToolCalls)
			if n > 0 {
				out.AssistantWithToolCalls++
				out.ToolCallCount += n
			}
		case "tool":
			out.ToolMessages++
		}
		if !m.CreatedAt.IsZero() {
			if !hasTime || m.CreatedAt.Before(first) {
				first = m.CreatedAt
			}
			if !hasTime || m.CreatedAt.After(last) {
				last = m.CreatedAt
			}
			hasTime = true
		}
	}

	if hasTime {
		out.FirstMessageAt = &first
		out.LastMessageAt = &last
	}
	if len(modelSet) > 0 {
		out.ModelsUsed = make([]string, 0, len(modelSet))
		for model := range modelSet {
			out.ModelsUsed = append(out.ModelsUsed, model)
		}
		sort.Strings(out.ModelsUsed)
	}
	return out
}

func buildTokenUsageStats(messages []Message) SessionTokenUsageStats {
	out := SessionTokenUsageStats{}
	for _, m := range messages {
		if m.Role != "assistant" || len(m.UsageDetails) == 0 {
			continue
		}
		var detail messageUsageDetail
		if err := json.Unmarshal(m.UsageDetails, &detail); err != nil {
			continue
		}
		accumulateUsageDetail(&out, detail)
	}
	if !out.HasUsageData {
		out.Hint = tokenUsageEmptyHint
	}
	return out
}

func accumulateUsageDetail(out *SessionTokenUsageStats, detail messageUsageDetail) {
	if detail.PromptTokens == 0 && detail.CompletionTokens == 0 && detail.TotalTokens == 0 {
		return
	}
	out.HasUsageData = true
	out.RoundsWithUsage++
	out.PromptTokens += detail.PromptTokens
	out.CompletionTokens += detail.CompletionTokens
	if detail.TotalTokens > 0 {
		out.TotalTokens += detail.TotalTokens
	} else {
		out.TotalTokens += detail.PromptTokens + detail.CompletionTokens
	}
	out.CachedPromptTokens += detail.CachedPromptTokens
	out.PromptCacheHitTokens += detail.PromptCacheHitTokens
	out.PromptCacheMissTokens += detail.PromptCacheMissTokens
	out.CacheReadInputTokens += detail.CacheReadInputTokens
	out.CacheCreationInputTokens += detail.CacheCreationInputTokens
	out.ReasoningTokens += detail.ReasoningTokens
	if detail.ElapsedMillis > 0 {
		out.UsageElapsedMillis += detail.ElapsedMillis
	}
}

const tokenUsageEmptyHint = "暂无 Token 明细。进行 AI 助理对话后，assistant 回复会自动记录用量。"

func countToolCalls(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var calls []json.RawMessage
	if err := json.Unmarshal(raw, &calls); err != nil {
		return 0
	}
	return len(calls)
}

// FormatSessionTitle returns a display title for UI.
func FormatSessionTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return "新对话"
	}
	return t
}
