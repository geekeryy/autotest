package mockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// EndpointInfo 用于 Spec 同步的端点信息。
// 通过接口解耦，避免直接依赖 spec 包。
type EndpointInfo struct {
	ID             uuid.UUID
	Method         string
	Path           string
	OperationID    string
	Summary        string
	ResponseSchema json.RawMessage
}

// SyncFromEndpoints 将 endpoints 同步为 Schema 模式的 MockRoutes。
// 逻辑：
// 1. 查找或创建项目下的默认 MockServer
// 2. 遍历 endpoints，为每个创建 Schema 模式的 MockRoute（已存在则跳过）
// 返回新创建的路由数量。
func (s *Service) SyncFromEndpoints(ctx context.Context, projectID uuid.UUID, endpoints []EndpointInfo) (int, error) {
	if len(endpoints) == 0 {
		return 0, nil
	}

	// 查找或创建默认 MockServer
	server, err := s.getOrCreateDefaultServer(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("get default mock server: %w", err)
	}

	// 获取已有路由，用于去重
	existingRoutes, err := s.repo.ListRoutes(ctx, projectID, server.ID)
	if err != nil {
		return 0, fmt.Errorf("list existing routes: %w", err)
	}
	existingSet := make(map[string]bool, len(existingRoutes))
	for _, r := range existingRoutes {
		key := strings.ToUpper(r.Method) + " " + r.Path
		existingSet[key] = true
	}

	created := 0
	for _, ep := range endpoints {
		key := strings.ToUpper(ep.Method) + " " + ep.Path
		if existingSet[key] {
			continue // 已存在，跳过
		}

		// 提取 response body schema
		bodySchema := extractResponseBodySchema(ep.ResponseSchema)
		if len(bodySchema) == 0 {
			continue // 无 schema，跳过
		}

		enabled := true
		input := MockRouteInput{
			Method:         ep.Method,
			Path:           ep.Path,
			Priority:       0,
			Enabled:        &enabled,
			ResponseMode:   ResponseModeSchema,
			ResponseStatus: 200,
			ResponseBodyType: ResponseBodyTypeJSON,
			ResponseSchema: bodySchema,
		}

		// 使用 operationId 或 summary 作为路由名（如果有 description 字段的话）
		// 目前 MockRoute 没有 name 字段，暂不设置

		if _, err := s.repo.CreateRoute(ctx, projectID, server.ID, input); err != nil {
			// 单个路由创建失败不影响其他
			continue
		}
		created++
	}

	return created, nil
}

// getOrCreateDefaultServer 查找或创建项目下的默认 MockServer。
// 默认服务器名称为 "default"，端口从 9000 开始递增查找可用端口。
func (s *Service) getOrCreateDefaultServer(ctx context.Context, projectID uuid.UUID) (*MockServer, error) {
	servers, err := s.repo.ListServers(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// 查找名为 "default" 的服务器
	for _, server := range servers {
		if server.Name == "default" {
			return &server, nil
		}
	}

	// 没有默认服务器，创建一个
	// 查找一个可用端口
	port := 9000
	usedPorts := make(map[int]bool, len(servers))
	for _, server := range servers {
		usedPorts[server.Port] = true
	}
	for usedPorts[port] {
		port++
		if port > 65535 {
			return nil, fmt.Errorf("no available port for default mock server")
		}
	}

	input := MockServerInput{
		Name:        "default",
		Description: "Auto-created default mock server for Spec sync",
		Port:        port,
		AutoStart:   false,
	}

	server, err := s.repo.CreateServer(ctx, projectID, input)
	if err != nil {
		return nil, fmt.Errorf("create default mock server: %w", err)
	}
	return server, nil
}

// extractResponseBodySchema 从 Endpoint 的 ResponseSchema 中提取 body 部分。
func extractResponseBodySchema(responseSchema json.RawMessage) json.RawMessage {
	if len(responseSchema) == 0 {
		return nil
	}
	var wrapper struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(responseSchema, &wrapper); err != nil {
		return nil
	}
	return wrapper.Body
}
