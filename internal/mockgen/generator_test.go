package mockgen

import (
	"encoding/json"
	"testing"
)

func TestGenerator_BasicTypes(t *testing.T) {
	g := Default()

	t.Run("string", func(t *testing.T) {
		schema := map[string]any{"type": "string"}
		v := g.Generate(schema)
		if _, ok := v.(string); !ok {
			t.Errorf("expected string, got %T: %v", v, v)
		}
	})

	t.Run("integer", func(t *testing.T) {
		schema := map[string]any{"type": "integer", "minimum": 10, "maximum": 20}
		v := g.Generate(schema)
		n, ok := v.(int)
		if !ok {
			t.Errorf("expected int, got %T: %v", v, v)
		}
		if n < 10 || n > 20 {
			t.Errorf("expected 10-20, got %d", n)
		}
	})

	t.Run("number", func(t *testing.T) {
		schema := map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0}
		v := g.Generate(schema)
		t.Logf("number value: %v (type: %T)", v, v)
		// number 可能返回 string（格式化后）或 float64
	})

	t.Run("boolean", func(t *testing.T) {
		schema := map[string]any{"type": "boolean"}
		v := g.Generate(schema)
		if _, ok := v.(bool); !ok {
			t.Errorf("expected bool, got %T: %v", v, v)
		}
	})
}

func TestGenerator_StringFormats(t *testing.T) {
	g := Default()

	formats := []string{"email", "uri", "url", "date-time", "date", "ipv4", "uuid", "hostname"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			schema := map[string]any{"type": "string", "format": format}
			v := g.Generate(schema)
			s, ok := v.(string)
			if !ok {
				t.Errorf("expected string, got %T: %v", v, v)
				return
			}
			if s == "" {
				t.Error("expected non-empty string")
			}
			t.Logf("%s: %s", format, s)
		})
	}
}

func TestGenerator_Enum(t *testing.T) {
	g := Default()
	schema := map[string]any{
		"type": "string",
		"enum": []any{"active", "inactive", "pending"},
	}

	for i := 0; i < 20; i++ {
		v := g.Generate(schema)
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expected string, got %T", v)
		}
		if s != "active" && s != "inactive" && s != "pending" {
			t.Errorf("unexpected enum value: %s", s)
		}
	}
}

func TestGenerator_Array(t *testing.T) {
	g := Default()
	schema := map[string]any{
		"type":     "array",
		"minItems": 2,
		"maxItems": 5,
		"items": map[string]any{
			"type": "string",
		},
	}

	v := g.Generate(schema)
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", v)
	}
	if len(arr) < 2 || len(arr) > 5 {
		t.Errorf("expected 2-5 items, got %d", len(arr))
	}
	for i, item := range arr {
		if _, ok := item.(string); !ok {
			t.Errorf("item[%d]: expected string, got %T: %v", i, item, item)
		}
	}
}

func TestGenerator_Object(t *testing.T) {
	g := Default()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type": "integer",
			},
			"name": map[string]any{
				"type": "string",
			},
			"email": map[string]any{
				"type":   "string",
				"format": "email",
			},
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []any{"id", "name", "email"},
	}

	v := g.Generate(schema)
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}

	// required 字段必须存在
	if _, ok := obj["id"]; !ok {
		t.Error("missing required field: id")
	}
	if _, ok := obj["name"]; !ok {
		t.Error("missing required field: name")
	}
	if _, ok := obj["email"]; !ok {
		t.Error("missing required field: email")
	}

	// 验证类型
	if _, ok := obj["id"].(int); !ok {
		t.Errorf("id: expected int, got %T", obj["id"])
	}
	if _, ok := obj["name"].(string); !ok {
		t.Errorf("name: expected string, got %T", obj["name"])
	}
	if _, ok := obj["email"].(string); !ok {
		t.Errorf("email: expected string, got %T", obj["email"])
	}

	t.Logf("generated object: %+v", obj)
}

func TestGenerator_SemanticInference(t *testing.T) {
	g := Default()

	tests := []struct {
		fieldName string
		check     func(string) bool
	}{
		{"userName", func(s string) bool { return len(s) > 0 }},
		{"phone", func(s string) bool { return len(s) == 11 && s[0] == '1' }},
		{"email", func(s string) bool { return len(s) > 0 }},
		{"city", func(s string) bool { return len(s) > 0 }},
		{"status", func(s string) bool { return s == "active" || s == "inactive" || s == "pending" || s == "disabled" }},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			schema := map[string]any{
				"type": "string",
				"title": tt.fieldName,
			}
			v := g.Generate(schema)
			s, ok := v.(string)
			if !ok {
				t.Fatalf("expected string, got %T: %v", v, v)
			}
			if !tt.check(s) {
				t.Errorf("semantic inference for %q failed: %s", tt.fieldName, s)
			}
			t.Logf("%s: %s", tt.fieldName, s)
		})
	}
}

func TestGenerator_Nullable(t *testing.T) {
	g := Default()
	schema := map[string]any{
		"type":     "string",
		"nullable": true,
	}

	// 多次生成，应该有概率返回 nil
	nilCount := 0
	for i := 0; i < 100; i++ {
		v := g.Generate(schema)
		if v == nil {
			nilCount++
		}
	}
	// 10% 概率，100 次应该至少有几次 nil
	if nilCount == 0 {
		t.Error("expected some nil values from nullable field, got none")
	}
	t.Logf("nil count: %d/100", nilCount)
}

func TestGenerator_GenerateJSON(t *testing.T) {
	g := Default()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "integer"},
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"id", "name"},
	}

	data, err := g.GenerateJSON(schema)
	if err != nil {
		t.Fatalf("GenerateJSON error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := obj["id"]; !ok {
		t.Error("missing id")
	}
	if _, ok := obj["name"]; !ok {
		t.Error("missing name")
	}
	t.Logf("JSON output: %s", string(data))
}

func TestGenerator_ResponseSchema(t *testing.T) {
	// 模拟从 ResponseSchema.body 提取的真实 schema
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":    "integer",
				"example": 200,
			},
			"message": map[string]any{
				"type":    "string",
				"example": "success",
			},
			"data": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"list": map[string]any{
						"type":     "array",
						"minItems": 1,
						"maxItems": 3,
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":         map[string]any{"type": "integer"},
								"userName":   map[string]any{"type": "string"},
								"phone":      map[string]any{"type": "string"},
								"email":      map[string]any{"type": "string", "format": "email"},
								"status":     map[string]any{"type": "string"},
								"createdAt":  map[string]any{"type": "string", "format": "date-time"},
							},
							"required": []any{"id", "userName"},
						},
					},
					"total": map[string]any{
						"type":    "integer",
						"minimum": 0,
						"maximum": 100,
					},
				},
			},
		},
		"required": []any{"code", "message", "data"},
	}

	g := Default()
	for i := 0; i < 5; i++ {
		data, err := g.GenerateJSON(schema)
		if err != nil {
			t.Fatalf("GenerateJSON error: %v", err)
		}
		t.Logf("Response %d: %s", i+1, string(data))
	}
}
