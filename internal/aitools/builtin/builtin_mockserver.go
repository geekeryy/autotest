package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/mockserver"

	"github.com/google/uuid"
)

func listMockServersTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_mock_servers",
		"列出当前项目下全部 Mock Server 配置及运行状态。",
		rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("list_mock_servers: mock server 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_mock_servers: %w", err)
			}
			servers, err := deps.MockServers.ListServers(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("list_mock_servers: 查询失败: %w", err)
			}
			return map[string]any{
				"servers": servers,
				"count":   len(servers),
			}, nil
		})
}

func getMockServerStatusTool(deps Deps) aitools.Tool {
	return readOnlyTool("get_mock_server_status",
		"查询单个 Mock Server 的运行时状态（running / port / url）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId": {"type": "string"}
            },
            "required": ["serverId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("get_mock_server_status: mock server 服务未配置")
			}
			var p struct {
				ServerID string `json:"serverId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_mock_server_status: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("get_mock_server_status: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("get_mock_server_status: serverId 不是合法 UUID: %w", err)
			}
			status, err := deps.MockServers.GetStatus(ctx, projectID, serverID)
			if err != nil {
				return nil, fmt.Errorf("get_mock_server_status: 查询失败: %w", err)
			}
			return status, nil
		})
}

func createMockServerTool(deps Deps) aitools.Tool {
	return writeTool("create_mock_server",
		"创建 Mock Server（name / port / autoStart / description）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "port":        {"type": "integer", "minimum": 1},
                "autoStart":   {"type": "boolean"}
            },
            "required": ["name", "port"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("create_mock_server: mock server 服务未配置")
			}
			var p struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Port        int    `json:"port"`
				AutoStart   bool   `json:"autoStart"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_mock_server: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_mock_server: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("create_mock_server: name 不能为空")
			}
			if p.Port <= 0 {
				return nil, errors.New("create_mock_server: port 必须大于 0")
			}
			srv, err := deps.MockServers.CreateServer(ctx, projectID, mockserver.MockServerInput{
				Name:        name,
				Description: strings.TrimSpace(p.Description),
				Port:        p.Port,
				AutoStart:   p.AutoStart,
			})
			if err != nil {
				return nil, fmt.Errorf("create_mock_server: 创建失败: %w", err)
			}
			return srv, nil
		})
}

func updateMockServerTool(deps Deps) aitools.Tool {
	return writeTool("update_mock_server",
		"更新 Mock Server 配置（name / port / autoStart / description）。运行中不可修改。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId":    {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "port":        {"type": "integer", "minimum": 1},
                "autoStart":   {"type": "boolean"}
            },
            "required": ["serverId", "name", "port"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("update_mock_server: mock server 服务未配置")
			}
			var p struct {
				ServerID    string `json:"serverId"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Port        int    `json:"port"`
				AutoStart   bool   `json:"autoStart"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_mock_server: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_mock_server: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("update_mock_server: serverId 不是合法 UUID: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("update_mock_server: name 不能为空")
			}
			if p.Port <= 0 {
				return nil, errors.New("update_mock_server: port 必须大于 0")
			}
			srv, err := deps.MockServers.UpdateServer(ctx, projectID, serverID, mockserver.MockServerInput{
				Name:        name,
				Description: strings.TrimSpace(p.Description),
				Port:        p.Port,
				AutoStart:   p.AutoStart,
			})
			if err != nil {
				return nil, fmt.Errorf("update_mock_server: 更新失败: %w", err)
			}
			return srv, nil
		})
}

func deleteMockServerTool(deps Deps) aitools.Tool {
	return deleteTool("delete_mock_server",
		"删除 Mock Server 及其全部路由（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId": {"type": "string"}
            },
            "required": ["serverId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("delete_mock_server: mock server 服务未配置")
			}
			var p struct {
				ServerID string `json:"serverId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_mock_server: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_mock_server: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("delete_mock_server: serverId 不是合法 UUID: %w", err)
			}
			if err := deps.MockServers.DeleteServer(ctx, projectID, serverID); err != nil {
				return nil, fmt.Errorf("delete_mock_server: 删除失败: %w", err)
			}
			return map[string]any{
				"serverId": serverID,
				"deleted":  true,
			}, nil
		})
}

func listMockRoutesTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_mock_routes",
		"列出某 Mock Server 下的全部路由规则。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId": {"type": "string"}
            },
            "required": ["serverId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("list_mock_routes: mock server 服务未配置")
			}
			var p struct {
				ServerID string `json:"serverId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_mock_routes: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_mock_routes: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("list_mock_routes: serverId 不是合法 UUID: %w", err)
			}
			routes, err := deps.MockServers.ListRoutes(ctx, projectID, serverID)
			if err != nil {
				return nil, fmt.Errorf("list_mock_routes: 查询失败: %w", err)
			}
			return map[string]any{
				"routes": routes,
				"count":  len(routes),
			}, nil
		})
}

func createMockRouteTool(deps Deps) aitools.Tool {
	return writeTool("create_mock_route",
		"为 Mock Server 新增路由（method / path / 响应配置 / 匹配条件等）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId":         {"type": "string"},
                "method":           {"type": "string"},
                "path":             {"type": "string"},
                "priority":         {"type": "integer"},
                "enabled":          {"type": "boolean"},
                "requestMatch":     {"type": ["object", "null"]},
                "responseMode":     {"type": "string", "enum": ["body", "redirect"]},
                "responseStatus":   {"type": "integer"},
                "responseHeaders":  {"type": ["object", "null"]},
                "redirectLocation": {"type": "string"},
                "responseBody":     {"type": "string"},
                "responseBodyType": {"type": "string", "enum": ["json", "text", "raw"]},
                "delayMillis":      {"type": "integer"}
            },
            "required": ["serverId", "method", "path"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("create_mock_route: mock server 服务未配置")
			}
			var p mockRouteArgs
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_mock_route: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_mock_route: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("create_mock_route: serverId 不是合法 UUID: %w", err)
			}
			input, err := p.toInput()
			if err != nil {
				return nil, fmt.Errorf("create_mock_route: %w", err)
			}
			route, err := deps.MockServers.CreateRoute(ctx, projectID, serverID, input)
			if err != nil {
				return nil, fmt.Errorf("create_mock_route: 创建失败: %w", err)
			}
			return route, nil
		})
}

func updateMockRouteTool(deps Deps) aitools.Tool {
	return writeTool("update_mock_route",
		"更新 Mock Server 路由配置。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId":         {"type": "string"},
                "routeId":          {"type": "string"},
                "method":           {"type": "string"},
                "path":             {"type": "string"},
                "priority":         {"type": "integer"},
                "enabled":          {"type": "boolean"},
                "requestMatch":     {"type": ["object", "null"]},
                "responseMode":     {"type": "string", "enum": ["body", "redirect"]},
                "responseStatus":   {"type": "integer"},
                "responseHeaders":  {"type": ["object", "null"]},
                "redirectLocation": {"type": "string"},
                "responseBody":     {"type": "string"},
                "responseBodyType": {"type": "string", "enum": ["json", "text", "raw"]},
                "delayMillis":      {"type": "integer"}
            },
            "required": ["serverId", "routeId", "method", "path"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("update_mock_route: mock server 服务未配置")
			}
			var p mockRouteArgs
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_mock_route: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_mock_route: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("update_mock_route: serverId 不是合法 UUID: %w", err)
			}
			routeID, err := uuid.Parse(strings.TrimSpace(p.RouteID))
			if err != nil {
				return nil, fmt.Errorf("update_mock_route: routeId 不是合法 UUID: %w", err)
			}
			input, err := p.toInput()
			if err != nil {
				return nil, fmt.Errorf("update_mock_route: %w", err)
			}
			route, err := deps.MockServers.UpdateRoute(ctx, projectID, serverID, routeID, input)
			if err != nil {
				return nil, fmt.Errorf("update_mock_route: 更新失败: %w", err)
			}
			return route, nil
		})
}

func deleteMockRouteTool(deps Deps) aitools.Tool {
	return deleteTool("delete_mock_route",
		"删除 Mock Server 路由（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serverId": {"type": "string"},
                "routeId":  {"type": "string"}
            },
            "required": ["serverId", "routeId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockServers == nil {
				return nil, errors.New("delete_mock_route: mock server 服务未配置")
			}
			var p struct {
				ServerID string `json:"serverId"`
				RouteID  string `json:"routeId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_mock_route: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_mock_route: %w", err)
			}
			serverID, err := uuid.Parse(strings.TrimSpace(p.ServerID))
			if err != nil {
				return nil, fmt.Errorf("delete_mock_route: serverId 不是合法 UUID: %w", err)
			}
			routeID, err := uuid.Parse(strings.TrimSpace(p.RouteID))
			if err != nil {
				return nil, fmt.Errorf("delete_mock_route: routeId 不是合法 UUID: %w", err)
			}
			if err := deps.MockServers.DeleteRoute(ctx, projectID, serverID, routeID); err != nil {
				return nil, fmt.Errorf("delete_mock_route: 删除失败: %w", err)
			}
			return map[string]any{
				"serverId": serverID,
				"routeId":  routeID,
				"deleted":  true,
			}, nil
		})
}

type mockRouteArgs struct {
	ServerID         string          `json:"serverId"`
	RouteID          string          `json:"routeId"`
	Method           string          `json:"method"`
	Path             string          `json:"path"`
	Priority         int             `json:"priority"`
	Enabled          *bool           `json:"enabled"`
	RequestMatch     json.RawMessage `json:"requestMatch"`
	ResponseMode     string          `json:"responseMode"`
	ResponseStatus   int             `json:"responseStatus"`
	ResponseHeaders  json.RawMessage `json:"responseHeaders"`
	RedirectLocation string          `json:"redirectLocation"`
	ResponseBody     string          `json:"responseBody"`
	ResponseBodyType string          `json:"responseBodyType"`
	DelayMillis      int             `json:"delayMillis"`
}

func (p mockRouteArgs) toInput() (mockserver.MockRouteInput, error) {
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	path := strings.TrimSpace(p.Path)
	if method == "" || path == "" {
		return mockserver.MockRouteInput{}, errors.New("method 与 path 不能为空")
	}
	return mockserver.MockRouteInput{
		Method:           method,
		Path:             path,
		Priority:         p.Priority,
		Enabled:          p.Enabled,
		RequestMatch:     defaultJSONObject(p.RequestMatch),
		ResponseMode:     strings.TrimSpace(p.ResponseMode),
		ResponseStatus:   p.ResponseStatus,
		ResponseHeaders:  defaultJSONObject(p.ResponseHeaders),
		RedirectLocation: strings.TrimSpace(p.RedirectLocation),
		ResponseBody:     p.ResponseBody,
		ResponseBodyType: strings.TrimSpace(p.ResponseBodyType),
		DelayMillis:      p.DelayMillis,
	}, nil
}
