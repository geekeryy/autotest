package mcp

import (
	"context"
	"fmt"
	"strings"

	"autotest/internal/runner"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerRunTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	type listRunsArgs struct {
		ProjectID  string `json:"projectId,omitempty"`
		ScenarioID string `json:"scenarioId,omitempty"`
		ServiceID  string `json:"serviceId,omitempty"`
		Status     string `json:"status,omitempty"`
		Limit      int    `json:"limit,omitempty"`
		Offset     int    `json:"offset,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_runs",
		Description: "List scenario run records for a project. Requires runs:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listRunsArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		var scenarioID, serviceID uuid.UUID
		if s := strings.TrimSpace(args.ScenarioID); s != "" {
			scenarioID, err = uuid.Parse(s)
			if err != nil {
				return toolError(fmt.Errorf("invalid scenarioId: %w", err))
			}
		}
		if s := strings.TrimSpace(args.ServiceID); s != "" {
			serviceID, err = uuid.Parse(s)
			if err != nil {
				return toolError(fmt.Errorf("invalid serviceId: %w", err))
			}
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 20
		}
		page, err := client.ListRuns(ctx, projectID, buildRunsQuery(scenarioID, serviceID, args.Status, limit, args.Offset))
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{
			"runs": page.Items, "total": page.Total,
			"page": page.Page, "pageSize": page.PageSize, "count": len(page.Items),
		})
	})

	type getRunResultArgs struct {
		RunID string `json:"runId"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_run_result",
		Description: "Get run details and step results. Requires runs:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getRunResultArgs) (*sdkmcp.CallToolResult, any, error) {
		runID, err := uuid.Parse(strings.TrimSpace(args.RunID))
		if err != nil {
			return toolError(fmt.Errorf("invalid runId: %w", err))
		}
		out, err := client.GetRunResult(ctx, runID)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(out)
	})

	type runScenarioArgs struct {
		ScenarioID    string         `json:"scenarioId"`
		EnvironmentID string         `json:"environmentId,omitempty"`
		Name          string         `json:"name,omitempty"`
		Variables     map[string]any `json:"variables,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "run_scenario",
		Description: "Execute a scenario in a target environment. Requires runs:execute. environmentId falls back to AUTOTEST_ENVIRONMENT_ID.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args runScenarioArgs) (*sdkmcp.CallToolResult, any, error) {
		scenarioID, err := uuid.Parse(strings.TrimSpace(args.ScenarioID))
		if err != nil {
			return toolError(fmt.Errorf("invalid scenarioId: %w", err))
		}
		envID, err := resolveEnvironmentID(cfg, args.EnvironmentID)
		if err != nil {
			return toolError(err)
		}
		out, err := client.RunScenario(ctx, scenarioID, runner.RunScenarioInput{
			EnvironmentID: envID,
			Name:          strings.TrimSpace(args.Name),
			Variables:     args.Variables,
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(out)
	})

	type runCaseArgs struct {
		CaseID        string         `json:"caseId"`
		EnvironmentID string         `json:"environmentId,omitempty"`
		Name          string         `json:"name,omitempty"`
		Variables     map[string]any `json:"variables,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "run_case",
		Description: "Execute a single request template. Requires runs:execute.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args runCaseArgs) (*sdkmcp.CallToolResult, any, error) {
		caseID, err := uuid.Parse(strings.TrimSpace(args.CaseID))
		if err != nil {
			return toolError(fmt.Errorf("invalid caseId: %w", err))
		}
		envID, err := resolveEnvironmentID(cfg, args.EnvironmentID)
		if err != nil {
			return toolError(err)
		}
		out, err := client.RunCase(ctx, caseID, runner.RunCaseInput{
			EnvironmentID: envID,
			Name:          strings.TrimSpace(args.Name),
			Variables:     args.Variables,
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(out)
	})
}
