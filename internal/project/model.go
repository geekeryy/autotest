package project

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ProjectRole string

const (
	ProjectRoleOwner     ProjectRole = "owner"
	ProjectRoleDeveloper ProjectRole = "developer"
	ProjectRoleViewer    ProjectRole = "viewer"
)

// projectRoleRank maps each role to a numeric rank for comparison.
var projectRoleRank = map[ProjectRole]int{
	ProjectRoleViewer:    1,
	ProjectRoleDeveloper: 2,
	ProjectRoleOwner:     3,
}

// ProjectRoleAtLeast returns true if role is at least as privileged as min.
func ProjectRoleAtLeast(role, min ProjectRole) bool {
	return projectRoleRank[role] >= projectRoleRank[min]
}

type Project struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	MyRole      *ProjectRole `json:"myRole,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type ProjectMember struct {
	ProjectID   uuid.UUID   `json:"projectId"`
	UserID      uuid.UUID   `json:"userId"`
	Username    string      `json:"username"`
	DisplayName string      `json:"displayName"`
	Role        ProjectRole `json:"role"`
	CreatedAt   time.Time   `json:"createdAt"`
}

type AddMemberInput struct {
	UserID uuid.UUID   `json:"userId"`
	Role   ProjectRole `json:"role"`
}

type UpdateMemberInput struct {
	Role ProjectRole `json:"role"`
}

type Service struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	McpEnabled   bool       `json:"mcpEnabled"`
	McpAPIKeyID  *uuid.UUID `json:"mcpApiKeyId,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Environment struct {
	ID        uuid.UUID       `json:"id"`
	ProjectID uuid.UUID       `json:"projectId"`
	ServiceID uuid.UUID       `json:"serviceId,omitempty"`
	Name      string          `json:"name"`
	BaseURL   string          `json:"baseUrl"`
	Variables json.RawMessage `json:"variables,omitempty"`
	Auth      json.RawMessage `json:"auth,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type CreateProjectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateServiceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	McpEnabled  bool   `json:"mcpEnabled"`
}

type UpdateServiceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	McpEnabled  bool   `json:"mcpEnabled"`
}

type CreateEnvironmentInput struct {
	Name      string          `json:"name"`
	BaseURL   string          `json:"baseUrl"`
	Variables json.RawMessage `json:"variables"`
	Auth      json.RawMessage `json:"auth"`
}

type UpdateEnvironmentInput struct {
	Name      string          `json:"name"`
	BaseURL   string          `json:"baseUrl"`
	Variables json.RawMessage `json:"variables"`
	Auth      json.RawMessage `json:"auth"`
}
