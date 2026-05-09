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

func TestFromSchemaWithOptionsPreferMockTagsEmitsPlaceholders(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"parameters": [
			{"name": "email", "in": "query", "schema": {"type": "string"}},
			{"name": "id",    "in": "path",  "schema": {"type": "string"}},
			{"name": "X-Request-Id", "in": "header", "schema": {"type": "string", "format": "uuid"}}
		],
		"body": {
			"type": "object",
			"properties": {
				"id":         {"type": "string"},
				"email":      {"type": "string"},
				"phone":      {"type": "string"},
				"createdAt":  {"type": "string", "format": "date-time"},
				"name":       {"type": "string"},
				"role":       {"type": "string"},
				"age":        {"type": "integer"},
				"price":      {"type": "number"},
				"active":     {"type": "boolean"},
				"tags":       {"type": "array", "items": {"type": "string", "format": "email"}}
			}
		}
	}`)

	sample := FromSchemaWithOptions(raw, Options{PreferMockTags: true})

	body, ok := sample.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body map, got %T", sample.Body)
	}

	wantString := map[string]string{
		"id":        "{{$mock.uuid}}",
		"email":     "{{$mock.email}}",
		"phone":     "{{$mock.phone}}",
		"createdAt": "{{$mock.dateTime}}",
		"name":      "{{$mock.name}}",
		"role":      "{{$mock.pick(admin,tester,viewer,developer)}}",
	}
	for key, expected := range wantString {
		got, _ := body[key].(string)
		if got != expected {
			t.Errorf("body[%q] = %q, want %q", key, got, expected)
		}
	}

	// Non-string fields must keep concrete typed values so the generated body
	// remains a valid JSON shape (mock tags only render at the string level).
	if _, ok := body["age"].(int); !ok {
		if _, ok := body["age"].(float64); !ok {
			t.Errorf("expected numeric age value, got %T", body["age"])
		}
	}
	if _, ok := body["price"].(float64); !ok {
		t.Errorf("expected float price value, got %T", body["price"])
	}
	if _, ok := body["active"].(bool); !ok {
		t.Errorf("expected boolean active value, got %T", body["active"])
	}

	// Array items still emit mock tags element by element, so the surrounding
	// JSON array structure stays valid.
	tags, _ := body["tags"].([]any)
	if len(tags) != 1 || tags[0] != "{{$mock.email}}" {
		t.Errorf("expected tags=[{{$mock.email}}], got %#v", tags)
	}

	if sample.Query["email"] != "{{$mock.email}}" {
		t.Errorf("expected query email tagged, got %q", sample.Query["email"])
	}
	if sample.Path["id"] != "{{$mock.uuid}}" {
		t.Errorf("expected path id tagged, got %q", sample.Path["id"])
	}
	if sample.Headers["X-Request-Id"] != "{{$mock.uuid}}" {
		t.Errorf("expected header X-Request-Id tagged via format, got %q", sample.Headers["X-Request-Id"])
	}
}

func TestFromSchemaWithOptionsPreferMockTagsRespectsExampleAndEnum(t *testing.T) {
	t.Parallel()

	// example / default / enum win over mock tags so users can pin specific
	// values without losing them when they click "生成参数".
	raw := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"id":     {"type": "string", "format": "uuid", "example": "fixed-id"},
				"email":  {"type": "string", "default": "fixed@example.com"},
				"role":   {"type": "string", "enum": ["admin", "tester"]},
				"plain":  {"type": "string"}
			}
		}
	}`)

	body, ok := FromSchemaWithOptions(raw, Options{PreferMockTags: true}).Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body map, got %T", FromSchemaWithOptions(raw, Options{PreferMockTags: true}).Body)
	}
	if body["id"] != "fixed-id" {
		t.Errorf("expected example to win over mock tag, got %#v", body["id"])
	}
	if body["email"] != "fixed@example.com" {
		t.Errorf("expected default to win over mock tag, got %#v", body["email"])
	}
	if body["role"] != "admin" {
		t.Errorf("expected enum[0] to win over mock tag, got %#v", body["role"])
	}
	// "plain" has no schema metadata or recognised name, so the fallback is a
	// concrete random word — never a placeholder. We only assert it is non-
	// empty and not a placeholder string.
	plain, _ := body["plain"].(string)
	if plain == "" || strings.HasPrefix(plain, "{{$mock") {
		t.Errorf("expected concrete word for plain field, got %q", plain)
	}
}

func TestFromSchemaDefaultsKeepConcreteValues(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"id":    {"type": "string"},
				"email": {"type": "string"}
			}
		}
	}`)

	body, ok := FromSchema(raw).Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body map, got %T", FromSchema(raw).Body)
	}
	for _, key := range []string{"id", "email"} {
		got, _ := body[key].(string)
		if strings.HasPrefix(got, "{{$mock") {
			t.Errorf("FromSchema should keep concrete values for backward compatibility (importer), got %s=%q", key, got)
		}
	}
}
