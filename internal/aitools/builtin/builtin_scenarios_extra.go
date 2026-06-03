package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

func updateScenarioTool(deps Deps) aitools.Tool {
	return writeTool("update_scenario",
		"更新场景元信息（name / description）。步骤编辑请使用 add_scenario_step / update_scenario_step 等工具。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId":  {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"}
            },
            "required": ["scenarioId", "name"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("update_scenario: scenario 服务未配置")
			}
			var p struct {
				ScenarioID  string `json:"scenarioId"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_scenario: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("update_scenario: scenarioId 不是合法 UUID: %w", err)
			}
			existing, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("update_scenario: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, existing.ProjectID); err != nil {
				return nil, fmt.Errorf("update_scenario: %w", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return nil, errors.New("update_scenario: name 不能为空")
			}
			sc, err := deps.Scenarios.Update(ctx, scenarioID, scenario.UpdateScenarioInput{
				Name:        name,
				Description: strings.TrimSpace(p.Description),
			})
			if err != nil {
				return nil, fmt.Errorf("update_scenario: 更新失败: %w", err)
			}
			return sc, nil
		})
}

func deleteScenarioTool(deps Deps) aitools.Tool {
	return deleteTool("delete_scenario",
		"删除整个测试场景（含全部步骤，不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string"}
            },
            "required": ["scenarioId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("delete_scenario: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string `json:"scenarioId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_scenario: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("delete_scenario: scenarioId 不是合法 UUID: %w", err)
			}
			sc, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("delete_scenario: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("delete_scenario: %w", err)
			}
			if err := deps.Scenarios.Delete(ctx, scenarioID); err != nil {
				return nil, fmt.Errorf("delete_scenario: 删除失败: %w", err)
			}
			return map[string]any{
				"scenarioId": scenarioID,
				"deleted":    true,
			}, nil
		})
}

func deleteScenariosTool(deps Deps) aitools.Tool {
	return deleteTool("delete_scenarios",
		"批量删除多个测试场景（含全部步骤，不可恢复）。用于删除当前服务下的全部或部分场景。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioIds": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "要删除的场景ID列表"
                }
            },
            "required": ["scenarioIds"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("delete_scenarios: scenario 服务未配置")
			}
			var p struct {
				ScenarioIDs []string `json:"scenarioIds"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_scenarios: 解析参数失败: %w", err)
			}
			if len(p.ScenarioIDs) == 0 {
				return nil, errors.New("delete_scenarios: scenarioIds 不能为空")
			}

			var deleted []string
			var failed []map[string]any

			for _, idStr := range p.ScenarioIDs {
				scenarioID, err := uuid.Parse(strings.TrimSpace(idStr))
				if err != nil {
					failed = append(failed, map[string]any{
						"scenarioId": idStr,
						"error":      "不是合法的UUID",
					})
					continue
				}

				sc, err := deps.Scenarios.Get(ctx, scenarioID)
				if err != nil {
					failed = append(failed, map[string]any{
						"scenarioId": idStr,
						"error":      fmt.Sprintf("查询失败: %v", err),
					})
					continue
				}

				if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
					failed = append(failed, map[string]any{
						"scenarioId": idStr,
						"error":      fmt.Sprintf("权限校验失败: %v", err),
					})
					continue
				}

				if err := deps.Scenarios.Delete(ctx, scenarioID); err != nil {
					failed = append(failed, map[string]any{
						"scenarioId": idStr,
						"error":      fmt.Sprintf("删除失败: %v", err),
					})
					continue
				}

				deleted = append(deleted, idStr)
			}

			return map[string]any{
				"deleted":     deleted,
				"failed":      failed,
				"deleteCount": len(deleted),
				"failCount":   len(failed),
			}, nil
		})
}
