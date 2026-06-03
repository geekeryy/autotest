package mockserver

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrMockServerNotFound indicates that a mock server does not exist in the requested project.
	ErrMockServerNotFound = errors.New("mock server not found")
	// ErrMockRouteNotFound indicates that a mock route does not exist on the requested mock server.
	ErrMockRouteNotFound = errors.New("mock route not found")
	// ErrMockServerRunning indicates that a configuration cannot be changed while the server is running.
	ErrMockServerRunning = errors.New("mock server is running")
	// ErrMockServerNotRunning indicates that a stop request targeted an inactive server.
	ErrMockServerNotRunning = errors.New("mock server is not running")
)

const (
	// ResponseBodyTypeJSON marks a response body as JSON.
	ResponseBodyTypeJSON = "json"
	// ResponseBodyTypeText marks a response body as plain text.
	ResponseBodyTypeText = "text"
	// ResponseBodyTypeRaw marks a response body as raw bytes.
	ResponseBodyTypeRaw = "raw"

	// ResponseModeBody returns a normal response with an optional body.
	ResponseModeBody = "body"
	// ResponseModeRedirect issues an HTTP redirect (Location header).
	ResponseModeRedirect = "redirect"
	// ResponseModeSchema uses JSON Schema to auto-generate response data.
	ResponseModeSchema = "schema"
	// ResponseModeRecord proxies requests to a real service and records responses.
	ResponseModeRecord = "record"
	// ResponseModeReplay returns previously recorded responses.
	ResponseModeReplay = "replay"
)

// MockServer stores a project-owned mock HTTP server configuration.
type MockServer struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Port        int       `json:"port"`
	// AutoStart 表示 API 进程启动时是否自动拉起该 Mock Server。
	AutoStart bool      `json:"autoStart"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MockServerInput is the payload used to create or update a mock server.
type MockServerInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Port        int    `json:"port"`
	// AutoStart 控制是否在 API 进程启动时自动拉起此 Mock Server。
	AutoStart bool `json:"autoStart"`
}

// MockRoute stores a request matcher and response definition for a mock server.
type MockRoute struct {
	ID               uuid.UUID       `json:"id"`
	MockServerID     uuid.UUID       `json:"mockServerId"`
	Method           string          `json:"method"`
	Path             string          `json:"path"`
	Priority         int             `json:"priority"`
	Enabled          bool            `json:"enabled"`
	RequestMatch     json.RawMessage `json:"requestMatch,omitempty"`
	ResponseMode      string          `json:"responseMode"`
	ResponseStatus    int             `json:"responseStatus"`
	ResponseHeaders   json.RawMessage `json:"responseHeaders,omitempty"`
	RedirectLocation  string          `json:"redirectLocation,omitempty"`
	ResponseBody      string          `json:"responseBody"`
	ResponseBodyType  string          `json:"responseBodyType"`
	// ResponseSchema 用于 schema 模式，存储 JSON Schema 自动生成响应数据。
	ResponseSchema    json.RawMessage `json:"responseSchema,omitempty"`
	// AIGeneratedPool AI 预生成的数据池，手动触发生成后缓存。
	AIGeneratedPool  json.RawMessage `json:"aiGeneratedPool,omitempty"`
	// AIGeneratedAt AI 数据池的生成时间。
	AIGeneratedAt    *time.Time      `json:"aiGeneratedAt,omitempty"`
	// RecordTargetURL 录制模式下真实服务的基础 URL。
	RecordTargetURL  string          `json:"recordTargetUrl,omitempty"`
	DelayMillis       int             `json:"delayMillis"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// MockRouteInput is the payload used to create or update a mock route.
type MockRouteInput struct {
	Method           string          `json:"method"`
	Path             string          `json:"path"`
	Priority         int             `json:"priority"`
	Enabled          *bool           `json:"enabled"`
	RequestMatch     json.RawMessage `json:"requestMatch"`
	ResponseMode      string          `json:"responseMode"`
	ResponseStatus    int             `json:"responseStatus"`
	ResponseHeaders   json.RawMessage `json:"responseHeaders"`
	RedirectLocation  string          `json:"redirectLocation"`
	ResponseBody      string          `json:"responseBody"`
	ResponseBodyType  string          `json:"responseBodyType"`
	// ResponseSchema 用于 schema 模式，存储 JSON Schema 自动生成响应数据。
	ResponseSchema    json.RawMessage `json:"responseSchema"`
	// AIGeneratedPool AI 预生成的数据池。
	AIGeneratedPool  json.RawMessage `json:"aiGeneratedPool"`
	// RecordTargetURL 录制模式下真实服务的基础 URL。
	RecordTargetURL  string          `json:"recordTargetUrl"`
	DelayMillis       int             `json:"delayMillis"`
}

// RouteInputFromRoute builds an update payload from an existing route.
func RouteInputFromRoute(route MockRoute) MockRouteInput {
	enabled := route.Enabled
	return MockRouteInput{
		Method:           route.Method,
		Path:             route.Path,
		Priority:         route.Priority,
		Enabled:          &enabled,
		RequestMatch:     route.RequestMatch,
		ResponseMode:     route.ResponseMode,
		ResponseStatus:   route.ResponseStatus,
		ResponseHeaders:  route.ResponseHeaders,
		RedirectLocation: route.RedirectLocation,
		ResponseBody:     route.ResponseBody,
		ResponseBodyType: route.ResponseBodyType,
		ResponseSchema:   route.ResponseSchema,
		AIGeneratedPool:  route.AIGeneratedPool,
		RecordTargetURL:  route.RecordTargetURL,
		DelayMillis:      route.DelayMillis,
	}
}

// RequestMatch describes supported request matching conditions.
type RequestMatch struct {
	Query        map[string]string `json:"query,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodyContains string            `json:"bodyContains,omitempty"`
	BodyJSON     map[string]any    `json:"bodyJson,omitempty"`
}

// ServerStatus describes the in-memory runtime state for a mock server.
type ServerStatus struct {
	ServerID  uuid.UUID  `json:"serverId"`
	Running   bool       `json:"running"`
	Port      int        `json:"port"`
	URL       string     `json:"url,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// ServerWithStatus combines persisted configuration with current runtime state.
type ServerWithStatus struct {
	MockServer
	Status ServerStatus `json:"status"`
}

const maxAccessLogBodyBytes = 4096

// MockAccessLog stores one HTTP access record for a mock server runtime request.
type MockAccessLog struct {
	ID              uuid.UUID       `json:"id"`
	ProjectID       uuid.UUID       `json:"projectId"`
	MockServerID    uuid.UUID       `json:"mockServerId"`
	MockRouteID     *uuid.UUID      `json:"mockRouteId,omitempty"`
	RouteMethod     string          `json:"routeMethod,omitempty"`
	RoutePath       string          `json:"routePath,omitempty"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	Query           string          `json:"query,omitempty"`
	ClientIP        string          `json:"clientIp,omitempty"`
	Matched         bool            `json:"matched"`
	ResponseStatus  int             `json:"responseStatus"`
	DurationMs      int             `json:"durationMs"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
	RequestHeaders  json.RawMessage `json:"requestHeaders,omitempty"`
	RequestBody     string          `json:"requestBody,omitempty"`
	ResponseHeaders json.RawMessage `json:"responseHeaders,omitempty"`
	ResponseBody    string          `json:"responseBody,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// ListAccessLogsFilter filters paginated mock access logs for one server.
type ListAccessLogsFilter struct {
	StatusFrom *int
	StatusTo   *int
	Matched    *bool
	RouteID    *uuid.UUID
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// ListAccessLogsPage is a paginated list of mock access logs.
type ListAccessLogsPage struct {
	Items  []MockAccessLog `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}
