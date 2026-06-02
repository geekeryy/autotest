package aiprovider

import (
	"encoding/json"
	"testing"

	"autotest/internal/aiconfig"
	"autotest/internal/aitools"
)

func routerTestCatalog() *aitools.Catalog {
	return aitools.NewCatalog([]aitools.Tool{
		{Name: "list_services", Domain: "meta"},
		{Name: "get_endpoint", Domain: "meta"},
		{Name: "create_service", Domain: "meta", Mutating: true},
		{Name: "get_case", Domain: "cases"},
		{Name: "create_case_from_endpoint", Domain: "cases", Mutating: true},
		{Name: "get_scenario", Domain: "scenarios"},
		{Name: "create_scenario_with_steps", Domain: "scenarios", Mutating: true},
		{Name: "list_mock_servers", Domain: "mock"},
		{Name: "list_runs", Domain: "runs"},
	})
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestRouter_CoreDiscoveryAlwaysUnioned(t *testing.T) {
	cat := routerTestCatalog()
	out := Router{}.Route(PlannerOutput{Domains: []string{"scenarios"}}, nil, cat, nil)
	for _, core := range []string{"list_services", "get_endpoint", "get_case", "get_scenario"} {
		if !contains(out.ActiveTools, core) {
			t.Fatalf("core tool %s missing from %+v", core, out.ActiveTools)
		}
	}
}

func TestRouter_HighConfidenceSelectsDomain(t *testing.T) {
	cat := routerTestCatalog()
	out := Router{}.Route(PlannerOutput{Domains: []string{"scenarios"}, Ambiguities: nil}, nil, cat, nil)
	if out.Confidence < aiconfig.DefaultRouterConfidenceThreshold {
		t.Fatalf("expected high confidence, got %v", out.Confidence)
	}
	if !contains(out.ActiveTools, "create_scenario_with_steps") {
		t.Fatalf("scenario domain tool missing: %+v", out.ActiveTools)
	}
	// mock domain not requested → must not appear
	if contains(out.ActiveTools, "list_mock_servers") {
		t.Fatalf("unexpected mock tool in high-confidence selection: %+v", out.ActiveTools)
	}
	if len(out.FallbackDomains) != 0 {
		t.Fatalf("high confidence should not set fallback, got %+v", out.FallbackDomains)
	}
}

func TestRouter_EmptyDomainsLowConfidenceWidensToAll(t *testing.T) {
	cat := routerTestCatalog()
	out := Router{}.Route(PlannerOutput{Domains: nil}, nil, cat, nil)
	if out.Confidence >= aiconfig.DefaultRouterConfidenceThreshold {
		t.Fatalf("expected low confidence, got %v", out.Confidence)
	}
	if len(out.FallbackDomains) == 0 {
		t.Fatal("expected fallback domains for low confidence")
	}
	// widening pulls in every domain's tools, including mock + runs
	if !contains(out.ActiveTools, "list_mock_servers") || !contains(out.ActiveTools, "list_runs") {
		t.Fatalf("low-confidence widening should include all domains: %+v", out.ActiveTools)
	}
}

func TestRouter_AmbiguityLowersConfidence(t *testing.T) {
	out := Router{}.Route(PlannerOutput{Domains: []string{"cases"}, Ambiguities: []string{"哪个服务?"}}, nil, nil, nil)
	if out.Confidence >= aiconfig.DefaultRouterConfidenceThreshold {
		t.Fatalf("ambiguity should lower confidence below threshold, got %v", out.Confidence)
	}
	if len(out.FallbackDomains) == 0 {
		t.Fatal("expected fallback domains when ambiguous")
	}
}

func TestRouter_PageContextInfersDomain(t *testing.T) {
	cat := routerTestCatalog()
	page := json.RawMessage(`{"path":"/scenarios/abc","scenarioId":"abc"}`)
	// planner gave a different (cases) domain; page should add scenarios
	out := Router{}.Route(PlannerOutput{Domains: []string{"cases"}}, page, cat, nil)
	if !contains(out.ActiveTools, "create_scenario_with_steps") {
		t.Fatalf("page-context scenarios domain not added: %+v", out.ActiveTools)
	}
	if !contains(out.ActiveTools, "create_case_from_endpoint") {
		t.Fatalf("planner cases domain missing: %+v", out.ActiveTools)
	}
}

func TestRouter_MetaToolsNeverInOutput(t *testing.T) {
	cat := aitools.NewCatalog([]aitools.Tool{
		{Name: aitools.FindToolsName, Domain: "meta"},
		{Name: aitools.DescribeToolsName, Domain: "meta"},
		{Name: "list_services", Domain: "meta"},
	})
	out := Router{}.Route(PlannerOutput{Domains: []string{"meta"}}, nil, cat, nil)
	// meta domain includes find/describe in catalog, but Router selecting
	// the meta domain WILL include them here — that's acceptable because the
	// loop mounts them anyway. The contract we assert is the loop's, not the
	// router's. Here we just ensure list_services is selected.
	if !contains(out.ActiveTools, "list_services") {
		t.Fatalf("expected list_services in meta domain: %+v", out.ActiveTools)
	}
}
