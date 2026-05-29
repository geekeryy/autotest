package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/scenariogen"

	"github.com/google/uuid"
)

func generateCoverageScenariosTool(deps Deps) aitools.Tool {
	return writeTool("generate_coverage_scenarios",
		"一键生成覆盖当前服务全部 OpenAPI 接口的可运行测试场景（按模块拆分为多个场景）。"+
			"会自动补齐缺失的请求模板、注入登录凭据与 Bearer Token 链式引用，适用于用户说「生成覆盖全部功能的测试场景」。"+
			"生成后请提示用户在前端选择环境并点击运行；本工具不执行场景。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "目标服务 UUID"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil || deps.Scenarios == nil || deps.Specs == nil {
				return nil, errors.New("generate_coverage_scenarios: 平台服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
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
			gen := scenariogen.NewGenerator(scenariogen.Deps{
				Cases:     deps.Cases,
				Scenarios: deps.Scenarios,
				Specs:     deps.Specs,
				Generator: deps.Generator,
			})
			result, err := gen.GenerateCoverage(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_scenarios: %w", err)
			}
			return result, nil
		})
}
