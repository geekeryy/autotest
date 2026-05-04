package assertion

import (
	"context"
	"testing"
)

func TestStatusCodeAssertion(t *testing.T) {
	t.Parallel()

	assertion := StatusCode{Expected: 201}

	passed := assertion.Assert(context.Background(), Input{StatusCode: 201})
	if !passed.Passed {
		t.Fatalf("expected assertion to pass: %#v", passed)
	}

	failed := assertion.Assert(context.Background(), Input{StatusCode: 500})
	if failed.Passed {
		t.Fatalf("expected assertion to fail")
	}
	if failed.Message == "" {
		t.Fatalf("expected failure message")
	}
}

func TestEngineEvaluatesStatusCode(t *testing.T) {
	t.Parallel()

	engine := NewEngine()
	results := engine.Evaluate(context.Background(), []byte(`[{"type":"status_code","expected":200}]`), Input{StatusCode: 200})
	if len(results) != 1 {
		t.Fatalf("expected one assertion result, got %d", len(results))
	}
	if !AllPassed(results) {
		t.Fatalf("expected all assertions to pass: %#v", results)
	}
}
