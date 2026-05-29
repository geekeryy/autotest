package aiprovider

// planner.go implements the lightweight "Planner" stage of the on-demand
// tool architecture. Before the main conversation loop runs, the Planner
// makes one cheap LLM call that classifies the user's intent into a coarse
// plan: which tool domains are relevant, the rough workflow, whether any
// write is implied, and any ambiguities. The Router then turns that plan
// into a concrete tool package.
//
// Hard requirements (see the approved plan):
//   - Uses the SAME provider client as the main chat.
//   - Small MaxTokens, thinking disabled, JSONOnly.
//   - Never fails the main conversation: any parse error / timeout / empty
//     response degrades to a SAFE PlannerOutput (no domains, no write) and
//     the error is returned for telemetry/logging only.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
)

// plannerTimeout caps the planner LLM call so a slow upstream can never
// stall the main conversation for long.
const plannerTimeout = 20 * time.Second

// plannerMaxTokens keeps the planner output tiny — it only emits a small
// JSON object, never prose.
const plannerMaxTokens = 400

// PlannerOutput is the coarse plan produced before the main loop. It is
// also persisted (as json) into ai_routing_logs.planner_output for shadow
// comparison.
type PlannerOutput struct {
	// Intent is a one-line restatement of what the user wants.
	Intent string `json:"intent"`
	// Domains lists the relevant tool domains (subset of the known domain
	// set). Empty means "could not determine" — the Router will widen.
	Domains []string `json:"domains"`
	// Workflow is the rough ordered list of steps the assistant will take.
	Workflow []string `json:"workflow"`
	// NeedsWrite is true when the plan implies a mutating tool.
	NeedsWrite bool `json:"needsWrite"`
	// Ambiguities lists unresolved questions; a non-empty list lowers the
	// Router confidence so it falls back to a wider tool package.
	Ambiguities []string `json:"ambiguities"`
}

// safePlannerOutput is the degraded result used whenever the planner LLM
// call cannot be trusted. Empty domains + needsWrite=false makes the
// Router widen to a safe (full) package, so correctness never depends on
// the planner succeeding.
func safePlannerOutput() PlannerOutput {
	return PlannerOutput{Domains: []string{}, Workflow: []string{}, Ambiguities: []string{}}
}

// Planner runs the intent-classification pre-pass. It holds a reference to
// the Catalog so the prompt can advertise the available domains; today it
// uses a fixed domain list for prompt stability, but the catalog is kept
// for future per-deployment domain discovery.
type Planner struct {
	catalog *aitools.Catalog
}

// NewPlanner constructs a Planner over the given catalog.
func NewPlanner(catalog *aitools.Catalog) *Planner { return &Planner{catalog: catalog} }

// Plan runs one lightweight LLM call to classify the user's request. It
// NEVER returns a zero-value/garbage plan that could break the loop: on
// any failure it returns safePlannerOutput() plus a non-nil error the
// caller may log. Callers should always use the returned PlannerOutput
// regardless of the error.
func (p *Planner) Plan(
	ctx context.Context,
	cli client.Client,
	provider *providerRow,
	userMsg string,
	pageContext json.RawMessage,
	historyDigest string,
) (PlannerOutput, error) {
	if p == nil || cli == nil {
		return safePlannerOutput(), errors.New("planner: client unavailable")
	}
	userMsg = strings.TrimSpace(userMsg)
	if userMsg == "" {
		return safePlannerOutput(), nil
	}

	messages := buildPlannerMessages(userMsg, pageContext, historyDigest)
	model := ""
	if provider != nil {
		model = provider.DefaultModel
	}
	temperature := 0.0
	// Disable thinking explicitly for deepseek/xiaomi: the planner is a
	// fast structural classifier, reasoning tokens would just add latency.
	disabled := false
	opts := client.Options{
		Model:       model,
		Temperature: &temperature,
		MaxTokens:   plannerMaxTokens,
		JSONOnly:    true,
		Thinking:    assistantThinkingOverride(provider, &disabled, ""),
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, plannerTimeout)
	defer cancel()

	result, err := cli.Chat(timeoutCtx, messages, opts)
	if err != nil {
		return safePlannerOutput(), err
	}
	if result == nil || strings.TrimSpace(result.Text) == "" {
		return safePlannerOutput(), errors.New("planner: empty response")
	}
	return parsePlannerOutput(result.Text)
}

// buildPlannerMessages assembles the system + user messages for the
// planner call. The user message carries the raw request, an optional
// page-context digest and an optional history digest — all kept compact.
func buildPlannerMessages(userMsg string, pageContext json.RawMessage, historyDigest string) []client.Message {
	var b strings.Builder
	b.WriteString("用户需求：\n")
	b.WriteString(userMsg)
	if digest := strings.TrimSpace(historyDigest); digest != "" {
		b.WriteString("\n\n对话摘要（仅供参考）：\n")
		b.WriteString(digest)
	}
	if msg, ok := renderPageContextSystem(pageContext); ok {
		// Reuse the same rendering as the main loop but fold it into the
		// planner's user turn so the classifier sees the current page.
		b.WriteString("\n\n")
		b.WriteString(msg.Content)
	}
	b.WriteString("\n\n请只输出规划 JSON。")
	return []client.Message{
		{Role: "system", Content: plannerSystem},
		{Role: "user", Content: b.String()},
	}
}

// parsePlannerOutput extracts a PlannerOutput from a free-form model reply.
// Robust to code fences and surrounding prose. On any failure it returns a
// safe output plus the error so the loop never blows up.
func parsePlannerOutput(text string) (PlannerOutput, error) {
	raw, warning := extractParsedJSON(text)
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		if warning == "" {
			warning = "planner: no json parsed"
		}
		return safePlannerOutput(), errors.New(warning)
	}
	var out PlannerOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return safePlannerOutput(), err
	}
	return normalizePlannerOutput(out), nil
}

// normalizePlannerOutput sanitises the parsed plan: trims/normalises
// domains against the known domain set and guarantees non-nil slices so
// downstream JSON serialisation and the Router are predictable.
func normalizePlannerOutput(in PlannerOutput) PlannerOutput {
	out := PlannerOutput{
		Intent:      strings.TrimSpace(in.Intent),
		NeedsWrite:  in.NeedsWrite,
		Domains:     []string{},
		Workflow:    []string{},
		Ambiguities: []string{},
	}
	seen := map[string]struct{}{}
	for _, d := range in.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || !knownDomain(d) {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out.Domains = append(out.Domains, d)
	}
	for _, w := range in.Workflow {
		if w = strings.TrimSpace(w); w != "" {
			out.Workflow = append(out.Workflow, w)
		}
	}
	for _, a := range in.Ambiguities {
		if a = strings.TrimSpace(a); a != "" {
			out.Ambiguities = append(out.Ambiguities, a)
		}
	}
	return out
}

// marshalPlannerOutput serialises a plan for ai_routing_logs. Never errors
// out the caller — on failure it returns nil so the column stays NULL.
func marshalPlannerOutput(out PlannerOutput) json.RawMessage {
	body, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return body
}
