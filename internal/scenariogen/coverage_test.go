package scenariogen

import (
	"testing"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

func TestPlanCoverageUsesDepgraphLoginFirst(t *testing.T) {
	t.Parallel()
	eps := []spec.Endpoint{
		{ID: uuid.New(), Method: "GET", Path: "/healthz", Summary: "health", Tags: []string{"public"}},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/me", Summary: "me", Tags: []string{"users"}, RequestSchema: mustSchema(t, `{"security":[{"BearerAuth":[]}]}`)},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/auth/login", Summary: "login", Tags: []string{"users"}},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/admin/auth/login", Summary: "admin login", Tags: []string{"admin"}, RequestSchema: mustSchema(t, `{}`)},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/admin/stats", Summary: "stats", Tags: []string{"admin"}, RequestSchema: mustSchema(t, `{"security":[{"AdminBearerAuth":[]}]}`)},
	}
	caseMap := map[uuid.UUID]uuid.UUID{}
	for _, ep := range eps {
		caseMap[ep.ID] = uuid.New()
	}
	plans := planCoverage(eps, buildPlanContext(caseMap, nil, nil))
	if len(plans) < 2 {
		t.Fatalf("expected at least 2 scenarios, got %d", len(plans))
	}
	var admin *ScenarioPlan
	for i := range plans {
		if plans[i].Name == "admin 接口覆盖" {
			admin = &plans[i]
			break
		}
	}
	if admin == nil {
		t.Fatal("admin scenario missing")
	}
	if !isLoginEndpoint(admin.Steps[0].Endpoint) {
		t.Fatalf("first admin step should be login, got %s %s", admin.Steps[0].Endpoint.Method, admin.Steps[0].Endpoint.Path)
	}
	if len(admin.Steps[0].RequestOverride) == 0 {
		t.Fatal("login step should have body override")
	}
	if len(admin.Steps[1].RequestOverride) == 0 {
		t.Fatal("authenticated step should have bearer override")
	}
}


func mustSchema(t *testing.T, s string) []byte {
	t.Helper()
	return []byte(s)
}
