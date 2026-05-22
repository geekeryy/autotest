package builtin

import (
	"context"

	testcase "autotest/internal/case"
	"autotest/internal/mockserver"
	"autotest/internal/mockset"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scriptlibrary"
	"autotest/internal/spec"
	"autotest/internal/testdata"

	"github.com/google/uuid"
)

// Deps gathers the platform services needed by the default tool set. Each
// dependency is an interface declared at the consumer side so test code
// can substitute fakes without dragging in heavier service-level wiring.
//
// All fields are optional at construction time. Tools whose dependency is
// nil return a structured error at run time rather than panicking, which
// lets the AI surface the missing capability back to the user instead of
// crashing the request.
type Deps struct {
	Cases       CaseService
	Scenarios   ScenarioService
	Specs       SpecRepository
	Projects    ProjectService
	ParamSource ParamSourceService
	TestData    TestDataService
	MockServers MockServerService
	MockSets    MockSetService
	Scripts     ScriptLibraryService
	Runs        RunsService
}

// CaseService is the slice of `internal/case.Service` we depend on.
type CaseService interface {
	Get(ctx context.Context, testCaseID uuid.UUID) (*testcase.TestCase, error)
	List(ctx context.Context, filter testcase.ListFilter) ([]testcase.TestCase, error)
	ListSaved(ctx context.Context, parentCaseID uuid.UUID) ([]testcase.TestCase, error)
	CreateManual(ctx context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error)
	CreateSaved(ctx context.Context, parentCaseID uuid.UUID, input testcase.CreateSavedInput) (*testcase.TestCase, error)
	Patch(ctx context.Context, testCaseID uuid.UUID, input testcase.PatchInput) (*testcase.TestCase, error)
}

// ScenarioService is the slice of `internal/scenario.Service` we depend on.
// It must cover the full orchestration surface so the AI can produce a
// scenario the user only needs to click "Run" on.
type ScenarioService interface {
	Get(ctx context.Context, scenarioID uuid.UUID) (*scenario.Scenario, error)
	List(ctx context.Context, filter scenario.ListFilter) ([]scenario.Scenario, error)
	Create(ctx context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error)
	Update(ctx context.Context, id uuid.UUID, input scenario.UpdateScenarioInput) (*scenario.Scenario, error)
	UpsertStep(ctx context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error)
	DeleteStep(ctx context.Context, stepID uuid.UUID) error
	ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error)
	ReorderSteps(ctx context.Context, scenarioID uuid.UUID, input scenario.ReorderStepsInput) error
}

// SpecRepository is the slice of `internal/spec.Repository` we depend on.
// We depend on the repository rather than spec.Service because the only
// calls we make are bulk listing and endpoint lookup — going through
// Service would force us to expose new helpers we do not otherwise need.
type SpecRepository interface {
	ListSpecs(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.APISpec, error)
	ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.Endpoint, error)
	GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*spec.Endpoint, error)
}

// ProjectService is the slice of `internal/project.ServiceLayer` we depend
// on. Service and environment writes are intentionally excluded: project
// structure (services / environments) is treated as platform-management
// surface that the user maintains by hand. Only read access is exposed
// so the AI can answer "what services / environments are configured"
// without ever mutating them.
type ProjectService interface {
	ListServices(ctx context.Context, projectID uuid.UUID) ([]project.Service, error)
	ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]project.Environment, error)
	ListServiceEnvironments(ctx context.Context, projectID, serviceID uuid.UUID) ([]project.Environment, error)
}

// ParamSourceService is the slice of `internal/paramsource.Service` we
// depend on. We expose database data sources (Postgres connection
// configs) and SQL parameter sources (parameterised queries used inline
// in templates).
type ParamSourceService interface {
	ListDataSources(ctx context.Context) ([]paramsource.DataSource, error)
	CreateDataSource(ctx context.Context, input paramsource.DataSourceInput) (*paramsource.DataSource, error)
	UpdateDataSource(ctx context.Context, id uuid.UUID, input paramsource.DataSourceInput) (*paramsource.DataSource, error)
	TestDataSource(ctx context.Context, id uuid.UUID) error
	GetDataSourceSchema(ctx context.Context, id uuid.UUID) ([]paramsource.SchemaTable, error)

	ListSQLParameterSources(ctx context.Context, projectID, serviceID uuid.UUID) ([]paramsource.SQLParameterSource, error)
	CreateSQLParameterSource(ctx context.Context, input paramsource.SQLParameterSourceInput) (*paramsource.SQLParameterSource, error)
	UpdateSQLParameterSource(ctx context.Context, id uuid.UUID, input paramsource.SQLParameterSourceInput) (*paramsource.SQLParameterSource, error)
	PreviewSQLParameterSource(ctx context.Context, id uuid.UUID, input paramsource.PreviewInput) (*paramsource.PreviewOutput, error)
}

// TestDataService is the slice of `internal/testdata.Service` we depend on.
// The Resolve / inline-reference and AI generation paths are intentionally
// excluded — those are runtime concerns, not chat-time tool calls.
type TestDataService interface {
	ListTables(ctx context.Context, projectID uuid.UUID) ([]testdata.Table, error)
	GetTable(ctx context.Context, projectID, tableID uuid.UUID) (*testdata.Table, error)
	CreateTable(ctx context.Context, projectID uuid.UUID, input testdata.TableInput) (*testdata.Table, error)
	UpdateTable(ctx context.Context, projectID, tableID uuid.UUID, input testdata.TableInput) (*testdata.Table, error)
	ListRows(ctx context.Context, projectID, tableID uuid.UUID) ([]testdata.Row, error)
	ReplaceRows(ctx context.Context, projectID, tableID uuid.UUID, input testdata.RowsReplaceInput) ([]testdata.Row, error)
	GenerateRows(ctx context.Context, projectID, tableID uuid.UUID, input testdata.GenerateRowsInput) (*testdata.GenerateRowsOutput, error)
}

// MockServerService is the slice of `internal/mockserver.Service` we
// depend on. Start/Stop are intentionally excluded — runtime control
// belongs to the user, mirroring the run-scenario boundary.
type MockServerService interface {
	ListServers(ctx context.Context, projectID uuid.UUID) ([]mockserver.ServerWithStatus, error)
	GetStatus(ctx context.Context, projectID, serverID uuid.UUID) (mockserver.ServerStatus, error)
	CreateServer(ctx context.Context, projectID uuid.UUID, input mockserver.MockServerInput) (*mockserver.ServerWithStatus, error)
	UpdateServer(ctx context.Context, projectID, serverID uuid.UUID, input mockserver.MockServerInput) (*mockserver.ServerWithStatus, error)

	ListRoutes(ctx context.Context, projectID, serverID uuid.UUID) ([]mockserver.MockRoute, error)
	CreateRoute(ctx context.Context, projectID, serverID uuid.UUID, input mockserver.MockRouteInput) (*mockserver.MockRoute, error)
	UpdateRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID, input mockserver.MockRouteInput) (*mockserver.MockRoute, error)
}

// MockSetService is the slice of `internal/mockset.Service` we depend on.
type MockSetService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]mockset.ValueSet, error)
	Create(ctx context.Context, projectID uuid.UUID, input mockset.CreateInput) (*mockset.ValueSet, error)
	Update(ctx context.Context, projectID, setID uuid.UUID, input mockset.UpdateInput) (*mockset.ValueSet, error)
}

// ScriptLibraryService is the slice of `internal/scriptlibrary.Service`
// we depend on. List is project-scoped and already merges built-in
// templates with project-custom ones.
type ScriptLibraryService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]scriptlibrary.Template, error)
	Create(ctx context.Context, input scriptlibrary.CreateInput) (*scriptlibrary.Template, error)
	Update(ctx context.Context, id uuid.UUID, input scriptlibrary.UpdateInput) (*scriptlibrary.Template, error)
}

// RunsService is the slice of `internal/runner.Service` we expose for
// read-only inspection of historical runs. Triggering runs is a user-only
// action and never exposed.
type RunsService interface {
	GetRunResult(ctx context.Context, runID uuid.UUID) (*runner.RunResultOutput, error)
}
