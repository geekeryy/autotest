package client

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestToOAMessages_PlainConversation(t *testing.T) {
	in := []Message{
		{Role: "system", Content: "你是助手"},
		{Role: "user", Content: "查询接口 X"},
		{Role: "assistant", Content: "好的"},
	}
	out, err := toOAMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	for i, m := range out {
		if len(m.Content) == 0 {
			t.Fatalf("message[%d] content unexpectedly empty", i)
		}
		var text string
		if err := json.Unmarshal(m.Content, &text); err != nil || text != in[i].Content {
			t.Fatalf("message[%d] content mismatch: %s", i, string(m.Content))
		}
	}
}

func TestToOAMessages_UserMultimodal(t *testing.T) {
	in := []Message{
		{
			Role:    "user",
			Content: "描述图片",
			Images: []ImageAttachment{
				{URL: "data:image/png;base64,AA=="},
			},
		},
	}
	out, err := toOAMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parts []map[string]any
	if err := json.Unmarshal(out[0].Content, &parts); err != nil {
		t.Fatalf("expected content array, got %s", out[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
}

func TestToOAMessages_AssistantToolCalls(t *testing.T) {
	in := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "get_case", Arguments: json.RawMessage(`{"caseId":"abc"}`)},
			},
		},
	}
	out, err := toOAMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if string(out[0].Content) != "null" {
		t.Fatalf("assistant tool_call message must have null content, got %s", out[0].Content)
	}
	if len(out[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out[0].ToolCalls))
	}
	tc := out[0].ToolCalls[0]
	if tc.Type != "function" || tc.ID != "call_1" || tc.Function.Name != "get_case" {
		t.Fatalf("unexpected tool call: %+v", tc)
	}
	if tc.Function.Arguments != `{"caseId":"abc"}` {
		t.Fatalf("unexpected arguments: %q", tc.Function.Arguments)
	}
}

func TestToOAMessages_AssistantReasoningContent(t *testing.T) {
	in := []Message{
		{
			Role:             "assistant",
			Content:          "准备调用工具",
			ReasoningContent: "需要先查询接口详情",
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "get_case", Arguments: json.RawMessage(`{"caseId":"abc"}`)},
			},
		},
	}
	out, err := toOAMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].ReasoningContent != "需要先查询接口详情" {
		t.Fatalf("reasoning_content dropped: %+v", out[0])
	}
}

func TestToOAMessages_ToolResult(t *testing.T) {
	in := []Message{
		{Role: "tool", ToolCallID: "call_1", Content: `{"ok":true}`},
	}
	out, err := toOAMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].Role != "tool" || out[0].ToolCallID != "call_1" {
		t.Fatalf("unexpected tool message: %+v", out[0])
	}
	var toolContent string
	if err := json.Unmarshal(out[0].Content, &toolContent); err != nil || toolContent != `{"ok":true}` {
		t.Fatalf("unexpected content: %s", out[0].Content)
	}
}

func TestToOAMessages_ToolMessageWithoutCallIDFails(t *testing.T) {
	in := []Message{{Role: "tool", Content: "x"}}
	if _, err := toOAMessages(in); err == nil {
		t.Fatalf("expected error for tool message without ToolCallID")
	}
}

func TestFromOAToolCalls_NormalisesEmptyArgs(t *testing.T) {
	calls := []oaToolCall{
		{ID: "c1", Type: "function", Function: oaToolCallFn{Name: "x", Arguments: ""}},
	}
	out := fromOAToolCalls(calls)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out))
	}
	if string(out[0].Arguments) != "{}" {
		t.Fatalf("expected empty arguments to be normalised to {}, got %q", out[0].Arguments)
	}
}

func TestNormaliseToolChoice(t *testing.T) {
	cases := map[string]any{
		"":         "auto",
		"auto":     "auto",
		"none":     "none",
		"required": "required",
		"AUTO":     "auto",
		"get_case": "get_case",
	}
	for in, want := range cases {
		got := normaliseToolChoice(in)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("normaliseToolChoice(%q): got %v, want %v", in, got, want)
		}
	}
}

func TestApplyOpenAIThinking(t *testing.T) {
	deepseek := oaChatRequest{}
	applyOpenAIThinking(&deepseek, &OpenAIThinkingConfig{Enabled: true, Dialect: "deepseek", Effort: "high"})
	if deepseek.ReasoningEffort != "high" {
		t.Fatalf("deepseek reasoning_effort mismatch: %+v", deepseek)
	}
	thinking, ok := deepseek.Thinking.(map[string]string)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("deepseek thinking payload mismatch: %#v", deepseek.Thinking)
	}

	xiaomi := oaChatRequest{}
	applyOpenAIThinking(&xiaomi, &OpenAIThinkingConfig{Enabled: true, Dialect: "xiaomi", Effort: "high", Summary: "auto"})
	reasoning, ok := xiaomi.Reasoning.(map[string]string)
	if !ok || reasoning["effort"] != "high" || reasoning["summary"] != "auto" {
		t.Fatalf("xiaomi reasoning payload mismatch: %#v", xiaomi.Reasoning)
	}
}

func TestMergeOpenAIThinking_RequestOverride(t *testing.T) {
	base := &OpenAIThinkingConfig{Enabled: true, Dialect: "deepseek", Effort: "high"}

	disabled := mergeOpenAIThinking(base, &OpenAIThinkingConfig{Enabled: false})
	if disabled == nil || disabled.Enabled || disabled.Dialect != "deepseek" || disabled.Effort != "high" {
		t.Fatalf("disabled override should keep dialect/effort but disable thinking: %+v", disabled)
	}

	custom := mergeOpenAIThinking(base, &OpenAIThinkingConfig{Enabled: true, Effort: "max"})
	if custom == nil || !custom.Enabled || custom.Dialect != "deepseek" || custom.Effort != "max" {
		t.Fatalf("custom override mismatch: %+v", custom)
	}
}

func TestOpenAIThinkingFromSpec_DefaultsAndOverrides(t *testing.T) {
	deepseek := openAIThinkingFromSpec(Spec{Type: "deepseek"})
	if deepseek == nil || !deepseek.Enabled || deepseek.Dialect != "deepseek" || deepseek.Effort != "high" {
		t.Fatalf("unexpected deepseek thinking config: %+v", deepseek)
	}

	disabled := openAIThinkingFromSpec(Spec{
		Type:        "xiaomi",
		ExtraConfig: map[string]any{"thinking": map[string]any{"enabled": false}},
	})
	if disabled != nil {
		t.Fatalf("expected disabled xiaomi thinking to return nil, got %+v", disabled)
	}

	custom := openAIThinkingFromSpec(Spec{
		Type: "xiaomi",
		ExtraConfig: map[string]any{
			"thinking": map[string]any{"effort": "medium", "summary": "auto"},
		},
	})
	if custom == nil || custom.Effort != "medium" || custom.Summary != "auto" {
		t.Fatalf("unexpected custom xiaomi thinking config: %+v", custom)
	}
}

func TestToAnthropicMessages_AssistantToolUse(t *testing.T) {
	in := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{
			Role:    "assistant",
			Content: "好的，让我查一下。",
			ToolCalls: []ToolCall{
				{ID: "toolu_1", Name: "get_case", Arguments: json.RawMessage(`{"caseId":"abc"}`)},
			},
		},
	}
	system, out, err := toAnthropicMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if system != "sys" {
		t.Fatalf("system mismatch: %q", system)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(out))
	}
	asst := out[1]
	if asst.Role != "assistant" {
		t.Fatalf("expected assistant, got %s", asst.Role)
	}
	if len(asst.Content) != 2 {
		t.Fatalf("expected 2 blocks (text + tool_use), got %d", len(asst.Content))
	}
	if asst.Content[0].Type != "text" || asst.Content[0].Text == "" {
		t.Fatalf("expected first block to be text: %+v", asst.Content[0])
	}
	if asst.Content[1].Type != "tool_use" || asst.Content[1].ID != "toolu_1" || asst.Content[1].Name != "get_case" {
		t.Fatalf("unexpected tool_use block: %+v", asst.Content[1])
	}
}

func TestToAnthropicMessages_MergesConsecutiveToolResults(t *testing.T) {
	in := []Message{
		{Role: "user", Content: "do stuff"},
		{
			Role:      "assistant",
			ToolCalls: []ToolCall{{ID: "tu1", Name: "get_case", Arguments: json.RawMessage(`{}`)}},
		},
		{Role: "tool", ToolCallID: "tu1", Content: `{"ok":1}`},
		{Role: "tool", ToolCallID: "tu2", Content: `{"ok":2}`},
	}
	_, out, err := toAnthropicMessages(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages (user, assistant, user-merged), got %d", len(out))
	}
	merged := out[2]
	if merged.Role != "user" {
		t.Fatalf("expected merged turn to be user role, got %s", merged.Role)
	}
	if len(merged.Content) != 2 {
		t.Fatalf("expected 2 tool_result blocks, got %d", len(merged.Content))
	}
	for i, b := range merged.Content {
		if b.Type != "tool_result" {
			t.Fatalf("block[%d] type: %s", i, b.Type)
		}
	}
}

func TestToAnthropicMessages_ToolWithoutCallIDFails(t *testing.T) {
	in := []Message{{Role: "tool", Content: "x"}}
	if _, _, err := toAnthropicMessages(in); err == nil {
		t.Fatalf("expected error for tool message without ToolCallID")
	}
}

func TestToAnthropicTools_DefaultsSchema(t *testing.T) {
	out := toAnthropicTools([]ToolDefinition{
		{Name: "x", Description: "y"},
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	schema := strings.TrimSpace(string(out[0].InputSchema))
	if schema == "" || schema == "null" {
		t.Fatalf("expected default schema, got %q", schema)
	}
}

func TestNormaliseAnthropicToolChoice(t *testing.T) {
	if c := normaliseAnthropicToolChoice(""); c == nil || c.Type != "auto" {
		t.Errorf("empty string should map to auto, got %+v", c)
	}
	if c := normaliseAnthropicToolChoice("required"); c == nil || c.Type != "any" {
		t.Errorf("required should map to any, got %+v", c)
	}
	if c := normaliseAnthropicToolChoice("none"); c != nil {
		t.Errorf("none should return nil (no native equivalent), got %+v", c)
	}
	if c := normaliseAnthropicToolChoice("get_case"); c == nil || c.Type != "tool" || c.Name != "get_case" {
		t.Errorf("named choice should map to tool/name, got %+v", c)
	}
}
