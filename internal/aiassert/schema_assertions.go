// Package aiassert 提供智能断言推断引擎，从 OpenAPI schema、历史响应和 LLM
// 语义分析三个层次自动生成测试断言。
package aiassert

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// schemaField 描述从 OpenAPI response schema 中提取的单个字段信息。
type schemaField struct {
	Path       string         // JSONPath 表达式，如 $.data.id
	Name       string         // 字段名
	Type       string         // OpenAPI 类型：string, integer, number, boolean, array, object
	Required   bool           // 是否为 required 字段
	Enum       []any          // 枚举值列表
	Minimum    *float64       // 数值最小值
	Maximum    *float64       // 数值最大值
	MinLength  *int           // 字符串最小长度
	MaxLength  *int           // 字符串最大长度
	Format     string         // 字符串格式：email, uri, uuid, date, date-time 等
	Items      map[string]any // 数组元素 schema
	Properties map[string]any // 对象属性 schema
}

// emailPattern 匹配基本的 email 格式。
var emailPattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`

// uriPattern 匹配 URI 格式。
var uriPattern = `^https?://[^\s/$.?#].[^\s]*$`

// uuidPattern 匹配 UUID v4 格式。
var uuidPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`

// datePattern 匹配 ISO 8601 日期格式。
var datePattern = `^\d{4}-\d{2}-\d{2}$`

// dateTimePattern 匹配 ISO 8601 日期时间格式。
var dateTimePattern = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`

// GenerateSchemaAssertions 从 OpenAPI response schema 中提取字段信息，自动生成断言列表。
// 返回的断言与 internal/assertion 包的 JSON 格式对齐，可直接序列化为 json.RawMessage。
func GenerateSchemaAssertions(responseSchema json.RawMessage) ([]map[string]any, error) {
	if len(responseSchema) == 0 {
		return nil, nil
	}

	var schema map[string]any
	if err := json.Unmarshal(responseSchema, &schema); err != nil {
		return nil, fmt.Errorf("解析 response schema 失败: %w", err)
	}

	// 收集 required 字段集合（顶层）
	requiredSet := extractRequiredSet(schema)

	// 遍历 properties 生成断言
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		return nil, nil
	}

	var assertions []map[string]any
	for fieldName, fieldDef := range properties {
		fieldSchema, ok := fieldDef.(map[string]any)
		if !ok {
			continue
		}
		isRequired := requiredSet[fieldName]
		path := "$." + fieldName
		field := parseSchemaField(path, fieldName, fieldSchema, isRequired)
		assertions = append(assertions, generateFieldAssertions(field)...)
	}

	// 递归处理嵌套对象的 required 字段
	assertions = append(assertions, generateNestedRequiredAssertions(schema, "$")...)

	return assertions, nil
}

// extractRequiredSet 从 schema 中提取 required 字段名集合。
func extractRequiredSet(schema map[string]any) map[string]bool {
	set := make(map[string]bool)
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if name, ok := r.(string); ok {
				set[name] = true
			}
		}
	}
	return set
}

// parseSchemaField 从 OpenAPI 属性定义中解析字段信息。
func parseSchemaField(path, name string, def map[string]any, required bool) schemaField {
	field := schemaField{
		Path:     path,
		Name:     name,
		Required: required,
	}

	if t, ok := def["type"].(string); ok {
		field.Type = t
	}
	if f, ok := def["format"].(string); ok {
		field.Format = f
	}
	if enum, ok := def["enum"].([]any); ok {
		field.Enum = enum
	}
	if min, ok := def["minimum"]; ok {
		if v, ok := toFloat(min); ok {
			field.Minimum = &v
		}
	}
	if max, ok := def["maximum"]; ok {
		if v, ok := toFloat(max); ok {
			field.Maximum = &v
		}
	}
	if minLen, ok := def["minLength"]; ok {
		if v, ok := toInt(minLen); ok {
			field.MinLength = &v
		}
	}
	if maxLen, ok := def["maxLength"]; ok {
		if v, ok := toInt(maxLen); ok {
			field.MaxLength = &v
		}
	}
	if items, ok := def["items"].(map[string]any); ok {
		field.Items = items
	}
	if props, ok := def["properties"].(map[string]any); ok {
		field.Properties = props
	}

	return field
}

// generateFieldAssertions 根据字段信息生成断言列表。
func generateFieldAssertions(field schemaField) []map[string]any {
	var assertions []map[string]any

	// 1. 必填字段 → exists 断言
	if field.Required {
		assertions = append(assertions, map[string]any{
			"type": "jsonpath",
			"path": field.Path,
			"op":   "exists",
		})
	}

	// 2. 根据字段类型生成断言
	switch field.Type {
	case "string":
		assertions = append(assertions, generateStringAssertions(field)...)
	case "integer", "number":
		assertions = append(assertions, generateNumberAssertions(field)...)
	case "boolean":
		// boolean 类型检查
		if field.Required {
			assertions = append(assertions, map[string]any{
				"type": "jsonpath",
				"path": field.Path,
				"op":   "is_not_null",
			})
		}
	case "array":
		// 数组类型检查
		if field.Required {
			assertions = append(assertions, map[string]any{
				"type": "jsonpath",
				"path": field.Path,
				"op":   "is_not_empty",
			})
		}
	}

	// 3. 枚举值 → eq 断言（仅当只有一个枚举值时自动精确匹配）
	if len(field.Enum) == 1 {
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "eq",
			"expected": field.Enum[0],
		})
	} else if len(field.Enum) > 1 {
		// 多个枚举值 → matches 正则（断言引擎不支持 oneOf）
		if pattern := enumRegexPattern(field.Enum); pattern != "" {
			assertions = append(assertions, map[string]any{
				"type":     "jsonpath",
				"path":     field.Path,
				"op":       "matches",
				"expected": pattern,
			})
		}
	}

	return assertions
}

// generateStringAssertions 为字符串字段生成断言。
func generateStringAssertions(field schemaField) []map[string]any {
	var assertions []map[string]any

	// 字符串格式 → matches 正则断言
	switch field.Format {
	case "email":
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "matches",
			"expected": emailPattern,
		})
	case "uri", "url":
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "matches",
			"expected": uriPattern,
		})
	case "uuid":
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "matches",
			"expected": uuidPattern,
		})
	case "date":
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "matches",
			"expected": datePattern,
		})
	case "date-time":
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "matches",
			"expected": dateTimePattern,
		})
	}

	return assertions
}

// generateNumberAssertions 为数值字段生成断言。
func generateNumberAssertions(field schemaField) []map[string]any {
	var assertions []map[string]any

	if field.Minimum != nil {
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "gte",
			"expected": *field.Minimum,
		})
	}
	if field.Maximum != nil {
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     field.Path,
			"op":       "lte",
			"expected": *field.Maximum,
		})
	}

	return assertions
}

// generateNestedRequiredAssertions 递归遍历嵌套对象，为 required 字段生成 exists 断言。
func generateNestedRequiredAssertions(schema map[string]any, parentPath string) []map[string]any {
	var assertions []map[string]any

	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		return nil
	}

	requiredSet := extractRequiredSet(schema)

	for fieldName, fieldDef := range properties {
		fieldSchema, ok := fieldDef.(map[string]any)
		if !ok {
			continue
		}

		childPath := parentPath + "." + fieldName
		isRequired := requiredSet[fieldName]

		// 必填字段生成 exists 断言
		if isRequired {
			assertions = append(assertions, map[string]any{
				"type": "jsonpath",
				"path": childPath,
				"op":   "exists",
			})
		}

		// 递归处理嵌套对象
		childType, _ := fieldSchema["type"].(string)
		if childType == "object" || fieldSchema["properties"] != nil {
			assertions = append(assertions, generateNestedRequiredAssertions(fieldSchema, childPath)...)
		}

		// 数组元素如果是对象，也递归处理
		if childType == "array" {
			if items, ok := fieldSchema["items"].(map[string]any); ok {
				itemType, _ := items["type"].(string)
				if itemType == "object" || items["properties"] != nil {
					// 对于数组元素，使用 $[*] 通配符
					assertions = append(assertions, generateNestedRequiredAssertions(items, childPath+"[*]")...)
				}
			}
		}
	}

	return assertions
}

// toFloat 将 any 转换为 float64。
func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// toInt 将 any 转换为 int。
func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// AnalyzeHistoryResponses 分析最近 N 次通过的响应快照，提取字段值模式。
// 返回基于历史模式推断的断言列表。
func AnalyzeHistoryResponses(responses []json.RawMessage) ([]map[string]any, error) {
	if len(responses) == 0 {
		return nil, nil
	}

	// 收集所有响应中的字段路径和值
	fieldValues := make(map[string][]any)         // path -> values
	fieldTypes := make(map[string]map[string]int) // path -> type -> count

	for _, resp := range responses {
		var doc any
		if err := json.Unmarshal(resp, &doc); err != nil {
			continue
		}
		collectFieldPatterns(doc, "$", fieldValues, fieldTypes)
	}

	var assertions []map[string]any

	// 分析字段值模式
	for path, values := range fieldValues {
		if len(values) == 0 {
			continue
		}

		// 如果所有样本中该字段都存在且非空，生成 exists 断言
		if len(values) == len(responses) {
			// 检查是否所有值都非空
			allNonEmpty := true
			for _, v := range values {
				if v == nil {
					allNonEmpty = false
					break
				}
			}
			if allNonEmpty {
				assertions = append(assertions, map[string]any{
					"type": "jsonpath",
					"path": path,
					"op":   "exists",
				})
			}
		}

		// 如果所有样本中该字段值都相同，生成 eq 断言（跳过动态字段与复合类型）
		if len(values) >= 2 && shouldInferEqAssertion(path, values[0]) {
			first := values[0]
			allSame := true
			for _, v := range values[1:] {
				if !jsonDeepEqual(first, v) {
					allSame = false
					break
				}
			}
			if allSame && first != nil {
				assertions = append(assertions, map[string]any{
					"type":     "jsonpath",
					"path":     path,
					"op":       "eq",
					"expected": first,
				})
			}
		}

		// 分析数值范围
		if types, ok := fieldTypes[path]; ok {
			if types["number"] > 0 || types["integer"] > 0 {
				minVal, maxVal := analyzeNumericRange(values)
				if minVal != nil {
					assertions = append(assertions, map[string]any{
						"type":     "jsonpath",
						"path":     path,
						"op":       "gte",
						"expected": *minVal,
					})
				}
				if maxVal != nil {
					assertions = append(assertions, map[string]any{
						"type":     "jsonpath",
						"path":     path,
						"op":       "lte",
						"expected": *maxVal,
					})
				}
			}
		}
	}

	return assertions, nil
}

// collectFieldPatterns 递归收集字段路径和值。
func collectFieldPatterns(doc any, path string, fieldValues map[string][]any, fieldTypes map[string]map[string]int) {
	switch v := doc.(type) {
	case map[string]any:
		for key, val := range v {
			childPath := path + "." + key
			typeName := jsonTypeOf(val)
			if fieldTypes[childPath] == nil {
				fieldTypes[childPath] = make(map[string]int)
			}
			fieldTypes[childPath][typeName]++
			// 仅记录标量值，避免对整段 object/array 生成 eq 断言
			switch val.(type) {
			case map[string]any, []any:
				// skip storing composite value
			default:
				fieldValues[childPath] = append(fieldValues[childPath], val)
			}
			collectFieldPatterns(val, childPath, fieldValues, fieldTypes)
		}
	case []any:
		if len(v) == 0 {
			return
		}
		childPath := path + "[*]"
		collectFieldPatterns(v[0], childPath, fieldValues, fieldTypes)
	}
}

// jsonTypeOf 返回值的 JSON 类型名。
func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// analyzeNumericRange 分析数值列表，返回最小值和最大值。
func analyzeNumericRange(values []any) (*float64, *float64) {
	var nums []float64
	for _, v := range values {
		if f, ok := toFloat(v); ok {
			nums = append(nums, f)
		}
	}
	if len(nums) == 0 {
		return nil, nil
	}
	min, max := nums[0], nums[0]
	for _, n := range nums[1:] {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	return &min, &max
}

// jsonDeepEqual 通过 JSON 序列化比较两个值是否深度相等。
func jsonDeepEqual(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ja) == string(jb)
}

// DeduplicateAssertions 对断言列表去重，基于 type+path+op 组合。
func DeduplicateAssertions(assertions []map[string]any) []map[string]any {
	seen := make(map[string]bool)
	var result []map[string]any
	for _, a := range assertions {
		key := assertionKey(a)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, a)
	}
	return result
}

// assertionKey 生成断言的唯一标识。
func assertionKey(a map[string]any) string {
	typ, _ := a["type"].(string)
	path, _ := a["path"].(string)
	op, _ := a["op"].(string)
	name, _ := a["name"].(string)
	return strings.Join([]string{typ, path, op, name}, "|")
}

// isLikelyIdentifier 判断字段名是否像标识符字段（id, uuid 等）。
func isLikelyIdentifier(name string) bool {
	lower := strings.ToLower(name)
	identifiers := []string{"id", "uuid", "guid", "key", "token", "code"}
	for _, id := range identifiers {
		if lower == id || strings.HasSuffix(lower, "_"+id) || strings.HasSuffix(lower, "Id") || strings.HasSuffix(lower, "ID") {
			return true
		}
	}
	return false
}

// isLikelyTimestampField 判断字段名是否像时间戳字段。
func isLikelyTimestampField(name string) bool {
	lower := strings.ToLower(name)
	timestamps := []string{
		"created_at", "updated_at", "deleted_at", "createdat", "updatedat",
		"created", "updated", "deleted", "timestamp", "time", "date",
	}
	for _, ts := range timestamps {
		if lower == ts || strings.HasSuffix(lower, "_"+ts) || strings.HasSuffix(lower, ts) {
			return true
		}
	}
	return false
}

// inferResponseStatus 从 Endpoint.ResponseSchema 提取成功状态码，无法解析时回退到 200。
func inferResponseStatus(responseSchema json.RawMessage) int {
	var wrapper struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(responseSchema, &wrapper); err == nil && wrapper.Status != "" {
		if code, err := strconv.Atoi(wrapper.Status); err == nil && code >= 100 && code < 600 {
			return code
		}
	}
	return 200
}

// enumRegexPattern 将枚举值列表转为 matches 可用的正则。
func enumRegexPattern(values []any) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		s := fmt.Sprintf("%v", v)
		if s == "" {
			continue
		}
		parts = append(parts, regexp.QuoteMeta(s))
	}
	if len(parts) == 0 {
		return ""
	}
	return "^(" + strings.Join(parts, "|") + ")$"
}

// shouldInferEqAssertion 判断历史样本是否适合生成 eq 断言。
func shouldInferEqAssertion(path string, sample any) bool {
	switch sample.(type) {
	case map[string]any, []any:
		return false
	}
	leaf := pathLeaf(path)
	if isLikelyIdentifier(leaf) || isLikelyTimestampField(leaf) {
		return false
	}
	return !strings.Contains(path, "[")
}

// pathLeaf 从 JSONPath 提取字段名。
func pathLeaf(path string) string {
	path = strings.TrimPrefix(path, "$.")
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		path = path[idx+1:]
	}
	if idx := strings.Index(path, "["); idx >= 0 {
		path = path[:idx]
	}
	return path
}
