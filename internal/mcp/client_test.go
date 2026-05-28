package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

func TestAPIClientImportSpec(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	serviceID := uuid.New()
	specID := uuid.New()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: got %s", r.Method)
		}
		want := "/api/v1/projects/" + projectID.String() + "/services/" + serviceID.String() + "/specs/import"
		if r.URL.Path != want {
			t.Fatalf("path: got %s want %s", r.URL.Path, want)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer at-test-key" {
			t.Fatalf("authorization: got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type: got %q", got)
		}
		payload := spec.ImportSummary{
			SpecID:           specID,
			SpecVersion:      1,
			ApiCount:         2,
			CreatedEndpoints: 2,
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	client := NewAPIClient(Config{
		APIBaseURL: srv.URL + "/api/v1",
		APIKey:     "at-test-key",
	})
	summary, err := client.ImportSpec(context.Background(), projectID, serviceID, []byte(`{"openapi":"3.0.0"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.SpecID != specID || summary.ApiCount != 2 {
		t.Fatalf("summary: %+v", summary)
	}
}

func TestDetectContentType(t *testing.T) {
	t.Parallel()
	if got := detectContentType([]byte(`{"openapi":"3.0.0"}`)); got != "application/json" {
		t.Fatalf("json: %s", got)
	}
	if got := detectContentType([]byte("openapi: 3.0.0\n")); got != "application/yaml" {
		t.Fatalf("yaml: %s", got)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv(envAPIKey, "at-abc")
	t.Setenv(envAPIBaseURL, "http://example.com/api/v1/")
	t.Setenv(envProjectID, uuid.New().String())

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBaseURL != "http://example.com/api/v1" {
		t.Fatalf("base url: %q", cfg.APIBaseURL)
	}
	if cfg.APIKey != "at-abc" {
		t.Fatalf("key: %q", cfg.APIKey)
	}
}

func TestLoadConfigFromEnvRequiresKey(t *testing.T) {
	t.Setenv(envAPIKey, "")
	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}
