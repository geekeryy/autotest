package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/mockset"

	"github.com/google/uuid"
)

func listMockValueSetsTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_mock_value_sets",
		"列出当前项目下全部命名值集合（Mock Value Sets）。",
		rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.MockSets == nil {
				return nil, errors.New("list_mock_value_sets: mock set 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_mock_value_sets: %w", err)
			}
			sets, err := deps.MockSets.List(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("list_mock_value_sets: 查询失败: %w", err)
			}
			return map[string]any{
				"valueSets": sets,
				"count":     len(sets),
			}, nil
		})
}

func createMockValueSetTool(deps Deps) aitools.Tool {
	return writeTool("create_mock_value_set",
		"创建命名值集合（key / name / values / 可选 weights）。key 创建后不可修改。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "key":         {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "values":      {"type": "array", "items": {}, "minItems": 1},
                "weights":     {"type": "array", "items": {"type": "number"}}
            },
            "required": ["key", "name", "values"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockSets == nil {
				return nil, errors.New("create_mock_value_set: mock set 服务未配置")
			}
			var p struct {
				Key         string            `json:"key"`
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Values      []json.RawMessage `json:"values"`
				Weights     []float64         `json:"weights"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_mock_value_set: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_mock_value_set: %w", err)
			}
			set, err := deps.MockSets.Create(ctx, projectID, mockset.CreateInput{
				Key:         strings.TrimSpace(p.Key),
				Name:        strings.TrimSpace(p.Name),
				Description: strings.TrimSpace(p.Description),
				Values:      p.Values,
				Weights:     p.Weights,
			})
			if err != nil {
				return nil, fmt.Errorf("create_mock_value_set: 创建失败: %w", err)
			}
			return set, nil
		})
}

func updateMockValueSetTool(deps Deps) aitools.Tool {
	return writeTool("update_mock_value_set",
		"更新命名值集合（name / description / values / weights）。key 不可修改。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "setId":       {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "values":      {"type": "array", "items": {}, "minItems": 1},
                "weights":     {"type": "array", "items": {"type": "number"}}
            },
            "required": ["setId", "name", "values"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockSets == nil {
				return nil, errors.New("update_mock_value_set: mock set 服务未配置")
			}
			var p struct {
				SetID       string            `json:"setId"`
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Values      []json.RawMessage `json:"values"`
				Weights     []float64         `json:"weights"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_mock_value_set: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_mock_value_set: %w", err)
			}
			setID, err := uuid.Parse(strings.TrimSpace(p.SetID))
			if err != nil {
				return nil, fmt.Errorf("update_mock_value_set: setId 不是合法 UUID: %w", err)
			}
			set, err := deps.MockSets.Update(ctx, projectID, setID, mockset.UpdateInput{
				Name:        strings.TrimSpace(p.Name),
				Description: strings.TrimSpace(p.Description),
				Values:      p.Values,
				Weights:     p.Weights,
			})
			if err != nil {
				return nil, fmt.Errorf("update_mock_value_set: 更新失败: %w", err)
			}
			return set, nil
		})
}

func deleteMockValueSetTool(deps Deps) aitools.Tool {
	return deleteTool("delete_mock_value_set",
		"删除命名值集合（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "setId": {"type": "string"}
            },
            "required": ["setId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.MockSets == nil {
				return nil, errors.New("delete_mock_value_set: mock set 服务未配置")
			}
			var p struct {
				SetID string `json:"setId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_mock_value_set: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_mock_value_set: %w", err)
			}
			setID, err := uuid.Parse(strings.TrimSpace(p.SetID))
			if err != nil {
				return nil, fmt.Errorf("delete_mock_value_set: setId 不是合法 UUID: %w", err)
			}
			if err := deps.MockSets.Delete(ctx, projectID, setID); err != nil {
				return nil, fmt.Errorf("delete_mock_value_set: 删除失败: %w", err)
			}
			return map[string]any{
				"setId":   setID,
				"deleted": true,
			}, nil
		})
}
