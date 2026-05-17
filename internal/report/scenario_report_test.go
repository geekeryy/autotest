package report

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildScenarioRunReportSummary(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	finish := time.Now()
	stepID := uuid.New()
	run := &Run{
		ID:         uuid.New(),
		Name:       "demo",
		Status:     RunFailed,
		StartedAt:  &start,
		FinishedAt: &finish,
		Snapshot:   []byte(`{"scenarioName":"S1","environmentName":"dev"}`),
	}
	results := []Result{
		{ID: uuid.New(), Status: ResultPassed, DurationMillis: 10, StepID: &stepID},
		{ID: uuid.New(), Status: ResultFailed, DurationMillis: 20, StepID: &stepID},
	}
	index := map[uuid.UUID]StepMeta{
		stepID: {StepSeq: 1, StepOrder: 1, Name: "login", StepType: "api"},
	}
	rep := BuildScenarioRunReport(run, results, index)
	if rep.Summary.Passed != 1 || rep.Summary.Failed != 1 || rep.Summary.TotalSteps != 2 {
		t.Fatalf("unexpected summary: %+v", rep.Summary)
	}
	if rep.Summary.DurationMillis <= 0 {
		t.Fatalf("expected positive duration")
	}
	if rep.RunSnapshot["scenarioName"] != "S1" {
		t.Fatalf("snapshot: %+v", rep.RunSnapshot)
	}
	if len(rep.StepResults) != 2 || rep.StepResults[0].Step == nil {
		t.Fatalf("step results: %+v", rep.StepResults)
	}
}

func TestExportJSONAndHTML(t *testing.T) {
	run := &Run{ID: uuid.New(), Name: "r", Status: RunPassed}
	rep := BuildScenarioRunReport(run, nil, nil)
	j, err := ExportJSON(&rep)
	if err != nil || len(j) == 0 {
		t.Fatalf("export json: %v", err)
	}
	h, err := ExportScenarioRunHTML(&rep)
	if err != nil || len(h) == 0 {
		t.Fatalf("export html: %v", err)
	}
	if !json.Valid(j) {
		t.Fatal("invalid json export")
	}
}

func TestBuildResultSummaryEntry(t *testing.T) {
	stepID := uuid.New()
	raw, _ := json.Marshal(map[string]any{"body": string(make([]byte, 9000))})
	res := Result{StepID: &stepID, RequestSnapshot: raw}
	entry := BuildResultSummaryEntry(res, map[uuid.UUID]StepMeta{stepID: {StepSeq: 2}}, 100)
	snap, ok := entry["requestSnapshot"].(map[string]any)
	if !ok {
		t.Fatalf("missing snapshot: %#v", entry)
	}
	body, _ := snap["body"].(string)
	if len(body) > 200 {
		t.Fatalf("expected truncated body")
	}
}
