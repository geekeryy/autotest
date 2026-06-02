package sampler

import (
	"encoding/json"
	"testing"

	"autotest/internal/testprofile"
)

func TestFromSchemaWithProfileUsesObservedValues(t *testing.T) {
	schema := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"status": {"type": "string"},
				"name":   {"type": "string"}
			}
		}
	}`)

	// Profile says "status" has been observed with these values.
	profiles := []testprofile.FieldProfile{
		{
			Method:         "POST",
			Path:           "/api/users",
			Location:       "body",
			FieldPath:      "status",
			ValueType:      "string",
			IsEnum:         true,
			ObservedValues: []any{"active", "inactive", "pending"},
		},
	}

	adapter := NewFieldProfileAdapter("POST", "/api/users", profiles)
	sample := FromSchemaWithOptions(schema, Options{
		PreferMockTags: false,
		Profile:        adapter,
	})

	body, ok := sample.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body to be map, got %T", sample.Body)
	}

	status, ok := body["status"].(string)
	if !ok {
		t.Fatalf("expected status to be string, got %T", body["status"])
	}
	if status != "active" && status != "inactive" && status != "pending" {
		t.Errorf("expected status to be one of [active, inactive, pending], got %q", status)
	}

	// "name" has no profile, should get a random value (not empty).
	name, ok := body["name"].(string)
	if !ok {
		t.Fatalf("expected name to be string, got %T", body["name"])
	}
	if name == "" {
		t.Error("expected name to be non-empty (random fallback)")
	}
}

func TestFromSchemaWithDependencyHintsSkipsFields(t *testing.T) {
	schema := json.RawMessage(`{
		"body": {
			"type": "object",
			"properties": {
				"userId": {"type": "string", "format": "uuid"},
				"name":   {"type": "string"}
			}
		}
	}`)

	deps := &DependencyHints{
		Hints: []DependencyHint{
			{
				FieldPath:    "userId",
				SourceMethod: "POST",
				SourcePath:   "/api/users",
				SourceField:  "response.body.data.id",
				ConsumerKind: "body_field",
			},
		},
	}

	sample := FromSchemaWithOptions(schema, Options{
		PreferMockTags: false,
		Dependencies:   deps,
	})

	body, ok := sample.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected body to be map, got %T", sample.Body)
	}

	userId, ok := body["userId"].(string)
	if !ok {
		t.Fatalf("expected userId to be string, got %T", body["userId"])
	}
	if !containsSubstring(userId, "__DEPENDENCY:") {
		t.Errorf("expected userId to contain dependency placeholder, got %q", userId)
	}

	// "name" should still get a normal value.
	name, ok := body["name"].(string)
	if !ok {
		t.Fatalf("expected name to be string, got %T", body["name"])
	}
	if name == "" {
		t.Error("expected name to be non-empty")
	}
}

func TestFromSchemaWithPathParamDependency(t *testing.T) {
	schema := json.RawMessage(`{
		"parameters": [
			{
				"name": "id",
				"in": "path",
				"schema": {"type": "string", "format": "uuid"}
			}
		],
		"body": {
			"type": "object",
			"properties": {
				"name": {"type": "string"}
			}
		}
	}`)

	deps := &DependencyHints{
		Hints: []DependencyHint{
			{
				FieldPath:    "id",
				SourceMethod: "POST",
				SourcePath:   "/api/users",
				SourceField:  "response.body.data.id",
				ConsumerKind: "path_param",
			},
		},
	}

	sample := FromSchemaWithOptions(schema, Options{
		PreferMockTags: false,
		Dependencies:   deps,
	})

	id, ok := sample.Path["id"]
	if !ok {
		t.Fatalf("expected path param 'id', got %v", sample.Path)
	}
	if !containsSubstring(id, "__DEPENDENCY:") {
		t.Errorf("expected id to contain dependency placeholder, got %q", id)
	}
}

func TestDependencyHintsHasDependency(t *testing.T) {
	dh := &DependencyHints{
		Hints: []DependencyHint{
			{FieldPath: "userId"},
			{FieldPath: "orderId"},
		},
	}
	if !dh.HasDependency("userId") {
		t.Error("expected HasDependency('userId') to be true")
	}
	if !dh.HasDependency("orderId") {
		t.Error("expected HasDependency('orderId') to be true")
	}
	if dh.HasDependency("name") {
		t.Error("expected HasDependency('name') to be false")
	}

	var nilDh *DependencyHints
	if nilDh.HasDependency("userId") {
		t.Error("expected nil DependencyHints to return false")
	}
}

func TestFieldProfileAdapterFiltersByMethodPath(t *testing.T) {
	profiles := []testprofile.FieldProfile{
		{
			Method:         "POST",
			Path:           "/api/users",
			Location:       "body",
			FieldPath:      "status",
			ObservedValues: []any{"active"},
		},
		{
			Method:         "GET",
			Path:           "/api/users",
			Location:       "body",
			FieldPath:      "status",
			ObservedValues: []any{"deleted"},
		},
	}

	adapter := NewFieldProfileAdapter("POST", "/api/users", profiles)
	vals := adapter.Lookup("status")
	if len(vals) != 1 {
		t.Fatalf("expected 1 value, got %d", len(vals))
	}
	if vals[0] != "active" {
		t.Errorf("expected 'active', got %v", vals[0])
	}

	// Different method should not match.
	adapter2 := NewFieldProfileAdapter("PUT", "/api/users", profiles)
	vals2 := adapter2.Lookup("status")
	if len(vals2) != 0 {
		t.Errorf("expected 0 values for PUT, got %d", len(vals2))
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
