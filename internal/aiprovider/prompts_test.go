package aiprovider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildMessagesGenerateParamsIncludesContextAndPreamble(t *testing.T) {
	t.Parallel()

	ctx := json.RawMessage(`{
		"method": "POST",
		"path": "/api/v1/users/{id}",
		"pathVarNames": ["id"],
		"endpoint": {
			"summary": "Create user",
			"requestSchema": {"parameters": [], "body": {"type": "object"}}
		},
		"currentRequest": {
			"pathVars": {"id": "fixed-id"},
			"query": {},
			"headers": {},
			"body": null
		}
	}`)

	messages, jsonOnly := buildMessages(ActionGenerateParams, "", ctx, "")
	if !jsonOnly {
		t.Fatalf("generate_params must request JSON-only mode")
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Fatalf("first message role = %q, want system", messages[0].Role)
	}
	if !strings.Contains(messages[0].Content, "请求参数生成器") {
		t.Errorf("system prompt missing role declaration: %q", messages[0].Content)
	}

	user := messages[1].Content
	requiredFragments := []string{
		"请基于下方上下文生成请求参数 JSON",
		"endpoint.requestSchema",
		"pathVarNames",
		"currentRequest",
		"上下文（JSON）",
		"\"path\": \"/api/v1/users/{id}\"",
		"\"id\": \"fixed-id\"",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(user, fragment) {
			t.Errorf("user message missing %q\n----\n%s\n----", fragment, user)
		}
	}
}

func TestGenerateParamsSystemContainsCriticalGuards(t *testing.T) {
	t.Parallel()

	// Lock the high-leverage rules into the system prompt so future edits do
	// not silently regress generation quality. Each fragment maps to a known
	// failure mode (e.g. losing user-filled values, hallucinated path vars,
	// markdown fences breaking JSON parse).
	guards := []string{
		"必须",
		"只能",
		"输出单个 JSON 对象",
		"pathVarNames",
		"保留 currentRequest 中已存在且非空的值",
		"不要使用三个反引号",
		"仅对字符串字段",
		"requestSchema.body.properties",
	}
	for _, fragment := range guards {
		if !strings.Contains(generateParamsSystem, fragment) {
			t.Errorf("generateParamsSystem missing critical guard %q", fragment)
		}
	}
}

func TestBuildMessagesAssertionRequiresIntent(t *testing.T) {
	t.Parallel()

	ctx := json.RawMessage(`{"snapshot": {"status": 200}}`)
	messages, jsonOnly := buildMessages(ActionGenerateAssertion, "断言 status=200 且 body.code 为 0", ctx, "")
	if jsonOnly {
		t.Fatalf("generate_assertion should NOT enable jsonOnly")
	}
	user := messages[1].Content
	if !strings.Contains(user, "测试意图（中文）") {
		t.Errorf("assertion user message missing intent label: %q", user)
	}
	if !strings.Contains(user, "断言 status=200 且 body.code 为 0") {
		t.Errorf("assertion intent text dropped: %q", user)
	}
	if !strings.Contains(user, "上下文（JSON）") {
		t.Errorf("assertion context block missing")
	}
}

func TestBuildMessagesSystemOverrideTakesPrecedence(t *testing.T) {
	t.Parallel()

	ctx := json.RawMessage(`{}`)
	messages, _ := buildMessages(ActionGenerateParams, "", ctx, "自定义 system prompt 内容")
	if messages[0].Content != "自定义 system prompt 内容" {
		t.Errorf("expected systemOverride to replace built-in prompt, got %q", messages[0].Content)
	}
}

func TestBuildMessagesEmptyContextStillIncludesPreamble(t *testing.T) {
	t.Parallel()

	messages, _ := buildMessages(ActionGenerateParams, "", nil, "")
	user := messages[1].Content
	if !strings.Contains(user, "请基于下方上下文生成请求参数 JSON") {
		t.Errorf("preamble missing for empty context: %q", user)
	}
}
