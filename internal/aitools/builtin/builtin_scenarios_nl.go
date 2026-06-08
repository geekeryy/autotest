// builtin_scenarios_nl.go provides the append_step_from_description tool,
// which proposes a single scenario step from a natural-language description
// while injecting context from variables already extracted by prior steps.
package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/ainl"
	"autotest/internal/aitools"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// appendStepResult is the output of append_step_from_description.
type appendStepResult struct {
	ProposedStep     *ainl.ParsedStep  `json:"proposedStep,omitempty"`
	PendingQuestions []pendingQuestion `json:"pendingQuestions,omitempty"`
	Ready            bool              `json:"ready"`
	ContextSummary   string            `json:"contextSummary"`
}

// pendingQuestion is a single interactive question to relay via ask_question.
type pendingQuestion struct {
	Field        string   `json:"field"`
	Description  string   `json:"description"`
	QuestionType string   `json:"questionType"` // "single_select" | "text_input" | "multi_select"
	Options      []string `json:"options,omitempty"`
}

// appendStepFromDescriptionTool creates the append_step_from_description tool.
// It reads existing scenario steps to discover available template-variable
// references, asks the LLM to parse a one-step natural-language description
// in that context, and returns the proposed step with any pending questions.
func appendStepFromDescriptionTool(nlDeps NLToolDeps, scenarios ScenarioService) aitools.Tool {
	return readOnlyTool("append_step_from_description",
		"向已有场景逐步追加一个自然语言描述的步骤。自动感知前序步骤提取的变量（如 authToken），"+
			"返回建议参数与待确认项（pendingQuestions），ready=true 时可直接调用 add_scenario_step。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {
                    "type": "string",
                    "description": "目标场景的 UUID"
                },
                "description": {
                    "type": "string",
                    "description": "用自然语言描述要追加的单个步骤（中文）"
                },
                "serviceId": {
                    "type": "string",
                    "description": "场景所属服务的 UUID"
                }
            },
            "required": ["scenarioId", "description", "serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if nlDeps.AI == nil {
				return nil, errors.New("append_step_from_description: AI 服务未配置")
			}
			if nlDeps.Specs == nil {
				return nil, errors.New("append_step_from_description: spec 仓库未配置")
			}
			if scenarios == nil {
				return nil, errors.New("append_step_from_description: scenario 服务未配置")
			}

			var p struct {
				ScenarioID  string `json:"scenarioId"`
				Description string `json:"description"`
				ServiceID   string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("append_step_from_description: 解析参数失败: %w", err)
			}

			scenarioID, err := uuid.Parse(strings.TrimSpace(p.ScenarioID))
			if err != nil {
				return nil, fmt.Errorf("append_step_from_description: scenarioId 不是合法 UUID: %w", err)
			}

			description := strings.TrimSpace(p.Description)
			if description == "" {
				return nil, errors.New("append_step_from_description: description 不能为空")
			}

			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("append_step_from_description: %w", err)
			}

			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("append_step_from_description: serviceId 不是合法 UUID: %w", err)
			}

			existingSteps, err := scenarios.ListSteps(ctx, scenarioID)
			if err != nil {
				return nil, fmt.Errorf("append_step_from_description: 获取场景步骤失败: %w", err)
			}

			contextSummary := buildStepContextSummary(existingSteps)
			enrichedDesc := enrichDescriptionWithContext(description, existingSteps)

			specAdapter := &specRepoAdapter{repo: nlDeps.Specs}
			orch := ainl.NewOrchestrator(nlDeps.AI, specAdapter)
			output, err := orch.Orchestrate(ctx, ainl.OrchestratorInput{
				Description: enrichedDesc,
				ProjectID:   projectID,
				ServiceID:   serviceID,
			})
			if err != nil {
				return nil, fmt.Errorf("append_step_from_description: 编排失败: %w", err)
			}

			if len(output.Steps) == 0 {
				return nil, errors.New("append_step_from_description: LLM 未能从描述中提取出步骤，请提供更具体的描述")
			}

			proposed := output.Steps[0]
			proposed.StepOrder = len(existingSteps) + 1

			questions := buildPendingQuestions(&proposed)

			return appendStepResult{
				ProposedStep:     &proposed,
				PendingQuestions: questions,
				Ready:            len(questions) == 0,
				ContextSummary:   contextSummary,
			}, nil
		},
	)
}

// stepConfigExtracts is the subset of step config we parse for variable names.
type stepConfigExtracts struct {
	Extracts []struct {
		Name string `json:"name"`
		From string `json:"from"`
	} `json:"extracts,omitempty"`
}

// collectExtractedVars returns all variable names produced by prior api steps.
func collectExtractedVars(steps []scenario.Step) []string {
	var vars []string
	for _, step := range steps {
		if step.StepType != scenario.StepTypeAPI {
			continue
		}
		if len(step.Config) == 0 {
			continue
		}
		var cfg stepConfigExtracts
		if err := json.Unmarshal(step.Config, &cfg); err != nil {
			continue
		}
		for _, e := range cfg.Extracts {
			if name := strings.TrimSpace(e.Name); name != "" {
				vars = append(vars, name)
			}
		}
	}
	return vars
}

// buildStepContextSummary produces a human-readable summary of existing steps.
func buildStepContextSummary(steps []scenario.Step) string {
	if len(steps) == 0 {
		return "场景暂无步骤"
	}
	vars := collectExtractedVars(steps)
	summary := fmt.Sprintf("已有 %d 个步骤", len(steps))
	if len(vars) > 0 {
		summary += fmt.Sprintf("，可引用变量：%s", strings.Join(vars, "、"))
	}
	return summary
}

// enrichDescriptionWithContext appends variable-reference hints to the
// single-step description so the LLM can wire up auth and prior-step outputs.
func enrichDescriptionWithContext(description string, steps []scenario.Step) string {
	vars := collectExtractedVars(steps)
	if len(vars) == 0 {
		return description
	}
	return description + "\n\n[可用前序变量（直接以 {{varName}} 引用）：" + strings.Join(vars, "、") + "]"
}

// buildPendingQuestions generates interactive questions for a step that has
// unresolved or low-confidence endpoint matches.
func buildPendingQuestions(step *ainl.ParsedStep) []pendingQuestion {
	if step.StepType != "api" && step.StepType != "" {
		return nil
	}
	var questions []pendingQuestion
	switch step.MatchConfidence {
	case "none", "":
		if step.EndpointMethod != "" || step.EndpointPath != "" {
			questions = append(questions, pendingQuestion{
				Field: "endpoint",
				Description: fmt.Sprintf(
					"接口 %s %s 未能自动匹配，请输入正确的接口路径（格式：METHOD /path，如 POST /api/v1/login）",
					step.EndpointMethod, step.EndpointPath),
				QuestionType: "text_input",
			})
		}
	case "low":
		questions = append(questions, pendingQuestion{
			Field: "endpoint",
			Description: fmt.Sprintf(
				"自动匹配到的接口 %s %s 置信度较低，请确认是否正确",
				step.EndpointMethod, step.EndpointPath),
			QuestionType: "single_select",
			Options: []string{
				fmt.Sprintf("是，使用 %s %s", step.EndpointMethod, step.EndpointPath),
				"否，我来指定正确的接口",
			},
		})
	}
	return questions
}
