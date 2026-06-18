package mcp

import (
	"context"
	"strings"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerMetaTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	type listServicesArgs struct {
		ProjectID string `json:"projectId,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_services",
		Description: "List services in a project. Requires API Key scope cases:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listServicesArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		items, err := client.ListServices(ctx, projectID)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"services": items, "count": len(items)})
	})

	type listEndpointsArgs struct {
		ProjectID string `json:"projectId,omitempty"`
		ServiceID string `json:"serviceId,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_endpoints",
		Description: "List OpenAPI endpoints for a project service. Requires cases:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listEndpointsArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, serviceID, err := resolveProjectService(cfg, args.ProjectID, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		items, err := client.ListEndpoints(ctx, projectID, serviceID)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"endpoints": items, "count": len(items)})
	})

	type listEnvironmentsArgs struct {
		ProjectID string `json:"projectId,omitempty"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_environments",
		Description: "List environments for a project. Requires cases:read.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listEnvironmentsArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, err := resolveProjectID(cfg, args.ProjectID)
		if err != nil {
			return toolError(err)
		}
		items, err := client.ListEnvironments(ctx, projectID)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(map[string]any{"environments": items, "count": len(items)})
	})
}

func parseOptionalServiceID(cfg Config, serviceID string) (uuid.UUID, error) {
	sid := strings.TrimSpace(serviceID)
	if sid == "" {
		sid = cfg.ServiceID
	}
	if sid == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(sid)
}
