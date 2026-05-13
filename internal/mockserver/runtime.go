package mockserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"autotest/internal/templating"

	"github.com/google/uuid"
)

const stopTimeout = 5 * time.Second

// MockSetService is the runtime-facing slice of `mockset.Service` we use to
// resolve `{{$mock.set.<key>}}` family tokens inside mock response bodies.
// Defined locally to avoid an import cycle and to keep the runtime testable.
type MockSetService interface {
	Lookup(ctx context.Context, projectID uuid.UUID, key string) (values []json.RawMessage, weights []float64, ok bool)
}

// Runtime manages in-process HTTP servers for mock server configurations.
type Runtime struct {
	repo     *Repository
	mockSets MockSetService
	mu       sync.Mutex
	running  map[uuid.UUID]*runningServer
}

type runningServer struct {
	server    MockServer
	http      *http.Server
	startedAt time.Time
}

// NewRuntime creates a Runtime backed by the repository used for live rule reads.
//
// mockSets 可选；非 nil 时启用 `{{$mock.set.<key>}}` 在 Mock 响应体里的解析。
// 每个 HTTP 请求获得独立的游标 map，意味着同一个请求内多次出现
// `{{$mock.set.<key>[*]}}` 会按顺序循环；不同请求互不影响。
func NewRuntime(repo *Repository, mockSets MockSetService) *Runtime {
	return &Runtime{
		repo:     repo,
		mockSets: mockSets,
		running:  map[uuid.UUID]*runningServer{},
	}
}

// Start begins listening for a mock server on its configured port.
func (rt *Runtime) Start(server MockServer) error {
	rt.mu.Lock()
	if _, ok := rt.running[server.ID]; ok {
		rt.mu.Unlock()
		return ErrMockServerRunning
	}
	rt.mu.Unlock()

	addr := fmt.Sprintf(":%d", server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen mock server: %w", err)
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           rt.mockHandler(server.ID),
		ReadHeaderTimeout: 10 * time.Second,
	}
	state := &runningServer{server: server, http: httpServer, startedAt: time.Now()}

	rt.mu.Lock()
	if _, ok := rt.running[server.ID]; ok {
		rt.mu.Unlock()
		if err := listener.Close(); err != nil {
			return fmt.Errorf("close duplicate mock listener: %w", err)
		}
		return ErrMockServerRunning
	}
	rt.running[server.ID] = state
	rt.mu.Unlock()

	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("mock server %s stopped unexpectedly: %v", server.ID, err)
		}
		rt.mu.Lock()
		if rt.running[server.ID] == state {
			delete(rt.running, server.ID)
		}
		rt.mu.Unlock()
	}()
	return nil
}

// Stop gracefully shuts down a running mock server.
func (rt *Runtime) Stop(ctx context.Context, serverID uuid.UUID) error {
	rt.mu.Lock()
	state, ok := rt.running[serverID]
	rt.mu.Unlock()
	if !ok {
		return ErrMockServerNotRunning
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if err := state.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("stop mock server: %w", err)
	}

	rt.mu.Lock()
	if rt.running[serverID] == state {
		delete(rt.running, serverID)
	}
	rt.mu.Unlock()
	return nil
}

// IsRunning reports whether a mock server is active in memory.
func (rt *Runtime) IsRunning(serverID uuid.UUID) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	_, ok := rt.running[serverID]
	return ok
}

// Status returns current in-memory status for a mock server configuration.
func (rt *Runtime) Status(server MockServer) ServerStatus {
	status := ServerStatus{
		ServerID: server.ID,
		Port:     server.Port,
	}
	rt.mu.Lock()
	state, ok := rt.running[server.ID]
	rt.mu.Unlock()
	if !ok {
		return status
	}
	startedAt := state.startedAt
	status.Running = true
	status.Port = state.server.Port
	status.URL = fmt.Sprintf("http://localhost:%d", state.server.Port)
	status.StartedAt = &startedAt
	return status
}

func (rt *Runtime) mockHandler(serverID uuid.UUID) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyMockCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeRuntimeError(w, http.StatusBadRequest, "读取请求体失败")
			return
		}
		server, err := rt.repo.GetServerByID(r.Context(), serverID)
		if err != nil {
			writeRuntimeError(w, http.StatusInternalServerError, "读取 Mock Server 失败")
			return
		}
		routes, err := rt.repo.ListEnabledRoutesForServer(r.Context(), serverID)
		if err != nil {
			writeRuntimeError(w, http.StatusInternalServerError, "读取 Mock 规则失败")
			return
		}
		route, err := MatchRules(routes, r, body)
		if err != nil {
			writeRuntimeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if route == nil {
			writeRuntimeError(w, http.StatusNotFound, "mock route not found")
			return
		}
		mockCfg := rt.buildRequestMockConfig(server.ProjectID)
		writeMockResponse(w, r, *route, body, mockCfg)
	})
}

// buildRequestMockConfig returns a per-request MockExpanderConfig wiring the
// project's value-set service into the templating Mock hook. Each HTTP
// request gets its own Cursors map so `[*]` sequential tokens cycle through
// values within a single response render but never leak across requests.
func (rt *Runtime) buildRequestMockConfig(projectID uuid.UUID) *templating.MockExpanderConfig {
	if rt.mockSets == nil {
		return nil
	}
	resolver := templating.MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return rt.mockSets.Lookup(context.Background(), projectID, key)
	})
	return &templating.MockExpanderConfig{
		Sets:    resolver,
		Cursors: map[string]*int{},
	}
}

func writeMockResponse(w http.ResponseWriter, r *http.Request, route MockRoute, body []byte, mockCfg *templating.MockExpanderConfig) {
	if route.DelayMillis > 0 {
		timer := time.NewTimer(time.Duration(route.DelayMillis) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			return
		}
	}

	responseBody, err := renderMockResponseBody(route.ResponseBody, route, r, body, mockCfg)
	if err != nil {
		writeRuntimeError(w, http.StatusInternalServerError, fmt.Sprintf("渲染 Mock 响应失败: %v", err))
		return
	}

	writeHeaders(w, route.ResponseHeaders)
	applyMockCORS(w, r)
	if w.Header().Get("Content-Type") == "" {
		switch route.ResponseBodyType {
		case ResponseBodyTypeText:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		case ResponseBodyTypeRaw:
			w.Header().Set("Content-Type", "application/octet-stream")
		default:
			w.Header().Set("Content-Type", "application/json")
		}
	}
	w.WriteHeader(route.ResponseStatus)
	if responseBody != "" {
		if _, err := w.Write([]byte(responseBody)); err != nil {
			return
		}
	}
}

func applyMockCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	if requestedHeaders := r.Header.Get("Access-Control-Request-Headers"); requestedHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
	} else {
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	}
	w.Header().Set("Access-Control-Max-Age", "600")
}

func writeHeaders(w http.ResponseWriter, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var headers map[string]any
	if err := json.Unmarshal(raw, &headers); err != nil {
		return
	}
	for key, value := range headers {
		w.Header().Set(key, fmt.Sprint(value))
	}
}

func writeRuntimeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		return
	}
}
