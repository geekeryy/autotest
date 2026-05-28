package testprofile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	maxEnumValues    = 20
	minOccurrences   = 3
	maxFieldProfiles = 200
)

// LearnFieldProfiles analyzes historical successful request bodies to discover
// the actual value domain for each field. Fields with a bounded set of observed
// values (≤maxEnumValues distinct values) are marked as enums.
type fieldAccum struct {
	method   string
	path     string
	location string
	field    string
	values   map[string]int
	vtype    string
}

func LearnFieldProfiles(results []RunResultInfo) []FieldProfile {

	accum := map[string]*fieldAccum{}

	for _, r := range results {
		if r.Status != "passed" {
			continue
		}
		var reqSnap map[string]json.RawMessage
		if err := json.Unmarshal(r.RequestSnapshot, &reqSnap); err != nil {
			continue
		}

		extractFields(accum, r.Method, r.Path, "body", reqSnap["body"])
		extractFields(accum, r.Method, r.Path, "query", reqSnap["query"])
		extractFields(accum, r.Method, r.Path, "headers", reqSnap["headers"])
	}

	var profiles []FieldProfile
	for _, a := range accum {
		if len(a.values) < minOccurrences || len(a.values) == 0 {
			continue
		}
		fp := FieldProfile{
			Method:    a.method,
			Path:      a.path,
			Location:  a.location,
			FieldPath: a.field,
			ValueType: a.vtype,
			IsEnum:    len(a.values) <= maxEnumValues,
		}
		if fp.IsEnum {
			fp.ObservedValues = topValues(a.values, maxEnumValues)
		}
		profiles = append(profiles, fp)
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Path != profiles[j].Path {
			return profiles[i].Path < profiles[j].Path
		}
		return profiles[i].FieldPath < profiles[j].FieldPath
	})

	if len(profiles) > maxFieldProfiles {
		profiles = profiles[:maxFieldProfiles]
	}
	return profiles
}

func extractFields(accum map[string]*fieldAccum, method, path, location string, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return
	}

	walkJSON("", value, func(fieldPath string, val any) {
		if fieldPath == "" {
			return
		}
		key := fmt.Sprintf("%s|%s|%s|%s", method, path, location, fieldPath)
		a, ok := accum[key]
		if !ok {
			a = &fieldAccum{
				method:   method,
				path:     path,
				location: location,
				field:    fieldPath,
				values:   map[string]int{},
			}
			accum[key] = a
		}
		strVal := fmt.Sprintf("%v", val)
		if len(strVal) > 200 {
			return
		}
		a.values[strVal]++
		if a.vtype == "" {
			a.vtype = jsonType(val)
		}
	})
}

func walkJSON(prefix string, value any, fn func(path string, val any)) {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			childPath := k
			if prefix != "" {
				childPath = prefix + "." + k
			}
			fn(childPath, child)
			if nested, ok := child.(map[string]any); ok {
				walkJSON(childPath, nested, fn)
			}
		}
	case []any:
		if len(v) > 0 {
			walkJSON(prefix+"[]", v[0], fn)
		}
	default:
		if prefix != "" {
			fn(prefix, value)
		}
	}
}

func jsonType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "unknown"
	}
}

func topValues(values map[string]int, limit int) []any {
	type kv struct {
		key   string
		count int
	}
	var sorted []kv
	for k, v := range values {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	result := make([]any, 0, len(sorted))
	for _, item := range sorted {
		result = append(result, parseScalarValue(item.key))
	}
	return result
}

func parseScalarValue(s string) any {
	s = strings.TrimSpace(s)
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	var num float64
	if _, err := fmt.Sscanf(s, "%f", &num); err == nil {
		if num == float64(int(num)) {
			return int(num)
		}
		return num
	}
	return s
}
