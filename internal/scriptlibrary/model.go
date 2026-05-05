package scriptlibrary

import (
	"time"

	"github.com/google/uuid"
)

// Template is a script snippet for assertion editors and scenario script steps.
// Built-in templates are global (ProjectID nil, IsBuiltin true); custom templates belong to a project.
type Template struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   *uuid.UUID `json:"projectId,omitempty"`
	IsBuiltin   bool       `json:"isBuiltin"`
	BuiltinKey  string     `json:"builtinKey,omitempty"`
	SortOrder   int        `json:"sortOrder"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Description string     `json:"description,omitempty"`
	Code        string     `json:"code"`
	Scopes      []string   `json:"scopes"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// CreateInput is used for POST /script-library-templates.
type CreateInput struct {
	ProjectID   uuid.UUID `json:"projectId"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Code        string    `json:"code"`
	Scopes      []string  `json:"scopes"`
}

// UpdateInput is used for PUT /script-library-templates/{id}.
type UpdateInput struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Code        string   `json:"code"`
	Scopes      []string `json:"scopes"`
}
