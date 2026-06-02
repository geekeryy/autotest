package depgraph

import (
	"testing"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

func TestBuildLoginAndBearerOrdering(t *testing.T) {
	t.Parallel()
	eps := []spec.Endpoint{
		{ID: uuid.New(), Method: "GET", Path: "/healthz", Summary: "health"},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/auth/login", Summary: "login"},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/me", Summary: "me", RequestSchema: mustSchema(`{"security":[{"BearerAuth":[]}]}`)},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/users/{id}", Summary: "user", RequestSchema: mustSchema(`{"security":[{"BearerAuth":[]}]}`)},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/users", Summary: "create", RequestSchema: mustSchema(`{"security":[{"BearerAuth":[]}]}`)},
	}
	g := Build(eps)
	if len(g.LoginIndices) != 1 {
		t.Fatalf("expected 1 login, got %d", len(g.LoginIndices))
	}
	if g.Endpoints[g.LoginIndices[0]].Path != "/api/v1/auth/login" {
		t.Fatalf("unexpected login path %s", g.Endpoints[g.LoginIndices[0]].Path)
	}
	order := map[int]int{}
	for i, idx := range g.TopoOrder {
		order[idx] = i
	}
	loginPos := order[g.LoginIndices[0]]
	mePos := -1
	for i, ep := range eps {
		if ep.Path == "/api/v1/me" {
			mePos = order[i]
		}
	}
	if mePos <= loginPos {
		t.Fatalf("login should precede /me: login=%d me=%d", loginPos, mePos)
	}
}

func TestBuildProducerConsumerMapping(t *testing.T) {
	t.Parallel()
	eps := []spec.Endpoint{
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/items", Summary: "create", ResponseSchema: mustSchema(`{"properties":{"id":{"type":"string"}}}`)},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/items/{id}", Summary: "get"},
	}
	g := Build(eps)
	if len(g.Mappings) == 0 {
		t.Fatal("expected producer-consumer mapping")
	}
	found := false
	for _, m := range g.Mappings {
		if m.ConsumerTarget == "id" && m.ConsumerKind == "path_param" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing id path_param mapping: %+v", g.Mappings)
	}
}

func TestTokenResponsePath(t *testing.T) {
	t.Parallel()
	flat := spec.Endpoint{ResponseSchema: mustSchema(`{"properties":{"token":{"type":"string"}}}`)}
	if p := TokenResponsePath(flat); p != "response.body.token" {
		t.Fatalf("flat token path = %q, want response.body.token", p)
	}
	wrapped := spec.Endpoint{ResponseSchema: mustSchema(`{"properties":{"data":{"properties":{"token":{"type":"string"}}}}}`)}
	if p := TokenResponsePath(wrapped); p != "response.body.data.token" {
		t.Fatalf("wrapped token path = %q, want response.body.data.token", p)
	}
}

func TestResourceGroupsByTag(t *testing.T) {
	t.Parallel()
	eps := []spec.Endpoint{
		{ID: uuid.New(), Method: "GET", Path: "/a", Tags: []string{"billing"}},
		{ID: uuid.New(), Method: "GET", Path: "/b", Tags: []string{"users"}},
	}
	g := Build(eps)
	if len(g.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(g.Groups))
	}
}

func TestIsLoginEndpointExtended(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		ep     spec.Endpoint
		expect bool
	}{
		{"classic login", spec.Endpoint{Method: "POST", Path: "/api/v1/auth/login"}, true},
		{"token endpoint", spec.Endpoint{Method: "POST", Path: "/api/v1/oauth/token"}, true},
		{"signin", spec.Endpoint{Method: "POST", Path: "/api/v1/signin"}, true},
		{"sign-in", spec.Endpoint{Method: "POST", Path: "/api/v1/sign-in"}, true},
		{"oauth authorize", spec.Endpoint{Method: "POST", Path: "/api/v1/oauth/authorize"}, true},
		{"sso login", spec.Endpoint{Method: "POST", Path: "/api/v1/sso/login"}, true},
		{"authenticate", spec.Endpoint{Method: "POST", Path: "/api/v1/authenticate"}, true},
		{"operationId login", spec.Endpoint{Method: "POST", Path: "/api/v1/auth", OperationID: "loginUser"}, true},
		{"summary login", spec.Endpoint{Method: "POST", Path: "/api/v1/auth", Summary: "用户登录"}, true},
		{"body credentials", spec.Endpoint{Method: "POST", Path: "/api/v1/session", RequestSchema: mustSchema(`{"body":{"properties":{"username":{"type":"string"},"password":{"type":"string"}}}}`)}, true},
		{"GET not login", spec.Endpoint{Method: "GET", Path: "/api/v1/login"}, false},
		{"unrelated POST", spec.Endpoint{Method: "POST", Path: "/api/v1/users"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLoginEndpoint(tc.ep)
			if got != tc.expect {
				t.Fatalf("IsLoginEndpoint(%s %s) = %v, want %v", tc.ep.Method, tc.ep.Path, got, tc.expect)
			}
		})
	}
}

func TestTokenResponsePathExtended(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		schema string
		expect string
	}{
		{"flat token", `{"properties":{"token":{"type":"string"}}}`, "response.body.token"},
		{"flat access_token", `{"properties":{"access_token":{"type":"string"}}}`, "response.body.access_token"},
		{"flat jwt", `{"properties":{"jwt":{"type":"string"}}}`, "response.body.jwt"},
		{"wrapped access_token", `{"properties":{"data":{"properties":{"access_token":{"type":"string"}}}}}`, "response.body.data.access_token"},
		{"prefers access_token over token", `{"properties":{"token":{"type":"string"},"access_token":{"type":"string"}}}`, "response.body.access_token"},
		{"no token field", `{"properties":{"name":{"type":"string"}}}`, "response.body.token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep := spec.Endpoint{ResponseSchema: mustSchema(tc.schema)}
			got := TokenResponsePath(ep)
			if got != tc.expect {
				t.Fatalf("TokenResponsePath() = %q, want %q", got, tc.expect)
			}
		})
	}
}

func mustSchema(s string) []byte {
	return []byte(s)
}
