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
	ResponseStatus   int             `json:"responseStatus"`
	ResponseHeaders  json.RawMessage `json:"responseHeaders,omitempty"`
	ResponseBody     string          `json:"responseBody"`
	ResponseBodyType string          `json:"responseBodyType"`
	DelayMillis      int             `json:"delayMillis"`
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
	ResponseStatus   int             `json:"responseStatus"`
	ResponseHeaders  json.RawMessage `json:"responseHeaders"`
	ResponseBody     string          `json:"responseBody"`
	ResponseBodyType string          `json:"responseBodyType"`
	DelayMillis      int             `json:"delayMillis"`
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
