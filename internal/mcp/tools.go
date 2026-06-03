package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"autotest/internal/spec"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTools attaches autotest OpenAPI/Swagger import tools to the MCP server.
func RegisterTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	type importSwaggerArgs struct {
		ProjectID   string `json:"projectId,omitempty" jsonschema:"project UUID; falls back to AUTOTEST_PROJECT_ID when omitted"`
		ServiceID   string `json:"serviceId,omitempty" jsonschema:"service UUID; falls back to AUTOTEST_SERVICE_ID when omitted"`
		FilePath    string `json:"filePath,omitempty" jsonschema:"local path to openapi.json/yaml or swagger file"`
		Content     string `json:"content,omitempty" jsonschema:"raw OpenAPI/Swagger document text (use when filePath is not set)"`
		ContentType string `json:"contentType,omitempty" jsonschema:"optional Content-Type: application/json or application/yaml (auto-detected when omitted)"`
		SyncMode    string `json:"syncMode,omitempty" jsonschema:"optional sync mode: merge (default) or overwrite"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "import_swagger",
		Description: "Import an OpenAPI 3.x or Swagger 2.0 document into autotest for a project service. Provide filePath or content. Requires API Key with specs:import scope.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args importSwaggerArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, serviceID, err := resolveProjectService(cfg, args.ProjectID, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		data, err := loadSpecBytes(args.FilePath, args.Content)
		if err != nil {
			return toolError(err)
		}
		summary, err := client.ImportSpec(ctx, projectID, serviceID, data, args.ContentType, args.SyncMode)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(summary)
	})

	type importSwaggerURLArgs struct {
		ProjectID   string `json:"projectId,omitempty" jsonschema:"project UUID; falls back to AUTOTEST_PROJECT_ID when omitted"`
		ServiceID   string `json:"serviceId,omitempty" jsonschema:"service UUID; falls back to AUTOTEST_SERVICE_ID when omitted"`
		URL         string `json:"url" jsonschema:"HTTP(S) URL of the OpenAPI/Swagger document"`
		ContentType string `json:"contentType,omitempty" jsonschema:"optional Content-Type override after download"`
		SyncMode    string `json:"syncMode,omitempty" jsonschema:"optional sync mode: merge (default) or overwrite"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "import_swagger_from_url",
		Description: "Download an OpenAPI/Swagger document from a URL and import it into autotest. Requires API Key with specs:import scope.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, args importSwaggerURLArgs) (*sdkmcp.CallToolResult, any, error) {
		projectID, serviceID, err := resolveProjectService(cfg, args.ProjectID, args.ServiceID)
		if err != nil {
			return toolError(err)
		}
		url := strings.TrimSpace(args.URL)
		if url == "" {
			return toolError(fmt.Errorf("url is required"))
		}
		data, err := fetchURL(ctx, url)
		if err != nil {
			return toolError(err)
		}
		summary, err := client.ImportSpec(ctx, projectID, serviceID, data, args.ContentType, args.SyncMode)
		if err != nil {
			return toolError(err)
		}
		return toolJSON(summary)
	})
}

func resolveProjectService(cfg Config, projectID, serviceID string) (uuid.UUID, uuid.UUID, error) {
	pid := strings.TrimSpace(projectID)
	if pid == "" {
		pid = cfg.ProjectID
	}
	sid := strings.TrimSpace(serviceID)
	if sid == "" {
		sid = cfg.ServiceID
	}
	if pid == "" || sid == "" {
		return uuid.Nil, uuid.Nil, fmt.Errorf("projectId and serviceId are required (pass in tool args or set %s / %s)", envProjectID, envServiceID)
	}
	parsedPID, err := uuid.Parse(pid)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid projectId: %w", err)
	}
	parsedSID, err := uuid.Parse(sid)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid serviceId: %w", err)
	}
	return parsedPID, parsedSID, nil
}

func loadSpecBytes(filePath, content string) ([]byte, error) {
	filePath = strings.TrimSpace(filePath)
	content = strings.TrimSpace(content)
	switch {
	case filePath != "" && content != "":
		return nil, fmt.Errorf("provide only one of filePath or content")
	case filePath != "":
		data, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", filePath, err)
		}
		return data, nil
	case content != "":
		return []byte(content), nil
	default:
		return nil, fmt.Errorf("either filePath or content is required")
	}
}

func fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download spec: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download spec: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSpecBytes {
		return nil, fmt.Errorf("downloaded spec exceeds %d bytes limit", maxSpecBytes)
	}
	return data, nil
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

func toolError(err error) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: err.Error()},
		},
		IsError: true,
	}, nil, nil
}

// FormatSummary returns a human-readable import result for tests and logging.
func FormatSummary(s *spec.ImportSummary) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("specId=%s apiCount=%d created=%d updated=%d generatedCases=%d",
		s.SpecID, s.ApiCount, s.CreatedEndpoints, s.UpdatedEndpoints, s.GeneratedCases)
}
