package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/aitools"
	"autotest/internal/report"

	"github.com/google/uuid"
)

func listRunsTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_runs",
		"分页列出当前项目的场景运行记录（可选按 scenarioId / serviceId / status / 时间范围过滤）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "scenarioId": {"type": "string", "description": "可选"},
                "serviceId":  {"type": "string", "description": "可选"},
                "status":     {"type": "string", "enum": ["queued", "running", "passed", "failed"], "description": "可选"},
                "from":       {"type": "string", "description": "可选；RFC3339 或 YYYY-MM-DD"},
                "to":         {"type": "string", "description": "可选；RFC3339 或 YYYY-MM-DD"},
                "limit":      {"type": "integer", "minimum": 1, "description": "可选，默认 20"},
                "offset":     {"type": "integer", "minimum": 0, "description": "可选，默认 0"}
            },
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Runs == nil {
				return nil, errors.New("list_runs: run 服务未配置")
			}
			var p struct {
				ScenarioID string `json:"scenarioId"`
				ServiceID  string `json:"serviceId"`
				Status     string `json:"status"`
				From       string `json:"from"`
				To         string `json:"to"`
				Limit      int    `json:"limit"`
				Offset     int    `json:"offset"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_runs: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_runs: %w", err)
			}
			filter := report.ListRunsFilter{
				ProjectID: projectID,
				Limit:     p.Limit,
				Offset:    p.Offset,
			}
			if s := strings.TrimSpace(p.ScenarioID); s != "" {
				id, err := uuid.Parse(s)
				if err != nil {
					return nil, fmt.Errorf("list_runs: scenarioId 不是合法 UUID: %w", err)
				}
				filter.ScenarioID = &id
			}
			if s := strings.TrimSpace(p.ServiceID); s != "" {
				id, err := uuid.Parse(s)
				if err != nil {
					return nil, fmt.Errorf("list_runs: serviceId 不是合法 UUID: %w", err)
				}
				filter.ServiceID = &id
			}
			if s := strings.TrimSpace(p.Status); s != "" {
				st := report.RunStatus(s)
				filter.Status = &st
			}
			if s := strings.TrimSpace(p.From); s != "" {
				t, err := parseFilterTime(s)
				if err != nil {
					return nil, fmt.Errorf("list_runs: from 时间格式无效: %w", err)
				}
				filter.From = &t
			}
			if s := strings.TrimSpace(p.To); s != "" {
				t, err := parseFilterTime(s)
				if err != nil {
					return nil, fmt.Errorf("list_runs: to 时间格式无效: %w", err)
				}
				filter.To = &t
			}
			items, total, limit, offset, err := deps.Runs.ListProjectRuns(ctx, filter)
			if err != nil {
				return nil, fmt.Errorf("list_runs: 查询失败: %w", err)
			}
			return map[string]any{
				"runs":   items,
				"total":  total,
				"limit":  limit,
				"offset": offset,
				"count":  len(items),
			}, nil
		})
}

func getRunResultTool(deps Deps) aitools.Tool {
	return readOnlyTool("get_run_result",
		"查询单次运行的详情与步骤结果（含 run 元信息与全部 step results）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "runId": {"type": "string"}
            },
            "required": ["runId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Runs == nil {
				return nil, errors.New("get_run_result: run 服务未配置")
			}
			var p struct {
				RunID string `json:"runId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_run_result: 解析参数失败: %w", err)
			}
			runID, err := uuid.Parse(strings.TrimSpace(p.RunID))
			if err != nil {
				return nil, fmt.Errorf("get_run_result: runId 不是合法 UUID: %w", err)
			}
			out, err := deps.Runs.GetRunResult(ctx, runID)
			if err != nil {
				return nil, fmt.Errorf("get_run_result: 查询失败: %w", err)
			}
			if out.Run != nil {
				if err := aitools.RequireProjectAccess(ctx, out.Run.ProjectID); err != nil {
					return nil, fmt.Errorf("get_run_result: %w", err)
				}
			}
			return out, nil
		})
}

func parseFilterTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("请使用 RFC3339 或 YYYY-MM-DD")
}
