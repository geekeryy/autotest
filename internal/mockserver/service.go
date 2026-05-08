package mockserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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
	if input.ResponseStatus == 0 {
		input.ResponseStatus = 200
	}
	if input.ResponseStatus < 100 || input.ResponseStatus > 599 {
		return errors.New("mock route response status must be between 100 and 599")
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
