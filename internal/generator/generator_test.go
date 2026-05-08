package generator

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestHappyPathFingerprintIsIdempotent(t *testing.T) {
	t.Parallel()

	endpointID := uuid.New()
	endpoint := Endpoint{
		ID:          endpointID,
		ProjectID:   uuid.New(),
		ServiceID:   uuid.New(),
		Method:      "POST",
		Path:        "/users",
		OperationID: "createUser",
		Summary:     "Create user",
		RequestSchema: []byte(`{
			"body": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"}
				}
			}
		}`),
		ResponseSchema: []byte(`{"status":"201"}`),
	}

	g := NewDefault()
	first, err := g.Generate(endpoint)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	second, err := g.Generate(endpoint)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected only happy path draft, got %d and %d", len(first), len(second))
	}
	if first[0].Fingerprint == "" {
		t.Fatalf("expected fingerprint")
	}
	if first[0].Fingerprint != second[0].Fingerprint {
		t.Fatalf("expected stable fingerprint: %s != %s", first[0].Fingerprint, second[0].Fingerprint)
	}
	if first[0].Name != "Create user" {
		t.Fatalf("expected API name, got %s", first[0].Name)
	}
}

func TestHappyPathSamplesRequestSchema(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		ServiceID:   uuid.New(),
		Method:      "POST",
		Path:        "/auth/login",
		OperationID: "login",
		Summary:     "Login",
		RequestSchema: []byte(`{
			"parameters": [
				{"name": "tenant", "in": "query", "schema": {"type": "string", "example": "demo"}},
				{"name": "X-Trace", "in": "header", "schema": {"type": "string", "default": "trace-1"}}
			],
			"security": [
				{"BearerAuth": []}
			],
			"body": {
				"type": "object",
				"properties": {
					"username": {"type": "string", "example": "admin"},
					"password": {"type": "string", "example": "admin123"}
				}
			}
		}`),
		ResponseSchema: []byte(`{"status":"200"}`),
	}

	drafts, err := NewDefault().Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}

	var request struct {
		Headers  map[string]string     `json:"headers"`
		Query    map[string]string     `json:"query"`
		Body     map[string]any        `json:"body"`
		Security []map[string][]string `json:"security"`
	}
	if err := json.Unmarshal(drafts[0].Request, &request); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if request.Query["tenant"] != "demo" {
		t.Fatalf("expected query sample, got %#v", request.Query)
	}
	if request.Headers["X-Trace"] != "trace-1" || request.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected header samples, got %#v", request.Headers)
	}
	if request.Body["username"] != "admin" || request.Body["password"] != "admin123" {
		t.Fatalf("expected login body samples, got %#v", request.Body)
	}
	if len(request.Security) != 1 || len(request.Security[0]) != 1 {
		t.Fatalf("expected security requirement, got %#v", request.Security)
	}
}
