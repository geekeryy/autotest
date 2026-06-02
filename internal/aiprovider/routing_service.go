package aiprovider

// routing_service.go contains the Service-level glue that runs the
// Planner→Router pre-pass for the SSE assistant and persists the resulting
// routing telemetry into ai_routing_logs. This is intentionally separate
// from the pure planner.go / router.go so those stay IO-free and unit
// testable.

import (
	"context"
	"encoding/json"

	"autotest/internal/aiprovider/client"
	"autotest/internal/logx"
)

// logRoutingWarn records a non-fatal routing/telemetry issue. Routing is a
// best-effort observability layer: a failure here must never surface to the
// user or change the conversation outcome.
func logRoutingWarn(msg string, err error) {
	if err == nil {
		return
	}
	logx.Warn("ai routing: "+msg, "err", err.Error())
}

// applyRouting runs the Planner (one cheap LLM call) and the Router (pure
// rules) before the main conversation loop, writing the resulting tool
// package and telemetry onto cfg. It NEVER fails the conversation: planner
// errors degrade to a safe plan and the loop proceeds (in shadow mode this
// is purely observational anyway).
func (s *Service) applyRouting(
	ctx context.Context,
	cfg *streamConfig,
	cli client.Client,
	provider *providerRow,
	userMsg string,
	pageContext json.RawMessage,
	messageSeq int,
) {
	if cfg == nil || cfg.Routing == nil || !s.routingConfigured() {
		return
	}
	cfg.Routing.MessageSeq = messageSeq

	plan, err := s.planner.Plan(ctx, cli, provider, userMsg, pageContext, "")
	if err != nil {
		// Degraded plan already returned; log nothing fatal. The router
		// will widen to a safe package.
		logRoutingWarn("planner degraded", err)
	}
	cfg.Routing.PlannerOutput = marshalPlannerOutput(plan)

	// Skill matching: find skills that match the user's input.
	var skillHints []SkillDomainHint
	if s.skillSvc != nil {
		projectID := cfg.ProjectID
		matched, err := s.skillSvc.MatchSkills(ctx, &projectID, userMsg, pageContext)
		if err == nil {
			for _, sk := range matched {
				skillHints = append(skillHints, SkillDomainHint{
					Name:        sk.Name,
					ToolDomains: sk.ToolDomains,
				})
			}
			if len(matched) > 0 {
				cfg.MatchedSkillName = matched[0].Name
			}
		}
	}

	routed := s.router.Route(plan, pageContext, s.catalog, skillHints)
	cfg.Routing.RouterOutput = marshalRouterOutput(routed)
	cfg.Routing.Confidence = routed.Confidence
	cfg.setActiveTools(routed.ActiveTools)
}

// recoverRouting rebuilds the active tool package for a resumed stream
// (ContinueAfterConfirm) WITHOUT re-running the Planner. We fall back to a
// page-context-only Router pass so confirm-time tool availability is at
// least as wide as the original turn would have offered.
func (s *Service) recoverRouting(
	cfg *streamConfig,
	pageContext json.RawMessage,
	messageSeq int,
) {
	if cfg == nil || cfg.Routing == nil || !s.routingConfigured() {
		return
	}
	cfg.Routing.MessageSeq = messageSeq
	// No planner output on resume; let the Router infer from the page only.
	// An empty plan yields low confidence, so the Router widens to a safe
	// package — exactly what we want when we cannot replay the intent.
	routed := s.router.Route(safePlannerOutput(), pageContext, s.catalog, nil)
	cfg.Routing.RouterOutput = marshalRouterOutput(routed)
	cfg.Routing.Confidence = routed.Confidence
	cfg.setActiveTools(routed.ActiveTools)
}

// finalizeRoutingLog persists the accumulated routing telemetry. Telemetry
// failures are swallowed (logged) — they must never surface to the user or
// alter the SSE outcome. streamErr (if any) flips the outcome to "error"
// when no terminal outcome was recorded by the loop.
func (s *Service) finalizeRoutingLog(ctx context.Context, cfg *streamConfig, store SessionStore, streamErr error) {
	if cfg == nil || cfg.Routing == nil || !cfg.Routing.Enabled || store == nil {
		return
	}
	outcome := cfg.Routing.Outcome
	if streamErr != nil && outcome == "" {
		outcome = "error"
	}
	if outcome == "" {
		outcome = "error"
	}
	input := RoutingLogInput{
		SessionID:     cfg.SessionID,
		MessageSeq:    cfg.Routing.MessageSeq,
		PlannerOutput: cfg.Routing.PlannerOutput,
		RouterOutput:  cfg.Routing.RouterOutput,
		PerHop:        cfg.Routing.marshalPerHop(),
		Outcome:       outcome,
	}
	if err := store.AppendRoutingLog(ctx, input); err != nil {
		logRoutingWarn("append routing log", err)
	}
}

// setActiveTools loads the Router's tool package into cfg.ActiveTools.
func (cfg *streamConfig) setActiveTools(names []string) {
	if cfg.ActiveTools == nil {
		cfg.ActiveTools = map[string]bool{}
	}
	for _, n := range names {
		cfg.ActiveTools[n] = true
	}
}
