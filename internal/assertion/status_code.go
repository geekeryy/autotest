package assertion

import (
	"context"
	"fmt"
)

type StatusCode struct {
	Expected int
}

func (a StatusCode) Type() string {
	return "status_code"
}

func (a StatusCode) Assert(ctx context.Context, input Input) Result {
	if input.StatusCode == a.Expected {
		return Result{Type: a.Type(), Passed: true}
	}
	return Result{
		Type:    a.Type(),
		Passed:  false,
		Message: fmt.Sprintf("expected status %d, got %d", a.Expected, input.StatusCode),
	}
}
