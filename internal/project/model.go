package project

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Service struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
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
}

type UpdateServiceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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
