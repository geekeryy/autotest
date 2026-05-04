package runner

import (
	"context"
	"testing"

	"autotest/internal/project"
)

func TestRenderVariablesHandlesWrappedPlaceholders(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"id": "42", "token": "abc$123"}

	if got := RenderVariables("/api/v1/users/{{id}}", vars); got != "/api/v1/users/42" {
		t.Fatalf("expected normal placeholder to render, got %q", got)
	}
	if got := RenderVariables("/api/v1/users/{{{{id}}}}", vars); got != "/api/v1/users/42" {
		t.Fatalf("expected over-wrapped placeholder to render, got %q", got)
	}
	if got := RenderVariables("Bearer {{token}}", vars); got != "Bearer abc$123" {
		t.Fatalf("expected replacement value to be used literally, got %q", got)
	}
}

func TestMergeVariablesUsesEnvironmentAndOverrides(t *testing.T) {
	t.Parallel()

	vars, err := mergeVariables([]byte(`{"token":"env-token","page":2,"debug":true}`), map[string]any{
		"token": "request-token",
		"user":  "alice",
		"retry": 3,
	})
	if err != nil {
		t.Fatalf("merge variables: %v", err)
	}

	if vars["token"] != "request-token" {
		t.Fatalf("expected override token, got %q", vars["token"])
	}
	if vars["page"] != "2" {
		t.Fatalf("expected numeric environment variable to be stringified, got %q", vars["page"])
	}
	if vars["debug"] != "true" {
		t.Fatalf("expected boolean environment variable to be stringified, got %q", vars["debug"])
	}
	if vars["user"] != "alice" {
		t.Fatalf("expected request variable to be included, got %q", vars["user"])
	}
	if vars["retry"] != "3" {
		t.Fatalf("expected numeric request variable to be stringified, got %q", vars["retry"])
	}
}

func TestBuildHTTPRequestAppliesEnvironmentAuthForSecuredRequest(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method:   "GET",
		Path:     "/api/v1/users",
		Security: []map[string][]string{{"BearerAuth": []string{}}},
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth:    []byte(`{"type":"bearer","token":"{{token}}"}`),
	}, map[string]string{"token": "env-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer env-token" {
		t.Fatalf("expected bearer auth header, got %q", got)
	}
}

func TestBuildHTTPRequestSelectsAuthProfileBySecurityScheme(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method:   "GET",
		Path:     "/api/v1/admin/users",
		Security: []map[string][]string{{"AdminAuth": []string{}}},
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"defaultProfile":"user",
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"},
				"admin":{"type":"bearer","token":"{{adminToken}}"}
			},
			"securitySchemes":{"UserAuth":"user","AdminAuth":"admin"},
			"pathRules":[{"prefix":"/api/v1/user","profile":"user"}]
		}`),
	}, map[string]string{"userToken": "user-token", "adminToken": "admin-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer admin-token" {
		t.Fatalf("expected admin auth header, got %q", got)
	}
}

func TestBuildHTTPRequestSelectsAuthProfileParametersBySecurityScheme(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method:   "GET",
		Path:     "/api/v1/admin/users",
		Security: []map[string][]string{{"AdminKey": []string{}}},
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"defaultProfile":"user",
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"},
				"admin":{"type":"api_key","in":"query","name":"admin_token","value":"{{adminToken}}"}
			},
			"securitySchemes":{"UserAuth":"user","AdminKey":"admin"}
		}`),
	}, map[string]string{"userToken": "user-token", "adminToken": "admin-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.URL.Query().Get("admin_token"); got != "admin-token" {
		t.Fatalf("expected admin auth query parameter, got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no bearer auth header, got %q", got)
	}
}

func TestBuildHTTPRequestSelectsAuthProfileByPathRule(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "GET",
		Path:   "/api/v1/admin/users",
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"},
				"admin":{"type":"bearer","token":"{{adminToken}}"}
			},
			"pathRules":[
				{"prefix":"/api/v1","profile":"user"},
				{"prefix":"/api/v1/admin","profile":"admin"}
			]
		}`),
	}, map[string]string{"userToken": "user-token", "adminToken": "admin-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer admin-token" {
		t.Fatalf("expected admin auth header, got %q", got)
	}
}

func TestBuildHTTPRequestUsesDefaultAuthProfileForUnmappedSecurity(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method:   "GET",
		Path:     "/api/v1/users",
		Security: []map[string][]string{{"BearerAuth": []string{}}},
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"defaultProfile":"user",
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"}
			}
		}`),
	}, map[string]string{"userToken": "user-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer user-token" {
		t.Fatalf("expected default auth header, got %q", got)
	}
}

func TestBuildHTTPRequestSkipsEnvironmentAuthWithoutSecurity(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "GET",
		Path:   "/api/v1/public",
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth:    []byte(`{"type":"bearer","token":"env-token"}`),
	}, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no auth header, got %q", got)
	}
}

func TestBuildHTTPRequestFallsBackToDefaultProfileWhenSecurityMissing(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "DELETE",
		Path:   "/api/v1/admin/users/1",
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"defaultProfile":"admin",
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"},
				"admin":{"type":"bearer","token":"{{adminToken}}"}
			},
			"securitySchemes":{"BearerAuth":"user","AdminBearerAuth":"admin"}
		}`),
	}, map[string]string{"userToken": "user-token", "adminToken": "admin-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer admin-token" {
		t.Fatalf("expected default profile to fallback, got %q", got)
	}
}

func TestBuildHTTPRequestSkipsAuthWhenDefaultProfileAbsent(t *testing.T) {
	t.Parallel()

	req, _, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "GET",
		Path:   "/api/v1/public",
	}, project.Environment{
		BaseURL: "https://example.test",
		Auth: []byte(`{
			"profiles":{
				"user":{"type":"bearer","token":"{{userToken}}"}
			}
		}`),
	}, map[string]string{"userToken": "user-token"})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no auth header without default profile, got %q", got)
	}
}
