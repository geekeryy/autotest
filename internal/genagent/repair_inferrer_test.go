package genagent

import (
	"context"
	"encoding/json"
	"testing"

	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// --- fakes ---

type fakeScenarioRepairService struct {
	upserted []scenario.UpsertStepInput
}

func (f *fakeScenarioRepairService) ListSteps(_ context.Context, _ uuid.UUID) ([]scenario.Step, error) {
	return nil, nil
}
func (f *fakeScenarioRepairService) UpsertStep(_ context.Context, _ uuid.UUID, in scenario.UpsertStepInput) (*scenario.Step, error) {
	f.upserted = append(f.upserted, in)
	return &scenario.Step{}, nil
}

type fakeInferrer struct {
	result json.RawMessage
	err    error
	called bool
}

func (f *fakeInferrer) InferForCase(_ context.Context, _ uuid.UUID) (json.RawMessage, error) {
	f.called = true
	return f.result, f.err
}

func stepResult(statusCode int) runner.StepRunResult {
	snap, _ := json.Marshal(map[string]any{"statusCode": statusCode})
	return runner.StepRunResult{
		Result: &report.Result{ResponseSnapshot: snap},
	}
}

// --- tests ---

func TestRepairGenerationUsesInferredAssertions(t *testing.T) {
	t.Parallel()

	inferred := json.RawMessage(`[{"type":"status_code","op":"eq","expected":200},{"type":"jsonpath","path":"$.data.id","op":"exists"}]`)
	svc := &fakeScenarioRepairService{}
	inferrer := &fakeInferrer{result: inferred}
	caseID := uuid.New()

	step := &scenario.Step{
		ID:         uuid.New(),
		TestCaseID: caseID,
		StepOrder:  1,
		StepType:   scenario.StepTypeAPI,
		Enabled:    true,
	}
	r := &Repairer{
		Scenarios:      svc,
		AssertInferrer: inferrer,
	}

	summary, err := repairGeneration(context.Background(), r, uuid.New(), uuid.New(), step, stepResult(400), failureClass{Reason: "missing assertions"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inferrer.called {
		t.Fatal("expected AssertionInferrer.InferForCase to be called")
	}
	if summary == nil {
		t.Fatal("expected non-nil RepairSummary")
	}
	if len(svc.upserted) == 0 {
		t.Fatal("expected step to be upserted")
	}

	var cfg map[string]any
	if err := json.Unmarshal(svc.upserted[0].Config, &cfg); err != nil {
		t.Fatalf("upserted config not valid JSON: %v", err)
	}
	arr, _ := cfg["assertions"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 inferred assertions in upserted config, got %d: %s", len(arr), svc.upserted[0].Config)
	}
}

func TestRepairGenerationFallsBackToStatusCodeWhenInferrerReturnsNothing(t *testing.T) {
	t.Parallel()

	svc := &fakeScenarioRepairService{}
	inferrer := &fakeInferrer{result: nil} // returns nothing
	step := &scenario.Step{
		ID:         uuid.New(),
		TestCaseID: uuid.New(),
		StepOrder:  1,
		StepType:   scenario.StepTypeAPI,
		Enabled:    true,
	}
	r := &Repairer{
		Scenarios:      svc,
		AssertInferrer: inferrer,
	}

	_, err := repairGeneration(context.Background(), r, uuid.New(), uuid.New(), step, stepResult(503), failureClass{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg map[string]any
	json.Unmarshal(svc.upserted[0].Config, &cfg)
	arr, _ := cfg["assertions"].([]any)
	if len(arr) != 1 {
		t.Fatalf("expected status-code fallback assertion, got %v", cfg["assertions"])
	}
	m, _ := arr[0].(map[string]any)
	if m["expected"] != float64(503) {
		t.Fatalf("expected fallback to stamp status 503, got %v", m["expected"])
	}
}

func TestRepairGenerationSkipsInferrerWhenNilCaseID(t *testing.T) {
	t.Parallel()

	svc := &fakeScenarioRepairService{}
	inferrer := &fakeInferrer{result: json.RawMessage(`[{"type":"status_code","op":"eq","expected":200}]`)}
	step := &scenario.Step{
		ID:        uuid.New(),
		StepOrder: 1,
		StepType:  scenario.StepTypeAPI,
		Enabled:   true,
		// TestCaseID is zero value — inferrer must not be called
	}
	r := &Repairer{Scenarios: svc, AssertInferrer: inferrer}

	_, err := repairGeneration(context.Background(), r, uuid.New(), uuid.New(), step, stepResult(400), failureClass{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inferrer.called {
		t.Fatal("inferrer must not be called when TestCaseID is zero")
	}
}

func TestRepairGenerationWithoutInferrerStampsStatusCode(t *testing.T) {
	t.Parallel()

	svc := &fakeScenarioRepairService{}
	step := &scenario.Step{
		ID:         uuid.New(),
		TestCaseID: uuid.New(),
		StepOrder:  1,
		StepType:   scenario.StepTypeAPI,
		Enabled:    true,
	}
	r := &Repairer{Scenarios: svc} // nil AssertInferrer — legacy path

	_, err := repairGeneration(context.Background(), r, uuid.New(), uuid.New(), step, stepResult(422), failureClass{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg map[string]any
	json.Unmarshal(svc.upserted[0].Config, &cfg)
	arr, _ := cfg["assertions"].([]any)
	if len(arr) != 1 {
		t.Fatalf("expected 1 status-code assertion, got %d", len(arr))
	}
	m, _ := arr[0].(map[string]any)
	if m["expected"] != float64(422) {
		t.Fatalf("expected 422, got %v", m["expected"])
	}
}
