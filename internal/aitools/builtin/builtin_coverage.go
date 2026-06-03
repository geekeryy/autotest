package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/generator"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// coverageCaseResult 描述单条生成结果摘要。
type coverageCaseResult struct {
	Name             string `json:"name"`
	Method           string `json:"method"`
	Path             string `json:"path"`
	GenerationRuleID string `json:"generationRuleId"`
	Fingerprint      string `json:"fingerprint"`
}

// generateCoverageCasesTool 允许 AI 选择覆盖维度，为指定端点批量生成测试用例草稿并持久化。
func generateCoverageCasesTool(deps Deps) aitools.Tool {
	return writeTool("generate_coverage_cases",
		"按覆盖维度为指定端点批量生成测试用例并保存。"+
			"支持的维度（rules 数组）：happy_path（正向用例）、required_missing（缺失必填字段）、type_mismatch（类型错误）、edge_cases（边界值）。"+
			"不传 rules 则默认全部维度。endpointId 可选，不传则对该服务全部端点生成。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":  {"type": "string", "description": "目标服务 UUID"},
                "endpointId": {"type": "string", "description": "可选；指定单个端点 UUID，不传则覆盖服务下全部端点"},
                "rules": {
                    "type": "array",
                    "description": "可选；要启用的覆盖维度 ID 列表。可选值: happy_path, required_missing, type_mismatch, edge_cases",
                    "items": {"type": "string", "enum": ["happy_path", "required_missing", "type_mismatch", "edge_cases"]}
                }
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Generator == nil {
				return nil, errors.New("generate_coverage_cases: 生成器未配置")
			}
			if deps.Cases == nil || deps.Specs == nil {
				return nil, errors.New("generate_coverage_cases: 平台服务未配置")
			}
			var p struct {
				ServiceID  string   `json:"serviceId"`
				EndpointID string   `json:"endpointId"`
				Rules      []string `json:"rules"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("generate_coverage_cases: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_cases: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("generate_coverage_cases: serviceId 不是合法 UUID: %w", err)
			}

			// 收集目标端点列表
			var endpoints []generator.Endpoint
			if epIDStr := strings.TrimSpace(p.EndpointID); epIDStr != "" {
				epID, err := uuid.Parse(epIDStr)
				if err != nil {
					return nil, fmt.Errorf("generate_coverage_cases: endpointId 不是合法 UUID: %w", err)
				}
				ep, err := deps.Specs.GetEndpointByID(ctx, epID)
				if err != nil {
					return nil, fmt.Errorf("generate_coverage_cases: 查询端点失败: %w", err)
				}
				endpoints = append(endpoints, specEndpointToGenerator(projectID, ep))
			} else {
				specEndpoints, err := deps.Specs.ListEndpoints(ctx, projectID, serviceID)
				if err != nil {
					return nil, fmt.Errorf("generate_coverage_cases: 查询端点列表失败: %w", err)
				}
				for i := range specEndpoints {
					endpoints = append(endpoints, specEndpointToGenerator(projectID, &specEndpoints[i]))
				}
			}

			if len(endpoints) == 0 {
				return nil, errors.New("generate_coverage_cases: 未找到目标端点")
			}

			// 按 rules 过滤生成器
			ruleFilter := make(map[string]bool)
			for _, r := range p.Rules {
				ruleFilter[strings.TrimSpace(r)] = true
			}
			useAllRules := len(ruleFilter) == 0

			allRules := []generator.Rule{
				generator.HappyPathRule{},
				generator.RequiredMissingRule{},
				generator.TypeMismatchRule{},
				generator.EdgeCasesRule{},
			}
			var filteredRules []generator.Rule
			for _, r := range allRules {
				if useAllRules || ruleFilter[r.ID()] {
					filteredRules = append(filteredRules, r)
				}
			}
			if len(filteredRules) == 0 {
				return nil, errors.New("generate_coverage_cases: 无有效的覆盖维度")
			}

			gen := generator.NewWithRules(filteredRules)

			// 生成并持久化
			var results []coverageCaseResult
			var created, skipped int
			for _, ep := range endpoints {
				drafts, err := gen.Generate(ep)
				if err != nil {
					return nil, fmt.Errorf("generate_coverage_cases: 端点 %s %s 生成失败: %w", ep.Method, ep.Path, err)
				}
				for _, d := range drafts {
					tc, err := deps.Cases.UpsertGenerated(ctx, d)
					if err != nil {
						return nil, fmt.Errorf("generate_coverage_cases: 保存用例失败: %w", err)
					}
					if tc != nil {
						created++
						results = append(results, coverageCaseResult{
							Name:             d.Name,
							Method:           d.Method,
							Path:             d.Path,
							GenerationRuleID: d.GenerationRuleID,
							Fingerprint:      d.Fingerprint,
						})
					} else {
						skipped++
					}
				}
			}
			return map[string]any{
				"created":   created,
				"skipped":   skipped,
				"endpoints": len(endpoints),
				"rules":     len(filteredRules),
				"cases":     results,
			}, nil
		})
}

// specEndpointToGenerator 将 spec.Endpoint 转换为 generator.Endpoint。
func specEndpointToGenerator(projectID uuid.UUID, ep *spec.Endpoint) generator.Endpoint {
	return generator.Endpoint{
		ID:             ep.ID,
		ProjectID:      projectID,
		ServiceID:      ep.ServiceID,
		Method:         ep.Method,
		Path:           ep.Path,
		OperationID:    ep.OperationID,
		Summary:        ep.Summary,
		RequestSchema:  ep.RequestSchema,
		ResponseSchema: ep.ResponseSchema,
	}
}
