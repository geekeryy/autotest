package aifactory

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestGenerateNestedBody(t *testing.T) {
	schema := json.RawMessage(`{
		"parameters": [],
		"body": {
			"type": "object",
			"properties": {
				"address": {
					"type": "object",
					"properties": {
						"city": {"type": "string"},
						"zip": {"type": "string"}
					}
				},
				"name": {"type": "string"}
			}
		}
	}`)

	factory := NewFactory()
	out, err := factory.Generate(schema, GenerateInput{
		EndpointID: uuid.New(),
		Count:      1,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(out.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(out.Rows))
	}

	row := out.Rows[0]
	address, ok := row["address"].(map[string]any)
	if !ok {
		t.Fatalf("address should be nested map, got %#v", row["address"])
	}
	if _, ok := address["city"]; !ok {
		t.Fatal("expected address.city")
	}
	if _, ok := row["name"]; !ok {
		t.Fatal("expected top-level name")
	}
	if _, ok := row["address.city"]; ok {
		t.Fatal("should not use dotted flat keys")
	}
}

func TestPickEnumBoundary(t *testing.T) {
	enum := []any{"small", "medium", "large"}
	first := pickEnum(enum, "boundary")
	last := pickEnum(enum, "boundary")
	if first != "small" && first != "large" {
		t.Fatalf("boundary pick should be edge value, got %v", first)
	}
	if last != "small" && last != "large" {
		t.Fatalf("boundary pick should be edge value, got %v", last)
	}
}
