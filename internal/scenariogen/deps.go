package scenariogen

import (
	"context"

	testcase "autotest/internal/case"
	"autotest/internal/generator"
	"autotest/internal/scenario"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// Deps wires domain services for coverage scenario generation.
type Deps struct {
	Cases     CaseService
	Scenarios ScenarioService
	Specs     SpecRepository
	Generator *generator.Generator
}

type CaseService interface {
	List(ctx context.Context, filter testcase.ListFilter) ([]testcase.TestCase, error)
	Get(ctx context.Context, testCaseID uuid.UUID) (*testcase.TestCase, error)
	CreateManual(ctx context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error)
	UpsertGenerated(ctx context.Context, draft testcase.Draft) (*testcase.TestCase, error)
}

type ScenarioService interface {
	Create(ctx context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error)
	UpsertStep(ctx context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error)
	ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error)
}

type SpecRepository interface {
	ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.Endpoint, error)
	GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*spec.Endpoint, error)
}
