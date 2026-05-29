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

func mustSchema(s string) []byte {
	return []byte(s)
}
