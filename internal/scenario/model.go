package scenario

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrScenarioNotFound = errors.New("scenario not found")
	ErrStepNotFound     = errors.New("scenario step not found")
)

// StepType identifies the executable kind of a scenario step.
type StepType string

const (
	StepTypeAPI       StepType = "api"
	StepTypeDatabase  StepType = "database"
	StepTypeScript    StepType = "script"
	StepTypeFor       StepType = "for"
	StepTypeCondition StepType = "condition"
)

type Scenario struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	ServiceID   uuid.UUID `json:"serviceId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Step struct {
	ID              uuid.UUID       `json:"id"`
	ScenarioID      uuid.UUID       `json:"scenarioId"`
	TestCaseID      uuid.UUID       `json:"testCaseId,omitempty"`
	StepSeq         int             `json:"stepSeq"`
	StepOrder       int             `json:"stepOrder"`
	StepType        StepType        `json:"stepType"`
	Name            string          `json:"name,omitempty"`
	Enabled         bool            `json:"enabled"`
	Config          json.RawMessage `json:"config,omitempty"`
	RequestOverride json.RawMessage `json:"requestOverride,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// ── CRUD inputs ────────────────────────────────────────────────────────────

type CreateScenarioInput struct {
	ProjectID   uuid.UUID `json:"projectId"`
	ServiceID   uuid.UUID `json:"serviceId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
}

type UpdateScenarioInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpsertStepInput struct {
	TestCaseID      uuid.UUID       `json:"testCaseId,omitempty"`
	StepOrder       int             `json:"stepOrder"`
	StepType        StepType        `json:"stepType"`
	Name            string          `json:"name"`
	Enabled         *bool           `json:"enabled"`
	Config          json.RawMessage `json:"config"`
	RequestOverride json.RawMessage `json:"requestOverride"`
	// If set, after this step is saved, append its step_seq to the parent control step's config.
	AttachUnderParent *AttachUnderParentInput `json:"attachUnderParent,omitempty"`
}

type ListFilter struct {
	ProjectID uuid.UUID
	ServiceID uuid.UUID
}

// ReorderStepsInput is the body for PUT .../steps/reorder.
type ReorderStepsInput struct {
	StepIDs []uuid.UUID `json:"stepIds"`
}
