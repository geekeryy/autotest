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
		"availableMockSets",
		"{{$mock.set.<key>}}",
	}
	for _, fragment := range guards {
		if !strings.Contains(generateParamsSystem, fragment) {
			t.Errorf("generateParamsSystem missing critical guard %q", fragment)
		}
	}
}

// TestMergeAvailableMockSetsInjectsField 锁定 availableMockSets 注入逻辑：
//   - 当 raw 为空对象 / nil 时仍能挂载字段；
//   - 当 raw 已包含字段时不会丢失原有字段；
//   - summaries 为空数组直接返回原 raw（避免噪声）。
func TestMergeAvailableMockSetsInjectsField(t *testing.T) {
	t.Parallel()

	summaries := []MockSetSummary{{
		Key:            "colors",
		Name:           "Colors",
		ValuesPreview:  []string{"red", "green", "blue"},
		HasWeights:     true,
		TotalValueSize: 3,
	}}

	got := mergeAvailableMockSets(nil, summaries)
	if !strings.Contains(string(got), "availableMockSets") || !strings.Contains(string(got), "\"key\":\"colors\"") {
		t.Fatalf("expected nil context to gain availableMockSets, got %s", got)
	}

	original := json.RawMessage(`{"method":"GET","path":"/api/v1/users"}`)
	merged := mergeAvailableMockSets(original, summaries)
	if !strings.Contains(string(merged), "\"method\":\"GET\"") {
		t.Fatalf("expected original fields preserved, got %s", merged)
	}
	if !strings.Contains(string(merged), "availableMockSets") {
		t.Fatalf("expected availableMockSets to be injected, got %s", merged)
	}

	if got := mergeAvailableMockSets(original, nil); string(got) != string(original) {
		t.Fatalf("expected empty summaries to leave context untouched, got %s", got)
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

// TestBuildMessagesAnalyzeFailureIncludesContext locks the basic shape of the
// `analyze_failure` user message: action-specific preamble + context block,
// plus the system prompt's three-section guard so the model's output remains
// stable even when callers bring different fields in `context`.
func TestBuildMessagesAnalyzeFailureIncludesContext(t *testing.T) {
	t.Parallel()

	ctx := json.RawMessage(`{
		"run": {"id": "abcd", "status": "failed"},
		"result": {
			"requestSnapshot": {"method": "POST", "url": "/api/orders"},
			"responseSnapshot": {"statusCode": 500, "body": {"error": "boom"}},
			"assertions": [{"type": "status_code", "passed": false, "message": "expected 200 got 500"}]
		}
	}`)
	messages, jsonOnly := buildMessages(ActionAnalyzeFailure, "", ctx, "")
	if jsonOnly {
		t.Fatalf("analyze_failure should NOT enable jsonOnly")
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatalf("unexpected messages shape: %+v", messages)
	}
	user := messages[1].Content
	for _, fragment := range []string{
		"请基于下方运行失败上下文",
		"上下文（JSON）",
		"\"statusCode\": 500",
		"\"method\": \"POST\"",
	} {
		if !strings.Contains(user, fragment) {
			t.Errorf("analyze_failure user message missing %q\n----\n%s", fragment, user)
		}
	}
}

// TestAnalyzeFailureSystemContainsCriticalGuards locks the high-leverage
// constraints so future edits cannot quietly drop the three-section structure
// or the "no fabrication" requirement.
func TestAnalyzeFailureSystemContainsCriticalGuards(t *testing.T) {
	t.Parallel()

	guards := []string{
		"测试失败分析助手",
		"## 失败原因",
		"## 关键证据",
		"## 修复建议",
		"禁止虚构",
		"requestSnapshot",
		"responseSnapshot",
		"assertions",
		"中文 Markdown",
		"工具策略",
	}
	for _, fragment := range guards {
		if !strings.Contains(analyzeFailureSystem, fragment) {
			t.Errorf("analyzeFailureSystem missing critical guard %q", fragment)
		}
	}
}

func TestBuildMessagesAnalyzeSpecChangesIncludesContext(t *testing.T) {
	t.Parallel()

	ctx := json.RawMessage(`{
		"diff": {
			"addedEndpoints": [{"method": "POST", "path": "/api/orders/v2"}],
			"removedEndpoints": [],
			"modifiedEndpoints": [
				{"method": "GET", "path": "/api/users/{id}", "changes": [
					{"fieldPath": "responseBody.email", "kind": "remove"}
				]}
			]
		},
		"affectedTemplates": [
			{"caseId": "c1", "name": "Get user", "method": "GET", "path": "/api/users/{id}"}
		],
		"affectedScenarioSteps": []
	}`)
	messages, jsonOnly := buildMessages(ActionAnalyzeSpecChanges, "", ctx, "")
	if jsonOnly {
		t.Fatalf("analyze_spec_changes should NOT enable jsonOnly")
	}
	user := messages[1].Content
	for _, fragment := range []string{
		"请基于下方 spec 结构化 diff",
		"上下文（JSON）",
		"addedEndpoints",
		"affectedTemplates",
		"\"path\": \"/api/users/{id}\"",
	} {
		if !strings.Contains(user, fragment) {
			t.Errorf("analyze_spec_changes user message missing %q\n----\n%s", fragment, user)
		}
	}
}

func TestAnalyzeSpecChangesSystemContainsCriticalGuards(t *testing.T) {
	t.Parallel()

	guards := []string{
		"OpenAPI/Swagger 变更影响分析助手",
		"## 变更概览",
		"## 详细变更清单",
		"## 对现有资产影响",
		"## 建议动作",
		"继续可用",
		"需调整",
		"应废弃",
		"affectedTemplates",
		"affectedScenarioSteps",
		"工具策略",
		"get_endpoint",
	}
	for _, fragment := range guards {
		if !strings.Contains(analyzeSpecChangesSystem, fragment) {
			t.Errorf("analyzeSpecChangesSystem missing critical guard %q", fragment)
		}
	}
}

// TestAssistantChatSystemContainsScenarioWorkflow locks the high-value
// guards in the global assistant prompt:
//   - The two-tier tool policy (read-only vs. mutating-with-confirmation),
//     including names so future renames break this test loudly.
//   - The scenario generation workflow (list → create_case → create
//     scenario) and the "you cannot run the scenario yourself" boundary
//     that came directly from the user's product requirement.
//   - The control-flow schema hints (stepOrder references, script.config,
//     for/condition shapes) so the model keeps emitting configs that
//     match the runtime parsers.
func TestAssistantChatSystemContainsScenarioWorkflow(t *testing.T) {
	t.Parallel()

	guards := []string{
		"全局 AI 助理",
		"list_services",
		"list_endpoints",
		"list_cases",
		"create_case_from_endpoint",
		"generate_coverage_scenarios",
		"create_scenario_with_steps",
		"add_scenario_step",
		"update_scenario_step",
		"delete_scenario_step",
		"reorder_scenario_steps",
		"update_case_assertions",
		"场景生成工作流",
		"你不能直接运行场景",
		"点击场景页右上角",
		"bodyStepOrders",
		"stepOrders",
		"goja 沙箱",
		"页面上下文",
		"权限与项目隔离",
		"projectId",
	}
	for _, fragment := range guards {
		if !strings.Contains(assistantChatSystem, fragment) {
			t.Errorf("assistantChatSystem missing critical guard %q", fragment)
		}
	}
}
