package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	testcase "autotest/internal/case"

	"github.com/google/uuid"
)

// caseDigest hides the bulky Request / Assertions JSON when listing
// templates. The model can call get_case to look at a single template's
// body once it has narrowed down the candidates.
type caseDigest struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"projectId"`
	ServiceID  uuid.UUID  `json:"serviceId"`
	EndpointID *uuid.UUID `json:"endpointId,omitempty"`
	Name       string     `json:"name"`
	Method     string     `json:"method"`
	Path       string     `json:"path"`
	Source     string     `json:"source"`
	Status     string     `json:"status"`
}

func toCaseDigests(cs []testcase.TestCase) []caseDigest {
	out := make([]caseDigest, 0, len(cs))
	for _, c := range cs {
		out = append(out, caseDigest{
			ID:         c.ID,
			ProjectID:  c.ProjectID,
			ServiceID:  c.ServiceID,
			EndpointID: c.EndpointID,
			Name:       c.Name,
			Method:     c.Method,
			Path:       c.Path,
			Source:     string(c.Source),
			Status:     string(c.Status),
		})
	}
	return out
}

func listCasesTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "list_cases",
		Description: "列出某个项目（可选 serviceId）下的接口请求模板摘要（id / name / method / path / source / endpointId）。" +
			"AI 生成场景的 API 步骤需要 testCaseId，先用本工具找现成模板，找不到再调 create_case_from_endpoint。" +
			"返回不含 request / assertions 大字段，需要时再 get_case。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "可选；填了就按服务过滤"},
                "filter":    {"type": "string", "description": "可选；按 name / method / path 子串过滤（不区分大小写）"}
            },
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil {
				return nil, errors.New("list_cases: case 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
				Filter    string `json:"filter"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_cases: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_cases: %w", err)
			}
			filter := testcase.ListFilter{ProjectID: projectID}
			if s := strings.TrimSpace(p.ServiceID); s != "" {
				sid, err := uuid.Parse(s)
				if err != nil {
					return nil, fmt.Errorf("list_cases: serviceId 不是合法 UUID: %w", err)
				}
				filter.ServiceID = sid
			}
			cs, err := deps.Cases.List(ctx, filter)
			if err != nil {
				return nil, fmt.Errorf("list_cases: 查询失败: %w", err)
			}
			digests := toCaseDigests(cs)
			needle := strings.ToLower(strings.TrimSpace(p.Filter))
			if needle != "" {
				filtered := digests[:0]
				for _, d := range digests {
					blob := strings.ToLower(d.Name + " " + d.Method + " " + d.Path)
					if strings.Contains(blob, needle) {
						filtered = append(filtered, d)
					}
				}
				digests = filtered
			}
			return map[string]any{
				"cases": digests,
				"count": len(digests),
			}, nil
		},
	}
}

func getCaseTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "get_case",
		Description: "查询单个接口模板或已保存测试用例的详情（method/path/请求模板 JSON/断言/状态/" +
			"创建更新时间）。当用户上下文里出现 caseId 或 testCaseId 字段，需要补充该 case 的请求/断言细节时使用。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "caseId": {
                    "type": "string",
                    "description": "接口模板或已保存测试用例的 UUID"
                }
            },
            "required": ["caseId"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil {
				return nil, errors.New("get_case: case 服务未配置")
			}
			var p struct {
				CaseID string `json:"caseId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_case: 解析参数失败: %w", err)
			}
			id, err := uuid.Parse(strings.TrimSpace(p.CaseID))
			if err != nil {
				return nil, fmt.Errorf("get_case: caseId 不是合法 UUID: %w", err)
			}
			tc, err := deps.Cases.Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get_case: 查询失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, tc.ProjectID); err != nil {
				return nil, fmt.Errorf("get_case: %w", err)
			}
			return tc, nil
		},
	}
}

// createCaseFromEndpointTool creates an API request template. API steps
// inside a scenario reference one of these templates by id, so the AI
// must usually create the template first when the user only described
// an endpoint by method+path.
//
// We accept either `endpointId` or `method` + `path`:
//   - When endpointId is given we copy method+path from the spec to keep
//     the template aligned with the OpenAPI definition.
//   - When only method+path are given we trust the caller (matches the
//     behavior of POST /projects/.../cases/manual in the API).
func createCaseFromEndpointTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "create_case_from_endpoint",
		Description: "为某个接口创建一个可运行的请求模板（test_cases 行）。API 类型的场景步骤需要 testCaseId，本工具是 AI 生成场景前的常规前置动作。" +
			"endpointId 与 method+path 至少传一组，建议优先 endpointId。" +
			"request 缺省为 `{}`，assertions 缺省为 `[]`；AI 可以同时给出基础断言（如 status==200）来减少用户后续工作。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":  {"type": "string"},
                "endpointId": {"type": "string", "description": "可选；建议优先提供，会用 spec 中的 method+path 覆盖入参"},
                "method":     {"type": "string", "description": "HTTP 方法；endpointId 缺省时必填"},
                "path":       {"type": "string", "description": "OpenAPI 路径；endpointId 缺省时必填"},
                "name":       {"type": "string", "description": "接口模板名称（人类可读）"},
                "request":    {"description": "可选；完整请求模板 JSON（pathVars / query / headers / body）。缺省为 {}", "type": ["object", "null"]},
                "assertions": {"description": "可选；断言数组。缺省为 []", "type": ["array", "null"]}
            },
            "required": ["serviceId", "name"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil {
				return nil, errors.New("create_case_from_endpoint: case 服务未配置")
			}
			var p struct {
				ServiceID  string          `json:"serviceId"`
				EndpointID string          `json:"endpointId"`
				Method     string          `json:"method"`
				Path       string          `json:"path"`
				Name       string          `json:"name"`
				Request    json.RawMessage `json:"request"`
				Assertions json.RawMessage `json:"assertions"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_case_from_endpoint: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_case_from_endpoint: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("create_case_from_endpoint: serviceId 不是合法 UUID: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("create_case_from_endpoint: name 不能为空")
			}

			input := testcase.CreateManualInput{
				ProjectID: projectID,
				ServiceID: serviceID,
				Name:      name,
			}
			if epIDStr := strings.TrimSpace(p.EndpointID); epIDStr != "" {
				if deps.Specs == nil {
					return nil, errors.New("create_case_from_endpoint: spec 仓库未配置，无法解析 endpointId")
				}
				epID, err := uuid.Parse(epIDStr)
				if err != nil {
					return nil, fmt.Errorf("create_case_from_endpoint: endpointId 不是合法 UUID: %w", err)
				}
				ep, err := deps.Specs.GetEndpointByID(ctx, epID)
				if err != nil {
					return nil, fmt.Errorf("create_case_from_endpoint: 查询 endpoint 失败: %w", err)
				}
				if ep.ServiceID != serviceID {
					return nil, fmt.Errorf("create_case_from_endpoint: endpoint 属于服务 %s，与入参 serviceId %s 不一致", ep.ServiceID, serviceID)
				}
				input.EndpointID = &ep.ID
				input.Method = ep.Method
				input.Path = ep.Path
			} else {
				method := strings.ToUpper(strings.TrimSpace(p.Method))
				path := strings.TrimSpace(p.Path)
				if method == "" || path == "" {
					return nil, errors.New("create_case_from_endpoint: 未提供 endpointId 时 method 与 path 必填")
				}
				input.Method = method
				input.Path = path
			}
			if len(p.Request) == 0 {
				input.Request = json.RawMessage(`{}`)
			} else {
				input.Request = p.Request
			}
			if len(p.Assertions) == 0 {
				input.Assertions = json.RawMessage(`[]`)
			} else {
				input.Assertions = p.Assertions
			}

			tc, err := deps.Cases.CreateManual(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("create_case_from_endpoint: 创建失败: %w", err)
			}
			return tc, nil
		},
	}
}

// updateCaseAssertionsTool replaces a test case's assertions array. Marked
// Mutating=true so the chat loop pauses and asks the user before invoking
// it. The schema accepts the full assertions JSON array; partial merges
// are intentionally out of scope to keep the audit trail readable.
func updateCaseAssertionsTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "update_case_assertions",
		Description: "覆盖测试用例的断言数组。调用前模型应已经通过 get_case 拿到现有断言，并在 assistant " +
			"文本里向用户解释将要替换为什么。assertions 必须为完整 JSON 数组，平台会原样保存。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "caseId": {
                    "type": "string",
                    "description": "测试用例的 UUID"
                },
                "assertions": {
                    "type": "array",
                    "description": "完整断言数组，将整体替换现有断言",
                    "items": {"type": "object"}
                }
            },
            "required": ["caseId", "assertions"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil {
				return nil, errors.New("update_case_assertions: case 服务未配置")
			}
			var p struct {
				CaseID     string          `json:"caseId"`
				Assertions json.RawMessage `json:"assertions"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_case_assertions: 解析参数失败: %w", err)
			}
			id, err := uuid.Parse(strings.TrimSpace(p.CaseID))
			if err != nil {
				return nil, fmt.Errorf("update_case_assertions: caseId 不是合法 UUID: %w", err)
			}
			if len(p.Assertions) == 0 {
				return nil, errors.New("update_case_assertions: assertions 不能为空")
			}
			// Reverse-check project ownership before mutating: caseId on its
			// own would otherwise let a malicious model patch test cases
			// across projects.
			existing, err := deps.Cases.Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("update_case_assertions: 查询测试用例失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, existing.ProjectID); err != nil {
				return nil, fmt.Errorf("update_case_assertions: %w", err)
			}
			tc, err := deps.Cases.Patch(ctx, id, testcase.PatchInput{Assertions: p.Assertions})
			if err != nil {
				return nil, fmt.Errorf("update_case_assertions: 更新失败: %w", err)
			}
			return tc, nil
		},
	}
}
