package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/genagent"
	"autotest/internal/scenariogen"

	"github.com/google/uuid"
)

func scenariogenDeps(deps Deps) scenariogen.Deps {
	return scenariogen.Deps{
		Cases:     deps.Cases,
		Scenarios: deps.Scenarios,
		Specs:     deps.Specs,
		Generator: deps.Generator,
	}
}

func listScenarioLoginHintsTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_scenario_login_hints",
		"发现当前服务可用于场景生成的登录凭据候选：已保存的登录接口模板、环境 auth 配置、环境变量等。"+
			"生成覆盖场景或真实环境验证前应先调用；密码仅返回脱敏提示，须让用户在确认面板填写完整凭据。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "目标服务 UUID"},
                "environmentId": {"type": "string", "description": "可选：目标运行环境 UUID，用于扫描环境 auth/variables"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil || deps.Specs == nil {
				return nil, errors.New("list_scenario_login_hints: 平台服务未配置")
			}
			var p struct {
				ServiceID     string `json:"serviceId"`
				EnvironmentID string `json:"environmentId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_scenario_login_hints: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_scenario_login_hints: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("list_scenario_login_hints: serviceId 不是合法 UUID: %w", err)
			}
			var environmentID uuid.UUID
			if strings.TrimSpace(p.EnvironmentID) != "" {
				environmentID, err = uuid.Parse(strings.TrimSpace(p.EnvironmentID))
				if err != nil {
					return nil, fmt.Errorf("list_scenario_login_hints: environmentId 不是合法 UUID: %w", err)
				}
			}
			return scenariogen.CollectLoginHints(ctx, scenariogenDeps(deps), deps.Projects, projectID, serviceID, environmentID)
		})
}

func generateCoverageScenariosTool(deps Deps) aitools.Tool {
	return confirmTool("generate_coverage_scenarios",
		"一键生成覆盖当前服务全部 OpenAPI 接口的可运行测试场景（按模块拆分为多个场景）。"+
			"会自动补齐缺失的请求模板、注入 Bearer Token 链式引用；登录步优先复用已保存用例 body。"+
			"缺失的登录字段由用户在对话内确认卡片填写 loginCredentials（含 body 扩展字段）；AI 不得先在聊天里追问，应直接调用本工具挂起确认卡片。"+
			"禁止 AI 在参数中自行填写 __FILL_*。本工具不执行场景。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "目标服务 UUID"},
                "loginCredentials": `+loginCredentialsSchema+`
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil || deps.Scenarios == nil || deps.Specs == nil {
				return nil, errors.New("generate_coverage_scenarios: 平台服务未配置")
			}
			var p struct {
				ServiceID        string                             `json:"serviceId"`
				LoginCredentials *scenariogen.LoginCredentialBundle `json:"loginCredentials"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("generate_coverage_scenarios: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_scenarios: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_scenarios: serviceId 不是合法 UUID: %w", err)
			}
			gen := scenariogen.NewGenerator(scenariogenDeps(deps))
			result, err := gen.GenerateCoverage(ctx, projectID, serviceID, &scenariogen.CoverageOptions{
				LoginCredentials: p.LoginCredentials,
			})
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_scenarios: %w", err)
			}
			return result, nil
		})
}

func GatedScenarioGenTools(deps Deps) []aitools.Tool {
	if deps.GenAgent == nil {
		return nil
	}
	return []aitools.Tool{generateAndVerifyScenariosTool(deps)}
}

// GatedMutating is an alias used by builtin.All.
func GatedMutating(deps Deps) []aitools.Tool {
	return GatedScenarioGenTools(deps)
}

func generateAndVerifyScenariosTool(deps Deps) aitools.Tool {
	return confirmTool("generate_and_verify_scenarios",
		"【警告】将对真实环境执行测试场景并可能产生/删除数据。"+
			"基于 OpenAPI 依赖图生成覆盖场景，在指定 environmentId 上执行；开启 autoRepair 时自动分析失败并修复后重跑，直到全部通过或达到 maxRounds。"+
			"缺失登录字段由用户在对话内确认卡片填写 loginCredentials（含 body 扩展字段）；AI 不得先在聊天里追问，应直接调用本工具挂起确认卡片。进度通过 SSE gen_job_progress 推送。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "目标服务 UUID"},
                "environmentId": {"type": "string", "description": "真实运行环境 UUID（必填）"},
                "autoRepair": {"type": "boolean", "description": "是否自动修复失败并重跑（默认 false）"},
                "maxRounds": {"type": "integer", "description": "执行-修复闭环最大轮次（默认 3）"},
                "stopOnRealDefect": {"type": "boolean", "description": "检测到真实缺陷时是否立即停止"},
                "loginCredentials": `+loginCredentialsSchema+`
            },
            "required": ["serviceId", "environmentId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.GenAgent == nil {
				return nil, errors.New("generate_and_verify_scenarios: 生成 Agent 未配置")
			}
			if !genagent.ScenarioAutorunEnabled() {
				return nil, errors.New("generate_and_verify_scenarios: 场景生成真实环境验证未在平台配置中开启")
			}
			var p struct {
				ServiceID        string                           `json:"serviceId"`
				EnvironmentID    string                           `json:"environmentId"`
				AutoRepair       *bool                            `json:"autoRepair"`
				MaxRounds        int                              `json:"maxRounds"`
				StopOnRealDefect bool                             `json:"stopOnRealDefect"`
				LoginCredentials *scenariogen.LoginCredentialBundle `json:"loginCredentials"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: %w", err)
			}
			caller, ok := aitools.CallerFromContext(ctx)
			if !ok || caller.UserID == uuid.Nil {
				return nil, errors.New("generate_and_verify_scenarios: 缺少调用者上下文")
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: serviceId 无效: %w", err)
			}
			environmentID, err := uuid.Parse(strings.TrimSpace(p.EnvironmentID))
			if err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: environmentId 无效: %w", err)
			}
			autoRepair := false
			if p.AutoRepair != nil {
				autoRepair = *p.AutoRepair
			}
			cfg := genagent.RunConfig{
				ProjectID:        projectID,
				ServiceID:        serviceID,
				EnvironmentID:    environmentID,
				AutoRepair:       autoRepair,
				MaxRounds:        p.MaxRounds,
				StopOnRealDefect: p.StopOnRealDefect,
				LoginCredentials: p.LoginCredentials,
				CreatedBy:        caller.UserID,
			}
			if err := genagent.AssertRunAllowed(cfg); err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: %w", err)
			}
			job, err := deps.GenAgent.RunAsync(ctx, cfg)
			if err != nil {
				return nil, fmt.Errorf("generate_and_verify_scenarios: %w", err)
			}
			return map[string]any{
				"jobId":   job.ID,
				"status":  job.Status,
				"warning": "任务已在真实环境异步执行，请关注 gen_job_progress 进度事件",
			}, nil
		})
}

const loginCredentialsSchema = `{
                    "type": "object",
                    "description": "用户在确认面板提供的登录凭据；普通与 admin 登录步分别使用 user/admin；body 承载 code/union_id 等非 username/password 字段",
                    "properties": {
                        "user": {
                            "type": "object",
                            "properties": {
                                "username": {"type": "string"},
                                "password": {"type": "string"},
                                "body": {
                                    "type": "object",
                                    "description": "登录接口 request body 扩展字段",
                                    "additionalProperties": true
                                }
                            }
                        },
                        "admin": {
                            "type": "object",
                            "properties": {
                                "username": {"type": "string"},
                                "password": {"type": "string"},
                                "body": {
                                    "type": "object",
                                    "description": "Admin 登录接口 request body 扩展字段",
                                    "additionalProperties": true
                                }
                            }
                        }
                    }
                }`
