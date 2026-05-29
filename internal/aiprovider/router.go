package aiprovider

// router.go implements the rule-based "Tool Router" stage. Given the
// PlannerOutput (coarse intent + domains) and the page context, it selects
// a concrete tool package to mount for the conversation's first hop. It is
// a PURE function with no IO so it is trivially unit-testable and can run
// in shadow mode without side effects.
//
// The meta tools (find_tools / describe_tools) are intentionally NOT part
// of the Router output: the conversation loop mounts them unconditionally
// on every hop. The Router only decides the domain packages.

import (
	"encoding/json"
	"sort"
	"strings"

	"autotest/internal/aiconfig"
	"autotest/internal/aitools"
)

// allDomains is the closed set of coarse tool domains. Mirrors the Domain
// values stamped on builtin tools (see internal/aitools/builtin).
var allDomains = []string{
	"meta", "cases", "scenarios", "mock", "mockset",
	"testdata", "paramsource", "scripts", "runs", "spec",
}

func knownDomain(d string) bool {
	for _, x := range allDomains {
		if x == d {
			return true
		}
	}
	return false
}

// coreDiscoveryTools are always unioned into the active package: they are
// the cheap, high-value discovery entry points the model needs in nearly
// every conversation. Only the ones actually present in the catalog are
// mounted.
var coreDiscoveryTools = []string{
	"list_services", "get_endpoint", "get_case", "get_scenario",
}

// RouterOutput is the Router's decision. ActiveTools is the concrete tool
// name set (sorted, deduped) to mount for the first hop. It is persisted
// into ai_routing_logs.router_output for shadow comparison.
type RouterOutput struct {
	ActiveTools     []string `json:"activeTools"`
	Confidence      float64  `json:"confidence"`
	FallbackDomains []string `json:"fallbackDomains"`
	Reason          string   `json:"reason"`
}

// Router selects a tool package from a plan. Stateless; safe to share.
type Router struct{}

// NewRouter constructs a Router.
func NewRouter() *Router { return &Router{} }

// Route applies the rule-based selection. cat may be nil (e.g. in a unit
// test exercising only the confidence heuristic) in which case the tool
// name lists come back empty but the confidence/fallback decision still
// holds.
func (Router) Route(plan PlannerOutput, pageContext json.RawMessage, cat *aitools.Catalog) RouterOutput {
	confidence := scoreConfidence(plan)

	// Domains to include: planner domains + any inferred from the page.
	domainSet := map[string]struct{}{}
	for _, d := range plan.Domains {
		if knownDomain(d) {
			domainSet[d] = struct{}{}
		}
	}
	for _, d := range inferDomainsFromPage(pageContext) {
		domainSet[d] = struct{}{}
	}

	var fallback []string
	reasons := []string{}
	if len(plan.Domains) == 0 {
		reasons = append(reasons, "planner 未给出 domains")
	}
	if len(plan.Ambiguities) > 0 {
		reasons = append(reasons, "存在歧义点")
	}

	// Low confidence → widen to all domains as a safety net.
	if confidence < aiconfig.RouterConfidenceThreshold() {
		fallback = appendMissingDomains(domainSet, allDomains)
		reasons = append(reasons, "confidence 低于阈值，纳入全部域兜底")
	}

	names := map[string]struct{}{}
	if cat != nil {
		for _, t := range coreDiscoveryTools {
			if _, ok := cat.Get(t); ok {
				names[t] = struct{}{}
			}
		}
		for d := range domainSet {
			for _, n := range cat.ByDomain(d) {
				names[n] = struct{}{}
			}
		}
	}

	out := RouterOutput{
		ActiveTools:     sortedKeys(names),
		Confidence:      confidence,
		FallbackDomains: fallback,
		Reason:          strings.Join(reasons, "；"),
	}
	if out.Reason == "" {
		out.Reason = "plan 明确，按域精确选包"
	}
	return out
}

// scoreConfidence is the simple heuristic from the plan: confident when
// the planner produced domains with no ambiguities; uncertain otherwise.
func scoreConfidence(plan PlannerOutput) float64 {
	switch {
	case len(plan.Domains) == 0:
		return 0.3
	case len(plan.Ambiguities) > 0:
		return 0.6
	default:
		return 0.9
	}
}

// inferDomainsFromPage maps the frontend page snapshot to likely domains.
// It is best-effort and never errors: malformed JSON yields no inference.
func inferDomainsFromPage(pageContext json.RawMessage) []string {
	raw := strings.TrimSpace(string(pageContext))
	if raw == "" || raw == "null" || raw == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	found := map[string]struct{}{}
	path := strings.ToLower(asString(m["path"]))
	addIf := func(cond bool, domain string) {
		if cond {
			found[domain] = struct{}{}
		}
	}
	addIf(asString(m["scenarioId"]) != "" || strings.Contains(path, "scenario"), "scenarios")
	addIf(asString(m["caseId"]) != "" || asString(m["testCaseId"]) != "" || strings.Contains(path, "/case") || strings.Contains(path, "api-management") || strings.Contains(path, "endpoint"), "cases")
	addIf(asString(m["serviceId"]) != "" || strings.Contains(path, "service"), "meta")
	addIf(strings.Contains(path, "mock-set") || strings.Contains(path, "mockset") || strings.Contains(path, "value-set"), "mockset")
	addIf((strings.Contains(path, "mock") && !strings.Contains(path, "mock-set") && !strings.Contains(path, "mockset")), "mock")
	addIf(strings.Contains(path, "test-data") || strings.Contains(path, "testdata"), "testdata")
	addIf(strings.Contains(path, "data-source") || strings.Contains(path, "param") || strings.Contains(path, "sql"), "paramsource")
	addIf(strings.Contains(path, "script"), "scripts")
	addIf(strings.Contains(path, "run") || strings.Contains(path, "report"), "runs")
	addIf(strings.Contains(path, "spec") || strings.Contains(path, "swagger") || strings.Contains(path, "openapi"), "spec")
	return sortedKeys(found)
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func appendMissingDomains(have map[string]struct{}, all []string) []string {
	out := []string{}
	for _, d := range all {
		if _, ok := have[d]; !ok {
			out = append(out, d)
		}
		have[d] = struct{}{}
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// marshalRouterOutput serialises a router decision for ai_routing_logs.
func marshalRouterOutput(out RouterOutput) json.RawMessage {
	body, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return body
}
