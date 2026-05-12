package projectprompt

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a prompt record cannot be located.
var ErrNotFound = errors.New("project ai prompt not found")

// Supported action values (must match the DB CHECK constraint).
const (
	ActionGenerateParams     = "generate_params"
	ActionGenerateAssertion  = "generate_assertion"
	ActionGenerateCaseData   = "generate_case_data"
	ActionAnalyzeFailure     = "analyze_failure"
	ActionAnalyzeSpecChanges = "analyze_spec_changes"
	ActionRaw                = "raw"
)

// ActionLabels maps each action value to a human-readable Chinese label.
var ActionLabels = map[string]string{
	ActionGenerateParams:     "生成请求参数",
	ActionGenerateAssertion:  "生成断言脚本",
	ActionGenerateCaseData:   "生成用例数据",
	ActionAnalyzeFailure:     "AI 分析失败原因",
	ActionAnalyzeSpecChanges: "AI 分析 Spec 变更",
	ActionRaw:                "通用对话",
}

// ProjectPrompt is the API-facing representation of a per-project prompt override.
type ProjectPrompt struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"projectId"`
	Action       string     `json:"action"`
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	DefaultModel string     `json:"defaultModel"`
	ProviderID   *uuid.UUID `json:"providerId,omitempty"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// CreateInput is the payload for POST /projects/{projectID}/ai-prompts.
type CreateInput struct {
	Action       string     `json:"action"`
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	DefaultModel string     `json:"defaultModel"`
	ProviderID   *uuid.UUID `json:"providerId,omitempty"`
	Enabled      *bool      `json:"enabled"`
}

// UpdateInput is the payload for PUT /projects/{projectID}/ai-prompts/{promptID}.
type UpdateInput struct {
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	DefaultModel string     `json:"defaultModel"`
	ProviderID   *uuid.UUID `json:"providerId,omitempty"`
	Enabled      *bool      `json:"enabled"`
}

func validAction(action string) bool {
	_, ok := ActionLabels[action]
	return ok
}
