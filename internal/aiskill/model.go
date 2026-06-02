// Package aiskill implements a reusable AI workflow template system.
// Skills are prompt + tool combinations that can be triggered by keywords,
// page context, or planner intent.
package aiskill

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Skill represents a reusable AI workflow template.
type Skill struct {
	ID             uuid.UUID       `json:"id"`
	ProjectID      *uuid.UUID      `json:"projectId,omitempty"` // NULL = global skill
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Description    string          `json:"description"`
	PromptTemplate string          `json:"promptTemplate"`
	ToolDomains    []string        `json:"toolDomains,omitempty"`
	InputSchema    json.RawMessage `json:"inputSchema,omitempty"`
	RequireConfirm *bool           `json:"requireConfirm,omitempty"`
	Triggers       []SkillTrigger  `json:"triggers,omitempty"`
	CreatedBy      *uuid.UUID      `json:"createdBy,omitempty"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// SkillTrigger defines when a skill should be automatically matched.
type SkillTrigger struct {
	Type    string `json:"type"`    // "keyword", "page", "intent"
	Pattern string `json:"pattern"` // Match pattern
}

// CreateSkillInput is the payload for creating a skill.
type CreateSkillInput struct {
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Description    string          `json:"description"`
	PromptTemplate string          `json:"promptTemplate"`
	ToolDomains    []string        `json:"toolDomains,omitempty"`
	InputSchema    json.RawMessage `json:"inputSchema,omitempty"`
	RequireConfirm *bool           `json:"requireConfirm,omitempty"`
	Triggers       []SkillTrigger  `json:"triggers,omitempty"`
}

// UpdateSkillInput is the payload for updating a skill.
type UpdateSkillInput struct {
	Label          *string         `json:"label,omitempty"`
	Description    *string         `json:"description,omitempty"`
	PromptTemplate *string         `json:"promptTemplate,omitempty"`
	ToolDomains    []string        `json:"toolDomains,omitempty"`
	InputSchema    json.RawMessage `json:"inputSchema,omitempty"`
	RequireConfirm *bool           `json:"requireConfirm,omitempty"`
	Triggers       []SkillTrigger  `json:"triggers,omitempty"`
	Enabled        *bool           `json:"enabled,omitempty"`
}

// ListSkillsOptions controls skill listing.
type ListSkillsOptions struct {
	Enabled   *bool  `json:"enabled,omitempty"`
	ProjectID *uuid.UUID `json:"projectId,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

// PresetSkill represents a built-in skill definition.
type PresetSkill struct {
	Name           string
	Label          string
	Description    string
	PromptTemplate string
	ToolDomains    []string
	Triggers       []SkillTrigger
}
