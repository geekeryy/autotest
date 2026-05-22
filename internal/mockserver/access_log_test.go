package mockserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTruncateAccessLogBody(t *testing.T) {
	short := strings.Repeat("a", 100)
	if got := truncateAccessLogBody(short); got != short {
		t.Fatalf("short body changed: len=%d", len(got))
	}

	long := strings.Repeat("b", maxAccessLogBodyBytes+10)
	got := truncateAccessLogBody(long)
	if len(got) <= maxAccessLogBodyBytes {
		t.Fatalf("truncated body len = %d, want > %d", len(got), maxAccessLogBodyBytes)
	}
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("truncated body suffix missing: %q", got[len(got)-20:])
	}
}

func TestCaptureRequestHeadersSanitizesAuthorization(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret-token")
	headers.Set("Content-Type", "application/json")
	headers.Set("User-Agent", "autotest/1.0")
	headers.Set("X-Trace-Id", "trace-1")

	raw := captureRequestHeaders(headers)
	text := string(raw)
	if strings.Contains(text, "secret-token") {
		t.Fatalf("authorization leaked: %s", text)
	}
	if !strings.Contains(text, "***") {
		t.Fatalf("authorization not masked: %s", text)
	}
	if !strings.Contains(text, "application/json") || !strings.Contains(text, "trace-1") {
		t.Fatalf("expected headers missing: %s", text)
	}
}

func TestCaptureResponseHeadersSkipsCORS(t *testing.T) {
	headers := http.Header{}
	headers.Set("Location", "https://example.com/callback")
	headers.Set("Content-Type", "application/json")
	headers.Set("Access-Control-Allow-Origin", "*")

	raw := captureResponseHeaders(headers)
	text := string(raw)
	if strings.Contains(text, "Access-Control-Allow-Origin") {
		t.Fatalf("cors header should be skipped: %s", text)
	}
	if !strings.Contains(text, "Location") {
		t.Fatalf("location header missing: %s", text)
	}
}

func TestResponseRecorderCapturesStatusAndBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := newResponseRecorder(recorder)

	wrapped.Header().Set("Content-Type", "application/json")
	wrapped.WriteHeader(http.StatusCreated)
	if _, err := wrapped.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if wrapped.Status() != http.StatusCreated {
		t.Fatalf("status = %d, want %d", wrapped.Status(), http.StatusCreated)
	}
	if wrapped.body.String() != `{"ok":true}` {
		t.Fatalf("body = %q", wrapped.body.String())
	}
	if recorder.Code != http.StatusCreated {
		t.Fatalf("underlying status = %d", recorder.Code)
	}
}

func TestClientIPFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if got := clientIPFromRequest(req); got != "127.0.0.1" {
		t.Fatalf("client ip = %q, want 127.0.0.1", got)
	}

	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	if got := clientIPFromRequest(req); got != "203.0.113.1" {
		t.Fatalf("forwarded ip = %q", got)
	}
}

type fakeAccessLogRepo struct {
	mu     sync.Mutex
	logs   []MockAccessLog
	server MockServer
	routes []MockRoute
}

func (f *fakeAccessLogRepo) GetServerByID(_ context.Context, serverID uuid.UUID) (*MockServer, error) {
	if serverID != f.server.ID {
		return nil, ErrMockServerNotFound
	}
	copyServer := f.server
	return &copyServer, nil
}

func (f *fakeAccessLogRepo) ListEnabledRoutesForServer(_ context.Context, serverID uuid.UUID) ([]MockRoute, error) {
	if serverID != f.server.ID {
		return nil, ErrMockServerNotFound
	}
	return append([]MockRoute(nil), f.routes...), nil
}

func (f *fakeAccessLogRepo) InsertAccessLog(_ context.Context, log MockAccessLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, log)
	return nil
}

func (f *fakeAccessLogRepo) lastLog() MockAccessLog {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.logs) == 0 {
		return MockAccessLog{}
	}
	return f.logs[len(f.logs)-1]
}

func TestMockHandlerRecordsAccessLogOnMatch(t *testing.T) {
	serverID := uuid.New()
	projectID := uuid.New()
	route := testRoute(http.MethodGet, "/api/ping")
	route.ID = uuid.New()
	route.MockServerID = serverID
	route.ResponseBody = `{"ok":true}`

	repo := &fakeAccessLogRepo{
		server: MockServer{ID: serverID, ProjectID: projectID, Port: 18081},
		routes: []MockRoute{route},
	}
	runtime := NewRuntime(repo, nil)
	runtime.mu.Lock()
	runtime.running[serverID] = &runningServer{server: repo.server, startedAt: time.Now()}
	runtime.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	recorder := httptest.NewRecorder()
	runtime.mockHandler(serverID).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		repo.mu.Lock()
		count := len(repo.logs)
		repo.mu.Unlock()
		if count > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("access log was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}

	log := repo.lastLog()
	if !log.Matched {
		t.Fatal("expected matched=true")
	}
	if log.ResponseStatus != http.StatusOK {
		t.Fatalf("response status = %d", log.ResponseStatus)
	}
	if log.MockRouteID == nil || *log.MockRouteID != route.ID {
		t.Fatalf("route id = %v, want %s", log.MockRouteID, route.ID)
	}
	if log.Path != "/api/ping" {
		t.Fatalf("path = %q", log.Path)
	}
}

func TestMockHandlerRecordsAccessLogOnNotFound(t *testing.T) {
	serverID := uuid.New()
	projectID := uuid.New()
	repo := &fakeAccessLogRepo{
		server: MockServer{ID: serverID, ProjectID: projectID, Port: 18082},
		routes: nil,
	}
	runtime := NewRuntime(repo, nil)
	runtime.mu.Lock()
	runtime.running[serverID] = &runningServer{server: repo.server, startedAt: time.Now()}
	runtime.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	recorder := httptest.NewRecorder()
	runtime.mockHandler(serverID).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		repo.mu.Lock()
		count := len(repo.logs)
		repo.mu.Unlock()
		if count > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("access log was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}

	log := repo.lastLog()
	if log.Matched {
		t.Fatal("expected matched=false")
	}
	if log.ResponseStatus != http.StatusNotFound {
		t.Fatalf("response status = %d", log.ResponseStatus)
	}
	if log.ErrorMessage != "mock route not found" {
		t.Fatalf("error message = %q", log.ErrorMessage)
	}
}

func TestMockHandlerSkipsOptionsAccessLog(t *testing.T) {
	serverID := uuid.New()
	repo := &fakeAccessLogRepo{
		server: MockServer{ID: serverID, ProjectID: uuid.New(), Port: 18083},
	}
	runtime := NewRuntime(repo, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/ping", nil)
	recorder := httptest.NewRecorder()
	runtime.mockHandler(serverID).ServeHTTP(recorder, req)

	time.Sleep(50 * time.Millisecond)
	repo.mu.Lock()
	count := len(repo.logs)
	repo.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected no access logs for OPTIONS, got %d", count)
	}
}
