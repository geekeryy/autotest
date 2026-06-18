package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// stepTypeEnum is used in MCP tool jsonschema for scenario steps.
const stepTypeEnum = `["api","database","script","for","condition"]`

func resolveProjectID(cfg Config, projectID string) (uuid.UUID, error) {
	pid := strings.TrimSpace(projectID)
	if pid == "" {
		pid = cfg.ProjectID
	}
	if pid == "" {
		return uuid.Nil, fmt.Errorf("projectId is required (pass in tool args or set %s)", envProjectID)
	}
	parsed, err := uuid.Parse(pid)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid projectId: %w", err)
	}
	return parsed, nil
}

func resolveEnvironmentID(cfg Config, environmentID string) (uuid.UUID, error) {
	eid := strings.TrimSpace(environmentID)
	if eid == "" {
		eid = cfg.EnvironmentID
	}
	if eid == "" {
		return uuid.Nil, fmt.Errorf("environmentId is required (pass in tool args or set %s)", envEnvironmentID)
	}
	parsed, err := uuid.Parse(eid)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid environmentId: %w", err)
	}
	return parsed, nil
}

func toolJSON(v any) (*sdkmcp.CallToolResult, any, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: string(raw)},
		},
	}, v, nil
}

func toolText(text string) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: text},
		},
	}, text, nil
}

func toolError(err error) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: err.Error()},
		},
		IsError: true,
	}, nil, nil
}
