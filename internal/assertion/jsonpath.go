package assertion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"
)

// JSONPathAssertion evaluates a JSONPath expression against the response body
// and asserts the extracted value using an operator.
//
// JSON spec example:
//
//	{"type":"jsonpath","path":"$.data.id","op":"exists"}
//	{"type":"jsonpath","path":"$.code","op":"eq","expected":0}
//	{"type":"jsonpath","path":"$.items","op":"is_not_empty"}
//	{"type":"jsonpath","path":"$.name","op":"matches","expected":"^test.*"}
//
// Supported ops: exists, not_exists, is_null, is_not_null, eq, ne,
// gt, gte, lt, lte, contains, not_contains, matches, is_empty, is_not_empty.
type JSONPathAssertion struct {
	Path     string
	Op       string
	Expected any
}

func (a JSONPathAssertion) Type() string { return "jsonpath" }

func (a JSONPathAssertion) Assert(ctx context.Context, input Input) Result {
	if len(input.Body) == 0 {
		if a.Op == "not_exists" || a.Op == "is_null" || a.Op == "is_empty" {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{Type: a.Type(), Passed: false, Message: "response body is empty"}
	}

	var doc any
	if err := json.Unmarshal(input.Body, &doc); err != nil {
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("response body is not valid JSON: %v", err),
		}
	}

	actual, err := jsonpath.Get(a.Path, doc)
	if err != nil {
		// Path not found – treat as null/not_exists
		if a.Op == "not_exists" || a.Op == "is_null" || a.Op == "is_empty" {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", a.Path, err),
		}
	}

	passed, msg := applyOp(actual, a.Op, a.Expected)
	if passed {
		return Result{Type: a.Type(), Passed: true}
	}
	return Result{
		Type:    a.Type(),
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: %s", a.Path, msg),
	}
}

func buildJSONPathAssertion(spec map[string]any) (JSONPathAssertion, error) {
	path, _ := spec["path"].(string)
	if path == "" {
		return JSONPathAssertion{}, fmt.Errorf("jsonpath assertion requires a path")
	}
	op, _ := spec["op"].(string)
	if op == "" {
		op = "exists"
	}
	return JSONPathAssertion{
		Path:     path,
		Op:       op,
		Expected: spec["expected"],
	}, nil
}
