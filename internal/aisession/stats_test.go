package aisession

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildSessionDetail(t *testing.T) {
	sid := uuid.New()
	now := time.Now().UTC()
	session := Session{ID: sid, Title: "测试会话", UpdatedAt: now}

	usage, _ := json.Marshal(messageUsageDetail{
		Hop:              1,
		PromptTokens:     100,
		CompletionTokens: 40,
		TotalTokens:      140,
		CachedPromptTokens: 30,
		ElapsedMillis:    1200,
		Model:            "gpt-4",
	})

	messages := []Message{
		{Role: "user", Content: "hi", CreatedAt: now},
		{Role: "assistant", Content: "hello", Status: StatusFinal, Model: "gpt-4", ElapsedMillis: 800, UsageDetails: usage, CreatedAt: now.Add(time.Second)},
		{Role: "tool", Content: "{}", ToolCallID: "c1", CreatedAt: now.Add(2 * time.Second)},
	}

	detail := BuildSessionDetail(session, messages)
	if detail.Conversation.TotalMessages != 3 {
		t.Fatalf("total messages: %d", detail.Conversation.TotalMessages)
	}
	if detail.Conversation.UserMessages != 1 || detail.Conversation.AssistantMessages != 1 {
		t.Fatalf("role counts: %+v", detail.Conversation)
	}
	if !detail.TokenUsage.HasUsageData || detail.TokenUsage.PromptTokens != 100 {
		t.Fatalf("token: %+v", detail.TokenUsage)
	}
}
