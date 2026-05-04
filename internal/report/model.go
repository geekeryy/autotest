package report

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RunStatus string

const (
	RunQueued  RunStatus = "queued"
	RunRunning RunStatus = "running"
	RunPassed  RunStatus = "passed"
	RunFailed  RunStatus = "failed"
)

type ResultStatus string

const (
	ResultPassed ResultStatus = "passed"
	ResultFailed ResultStatus = "failed"
	ResultError  ResultStatus = "error"
)

type Run struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name,omitempty"`
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	SuiteID       *uuid.UUID      `json:"suiteId,omitempty"`
	EnvironmentID *uuid.UUID      `json:"environmentId,omitempty"`
	Status        RunStatus       `json:"status"`
	Variables     json.RawMessage `json:"variables"`
	Snapshot      json.RawMessage `json:"snapshot"`
	StartedAt     *time.Time      `json:"startedAt,omitempty"`
	FinishedAt    *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type CreateRunInput struct {
	Name          string          `json:"name"`
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	SuiteID       *uuid.UUID      `json:"suiteId,omitempty"`
	EnvironmentID uuid.UUID       `json:"environmentId"`
	Variables     json.RawMessage `json:"variables"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

type Result struct {
	ID               uuid.UUID       `json:"id"`
	RunID            uuid.UUID       `json:"runId"`
	TestCaseID       uuid.UUID       `json:"testCaseId"`
	StepID           *uuid.UUID      `json:"stepId,omitempty"`
	Status           ResultStatus    `json:"status"`
	DurationMillis   int64           `json:"durationMillis"`
	RequestSnapshot  json.RawMessage `json:"requestSnapshot"`
	ResponseSnapshot json.RawMessage `json:"responseSnapshot"`
	Assertions       json.RawMessage `json:"assertions"`
	Error            string          `json:"error,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}
