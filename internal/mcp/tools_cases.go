package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	testcase "autotest/internal/case"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCaseTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	type listCasesArgs struct {
		ProjectID string `json:"projectId,omitempty"`
		ServiceID string `json:"serviceId,omitempty"`
		Filter    string `json:"filter,omitempty" jsonschema:"optional substring filter on name/method/path"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_cases",
		Description: "List request template summaries for a project (optional serviceId filter). Requires cases:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listCasesArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		serviceID, err := parseOptionalServiceID(cfg, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		cs, err := client.ListCases(ctx, projectID, serviceID)
		if err != nil {
			return toolError(err)
		}
		needle := strings.ToLower(strings.TrimSpace(args.Filter))
		digests := make([]map[string]any, 0, len(cs))
		for _, c := range cs {
			if needle != "" {
				blob := strings.ToLower(c.Name + " " + c.Method + " " + c.Path)
				if !strings.Contains(blob, needle) {
					continue
				}
			}
			digests = append(digests, map[string]any{
				"id": c.ID, "projectId": c.ProjectID, "serviceId": c.ServiceID,
				"endpointId": c.EndpointID, "name": c.Name, "method": c.Method,
				"path": c.Path, "source": c.Source, "status": c.Status,
			})
		}
		return toolJSON(map[string]any{"cases": digests, "count": len(digests)})
	})

	type getCaseArgs struct {
		CaseID string `json:"caseId"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_case",
		Description: "Get a single request template or saved case. Requires cases:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getCaseArgs) (*sdkmcp.CallToolResult, any, error) {
		id, err := uuid.Parse(strings.TrimSpace(args.CaseID))
		if err != nil {
			return toolError(fmt.Errorf("invalid caseId: %w", err))
		}
		tc, err := client.GetCase(ctx, id)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(tc)
	})

	type createCaseArgs struct {
		ProjectID  string          `json:"projectId,omitempty"`
		ServiceID  string          `json:"serviceId,omitempty"`
		EndpointID string          `json:"endpointId,omitempty"`
		Method     string          `json:"method,omitempty"`
		Path       string          `json:"path,omitempty"`
		Name       string          `json:"name"`
		Request    json.RawMessage `json:"request,omitempty"`
		Assertions json.RawMessage `json:"assertions,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_case_from_endpoint",
		Description: "Create a runnable API request template. Requires cases:write. endpointId or method+path required.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args createCaseArgs) (*sdkmcp.CallToolResult, any, error) {
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
		if strings.TrimSpace(args.Name) == "" {
			return toolError(fmt.Errorf("name is required"))
		}
		method := strings.TrimSpace(args.Method)
		path := strings.TrimSpace(args.Path)
		var endpointID *uuid.UUID
		if s := strings.TrimSpace(args.EndpointID); s != "" {
			id, err := uuid.Parse(s)
			if err != nil {
				return toolError(fmt.Errorf("invalid endpointId: %w", err))
			}
			endpointID = &id
		}
		if endpointID == nil && (method == "" || path == "") {
			return toolError(fmt.Errorf("endpointId or method+path is required"))
		}
		req := args.Request
		if len(req) == 0 {
			req = json.RawMessage(`{}`)
		}
		assertions := args.Assertions
		if len(assertions) == 0 {
			assertions = json.RawMessage(`[]`)
		}
		tc, err := client.CreateCase(ctx, testcase.CreateManualInput{
			ProjectID:  projectID,
			ServiceID:  serviceID,
			EndpointID: endpointID,
			Name:       strings.TrimSpace(args.Name),
			Method:     strings.ToUpper(method),
			Path:       path,
			Request:    req,
			Assertions: assertions,
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(tc)
	})

	type patchCaseArgs struct {
		CaseID     string          `json:"caseId"`
		Name       *string         `json:"name,omitempty"`
		Assertions json.RawMessage `json:"assertions,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_case",
		Description: "Patch a request template (name and/or assertions). Requires cases:write.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args patchCaseArgs) (*sdkmcp.CallToolResult, any, error) {
		id, err := uuid.Parse(strings.TrimSpace(args.CaseID))
		if err != nil {
			return toolError(fmt.Errorf("invalid caseId: %w", err))
		}
		tc, err := client.PatchCase(ctx, id, testcase.PatchInput{
			Name:       args.Name,
			Assertions: args.Assertions,
		})
		if err != nil {
			return toolError(err)
		}
		return toolJSON(tc)
	})
}
