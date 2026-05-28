package aiprovider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
)

// TestBuildClientMessages_FiltersPendingAndPreservesToolCalls covers the
// most subtle conversion logic in the assistant loop: persisted
// pending_confirm messages must be dropped before replaying the history,
// otherwise the model would re-request the same mutating tool and the
// user would see a duplicate confirmation card.
func TestBuildClientMessages_FiltersPendingAndPreservesToolCalls(t *testing.T) {
	history := []StoredMessage{
		{Role: "user", Content: "你好", Status: storedStatusFinal},
		{
			Role:             "assistant",
			Content:          "我会拉取信息",
			ReasoningContent: "需要先查接口",
			Status:           storedStatusFinal,
			ToolCalls: json.RawMessage(`[
				{"id":"call_1","name":"get_case","arguments":{"caseId":"abc"}}
			]`),
		},
		{Role: "tool", Content: `{"id":"abc"}`, ToolCallID: "call_1", Status: storedStatusFinal},
		{
			Role:      "assistant",
			Content:   "需要更新断言",
			Status:    storedStatusPendingConfirm,
			ToolCalls: json.RawMessage(`[{"id":"call_2","name":"update_case_assertions","arguments":{}}]`),
		},
	}

	msgs := buildClientMessages(history, nil, "", true)

	// 1) 系统提示先行；2) 跳过 pending_confirm 消息。
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (system + user + assistant + tool), got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != assistantChatSystem {
		t.Fatalf("first message must be the assistant system prompt, got %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "你好" {
		t.Fatalf("unexpected user message: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" {
		t.Fatalf("expected assistant turn after user, got %+v", msgs[2])
	}
	if len(msgs[2].ToolCalls) != 1 || msgs[2].ToolCalls[0].ID != "call_1" {
		t.Fatalf("assistant turn should keep tool_calls intact, got %+v", msgs[2].ToolCalls)
	}
	if msgs[2].ReasoningContent != "需要先查接口" {
		t.Fatalf("assistant turn should keep reasoning_content, got %q", msgs[2].ReasoningContent)
	}
	if msgs[3].Role != "tool" || msgs[3].ToolCallID != "call_1" {
		t.Fatalf("expected tool turn linked by call_1, got %+v", msgs[3])
	}
}

// TestBuildClientMessages_InjectsPageContext verifies that the
// page-context snapshot (an opaque JSON blob the frontend submits with
// every chat request) is rendered as an extra system message inserted
// right after the assistant prompt. This is the mechanism that lets the
// AI assistant be aware of the user's current page.
func TestBuildClientMessages_InjectsPageContext(t *testing.T) {
	page := json.RawMessage(`{"path":"/scenarios/abc","scenarioId":"abc","scenarioName":"checkout flow"}`)
	msgs := buildClientMessages(nil, page, "", false)

	if len(msgs) != 2 {
		t.Fatalf("expected system + page-context system message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != assistantChatSystem {
		t.Fatalf("first message must be the assistant system prompt, got %+v", msgs[0])
	}
	if msgs[1].Role != "system" {
		t.Fatalf("page-context message should be a system message, got %+v", msgs[1])
	}
	if !strings.Contains(msgs[1].Content, "用户当前页面上下文") {
		t.Errorf("page context system message missing header, got %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "checkout flow") {
		t.Errorf("page context system message should contain scenarioName, got %q", msgs[1].Content)
	}
}

// TestBuildClientMessages_SkipsEmptyPageContext makes sure we don't
// spend a token budget on placeholder snapshots — null / empty object /
// pure whitespace must produce no extra system message.
func TestBuildClientMessages_SkipsEmptyPageContext(t *testing.T) {
	cases := []struct {
		name string
		in   json.RawMessage
	}{
		{"nil", nil},
		{"null literal", json.RawMessage("null")},
		{"empty object", json.RawMessage("{}")},
		{"only whitespace", json.RawMessage("   ")},
		{"malformed", json.RawMessage("{not-json")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs := buildClientMessages(nil, tc.in, "", false)
			if len(msgs) != 1 {
				t.Fatalf("expected only the assistant prompt, got %d msgs", len(msgs))
			}
			if msgs[0].Content != assistantChatSystem {
				t.Errorf("unexpected first message: %+v", msgs[0])
			}
		})
	}
}

// TestPickMutating_FiltersByToolFlag ensures we only treat tools whose
// registry entry has Mutating=true as confirmation-worthy.
func TestPickMutating_FiltersByToolFlag(t *testing.T) {
	tools := map[string]aitools.Tool{
		"get_case":               {Name: "get_case", Mutating: false, Run: noopRun},
		"update_case_assertions": {Name: "update_case_assertions", Mutating: true, Run: noopRun},
	}
	calls := []client.ToolCall{
		{ID: "c1", Name: "get_case", Arguments: json.RawMessage(`{}`)},
		{ID: "c2", Name: "update_case_assertions", Arguments: json.RawMessage(`{"caseId":"x"}`)},
		{ID: "c3", Name: "unknown_tool", Arguments: json.RawMessage(`{}`)},
	}
	got := pickMutating(calls, tools)
	if len(got) != 1 {
		t.Fatalf("expected exactly one mutating call, got %d", len(got))
	}
	if got[0].ID != "c2" || !got[0].Mutating {
		t.Fatalf("unexpected mutating projection: %+v", got[0])
	}
}

// TestToStoredCalls_TagsMutatingFlag verifies that the JSON we persist
// for an assistant turn preserves the Mutating flag derived from the tool
// registry — confirm endpoints rely on this flag to decide whether a
// call can be skipped entirely.
func TestToStoredCalls_TagsMutatingFlag(t *testing.T) {
	tools := map[string]aitools.Tool{
		"a": {Name: "a", Mutating: false, Run: noopRun},
		"b": {Name: "b", Mutating: true, Run: noopRun},
	}
	calls := []client.ToolCall{
		{ID: "1", Name: "a", Arguments: json.RawMessage(`{}`)},
		{ID: "2", Name: "b", Arguments: json.RawMessage(`{"foo":1}`)},
	}
	stored := toStoredCalls(calls, tools)
	if len(stored) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(stored))
	}
	if stored[0].Mutating || !stored[1].Mutating {
		t.Fatalf("mutating flag mismatch: %+v", stored)
	}
	// 序列化结果可以反序列化回相同的形状。
	body, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal stored calls: %v", err)
	}
	var back []StoredToolCall
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal stored calls: %v", err)
	}
	if back[1].Mutating != true {
		t.Fatalf("mutating flag dropped on round-trip: %+v", back)
	}
}

// TestRemoveCall_DropsTargetAndPreservesOthers checks the helper used to
// rewrite a pending assistant message after the user has decided one of
// its calls.
func TestRemoveCall_DropsTargetAndPreservesOthers(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":"c1","name":"a","arguments":{}},
		{"id":"c2","name":"b","arguments":{},"mutating":true},
		{"id":"c3","name":"c","arguments":{}}
	]`)
	out := removeCall(raw, "c2")
	var calls []StoredToolCall
	if err := json.Unmarshal(out, &calls); err != nil {
		t.Fatalf("unmarshal output: %v (raw=%s)", err, out)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls remaining, got %d", len(calls))
	}
	if calls[0].ID != "c1" || calls[1].ID != "c3" {
		t.Fatalf("unexpected ordering after removal: %+v", calls)
	}
	// 若所有 call 都被剔除应返回空数组而非 null。
	emptied := removeCall(out, "c1")
	emptied = removeCall(emptied, "c3")
	if got := strings.TrimSpace(string(emptied)); got != "[]" {
		t.Fatalf("expected empty array, got %q", got)
	}
}

func TestKeepOnlyCall_PreservesDecidedCall(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":"c1","name":"a","arguments":{}},
		{"id":"c2","name":"b","arguments":{"x":1},"mutating":true},
		{"id":"c3","name":"c","arguments":{}}
	]`)
	out := keepOnlyCall(raw, "c2")
	var calls []StoredToolCall
	if err := json.Unmarshal(out, &calls); err != nil {
		t.Fatalf("unmarshal output: %v (raw=%s)", err, out)
	}
	if len(calls) != 1 || calls[0].ID != "c2" || !calls[0].Mutating {
		t.Fatalf("expected only decided call to remain, got %+v", calls)
	}
}

func TestAssistantThinkingOverride(t *testing.T) {
	enabled := true
	cfg := assistantThinkingOverride(&providerRow{ProviderType: ProviderTypeDeepSeek}, &enabled, "max")
	if cfg == nil || !cfg.Enabled || cfg.Dialect != "deepseek" || cfg.Effort != "max" {
		t.Fatalf("unexpected deepseek override: %+v", cfg)
	}

	disabled := false
	cfg = assistantThinkingOverride(&providerRow{ProviderType: ProviderTypeXiaomi}, &disabled, "")
	if cfg == nil || cfg.Enabled || cfg.Dialect != "xiaomi" {
		t.Fatalf("unexpected disabled xiaomi override: %+v", cfg)
	}

	cfg = assistantThinkingOverride(&providerRow{ProviderType: ProviderTypeOpenAI}, &enabled, "high")
	if cfg == nil || cfg.Enabled || cfg.Dialect != "" {
		t.Fatalf("unsupported provider must not enable thinking: %+v", cfg)
	}
}

func noopRun(ctx context.Context, args json.RawMessage) (any, error) {
	return nil, nil
}

func TestBuildHopUsage_PersistsWhenDebugOff(t *testing.T) {
	cfg := &streamConfig{DebugEnabled: false, Provider: &providerRow{ProviderType: ProviderTypeOpenAI}}
	usage := client.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	raw, detail := cfg.buildHopUsage(1, "gpt-4", 120, usage)
	if len(raw) == 0 || detail == nil || detail.PromptTokens != 10 {
		t.Fatalf("expected usage persisted, got raw=%q detail=%+v", raw, detail)
	}
	if err := cfg.emitUsageIfNeeded(detail); err != nil {
		t.Fatalf("emitUsageIfNeeded: %v", err)
	}
}

func TestBuildHopUsage_EmitsSSEOnlyWhenDebugOn(t *testing.T) {
	cfg := &streamConfig{DebugEnabled: true, Provider: &providerRow{ProviderType: ProviderTypeOpenAI}}
	usage := client.TokenUsage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4}
	_, detail := cfg.buildHopUsage(1, "gpt-4", 50, usage)
	var got *AssistantUsageDetail
	cfg.Sink = func(ev AssistantStreamEvent) error {
		if ev.Kind == StreamEventUsage {
			got = ev.Usage
		}
		return nil
	}
	if err := cfg.emitUsageIfNeeded(detail); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PromptTokens != 3 {
		t.Fatalf("expected usage SSE, got %+v", got)
	}
}
