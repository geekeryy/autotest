package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/aitools"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

type specDigest struct {
	ID          uuid.UUID `json:"id"`
	ServiceID   uuid.UUID `json:"serviceId"`
	Version     int       `json:"version"`
	ContentHash string    `json:"contentHash"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toSpecDigests(specs []spec.APISpec) []specDigest {
	out := make([]specDigest, 0, len(specs))
	for _, s := range specs {
		out = append(out, specDigest{
			ID:          s.ID,
			ServiceID:   s.ServiceID,
			Version:     s.Version,
			ContentHash: s.ContentHash,
			Status:      s.Status,
			CreatedAt:   s.CreatedAt,
		})
	}
	return out
}

func listSpecsTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_specs",
		"列出某服务下已导入的 OpenAPI/Swagger 规范版本摘要（id / version / contentHash / status），不含原始文档内容。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.Specs == nil {
				return nil, errors.New("list_specs: spec 仓库未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_specs: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_specs: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("list_specs: serviceId 不是合法 UUID: %w", err)
			}
			specs, err := deps.Specs.ListSpecs(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("list_specs: 查询失败: %w", err)
			}
			digests := toSpecDigests(specs)
			return map[string]any{
				"specs": digests,
				"count": len(digests),
			}, nil
		})
}

func importOpenAPITool(deps Deps) aitools.Tool {
	return writeTool("import_openapi",
		"向指定服务导入 OpenAPI/Swagger 文档（YAML 或 JSON 字符串）。返回导入统计与端点变更摘要。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"},
                "content":   {"type": "string", "description": "OpenAPI/Swagger 文档全文（YAML 或 JSON）"}
            },
            "required": ["serviceId", "content"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.SpecImporter == nil {
				return nil, errors.New("import_openapi: spec 导入服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
				Content   string `json:"content"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("import_openapi: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("import_openapi: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("import_openapi: serviceId 不是合法 UUID: %w", err)
			}
			content := strings.TrimSpace(p.Content)
			if content == "" {
				return nil, errors.New("import_openapi: content 不能为空")
			}
			summary, err := deps.SpecImporter.Import(ctx, projectID, serviceID, []byte(content))
			if err != nil {
				return nil, fmt.Errorf("import_openapi: 导入失败: %w", err)
			}
			return summary, nil
		})
}
