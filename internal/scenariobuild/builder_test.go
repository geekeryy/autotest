package scenariobuild

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"autotest/internal/scenario"

	"github.com/google/uuid"
)

func TestValidateStepOrders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		steps   []StepInput
		wantErr string
	}{
		{
			name: "ok unique 1-based",
			steps: []StepInput{
				{StepOrder: 1, StepType: "api", Name: "a"},
				{StepOrder: 2, StepType: "api", Name: "b"},
			},
		},
		{
			name: "duplicate stepOrder",
			steps: []StepInput{
				{StepOrder: 1, StepType: "api", Name: "a"},
				{StepOrder: 1, StepType: "api", Name: "b"},
			},
			wantErr: "stepOrder=1",
		},
		{
			name: "zero is invalid",
			steps: []StepInput{
				{StepOrder: 0, StepType: "api", Name: "a"},
			},
			wantErr: ">= 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStepOrders(tc.steps)
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

	_, err := BuildUpsertInput(StepInput{StepOrder: 1, StepType: "api", Name: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "testCaseId") {
		t.Fatalf("expected testCaseId required, got %v", err)
	}

	id := uuid.New()
	in, err := BuildUpsertInput(StepInput{
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

	in, err := BuildUpsertInput(StepInput{
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

	out, rewritten, err := RewriteControlFlowConfig("for", cfg, orderToSeq)
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
	out, rewritten, err := RewriteControlFlowConfig("condition", cfg, orderToSeq)
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
	if _, _, err := RewriteControlFlowConfig("for", cfg, orderToSeq); err == nil ||
		!strings.Contains(err.Error(), "99") {
		t.Fatalf("expected unknown stepOrder error, got %v", err)
	}
}

func TestRewriteControlFlowConfig_NoopWhenNoOrdersFields(t *testing.T) {
	t.Parallel()

	cfg := json.RawMessage(`{"bodyStepSeqs":[12,13],"mode":"count"}`)
	out, rewritten, err := RewriteControlFlowConfig("for", cfg, map[int]int{})
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

type fakeScenarioAPI struct {
	scenarios map[uuid.UUID]*scenario.Scenario
	stepsByOS map[uuid.UUID]map[int]*scenario.Step
	nextSeq   int
}

func newFakeScenarioAPI() *fakeScenarioAPI {
	return &fakeScenarioAPI{
		scenarios: map[uuid.UUID]*scenario.Scenario{},
		stepsByOS: map[uuid.UUID]map[int]*scenario.Step{},
	}
}

func (f *fakeScenarioAPI) Create(_ context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error) {
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

func (f *fakeScenarioAPI) UpsertStep(_ context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error) {
	slots := f.stepsByOS[scenarioID]
	existing, hit := slots[input.StepOrder]
	if hit {
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
	return step, nil
}

func TestCreateScenarioWithSteps_ControlFlowRewrite(t *testing.T) {
	t.Parallel()

	api := newFakeScenarioAPI()
	caseA := uuid.New()
	caseB := uuid.New()
	projectID := uuid.New()
	serviceID := uuid.New()

	result, err := CreateScenarioWithSteps(context.Background(), api, CreateInput{
		ProjectID: projectID,
		ServiceID: serviceID,
		Name:      "checkout flow",
		Steps: []StepInput{
			{StepOrder: 1, StepType: "api", Name: "login", TestCaseID: caseA.String()},
			{StepOrder: 2, StepType: "api", Name: "create", TestCaseID: caseB.String()},
			{
				StepOrder: 3,
				StepType:  "for",
				Name:      "retry create",
				Config:    json.RawMessage(`{"mode":"count","count":2,"bodyStepOrders":[2]}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateScenarioWithSteps: %v", err)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}
	var forStep *scenario.Step
	for i := range result.Steps {
		if result.Steps[i].StepType == scenario.StepTypeFor {
			forStep = &result.Steps[i]
			break
		}
	}
	if forStep == nil {
		t.Fatal("for step missing")
	}
	var cfg map[string]any
	if err := json.Unmarshal(forStep.Config, &cfg); err != nil {
		t.Fatalf("for config: %v", err)
	}
	if _, ok := cfg["bodyStepOrders"]; ok {
		t.Errorf("bodyStepOrders should be rewritten: %s", forStep.Config)
	}
	seqs, _ := cfg["bodyStepSeqs"].([]any)
	if len(seqs) != 1 {
		t.Fatalf("bodyStepSeqs: %s", forStep.Config)
	}
	var apiSeq int
	for _, s := range result.Steps {
		if s.StepOrder == 2 {
			apiSeq = s.StepSeq
		}
	}
	if int(seqs[0].(float64)) != apiSeq {
		t.Errorf("bodyStepSeqs[0]=%v stepOrder=2 seq=%d", seqs[0], apiSeq)
	}
}
