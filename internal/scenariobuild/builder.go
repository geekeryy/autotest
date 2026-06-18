// Package scenariobuild provides shared scenario + step orchestration used by
// AI builtin tools and the MCP HTTP client.
package scenariobuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// StepInput is the AI/MCP-facing shape of a single scenario step.
type StepInput struct {
	StepOrder       int             `json:"stepOrder"`
	StepType        string          `json:"stepType"`
	Name            string          `json:"name"`
	Enabled         *bool           `json:"enabled,omitempty"`
	TestCaseID      string          `json:"testCaseId,omitempty"`
	Config          json.RawMessage `json:"config,omitempty"`
	RequestOverride json.RawMessage `json:"requestOverride,omitempty"`
}

// CreateInput describes a scenario plus ordered steps to create in one pass.
type CreateInput struct {
	ProjectID   uuid.UUID
	ServiceID   uuid.UUID
	Name        string
	Description string
	Steps       []StepInput
}

// ScenarioAPI is the narrow persistence surface needed to create a scenario with steps.
type ScenarioAPI interface {
	Create(ctx context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error)
	UpsertStep(ctx context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error)
}

// Result is returned by CreateScenarioWithSteps.
type Result struct {
	Scenario *scenario.Scenario
	Steps    []scenario.Step
}

// ValidateStepOrders ensures stepOrder values are >= 1 and unique within the payload.
func ValidateStepOrders(steps []StepInput) error {
	seen := map[int]bool{}
	for i, s := range steps {
		if s.StepOrder < 1 {
			return fmt.Errorf("第 %d 个步骤的 stepOrder 必须 >= 1", i+1)
		}
		if seen[s.StepOrder] {
			return fmt.Errorf("stepOrder=%d 在 steps 中重复", s.StepOrder)
		}
		seen[s.StepOrder] = true
	}
	return nil
}

// BuildOrderToSeq maps stepOrder to the stable step_seq assigned on insert.
func BuildOrderToSeq(steps []scenario.Step) map[int]int {
	out := make(map[int]int, len(steps))
	for _, s := range steps {
		out[s.StepOrder] = s.StepSeq
	}
	return out
}

// NeedsSeqRewrite reports whether a step type references children by stepOrder in config.
func NeedsSeqRewrite(stepType string) bool {
	t := strings.TrimSpace(stepType)
	return t == string(scenario.StepTypeFor) || t == string(scenario.StepTypeCondition)
}

// BuildUpsertInput maps StepInput onto scenario.UpsertStepInput.
func BuildUpsertInput(in StepInput, _ map[int]int) (scenario.UpsertStepInput, error) {
	stepType := scenario.StepType(strings.TrimSpace(in.StepType))
	if stepType == "" {
		stepType = scenario.StepTypeAPI
	}
	out := scenario.UpsertStepInput{
		StepOrder: in.StepOrder,
		StepType:  stepType,
		Name:      strings.TrimSpace(in.Name),
		Enabled:   in.Enabled,
		Config:    in.Config,
	}
	if len(in.RequestOverride) > 0 && !IsJSONNull(in.RequestOverride) {
		out.RequestOverride = in.RequestOverride
	}
	if stepType == scenario.StepTypeAPI {
		idStr := strings.TrimSpace(in.TestCaseID)
		if idStr == "" {
			return scenario.UpsertStepInput{}, fmt.Errorf("stepOrder=%d 的 API 步骤必须提供 testCaseId", in.StepOrder)
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			return scenario.UpsertStepInput{}, fmt.Errorf("stepOrder=%d 的 testCaseId 不是合法 UUID: %w", in.StepOrder, err)
		}
		out.TestCaseID = id
	}
	return out, nil
}

// RewriteControlFlowConfig translates stepOrder references in control-flow config
// into step_seq values using orderToSeq.
func RewriteControlFlowConfig(stepType string, cfg json.RawMessage, orderToSeq map[int]int) (json.RawMessage, bool, error) {
	if len(cfg) == 0 || IsJSONNull(cfg) {
		return cfg, false, nil
	}
	var blob map[string]any
	if err := json.Unmarshal(cfg, &blob); err != nil {
		return nil, false, fmt.Errorf("config 必须是 JSON 对象: %w", err)
	}

	rewritten := false

	tryRewriteSlice := func(orderKey, seqKey string) error {
		raw, ok := blob[orderKey]
		if !ok {
			return nil
		}
		arr, err := toIntSlice(raw)
		if err != nil {
			return fmt.Errorf("%s 必须是整数数组: %w", orderKey, err)
		}
		seqs, err := mapOrdersToSeqs(arr, orderToSeq)
		if err != nil {
			return fmt.Errorf("%s: %w", orderKey, err)
		}
		blob[seqKey] = seqs
		delete(blob, orderKey)
		rewritten = true
		return nil
	}

	switch strings.TrimSpace(stepType) {
	case string(scenario.StepTypeFor):
		if err := tryRewriteSlice("bodyStepOrders", "bodyStepSeqs"); err != nil {
			return nil, false, err
		}
	case string(scenario.StepTypeCondition):
		if err := tryRewriteSlice("thenStepOrders", "thenStepSeqs"); err != nil {
			return nil, false, err
		}
		if err := tryRewriteSlice("elseStepOrders", "elseStepSeqs"); err != nil {
			return nil, false, err
		}
		if branchesRaw, ok := blob["branches"]; ok {
			branchesSlice, ok := branchesRaw.([]any)
			if !ok {
				return nil, false, errors.New("branches 必须是数组")
			}
			for i, b := range branchesSlice {
				bm, ok := b.(map[string]any)
				if !ok {
					return nil, false, fmt.Errorf("branches[%d] 必须是对象", i)
				}
				raw, ok := bm["stepOrders"]
				if !ok {
					continue
				}
				arr, err := toIntSlice(raw)
				if err != nil {
					return nil, false, fmt.Errorf("branches[%d].stepOrders 必须是整数数组: %w", i, err)
				}
				seqs, err := mapOrdersToSeqs(arr, orderToSeq)
				if err != nil {
					return nil, false, fmt.Errorf("branches[%d].stepOrders: %w", i, err)
				}
				bm["stepSeqs"] = seqs
				delete(bm, "stepOrders")
				branchesSlice[i] = bm
				rewritten = true
			}
			blob["branches"] = branchesSlice
		}
	}

	if !rewritten {
		return cfg, false, nil
	}
	out, err := json.Marshal(blob)
	if err != nil {
		return nil, false, fmt.Errorf("重新序列化 config 失败: %w", err)
	}
	return out, true, nil
}

// CreateScenarioWithSteps creates a scenario and upserts all steps, then rewrites
// control-flow references from stepOrder to step_seq. Partial work is preserved on failure.
func CreateScenarioWithSteps(ctx context.Context, api ScenarioAPI, in CreateInput) (*Result, error) {
	if api == nil {
		return nil, errors.New("scenario API 未配置")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("name 不能为空")
	}
	if len(in.Steps) == 0 {
		return nil, errors.New("steps 不能为空，至少包含一个步骤")
	}
	if err := ValidateStepOrders(in.Steps); err != nil {
		return nil, err
	}

	sc, err := api.Create(ctx, scenario.CreateScenarioInput{
		ProjectID:   in.ProjectID,
		ServiceID:   in.ServiceID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	})
	if err != nil {
		return nil, fmt.Errorf("创建场景失败: %w", err)
	}

	created := make([]scenario.Step, 0, len(in.Steps))
	orderToSeq := make(map[int]int, len(in.Steps))
	for i, sIn := range in.Steps {
		upsert, err := BuildUpsertInput(sIn, nil)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个步骤参数错误: %w", i+1, err)
		}
		step, err := api.UpsertStep(ctx, sc.ID, upsert)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个步骤写入失败（已创建 scenarioId=%s 与 %d 个步骤，可在 UI 中继续编辑）: %w", i+1, sc.ID, len(created), err)
		}
		created = append(created, *step)
		orderToSeq[step.StepOrder] = step.StepSeq
	}

	for i, sIn := range in.Steps {
		if !NeedsSeqRewrite(sIn.StepType) {
			continue
		}
		newConfig, rewritten, err := RewriteControlFlowConfig(sIn.StepType, sIn.Config, orderToSeq)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个步骤 config 引用解析失败: %w", i+1, err)
		}
		if !rewritten {
			continue
		}
		upsert, err := BuildUpsertInput(StepInput{
			StepOrder:       sIn.StepOrder,
			StepType:        sIn.StepType,
			Name:            sIn.Name,
			Enabled:         sIn.Enabled,
			TestCaseID:      sIn.TestCaseID,
			Config:          newConfig,
			RequestOverride: sIn.RequestOverride,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("控制流步骤 stepOrder=%d 重写失败: %w", sIn.StepOrder, err)
		}
		updated, err := api.UpsertStep(ctx, sc.ID, upsert)
		if err != nil {
			return nil, fmt.Errorf("控制流步骤 stepOrder=%d 写入引用失败: %w", sIn.StepOrder, err)
		}
		for j := range created {
			if created[j].StepOrder == updated.StepOrder {
				created[j] = *updated
				break
			}
		}
	}

	return &Result{Scenario: sc, Steps: created}, nil
}

func toIntSlice(v any) ([]int, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, errors.New("不是数组")
	}
	out := make([]int, 0, len(arr))
	for i, item := range arr {
		switch n := item.(type) {
		case float64:
			if n != float64(int(n)) {
				return nil, fmt.Errorf("[%d] 必须是整数", i)
			}
			out = append(out, int(n))
		case int:
			out = append(out, n)
		default:
			return nil, fmt.Errorf("[%d] 必须是整数", i)
		}
	}
	return out, nil
}

func mapOrdersToSeqs(orders []int, orderToSeq map[int]int) ([]int, error) {
	out := make([]int, 0, len(orders))
	for _, o := range orders {
		seq, ok := orderToSeq[o]
		if !ok {
			return nil, fmt.Errorf("引用了不存在的 stepOrder=%d", o)
		}
		out = append(out, seq)
	}
	return out, nil
}

// IsJSONNull reports whether raw JSON is the literal null.
func IsJSONNull(b json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(b))
	return trimmed == "null"
}
