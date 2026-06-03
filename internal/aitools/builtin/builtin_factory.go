package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aifactory"
	"autotest/internal/aitools"

	"github.com/google/uuid"
)

func generateTestDataTool(deps Deps) aitools.Tool {
	return writeTool("generate_test_data",
		"基于接口 OpenAPI Schema 语义分析，智能生成测试数据。支持 normal（正常）、boundary（边界）、stress（压力）三种场景模式，以及 zh_CN / en_US 两种区域。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "endpointId":  {"type": "string", "description": "接口 Endpoint 的 UUID"},
                "scenario":    {"type": "string", "enum": ["normal", "boundary", "stress"], "description": "场景模式：normal（默认）正常值、boundary 边界值、stress 压力值"},
                "count":       {"type": "integer", "minimum": 1, "maximum": 1000, "default": 10, "description": "生成行数，默认 10"},
                "locale":      {"type": "string", "enum": ["zh_CN", "en_US"], "description": "区域设置，默认 zh_CN"},
                "constraints": {"type": "object", "description": "可选的字段级约束覆盖，key 为字段名，value 为固定值"}
            },
            "required": ["endpointId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Specs == nil {
				return nil, errors.New("generate_test_data: spec 仓库未配置")
			}

			var p struct {
				EndpointID  string         `json:"endpointId"`
				Scenario    string         `json:"scenario"`
				Count       int            `json:"count"`
				Locale      string         `json:"locale"`
				Constraints map[string]any `json:"constraints"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("generate_test_data: 解析参数失败: %w", err)
			}

			endpointID, err := uuid.Parse(strings.TrimSpace(p.EndpointID))
			if err != nil {
				return nil, fmt.Errorf("generate_test_data: endpointId 不是合法 UUID: %w", err)
			}

			// 查询 Endpoint 获取 RequestSchema
			endpoint, err := deps.Specs.GetEndpointByID(ctx, endpointID)
			if err != nil {
				return nil, fmt.Errorf("generate_test_data: 查询接口失败: %w", err)
			}

			// 构建工厂输入
			input := aifactory.GenerateInput{
				EndpointID:  endpointID,
				Scenario:    p.Scenario,
				Count:       p.Count,
				Locale:      p.Locale,
				Constraints: p.Constraints,
			}
			if err := input.Validate(); err != nil {
				return nil, fmt.Errorf("generate_test_data: 参数校验失败: %w", err)
			}

			// 调用数据工厂生成
			factory := aifactory.NewFactory()
			output, err := factory.Generate(endpoint.RequestSchema, input)
			if err != nil {
				return nil, fmt.Errorf("generate_test_data: 生成失败: %w", err)
			}

			return output, nil
		})
}
