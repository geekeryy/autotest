package generator

import (
	"encoding/json"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/sampler"
	"autotest/internal/testprofile"
)

// defaultExpectedErrorStatus 返回用于验证请求错误的默认 HTTP 状态码。
func defaultExpectedErrorStatus() int {
	return 400
}

const (
	RuleHappyPath       = "happy_path"
	RuleRequiredMissing = "required_missing"
	RuleTypeMismatch    = "type_mismatch"
)

type HappyPathRule struct{}

func (HappyPathRule) ID() string {
	return RuleHappyPath
}

func (r HappyPathRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	return r.generate(endpoint, nil)
}

func (r HappyPathRule) GenerateWithProfile(endpoint Endpoint, profile *testprofile.Profile) ([]testcase.Draft, error) {
	return r.generate(endpoint, profile)
}

func (HappyPathRule) generate(endpoint Endpoint, profile *testprofile.Profile) ([]testcase.Draft, error) {
	var opts sampler.Options
	if profile != nil && len(profile.FieldProfiles) > 0 {
		opts.Profile = sampler.NewFieldProfileAdapter(endpoint.Method, endpoint.Path, profile.FieldProfiles)
	}
	sample := sampler.FromSchemaWithOptions(endpoint.RequestSchema, opts)
	request := map[string]any{
		"method":    strings.ToUpper(endpoint.Method),
		"path":      endpoint.Path,
		"headers":   sample.Headers,
		"query":     sample.Query,
		"variables": sample.Path,
		"body":      sample.Body,
	}
	if sample.Security != nil {
		request["security"] = sample.Security
	}

	expectedStatus := defaultExpectedStatus(endpoint.ResponseSchema)
	var assertions []map[string]any

	if profile != nil && profile.ResponseConvention != nil {
		assertions = testprofile.BuildConventionAssertions(profile.ResponseConvention, expectedStatus)
	} else {
		assertions = []map[string]any{
			{"type": "status_code", "expected": expectedStatus},
		}
	}

	rawRequest, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	rawAssertions, err := json.Marshal(assertions)
	if err != nil {
		return nil, err
	}

	endpointID := endpoint.ID
	draft := testcase.Draft{
		ProjectID:        endpoint.ProjectID,
		ServiceID:        endpoint.ServiceID,
		EndpointID:       &endpointID,
		Source:           testcase.SourceAuto,
		Name:             caseName(endpoint),
		Method:           endpoint.Method,
		Path:             endpoint.Path,
		GenerationRuleID: RuleHappyPath,
		Request:          rawRequest,
		Assertions:       rawAssertions,
		Status:           testcase.StatusActive,
	}
	draft.Fingerprint = buildFingerprint(endpoint, RuleHappyPath)
	return []testcase.Draft{draft}, nil
}

type RequiredMissingRule struct{}

func (RequiredMissingRule) ID() string {
	return RuleRequiredMissing
}

func (r RequiredMissingRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	requiredFields := extractRequiredBodyFields(endpoint.RequestSchema)
	if len(requiredFields) == 0 {
		return nil, nil
	}

	// 生成一份完整的请求采样作为基线
	fullSample := sampler.FromSchema(endpoint.RequestSchema)
	bodyMap, _ := fullSample.Body.(map[string]any)
	if bodyMap == nil {
		return nil, nil
	}

	var drafts []testcase.Draft
	for _, field := range requiredFields {
		if _, exists := bodyMap[field]; !exists {
			continue
		}
		// 深拷贝 body 并删除指定必填字段
		trimmed := cloneMap(bodyMap)
		delete(trimmed, field)

		request := map[string]any{
			"method":    strings.ToUpper(endpoint.Method),
			"path":      endpoint.Path,
			"headers":   fullSample.Headers,
			"query":     fullSample.Query,
			"variables": fullSample.Path,
			"body":      trimmed,
		}
		if fullSample.Security != nil {
			request["security"] = fullSample.Security
		}

		assertions := []map[string]any{
			{"type": "status_code", "expected": defaultExpectedErrorStatus()},
		}

		rawRequest, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		rawAssertions, err := json.Marshal(assertions)
		if err != nil {
			return nil, err
		}

		endpointID := endpoint.ID
		draft := testcase.Draft{
			ProjectID:        endpoint.ProjectID,
			ServiceID:        endpoint.ServiceID,
			EndpointID:       &endpointID,
			Source:           testcase.SourceAuto,
			Name:             caseName(endpoint) + " [缺失必填字段: " + field + "]",
			Method:           endpoint.Method,
			Path:             endpoint.Path,
			GenerationRuleID: RuleRequiredMissing,
			Request:          rawRequest,
			Assertions:       rawAssertions,
			Status:           testcase.StatusActive,
		}
		draft.Fingerprint = buildFingerprint(endpoint, RuleRequiredMissing+":"+field)
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

type TypeMismatchRule struct{}

func (TypeMismatchRule) ID() string {
	return RuleTypeMismatch
}

func (r TypeMismatchRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	properties, requiredMap := extractBodyProperties(endpoint.RequestSchema)
	if len(properties) == 0 {
		return nil, nil
	}

	// 生成一份完整的请求采样作为基线
	fullSample := sampler.FromSchema(endpoint.RequestSchema)
	bodyMap, _ := fullSample.Body.(map[string]any)
	if bodyMap == nil {
		return nil, nil
	}

	// 预计算断言
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
		wrongValues := wrongTypeValues(schemaType)
		for _, wrongVal := range wrongValues {
			mutated := cloneMap(bodyMap)
			mutated[field] = wrongVal

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

			typeLabel := wrongValueTypeLabel(wrongVal)
			endpointID := endpoint.ID
			draft := testcase.Draft{
				ProjectID:        endpoint.ProjectID,
				ServiceID:        endpoint.ServiceID,
				EndpointID:       &endpointID,
				Source:           testcase.SourceAuto,
				Name:             caseName(endpoint) + " [类型错误: " + field + " 传" + typeLabel + "]",
				Method:           endpoint.Method,
				Path:             endpoint.Path,
				GenerationRuleID: RuleTypeMismatch,
				Request:          rawRequest,
				Assertions:       rawAssertions,
				Status:           testcase.StatusActive,
			}
			draft.Fingerprint = buildFingerprint(endpoint, RuleTypeMismatch+":"+field+":"+typeLabel)
			drafts = append(drafts, draft)
		}
	}
	_ = requiredMap
	return drafts, nil
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// extractRequiredBodyFields 从 RequestSchema 中提取 body 的 required 字段列表。
func extractRequiredBodyFields(requestSchema json.RawMessage) []string {
	var schema map[string]any
	if err := json.Unmarshal(requestSchema, &schema); err != nil {
		return nil
	}
	body, _ := schema["body"].(map[string]any)
	if body == nil {
		return nil
	}
	required, _ := body["required"].([]any)
	out := make([]string, 0, len(required))
	for _, r := range required {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// extractBodyProperties 从 RequestSchema 中提取 body 的 properties 和 required 集合。
func extractBodyProperties(requestSchema json.RawMessage) (map[string]map[string]any, map[string]bool) {
	var schema map[string]any
	if err := json.Unmarshal(requestSchema, &schema); err != nil {
		return nil, nil
	}
	body, _ := schema["body"].(map[string]any)
	if body == nil {
		return nil, nil
	}
	properties, _ := body["properties"].(map[string]any)
	out := make(map[string]map[string]any, len(properties))
	for name, prop := range properties {
		if m, ok := prop.(map[string]any); ok {
			out[name] = m
		}
	}
	requiredSet := make(map[string]bool)
	required, _ := body["required"].([]any)
	for _, r := range required {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}
	return out, requiredSet
}

// cloneMap 浅拷贝 map[string]any。
func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// wrongTypeValues 根据 schema 类型返回一组错误类型的值。
func wrongTypeValues(schemaType string) []any {
	switch schemaType {
	case "string":
		return []any{12345, true, []any{1, 2, 3}}
	case "integer":
		return []any{"not_an_integer", 3.14, true}
	case "number":
		return []any{"not_a_number", true}
	case "boolean":
		return []any{"not_a_boolean", 123}
	case "array":
		return []any{"not_an_array", 123}
	case "object":
		return []any{"not_an_object", 123}
	default:
		return nil
	}
}

// wrongValueTypeLabel 返回错误值的类型标签，用于用例命名。
func wrongValueTypeLabel(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int, int64, float64:
		return "number"
	case bool:
		return "bool"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "invalid"
	}
}
