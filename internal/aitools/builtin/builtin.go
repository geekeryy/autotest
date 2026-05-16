// Package builtin assembles the default tool set wired against the
// platform's domain services.
//
// Tools are grouped along two axes:
//
//   - Read-only vs. Mutating. Read-only tools (`get_*`, `list_*`) are also
//     served to the smart-analysis flow. Mutating tools (`create_*`,
//     `add_*`, `update_*`, `delete_*`, `reorder_*`) are reserved for the
//     conversational assistant and only execute after the user approves
//     them through the human-in-the-loop confirmation flow.
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
		// cases
		listCasesTool(deps),
		getCaseTool(deps),
		// scenarios
		listScenariosTool(deps),
		getScenarioTool(deps),
	}
}

// Mutating returns the human-in-the-loop tools that may modify platform
// data. Only the conversational assistant exposes these to the model, and
// every call must be confirmed by the user before it actually runs.
//
// The deliberate omission here is anything that "runs" a scenario or
// triggers a test execution: by product decision, the user always pushes
// the run button themselves, so the AI's contract ends at generating the
// scenario ready to run.
func Mutating(deps Deps) []aitools.Tool {
	return []aitools.Tool{
		// cases
		createCaseFromEndpointTool(deps),
		updateCaseAssertionsTool(deps),
		// scenarios
		createScenarioWithStepsTool(deps),
		addScenarioStepTool(deps),
		updateScenarioStepTool(deps),
		deleteScenarioStepTool(deps),
		reorderScenarioStepsTool(deps),
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
