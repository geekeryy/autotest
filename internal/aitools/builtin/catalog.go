// catalog.go exposes the complete domain tool universe for catalog and
// embedding precomputation. Several tools are registered conditionally on a
// non-nil runtime dependency (the natural-language scenario tools need AI +
// spec access; generate_and_verify_scenarios needs the gen-agent). When the
// catalog is constructed with empty Deps — as the embedding generator and
// offline tooling do — those tools are silently dropped and end up missing
// from find_tools vector search.
//
// CatalogTools sidesteps that by registering every tool against sentinel
// dependencies whose only job is to be non-nil. The returned tools are for
// metadata enumeration (name / description / domain / schema) only; their
// Run closures capture the stubs and must never be executed.
package builtin

import (
	"context"
	"errors"

	"autotest/internal/aitools"
	"autotest/internal/genagent"
	"autotest/internal/scenario"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// CatalogTools returns the full domain tool set (ReadOnly + Mutating +
// GatedMutating) with every conditionally-registered tool included. Use it
// wherever the complete tool catalog is needed independent of runtime wiring,
// e.g. precomputing find_tools embeddings.
func CatalogTools() []aitools.Tool {
	deps := Deps{
		AI:        catalogStubChatProvider{},
		Specs:     catalogStubSpecRepo{},
		Scenarios: catalogStubScenarioService{},
		GenAgent:  catalogStubGenAgent{},
	}
	tools := ReadOnly(deps)
	tools = append(tools, Mutating(deps)...)
	tools = append(tools, GatedMutating(deps)...)
	return tools
}

// errCatalogStub is returned by any sentinel method that is accidentally
// invoked. Catalog enumeration never calls these; a non-nil error here
// signals a programming mistake (executing a stub-backed tool).
var errCatalogStub = errors.New("builtin: catalog stub dependency is metadata-only and must not be executed")

type catalogStubChatProvider struct{}

func (catalogStubChatProvider) Chat(context.Context, uuid.UUID, any) (any, error) {
	return nil, errCatalogStub
}
func (catalogStubChatProvider) ResolveDefaultChatProvider(context.Context) (uuid.UUID, string, error) {
	return uuid.Nil, "", errCatalogStub
}

type catalogStubSpecRepo struct{}

func (catalogStubSpecRepo) ListSpecs(context.Context, uuid.UUID, uuid.UUID) ([]spec.APISpec, error) {
	return nil, errCatalogStub
}
func (catalogStubSpecRepo) ListEndpoints(context.Context, uuid.UUID, uuid.UUID) ([]spec.Endpoint, error) {
	return nil, errCatalogStub
}
func (catalogStubSpecRepo) GetEndpointByID(context.Context, uuid.UUID) (*spec.Endpoint, error) {
	return nil, errCatalogStub
}

type catalogStubScenarioService struct{}

func (catalogStubScenarioService) Get(context.Context, uuid.UUID) (*scenario.Scenario, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) List(context.Context, scenario.ListFilter) ([]scenario.Scenario, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) Create(context.Context, scenario.CreateScenarioInput) (*scenario.Scenario, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) Update(context.Context, uuid.UUID, scenario.UpdateScenarioInput) (*scenario.Scenario, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) Delete(context.Context, uuid.UUID) error {
	return errCatalogStub
}
func (catalogStubScenarioService) UpsertStep(context.Context, uuid.UUID, scenario.UpsertStepInput) (*scenario.Step, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) DeleteStep(context.Context, uuid.UUID) error {
	return errCatalogStub
}
func (catalogStubScenarioService) ListSteps(context.Context, uuid.UUID) ([]scenario.Step, error) {
	return nil, errCatalogStub
}
func (catalogStubScenarioService) ReorderSteps(context.Context, uuid.UUID, scenario.ReorderStepsInput) error {
	return errCatalogStub
}

type catalogStubGenAgent struct{}

func (catalogStubGenAgent) RunAsync(context.Context, genagent.RunConfig) (*genagent.Job, error) {
	return nil, errCatalogStub
}
