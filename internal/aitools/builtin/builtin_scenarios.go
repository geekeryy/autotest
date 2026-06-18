// builtin_scenarios.go hosts the scenario orchestration tools. These are
// the heart of the "AI generates a scenario the user just clicks Run on"
// flow: a coarse-grained creator (create_scenario_with_steps) plus
// fine-grained editors for refinement (add / update / delete / reorder).
//
// The non-trivial part is control flow. For/Condition steps reference
// their child steps by `step_seq`, which is assigned by the database at
// insert time. Since the model doesn't know real step_seq values when it
// emits the tool call, we let the model reference children by
// `stepOrder` in `config.bodyStepOrders` / `branches[].stepOrders` and
// translate stepOrder → stepSeq in a second pass inside the tool
// implementation. The same model is used by update_scenario_step.
package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/scenariobuild"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// scenarioStepInput 与 scenariobuild.StepInput 同构，供 AI 工具 JSON 反序列化。
type scenarioStepInput = scenariobuild.StepInput

func listScenariosTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "list_scenarios",
		Description: "列出项目（可选 serviceId）下的全部场景摘要（id / name / serviceId / createdAt）。" +
			"在新建场景前调用，避免命名重复；也可用于参考其它场景的步骤结构。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string", "description": "可选；填了就按服务过滤"}
            },
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("list_scenarios: scenario 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_scenarios: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_scenarios: %w", err)
			}
			filter := scenario.ListFilter{ProjectID: projectID}
			if s := strings.TrimSpace(p.ServiceID); s != "" {
				sid, err := uuid.Parse(s)
				if err != nil {
					return nil, fmt.Errorf("list_scenarios: serviceId 不是合法 UUID: %w", err)
				}
				filter.ServiceID = sid
			}
			items, err := deps.Scenarios.List(ctx, filter)
			if err != nil {
				return nil, fmt.Errorf("list_scenarios: 查询失败: %w", err)
			}
			return map[string]any{
				"scenarios": items,
				"count":     len(items),
			}, nil
		},
	}
}

func getScenarioTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "get_scenario",
		Description: "查询场景详情，含场景元信息和所有步骤（步骤序、类型、所引用的 case ID、是否启用、" +
			"步骤配置 JSON）。当用户上下文里出现 scenarioId 字段，需要展开场景全貌时使用。",
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {
                    "type": "string",
                    "description": "场景的 UUID"
                }
            },
            "required": ["scenarioId"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("get_scenario: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string `json:"scenarioId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_scenario: 解析参数失败: %w", err)
			}
			id, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("get_scenario: scenarioId 不是合法 UUID: %w", err)
			}
			sc, err := deps.Scenarios.Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get_scenario: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("get_scenario: %w", err)
			}
			steps, err := deps.Scenarios.ListSteps(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get_scenario: 查询步骤失败: %w", err)
			}
			return map[string]any{
				"scenario": sc,
				"steps":    steps,
			}, nil
		},
	}
}

// createScenarioWithStepsTool is the coarse-grained generator. The model
// produces an entire scenario + ordered steps in one call. The user only
// confirms once.
//
// Order of operations:
//  1. Create the scenario row.
//  2. UpsertStep every input step in stepOrder order. After this pass we
//     have a stepOrder → stepSeq map populated from the database.
//  3. For any control-flow step (for/condition) whose config references
//     children by stepOrder, rewrite those references to real step_seq
//     values and UpsertStep that step again.
//
// On failure the scenario is left in whatever state we reached, with the
// error reported back to the AI; we do not rollback because partial
// scenarios are still useful for the user to fix in the UI and rollback
// would silently throw away the AI's work.
func createScenarioWithStepsTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "create_scenario_with_steps",
		Description: "一次性创建测试场景 + 全部步骤。AI 生成完整场景的主路径，调用前必须先用 list_endpoints / list_cases 找好真实 case；" +
			"API 步骤需要 testCaseId（没有可先调 create_case_from_endpoint）；script/for/condition 步骤通过 config 表达逻辑。" +
			"For / Condition 步骤的子步骤引用使用 stepOrder（本次调用 steps 数组里其他步骤的 stepOrder），平台会自动转换为内部 step_seq。" +
			"成功后返回 scenarioId，用户即可在场景页面点击「运行」执行。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":   {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "steps": {
                    "type": "array",
                    "description": "按执行顺序排列的步骤列表",
                    "items": {
                        "type": "object",
                        "properties": {
                            "stepOrder": {"type": "integer", "minimum": 1, "description": "1-based 顺序，所有步骤的 stepOrder 必须互不相同；控制流子步骤通过这个字段被引用"},
                            "stepType":  {"type": "string", "enum": ["api", "database", "script", "for", "condition"]},
                            "name":      {"type": "string"},
                            "enabled":   {"type": "boolean", "description": "可选，默认 true"},
                            "testCaseId":{"type": "string", "description": "API 步骤必填；其它步骤忽略"},
                            "config":    {"type": ["object", "null"], "description": "步骤配置 JSON；详见说明"},
                            "requestOverride": {"type": ["object", "null"], "description": "API 步骤可选，覆盖请求模板字段"}
                        },
                        "required": ["stepOrder", "stepType", "name"],
                        "additionalProperties": false
                    }
                }
            },
            "required": ["serviceId", "name", "steps"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("create_scenario_with_steps: scenario 服务未配置")
			}
			var p struct {
				ServiceID   string              `json:"serviceId"`
				Name        string              `json:"name"`
				Description string              `json:"description"`
				Steps       []scenarioStepInput `json:"steps"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_scenario_with_steps: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_scenario_with_steps: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("create_scenario_with_steps: serviceId 不是合法 UUID: %w", err)
			}
			if strings.TrimSpace(p.Name) == "" {
				return nil, errors.New("create_scenario_with_steps: name 不能为空")
			}
			result, err := scenariobuild.CreateScenarioWithSteps(ctx, deps.Scenarios, scenariobuild.CreateInput{
				ProjectID:   projectID,
				ServiceID:   serviceID,
				Name:        p.Name,
				Description: p.Description,
				Steps:       p.Steps,
			})
			if err != nil {
				return nil, fmt.Errorf("create_scenario_with_steps: %w", err)
			}

			return map[string]any{
				"scenario": result.Scenario,
				"steps":    result.Steps,
			}, nil
		},
	}
}

// addScenarioStepTool appends or overwrites a single step.
//
// stepOrder uniqueness is enforced at the DB layer via (scenarioId,
// stepOrder); if the model picks an existing stepOrder the previous step
// at that slot is replaced. For "insert between existing steps" the AI
// should reorder first.
func addScenarioStepTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "add_scenario_step",
		Description: "向已有场景追加一个步骤（默认场景末尾，stepOrder 应当等于现有步骤数+1）。" +
			"如果传入的 stepOrder 已被占用，会覆盖原步骤；若要插入到中间请先用 reorder_scenario_steps 腾位置。" +
			"For / Condition 子步骤引用使用 stepOrder（必须已存在于场景中）。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string"},
                "stepOrder":  {"type": "integer", "minimum": 1},
                "stepType":   {"type": "string", "enum": ["api", "database", "script", "for", "condition"]},
                "name":       {"type": "string"},
                "enabled":    {"type": "boolean"},
                "testCaseId": {"type": "string", "description": "API 步骤必填"},
                "config":     {"type": ["object", "null"]},
                "requestOverride": {"type": ["object", "null"]}
            },
            "required": ["scenarioId", "stepOrder", "stepType", "name"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("add_scenario_step: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string `json:"scenarioId"`
				scenarioStepInput
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("add_scenario_step: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("add_scenario_step: scenarioId 不是合法 UUID: %w", err)
			}
			sc, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("add_scenario_step: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("add_scenario_step: %w", err)
			}

			cfg := p.Config
			if scenariobuild.NeedsSeqRewrite(p.StepType) {
				existing, err := deps.Scenarios.ListSteps(ctx, scenarioID)
				if err != nil {
					return nil, fmt.Errorf("add_scenario_step: 读取场景步骤失败: %w", err)
				}
				orderToSeq := scenariobuild.BuildOrderToSeq(existing)
				rewritten, _, err := scenariobuild.RewriteControlFlowConfig(p.StepType, cfg, orderToSeq)
				if err != nil {
					return nil, fmt.Errorf("add_scenario_step: 控制流 config 引用解析失败: %w", err)
				}
				cfg = rewritten
			}

			upsert, err := scenariobuild.BuildUpsertInput(scenarioStepInput{
				StepOrder:       p.StepOrder,
				StepType:        p.StepType,
				Name:            p.Name,
				Enabled:         p.Enabled,
				TestCaseID:      p.TestCaseID,
				Config:          cfg,
				RequestOverride: p.RequestOverride,
			}, nil)
			if err != nil {
				return nil, fmt.Errorf("add_scenario_step: 参数错误: %w", err)
			}
			step, err := deps.Scenarios.UpsertStep(ctx, scenarioID, upsert)
			if err != nil {
				return nil, fmt.Errorf("add_scenario_step: 写入失败: %w", err)
			}
			return step, nil
		},
	}
}

// updateScenarioStepTool patches an existing step. Because UpsertStep is
// destructive (it overwrites every column), we implement the partial
// update on the client side: list the steps, find the slot the model
// targeted, fill missing fields with current values, then UpsertStep.
func updateScenarioStepTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "update_scenario_step",
		Description: "修改场景中已有步骤的部分字段（按 stepOrder 定位）。未传的字段保留原值，传 null 含义同未传。" +
			"如果改变 stepType 需要同时给出该类型必填的字段（如 API 的 testCaseId）。" +
			"For / Condition 子步骤引用同样使用 stepOrder（必须已存在于场景中）。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string"},
                "stepOrder":  {"type": "integer", "minimum": 1},
                "stepType":   {"type": "string", "enum": ["api", "database", "script", "for", "condition"]},
                "name":       {"type": "string"},
                "enabled":    {"type": "boolean"},
                "testCaseId": {"type": "string"},
                "config":     {"type": ["object", "null"]},
                "requestOverride": {"type": ["object", "null"]}
            },
            "required": ["scenarioId", "stepOrder"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("update_scenario_step: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string  `json:"scenarioId"`
				StepOrder  int     `json:"stepOrder"`
				StepType   *string `json:"stepType,omitempty"`
				Name       *string `json:"name,omitempty"`
				Enabled    *bool   `json:"enabled,omitempty"`
				TestCaseID *string `json:"testCaseId,omitempty"`
				// Distinguish "field absent" from "field present and JSON
				// null" by using a json.RawMessage: missing field decodes
				// to nil, explicit null decodes to []byte("null").
				Config          json.RawMessage `json:"config,omitempty"`
				RequestOverride json.RawMessage `json:"requestOverride,omitempty"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_scenario_step: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("update_scenario_step: scenarioId 不是合法 UUID: %w", err)
			}
			if p.StepOrder <= 0 {
				return nil, errors.New("update_scenario_step: stepOrder 必须 >= 1")
			}
			sc, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("update_scenario_step: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("update_scenario_step: %w", err)
			}

			existing, err := deps.Scenarios.ListSteps(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("update_scenario_step: 读取场景步骤失败: %w", err)
			}
			var current *scenario.Step
			for i := range existing {
				if existing[i].StepOrder == p.StepOrder {
					current = &existing[i]
					break
				}
			}
			if current == nil {
				return nil, fmt.Errorf("update_scenario_step: 场景中不存在 stepOrder=%d 的步骤", p.StepOrder)
			}

			merged := scenarioStepInput{
				StepOrder: p.StepOrder,
				StepType:  string(current.StepType),
				Name:      current.Name,
			}
			merged.Enabled = boolPtr(current.Enabled)
			if current.TestCaseID != uuid.Nil {
				merged.TestCaseID = current.TestCaseID.String()
			}
			merged.Config = current.Config
			merged.RequestOverride = current.RequestOverride

			if p.StepType != nil {
				merged.StepType = strings.TrimSpace(*p.StepType)
			}
			if p.Name != nil {
				merged.Name = *p.Name
			}
			if p.Enabled != nil {
				merged.Enabled = boolPtr(*p.Enabled)
			}
			if p.TestCaseID != nil {
				merged.TestCaseID = strings.TrimSpace(*p.TestCaseID)
			}
			if p.Config != nil && !scenariobuild.IsJSONNull(p.Config) {
				merged.Config = p.Config
			}
			if p.RequestOverride != nil && !scenariobuild.IsJSONNull(p.RequestOverride) {
				merged.RequestOverride = p.RequestOverride
			}

			cfg := merged.Config
			if scenariobuild.NeedsSeqRewrite(merged.StepType) {
				orderToSeq := scenariobuild.BuildOrderToSeq(existing)
				rewritten, _, err := scenariobuild.RewriteControlFlowConfig(merged.StepType, cfg, orderToSeq)
				if err != nil {
					return nil, fmt.Errorf("update_scenario_step: 控制流 config 引用解析失败: %w", err)
				}
				cfg = rewritten
			}
			merged.Config = cfg

			upsert, err := scenariobuild.BuildUpsertInput(merged, nil)
			if err != nil {
				return nil, fmt.Errorf("update_scenario_step: 参数错误: %w", err)
			}
			step, err := deps.Scenarios.UpsertStep(ctx, scenarioID, upsert)
			if err != nil {
				return nil, fmt.Errorf("update_scenario_step: 写入失败: %w", err)
			}
			return step, nil
		},
	}
}

func deleteScenarioStepTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "delete_scenario_step",
		Description: "按 stepId 删除场景步骤（软删除，stepOrder 槽位会被释放）。" +
			"scenarioId 必填以便平台校验权限并确认 step 属于该场景；删除控制流步骤前请先 update_scenario_step 把引用它的父步骤的 stepOrders 清理掉。",
		Mutating:        true,
		RequiresConfirm: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string", "description": "stepId 所属场景的 UUID"},
                "stepId":     {"type": "string", "description": "scenario step 的 UUID（注意不是 stepOrder）"}
            },
            "required": ["scenarioId", "stepId"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("delete_scenario_step: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string `json:"scenarioId"`
				StepID     string `json:"stepId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_scenario_step: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("delete_scenario_step: scenarioId 不是合法 UUID: %w", err)
			}
			stepID, err := uuid.Parse(strings.TrimSpace(p.StepID))
			if err != nil {
				return nil, fmt.Errorf("delete_scenario_step: stepId 不是合法 UUID: %w", err)
			}
			sc, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("delete_scenario_step: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("delete_scenario_step: %w", err)
			}
			steps, err := deps.Scenarios.ListSteps(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("delete_scenario_step: 读取场景步骤失败: %w", err)
			}
			found := false
			for _, s := range steps {
				if s.ID == stepID {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("delete_scenario_step: stepId %s 不属于场景 %s", stepID, scenarioID)
			}
			if err := deps.Scenarios.DeleteStep(ctx, stepID); err != nil {
				return nil, fmt.Errorf("delete_scenario_step: 删除失败: %w", err)
			}
			return map[string]any{
				"scenarioId": scenarioID,
				"stepId":     stepID,
				"deleted":    true,
			}, nil
		},
	}
}

func reorderScenarioStepsTool(deps Deps) aitools.Tool {
	return aitools.Tool{
		Name: "reorder_scenario_steps",
		Description: "按给定 stepIds 顺序重新排列场景步骤；stepIds 必须等于场景当前所有未删除步骤的 ID 集合（顺序就是新的执行顺序）。" +
			"reorder 不会改变 step_seq，因此控制流引用不受影响。",
		Mutating: true,
		Parameters: rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string"},
                "stepIds":    {"type": "array", "items": {"type": "string"}, "minItems": 1}
            },
            "required": ["scenarioId", "stepIds"],
            "additionalProperties": false
        }`),
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scenarios == nil {
				return nil, errors.New("reorder_scenario_steps: scenario 服务未配置")
			}
			var p struct {
				ScenarioID string   `json:"scenarioId"`
				StepIDs    []string `json:"stepIds"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("reorder_scenario_steps: 解析参数失败: %w", err)
			}
			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("reorder_scenario_steps: scenarioId 不是合法 UUID: %w", err)
			}
			sc, err := deps.Scenarios.Get(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("reorder_scenario_steps: 查询场景失败: %w", err)
			}
			if err := aitools.RequireProjectAccess(ctx, sc.ProjectID); err != nil {
				return nil, fmt.Errorf("reorder_scenario_steps: %w", err)
			}
			ids := make([]uuid.UUID, 0, len(p.StepIDs))
			for i, s := range p.StepIDs {
				id, err := uuid.Parse(strings.TrimSpace(s))
				if err != nil {
					return nil, fmt.Errorf("reorder_scenario_steps: stepIds[%d] 不是合法 UUID: %w", i, err)
				}
				ids = append(ids, id)
			}
			if err := deps.Scenarios.ReorderSteps(ctx, scenarioID, scenario.ReorderStepsInput{StepIDs: ids}); err != nil {
				return nil, fmt.Errorf("reorder_scenario_steps: 重排失败: %w", err)
			}
			return map[string]any{
				"scenarioId": scenarioID,
				"stepIds":    ids,
			}, nil
		},
	}
}

func boolPtr(b bool) *bool { return &b }
