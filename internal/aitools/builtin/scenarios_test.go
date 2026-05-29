package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"autotest/internal/aitools"
	testcase "autotest/internal/case"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// testCallerCtx stamps a CallerContext onto a fresh context so tools that
// enforce project isolation pass through during unit tests. Each test
// should pass the same projectID it puts into args so the validation
// succeeds.
func testCallerCtx(projectID uuid.UUID) context.Context {
	return aitools.WithCaller(context.Background(), aitools.CallerContext{
		UserID:    uuid.New(),
		Username:  "tester",
		ProjectID: projectID,
	})
}

// fakeScenarioService is a minimal in-memory ScenarioService implementation
// just expressive enough to validate the create_scenario_with_steps and
// related step tools. It assigns step_seq monotonically (mirroring the
// real repository's behavior of allocating a stable sequence number on
// first insert) and respects (scenarioId, stepOrder) as the upsert key.
type fakeScenarioService struct {
	scenarios map[uuid.UUID]*scenario.Scenario
	stepsByID map[uuid.UUID]*scenario.Step
	stepsByOS map[uuid.UUID]map[int]*scenario.Step
	nextSeq   int
	upsertErr error
}

func newFakeScenarios() *fakeScenarioService {
	return &fakeScenarioService{
		scenarios: map[uuid.UUID]*scenario.Scenario{},
		stepsByID: map[uuid.UUID]*scenario.Step{},
		stepsByOS: map[uuid.UUID]map[int]*scenario.Step{},
	}
}

func (f *fakeScenarioService) Get(_ context.Context, id uuid.UUID) (*scenario.Scenario, error) {
	sc, ok := f.scenarios[id]
	if !ok {
		return nil, scenario.ErrScenarioNotFound
	}
	return sc, nil
}

func (f *fakeScenarioService) List(_ context.Context, _ scenario.ListFilter) ([]scenario.Scenario, error) {
	out := make([]scenario.Scenario, 0, len(f.scenarios))
	for _, sc := range f.scenarios {
		out = append(out, *sc)
	}
	return out, nil
}

func (f *fakeScenarioService) Create(_ context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error) {
	sc := &scenario.Scenario{
		ID:          uuid.New(),
		ProjectID:   input.ProjectID,
		ServiceID:   input.ServiceID,
		Name:        input.Name,
		Description: input.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	f.scenarios[sc.ID] = sc
	f.stepsByOS[sc.ID] = map[int]*scenario.Step{}
	return sc, nil
}

func (f *fakeScenarioService) Update(_ context.Context, id uuid.UUID, input scenario.UpdateScenarioInput) (*scenario.Scenario, error) {
	sc, ok := f.scenarios[id]
	if !ok {
		return nil, scenario.ErrScenarioNotFound
	}
	sc.Name = input.Name
	sc.Description = input.Description
	return sc, nil
}

func (f *fakeScenarioService) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.scenarios[id]; !ok {
		return scenario.ErrScenarioNotFound
	}
	delete(f.scenarios, id)
	delete(f.stepsByOS, id)
	return nil
}

func (f *fakeScenarioService) UpsertStep(_ context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	slots, ok := f.stepsByOS[scenarioID]
	if !ok {
		return nil, scenario.ErrScenarioNotFound
	}
	existing, hit := slots[input.StepOrder]
	if hit {
		// stepSeq stays stable across updates (matches repository contract).
		existing.StepType = input.StepType
		existing.Name = input.Name
		if input.Enabled != nil {
			existing.Enabled = *input.Enabled
		}
		existing.TestCaseID = input.TestCaseID
		if len(input.Config) > 0 {
			existing.Config = input.Config
		}
		if len(input.RequestOverride) > 0 {
			existing.RequestOverride = input.RequestOverride
		}
		existing.UpdatedAt = time.Now()
		return existing, nil
	}
	f.nextSeq++
	step := &scenario.Step{
		ID:              uuid.New(),
		ScenarioID:      scenarioID,
		TestCaseID:      input.TestCaseID,
		StepSeq:         f.nextSeq,
		StepOrder:       input.StepOrder,
		StepType:        input.StepType,
		Name:            input.Name,
		Enabled:         true,
		Config:          input.Config,
		RequestOverride: input.RequestOverride,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if input.Enabled != nil {
		step.Enabled = *input.Enabled
	}
	slots[input.StepOrder] = step
	f.stepsByID[step.ID] = step
	return step, nil
}

func (f *fakeScenarioService) DeleteStep(_ context.Context, stepID uuid.UUID) error {
	step, ok := f.stepsByID[stepID]
	if !ok {
		return scenario.ErrStepNotFound
	}
	delete(f.stepsByID, stepID)
	delete(f.stepsByOS[step.ScenarioID], step.StepOrder)
	return nil
}

func (f *fakeScenarioService) ListSteps(_ context.Context, scenarioID uuid.UUID) ([]scenario.Step, error) {
	slots := f.stepsByOS[scenarioID]
	out := make([]scenario.Step, 0, len(slots))
	for _, s := range slots {
		out = append(out, *s)
	}
	return out, nil
}

func (f *fakeScenarioService) ReorderSteps(_ context.Context, _ uuid.UUID, _ scenario.ReorderStepsInput) error {
	return nil
}

// ─── tool tests ────────────────────────────────────────────────────────

func TestValidateStepOrders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		steps   []scenarioStepInput
		wantErr string
	}{
		{
			name: "ok unique 1-based",
			steps: []scenarioStepInput{
				{StepOrder: 1, StepType: "api", Name: "a"},
				{StepOrder: 2, StepType: "api", Name: "b"},
			},
		},
		{
			name: "duplicate stepOrder",
			steps: []scenarioStepInput{
				{StepOrder: 1, StepType: "api", Name: "a"},
				{StepOrder: 1, StepType: "api", Name: "b"},
			},
			wantErr: "stepOrder=1",
		},
		{
			name: "zero is invalid",
			steps: []scenarioStepInput{
				{StepOrder: 0, StepType: "api", Name: "a"},
			},
			wantErr: ">= 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStepOrders(tc.steps)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestBuildUpsertInput_APIRequiresTestCaseID(t *testing.T) {
	t.Parallel()

	_, err := buildUpsertInput(scenarioStepInput{StepOrder: 1, StepType: "api", Name: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "testCaseId") {
		t.Fatalf("expected testCaseId required, got %v", err)
	}

	id := uuid.New()
	in, err := buildUpsertInput(scenarioStepInput{
		StepOrder:  2,
		StepType:   "api",
		Name:       "x",
		TestCaseID: id.String(),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if in.TestCaseID != id {
		t.Fatalf("TestCaseID = %v, want %v", in.TestCaseID, id)
	}
}

func TestBuildUpsertInput_ScriptIgnoresTestCaseID(t *testing.T) {
	t.Parallel()

	in, err := buildUpsertInput(scenarioStepInput{
		StepOrder: 1,
		StepType:  "script",
		Name:      "after-create",
		Config:    json.RawMessage(`{"script": "pm.test('ok', () => {})"}`),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if in.TestCaseID != uuid.Nil {
		t.Fatalf("script step should not have TestCaseID, got %v", in.TestCaseID)
	}
	if string(in.Config) == "" {
		t.Fatalf("script step config dropped")
	}
}

func TestRewriteControlFlowConfig_ForBodyOrders(t *testing.T) {
	t.Parallel()

	orderToSeq := map[int]int{2: 12, 3: 13}
	cfg := json.RawMessage(`{"mode":"count","count":3,"bodyStepOrders":[2,3],"itemVar":"x"}`)

	out, rewritten, err := rewriteControlFlowConfig("for", cfg, orderToSeq)
	if err != nil || !rewritten {
		t.Fatalf("rewrite failed: rewritten=%v err=%v", rewritten, err)
	}
	var blob map[string]any
	if err := json.Unmarshal(out, &blob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := blob["bodyStepOrders"]; ok {
		t.Errorf("bodyStepOrders should have been removed: %s", out)
	}
	seqs, ok := blob["bodyStepSeqs"].([]any)
	if !ok || len(seqs) != 2 || seqs[0].(float64) != 12 || seqs[1].(float64) != 13 {
		t.Errorf("bodyStepSeqs not rewritten correctly: %s", out)
	}
	if blob["itemVar"] != "x" {
		t.Errorf("non-reference fields should be preserved, got %s", out)
	}
}

func TestRewriteControlFlowConfig_ConditionBranches(t *testing.T) {
	t.Parallel()

	orderToSeq := map[int]int{2: 22, 3: 23, 4: 24}
	cfg := json.RawMessage(`{
		"branches":[
			{"left":"a","operator":"==","right":"b","stepOrders":[2,3]}
		],
		"elseStepOrders":[4]
	}`)
	out, rewritten, err := rewriteControlFlowConfig("condition", cfg, orderToSeq)
	if err != nil || !rewritten {
		t.Fatalf("rewrite failed: rewritten=%v err=%v", rewritten, err)
	}
	var blob map[string]any
	if err := json.Unmarshal(out, &blob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := blob["elseStepOrders"]; ok {
		t.Errorf("elseStepOrders should be removed: %s", out)
	}
	elseSeqs, _ := blob["elseStepSeqs"].([]any)
	if len(elseSeqs) != 1 || elseSeqs[0].(float64) != 24 {
		t.Errorf("elseStepSeqs not rewritten: %s", out)
	}
	branches, _ := blob["branches"].([]any)
	first, _ := branches[0].(map[string]any)
	if _, ok := first["stepOrders"]; ok {
		t.Errorf("branches[0].stepOrders should be removed: %s", out)
	}
	seqs, _ := first["stepSeqs"].([]any)
	if len(seqs) != 2 || seqs[0].(float64) != 22 || seqs[1].(float64) != 23 {
		t.Errorf("branches[0].stepSeqs not rewritten: %s", out)
	}
}

func TestRewriteControlFlowConfig_ErrorsOnUnknownOrder(t *testing.T) {
	t.Parallel()

	orderToSeq := map[int]int{2: 22}
	cfg := json.RawMessage(`{"bodyStepOrders":[2,99]}`)
	if _, _, err := rewriteControlFlowConfig("for", cfg, orderToSeq); err == nil ||
		!strings.Contains(err.Error(), "99") {
		t.Fatalf("expected unknown stepOrder error, got %v", err)
	}
}

func TestRewriteControlFlowConfig_NoopWhenNoOrdersFields(t *testing.T) {
	t.Parallel()

	cfg := json.RawMessage(`{"bodyStepSeqs":[12,13],"mode":"count"}`)
	out, rewritten, err := rewriteControlFlowConfig("for", cfg, map[int]int{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rewritten {
		t.Fatalf("rewrite must be a noop when only *Seqs are present")
	}
	if string(out) != string(cfg) {
		t.Fatalf("config altered unnecessarily: %s", out)
	}
}

// ─── end-to-end: create_scenario_with_steps ────────────────────────────

func TestCreateScenarioWithSteps_TwoPhaseControlFlow(t *testing.T) {
	t.Parallel()

	svc := newFakeScenarios()
	tool := createScenarioWithStepsTool(Deps{Scenarios: svc})

	caseA := uuid.New()
	caseB := uuid.New()
	projectID := uuid.New()
	serviceID := uuid.New()
	args := mustJSON(t, map[string]any{
		"projectId":   projectID.String(),
		"serviceId":   serviceID.String(),
		"name":        "checkout flow",
		"description": "AI generated",
		"steps": []map[string]any{
			{"stepOrder": 1, "stepType": "api", "name": "login", "testCaseId": caseA.String()},
			{"stepOrder": 2, "stepType": "api", "name": "create", "testCaseId": caseB.String()},
			{
				"stepOrder": 3,
				"stepType":  "for",
				"name":      "retry create",
				"config":    map[string]any{"mode": "count", "count": 2, "bodyStepOrders": []int{2}},
			},
		},
	})

	out, err := tool.Run(testCallerCtx(projectID), args)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", out)
	}
	sc, ok := result["scenario"].(*scenario.Scenario)
	if !ok {
		t.Fatalf("scenario missing in result: %#v", result)
	}
	if sc.ProjectID != projectID || sc.ServiceID != serviceID {
		t.Errorf("scenario fields not propagated: %+v", sc)
	}
	steps, ok := result["steps"].([]scenario.Step)
	if !ok || len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %#v", result["steps"])
	}

	// Find the for-step and validate seq rewrite happened.
	var forStep *scenario.Step
	for i := range steps {
		if steps[i].StepType == scenario.StepTypeFor {
			forStep = &steps[i]
			break
		}
	}
	if forStep == nil {
		t.Fatalf("for step missing in result")
	}
	var cfg map[string]any
	if err := json.Unmarshal(forStep.Config, &cfg); err != nil {
		t.Fatalf("for step config not valid JSON: %v", err)
	}
	if _, leftover := cfg["bodyStepOrders"]; leftover {
		t.Errorf("bodyStepOrders should be rewritten away: %s", forStep.Config)
	}
	seqs, _ := cfg["bodyStepSeqs"].([]any)
	if len(seqs) != 1 {
		t.Fatalf("bodyStepSeqs missing in final config: %s", forStep.Config)
	}
	// Find the API step at stepOrder=2 and confirm its seq matches.
	var apiSeq int
	for _, s := range steps {
		if s.StepOrder == 2 {
			apiSeq = s.StepSeq
		}
	}
	if int(seqs[0].(float64)) != apiSeq {
		t.Errorf("bodyStepSeqs[0]=%v but stepOrder=2 has seq=%d", seqs[0], apiSeq)
	}
}

func TestCreateScenarioWithSteps_RejectsAPIWithoutTestCase(t *testing.T) {
	t.Parallel()

	svc := newFakeScenarios()
	tool := createScenarioWithStepsTool(Deps{Scenarios: svc})

	projectID := uuid.New()
	args := mustJSON(t, map[string]any{
		"projectId": projectID.String(),
		"serviceId": uuid.New().String(),
		"name":      "x",
		"steps": []map[string]any{
			{"stepOrder": 1, "stepType": "api", "name": "login"},
		},
	})
	_, err := tool.Run(testCallerCtx(projectID), args)
	if err == nil || !strings.Contains(err.Error(), "testCaseId") {
		t.Fatalf("expected testCaseId required, got %v", err)
	}
	// Scenario was created but the first step write failed; that's OK
	// and matches the documented behavior of leaving partial work for
	// the user to fix in the UI.
	if len(svc.scenarios) != 1 {
		t.Errorf("scenario should still be created (partial work preserved), got %d", len(svc.scenarios))
	}
}

func TestCreateScenarioWithSteps_RejectsDuplicateStepOrder(t *testing.T) {
	t.Parallel()

	svc := newFakeScenarios()
	tool := createScenarioWithStepsTool(Deps{Scenarios: svc})

	projectID := uuid.New()
	args := mustJSON(t, map[string]any{
		"projectId": projectID.String(),
		"serviceId": uuid.New().String(),
		"name":      "x",
		"steps": []map[string]any{
			{"stepOrder": 1, "stepType": "api", "name": "a", "testCaseId": uuid.New().String()},
			{"stepOrder": 1, "stepType": "api", "name": "b", "testCaseId": uuid.New().String()},
		},
	})
	_, err := tool.Run(testCallerCtx(projectID), args)
	if err == nil || !strings.Contains(err.Error(), "stepOrder=1") {
		t.Fatalf("expected duplicate stepOrder rejected, got %v", err)
	}
}

// updateScenarioStep should merge missing fields with the current step,
// not blank them out. This guards against a regression where future
// refactors accidentally turn the partial update into a full overwrite.
func TestUpdateScenarioStep_PartialMergesCurrentValues(t *testing.T) {
	t.Parallel()

	svc := newFakeScenarios()
	caseID := uuid.New()
	projectID := uuid.New()
	ctx := testCallerCtx(projectID)
	create := createScenarioWithStepsTool(Deps{Scenarios: svc})
	createArgs := mustJSON(t, map[string]any{
		"projectId": projectID.String(),
		"serviceId": uuid.New().String(),
		"name":      "x",
		"steps": []map[string]any{
			{
				"stepOrder":  1,
				"stepType":   "api",
				"name":       "login",
				"testCaseId": caseID.String(),
				"config":     map[string]any{"hint": "preserved"},
			},
		},
	})
	created, err := create.Run(ctx, createArgs)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	sc := created.(map[string]any)["scenario"].(*scenario.Scenario)

	update := updateScenarioStepTool(Deps{Scenarios: svc})
	updateArgs := mustJSON(t, map[string]any{
		"scenarioId": sc.ID.String(),
		"stepOrder":  1,
		"name":       "login (renamed)",
	})
	updated, err := update.Run(ctx, updateArgs)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	step := updated.(*scenario.Step)
	if step.Name != "login (renamed)" {
		t.Errorf("name not updated: %q", step.Name)
	}
	if step.TestCaseID != caseID {
		t.Errorf("testCaseId should be preserved: got %v want %v", step.TestCaseID, caseID)
	}
	if !strings.Contains(string(step.Config), "preserved") {
		t.Errorf("config should be preserved when not provided, got %s", step.Config)
	}
}

func TestUpdateScenarioStep_FailsOnMissingStep(t *testing.T) {
	t.Parallel()

	svc := newFakeScenarios()
	projectID := uuid.New()
	sc, _ := svc.Create(context.Background(), scenario.CreateScenarioInput{
		ProjectID: projectID,
		ServiceID: uuid.New(),
		Name:      "empty",
	})

	tool := updateScenarioStepTool(Deps{Scenarios: svc})
	args := mustJSON(t, map[string]any{
		"scenarioId": sc.ID.String(),
		"stepOrder":  5,
		"name":       "ghost",
	})
	_, err := tool.Run(testCallerCtx(projectID), args)
	if err == nil || !strings.Contains(err.Error(), "stepOrder=5") {
		t.Fatalf("expected missing-step error, got %v", err)
	}
}

func TestCreateCaseFromEndpoint_RequiresMethodPathWhenNoEndpoint(t *testing.T) {
	t.Parallel()

	cases := &capturingCaseService{}
	tool := createCaseFromEndpointTool(Deps{Cases: cases})

	projectID := uuid.New()
	args := mustJSON(t, map[string]any{
		"projectId": projectID.String(),
		"serviceId": uuid.New().String(),
		"name":      "x",
	})
	_, err := tool.Run(testCallerCtx(projectID), args)
	if err == nil || !strings.Contains(err.Error(), "method") {
		t.Fatalf("expected method/path requirement, got %v", err)
	}
	if cases.lastCreate != nil {
		t.Errorf("CaseService.CreateManual should not be called when params are invalid")
	}
}

func TestCreateCaseFromEndpoint_FillsDefaultsForRequestAndAssertions(t *testing.T) {
	t.Parallel()

	cases := &capturingCaseService{}
	tool := createCaseFromEndpointTool(Deps{Cases: cases})

	projectID := uuid.New()
	args := mustJSON(t, map[string]any{
		"projectId": projectID.String(),
		"serviceId": uuid.New().String(),
		"name":      "list users",
		"method":    "get",
		"path":      "/api/v1/users",
	})
	if _, err := tool.Run(testCallerCtx(projectID), args); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if cases.lastCreate == nil {
		t.Fatalf("CaseService.CreateManual was not invoked")
	}
	if cases.lastCreate.Method != "GET" {
		t.Errorf("method should be normalized to upper-case, got %q", cases.lastCreate.Method)
	}
	if string(cases.lastCreate.Request) != "{}" {
		t.Errorf("request default should be {}, got %s", cases.lastCreate.Request)
	}
	if string(cases.lastCreate.Assertions) != "[]" {
		t.Errorf("assertions default should be [], got %s", cases.lastCreate.Assertions)
	}
}

// capturingCaseService is a minimal CaseService implementation. The
// scenarios tests only exercise CreateManual; the other methods exist to
// satisfy the interface and panic if invoked unexpectedly so we get loud
// failures rather than silent zero values.
type capturingCaseService struct {
	lastCreate *testcase.CreateManualInput
}

func (c *capturingCaseService) Get(_ context.Context, _ uuid.UUID) (*testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.Get not implemented in test")
}

func (c *capturingCaseService) List(_ context.Context, _ testcase.ListFilter) ([]testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.List not implemented in test")
}

func (c *capturingCaseService) ListSaved(_ context.Context, _ uuid.UUID) ([]testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.ListSaved not implemented in test")
}

func (c *capturingCaseService) CreateSaved(_ context.Context, _ uuid.UUID, _ testcase.CreateSavedInput) (*testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.CreateSaved not implemented in test")
}

func (c *capturingCaseService) CreateManual(_ context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error) {
	captured := input
	c.lastCreate = &captured
	return &testcase.TestCase{
		ID:         uuid.New(),
		ProjectID:  input.ProjectID,
		ServiceID:  input.ServiceID,
		Name:       input.Name,
		Method:     input.Method,
		Path:       input.Path,
		Request:    input.Request,
		Assertions: input.Assertions,
		Status:     testcase.StatusActive,
	}, nil
}

func (c *capturingCaseService) Patch(_ context.Context, _ uuid.UUID, _ testcase.PatchInput) (*testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.Patch not implemented in test")
}

func (c *capturingCaseService) UpsertGenerated(_ context.Context, _ testcase.Draft) (*testcase.TestCase, error) {
	return nil, errors.New("capturingCaseService.UpsertGenerated not implemented in test")
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
