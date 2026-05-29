package genagent

import (
	"autotest/internal/aiconfig"
	"autotest/internal/scenariogen"

	"github.com/google/uuid"
)

// LoginCredentialBundle carries user-confirmed login credentials for a gen job.
type LoginCredentialBundle = scenariogen.LoginCredentialBundle

// RunConfig controls the generate-and-verify agent loop.
type RunConfig struct {
	ProjectID        uuid.UUID `json:"projectId"`
	ServiceID        uuid.UUID `json:"serviceId"`
	EnvironmentID    uuid.UUID `json:"environmentId"`
	AutoRepair       bool      `json:"autoRepair"`
	MaxRounds        int       `json:"maxRounds"`
	StopOnRealDefect bool      `json:"stopOnRealDefect"`
	LoginCredentials *LoginCredentialBundle `json:"loginCredentials,omitempty"`
	CreatedBy        uuid.UUID `json:"createdBy,omitempty"`
}

// DefaultMaxRounds is used when RunConfig.MaxRounds is zero or negative.
const DefaultMaxRounds = aiconfig.DefaultScenarioGenMaxRounds

func (c RunConfig) NormalizedMaxRounds() int {
	if c.MaxRounds <= 0 {
		return DefaultMaxRounds
	}
	return c.MaxRounds
}

func (c RunConfig) normalizedMaxRounds() int {
	return c.NormalizedMaxRounds()
}
