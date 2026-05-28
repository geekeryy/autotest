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

func updateCaseTool(deps Deps) aitools.Tool {
	return writeTool("update_case",
		"部分更新接口模板/测试用例（name 和/或 assertions）。未传字段保留原值；assertions 传则整体替换。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "caseId":     {"type": "string"},
                "name":       {"type": "string", "description": "可选"},
                "assertions": {"type": "array", "description": "可选；完整断言数组", "items": {"type": "object"}}
            },
            "required": ["caseId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Cases == nil {
				return nil, errors.New("update_case: case 服务未配置")
			}
			var p struct {
				CaseID     string          `json:"caseId"`
				Name       *string         `json:"name"`
				Assertions json.RawMessage `json:"assertions"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_case: 解析参数失败: %w", err)
			}
			caseID, err := uuid.Parse(strings.TrimSpace(p.CaseID))
			if err != nil {
				return nil, fmt.Errorf("update_case: caseId 不是合法 UUID: %w", err)
			}
			existing, err := deps.Cases.Get(ctx, caseID)
			if err != nil {
				return nil, fmt.Errorf("update_case: 查询测试用例失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, existing.ProjectID); err != nil {
				return nil, fmt.Errorf("update_case: %w", err)
			}
			input := testcase.PatchInput{}
			hasPatch := false
			if p.Name != nil {
				name := strings.TrimSpace(*p.Name)
				if name == "" {
					return nil, errors.New("update_case: name 不能为空")
				}
				input.Name = &name
				hasPatch = true
			}
			if len(p.Assertions) > 0 && !isJSONNull(p.Assertions) {
				input.Assertions = p.Assertions
				hasPatch = true
			}
			if !hasPatch {
				return nil, errors.New("update_case: 至少提供 name 或 assertions 之一")
			}
			tc, err := deps.Cases.Patch(ctx, caseID, input)
			if err != nil {
				return nil, fmt.Errorf("update_case: 更新失败: %w", err)
			}
			return tc, nil
		})
}
