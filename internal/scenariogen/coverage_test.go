package scenariogen

import (
	"testing"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

func TestPlanE2ECoverageOrdersLoginFirst(t *testing.T) {
	t.Parallel()
	eps := []spec.Endpoint{
		{ID: uuid.New(), Method: "GET", Path: "/healthz", Summary: "health"},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/me", Summary: "me", RequestSchema: mustSchema(t, `{"security":[{"BearerAuth":[]}]}`)},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/auth/login", Summary: "login"},
		{ID: uuid.New(), Method: "POST", Path: "/api/v1/admin/auth/login", Summary: "admin login"},
		{ID: uuid.New(), Method: "GET", Path: "/api/v1/admin/stats", Summary: "stats", RequestSchema: mustSchema(t, `{"security":[{"AdminBearerAuth":[]}]}`)},
	}
	caseMap := map[uuid.UUID]uuid.UUID{}
	for _, ep := range eps {
		caseMap[ep.ID] = uuid.New()
	}
	plans := planE2ECoverage(eps, caseMap)
	if len(plans) < 3 {
		t.Fatalf("expected at least 3 scenarios, got %d", len(plans))
	}
	var admin *ScenarioPlan
	for i := range plans {
		if plans[i].Name == "管理员 API 全流程" {
			admin = &plans[i]
			break
		}
	}
	if admin == nil {
		t.Fatal("admin scenario missing")
	}
	if admin.Steps[0].Endpoint.Path != "/api/v1/admin/auth/login" {
		t.Fatalf("first admin step should be login, got %s", admin.Steps[0].Endpoint.Path)
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
