package aiassert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

// ErrCaseIDRequired 表示未提供测试用例 ID。
var ErrCaseIDRequired = errors.New("caseId 不能为空")

// ErrAssertionsEmpty 表示未提供任何断言。
var ErrAssertionsEmpty = errors.New("assertions 不能为空")

// InferredAssertion 表示一条推断出的断言。
type InferredAssertion struct {
	Type       string `json:"type"`                 // 断言类型：status_code, jsonpath, header, body_contains, response_time
	Expression string `json:"expression,omitempty"` // 表达式，如 jsonpath 路径
	Operator   string `json:"operator"`             // 操作符：exists, eq, ne, gt, gte, lt, lte, matches 等
	Expected   any    `json:"expected,omitempty"`    // 期望值
	Confidence float64 `json:"confidence"`           // 置信度：0.0 ~ 1.0
	Source     string `json:"source"`                // 推断来源：schema, history, semantic
	Reason     string `json:"reason"`               // 推断理由
}

// InferInput 是断言推断的输入参数。
type InferInput struct {
	CaseID      uuid.UUID `json:"caseId"`                // 测试用例 ID
	SampleCount int       `json:"sampleCount,omitempty"` // 历史样本数量，默认 10
	Sources     []string  `json:"sources,omitempty"`     // 推断来源：schema, history, semantic
}

// InferOutput 是断言推断的输出结果。
type InferOutput struct {
	Assertions []InferredAssertion `json:"assertions"` // 推断出的断言列表
	Summary    InferSummary        `json:"summary"`    // 推断摘要
}

// InferSummary 提供推断过程的统计信息。
type InferSummary struct {
	TotalAssertions int            `json:"totalAssertions"`
	BySource        map[string]int `json:"bySource"`  // 按来源统计
	HistorySamples  int            `json:"historySamples"` // 使用的历史样本数
	HasSchema       bool           `json:"hasSchema"`      // 是否有 OpenAPI schema
}

// EndpointInfo 是从 spec 包获取的接口信息（避免直接依赖 spec 包）。
type EndpointInfo struct {
	ID             uuid.UUID       `json:"id"`
	Method         string          `json:"method"`
	Path           string          `json:"path"`
	OperationID    string          `json:"operationId,omitempty"`
	Summary        string          `json:"summary,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	ResponseSchema json.RawMessage `json:"responseSchema"`
}

// TestCaseInfo 是从 case 包获取的测试用例信息。
type TestCaseInfo struct {
	ID                   uuid.UUID       `json:"id"`
	ProjectID            uuid.UUID       `json:"projectId"`
	ServiceID            uuid.UUID       `json:"serviceId"`
	EndpointID           *uuid.UUID      `json:"endpointId,omitempty"`
	Method               string          `json:"method"`
	Path                 string          `json:"path"`
	Assertions           json.RawMessage `json:"assertions"`
	LastResponseSnapshot json.RawMessage `json:"lastResponseSnapshot,omitempty"`
}

// HistoryResult 表示一次历史运行结果的快照。
type HistoryResult struct {
	ResponseSnapshot json.RawMessage `json:"responseSnapshot"`
	Status           string          `json:"status"`
}

// EndpointGetter 获取接口定义。由 spec.Repository 适配。
type EndpointGetter interface {
	GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*EndpointInfo, error)
}

// CaseGetter 获取测试用例信息。由 case.Repository 适配。
type CaseGetter interface {
	Get(ctx context.Context, id uuid.UUID) (*TestCaseInfo, error)
}

// HistoryCollector 收集历史运行结果。由 report.Repository 适配。
type HistoryCollector interface {
	ListPassedResponsesByTestCase(ctx context.Context, testCaseID uuid.UUID, limit int) ([]HistoryResult, error)
}

// InferEngine 是断言推断引擎。
type InferEngine struct {
	endpoints EndpointGetter
	cases     CaseGetter
	history   HistoryCollector
}

// NewInferEngine 创建断言推断引擎实例。
func NewInferEngine(endpoints EndpointGetter, cases CaseGetter, history HistoryCollector) *InferEngine {
	return &InferEngine{
		endpoints: endpoints,
		cases:     cases,
		history:   history,
	}
}

// InferAssertions 执行三层断言推断策略：
// 1. Schema 规则推断（零 LLM 调用）：从 OpenAPI response schema 自动生成
// 2. 历史响应分析：分析最近通过的响应快照中的字段值模式
// 3. LLM 语义推断：结合 endpoint 元数据推断业务断言
func (e *InferEngine) InferAssertions(ctx context.Context, input InferInput) (*InferOutput, error) {
	if input.CaseID == uuid.Nil {
		return nil, ErrCaseIDRequired
	}

	// 默认样本数
	if input.SampleCount <= 0 {
		input.SampleCount = 10
	}

	// 默认启用所有推断来源
	sources := input.Sources
	if len(sources) == 0 {
		sources = []string{"schema", "history"}
	}
	sourceSet := make(map[string]bool)
	for _, s := range sources {
		sourceSet[s] = true
	}

	// 获取测试用例信息
	tc, err := e.cases.Get(ctx, input.CaseID)
	if err != nil {
		return nil, fmt.Errorf("获取测试用例失败: %w", err)
	}

	var allAssertions []InferredAssertion
	summary := InferSummary{
		BySource: make(map[string]int),
	}

	// 第一层：Schema 规则推断
	if sourceSet["schema"] && tc.EndpointID != nil && *tc.EndpointID != uuid.Nil {
		schemaAssertions, err := e.inferFromSchema(ctx, *tc.EndpointID)
		if err == nil {
			allAssertions = append(allAssertions, schemaAssertions...)
			summary.BySource["schema"] = len(schemaAssertions)
			summary.HasSchema = true
		}
	}

	// 第二层：历史响应分析
	if sourceSet["history"] && e.history != nil {
		historyAssertions, sampleCount, err := e.inferFromHistory(ctx, input.CaseID, input.SampleCount)
		if err == nil {
			allAssertions = append(allAssertions, historyAssertions...)
			summary.BySource["history"] = len(historyAssertions)
			summary.HistorySamples = sampleCount
		}
	}

	// 第三层：LLM 语义推断（预留接口，当前返回空）
	if sourceSet["semantic"] {
		semanticAssertions := e.inferFromSemantic(tc)
		allAssertions = append(allAssertions, semanticAssertions...)
		summary.BySource["semantic"] = len(semanticAssertions)
	}

	// 去重并按置信度排序
	allAssertions = deduplicateInferred(allAssertions)
	sortByConfidence(allAssertions)

	summary.TotalAssertions = len(allAssertions)

	return &InferOutput{
		Assertions: allAssertions,
		Summary:    summary,
	}, nil
}

// inferFromSchema 从 OpenAPI response schema 推断断言。
func (e *InferEngine) inferFromSchema(ctx context.Context, endpointID uuid.UUID) ([]InferredAssertion, error) {
	ep, err := e.endpoints.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}

	if len(ep.ResponseSchema) == 0 {
		return nil, nil
	}

	var assertions []InferredAssertion

	// 推断状态码断言（优先使用 ResponseSchema 中记录的成功状态码）
	statusCode := inferResponseStatus(ep.ResponseSchema)
	assertions = append(assertions, InferredAssertion{
		Type:       "status_code",
		Operator:   "eq",
		Expected:   statusCode,
		Confidence: 0.95,
		Source:     "schema",
		Reason:     fmt.Sprintf("OpenAPI 成功响应状态码为 %d", statusCode),
	})

	// 从 body schema 生成字段断言
	bodySchema := spec.ExtractResponseBodySchema(ep.ResponseSchema)
	schemaAssertions, err := GenerateSchemaAssertions(bodySchema)
	if err != nil {
		return assertions, nil // 已有状态码断言，schema 解析失败不阻断
	}

	for _, sa := range schemaAssertions {
		inferred := schemaAssertionToInferred(sa)
		assertions = append(assertions, inferred)
	}

	return assertions, nil
}

// inferFromHistory 从历史响应快照推断断言。
func (e *InferEngine) inferFromHistory(ctx context.Context, caseID uuid.UUID, sampleCount int) ([]InferredAssertion, int, error) {
	results, err := e.history.ListPassedResponsesByTestCase(ctx, caseID, sampleCount)
	if err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return nil, 0, nil
	}

	// 收集所有响应快照
	responses := make([]json.RawMessage, 0, len(results))
	for _, r := range results {
		if len(r.ResponseSnapshot) > 0 {
			responses = append(responses, r.ResponseSnapshot)
		}
	}
	if len(responses) == 0 {
		return nil, len(results), nil
	}

	// 分析历史响应
	historyRaw, err := AnalyzeHistoryResponses(responses)
	if err != nil {
		return nil, len(results), err
	}

	var assertions []InferredAssertion
	for _, raw := range historyRaw {
		assertions = append(assertions, InferredAssertion{
			Type:       getString(raw, "type"),
			Expression: getString(raw, "path"),
			Operator:   getString(raw, "op"),
			Expected:   raw["expected"],
			Confidence: 0.7,
			Source:     "history",
			Reason:     fmt.Sprintf("在 %d 次历史通过的响应中观察到一致的字段模式", len(responses)),
		})
	}

	return assertions, len(results), nil
}

// inferFromSemantic 基于 endpoint 元数据推断语义断言（当前为简单规则，未来可接入 LLM）。
func (e *InferEngine) inferFromSemantic(tc *TestCaseInfo) []InferredAssertion {
	var assertions []InferredAssertion

	// 基于方法推断响应时间断言
	assertions = append(assertions, InferredAssertion{
		Type:       "response_time",
		Operator:   "lte",
		Expected:   5000,
		Confidence: 0.5,
		Source:     "semantic",
		Reason:     "API 响应时间应在合理范围内（5 秒以内）",
	})

	return assertions
}

// schemaAssertionToInferred 将 schema 断言转换为 InferredAssertion。
func schemaAssertionToInferred(raw map[string]any) InferredAssertion {
	assertion := InferredAssertion{
		Type:       getString(raw, "type"),
		Expression: getString(raw, "path"),
		Operator:   getString(raw, "op"),
		Expected:   raw["expected"],
		Source:     "schema",
	}

	// 根据断言类型设置置信度和理由
	switch assertion.Type {
	case "jsonpath":
		switch assertion.Operator {
		case "exists":
			assertion.Confidence = 0.9
			assertion.Reason = "字段在 OpenAPI schema 中标记为 required"
		case "is_not_null":
			assertion.Confidence = 0.85
			assertion.Reason = "字段在 OpenAPI schema 中定义且必填"
		case "eq":
			assertion.Confidence = 0.85
			assertion.Reason = "字段在 OpenAPI schema 中定义了枚举值"
		case "gte", "lte":
			assertion.Confidence = 0.8
			assertion.Reason = "字段在 OpenAPI schema 中定义了数值范围"
		case "matches":
			assertion.Confidence = 0.8
			assertion.Reason = fmt.Sprintf("字段格式为 %s，使用正则校验", getString(raw, "format"))
		case "is_not_empty":
			assertion.Confidence = 0.8
			assertion.Reason = "数组类型字段应非空"
		default:
			assertion.Confidence = 0.7
			assertion.Reason = "基于 OpenAPI schema 规则推断"
		}
	default:
		assertion.Confidence = 0.7
		assertion.Reason = "基于 OpenAPI schema 规则推断"
	}

	return assertion
}

// deduplicateInferred 对推断断言去重，保留置信度最高的。
func deduplicateInferred(assertions []InferredAssertion) []InferredAssertion {
	best := make(map[string]InferredAssertion)
	for _, a := range assertions {
		key := strings.Join([]string{a.Type, a.Expression, a.Operator}, "|")
		if existing, ok := best[key]; !ok || a.Confidence > existing.Confidence {
			best[key] = a
		}
	}
	result := make([]InferredAssertion, 0, len(best))
	for _, a := range best {
		result = append(result, a)
	}
	return result
}

// sortByConfidence 按置信度降序排序。
func sortByConfidence(assertions []InferredAssertion) {
	for i := 1; i < len(assertions); i++ {
		for j := i; j > 0 && assertions[j].Confidence > assertions[j-1].Confidence; j-- {
			assertions[j], assertions[j-1] = assertions[j-1], assertions[j]
		}
	}
}

// getString 安全地从 map 中获取字符串值。
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// ToAssertionJSON 将推断断言列表转换为 internal/assertion 包兼容的 JSON 数组。
func ToAssertionJSON(assertions []InferredAssertion) (json.RawMessage, error) {
	result := make([]map[string]any, 0, len(assertions))
	for _, a := range assertions {
		item := map[string]any{
			"type": a.Type,
		}
		if a.Expression != "" {
			item["path"] = a.Expression
		}
		if a.Operator != "" {
			item["op"] = a.Operator
		}
		if a.Expected != nil {
			item["expected"] = a.Expected
		}
		// status_code 断言使用 expected 字段
		if a.Type == "status_code" {
			item["expected"] = a.Expected
		}
		result = append(result, item)
	}
	return json.Marshal(result)
}

// ApplyAssertionsInput 是批量应用断言的输入。
type ApplyAssertionsInput struct {
	Assertions    []InferredAssertion `json:"assertions"`    // 要应用的断言列表
	ReplaceMode   bool                `json:"replaceMode"`  // true=替换现有断言，false=追加
}

// MergeWithExisting 将推断的断言与现有断言合并。
func MergeWithExisting(existing json.RawMessage, inferred []InferredAssertion, replaceMode bool) (json.RawMessage, error) {
	if replaceMode {
		return ToAssertionJSON(inferred)
	}

	// 解析现有断言
	var existingList []map[string]any
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &existingList); err != nil {
			return nil, fmt.Errorf("解析现有断言失败: %w", err)
		}
	}

	// 将推断断言转换为 map 格式
	inferredJSON, err := ToAssertionJSON(inferred)
	if err != nil {
		return nil, err
	}
	var inferredList []map[string]any
	if err := json.Unmarshal(inferredJSON, &inferredList); err != nil {
		return nil, err
	}

	// 合并并去重（基于 type+path+op）
	seen := make(map[string]bool)
	var merged []map[string]any

	for _, a := range existingList {
		key := assertionMapKey(a)
		seen[key] = true
		merged = append(merged, a)
	}
	for _, a := range inferredList {
		key := assertionMapKey(a)
		if !seen[key] {
			merged = append(merged, a)
		}
	}

	return json.Marshal(merged)
}

// assertionMapKey 生成 map 断言的唯一标识。
func assertionMapKey(a map[string]any) string {
	typ, _ := a["type"].(string)
	path, _ := a["path"].(string)
	op, _ := a["op"].(string)
	name, _ := a["name"].(string)
	return strings.Join([]string{typ, path, op, name}, "|")
}
