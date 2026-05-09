package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	testcase "autotest/internal/case"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/report"

	"github.com/google/uuid"
)

func TestRenderVariablesHandlesWrappedPlaceholders(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"id": "42", "token": "abc$123"}

	if got := renderVariables("/api/v1/users/{{id}}", vars); got != "/api/v1/users/42" {
		t.Fatalf("expected normal placeholder to render, got %q", got)
	}
	if got := renderVariables("/api/v1/users/{{{{id}}}}", vars); got != "/api/v1/users/42" {
		t.Fatalf("expected over-wrapped placeholder to render, got %q", got)
	}
	if got := renderVariables("Bearer {{token}}", vars); got != "Bearer abc$123" {
		t.Fatalf("expected replacement value to be used literally, got %q", got)
	}
	if got := renderVariables("/api/v1/users/{{id}}", map[string]string{
		"id":              "{{sql.userSeed.id}}",
		"sql.userSeed.id": "99",
	}); got != "/api/v1/users/99" {
		t.Fatalf("expected nested inline placeholder to render, got %q", got)
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

func TestExecuteCaseRunVariantsRunsEveryVariant(t *testing.T) {
	t.Parallel()

	var seen []string
	first, results, status, err := executeCaseRunVariants(context.Background(), []caseRunVariant{
		{Variables: map[string]string{"id": "row-1"}},
		{Variables: map[string]string{"id": "row-2"}},
	}, func(_ context.Context, variant caseRunVariant) (*report.Result, error) {
		seen = append(seen, variant.Variables["id"])
		return &report.Result{Status: report.ResultPassed}, nil
	})
	if err != nil {
		t.Fatalf("execute variants: %v", err)
	}
	if first == nil {
		t.Fatalf("expected first result")
	}
	if status != report.RunPassed {
		t.Fatalf("expected run to pass, got %s", status)
	}
	if len(results) != 2 {
		t.Fatalf("expected two saved result records, got %d", len(results))
	}
	if strings.Join(seen, ",") != "row-1,row-2" {
		t.Fatalf("expected both rows to execute in order, got %#v", seen)
	}
}

func TestBuildHTTPRequestRendersNestedBodyVariables(t *testing.T) {
	t.Parallel()

	_, snapshot, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "POST",
		Path:   "/api/v1/users/{{userId}}",
		Body: map[string]any{
			"id": "{{userId}}",
			"profile": map[string]any{
				"email":  "{{email}}",
				"seedId": "{{sql.userSeed.id}}",
				"tags":   []any{"tenant-{{tenantId}}", "active"},
			},
		},
	}, project.Environment{BaseURL: "https://example.test"}, map[string]string{
		"userId":          "42",
		"email":           "u@example.test",
		"tenantId":        "acme",
		"sql.userSeed.id": "99",
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	got := string(snapshot)
	for _, want := range []string{"\"id\":\"42\"", "\"email\":\"u@example.test\"", "\"seedId\":\"99\"", "tenant-acme"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected snapshot body to contain %s, got %s", want, got)
		}
	}
}

func TestRenderVariablesExpandsMockDataTokens(t *testing.T) {
	t.Parallel()

	got := renderVariables("user-{{$mock.uuid}}-{{tenant}}", map[string]string{
		"tenant": "acme",
	})
	if !strings.HasSuffix(got, "-acme") {
		t.Fatalf("expected variable to be substituted, got %q", got)
	}
	if !strings.HasPrefix(got, "user-") {
		t.Fatalf("expected literal prefix preserved, got %q", got)
	}
	if strings.Contains(got, "$mock") || strings.Contains(got, "{{") {
		t.Fatalf("expected mock token to be expanded, got %q", got)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(got, "user-"), "-acme")
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("expected uuid in expansion, got %q (%v)", id, err)
	}
}

func TestBuildHTTPRequestExpandsMockTokensAcrossRequest(t *testing.T) {
	t.Parallel()

	req, snapshot, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method:  "POST",
		Path:    "/api/v1/users/{{$mock.uuid}}",
		Headers: map[string]string{"X-Trace": "trace-{{$mock.uuid}}"},
		Query:   map[string]string{"ts": "{{$mock.timestamp}}"},
		Body: map[string]any{
			"id":    "{{$mock.uuid}}",
			"email": "{{$mock.email}}",
		},
	}, project.Environment{BaseURL: "https://example.test"}, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if strings.Contains(req.URL.Path, "$mock") {
		t.Fatalf("expected URL path to be rendered, got %s", req.URL.String())
	}
	pathSegments := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	pathID := pathSegments[len(pathSegments)-1]
	if _, err := uuid.Parse(pathID); err != nil {
		t.Fatalf("expected path uuid, got %q (%v)", pathID, err)
	}

	trace := req.Header.Get("X-Trace")
	if !strings.HasPrefix(trace, "trace-") {
		t.Fatalf("expected trace header to be rendered, got %q", trace)
	}
	if _, err := uuid.Parse(strings.TrimPrefix(trace, "trace-")); err != nil {
		t.Fatalf("expected trace header uuid, got %q (%v)", trace, err)
	}

	tsRaw := req.URL.Query().Get("ts")
	if tsRaw == "" || strings.Contains(tsRaw, "$mock") {
		t.Fatalf("expected timestamp query value, got %q", tsRaw)
	}

	got := string(snapshot)
	if !strings.Contains(got, "\"email\":") || strings.Contains(got, "$mock") {
		t.Fatalf("expected body mock tokens replaced, snapshot=%s", got)
	}

	// The two body uuids should be independent (different) values.
	type bodyShape struct {
		Body struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"body"`
	}
	var parsed bodyShape
	if err := json.Unmarshal(snapshot, &parsed); err != nil {
		t.Fatalf("snapshot JSON: %v", err)
	}
	if _, err := uuid.Parse(parsed.Body.ID); err != nil {
		t.Fatalf("expected body id uuid, got %q (%v)", parsed.Body.ID, err)
	}
	if !strings.Contains(parsed.Body.Email, "@") {
		t.Fatalf("expected email-like body field, got %q", parsed.Body.Email)
	}
	if parsed.Body.ID == pathID {
		t.Fatalf("expected independent uuids per occurrence, got duplicate %q", parsed.Body.ID)
	}
}

func TestBuildHTTPRequestUsesRequestVariablesAsDefaults(t *testing.T) {
	t.Parallel()

	_, snapshot, err := buildHTTPRequest(context.Background(), RequestDefinition{
		Method: "GET",
		Path:   "/api/v1/users/{{id}}",
		Query:  map[string]string{"tenant": "{{tenant}}"},
		Variables: map[string]any{
			"id":     "42",
			"tenant": "saved",
		},
	}, project.Environment{BaseURL: "https://example.test"}, map[string]string{
		"tenant": "override",
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	got := string(snapshot)
	if !strings.Contains(got, "https://example.test/api/v1/users/42?tenant=override") {
		t.Fatalf("expected request variables to render URL with runtime overrides, got %s", got)
	}
}

func TestCollectInlineSQLReferencesScansRequestRecursively(t *testing.T) {
	t.Parallel()

	refs, err := collectInlineSQLReferences(RequestDefinition{
		Path:    "/api/v1/users/{{sql.userSeed.id}}",
		Query:   map[string]string{"tenant": "{{sql.tenantSeed.code}}"},
		Headers: map[string]string{"X-User": "{{sql.userSeed[status=1].id}}"},
		Variables: map[string]any{
			"createdBy": "{{sql.actorSeed.id}}",
		},
		Body: map[string]any{
			"user": map[string]any{
				"id":   "{{sql.userSeed.id}}",
				"tags": []any{"role-{{sql.roleSeed[name=admin].id}}"},
			},
		},
	})
	if err != nil {
		t.Fatalf("collect inline sql references: %v", err)
	}
	expressions := map[string]bool{}
	for _, ref := range refs {
		expressions[ref.Expression] = true
	}
	for _, want := range []string{
		"sql.userSeed.id",
		"sql.tenantSeed.code",
		"sql.userSeed[status=1].id",
		"sql.actorSeed.id",
		"sql.roleSeed[name=admin].id",
	} {
		if !expressions[want] {
			t.Fatalf("expected inline reference %q in %#v", want, refs)
		}
	}
}

func TestCollectInlineSQLReferencesScansRunVariables(t *testing.T) {
	t.Parallel()

	refs, err := collectInlineSQLReferencesFromVariables(map[string]any{
		"id": "{{sql.userSeed.id}}",
		"payload": map[string]any{
			"role": "{{sql.roleSeed[name=admin].id}}",
		},
	})
	if err != nil {
		t.Fatalf("collect inline sql references from variables: %v", err)
	}
	expressions := map[string]bool{}
	for _, ref := range refs {
		expressions[ref.Expression] = true
	}
	for _, want := range []string{"sql.userSeed.id", "sql.roleSeed[name=admin].id"} {
		if !expressions[want] {
			t.Fatalf("expected variable inline reference %q in %#v", want, refs)
		}
	}
}

func TestApplyInlineSQLReferencesDoesNotRequireCaseBinding(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	serviceID := uuid.New()
	params := &fakeParameterSourceService{
		inlineVars: map[string]string{"sql.userSeed.id": "42"},
		inlineSnapshots: []paramsource.ExecutionSnapshot{
			{SourceName: "user seed", Result: map[string]string{"id": "42"}},
		},
	}
	service := &Service{params: params}
	variants, err := service.applyInlineSQLReferences(
		context.Background(),
		&testcase.TestCase{ProjectID: projectID, ServiceID: serviceID},
		[]paramsource.InlineReference{{Expression: "sql.userSeed.id", SourceKey: "userSeed", Column: "id"}},
		[]caseRunVariant{{Variables: map[string]string{"tenant": "acme"}}},
	)
	if err != nil {
		t.Fatalf("apply inline sql references: %v", err)
	}
	if params.projectID != projectID || params.serviceID != serviceID {
		t.Fatalf("unexpected inline execution scope")
	}
	if variants[0].Variables["sql.userSeed.id"] != "42" {
		t.Fatalf("expected inline variable to be injected, got %#v", variants[0].Variables)
	}
	if len(variants[0].ParameterSourceSnapshots) != 1 {
		t.Fatalf("expected inline execution snapshot, got %#v", variants[0].ParameterSourceSnapshots)
	}
}

func TestInjectStepRefsResolvesPathsFromStepOutputs(t *testing.T) {
	t.Parallel()

	stepOutputs := map[int]any{
		1: map[string]any{
			"status": float64(200),
			"body": map[string]any{
				"data": map[string]any{"token": "abc123"},
			},
		},
		2: map[string]any{
			"firstRow": map[string]any{"user_id": "42"},
			"rows":     []any{map[string]any{"name": "Alice"}},
		},
	}

	vars := map[string]string{}
	injectStepRefs("Bearer {{$steps[1].body.data.token}}", stepOutputs, vars)
	if vars["$steps[1].body.data.token"] != "abc123" {
		t.Fatalf("expected token resolved, got %#v", vars)
	}

	injectStepRefs("{{$steps[1].status}}", stepOutputs, vars)
	if vars["$steps[1].status"] != "200" {
		t.Fatalf("expected status resolved, got %#v", vars)
	}

	injectStepRefs("{{$steps[2].firstRow.user_id}}", stepOutputs, vars)
	if vars["$steps[2].firstRow.user_id"] != "42" {
		t.Fatalf("expected firstRow resolved, got %#v", vars)
	}

	injectStepRefs("{{$steps[2].rows[0].name}}", stepOutputs, vars)
	if vars["$steps[2].rows[0].name"] != "Alice" {
		t.Fatalf("expected row field resolved, got %#v", vars)
	}

	// Missing step ref should not panic and should not add a key.
	before := len(vars)
	injectStepRefs("{{$steps[99].body}}", stepOutputs, vars)
	if len(vars) != before {
		t.Fatalf("expected missing step ref to be skipped, vars: %#v", vars)
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

type fakeParameterSourceService struct {
	projectID       uuid.UUID
	serviceID       uuid.UUID
	inlineVars      map[string]string
	inlineSnapshots []paramsource.ExecutionSnapshot
	inlineErr       error
}

func (f *fakeParameterSourceService) ExecuteInlineReferences(_ context.Context, projectID, serviceID uuid.UUID, _ []paramsource.InlineReference, _ map[string]string) (map[string]string, []paramsource.ExecutionSnapshot, error) {
	f.projectID = projectID
	f.serviceID = serviceID
	return f.inlineVars, f.inlineSnapshots, f.inlineErr
}

func (f *fakeParameterSourceService) ExecuteSQLStep(_ context.Context, projectID, serviceID, environmentID uuid.UUID, _ paramsource.SQLStepInput, _ map[string]string) (paramsource.SQLStepResult, error) {
	f.projectID = projectID
	f.serviceID = serviceID
	return paramsource.SQLStepResult{}, nil
}
