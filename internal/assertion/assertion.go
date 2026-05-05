package assertion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Input holds all data available to assertion evaluators.
type Input struct {
	StatusCode     int
	Headers        http.Header
	Body           []byte
	DurationMillis int64
	Variables      map[string]string
}

// Result is the outcome of a single assertion check.
type Result struct {
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// Assertion evaluates a single condition and returns one Result.
type Assertion interface {
	Type() string
	Assert(ctx context.Context, input Input) Result
}

// MultiAssertion evaluates a condition that may produce multiple Results
// (e.g. a script with several pm.test() calls).
type MultiAssertion interface {
	AssertMulti(ctx context.Context, input Input) []Result
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Evaluate(ctx context.Context, raw json.RawMessage, input Input) []Result {
	if len(raw) == 0 {
		return nil
	}

	var specs []map[string]any
	if err := json.Unmarshal(raw, &specs); err != nil {
		return []Result{{Type: "parse", Passed: false, Message: err.Error()}}
	}

	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		a, err := build(spec)
		if err != nil {
			results = append(results, Result{Type: fmt.Sprint(spec["type"]), Passed: false, Message: err.Error()})
			continue
		}
		switch v := a.(type) {
		case MultiAssertion:
			results = append(results, v.AssertMulti(ctx, input)...)
		case Assertion:
			results = append(results, v.Assert(ctx, input))
		}
	}
	return results
}

func AllPassed(results []Result) bool {
	for _, result := range results {
		if !result.Passed {
			return false
		}
	}
	return true
}

func build(spec map[string]any) (any, error) {
	assertionType, _ := spec["type"].(string)
	switch assertionType {
	case "status_code":
		expected, ok := numberAsInt(spec["expected"])
		if !ok {
			return nil, fmt.Errorf("status_code assertion: expected must be a number")
		}
		return StatusCode{Expected: expected}, nil

	case "jsonpath":
		return buildJSONPathAssertion(spec)

	case "header":
		return buildHeaderAssertion(spec)

	case "body_contains":
		return buildBodyContainsAssertion(spec)

	case "response_time":
		return buildResponseTimeAssertion(spec)

	case "script":
		script, _ := spec["script"].(string)
		return ScriptAssertion{Script: script}, nil

	case "schema":
		return Unsupported{assertionType: assertionType}, nil

	default:
		return nil, fmt.Errorf("unknown assertion type %q", assertionType)
	}
}

func numberAsInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

type Unsupported struct {
	assertionType string
}

func (u Unsupported) Type() string {
	return u.assertionType
}

func (u Unsupported) Assert(ctx context.Context, input Input) Result {
	return Result{Type: u.assertionType, Passed: false, Message: "assertion type is not implemented yet"}
}
