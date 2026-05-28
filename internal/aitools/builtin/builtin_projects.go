package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/project"

	"github.com/google/uuid"
)

func createServiceTool(deps Deps) aitools.Tool {
	return writeTool("create_service",
		"在当前项目下创建服务（name / description）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "name":        {"type": "string"},
                "description": {"type": "string"}
            },
            "required": ["name"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("create_service: project 服务未配置")
			}
			var p struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_service: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_service: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("create_service: name 不能为空")
			}
			svc, err := deps.Projects.CreateService(ctx, projectID, project.CreateServiceInput{
				Name:        name,
				Description: strings.TrimSpace(p.Description),
			})
			if err != nil {
				return nil, fmt.Errorf("create_service: 创建失败: %w", err)
			}
			return svc, nil
		})
}

func updateServiceTool(deps Deps) aitools.Tool {
	return writeTool("update_service",
		"更新服务的 name / description。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":   {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"}
            },
            "required": ["serviceId", "name"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("update_service: project 服务未配置")
			}
			var p struct {
				ServiceID   string `json:"serviceId"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_service: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_service: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("update_service: serviceId 不是合法 UUID: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("update_service: name 不能为空")
			}
			svc, err := deps.Projects.UpdateService(ctx, projectID, serviceID, project.UpdateServiceInput{
				Name:        name,
				Description: strings.TrimSpace(p.Description),
			})
			if err != nil {
				return nil, fmt.Errorf("update_service: 更新失败: %w", err)
			}
			return svc, nil
		})
}

func deleteServiceTool(deps Deps) aitools.Tool {
	return deleteTool("delete_service",
		"删除服务及其关联配置（不可恢复，请谨慎）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("delete_service: project 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_service: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_service: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("delete_service: serviceId 不是合法 UUID: %w", err)
			}
			if err := deps.Projects.DeleteService(ctx, projectID, serviceID); err != nil {
				return nil, fmt.Errorf("delete_service: 删除失败: %w", err)
			}
			return map[string]any{
				"serviceId": serviceID,
				"deleted":   true,
			}, nil
		})
}

func createServiceEnvironmentTool(deps Deps) aitools.Tool {
	return writeTool("create_service_environment",
		"为指定服务创建运行环境（name / baseUrl / variables / auth）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"},
                "name":      {"type": "string"},
                "baseUrl":   {"type": "string"},
                "variables": {"type": ["object", "null"]},
                "auth":      {"type": ["object", "null"]}
            },
            "required": ["serviceId", "name", "baseUrl"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("create_service_environment: project 服务未配置")
			}
			var p struct {
				ServiceID string          `json:"serviceId"`
				Name      string          `json:"name"`
				BaseURL   string          `json:"baseUrl"`
				Variables json.RawMessage `json:"variables"`
				Auth      json.RawMessage `json:"auth"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_service_environment: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_service_environment: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("create_service_environment: serviceId 不是合法 UUID: %w", err)
			}
			env, err := deps.Projects.CreateServiceEnvironment(ctx, projectID, serviceID, project.CreateEnvironmentInput{
				Name:      strings.TrimSpace(p.Name),
				BaseURL:   strings.TrimSpace(p.BaseURL),
				Variables: defaultJSONObject(p.Variables),
				Auth:      defaultJSONObject(p.Auth),
			})
			if err != nil {
				return nil, fmt.Errorf("create_service_environment: 创建失败: %w", err)
			}
			return env, nil
		})
}

func updateServiceEnvironmentTool(deps Deps) aitools.Tool {
	return writeTool("update_service_environment",
		"更新服务级运行环境（name / baseUrl / variables / auth）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":     {"type": "string"},
                "environmentId": {"type": "string"},
                "name":          {"type": "string"},
                "baseUrl":       {"type": "string"},
                "variables":     {"type": ["object", "null"]},
                "auth":          {"type": ["object", "null"]}
            },
            "required": ["serviceId", "environmentId", "name", "baseUrl"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("update_service_environment: project 服务未配置")
			}
			var p struct {
				ServiceID     string          `json:"serviceId"`
				EnvironmentID string          `json:"environmentId"`
				Name          string          `json:"name"`
				BaseURL       string          `json:"baseUrl"`
				Variables     json.RawMessage `json:"variables"`
				Auth          json.RawMessage `json:"auth"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_service_environment: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_service_environment: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("update_service_environment: serviceId 不是合法 UUID: %w", err)
			}
			environmentID, err := uuid.Parse(strings.TrimSpace(p.EnvironmentID))
			if err != nil {
				return nil, fmt.Errorf("update_service_environment: environmentId 不是合法 UUID: %w", err)
			}
			env, err := deps.Projects.UpdateServiceEnvironment(ctx, projectID, serviceID, environmentID, project.UpdateEnvironmentInput{
				Name:      strings.TrimSpace(p.Name),
				BaseURL:   strings.TrimSpace(p.BaseURL),
				Variables: defaultJSONObject(p.Variables),
				Auth:      defaultJSONObject(p.Auth),
			})
			if err != nil {
				return nil, fmt.Errorf("update_service_environment: 更新失败: %w", err)
			}
			return env, nil
		})
}

func deleteServiceEnvironmentTool(deps Deps) aitools.Tool {
	return deleteTool("delete_service_environment",
		"删除服务级运行环境（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":     {"type": "string"},
                "environmentId": {"type": "string"}
            },
            "required": ["serviceId", "environmentId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("delete_service_environment: project 服务未配置")
			}
			var p struct {
				ServiceID     string `json:"serviceId"`
				EnvironmentID string `json:"environmentId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_service_environment: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_service_environment: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("delete_service_environment: serviceId 不是合法 UUID: %w", err)
			}
			environmentID, err := uuid.Parse(strings.TrimSpace(p.EnvironmentID))
			if err != nil {
				return nil, fmt.Errorf("delete_service_environment: environmentId 不是合法 UUID: %w", err)
			}
			if err := deps.Projects.DeleteServiceEnvironment(ctx, projectID, serviceID, environmentID); err != nil {
				return nil, fmt.Errorf("delete_service_environment: 删除失败: %w", err)
			}
			return map[string]any{
				"serviceId":     serviceID,
				"environmentId": environmentID,
				"deleted":       true,
			}, nil
		})
}

func getEnvironmentTool(deps Deps) aitools.Tool {
	return readOnlyTool("get_environment",
		"查询单个服务级运行环境详情（name / baseUrl / variables / auth）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":     {"type": "string"},
                "environmentId": {"type": "string"}
            },
            "required": ["serviceId", "environmentId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("get_environment: project 服务未配置")
			}
			var p struct {
				ServiceID     string `json:"serviceId"`
				EnvironmentID string `json:"environmentId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_environment: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("get_environment: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("get_environment: serviceId 不是合法 UUID: %w", err)
			}
			environmentID, err := uuid.Parse(strings.TrimSpace(p.EnvironmentID))
			if err != nil {
				return nil, fmt.Errorf("get_environment: environmentId 不是合法 UUID: %w", err)
			}
			env, err := deps.Projects.GetServiceEnvironment(ctx, projectID, serviceID, environmentID)
			if err != nil {
				return nil, fmt.Errorf("get_environment: 查询失败: %w", err)
			}
			return env, nil
		})
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || isJSONNull(raw) {
		return json.RawMessage(`{}`)
	}
	return raw
}
