package genagent

import (
	"autotest/internal/aiconfig"

	"github.com/google/uuid"
)

// ScenarioAutorunEnabled reports whether real-environment scenario verify is allowed.
func ScenarioAutorunEnabled() bool {
	return aiconfig.ScenarioAutorunEnabled()
}

// AssertRunAllowed verifies feature flag and environment requirements.
func AssertRunAllowed(cfg RunConfig) error {
	if !ScenarioAutorunEnabled() {
		return errAutorunDisabled
	}
	if cfg.EnvironmentID == uuid.Nil {
		return errEnvironmentRequired
	}
	if cfg.ServiceID == uuid.Nil {
		return errServiceRequired
	}
	return nil
}
