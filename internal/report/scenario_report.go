package report

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
)

// StepMeta describes a scenario step for labeling run results.
type StepMeta struct {
	StepSeq    int        `json:"stepSeq"`
	StepOrder  int        `json:"stepOrder"`
	Name       string     `json:"name,omitempty"`
	StepType   string     `json:"stepType"`
	TestCaseID *uuid.UUID `json:"testCaseId,omitempty"`
}

// RunSummary aggregates counts for a scenario run report.
type RunSummary struct {
	Passed         int   `json:"passed"`
	Failed         int   `json:"failed"`
	Error          int   `json:"error"`
	TotalSteps     int   `json:"totalSteps"`
	DurationMillis int64 `json:"durationMillis"`
}

// ScenarioStepResult is one executed step entry in a scenario report.
type ScenarioStepResult struct {
	Result     Result          `json:"result"`
	Step       *StepMeta       `json:"step,omitempty"`
	Assertions []any           `json:"assertions,omitempty"`
	Output     map[string]any    `json:"output,omitempty"`
	StepErrors []string          `json:"stepErrors,omitempty"`
}

// ScenarioRunReport is the full report payload for a scenario test run.
type ScenarioRunReport struct {
	Run         Run                  `json:"run"`
	Summary     RunSummary           `json:"summary"`
	RunSnapshot map[string]any       `json:"runSnapshot,omitempty"`
	StepResults []ScenarioStepResult `json:"stepResults"`
}

// LoadScenarioStepIndex loads active steps for a scenario keyed by step ID.
func (r *Repository) LoadScenarioStepIndex(ctx context.Context, scenarioID uuid.UUID) (map[uuid.UUID]StepMeta, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, test_case_id, step_seq, step_order, name, step_type
		from test_scenario_steps
		where scenario_id = $1 and deleted_at is null
	`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("load scenario steps: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID]StepMeta{}
	for rows.Next() {
		var id uuid.UUID
		var testCaseID *uuid.UUID
		var meta StepMeta
		if err := rows.Scan(&id, &testCaseID, &meta.StepSeq, &meta.StepOrder, &meta.Name, &meta.StepType); err != nil {
			return nil, fmt.Errorf("scan scenario step: %w", err)
		}
		meta.TestCaseID = testCaseID
		out[id] = meta
	}
	return out, rows.Err()
}

// BuildScenarioRunReport assembles a scenario run report from persisted data.
func BuildScenarioRunReport(run *Run, results []Result, stepIndex map[uuid.UUID]StepMeta) ScenarioRunReport {
	summary := RunSummary{TotalSteps: len(results)}
	for _, res := range results {
		switch res.Status {
		case ResultPassed:
			summary.Passed++
		case ResultFailed:
			summary.Failed++
		case ResultError:
			summary.Error++
		}
	}
	if run.StartedAt != nil && run.FinishedAt != nil {
		summary.DurationMillis = run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}

	var snap map[string]any
	if len(run.Snapshot) > 0 {
		_ = json.Unmarshal(run.Snapshot, &snap)
	}

	stepResults := make([]ScenarioStepResult, 0, len(results))
	for _, res := range results {
		entry := ScenarioStepResult{Result: res}
		if res.StepID != nil {
			if meta, ok := stepIndex[*res.StepID]; ok {
				m := meta
				entry.Step = &m
			}
		}
		if assertions := DecodeJSONAny(res.Assertions); assertions != nil {
			if slice, ok := assertions.([]any); ok {
				entry.Assertions = slice
			}
		}
		stepResults = append(stepResults, entry)
	}

	return ScenarioRunReport{
		Run:         *run,
		Summary:     summary,
		RunSnapshot: snap,
		StepResults: stepResults,
	}
}

// DecodeJSONAny unmarshals raw JSON into a generic value.
func DecodeJSONAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

// GetScenarioRunReport loads and builds a full scenario report.
func (r *Repository) GetScenarioRunReport(ctx context.Context, runID uuid.UUID) (*ScenarioRunReport, error) {
	run, err := r.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.ScenarioID == nil || *run.ScenarioID == uuid.Nil {
		return nil, ErrNotScenarioRun
	}
	results, err := r.ListResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	stepIndex, err := r.LoadScenarioStepIndex(ctx, *run.ScenarioID)
	if err != nil {
		return nil, err
	}
	report := BuildScenarioRunReport(run, results, stepIndex)
	return &report, nil
}

var ErrNotScenarioRun = fmt.Errorf("not a scenario run")
