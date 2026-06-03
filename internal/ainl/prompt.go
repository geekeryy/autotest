// Package ainl 实现自然语言编排引擎，将用户的自然语言描述转换为可运行的测试场景。
package ainl

import (
	"fmt"
	"strings"

	"autotest/internal/spec"
)

// EndpointBrief 是端点的精简描述，用于 prompt 构建和语义匹配。
type EndpointBrief struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary,omitempty"`
	OperationID string `json:"operationId,omitempty"`
}

// toEndpointBriefs 将 spec.Endpoint 列表转换为精简格式。
func toEndpointBriefs(endpoints []spec.Endpoint) []EndpointBrief {
	out := make([]EndpointBrief, 0, len(endpoints))
	for _, ep := range endpoints {
		out = append(out, EndpointBrief{
			Method:      ep.Method,
			Path:        ep.Path,
			Summary:     ep.Summary,
			OperationID: ep.OperationID,
		})
	}
	return out
}

// buildEndpointCatalog 将端点列表格式化为 prompt 中的目录文本。
func buildEndpointCatalog(endpoints []EndpointBrief) string {
	var sb strings.Builder
	for i, ep := range endpoints {
		line := fmt.Sprintf("%d. %s %s", i+1, ep.Method, ep.Path)
		if ep.Summary != "" {
			line += " — " + ep.Summary
		}
		if ep.OperationID != "" {
			line += fmt.Sprintf(" (operationId: %s)", ep.OperationID)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// BuildParsePrompt 构建用于 LLM 解析自然语言的完整 prompt。
// 输入：用户描述 + 可用端点列表。
// 输出：要求 LLM 返回 JSON 格式的操作序列。
func BuildParsePrompt(description string, endpoints []EndpointBrief) string {
	catalog := buildEndpointCatalog(endpoints)

	return fmt.Sprintf(`你是一个接口自动化测试场景编排助手。用户会用自然语言描述一个测试流程，你需要将其解析为结构化的操作序列。

## 可用接口列表
%s
## 用户描述
%s

## 输出要求
请严格输出 JSON（不要 markdown 围栏），格式如下：
{
  "scenarioName": "场景名称（简洁概括用户意图）",
  "scenarioDescription": "场景描述（一句话说明测试目的）",
  "steps": [
    {
      "stepOrder": 1,
      "stepType": "api",
      "name": "步骤名称",
      "method": "GET",
      "path": "/api/users",
      "request": {
        "query": {"page": "1"},
        "headers": {},
        "body": null
      },
      "assertions": [
        {"type": "status_code", "expected": 200},
        {"type": "json_path", "path": "$.code", "expected": 0}
      ],
      "description": "这一步做了什么"
    }
  ],
  "notes": "编排说明或注意事项（可选）"
}

## 规则
1. method 和 path 必须严格匹配上面可用接口列表中的某个接口
2. 如果用户描述的操作无法匹配到可用接口，在 notes 中说明并跳过该步骤
3. 步骤顺序应遵循用户描述的操作顺序
4. 如果前置步骤（如登录）的响应可能包含 token，后续步骤应在 request.headers 中用 {"Authorization": "Bearer {{steps[N].response.data.token}}"} 的形式引用
5. 每个 API 步骤至少包含一个 status_code 断言
6. request 中不需要的字段设为 null 或省略
7. assertions 的 type 支持：status_code、json_path、json_path_exists、response_time、body_contains
8. 如果用户描述模糊，根据接口的 summary 推断最合理的参数
`, catalog, description)
}

// LLMOperations 是 LLM 返回的解析结果。
type LLMOperations struct {
	ScenarioName        string       `json:"scenarioName"`
	ScenarioDescription string       `json:"scenarioDescription"`
	Steps               []LLMStep    `json:"steps"`
	Notes               string       `json:"notes,omitempty"`
}

// LLMStep 是 LLM 返回的单个步骤。
type LLMStep struct {
	StepOrder   int                `json:"stepOrder"`
	StepType    string             `json:"stepType"`
	Name        string             `json:"name"`
	Method      string             `json:"method"`
	Path        string             `json:"path"`
	Request     map[string]any     `json:"request,omitempty"`
	Assertions  []map[string]any   `json:"assertions,omitempty"`
	Description string             `json:"description,omitempty"`
}

// BuildMatchPrompt 构建端点模糊匹配的 prompt。
// 当 LLM 输出的 method+path 无法精确匹配时，用此 prompt 让 LLM 选择最接近的端点。
func BuildMatchPrompt(method, path string, candidates []EndpointBrief) string {
	var sb strings.Builder
	sb.WriteString("你是一个 API 端点匹配助手。以下接口无法精确匹配，请从候选列表中选择最接近的一个。\n\n")
	sb.WriteString(fmt.Sprintf("目标接口：%s %s\n\n", method, path))
	sb.WriteString("候选接口：\n")
	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("%d. %s %s", i+1, c.Method, c.Path))
		if c.Summary != "" {
			sb.WriteString(" — " + c.Summary)
		}
		sb.WriteString("\n")
	}
	sb.WriteString(`\n请输出 JSON（不要 markdown 围栏）：
{"matchedIndex": 1, "confidence": "high|medium|low", "reason": "匹配原因"}

如果没有合适的匹配，输出：
{"matchedIndex": -1, "confidence": "none", "reason": "原因"}
`)
	return sb.String()
}
