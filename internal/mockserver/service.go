package mockserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"autotest/internal/logx"
	"autotest/internal/urlx"

	"github.com/google/uuid"
)

// Service applies validation and runtime coordination for mock servers.
type Service struct {
	repo    *Repository
	runtime *Runtime
}

// NewService creates a Service.
func NewService(repo *Repository, runtime *Runtime) *Service {
	return &Service{repo: repo, runtime: runtime}
}

// CreateServer validates and creates a mock server.
func (s *Service) CreateServer(ctx context.Context, projectID uuid.UUID, input MockServerInput) (*ServerWithStatus, error) {
	if err := normalizeServerInput(&input); err != nil {
		return nil, err
	}
	server, err := s.repo.CreateServer(ctx, projectID, input)
	if err != nil {
		return nil, err
	}
	return s.withStatus(*server), nil
}

// ListServers returns mock servers with in-memory runtime status.
func (s *Service) ListServers(ctx context.Context, projectID uuid.UUID) ([]ServerWithStatus, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	servers, err := s.repo.ListServers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	items := make([]ServerWithStatus, 0, len(servers))
	for _, server := range servers {
		items = append(items, *s.withStatus(server))
	}
	return items, nil
}

// UpdateServer updates a stopped mock server.
func (s *Service) UpdateServer(ctx context.Context, projectID, serverID uuid.UUID, input MockServerInput) (*ServerWithStatus, error) {
	if serverID == uuid.Nil {
		return nil, errors.New("mock server id is required")
	}
	if s.runtime != nil && s.runtime.IsRunning(serverID) {
		return nil, ErrMockServerRunning
	}
	if err := normalizeServerInput(&input); err != nil {
		return nil, err
	}
	server, err := s.repo.UpdateServer(ctx, projectID, serverID, input)
	if err != nil {
		return nil, err
	}
	return s.withStatus(*server), nil
}

// DeleteServer stops and soft-deletes a mock server.
func (s *Service) DeleteServer(ctx context.Context, projectID, serverID uuid.UUID) error {
	if serverID == uuid.Nil {
		return errors.New("mock server id is required")
	}
	if s.runtime != nil && s.runtime.IsRunning(serverID) {
		if err := s.runtime.Stop(ctx, serverID); err != nil && !errors.Is(err, ErrMockServerNotRunning) {
			return err
		}
	}
	return s.repo.DeleteServer(ctx, projectID, serverID)
}

// AutoStartAll 在 API 进程启动时按持久化的 auto_start=true 配置自动拉起 Mock Server。
// 单个 Server 启动失败（端口占用、配置异常等）只记录日志，不影响其他 Mock Server，
// 也不会让调用方失败。返回的 error 仅用于读取持久化列表本身失败的场景。
func (s *Service) AutoStartAll(ctx context.Context) error {
	if s == nil || s.runtime == nil || s.repo == nil {
		return nil
	}
	servers, err := s.repo.ListAutoStartServers(ctx)
	if err != nil {
		return fmt.Errorf("加载默认启动 Mock Server 列表: %w", err)
	}
	for _, server := range servers {
		if err := s.runtime.Start(server); err != nil {
			if errors.Is(err, ErrMockServerRunning) {
				continue
			}
			logx.Warn("默认启动 Mock Server 失败",
				"id", server.ID, "name", server.Name, "port", server.Port, "err", err)
			continue
		}
		logx.Info("默认启动 Mock Server 成功",
			"id", server.ID, "name", server.Name, "port", server.Port)
	}
	return nil
}

// StartServer starts a configured mock server on its port.
func (s *Service) StartServer(ctx context.Context, projectID, serverID uuid.UUID) (ServerStatus, error) {
	if serverID == uuid.Nil {
		return ServerStatus{}, errors.New("mock server id is required")
	}
	server, err := s.repo.GetServer(ctx, projectID, serverID)
	if err != nil {
		return ServerStatus{}, err
	}
	if s.runtime == nil {
		return ServerStatus{}, errors.New("mock runtime is not configured")
	}
	if err := s.runtime.Start(*server); err != nil {
		return ServerStatus{}, err
	}
	return s.runtime.Status(*server), nil
}

// StopServer stops a running mock server.
func (s *Service) StopServer(ctx context.Context, projectID, serverID uuid.UUID) (ServerStatus, error) {
	if serverID == uuid.Nil {
		return ServerStatus{}, errors.New("mock server id is required")
	}
	server, err := s.repo.GetServer(ctx, projectID, serverID)
	if err != nil {
		return ServerStatus{}, err
	}
	if s.runtime == nil {
		return ServerStatus{}, errors.New("mock runtime is not configured")
	}
	if err := s.runtime.Stop(ctx, serverID); err != nil {
		return ServerStatus{}, err
	}
	return s.runtime.Status(*server), nil
}

// GetStatus returns the in-memory status for a mock server.
func (s *Service) GetStatus(ctx context.Context, projectID, serverID uuid.UUID) (ServerStatus, error) {
	if serverID == uuid.Nil {
		return ServerStatus{}, errors.New("mock server id is required")
	}
	server, err := s.repo.GetServer(ctx, projectID, serverID)
	if err != nil {
		return ServerStatus{}, err
	}
	if s.runtime == nil {
		return ServerStatus{ServerID: server.ID, Port: server.Port}, nil
	}
	return s.runtime.Status(*server), nil
}

// CreateRoute validates and creates a mock route.
func (s *Service) CreateRoute(ctx context.Context, projectID, serverID uuid.UUID, input MockRouteInput) (*MockRoute, error) {
	if err := normalizeRouteInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateRoute(ctx, projectID, serverID, input)
}

// ListRoutes returns routes for a mock server.
func (s *Service) ListRoutes(ctx context.Context, projectID, serverID uuid.UUID) ([]MockRoute, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serverID == uuid.Nil {
		return nil, errors.New("mock server id is required")
	}
	return s.repo.ListRoutes(ctx, projectID, serverID)
}

// GetRoute returns a single mock route by ID.
func (s *Service) GetRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID) (*MockRoute, error) {
	if routeID == uuid.Nil {
		return nil, errors.New("mock route id is required")
	}
	routes, err := s.repo.ListRoutes(ctx, projectID, serverID)
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if route.ID == routeID {
			return &route, nil
		}
	}
	return nil, ErrMockRouteNotFound
}

// UpdateRoute validates and updates a mock route.
func (s *Service) UpdateRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID, input MockRouteInput) (*MockRoute, error) {
	if routeID == uuid.Nil {
		return nil, errors.New("mock route id is required")
	}
	if err := normalizeRouteInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateRoute(ctx, projectID, serverID, routeID, input)
}

// DeleteRoute soft-deletes a mock route.
func (s *Service) DeleteRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID) error {
	if routeID == uuid.Nil {
		return errors.New("mock route id is required")
	}
	return s.repo.DeleteRoute(ctx, projectID, serverID, routeID)
}

// ListAccessLogs returns paginated access logs for a mock server.
func (s *Service) ListAccessLogs(ctx context.Context, projectID, serverID uuid.UUID, filter ListAccessLogsFilter) (*ListAccessLogsPage, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serverID == uuid.Nil {
		return nil, errors.New("mock server id is required")
	}
	if _, err := s.repo.GetServer(ctx, projectID, serverID); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	total, err := s.repo.CountAccessLogs(ctx, projectID, serverID, filter)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListAccessLogs(ctx, projectID, serverID, filter)
	if err != nil {
		return nil, err
	}
	return &ListAccessLogsPage{
		Items:  items,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

// ClearAccessLogs deletes all access logs for a mock server.
func (s *Service) ClearAccessLogs(ctx context.Context, projectID, serverID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if serverID == uuid.Nil {
		return errors.New("mock server id is required")
	}
	return s.repo.ClearAccessLogs(ctx, projectID, serverID)
}

func parseAccessLogsFilter(values func(string) string) (ListAccessLogsFilter, error) {
	filter := ListAccessLogsFilter{Limit: 20, Offset: 0}
	if raw := strings.TrimSpace(values("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return filter, errors.New("limit must be a positive integer")
		}
		filter.Limit = limit
	}
	if raw := strings.TrimSpace(values("offset")); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return filter, errors.New("offset must be a non-negative integer")
		}
		filter.Offset = offset
	}
	if raw := strings.TrimSpace(values("status")); raw != "" {
		status, err := strconv.Atoi(raw)
		if err != nil || status < 100 || status > 599 {
			return filter, errors.New("status must be between 100 and 599")
		}
		filter.StatusFrom = &status
		filter.StatusTo = &status
	}
	if raw := strings.TrimSpace(values("statusFrom")); raw != "" {
		status, err := strconv.Atoi(raw)
		if err != nil || status < 100 || status > 599 {
			return filter, errors.New("statusFrom must be between 100 and 599")
		}
		filter.StatusFrom = &status
	}
	if raw := strings.TrimSpace(values("statusTo")); raw != "" {
		status, err := strconv.Atoi(raw)
		if err != nil || status < 100 || status > 599 {
			return filter, errors.New("statusTo must be between 100 and 599")
		}
		filter.StatusTo = &status
	}
	if raw := strings.TrimSpace(values("matched")); raw != "" {
		matched, err := strconv.ParseBool(raw)
		if err != nil {
			return filter, errors.New("matched must be true or false")
		}
		filter.Matched = &matched
	}
	if raw := strings.TrimSpace(values("routeId")); raw != "" {
		routeID, err := uuid.Parse(raw)
		if err != nil {
			return filter, errors.New("routeId must be a valid uuid")
		}
		filter.RouteID = &routeID
	}
	if raw := strings.TrimSpace(values("from")); raw != "" {
		from, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, errors.New("from must be RFC3339 timestamp")
		}
		filter.From = &from
	}
	if raw := strings.TrimSpace(values("to")); raw != "" {
		to, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, errors.New("to must be RFC3339 timestamp")
		}
		filter.To = &to
	}
	return filter, nil
}

func (s *Service) withStatus(server MockServer) *ServerWithStatus {
	status := ServerStatus{ServerID: server.ID, Port: server.Port}
	if s.runtime != nil {
		status = s.runtime.Status(server)
	}
	return &ServerWithStatus{MockServer: server, Status: status}
}

func normalizeServerInput(input *MockServerInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return errors.New("mock server name is required")
	}
	if input.Port <= 0 || input.Port > 65535 {
		return errors.New("mock server port must be between 1 and 65535")
	}
	return nil
}

func normalizeRouteInput(input *MockRouteInput) error {
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	if input.Method == "" {
		return errors.New("mock route method is required")
	}
	input.Path = strings.TrimSpace(input.Path)
	if input.Path == "" || !strings.HasPrefix(input.Path, "/") {
		return errors.New("mock route path must start with /")
	}
	input.ResponseMode = strings.TrimSpace(strings.ToLower(input.ResponseMode))
	if input.ResponseMode == "" {
		input.ResponseMode = ResponseModeBody
	}
	if input.ResponseMode != ResponseModeBody &&
		input.ResponseMode != ResponseModeRedirect &&
		input.ResponseMode != ResponseModeSchema &&
		input.ResponseMode != ResponseModeRecord &&
		input.ResponseMode != ResponseModeReplay {
		return errors.New("mock route responseMode must be body, redirect, schema, record, or replay")
	}
	input.RecordTargetURL = strings.TrimRight(strings.TrimSpace(input.RecordTargetURL), "/")
	if input.ResponseMode == ResponseModeRecord {
		if err := urlx.ValidateHTTPServiceURL(input.RecordTargetURL); err != nil {
			return fmt.Errorf("recordTargetUrl: %w", err)
		}
	}
	input.RedirectLocation = strings.TrimSpace(input.RedirectLocation)
	if input.ResponseMode == ResponseModeRedirect && input.RedirectLocation == "" {
		return errors.New("mock route redirectLocation is required for redirect response")
	}
	// schema 模式下，如果提供了 responseSchema 则校验格式
	if input.ResponseMode == ResponseModeSchema && len(input.ResponseSchema) > 0 {
		var schema map[string]any
		if err := json.Unmarshal(input.ResponseSchema, &schema); err != nil {
			return errors.New("mock route responseSchema must be a valid JSON object")
		}
	}
	if input.ResponseStatus == 0 {
		if input.ResponseMode == ResponseModeRedirect {
			input.ResponseStatus = http.StatusFound
		} else {
			input.ResponseStatus = 200
		}
	}
	if input.ResponseStatus < 100 || input.ResponseStatus > 599 {
		return errors.New("mock route response status must be between 100 and 599")
	}
	if input.ResponseMode == ResponseModeRedirect && (input.ResponseStatus < 300 || input.ResponseStatus > 399) {
		return errors.New("mock route redirect response status must be between 300 and 399")
	}
	input.ResponseBodyType = strings.TrimSpace(input.ResponseBodyType)
	if input.ResponseBodyType == "" {
		input.ResponseBodyType = ResponseBodyTypeJSON
	}
	if input.ResponseBodyType != ResponseBodyTypeJSON &&
		input.ResponseBodyType != ResponseBodyTypeText &&
		input.ResponseBodyType != ResponseBodyTypeRaw {
		return errors.New("mock route response body type is invalid")
	}
	if input.DelayMillis < 0 {
		return errors.New("mock route delayMillis must be non-negative")
	}
	if err := validateRequestMatch(input.RequestMatch); err != nil {
		return err
	}
	if err := validateHeaders(input.ResponseHeaders); err != nil {
		return err
	}
	return nil
}

func validateRequestMatch(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var match RequestMatch
	if err := json.Unmarshal(raw, &match); err != nil {
		return errors.New("mock route requestMatch must be a valid JSON object")
	}
	return nil
}

func validateHeaders(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var headers map[string]any
	if err := json.Unmarshal(raw, &headers); err != nil {
		return errors.New("mock route responseHeaders must be a valid JSON object")
	}
	return nil
}
