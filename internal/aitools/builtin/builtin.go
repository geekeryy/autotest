// Package builtin assembles the default tool set wired against the
// platform's domain services.
//
// Tools are grouped along two axes:
//
//   - Read-only vs. mutating vs. RequiresConfirm. Read-only tools are also
//     served to smart-analysis. Mutating create/update tools run immediately
//     in the assistant stream; delete_* tools require user confirmation.
//   - Domain. `builtin_meta.go` covers project/service/environment/endpoint
//     introspection. `builtin_cases.go` covers API request templates.
//     `builtin_scenarios.go` covers scenarios and their steps — which is
//     where the AI-driven scenario generation flow lives.
package builtin

import (
	"encoding/json"
	"fmt"

	"autotest/internal/aitools"
)

// ReadOnly returns the read-only tools shared between smart analysis and
// the conversational assistant.
func ReadOnly(deps Deps) []aitools.Tool {
	return []aitools.Tool{
		// meta / discovery
		listServicesTool(deps),
		listEndpointsTool(deps),
		listEnvironmentsTool(deps),
		getEndpointTool(deps),
		getEnvironmentTool(deps),
		// specs
		listSpecsTool(deps),
		// cases
		listCasesTool(deps),
		getCaseTool(deps),
		// scenarios
		listScenariosTool(deps),
		getScenarioTool(deps),
		// mock servers
		listMockServersTool(deps),
		getMockServerStatusTool(deps),
		listMockRoutesTool(deps),
		// mock value sets
		listMockValueSetsTool(deps),
		// test data
		listTestDataTablesTool(deps),
		getTestDataTableTool(deps),
		listTestDataRowsTool(deps),
		// param sources
		listDataSourcesTool(deps),
		listSQLParameterSourcesTool(deps),
		previewSQLParameterSourceTool(deps),
		// scripts
		listScriptTemplatesTool(deps),
		// runs
		listRunsTool(deps),
		getRunResultTool(deps),
	}
}

// Mutating returns write tools for the conversational assistant. Delete
// tools set RequiresConfirm and pause the stream until the user approves.
//
// The deliberate omission here is anything that "runs" a scenario or
// triggers a test execution: by product decision, the user always pushes
// the run button themselves, so the AI's contract ends at generating the
// scenario ready to run.
func Mutating(deps Deps) []aitools.Tool {
	return []aitools.Tool{
		// projects / services / environments
		createServiceTool(deps),
		updateServiceTool(deps),
		deleteServiceTool(deps),
		createServiceEnvironmentTool(deps),
		updateServiceEnvironmentTool(deps),
		deleteServiceEnvironmentTool(deps),
		// specs
		importOpenAPITool(deps),
		// cases
		createCaseFromEndpointTool(deps),
		updateCaseAssertionsTool(deps),
		updateCaseTool(deps),
		// scenarios
		createScenarioWithStepsTool(deps),
		addScenarioStepTool(deps),
		updateScenarioStepTool(deps),
		deleteScenarioStepTool(deps),
		reorderScenarioStepsTool(deps),
		updateScenarioTool(deps),
		deleteScenarioTool(deps),
		// mock servers
		createMockServerTool(deps),
		updateMockServerTool(deps),
		deleteMockServerTool(deps),
		createMockRouteTool(deps),
		updateMockRouteTool(deps),
		deleteMockRouteTool(deps),
		// mock value sets
		createMockValueSetTool(deps),
		updateMockValueSetTool(deps),
		deleteMockValueSetTool(deps),
		// test data
		createTestDataTableTool(deps),
		updateTestDataTableTool(deps),
		deleteTestDataTableTool(deps),
		replaceTestDataRowsTool(deps),
		// param sources
		createDataSourceTool(deps),
		updateDataSourceTool(deps),
		deleteDataSourceTool(deps),
		createSQLParameterSourceTool(deps),
		updateSQLParameterSourceTool(deps),
		deleteSQLParameterSourceTool(deps),
		// scripts
		createScriptTemplateTool(deps),
		updateScriptTemplateTool(deps),
		deleteScriptTemplateTool(deps),
	}
}

// All concatenates ReadOnly + Mutating in a stable order.
func All(deps Deps) []aitools.Tool {
	tools := ReadOnly(deps)
	return append(tools, Mutating(deps)...)
}

// rawSchema is a tiny helper to author JSON Schema bodies as Go raw
// strings while still going through json.Unmarshal as a syntax sanity
// check. A parse failure is a programming bug and panics; production
// callers never see it.
func rawSchema(s string) json.RawMessage {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic(fmt.Errorf("aitools/builtin: invalid JSON schema literal: %w\n%s", err, s))
	}
	return json.RawMessage(s)
}
