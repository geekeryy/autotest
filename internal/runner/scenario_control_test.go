package runner

import (
	"context"
	"encoding/json"
	"testing"

	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

func TestEvaluateConditionRendersVariablesAndStepRefs(t *testing.T) {
	t.Parallel()

	stepOutputs := map[int]any{
		1: map[string]any{
			"response": map[string]any{
				"status": 201,
				"body": map[string]any{
					"count": 3,
				},
			},
		},
	}

	matched, detail, err := evaluateCondition(conditionStepConfig{
		Left:     "{{$steps[1].response.status}}",
		Operator: "greater_or_equal",
		Right:    "{{minStatus}}",
	}, map[string]string{"minStatus": "200"}, nil, stepOutputs, nil)
	if err != nil {
		t.Fatalf("evaluate condition: %v", err)
	}
	if !matched {
		t.Fatalf("expected condition to match, detail=%#v", detail)
	}

	matched, _, err = evaluateCondition(conditionStepConfig{
		Left:     "{{$steps[1].response.body.count}}",
		Operator: "less_than",
		Right:    "3",
	}, nil, nil, stepOutputs, nil)
	if err != nil {
		t.Fatalf("evaluate numeric condition: %v", err)
	}
	if matched {
		t.Fatalf("expected condition to be false")
	}
}

func TestConditionConfigSupportsOrderedBranches(t *testing.T) {
	t.Parallel()

	cfg := conditionStepConfig{
		Branches: []conditionBranchConfig{
			{Left: "{{status}}", Operator: "equals", Right: "201", StepSeqs: []int{2}},
			{Left: "{{status}}", Operator: "equals", Right: "200", StepSeqs: []int{3, 3}},
		},
		ElseStepSeqs: []int{4},
	}

	branches := cfg.resolvedBranches()
	if len(branches) != 2 || len(branches[1].StepSeqs) != 1 || branches[1].StepSeqs[0] != 3 {
		t.Fatalf("unexpected resolved branches: %#v", branches)
	}

	var picked = -1
	var selected []int
	for i, br := range branches {
		matched, _, err := evaluateCondition(conditionStepConfig{
			Left:     br.Left,
			Operator: br.Operator,
			Right:    br.Right,
		}, map[string]string{"status": "200"}, nil, nil, nil)
		if err != nil {
			t.Fatalf("evaluate branch %d: %v", i, err)
		}
		if matched {
			picked = i
			selected = br.StepSeqs
			break
		}
	}
	if picked != 1 || len(selected) != 1 || selected[0] != 3 {
		t.Fatalf("expected second branch to be selected, picked=%d selected=%#v", picked, selected)
	}

	skipped := skippedConditionSeqs(branches, cfg.resolvedElseSeqs(), picked, false)
	if len(skipped) != 2 || skipped[0] != 2 || skipped[1] != 4 {
		t.Fatalf("unexpected skipped steps: %#v", skipped)
	}
}

func TestBuildLoopItemsSupportsCountAndArrayExpressions(t *testing.T) {
	t.Parallel()

	items, mode, err := buildLoopItems(forStepConfig{
		Mode:            "count",
		CountExpression: "{{count}}",
		MaxIterations:   5,
	}, map[string]string{"count": "3"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("build count loop: %v", err)
	}
	if mode != "count" || len(items) != 3 {
		t.Fatalf("unexpected count loop mode=%s items=%#v", mode, items)
	}

	stepOutputs := map[int]any{
		1: map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"items": []any{
						map[string]any{"id": "u1"},
						map[string]any{"id": "u2"},
					},
				},
			},
		},
	}
	items, mode, err = buildLoopItems(forStepConfig{
		Mode:            "array",
		ItemsExpression: "{{$steps[1].response.body.items}}",
		MaxIterations:   5,
	}, nil, nil, stepOutputs, nil)
	if err != nil {
		t.Fatalf("build array loop: %v", err)
	}
	if mode != "array" || len(items) != 2 {
		t.Fatalf("unexpected array loop mode=%s items=%#v", mode, items)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != "u1" {
		t.Fatalf("expected first item to be decoded object, got %#v", items[0])
	}
}

func TestBuildLoopItemsUnwrapsCommonObjectArrayFields(t *testing.T) {
	t.Parallel()

	items, mode, err := buildLoopItems(forStepConfig{
		Mode:            "array",
		ItemsExpression: `{"data":[{"id":"u1"},{"id":"u2"}],"total":2}`,
		MaxIterations:   5,
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build wrapped array loop: %v", err)
	}
	if mode != "array" || len(items) != 2 {
		t.Fatalf("unexpected wrapped array loop mode=%s items=%#v", mode, items)
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != "u1" {
		t.Fatalf("expected first wrapped item, got %#v", items[0])
	}
}

func TestBuildLoopItemsRejectsTooManyIterations(t *testing.T) {
	t.Parallel()

	_, _, err := buildLoopItems(forStepConfig{
		Mode:          "count",
		Count:         3,
		MaxIterations: 2,
	}, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected loop limit error")
	}
}

func TestApplyLoopVariablesFlattensAndRestoresOnlyLoopVars(t *testing.T) {
	t.Parallel()

	exec := &scenarioExecution{
		baseVars: map[string]string{
			"item": "old",
			"keep": "value",
		},
	}
	restore := exec.applyLoopVariables(forStepConfig{
		ItemVar:  "item",
		IndexVar: "i",
	}, 2, map[string]any{"id": "u1", "active": true})

	if exec.baseVars["item.id"] != "u1" || exec.baseVars["item.active"] != "true" || exec.baseVars["i"] != "2" {
		t.Fatalf("loop vars not applied: %#v", exec.baseVars)
	}
	exec.baseVars["createdByChild"] = "yes"
	restore()

	if exec.baseVars["item"] != "old" || exec.baseVars["keep"] != "value" {
		t.Fatalf("expected original vars to be restored: %#v", exec.baseVars)
	}
	if _, ok := exec.baseVars["item.id"]; ok {
		t.Fatalf("expected flattened loop var to be removed: %#v", exec.baseVars)
	}
	if exec.baseVars["createdByChild"] != "yes" {
		t.Fatalf("expected non-loop child variable to remain: %#v", exec.baseVars)
	}
}

func TestCollectControlledStepSeqs(t *testing.T) {
	t.Parallel()

	forCfg, _ := json.Marshal(forStepConfig{BodyStepSeqs: []int{2, 3}})
	conditionCfg, _ := json.Marshal(conditionStepConfig{
		Branches: []conditionBranchConfig{
			{StepSeqs: []int{4}},
			{StepSeqs: []int{5}},
		},
		ElseStepSeqs: []int{6},
	})
	controlled := collectControlledStepSeqs([]scenario.Step{
		{StepSeq: 1, StepType: scenario.StepTypeFor, Config: forCfg},
		{StepSeq: 7, StepType: scenario.StepTypeCondition, Config: conditionCfg},
	})
	for _, seq := range []int{2, 3, 4, 5, 6} {
		if !controlled[seq] {
			t.Fatalf("expected step #%d to be controlled: %#v", seq, controlled)
		}
	}
	if controlled[1] || controlled[7] {
		t.Fatalf("control steps themselves should not be marked controlled: %#v", controlled)
	}
}

func TestCollectReachableControlSeqsIncludesNestedChildren(t *testing.T) {
	t.Parallel()

	innerForCfg, _ := json.Marshal(forStepConfig{BodyStepSeqs: []int{3}})
	outerForCfg, _ := json.Marshal(forStepConfig{BodyStepSeqs: []int{2}})
	steps := []scenario.Step{
		{StepSeq: 1, StepType: scenario.StepTypeFor, Config: outerForCfg},
		{StepSeq: 2, StepType: scenario.StepTypeFor, Config: innerForCfg},
		{StepSeq: 3, StepType: scenario.StepTypeAPI},
	}
	exec := newScenarioExecution(nil, uuid.New(), project.Environment{}, nil, steps, nil)
	seqs := exec.collectReachableControlSeqs(steps[0], map[int]bool{})
	if len(seqs) != 2 || seqs[0] != 2 || seqs[1] != 3 {
		t.Fatalf("unexpected reachable seqs: %#v", seqs)
	}
}

func TestRunMarksDisabledControlSubtreeSkipped(t *testing.T) {
	t.Parallel()

	forCfg, _ := json.Marshal(forStepConfig{BodyStepSeqs: []int{2}})
	steps := []scenario.Step{
		{StepSeq: 1, StepOrder: 1, StepType: scenario.StepTypeFor, Enabled: false, Config: forCfg},
		{StepSeq: 2, StepOrder: 2, StepType: scenario.StepTypeAPI, Enabled: true},
	}
	exec := newScenarioExecution(nil, uuid.New(), project.Environment{}, map[string]string{}, steps, nil)
	if status := exec.run(context.Background()); status != report.RunPassed {
		t.Fatalf("expected run passed, got %s", status)
	}
	found := false
	for _, sr := range exec.stepResults {
		if sr.Step.StepSeq == 2 && sr.Output != nil && sr.Output["skipped"] == true {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skipped child step, results=%#v", exec.stepResults)
	}
}
