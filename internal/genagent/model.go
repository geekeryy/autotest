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

// FailureEvidence is the structured context sent to the LLM for failure classification.
type FailureEvidence struct {
	ScenarioName string `json:"scenarioName"`
	StepName     string `json:"stepName"`
	StepOrder    int    `json:"stepOrder"`
	StepType     string `json:"stepType"`

	StepConfig          json.RawMessage `json:"stepConfig,omitempty"`
	StepRequestOverride json.RawMessage `json:"stepRequestOverride,omitempty"`

	Status           string          `json:"status"`
	Error            string          `json:"error,omitempty"`
	Assertions       json.RawMessage `json:"assertions,omitempty"`
	RequestSnapshot  json.RawMessage `json:"requestSnapshot,omitempty"`
	ResponseSnapshot json.RawMessage `json:"responseSnapshot,omitempty"`

	StepOutput          map[string]any      `json:"stepOutput,omitempty"`
	PreviousStepOutputs []StepOutputSummary  `json:"previousStepOutputs,omitempty"`
	AssertionDiffs      []AssertionDiff      `json:"assertionDiffs,omitempty"`
	PreviousRepairs     []RepairAttempt      `json:"previousRepairs,omitempty"`

	// Endpoint metadata for context-enriched repair. Helps the LLM
	// understand what the endpoint expects and generate better fixes.
	EndpointSummary       string   `json:"endpointSummary,omitempty"`
	EndpointRequiredFields []string `json:"endpointRequiredFields,omitempty"`
	EndpointEnumFields    map[string][]any `json:"endpointEnumFields,omitempty"`
}

// StepOutputSummary is a compact representation of a prior step's output,
// included so the LLM can trace variable references like $steps[1].output.token.
type StepOutputSummary struct {
	StepOrder int            `json:"stepOrder"`
	StepName  string         `json:"stepName"`
	Status    string         `json:"status"`
	Output    map[string]any `json:"output,omitempty"`
}

// AssertionDiff captures the expected vs actual for a single failed assertion.
type AssertionDiff struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message,omitempty"`
}

// RepairAttempt records a previous repair action for cross-round memory.
type RepairAttempt struct {
	Round    int    `json:"round"`
	Category string `json:"category"`
	Action   string `json:"action"`
	Detail   string `json:"detail,omitempty"`
	Success  bool   `json:"success"`
}

// JobRepository persists scenario_gen_jobs.
type JobRepository interface {
	Create(ctx context.Context, cfg RunConfig) (*Job, error)
	Update(ctx context.Context, jobID uuid.UUID, status JobStatus, rounds int, result json.RawMessage) error
	Get(ctx context.Context, projectID, jobID uuid.UUID) (*Job, error)
}
