// builtin_nl.go 注册自然语言编排工具 describe_scenario_in_natural_language。
// 该工具将用户的自然语言描述转换为可运行的测试场景。
package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/ainl"
	"autotest/internal/aitools"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// NLToolDeps 是自然语言编排工具的额外依赖。
type NLToolDeps struct {
	AI    ChatProvider
	Specs SpecRepository
}

// describeScenarioInNLTool 创建自然语言场景编排工具。
func describeScenarioInNLTool(nlDeps NLToolDeps) aitools.Tool {
	return aitools.Tool{
		Name: "describe_scenario_in_natural_language",
		Description: "将用户的自然语言描述转换为可运行的测试场景。输入一段中文描述（如'先登录，然后查询用户列表，验证返回 code 为 0'），" +
			"工具会自动解析操作序列、匹配已有接口、生成步骤列表和断言。返回场景预览供用户确认。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "description": {
                    "type": "string",
                    "description": "自然语言描述的测试流程（中文）"
                },
                "serviceId": {
                    "type": "string",
                    "description": "目标服务的 UUID"
                }
            },
            "required": ["description", "serviceId"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if nlDeps.AI == nil {
				return nil, errors.New("describe_scenario_in_natural_language: AI 服务未配置")
			}
			if nlDeps.Specs == nil {
				return nil, errors.New("describe_scenario_in_natural_language: spec 仓库未配置")
			}

			var p struct {
				Description string `json:"description"`
				ServiceID   string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("describe_scenario_in_natural_language: 解析参数失败: %w", err)
			}

			description := strings.TrimSpace(p.Description)
			if description == "" {
				return nil, errors.New("describe_scenario_in_natural_language: description 不能为空")
			}

			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("describe_scenario_in_natural_language: %w", err)
			}

			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("describe_scenario_in_natural_language: serviceId 不是合法 UUID: %w", err)
			}

			// 创建 spec 适配器
			specAdapter := &specRepoAdapter{repo: nlDeps.Specs}

			// 创建编排引擎
			orch := ainl.NewOrchestrator(nlDeps.AI, specAdapter)

			// 执行编排
			output, err := orch.Orchestrate(ctx, ainl.OrchestratorInput{
				Description: description,
				ProjectID:   projectID,
				ServiceID:   serviceID,
			})
			if err != nil {
				return nil, fmt.Errorf("describe_scenario_in_natural_language: 编排失败: %w", err)
			}

			return output, nil
		},
	}
}

// specRepoAdapter 将 builtin.SpecRepository 接口适配到 ainl.SpecRepository。
type specRepoAdapter struct {
	repo SpecRepository
}

func (a *specRepoAdapter) ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.Endpoint, error) {
	return a.repo.ListEndpoints(ctx, projectID, serviceID)
}
