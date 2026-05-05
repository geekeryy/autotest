package assertion

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// applyOp compares actual against expected using the given operator.
// Returns (passed bool, failureMessage string).
//
// Supported operators:
//
//	exists / is_not_null   – actual is not nil
//	not_exists / is_null   – actual is nil
//	eq                     – deep JSON equality
//	ne                     – not deeply equal
//	gt / gte / lt / lte    – numeric comparison
//	contains               – string contains substring
//	not_contains           – string does not contain substring
//	matches                – string matches regexp
//	is_empty               – nil, empty string, empty array, or empty object
//	is_not_empty           – inverse of is_empty
func applyOp(actual any, op string, expected any) (bool, string) {
	switch op {
	case "exists", "is_not_null":
		if actual == nil {
			return false, "expected value to exist but got null"
		}
		return true, ""

	case "not_exists", "is_null":
		if actual != nil {
			return false, fmt.Sprintf("expected null, got %s", jsonStr(actual))
		}
		return true, ""

	case "eq":
		if jsonDeepEqual(actual, expected) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %s, got %s", jsonStr(expected), jsonStr(actual))

	case "ne":
		if !jsonDeepEqual(actual, expected) {
			return true, ""
		}
		return false, fmt.Sprintf("expected value not to equal %s", jsonStr(expected))

	case "gt":
		a, e, err := toFloats(actual, expected)
		if err != nil {
			return false, err.Error()
		}
		if a > e {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v > %v", a, e)

	case "gte":
		a, e, err := toFloats(actual, expected)
		if err != nil {
			return false, err.Error()
		}
		if a >= e {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v >= %v", a, e)

	case "lt":
		a, e, err := toFloats(actual, expected)
		if err != nil {
			return false, err.Error()
		}
		if a < e {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v < %v", a, e)

	case "lte":
		a, e, err := toFloats(actual, expected)
		if err != nil {
			return false, err.Error()
		}
		if a <= e {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v <= %v", a, e)

	case "contains":
		s, ok := actual.(string)
		if !ok {
			return false, fmt.Sprintf("contains requires string value, got %T", actual)
		}
		sub := fmt.Sprintf("%v", expected)
		if strings.Contains(s, sub) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %q to contain %q", s, sub)

	case "not_contains":
		s, ok := actual.(string)
		if !ok {
			return false, fmt.Sprintf("not_contains requires string value, got %T", actual)
		}
		sub := fmt.Sprintf("%v", expected)
		if !strings.Contains(s, sub) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %q not to contain %q", s, sub)

	case "matches":
		s, ok := actual.(string)
		if !ok {
			return false, fmt.Sprintf("matches requires string value, got %T", actual)
		}
		pattern, ok2 := expected.(string)
		if !ok2 {
			return false, "matches requires string pattern as expected"
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, fmt.Sprintf("invalid regex pattern: %v", err)
		}
		if re.MatchString(s) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %q to match /%s/", s, pattern)

	case "is_empty":
		if isEmpty(actual) {
			return true, ""
		}
		return false, fmt.Sprintf("expected empty, got %s", jsonStr(actual))

	case "is_not_empty":
		if !isEmpty(actual) {
			return true, ""
		}
		return false, "expected not empty"

	default:
		return false, fmt.Sprintf("unknown comparison operator %q", op)
	}
}

func toFloats(a, b any) (float64, float64, error) {
	fa, err := anyToFloat64(a)
	if err != nil {
		return 0, 0, fmt.Errorf("actual value %v cannot be used as number: %w", a, err)
	}
	fb, err := anyToFloat64(b)
	if err != nil {
		return 0, 0, fmt.Errorf("expected value %v cannot be used as number: %w", b, err)
	}
	return fa, fb, nil
}

func anyToFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

func jsonStr(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func jsonDeepEqual(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return reflect.DeepEqual(a, b)
	}
	return string(ja) == string(jb)
}

func numberAsInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}
