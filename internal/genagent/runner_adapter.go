package genagent

import (
	"context"

	"autotest/internal/runner"

	"github.com/google/uuid"
)

// RunnerAdapter wraps runner.Service with a fixed scenario repository.
type RunnerAdapter struct {
	Svc  *runner.Service
	Repo scenarioRepository
}

func (a *RunnerAdapter) RunScenario(ctx context.Context, scenarioID uuid.UUID, input runner.RunScenarioInput) (*runner.RunScenarioOutput, error) {
	return a.Svc.RunScenario(ctx, a.Repo, scenarioID, input)
}
