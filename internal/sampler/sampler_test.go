package sampler

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestFromSchemaPrefersExampleDefaultEnum(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"parameters": [
			{"name": "tenant", "in": "query", "schema": {"type": "string", "example": "demo"}},
			{"name": "X-Trace", "in": "header", "schema": {"type": "string", "default": "trace-1"}},
			{"name": "id", "in": "path", "schema": {"type": "string", "enum": ["alpha", "beta"]}}
		],
		"body": {
			"type": "object",
			"properties": {
				"username": {"type": "string", "example": "admin"},
				"role": {"type": "string", "enum": ["tester", "admin"]}
			}
		}
	}`)

	sample := FromSchema(raw)
	if sample.Query["tenant"] != "demo" {
		t.Fatalf("expected query example, got %#v", sample.Query)
	}
	if sample.Headers["X-Trace"] != "trace-1" {
		t.Fatalf("expected header default, got %#v", sample.Headers)
	}
	if sample.Path["id"] != "alpha" {
		t.Fatalf("expected path enum, got %#v", sample.Path)
	}
	if sample.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type to be auto-added, got %#v", sample.Headers)
	}

	body, _ := sample.Body.(map[string]any)
	if body["username"] != "admin" {
		t.Fatalf("expected body example, got %#v", body)
	}
	if body["role"] != "tester" {
		t.Fatalf("expected body enum, got %#v", body)
	}
}

func TestFromSchemaFallbackProducesFreshRandomValues(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"id":         {"type": "string"},
				"name":       {"type": "string"},
				"email":      {"type": "string"},
				"phone":      {"type": "string"},
				"createdAt":  {"type": "string"},
				"age":        {"type": "integer"},
				"price":      {"type": "number"}
			}
		}
	}`)

	first, ok := FromSchema(raw).Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body map, got %T", FromSchema(raw).Body)
	}
	second, ok := FromSchema(raw).Body.(map[string]any)
	if !ok {
		t.Fatalf("expected second body map, got %T", FromSchema(raw).Body)
	}

	// At least one of these random fields should differ between two calls.
	differs := false
	for _, key := range []string{"id", "name", "email", "phone", "createdAt", "age", "price"} {
		if first[key] != second[key] {
			differs = true
			break
		}
	}
	if !differs {
		t.Fatalf("expected fallback values to vary between calls, both produced %#v", first)
	}

	emailRE := regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	if email, _ := first["email"].(string); !emailRE.MatchString(email) {
		t.Fatalf("expected email-like value, got %q", email)
	}
	if name, _ := first["name"].(string); strings.TrimSpace(name) == "" {
		t.Fatalf("expected non-empty name, got %q", name)
	}
	uuidRE := regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	if id, _ := first["id"].(string); !uuidRE.MatchString(id) {
		t.Fatalf("expected uuid-like id value, got %q", id)
	}
	if _, ok := first["age"].(float64); !ok {
		// JSON unmarshalling into any uses float64; since we already have map[string]any
		// from the unmarshal of body schema sample, integerSample returns int. Allow either.
		if _, intOk := first["age"].(int); !intOk {
			t.Fatalf("expected numeric age value, got %T", first["age"])
		}
	}
}

func TestFromSchemaFormatHonoursExplicitExample(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"createdAt": {"type": "string", "format": "date-time", "example": "2026-01-01T00:00:00Z"}
			}
		}
	}`)

	body, ok := FromSchema(raw).Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body map, got %T", FromSchema(raw).Body)
	}
	if body["createdAt"] != "2026-01-01T00:00:00Z" {
		t.Fatalf("expected explicit example to win, got %#v", body)
	}
}

func TestFromSchemaPreservesSecurity(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"security": [{"BearerAuth": []}],
		"body": {"type": "object"}
	}`)

	sample := FromSchema(raw)
	if sample.Security == nil {
		t.Fatalf("expected security to be preserved")
	}
}
