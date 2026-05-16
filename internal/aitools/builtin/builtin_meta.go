package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// endpointDigest is a slim view of spec.Endpoint that hides the heavy
// requestSchema / responseSchema fields. We use it in list_* tools to
// keep tool results compact; the model can call get_endpoint when it
// wants the full schemas.
type endpointDigest struct {
	ID          uuid.UUID `json:"id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	OperationID string    `json:"operationId,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

func toEndpointDigests(eps []spec.Endpoint) []endpointDigest {
	out := make([]endpointDigest, 0, len(eps))
	for _, ep := range eps {
		out = append(out, endpointDigest{
			ID:          ep.ID,
			Method:      ep.Method,
			Path:        ep.Path,
			OperationID: ep.OperationID,
			Summary:     ep.Summary,
			Tags:        ep.Tags,
		})
	}
	return out
}

func listServicesTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "list_services",
		Description: "列出当前会话项目下的全部服务（id / name / description）。" +
			"当用户提到接口、场景或环境，但你还不知道 serviceId 时优先调用，再决定后续动作。" +
			"平台会自动使用当前会话项目；本工具无需任何入参，不要追问用户项目 ID。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("list_services: project 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_services: %w", err)
			}
			svcs, err := deps.Projects.ListServices(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("list_services: 查询失败: %w", err)
			}
			return map[string]any{
				"services": svcs,
				"count":    len(svcs),
			}, nil
		},
	}
}

// listEndpointsTool returns the digest list under one service. The model
// is expected to call this before generating a scenario so it sees what
// real endpoints exist; without this it tends to invent paths.
func listEndpointsTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "list_endpoints",
		Description: "列出某个服务下所有 OpenAPI/Swagger 接口的摘要（method/path/operationId/summary/tags），" +
			"不含 requestSchema / responseSchema 大字段。需要看具体字段时再调用 get_endpoint。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"},
                "filter":    {"type": "string", "description": "可选，按 method/path/operationId/summary 子串过滤（不区分大小写）"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Specs == nil {
				return nil, errors.New("list_endpoints: spec 仓库未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
				Filter    string `json:"filter"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_endpoints: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_endpoints: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("list_endpoints: serviceId 不是合法 UUID: %w", err)
			}
			eps, err := deps.Specs.ListEndpoints(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("list_endpoints: 查询失败: %w", err)
			}
			digests := toEndpointDigests(eps)
			filter := strings.ToLower(strings.TrimSpace(p.Filter))
			if filter != "" {
				filtered := digests[:0]
				for _, d := range digests {
					blob := strings.ToLower(d.Method + " " + d.Path + " " + d.OperationID + " " + d.Summary)
					if strings.Contains(blob, filter) {
						filtered = append(filtered, d)
					}
				}
				digests = filtered
			}
			return map[string]any{
				"endpoints": digests,
				"count":     len(digests),
			}, nil
		},
	}
}

// getEndpointTool stays a lookup-by-method+path helper (rather than by id)
// because callers usually start from method+path strings (e.g. failure
// analysis context, user prompts).
func getEndpointTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "get_endpoint",
		Description: "在指定项目/服务下，按 method+path 精确查找 OpenAPI/Swagger 接口定义" +
			"（含 summary、tags、requestSchema、responseSchema）。method 大小写不敏感，path 必须与 spec 中字面值一致" +
			"（含 OpenAPI 占位符如 `{id}`）。当需要确认变更后字段类型/必填情况时使用。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "服务 UUID"},
                "method":    {"type": "string", "description": "HTTP 方法，例如 GET/POST"},
                "path":      {"type": "string", "description": "接口路径，含 {var} 占位符"}
            },
            "required": ["serviceId", "method", "path"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Specs == nil {
				return nil, errors.New("get_endpoint: spec 仓库未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
				Method    string `json:"method"`
				Path      string `json:"path"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_endpoint: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("get_endpoint: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("get_endpoint: serviceId 不是合法 UUID: %w", err)
			}
			method := strings.ToUpper(strings.TrimSpace(p.Method))
			path := strings.TrimSpace(p.Path)
			if method == "" || path == "" {
				return nil, errors.New("get_endpoint: method 与 path 不能为空")
			}
			endpoints, err := deps.Specs.ListEndpoints(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("get_endpoint: 查询失败: %w", err)
			}
			for _, ep := range endpoints {
				if strings.EqualFold(ep.Method, method) && ep.Path == path {
					return ep, nil
				}
			}
			return nil, fmt.Errorf("get_endpoint: 未找到 %s %s（项目 %s / 服务 %s）", method, path, projectID, serviceID)
		},
	}
}

// listEnvironmentsTool gives the AI visibility into project / service
// environments. We do not let the AI pick an environment at scenario
// generation time — that decision belongs at run time. The tool exists so
// the AI can answer "what envs are configured" questions and so a future
// run-tool can use it.
func listEnvironmentsTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "list_environments",
		Description: "列出环境配置。不传 serviceId 时返回项目级环境；传 serviceId 时返回服务级环境。" +
			"返回字段：id / name / baseUrl / projectId / serviceId。用户运行场景时再选择具体环境，AI 不负责选环境。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "可选；填了就返回该服务下的环境"}
            },
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Projects == nil {
				return nil, errors.New("list_environments: project 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_environments: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_environments: %w", err)
			}
			if strings.TrimSpace(p.ServiceID) == "" {
				envs, err := deps.Projects.ListEnvironments(ctx, projectID)
				if err != nil {
					return nil, fmt.Errorf("list_environments: 查询失败: %w", err)
				}
				return map[string]any{
					"environments": envs,
					"count":        len(envs),
					"scope":        "project",
				}, nil
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("list_environments: serviceId 不是合法 UUID: %w", err)
			}
			envs, err := deps.Projects.ListServiceEnvironments(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("list_environments: 查询失败: %w", err)
			}
			return map[string]any{
				"environments": envs,
				"count":        len(envs),
				"scope":        "service",
			}, nil
		},
	}
}
