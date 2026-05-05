package assertion

import (
	"context"
	"fmt"
)

// HeaderAssertion checks a response header value using an operator.
//
// JSON spec example:
//
//	{"type":"header","name":"Content-Type","op":"exists"}
//	{"type":"header","name":"Content-Type","op":"contains","expected":"application/json"}
//	{"type":"header","name":"X-Request-Id","op":"matches","expected":"^[0-9a-f-]{36}$"}
//
// Supported ops: exists, not_exists, eq, ne, contains, not_contains, matches,
// is_empty, is_not_empty, is_null (alias for not_exists), is_not_null (alias for exists).
type HeaderAssertion struct {
	Name     string
	Op       string
	Expected string
}

func (a HeaderAssertion) Type() string { return "header" }

func (a HeaderAssertion) Assert(ctx context.Context, input Input) Result {
	values := input.Headers.Values(a.Name)
	op := a.Op
	if op == "" {
		op = "exists"
	}

	absent := len(values) == 0

	switch op {
	case "not_exists", "is_null":
		if absent {
			return Result{Type: a.Type(), Passed: true}
		}
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("expected header %q to not exist, but found %q", a.Name, values[0]),
		}
	}

	if absent {
		return Result{
			Type:    a.Type(),
			Passed:  false,
			Message: fmt.Sprintf("header %q not found", a.Name),
		}
	}

	actual := values[0]

	switch op {
	case "exists", "is_not_null":
		return Result{Type: a.Type(), Passed: true}
	}

	passed, msg := applyOp(actual, op, a.Expected)
	if passed {
		return Result{Type: a.Type(), Passed: true}
	}
	return Result{
		Type:    a.Type(),
		Passed:  false,
		Message: fmt.Sprintf("header %q: %s", a.Name, msg),
	}
}

func buildHeaderAssertion(spec map[string]any) (HeaderAssertion, error) {
	name, _ := spec["name"].(string)
	if name == "" {
		return HeaderAssertion{}, fmt.Errorf("header assertion requires name")
	}
	op, _ := spec["op"].(string)
	if op == "" {
		op = "exists"
	}
	expected, _ := spec["expected"].(string)
	return HeaderAssertion{Name: name, Op: op, Expected: expected}, nil
}
