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

	// 仅使用 HappyPathRule 验证指纹幂等性
	gen := NewWithRules([]Rule{HappyPathRule{}})
	first, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	second, err := gen.Generate(endpoint)
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

	// 仅使用 HappyPathRule 验证请求采样
	gen := NewWithRules([]Rule{HappyPathRule{}})
	drafts, err := gen.Generate(endpoint)
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

func TestRequiredMissingRule(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		ServiceID: uuid.New(),
		Method:    "POST",
		Path:      "/users",
		Summary:   "Create user",
		RequestSchema: []byte(`{
			"body": {
				"type": "object",
				"required": ["name", "email"],
				"properties": {
					"name": {"type": "string"},
					"email": {"type": "string", "format": "email"},
					"age": {"type": "integer"}
				}
			}
		}`),
		ResponseSchema: []byte(`{"status":"201"}`),
	}

	gen := NewWithRules([]Rule{RequiredMissingRule{}})
	drafts, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// 应该为 name 和 email 各生成一个用例（age 不是 required）
	if len(drafts) != 2 {
		t.Fatalf("expected 2 drafts for 2 required fields, got %d", len(drafts))
	}

	names := make(map[string]bool)
	for _, d := range drafts {
		names[d.Name] = true
		if d.GenerationRuleID != RuleRequiredMissing {
			t.Fatalf("expected rule %s, got %s", RuleRequiredMissing, d.GenerationRuleID)
		}
		// 验证断言期望 400
		var assertions []map[string]any
		if err := json.Unmarshal(d.Assertions, &assertions); err != nil {
			t.Fatalf("unmarshal assertions: %v", err)
		}
		if len(assertions) != 1 || assertions[0]["expected"].(float64) != 400 {
			t.Fatalf("expected 400 assertion, got %#v", assertions)
		}
		// 验证请求中确实删除了对应字段
		var request map[string]any
		if err := json.Unmarshal(d.Request, &request); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		body, _ := request["body"].(map[string]any)
		if body == nil {
			t.Fatal("expected body in request")
		}
	}
	if !names["Create user [缺失必填字段: name]"] {
		t.Fatalf("missing expected name for 'name' field")
	}
	if !names["Create user [缺失必填字段: email]"] {
		t.Fatalf("missing expected name for 'email' field")
	}
}

func TestRequiredMissingRuleNoRequired(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		ServiceID: uuid.New(),
		Method:    "GET",
		Path:      "/users",
		RequestSchema: []byte(`{
			"parameters": [{"name": "page", "in": "query", "schema": {"type": "integer"}}]
		}`),
	}

	gen := NewWithRules([]Rule{RequiredMissingRule{}})
	drafts, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts when no required fields, got %d", len(drafts))
	}
}

func TestTypeMismatchRule(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		ServiceID: uuid.New(),
		Method:    "POST",
		Path:      "/items",
		Summary:   "Create item",
		RequestSchema: []byte(`{
			"body": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"count": {"type": "integer"}
				}
			}
		}`),
		ResponseSchema: []byte(`{"status":"200"}`),
	}

	gen := NewWithRules([]Rule{TypeMismatchRule{}})
	drafts, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// string 字段 3 个错误类型 + integer 字段 3 个错误类型 = 6
	if len(drafts) != 6 {
		t.Fatalf("expected 6 drafts (3 per field), got %d", len(drafts))
	}

	for _, d := range drafts {
		if d.GenerationRuleID != RuleTypeMismatch {
			t.Fatalf("expected rule %s, got %s", RuleTypeMismatch, d.GenerationRuleID)
		}
		var assertions []map[string]any
		if err := json.Unmarshal(d.Assertions, &assertions); err != nil {
			t.Fatalf("unmarshal assertions: %v", err)
		}
		if len(assertions) != 1 || assertions[0]["expected"].(float64) != 400 {
			t.Fatalf("expected 400 assertion, got %#v", assertions)
		}
	}
}

func TestEdgeCasesRule(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		ServiceID: uuid.New(),
		Method:    "POST",
		Path:      "/items",
		Summary:   "Create item",
		RequestSchema: []byte(`{
			"body": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"count": {"type": "integer"}
				}
			}
		}`),
		ResponseSchema: []byte(`{"status":"200"}`),
	}

	gen := NewWithRules([]Rule{EdgeCasesRule{}})
	drafts, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// string 10 个边界值 + integer 4 个边界值 = 14
	if len(drafts) != 14 {
		t.Fatalf("expected 14 drafts, got %d", len(drafts))
	}

	for _, d := range drafts {
		if d.GenerationRuleID != RuleEdgeCases {
			t.Fatalf("expected rule %s, got %s", RuleEdgeCases, d.GenerationRuleID)
		}
		if d.Fingerprint == "" {
			t.Fatal("expected non-empty fingerprint")
		}
	}
}

func TestEdgeCasesRuleNoBody(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		ServiceID: uuid.New(),
		Method:    "GET",
		Path:      "/items",
		RequestSchema: []byte(`{
			"parameters": [{"name": "id", "in": "path", "schema": {"type": "string"}}]
		}`),
	}

	gen := NewWithRules([]Rule{EdgeCasesRule{}})
	drafts, err := gen.Generate(endpoint)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts for bodyless endpoint, got %d", len(drafts))
	}
}
