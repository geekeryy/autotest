package mcp

import (
	"context"

	"autotest/internal/scenario"
	"autotest/internal/scenariobuild"

	"github.com/google/uuid"
)

// HTTPScenarioAPI adapts APIClient to scenariobuild.ScenarioAPI and full scenario CRUD.
type HTTPScenarioAPI struct {
	client *APIClient
}

func NewHTTPScenarioAPI(client *APIClient) *HTTPScenarioAPI {
	return &HTTPScenarioAPI{client: client}
}

func (a *HTTPScenarioAPI) Create(ctx context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error) {
	return a.client.CreateScenario(ctx, input)
}

func (a *HTTPScenarioAPI) UpsertStep(ctx context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error) {
	return a.client.UpsertScenarioStep(ctx, scenarioID, input.StepOrder, input)
}

// CreateScenarioWithSteps creates a scenario and all steps via the shared builder.
func (a *HTTPScenarioAPI) CreateScenarioWithSteps(ctx context.Context, in scenariobuild.CreateInput) (*scenariobuild.Result, error) {
	return scenariobuild.CreateScenarioWithSteps(ctx, a, in)
}
