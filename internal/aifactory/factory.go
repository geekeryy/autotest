package aifactory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Scenario 模式常量。
const (
	ScenarioNormal  = "normal"  // 正常场景
	ScenarioBoundary = "boundary" // 边界场景
	ScenarioStress  = "stress"  // 压力场景
)

// GenerateInput 是数据工厂的输入参数。
type GenerateInput struct {
	EndpointID  uuid.UUID      `json:"endpointId"`  // 关联的接口 ID
	Scenario    string         `json:"scenario"`     // normal, boundary, stress
	Count       int            `json:"count"`        // 生成行数
	Locale      string         `json:"locale"`       // zh_CN, en_US
	Constraints map[string]any `json:"constraints"`  // 可选的额外约束（字段级覆盖）
}

// GenerateOutput 是数据工厂的输出。
type GenerateOutput struct {
	Rows     []map[string]any `json:"rows"`     // 生成的数据行
	Metadata GenerateMetadata  `json:"metadata"` // 生成元信息
}

// GenerateMetadata 记录本次生成的元信息。
type GenerateMetadata struct {
	TotalCount  int       `json:"totalCount"`
	GeneratedAt time.Time `json:"generatedAt"`
	Locale      string    `json:"locale"`
	Scenario    string    `json:"scenario"`
	EndpointID  uuid.UUID `json:"endpointId"`
}

// Factory 是语义感知的测试数据工厂。
type Factory struct{}

// NewFactory 创建一个新的数据工厂实例。
func NewFactory() *Factory {
	return &Factory{}
}

// Generate 根据 OpenAPI RequestSchema 和输入参数生成测试数据。
// requestSchema 是 Endpoint.RequestSchema 的 JSON 原文，
// 格式为 {"parameters": [...], "body": {...}} 结构。
func (f *Factory) Generate(requestSchema json.RawMessage, input GenerateInput) (*GenerateOutput, error) {
	if input.Count <= 0 {
		input.Count = 10
	}
	if input.Count > 1000 {
		input.Count = 1000
	}

	locale := NormalizeLocale(input.Locale)
	scenario := input.Scenario
	if scenario == "" {
		scenario = ScenarioNormal
	}

	// 解析 RequestSchema 提取字段定义
	fields := parseRequestSchema(requestSchema)
	if len(fields) == 0 {
		return &GenerateOutput{
			Rows:     []map[string]any{},
			Metadata: newMetadata(0, locale, scenario, input.EndpointID),
		}, nil
	}

	// 逐行生成数据
	rows := make([]map[string]any, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		row := make(map[string]any)
		for _, field := range fields {
			fieldName := field.leafName()
			sem := InferSemantics(fieldName, field.Schema, field.Required)

			if constraintVal, ok := input.constraints().get(fieldName); ok {
				setNestedValue(row, field.Path, constraintVal)
				continue
			}
			if constraintVal, ok := input.constraints().get(joinPath(field.Path)); ok {
				setNestedValue(row, field.Path, constraintVal)
				continue
			}

			setNestedValue(row, field.Path, GenerateValue(sem, locale, scenario))
		}
		rows = append(rows, row)
	}

	return &GenerateOutput{
		Rows:     rows,
		Metadata: newMetadata(len(rows), locale, scenario, input.EndpointID),
	}, nil
}

// GenerateFromColumns 根据列定义直接生成数据（用于 testdata 表场景）。
// columns 是字段名到语义类型的映射。
func (f *Factory) GenerateFromColumns(columns []ColumnInput, input GenerateInput) (*GenerateOutput, error) {
	if input.Count <= 0 {
		input.Count = 10
	}
	if input.Count > 1000 {
		input.Count = 1000
	}

	locale := NormalizeLocale(input.Locale)
	scenario := input.Scenario
	if scenario == "" {
		scenario = ScenarioNormal
	}

	rows := make([]map[string]any, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		row := make(map[string]any, len(columns))
		for _, col := range columns {
			sem := FieldSemantics{
				FieldName: col.Name,
				Type:      col.SemanticType,
				Required:  true,
			}

			if constraintVal, ok := input.constraints().get(col.Name); ok {
				row[col.Name] = constraintVal
				continue
			}

			row[col.Name] = GenerateValue(sem, locale, scenario)
		}
		rows = append(rows, row)
	}

	return &GenerateOutput{
		Rows:     rows,
		Metadata: newMetadata(len(rows), locale, scenario, input.EndpointID),
	}, nil
}

// ColumnInput 用于 GenerateFromColumns，描述一个需要生成的列。
type ColumnInput struct {
	Name         string       // 字段名
	SemanticType SemanticType // 语义类型
}

// ── RequestSchema 解析 ─────────────────────────────────────────────────

// schemaField 描述从 RequestSchema 中解析出的一个字段。
type schemaField struct {
	Path     []string       // 嵌套路径，如 ["address", "city"]
	Schema   map[string]any // 字段的 schema 定义
	Required bool           // 是否必填
}

// parseRequestSchema 从 Endpoint.RequestSchema JSON 中提取所有可生成的字段。
// RequestSchema 格式: {"parameters": [...], "body": {...}}
func parseRequestSchema(raw json.RawMessage) []schemaField {
	if len(raw) == 0 {
		return nil
	}

	var schema struct {
		Parameters []map[string]any `json:"parameters"`
		Body       map[string]any   `json:"body"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}

	var fields []schemaField

	// 解析 parameters
	for _, param := range schema.Parameters {
		name, _ := param["name"].(string)
		if name == "" {
			continue
		}
		required, _ := param["required"].(bool)
		paramSchema, _ := param["schema"].(map[string]any)
		if paramSchema == nil {
			paramSchema = make(map[string]any)
		}
		// 继承 parameter 级别的 description
		if desc, ok := param["description"].(string); ok {
			if _, exists := paramSchema["description"]; !exists {
				paramSchema["description"] = desc
			}
		}
		fields = append(fields, schemaField{
			Path:     []string{name},
			Schema:   paramSchema,
			Required: required,
		})
	}

	// 解析 body 中的 properties
	bodyFields := extractProperties(schema.Body, nil)
	fields = append(fields, bodyFields...)

	return fields
}

// extractProperties 递归提取 schema 中的 properties 字段。
// requiredSet 记录当前层级的必填字段名。
func extractProperties(schema map[string]any, requiredSet map[string]bool) []schemaField {
	if schema == nil {
		return nil
	}

	// 构建 required 集合
	if requiredSet == nil {
		requiredSet = make(map[string]bool)
		if reqArr, ok := schema["required"].([]any); ok {
			for _, r := range reqArr {
				if s, ok := r.(string); ok {
					requiredSet[s] = true
				}
			}
		}
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	var fields []schemaField
	for propName, propRaw := range props {
		propSchema, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}

		// 跳过嵌套 object 和 array（只生成扁平字段）
		propType, _ := propSchema["type"].(string)
		if propType == "object" {
			nested := extractProperties(propSchema, nil)
			for _, nf := range nested {
				nf.Path = append([]string{propName}, nf.Path...)
				fields = append(fields, nf)
			}
			continue
		}
		if propType == "array" {
			// 跳过数组类型
			continue
		}

		fields = append(fields, schemaField{
			Path:     []string{propName},
			Schema:   propSchema,
			Required: requiredSet[propName],
		})
	}

	return fields
}

// ── 辅助函数 ───────────────────────────────────────────────────────────

func newMetadata(count int, locale Locale, scenario string, endpointID uuid.UUID) GenerateMetadata {
	return GenerateMetadata{
		TotalCount:  count,
		GeneratedAt: time.Now().UTC(),
		Locale:      string(locale),
		Scenario:    scenario,
		EndpointID:  endpointID,
	}
}

// constraintsHelper 为 Constraints map 提供类型安全的访问。
type constraintsHelper map[string]any

// get 从 constraints 中获取指定字段的值。
func (c constraintsHelper) get(fieldName string) (any, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c[fieldName]
	return v, ok
}

// constraints 返回 input.Constraints 的 helper 包装。
func (g GenerateInput) constraints() constraintsHelper {
	return constraintsHelper(g.Constraints)
}

func (f schemaField) leafName() string {
	if len(f.Path) == 0 {
		return ""
	}
	return f.Path[len(f.Path)-1]
}

func joinPath(path []string) string {
	return strings.Join(path, ".")
}

// setNestedValue 按路径写入嵌套 map。
func setNestedValue(root map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		root[path[0]] = value
		return
	}
	key := path[0]
	next, ok := root[key].(map[string]any)
	if !ok {
		next = make(map[string]any)
		root[key] = next
	}
	setNestedValue(next, path[1:], value)
}

// Validate 校验 GenerateInput 参数合法性。
func (g GenerateInput) Validate() error {
	if g.EndpointID == uuid.Nil {
		return fmt.Errorf("endpointId 不能为空")
	}
	switch g.Scenario {
	case ScenarioNormal, ScenarioBoundary, ScenarioStress, "":
	default:
		return fmt.Errorf("scenario 不合法: %s，支持 normal/boundary/stress", g.Scenario)
	}
	if g.Count < 0 {
		return fmt.Errorf("count 不能为负数")
	}
	return nil
}
