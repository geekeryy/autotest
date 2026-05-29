package genagent

import (
	"context"
	"encoding/json"
	"time"

	"autotest/internal/runner"

	"github.com/google/uuid"
)

// JobStatus enumerates persisted job states.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobStopped   JobStatus = "stopped"
)

// Job is a scenario generation + verify task.
type Job struct {
	ID            uuid.UUID       `json:"id"`
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	EnvironmentID uuid.UUID       `json:"environmentId"`
	Config        json.RawMessage `json:"config"`
	Status        JobStatus       `json:"status"`
	Rounds        int             `json:"rounds"`
	Result        json.RawMessage `json:"result,omitempty"`
	CreatedBy     *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// RoundResult captures one execute/repair iteration.
type RoundResult struct {
	Round          int              `json:"round"`
	ScenariosTotal int              `json:"scenariosTotal"`
	Passed         int              `json:"passed"`
	Failed         int              `json:"failed"`
	Repairs        []RepairSummary  `json:"repairs,omitempty"`
	StoppedReason  string           `json:"stoppedReason,omitempty"`
	ScenarioRuns   []ScenarioRunRef `json:"scenarioRuns,omitempty"`
}

// ScenarioRunRef links a scenario to its latest run outcome.
type ScenarioRunRef struct {
	ScenarioID   uuid.UUID                  `json:"scenarioId"`
	ScenarioName string                     `json:"scenarioName"`
	RunID        uuid.UUID                  `json:"runId,omitempty"`
	Status       string                     `json:"status"`
	Failures     []string                   `json:"failures,omitempty"`
	Output       *runner.RunScenarioOutput  `json:"-"`
}

// RepairSummary describes an automatic fix attempt.
type RepairSummary struct {
	ScenarioID uuid.UUID `json:"scenarioId"`
	StepID     uuid.UUID `json:"stepId,omitempty"`
	Category   string    `json:"category"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail,omitempty"`
}

// JobResult is the final aggregated outcome stored in result jsonb.
type JobResult struct {
	Rounds         []RoundResult `json:"rounds"`
	FinalPassRate  float64       `json:"finalPassRate"`
	Scenarios      int           `json:"scenarios"`
	RealDefects    int           `json:"realDefects"`
	CoverageResult json.RawMessage `json:"coverageResult,omitempty"`
}

// JobRepository persists scenario_gen_jobs.
type JobRepository interface {
	Create(ctx context.Context, cfg RunConfig) (*Job, error)
	Update(ctx context.Context, jobID uuid.UUID, status JobStatus, rounds int, result json.RawMessage) error
	Get(ctx context.Context, projectID, jobID uuid.UUID) (*Job, error)
}
