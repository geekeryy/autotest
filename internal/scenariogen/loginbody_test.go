package scenariogen

import (
	"encoding/json"
	"testing"

	"autotest/internal/spec"
)

func TestBuildLoginRequestBodyOAuthFields(t *testing.T) {
	t.Parallel()
	ep := spec.Endpoint{
		RequestSchema: mustSchema(t, `{
			"body": {
				"type": "object",
				"required": ["code", "source"],
				"properties": {
					"code": {"type": "string", "description": "登录code"},
					"source": {"type": "integer", "default": 1},
					"union_id": {"type": "string"},
					"phone_code": {"type": "string", "description": "手机区号"}
				}
			}
		}`),
	}
	raw, pending, needs := BuildLoginRequestBody(ep, nil, nil, false)
	if !needs {
		t.Fatal("expected needs confirm")
	}
	if len(pending) < 2 {
		t.Fatalf("expected pending credential fields, got %v", pending)
	}
	var ro map[string]any
	if err := json.Unmarshal(raw, &ro); err != nil {
		t.Fatal(err)
	}
	body, _ := ro["body"].(map[string]any)
	if body["source"] != float64(1) && body["source"] != 1 {
		t.Fatalf("source default missing: %#v", body["source"])
	}
	if body["code"] != FillFieldPlaceholder("code") {
		t.Fatalf("code placeholder=%v", body["code"])
	}
}

func TestBuildLoginRequestBodyMergesUserBody(t *testing.T) {
	t.Parallel()
	ep := spec.Endpoint{
		RequestSchema: mustSchema(t, `{
			"body": {
				"type": "object",
				"required": ["code"],
				"properties": {
					"code": {"type": "string"},
					"union_id": {"type": "string"}
				}
			}
		}`),
	}
	creds := &LoginCredentialBundle{
		User: &CredentialPair{
			Body: map[string]any{
				"code":     "wx-code-123",
				"union_id": "u-1",
			},
		},
	}
	raw, pending, needs := BuildLoginRequestBody(ep, nil, creds, false)
	if needs || len(pending) > 0 {
		t.Fatalf("unexpected pending=%v needs=%v", pending, needs)
	}
	var ro map[string]any
	_ = json.Unmarshal(raw, &ro)
	body, _ := ro["body"].(map[string]any)
	if body["code"] != "wx-code-123" || body["union_id"] != "u-1" {
		t.Fatalf("body=%#v", body)
	}
}

func TestCredentialValueForField(t *testing.T) {
	t.Parallel()
	creds := &LoginCredentialBundle{
		User: &CredentialPair{
			Body: map[string]any{"phone_code": "+86"},
		},
	}
	val, ok := CredentialValueForField("phone_code", creds, false)
	if !ok || val != "+86" {
		t.Fatalf("got %q ok=%v", val, ok)
	}
}
