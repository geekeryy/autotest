package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopbackAPIBaseURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		":8080":              "http://127.0.0.1:8080/api/v1",
		"0.0.0.0:8080":       "http://127.0.0.1:8080/api/v1",
		"127.0.0.1:9090":     "http://127.0.0.1:9090/api/v1",
		"localhost:3000":     "http://localhost:3000/api/v1",
	}
	for in, want := range cases {
		if got := LoopbackAPIBaseURL(in); got != want {
			t.Errorf("LoopbackAPIBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfigFromRequest(t *testing.T) {
	t.Parallel()
	defaults := Config{APIBaseURL: "http://127.0.0.1:8080/api/v1", ProjectID: "p1"}
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer at-secret-key")

	cfg, err := ConfigFromRequest(req, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "at-secret-key" || cfg.ProjectID != "p1" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigFromRequest_missingAuth(t *testing.T) {
	t.Parallel()
	_, err := ConfigFromRequest(httptest.NewRequest(http.MethodPost, "/mcp", nil), Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHTTPHandler_unauthorized(t *testing.T) {
	t.Parallel()
	h := NewHTTPHandler(Config{APIBaseURL: "http://127.0.0.1:8080/api/v1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
