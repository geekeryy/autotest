package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/scriptlibrary"

	"github.com/google/uuid"
)

func listScriptTemplatesTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_script_templates",
		"列出当前项目可用的脚本模板（含内置模板与项目自定义模板）。",
		rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.Scripts == nil {
				return nil, errors.New("list_script_templates: script 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_script_templates: %w", err)
			}
			templates, err := deps.Scripts.List(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("list_script_templates: 查询失败: %w", err)
			}
			return map[string]any{
				"templates": templates,
				"count":     len(templates),
			}, nil
		})
}

func createScriptTemplateTool(deps Deps) aitools.Tool {
	return writeTool("create_script_template",
		"创建项目自定义脚本模板（name / category / code / scopes）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "name":        {"type": "string"},
                "category":    {"type": "string"},
                "description": {"type": "string"},
                "code":        {"type": "string"},
                "scopes":      {"type": "array", "items": {"type": "string", "enum": ["assertion", "scenario"]}}
            },
            "required": ["name", "code"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scripts == nil {
				return nil, errors.New("create_script_template: script 服务未配置")
			}
			var p struct {
				Name        string   `json:"name"`
				Category    string   `json:"category"`
				Description string   `json:"description"`
				Code        string   `json:"code"`
				Scopes      []string `json:"scopes"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_script_template: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_script_template: %w", err)
			}
			tmpl, err := deps.Scripts.Create(ctx, scriptlibrary.CreateInput{
				ProjectID:   projectID,
				Name:        strings.TrimSpace(p.Name),
				Category:    strings.TrimSpace(p.Category),
				Description: strings.TrimSpace(p.Description),
				Code:        p.Code,
				Scopes:      p.Scopes,
			})
			if err != nil {
				return nil, fmt.Errorf("create_script_template: 创建失败: %w", err)
			}
			return tmpl, nil
		})
}

func updateScriptTemplateTool(deps Deps) aitools.Tool {
	return writeTool("update_script_template",
		"更新项目自定义脚本模板。内置模板只读，不可通过本工具修改。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "templateId":  {"type": "string"},
                "name":        {"type": "string"},
                "category":    {"type": "string"},
                "description": {"type": "string"},
                "code":        {"type": "string"},
                "scopes":      {"type": "array", "items": {"type": "string", "enum": ["assertion", "scenario"]}}
            },
            "required": ["templateId", "name", "code"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scripts == nil {
				return nil, errors.New("update_script_template: script 服务未配置")
			}
			var p struct {
				TemplateID  string   `json:"templateId"`
				Name        string   `json:"name"`
				Category    string   `json:"category"`
				Description string   `json:"description"`
				Code        string   `json:"code"`
				Scopes      []string `json:"scopes"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_script_template: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_script_template: %w", err)
			}
			templateID, err := uuid.Parse(strings.TrimSpace(p.TemplateID))
			if err != nil {
				return nil, fmt.Errorf("update_script_template: templateId 不是合法 UUID: %w", err)
			}
			tmpl, err := findScriptTemplate(ctx, deps, projectID, templateID)
			if err != nil {
				return nil, fmt.Errorf("update_script_template: %w", err)
			}
			if tmpl.IsBuiltin {
				return nil, errors.New("update_script_template: 内置脚本模板不可修改")
			}
			updated, err := deps.Scripts.Update(ctx, templateID, scriptlibrary.UpdateInput{
				Name:        strings.TrimSpace(p.Name),
				Category:    strings.TrimSpace(p.Category),
				Description: strings.TrimSpace(p.Description),
				Code:        p.Code,
				Scopes:      p.Scopes,
			})
			if err != nil {
				return nil, fmt.Errorf("update_script_template: 更新失败: %w", err)
			}
			return updated, nil
		})
}

func deleteScriptTemplateTool(deps Deps) aitools.Tool {
	return deleteTool("delete_script_template",
		"删除项目自定义脚本模板。内置模板不可删除。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "templateId": {"type": "string"}
            },
            "required": ["templateId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Scripts == nil {
				return nil, errors.New("delete_script_template: script 服务未配置")
			}
			var p struct {
				TemplateID string `json:"templateId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_script_template: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_script_template: %w", err)
			}
			templateID, err := uuid.Parse(strings.TrimSpace(p.TemplateID))
			if err != nil {
				return nil, fmt.Errorf("delete_script_template: templateId 不是合法 UUID: %w", err)
			}
			tmpl, err := findScriptTemplate(ctx, deps, projectID, templateID)
			if err != nil {
				return nil, fmt.Errorf("delete_script_template: %w", err)
			}
			if tmpl.IsBuiltin {
				return nil, errors.New("delete_script_template: 内置脚本模板不可删除")
			}
			if err := deps.Scripts.Delete(ctx, templateID); err != nil {
				return nil, fmt.Errorf("delete_script_template: 删除失败: %w", err)
			}
			return map[string]any{
				"templateId": templateID,
				"deleted":    true,
			}, nil
		})
}

func findScriptTemplate(ctx context.Context, deps Deps, projectID, templateID uuid.UUID) (*scriptlibrary.Template, error) {
	templates, err := deps.Scripts.List(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("查询脚本模板失败: %w", err)
	}
	for i := range templates {
		if templates[i].ID == templateID {
			return &templates[i], nil
		}
	}
	return nil, fmt.Errorf("templateId %s 不存在或不可访问", templateID)
}
