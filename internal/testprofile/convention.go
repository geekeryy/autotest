package testprofile

import (
	"encoding/json"
	"sort"
	"strings"
)

// InferResponseConvention analyzes a set of successful API response bodies and
// infers the project's common response envelope. It uses frequency analysis
// (no LLM) so it's fast and deterministic.
//
// The algorithm:
//  1. Parse each response body as a JSON object.
//  2. Count how often each top-level key appears.
//  3. Keys appearing in ≥80% of responses are candidate wrapper fields.
//  4. Among those, heuristically identify success-indicator, data, and message fields.
func InferResponseConvention(responses []json.RawMessage) *ResponseConvention {
	if len(responses) == 0 {
		return nil
	}

	type fieldStats struct {
		count       int
		values      map[string]int // stringified value → count (for success field detection)
		isScalar    int            // how often the value is scalar (not object/array)
		isContainer int            // how often the value is object or array
	}

	stats := map[string]*fieldStats{}
	validCount := 0

	for _, raw := range responses {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		validCount++
		for key, val := range obj {
			s, ok := stats[key]
			if !ok {
				s = &fieldStats{values: map[string]int{}}
				stats[key] = s
			}
			s.count++

			trimmed := strings.TrimSpace(string(val))
			if isScalarJSON(trimmed) {
				s.isScalar++
				s.values[trimmed]++
			} else {
				s.isContainer++
			}
		}
	}

	if validCount < 3 {
		return nil
	}

	threshold := float64(validCount) * 0.8
	var wrapperFields []string
	for key, s := range stats {
		if float64(s.count) >= threshold {
			wrapperFields = append(wrapperFields, key)
		}
	}
	sort.Strings(wrapperFields)

	if len(wrapperFields) < 2 {
		return nil
	}

	conv := &ResponseConvention{
		WrapperFields: wrapperFields,
		Confidence:    float64(validCount) / float64(len(responses)),
	}

	// Identify success indicator: a scalar field with very few distinct values
	// (typically 1-3) where one value dominates.
	for _, key := range wrapperFields {
		s := stats[key]
		if s.isScalar < s.isContainer {
			continue
		}
		if len(s.values) == 0 || len(s.values) > 5 {
			continue
		}
		if isSuccessFieldName(key) {
			dominant, dominantCount := dominantValue(s.values)
			if float64(dominantCount) >= float64(s.count)*0.7 {
				conv.SuccessField = key
				conv.SuccessValue = parseJSONScalar(dominant)
				break
			}
		}
	}

	// Identify data field: a container field (object/array) that appears frequently.
	for _, key := range wrapperFields {
		if isDataFieldName(key) {
			s := stats[key]
			if s.isContainer >= s.isScalar {
				conv.DataField = key
				break
			}
		}
	}

	// Identify message field: a scalar string field.
	for _, key := range wrapperFields {
		if isMessageFieldName(key) {
			s := stats[key]
			if s.isScalar >= s.isContainer {
				conv.MessageField = key
				break
			}
		}
	}

	return conv
}

// BuildConventionAssertions generates assertion specs from a response convention.
func BuildConventionAssertions(conv *ResponseConvention, expectedStatus int) []map[string]any {
	assertions := []map[string]any{
		{"type": "status_code", "expected": expectedStatus},
	}
	if conv == nil {
		return assertions
	}
	if conv.SuccessField != "" && conv.SuccessValue != nil {
		assertions = append(assertions, map[string]any{
			"type":     "jsonpath",
			"path":     "$." + conv.SuccessField,
			"op":       "eq",
			"expected": conv.SuccessValue,
		})
	}
	if conv.DataField != "" {
		assertions = append(assertions, map[string]any{
			"type": "jsonpath",
			"path": "$." + conv.DataField,
			"op":   "is_not_null",
		})
	}
	return assertions
}

func isSuccessFieldName(name string) bool {
	n := strings.ToLower(name)
	return n == "code" || n == "status" || n == "errcode" || n == "err_code" ||
		n == "errno" || n == "ret" || n == "retcode" || n == "ret_code" ||
		n == "error_code" || n == "resultcode" || n == "result_code" || n == "success"
}

func isDataFieldName(name string) bool {
	n := strings.ToLower(name)
	return n == "data" || n == "result" || n == "body" || n == "payload" ||
		n == "content" || n == "results" || n == "items" || n == "response"
}

func isMessageFieldName(name string) bool {
	n := strings.ToLower(name)
	return n == "msg" || n == "message" || n == "errmsg" || n == "err_msg" ||
		n == "error_message" || n == "errormessage" || n == "info" || n == "desc" ||
		n == "description" || n == "reason" || n == "error"
}

func isScalarJSON(s string) bool {
	if s == "" {
		return false
	}
	return s[0] != '{' && s[0] != '['
}

func dominantValue(values map[string]int) (string, int) {
	var maxKey string
	var maxCount int
	for k, v := range values {
		if v > maxCount {
			maxKey = k
			maxCount = v
		}
	}
	return maxKey, maxCount
}

func parseJSONScalar(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	return v
}
