package testprofile

import (
	"encoding/json"
	"testing"
)

func TestInferDependencies_CRUDPattern(t *testing.T) {
	endpoints := []EndpointInfo{
		{
			Method:         "POST",
			Path:           "/api/v1/users",
			ResponseSchema: json.RawMessage(`{"body":{"properties":{"id":{"type":"string"},"name":{"type":"string"}}}}`),
		},
		{
			Method: "GET",
			Path:   "/api/v1/users/{id}",
		},
		{
			Method: "PUT",
			Path:   "/api/v1/users/{id}",
		},
		{
			Method: "DELETE",
			Path:   "/api/v1/users/{id}",
		},
	}

	deps := InferDependencies(endpoints)
	if len(deps) == 0 {
		t.Fatal("expected at least one dependency")
	}

	found := false
	for _, dep := range deps {
		if dep.SourceMethod == "POST" && dep.SourcePath == "/api/v1/users" &&
			dep.TargetPath == "/api/v1/users/{id}" && dep.TargetField == "path.id" {
			found = true
			if dep.Confidence < 0.5 {
				t.Errorf("expected confidence >= 0.5, got %f", dep.Confidence)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected dependency from POST /users to GET /users/{id}, got %+v", deps)
	}
}

func TestInferDependencies_NoDeps(t *testing.T) {
	endpoints := []EndpointInfo{
		{Method: "GET", Path: "/api/v1/health"},
		{Method: "GET", Path: "/api/v1/config"},
	}

	deps := InferDependencies(endpoints)
	if len(deps) != 0 {
		t.Errorf("expected no dependencies, got %+v", deps)
	}
}

func TestExtractResource(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/users", "users"},
		{"/api/v1/users/{id}", "users"},
		{"/api/v1/users/{userId}/orders", "orders"},
		{"/api/v1/users/{userId}/orders/{orderId}", "orders"},
		{"/health", "health"},
	}

	for _, tt := range tests {
		result := extractResource(tt.path)
		if result != tt.expected {
			t.Errorf("extractResource(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestExtractPathParams(t *testing.T) {
	params := extractPathParams("/api/v1/users/{userId}/orders/{orderId}")
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0] != "userId" || params[1] != "orderId" {
		t.Errorf("unexpected params: %v", params)
	}
}
