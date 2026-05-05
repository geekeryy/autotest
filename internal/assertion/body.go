package assertion

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// BodyContains asserts against the raw response body string.
//
// JSON spec example:
//
//	{"type":"body_contains","value":"success"}
//	{"type":"body_contains","op":"not_contains","value":"error"}
//	{"type":"body_contains","op":"matches","value":"^\\{\"code\":0"}
//
// Supported ops (default: contains): contains, not_contains, matches, eq.
type BodyContains struct {
	Value string
	Op    string
}

func (a BodyContains) Type() string { return "body_contains" }

func (a BodyContains) Assert(ctx context.Context, input Input) Result {
	body := string(input.Body)
	op := a.Op
	if op == "" {
		op = "contains"
	}

	switch op {
	case "contains":
		if strings.Contains(body, a.Value) {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("response body does not contain %q", a.Value),
		}

	case "not_contains":
		if !strings.Contains(body, a.Value) {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("response body contains %q (expected not to)", a.Value),
		}

	case "matches":
		re, err := regexp.Compile(a.Value)
		if err != nil {
			return Result{
				Type:    a.Type(),
				Passed:  false,
				Message: fmt.Sprintf("invalid regex %q: %v", a.Value, err),
			}
		}
		if re.MatchString(body) {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("response body does not match /%s/", a.Value),
		}

	case "eq":
		if body == a.Value {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("expected body to equal %q", a.Value),
		}

	default:
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("unknown op %q for body_contains", op),
		}
	}
}

func buildBodyContainsAssertion(spec map[string]any) (BodyContains, error) {
	value, _ := spec["value"].(string)
	op, _ := spec["op"].(string)
	return BodyContains{Value: value, Op: op}, nil
}

// ResponseTime asserts that the HTTP response time satisfies a numeric threshold.
//
// JSON spec example:
//
//	{"type":"response_time","op":"lte","expected":500}
//	{"type":"response_time","op":"lt","expected":1000}
//
// Supported ops (default: lte): lt, lte, gt, gte, eq, ne.
// The expected value is in milliseconds.
type ResponseTime struct {
	Op       string
	Expected int64
}

func (a ResponseTime) Type() string { return "response_time" }

func (a ResponseTime) Assert(ctx context.Context, input Input) Result {
	op := a.Op
	if op == "" {
		op = "lte"
	}
	passed, msg := applyOp(float64(input.DurationMillis), op, float64(a.Expected))
	if passed {
		return Result{Type: a.Type(), Passed: true}
	}
	return Result{
		Type:    a.Type(),
		Passed:  false,
		Message: fmt.Sprintf("response time %dms: %s", input.DurationMillis, msg),
	}
}

func buildResponseTimeAssertion(spec map[string]any) (ResponseTime, error) {
	op, _ := spec["op"].(string)
	if op == "" {
		op = "lte"
	}
	expected, ok := numberAsInt64(spec["expected"])
	if !ok {
		return ResponseTime{}, fmt.Errorf("response_time assertion: expected must be a number (milliseconds)")
	}
	return ResponseTime{Op: op, Expected: expected}, nil
}
