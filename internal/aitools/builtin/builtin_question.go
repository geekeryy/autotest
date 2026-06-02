package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/aitools"
)

// askQuestionTool is a virtual tool that lets the AI ask the user structured
// questions (single-select, multi-select, or text input). The frontend detects
// this tool and renders an interactive card instead of a plain confirm dialog.
// The user's answer is passed back via toolArguments and returned as-is.
func askQuestionTool() aitools.Tool {
	schema := rawSchema(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "向用户提出的问题，简洁明了"
			},
			"type": {
				"type": "string",
				"enum": ["single_select", "multi_select", "text_input"],
				"description": "交互类型：single_select=单选, multi_select=多选, text_input=文本输入"
			},
			"options": {
				"type": "array",
				"description": "选项列表（single_select/multi_select 必填）",
				"items": {
					"type": "object",
					"properties": {
						"label": { "type": "string", "description": "选项显示文本" },
						"value": { "type": "string", "description": "选项返回值" },
						"description": { "type": "string", "description": "选项补充说明（可选）" }
					},
					"required": ["label", "value"]
				}
			},
			"required": {
				"type": "boolean",
				"description": "是否必答，默认 true。设为 false 时前端显示「跳过」按钮"
			},
			"placeholder": {
				"type": "string",
				"description": "text_input 类型的输入框占位提示"
			}
		},
		"required": ["question", "type"]
	}`)

	return confirmTool(
		"ask_question",
		"向用户提出交互式问题（单选/多选/文本输入）。当需要用户做选择、确认方案、补充信息时使用。工具会暂停等待用户回答，用户的选择将作为工具返回值。",
		schema,
		func(ctx context.Context, args json.RawMessage) (any, error) {
			// Parse the original question definition
			var question struct {
				Question  string `json:"question"`
				Type      string `json:"type"`
				Options   []struct {
					Label       string `json:"label"`
					Value       string `json:"value"`
					Description string `json:"description,omitempty"`
				} `json:"options,omitempty"`
				Required    *bool  `json:"required,omitempty"`
				Placeholder string `json:"placeholder,omitempty"`
			}
			if err := json.Unmarshal(args, &question); err != nil {
				return nil, fmt.Errorf("解析 ask_question 参数失败: %w", err)
			}

			// The user's answer arrives via toolArguments override in applyDecision.
			// We check if the args contain an "answer" field (set by the frontend).
			var answer struct {
				Answer any `json:"answer"`
			}
			if err := json.Unmarshal(args, &answer); err == nil && answer.Answer != nil {
				return map[string]any{
					"question": question.Question,
					"answer":   answer.Answer,
				}, nil
			}

			// Fallback: return the raw args (should not normally reach here
			// because the frontend always sends toolArguments with the answer).
			return map[string]any{
				"question": question.Question,
				"answer":   args,
			}, nil
		},
	)
}
