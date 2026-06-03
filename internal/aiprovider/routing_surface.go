package aiprovider

// routing_surface.go holds the on-demand tool surface plumbing that the
// assistant loop uses to narrow the tool definitions sent to the LLM on a
// per-hop basis. It is deliberately separated from assistant.go so the
// (large) streaming loop stays readable.
//
// Key idea: cfg.Tools (the execution map) stays FULL so any tool the model
// somehow calls can still run; only the *definitions shipped to the LLM*
// are narrowed. The narrowing decision is governed by the gray-rollout
// RoutingMode.

import (
	"encoding/json"
	"sort"
	"strings"

	"autotest/internal/aiconfig"
	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
)

// RoutingMode is the gray-rollout switch for the on-demand tool surface.
// It controls whether (and how) the per-hop tool definitions are narrowed.
// Planner/Router are ALWAYS computed and logged when routing is configured
// (shadow telemetry); the mode only changes what the LLM actually sees.
type RoutingMode string

const (
	// RoutingModeShadow (default, zero-risk): always ship the full tool
	// set to the LLM, but still compute Planner/Router and write
	// ai_routing_logs for offline comparison.
	RoutingModeShadow RoutingMode = "shadow"
	// RoutingModeDynamicFallback (Phase 1): when Router confidence is high
	// ship the narrowed package; otherwise fall back to the full set.
	RoutingModeDynamicFallback RoutingMode = "dynamic_fallback"
	// RoutingModeDynamic (Phase 2): always ship the narrowed package, but
	// fall back to full when the narrowed package would carry no
	// domain tools (router selected nothing usable).
	RoutingModeDynamic RoutingMode = "dynamic"
	// RoutingModeMetaOnly (Phase 3): ship only CoreDiscovery + meta tools +
	// already-described tools; the model must discover everything else via
	// find_tools / describe_tools.
	RoutingModeMetaOnly RoutingMode = "meta_only"
)

// ParseRoutingMode normalises an env/config string into a RoutingMode,
// defaulting to the zero-risk shadow mode for unknown/empty values.
func ParseRoutingMode(s string) RoutingMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(RoutingModeDynamicFallback):
		return RoutingModeDynamicFallback
	case string(RoutingModeDynamic):
		return RoutingModeDynamic
	case string(RoutingModeMetaOnly):
		return RoutingModeMetaOnly
	case string(RoutingModeShadow), "":
		return RoutingModeShadow
	default:
		return RoutingModeShadow
	}
}

// hopLog captures one hop's tool exposure for offline metrics:
//   - Offered: names actually sent to the LLM this hop.
//   - Called: names the model actually invoked this hop.
//   - Missing: names called but not offered (should be empty; tracked for
//     mis-route detection).
//   - InputTokens: prompt tokens consumed by this hop.
type hopLog struct {
	Hop          int      `json:"hop"`
	Offered      []string `json:"offered"`
	Called       []string `json:"called"`
	Missing      []string `json:"missing,omitempty"`
	InputTokens  int      `json:"inputTokens,omitempty"`
	Narrowed     bool     `json:"narrowed"`
}

// routingState accumulates everything needed to write one ai_routing_logs
// row at the end of a stream. It lives on streamConfig.
type routingState struct {
	Enabled       bool
	Mode          RoutingMode
	MessageSeq    int
	PlannerOutput json.RawMessage
	RouterOutput  json.RawMessage
	Confidence    float64
	Hops          []hopLog
	Outcome       string
}

// recordHop appends one hop's telemetry. offered is the exact name set sent
// to the LLM this hop; calls are the model's tool invocations.
func (rs *routingState) recordHop(hop int, offered []string, calls []client.ToolCall, inputTokens int, narrowed bool) {
	if rs == nil || !rs.Enabled {
		return
	}
	offeredSet := map[string]struct{}{}
	for _, n := range offered {
		offeredSet[n] = struct{}{}
	}
	called := make([]string, 0, len(calls))
	var missing []string
	for _, c := range calls {
		called = append(called, c.Name)
		if _, ok := offeredSet[c.Name]; !ok && !isProviderNativeTool(c.Name) {
			missing = append(missing, c.Name)
		}
	}
	rs.Hops = append(rs.Hops, hopLog{
		Hop:         hop,
		Offered:     offered,
		Called:      called,
		Missing:     missing,
		InputTokens: inputTokens,
		Narrowed:    narrowed,
	})
}

// setOutcome records the terminal outcome of a stream (success /
// pending_confirm / hop_exhausted / error) for ai_routing_logs.
func (cfg *streamConfig) setOutcome(outcome string) {
	if cfg == nil || cfg.Routing == nil {
		return
	}
	cfg.Routing.Outcome = outcome
}

// marshalPerHop serialises the accumulated per-hop log for ai_routing_logs.
func (rs *routingState) marshalPerHop() json.RawMessage {
	if rs == nil || len(rs.Hops) == 0 {
		return nil
	}
	body, err := json.Marshal(rs.Hops)
	if err != nil {
		return nil
	}
	return body
}

// hopActiveNames returns the tool names to mount on the LLM wire for this
// hop, or nil meaning "ship the full set". cfg.ActiveTools is the Router's
// initial package; cfg.DescribedTools accumulates names the model expanded
// via describe_tools.
func (cfg *streamConfig) hopActiveNames() []string {
	if cfg.Catalog == nil || cfg.Routing == nil {
		return nil
	}
	switch cfg.Routing.Mode {
	case RoutingModeShadow, "":
		return nil // always full
	case RoutingModeDynamicFallback:
		if cfg.Routing.Confidence < aiconfig.RouterConfidenceThreshold() {
			return nil // low confidence → full fallback
		}
		return cfg.narrowedNames(true)
	case RoutingModeDynamic:
		names := cfg.narrowedNames(true)
		if !cfg.routerHasDomainTools() {
			return nil // nothing useful selected → full fallback
		}
		return names
	case RoutingModeMetaOnly:
		return cfg.narrowedNames(false) // core + meta + described only
	default:
		return nil
	}
}

// narrowedNames computes union(CoreDiscovery, [ActiveTools], DescribedTools,
// {find_tools, describe_tools, ask_question}) filtered to names present in cfg.Tools.
// includeRouter toggles whether the Router's ActiveTools are folded in
// (false for meta-only mode).
func (cfg *streamConfig) narrowedNames(includeRouter bool) []string {
	set := map[string]struct{}{}
	add := func(name string) {
		if _, ok := cfg.Tools[name]; ok {
			set[name] = struct{}{}
		}
	}
	for _, n := range coreDiscoveryTools {
		add(n)
	}
	add(aitools.FindToolsName)
	add(aitools.DescribeToolsName)
	add(aitools.AskQuestionName) // always available for interactive Q&A
	if includeRouter {
		for n := range cfg.ActiveTools {
			add(n)
		}
	}
	for n := range cfg.DescribedTools {
		add(n)
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// routerHasDomainTools reports whether the Router selected any tool beyond
// the always-on core/meta set. Used by dynamic mode to decide whether to
// fall back to the full set.
func (cfg *streamConfig) routerHasDomainTools() bool {
	for n := range cfg.ActiveTools {
		if n == aitools.FindToolsName || n == aitools.DescribeToolsName || n == aitools.AskQuestionName {
			continue
		}
		if isCoreDiscovery(n) {
			continue
		}
		if _, ok := cfg.Tools[n]; ok {
			return true
		}
	}
	return false
}

func isCoreDiscovery(name string) bool {
	for _, n := range coreDiscoveryTools {
		if n == name {
			return true
		}
	}
	return false
}

// hopToolDefs builds the tool definitions for this hop. It returns the
// definitions plus the offered name list (for telemetry) and whether the
// set was narrowed.
func (cfg *streamConfig) hopToolDefs() (defs []client.ToolDefinition, offered []string, narrowed bool) {
	names := cfg.hopActiveNames()
	if names == nil {
		// Full set: cfg.BaseOpts.Tools already merged web_search.
		offered = make([]string, 0, len(cfg.ToolDefs))
		for _, d := range cfg.ToolDefs {
			offered = append(offered, d.Name)
		}
		return cfg.BaseOpts.Tools, offered, false
	}
	tools := make([]aitools.Tool, 0, len(names))
	for _, n := range names {
		if t, ok := cfg.Tools[n]; ok {
			tools = append(tools, t)
		}
	}
	merged := mergeAssistantToolDefs(aitools.Describe(tools), cfg.Provider, cfg.WebSearchEnabled)
	offered = make([]string, 0, len(merged))
	for _, d := range merged {
		offered = append(offered, d.Name)
	}
	return merged, offered, true
}

// FirstHopToolNames returns the tool names that WOULD be offered to the LLM
// on the first hop given a Router decision. It mirrors the loop's
// narrowedNames(includeRouter=true) — CoreDiscovery ∪ Router.ActiveTools ∪
// {find_tools, describe_tools} — filtered to names known to the catalog
// (the two meta tools are always on). Exported so the offline eval harness
// can score routing without reconstructing the loop.
func FirstHopToolNames(routed RouterOutput, cat *aitools.Catalog) []string {
	set := map[string]struct{}{}
	known := func(n string) bool {
		if cat == nil {
			return false
		}
		_, ok := cat.Get(n)
		return ok
	}
	for _, n := range coreDiscoveryTools {
		if known(n) {
			set[n] = struct{}{}
		}
	}
	set[aitools.FindToolsName] = struct{}{}
	set[aitools.DescribeToolsName] = struct{}{}
	for _, n := range routed.ActiveTools {
		if known(n) {
			set[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// mergeDescribedNames folds names recorded by describe_tools (via the
// per-stream collector) into cfg.DescribedTools so the next hop mounts them.
func (cfg *streamConfig) mergeDescribedNames(names []string) {
	if len(names) == 0 {
		return
	}
	if cfg.DescribedTools == nil {
		cfg.DescribedTools = map[string]bool{}
	}
	for _, n := range names {
		if _, ok := cfg.Tools[n]; ok {
			cfg.DescribedTools[n] = true
		}
	}
}
