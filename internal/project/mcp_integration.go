package project

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"autotest/internal/apikey"
	"autotest/internal/config"

	"github.com/google/uuid"
)

// McpIntegrationEnvironment is a brief environment row for MCP setup UI.
type McpIntegrationEnvironment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// McpIntegrationGuide is returned when configuring MCP for a service.
type McpIntegrationGuide struct {
	ServiceMcpEnabled    bool                        `json:"serviceMcpEnabled"`
	ServerMcpHttpEnabled bool                        `json:"serverMcpHttpEnabled"`
	McpHttpPath          string                      `json:"mcpHttpPath"`
	McpHttpURL           string                      `json:"mcpHttpURL"`
	APIBaseURL           string                      `json:"apiBaseUrl"`
	ProjectID            string                      `json:"projectId"`
	ServiceID            string                      `json:"serviceId"`
	DefaultEnvironmentID string                      `json:"defaultEnvironmentId,omitempty"`
	Environments         []McpIntegrationEnvironment `json:"environments"`
	RequiredScopes       []string                    `json:"requiredScopes"`
	CursorHTTP           json.RawMessage             `json:"cursorHttp"`
	CursorStdio          json.RawMessage             `json:"cursorStdio"`
	CursorInstallHTTP    string                      `json:"cursorInstallHttpLink,omitempty"`
	CursorInstallStdio   string                      `json:"cursorInstallStdioLink,omitempty"`
	CursorServerName     string                      `json:"cursorServerName"`
	ApiKeyToken          string                      `json:"apiKeyToken,omitempty"`
	ApiKeyMask           string                      `json:"apiKeyMask,omitempty"`
	ApiKeyID             string                      `json:"apiKeyId,omitempty"`
	ServerEnvHint        string                      `json:"serverEnvHint"`
}

func mcpAuthBearer(apiKey string) string {
	k := strings.TrimSpace(apiKey)
	if k == "" {
		return "Bearer at-your-api-key"
	}
	if strings.HasPrefix(strings.ToLower(k), "bearer ") {
		return k
	}
	return "Bearer " + k
}

func mcpAPIKeyValue(apiKey string) string {
	k := strings.TrimSpace(apiKey)
	if k == "" {
		return "at-your-api-key"
	}
	return strings.TrimPrefix(strings.TrimPrefix(k, "Bearer "), "bearer ")
}

// BuildMcpIntegrationGuide assembles copy-paste snippets for Cursor / Claude Desktop.
// apiKey is optional plaintext token to embed in install links (only when freshly issued).
func BuildMcpIntegrationGuide(httpCfg config.MCPHTTP, listenAddr string, r *http.Request, svc Service, envs []Environment, apiKey string) McpIntegrationGuide {
	origin := mcpRequestOrigin(r)
	path := httpCfg.Path
	if path == "" {
		path = "/mcp"
	}
	mcpURL := strings.TrimRight(origin, "/") + path
	apiBase := httpCfg.APIBaseURL
	if apiBase == "" {
		apiBase = loopbackAPIBaseURL(listenAddr)
	}

	defEnvID := ""
	intEnvs := make([]McpIntegrationEnvironment, 0, len(envs))
	for i, e := range envs {
		intEnvs = append(intEnvs, McpIntegrationEnvironment{ID: e.ID.String(), Name: e.Name})
		if i == 0 {
			defEnvID = e.ID.String()
		}
	}

	pid := svc.ProjectID.String()
	sid := svc.ID.String()
	serverName := cursorServerName(sid)
	keyValue := mcpAPIKeyValue(apiKey)

	cursorHTTP, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			serverName: map[string]any{
				"url": mcpURL,
				"headers": map[string]string{
					"Authorization": mcpAuthBearer(apiKey),
				},
			},
		},
	})
	cursorStdio, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			serverName: map[string]any{
				"type":    "stdio",
				"command": stdioMCPCommand(),
				"env": map[string]string{
					"AUTOTEST_API_BASE_URL":   apiBase,
					"AUTOTEST_API_KEY":        keyValue,
					"AUTOTEST_PROJECT_ID":     pid,
					"AUTOTEST_SERVICE_ID":     sid,
					"AUTOTEST_ENVIRONMENT_ID": defEnvID,
				},
			},
		},
	})

	serverHint := "在 API 进程环境变量中设置 MCP_HTTP_ENABLED=true 并重启服务后，HTTP 方式方可连接。"
	if !httpCfg.Enabled {
		serverHint = "当前 API 未启用 MCP HTTP（MCP_HTTP_ENABLED 未开启）。可改用下方 stdio 独立进程，或为 API 设置 MCP_HTTP_ENABLED=true 后重启。"
	}
	httpTransport := map[string]any{
		"url": mcpURL,
		"headers": map[string]string{
			"Authorization": mcpAuthBearer(apiKey),
		},
	}
	stdioTransport := map[string]any{
		"type":    "stdio",
		"command": stdioMCPCommand(),
		"env": map[string]string{
			"AUTOTEST_API_BASE_URL":   apiBase,
			"AUTOTEST_API_KEY":        keyValue,
			"AUTOTEST_PROJECT_ID":     pid,
			"AUTOTEST_SERVICE_ID":     sid,
			"AUTOTEST_ENVIRONMENT_ID": defEnvID,
		},
	}

	guide := McpIntegrationGuide{
		ServiceMcpEnabled:    svc.McpEnabled,
		ServerMcpHttpEnabled: httpCfg.Enabled,
		McpHttpPath:          path,
		McpHttpURL:           mcpURL,
		APIBaseURL:           apiBase,
		ProjectID:            pid,
		ServiceID:            sid,
		DefaultEnvironmentID: defEnvID,
		Environments:         intEnvs,
		RequiredScopes: apikey.MCPAutomationScopes(),
		CursorHTTP:       cursorHTTP,
		CursorStdio:      cursorStdio,
		CursorServerName: serverName,
		ServerEnvHint:    serverHint,
	}
	if httpCfg.Enabled {
		guide.CursorInstallHTTP, _ = CursorInstallLink(serverName, httpTransport)
	}
	guide.CursorInstallStdio, _ = CursorInstallLink(serverName, stdioTransport)
	return guide
}

func stdioMCPCommand() string {
	// Cursor mcp.json supports ${workspaceFolder} after install.
	return "${workspaceFolder}/bin/autotest-mcp"
}

func mcpRequestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	host := strings.TrimSpace(r.Host)
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if host == "" {
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func loopbackAPIBaseURL(listenAddr string) string {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return fmt.Sprintf("http://127.0.0.1%s/api/v1", addr)
		}
		return "http://127.0.0.1:8080/api/v1"
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s/api/v1", net.JoinHostPort(host, port))
}

// PatchMcpIntegrationEnvironment updates snippets when the UI picks another default environment.
// Preserves ApiKeyToken on the guide when rebuilding stdio install link.
func PatchMcpIntegrationEnvironment(guide McpIntegrationGuide, environmentID uuid.UUID) McpIntegrationGuide {
	apiKey := guide.ApiKeyToken
	eid := environmentID.String()
	guide.DefaultEnvironmentID = eid

	var stdio map[string]any
	if err := json.Unmarshal(guide.CursorStdio, &stdio); err == nil {
		if servers, ok := stdio["mcpServers"].(map[string]any); ok {
			if at, ok := servers[guide.CursorServerName].(map[string]any); ok {
				if env, ok := at["env"].(map[string]string); ok {
					env["AUTOTEST_ENVIRONMENT_ID"] = eid
					at["env"] = env
					servers[guide.CursorServerName] = at
					stdio["mcpServers"] = servers
					guide.CursorStdio, _ = json.Marshal(stdio)
				}
			}
		}
	}

	if guide.CursorServerName != "" {
		stdioTransport := map[string]any{
			"type":    "stdio",
			"command": stdioMCPCommand(),
			"env": map[string]string{
				"AUTOTEST_API_BASE_URL":   guide.APIBaseURL,
				"AUTOTEST_API_KEY":        mcpAPIKeyValue(apiKey),
				"AUTOTEST_PROJECT_ID":     guide.ProjectID,
				"AUTOTEST_SERVICE_ID":     guide.ServiceID,
				"AUTOTEST_ENVIRONMENT_ID": eid,
			},
		}
		guide.CursorInstallStdio, _ = CursorInstallLink(guide.CursorServerName, stdioTransport)
		if guide.ServerMcpHttpEnabled {
			guide.CursorInstallHTTP, _ = CursorInstallLink(guide.CursorServerName, map[string]any{
				"url": guide.McpHttpURL,
				"headers": map[string]string{
					"Authorization": mcpAuthBearer(apiKey),
				},
			})
		}
	}
	return guide
}
