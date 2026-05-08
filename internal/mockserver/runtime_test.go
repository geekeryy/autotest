package mockserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMockHandlerHandlesCORSPreflight(t *testing.T) {
	runtime := NewRuntime(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Headers", "content-type,x-mock-case")
	recorder := httptest.NewRecorder()

	runtime.mockHandler(uuid.New()).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("Access-Control-Allow-Methods is empty")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "content-type,x-mock-case" {
		t.Fatalf("Access-Control-Allow-Headers = %q, want requested headers", got)
	}
}

func TestWriteMockResponseRendersRequestReferences(t *testing.T) {
	body := `{"user":{"name":"jiangyang","active":true},"count":2}`
	route := testRoute(http.MethodPost, "/api/users/{id}")
	route.ResponseBody = `{"id":"{{request.pathvar.id}}","mode":"{{request.query.mode}}","name":"{{request.body.user.name}}","active":{{request.body.user.active}},"trace":"{{request.header.X-Trace}}","raw":{{request.bodyRaw}},"literal":"{{other.value}}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users/42?mode=preview", strings.NewReader(body))
	req.Header.Set("X-Trace", "abc-123")
	recorder := httptest.NewRecorder()

	writeMockResponse(recorder, req, route, []byte(body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, recorder.Body.String())
	}
	if got["id"] != "42" {
		t.Fatalf("id = %v, want 42", got["id"])
	}
	if got["mode"] != "preview" {
		t.Fatalf("mode = %v, want preview", got["mode"])
	}
	if got["name"] != "jiangyang" {
		t.Fatalf("name = %v, want jiangyang", got["name"])
	}
	if got["active"] != true {
		t.Fatalf("active = %v, want true", got["active"])
	}
	if got["trace"] != "abc-123" {
		t.Fatalf("trace = %v, want abc-123", got["trace"])
	}
	if got["literal"] != "{{other.value}}" {
		t.Fatalf("literal = %v, want untouched placeholder", got["literal"])
	}
	raw, ok := got["raw"].(map[string]any)
	if !ok || raw["count"] != float64(2) {
		t.Fatalf("raw = %#v, want object with count=2", got["raw"])
	}
}

func TestWriteMockResponseReportsMissingRequestReference(t *testing.T) {
	route := testRoute(http.MethodGet, "/api/users/{id}")
	route.ResponseBody = `{"missing":"{{request.query.missing}}"}`
	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	recorder := httptest.NewRecorder()

	writeMockResponse(recorder, req, route, nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "request.query.missing") {
		t.Fatalf("error body = %q, want missing reference name", recorder.Body.String())
	}
}
