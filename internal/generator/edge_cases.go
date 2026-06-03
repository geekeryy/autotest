package generator

import (
	"encoding/json"
	"math"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/sampler"
)

const (
	RuleEdgeCases = "edge_cases"
)

// EdgeCasesRule 对每个字段生成边界值测试用例，验证服务端对极端输入的处理能力。
type EdgeCasesRule struct{}

func (EdgeCasesRule) ID() string {
	return RuleEdgeCases
}

func (r EdgeCasesRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	properties, _ := extractBodyProperties(endpoint.RequestSchema)
	if len(properties) == 0 {
		return nil, nil
	}

	fullSample := sampler.FromSchema(endpoint.RequestSchema)
	bodyMap, _ := fullSample.Body.(map[string]any)
	if bodyMap == nil {
		return nil, nil
	}

	assertions := []map[string]any{
		{"type": "status_code", "expected": defaultExpectedErrorStatus()},
	}
	rawAssertions, err := json.Marshal(assertions)
	if err != nil {
		return nil, err
	}

	var drafts []testcase.Draft
	for field, propSchema := range properties {
		schemaType, _ := propSchema["type"].(string)
		if schemaType == "" {
			continue
		}
		edgeValues := edgeCaseValues(field, schemaType, propSchema)
		for _, ev := range edgeValues {
			mutated := cloneMap(bodyMap)
			mutated[field] = ev.value

			request := map[string]any{
				"method":    strings.ToUpper(endpoint.Method),
				"path":      endpoint.Path,
				"headers":   fullSample.Headers,
				"query":     fullSample.Query,
				"variables": fullSample.Path,
				"body":      mutated,
			}
			if fullSample.Security != nil {
				request["security"] = fullSample.Security
			}

			rawRequest, err := json.Marshal(request)
			if err != nil {
				return nil, err
			}

			endpointID := endpoint.ID
			draft := testcase.Draft{
				ProjectID:        endpoint.ProjectID,
				ServiceID:        endpoint.ServiceID,
				EndpointID:       &endpointID,
				Source:           testcase.SourceAuto,
				Name:             caseName(endpoint) + " [边界值: " + field + " " + ev.label + "]",
				Method:           endpoint.Method,
				Path:             endpoint.Path,
				GenerationRuleID: RuleEdgeCases,
				Request:          rawRequest,
				Assertions:       rawAssertions,
				Status:           testcase.StatusActive,
			}
			draft.Fingerprint = buildFingerprint(endpoint, RuleEdgeCases+":"+field+":"+ev.label)
			drafts = append(drafts, draft)
		}
	}
	return drafts, nil
}

// edgeCase 表示一个边界值测试点。
type edgeCase struct {
	value any
	label string
}

// edgeCaseValues 根据字段类型和 schema 生成边界值列表。
func edgeCaseValues(fieldName, schemaType string, schema map[string]any) []edgeCase {
	switch schemaType {
	case "string":
		return stringEdgeCases(fieldName, schema)
	case "integer":
		return integerEdgeCases()
	case "number":
		return numberEdgeCases()
	case "array":
		return arrayEdgeCases(fieldName, schema)
	case "object":
		return objectEdgeCases()
	default:
		return nil
	}
}

// stringEdgeCases 生成字符串边界值：空字符串、超长字符串、特殊字符、SQL 注入、XSS。
func stringEdgeCases(fieldName string, schema map[string]any) []edgeCase {
	cases := []edgeCase{
		{value: "", label: "空字符串"},
		{value: strings.Repeat("A", 10000), label: "超长字符串(10000字符)"},
		{value: "!@#$%^&*()_+-=[]{}|;':\",./<>?`~", label: "特殊字符"},
		{value: "' OR '1'='1' --", label: "SQL注入"},
		{value: "Robert'); DROP TABLE Students;--", label: "SQL注入(DELETE)"},
		{value: "<script>alert('xss')</script>", label: "XSS脚本"},
		{value: "<img src=x onerror=alert(1)>", label: "XSS图片标签"},
		{value: "\x00\x01\x02\x03", label: "控制字符"},
		{value: "日本語テスト", label: "Unicode非ASCII"},
		{value: "🔥💀🎉", label: "Emoji"},
	}
	_ = schema
	_ = fieldName
	return cases
}

// integerEdgeCases 生成整数边界值。
func integerEdgeCases() []edgeCase {
	return []edgeCase{
		{value: 0, label: "零"},
		{value: -1, label: "负一"},
		{value: math.MaxInt64, label: "最大整数"},
		{value: math.MinInt64, label: "最小整数"},
	}
}

// numberEdgeCases 生成浮点数边界值。
func numberEdgeCases() []edgeCase {
	return []edgeCase{
		{value: 0.0, label: "零"},
		{value: -0.0, label: "负零"},
		{value: math.SmallestNonzeroFloat64, label: "极小正浮点数"},
		{value: math.MaxFloat64, label: "最大浮点数"},
		{value: math.NaN(), label: "NaN"},
		{value: math.Inf(1), label: "正无穷"},
		{value: math.Inf(-1), label: "负无穷"},
	}
}

// arrayEdgeCases 生成数组边界值。
func arrayEdgeCases(fieldName string, schema map[string]any) []edgeCase {
	// 空数组
	cases := []edgeCase{
		{value: []any{}, label: "空数组"},
	}

	// 超大数组：100 个元素
	itemSchema, _ := schema["items"].(map[string]any)
	largeArr := make([]any, 100)
	if itemSchema != nil {
		for i := range largeArr {
			largeArr[i] = sampleItemForArray(fieldName, itemSchema)
		}
	} else {
		for i := range largeArr {
			largeArr[i] = "item"
		}
	}
	cases = append(cases, edgeCase{value: largeArr, label: "超大数组(100元素)"})
	return cases
}

// objectEdgeCases 生成对象边界值。
func objectEdgeCases() []edgeCase {
	return []edgeCase{
		{value: map[string]any{}, label: "空对象"},
		{value: map[string]any{"unknown_field_xyz": "unexpected", "extra_123": 42}, label: "额外未知字段"},
	}
}

// sampleItemForArray 为数组元素生成一个简单的采样值。
func sampleItemForArray(fieldName string, schema map[string]any) any {
	schemaType, _ := schema["type"].(string)
	switch schemaType {
	case "string":
		return "item"
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	case "object":
		return map[string]any{}
	default:
		return "item"
	}
}
