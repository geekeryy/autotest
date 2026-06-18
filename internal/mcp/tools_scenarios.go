package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/scenario"
	"autotest/internal/scenariobuild"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerScenarioTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	scAPI := NewHTTPScenarioAPI(client)

	type listScenariosArgs struct {
		ProjectID string `json:"projectId,omitempty"`
		ServiceID string `json:"serviceId,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_scenarios",
		Description: "List scenarios in a project. Requires scenarios:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listScenariosArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		serviceID, err := parseOptionalServiceID(cfg, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		items, err := client.ListScenarios(ctx, projectID, serviceID)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenarios": items, "count": len(items)})
	})

	type getScenarioArgs struct {
		ScenarioID string `json:"scenarioId"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_scenario",
		Description: "Get scenario metadata and all steps. Requires scenarios:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getScenarioArgs) (*sdkmcp.CallToolResult, any, error) {
		id, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		sc, err := client.GetScenario(ctx, id)
		if err != nil {
			return toolError(err)
		}
		steps, err := client.ListSteps(ctx, id)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenario": sc, "steps": steps})
	})

	type createScenarioWithStepsArgs struct {
		ProjectID   string                `json:"projectId,omitempty"`
		ServiceID   string                `json:"serviceId,omitempty"`
		Name        string                `json:"name"`
		Description string                `json:"description,omitempty"`
		Steps       []scenariobuild.StepInput `json:"steps"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "create_scenario_with_steps",
		Description: "Create a scenario and all steps in one call. Control-flow children use stepOrder in config; platform converts to step_seq. " +
			"API steps need testCaseId. requestOverride may include template variables (see describe_template_syntax). Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args createScenarioWithStepsArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		serviceID, err := parseOptionalServiceID(cfg, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		if serviceID == uuid.Nil {
			return toolError(fmt.Errorf("serviceId is required (pass in tool args or set %s)", envServiceID))
		}
		result, err := scAPI.CreateScenarioWithSteps(ctx, scenariobuild.CreateInput{
			ProjectID:   projectID,
			ServiceID:   serviceID,
			Name:        args.Name,
			Description: args.Description,
			Steps:       args.Steps,
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenario": result.Scenario, "steps": result.Steps})
	})

	type updateScenarioArgs struct {
		ScenarioID  string `json:"scenarioId"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_scenario",
		Description: "Update scenario name/description. Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateScenarioArgs) (*sdkmcp.CallToolResult, any, error) {
		id, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		sc, err := client.UpdateScenario(ctx, id, scenario.UpdateScenarioInput{
			Name:        strings.TrimSpace(args.Name),
			Description: strings.TrimSpace(args.Description),
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(sc)
	})

	type deleteScenarioArgs struct {
		ScenarioID string `json:"scenarioId"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_scenario",
		Description: "Delete a scenario (irreversible). Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args deleteScenarioArgs) (*sdkmcp.CallToolResult, any, error) {
		id, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		if err := client.DeleteScenario(ctx, id); err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenarioId": id, "deleted": true})
	})

	type upsertStepArgs struct {
		ScenarioID      string          `json:"scenarioId"`
		StepOrder       int             `json:"stepOrder"`
		StepType        string          `json:"stepType" jsonschema:"enum=api,database,script,for,condition"`
		Name            string          `json:"name"`
		Enabled         *bool           `json:"enabled,omitempty"`
		TestCaseID      string          `json:"testCaseId,omitempty"`
		Config          json.RawMessage `json:"config,omitempty"`
		RequestOverride json.RawMessage `json:"requestOverride,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "upsert_scenario_step",
		Description: "Create or replace a step at stepOrder. Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args upsertStepArgs) (*sdkmcp.CallToolResult, any, error) {
		scenarioID, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		in := scenariobuild.StepInput{
			StepOrder: args.StepOrder, StepType: args.StepType, Name: args.Name,
			Enabled: args.Enabled, TestCaseID: args.TestCaseID,
			Config: args.Config, RequestOverride: args.RequestOverride,
		}
		cfg := in.Config
		if scenariobuild.NeedsSeqRewrite(in.StepType) {
			existing, err := client.ListSteps(ctx, scenarioID)
			if err != nil {
				return toolError(err)
			}
			orderToSeq := scenariobuild.BuildOrderToSeq(existing)
			rewritten, _, err := scenariobuild.RewriteControlFlowConfig(in.StepType, cfg, orderToSeq)
			if err != nil {
				return toolError(err)
			}
			cfg = rewritten
			in.Config = cfg
		}
		upsert, err := scenariobuild.BuildUpsertInput(in, nil)
		if err != nil {
			return toolError(err)
		}
		step, err := client.UpsertStep(ctx, scenarioID, args.StepOrder, upsert)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(step)
	})

	type deleteStepArgs struct {
		ScenarioID string `json:"scenarioId"`
		StepID     string `json:"stepId"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_scenario_step",
		Description: "Delete a scenario step by step UUID. Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args deleteStepArgs) (*sdkmcp.CallToolResult, any, error) {
		scenarioID, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		stepID, err := uuid.Parse(strings.TrimSpace(args.StepID))
		if err != nil {
			return toolError(fmt.Errorf("invalid stepId: %w", err))
		}
		if err := client.DeleteStep(ctx, scenarioID, stepID); err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenarioId": scenarioID, "stepId": stepID, "deleted": true})
	})

	type reorderArgs struct {
		ScenarioID string   `json:"scenarioId"`
		StepIDs    []string `json:"stepIds"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reorder_scenario_steps",
		Description: "Reorder steps by step UUID list. Requires scenarios:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args reorderArgs) (*sdkmcp.CallToolResult, any, error) {
		scenarioID, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		ids := make([]uuid.UUID, 0, len(args.StepIDs))
		for i, s := range args.StepIDs {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil {
				return toolError(fmt.Errorf("stepIds[%d]: %w", i, err))
			}
			ids = append(ids, id)
		}
		if err := client.ReorderSteps(ctx, scenarioID, scenario.ReorderStepsInput{StepIDs: ids}); err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"scenarioId": scenarioID, "stepIds": ids})
	})
}
