package aiskill

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"autotest/internal/logx"

	"github.com/google/uuid"
)

// DiscoveryDeps holds dependencies for skill candidate discovery.
type DiscoveryDeps struct {
	// RoutingLogQuerier provides access to ai_routing_logs for aggregation.
	RoutingLogQuerier RoutingLogQuerier
	// ToolOutcomeQuerier provides access to ai_tool_outcomes for success rates.
	ToolOutcomeQuerier ToolOutcomeQuerier
	// CandidateRepo persists discovered candidates.
	CandidateRepo *CandidateRepository
	// SkillRepo is used to auto-approve candidates into正式技能.
	SkillRepo *Repository
}

// RoutingLogQuerier abstracts routing log queries.
type RoutingLogQuerier interface {
	AggregateToolSequences(ctx context.Context, since time.Time) ([]ToolSequenceAgg, error)
}

// ToolOutcomeQuerier abstracts tool outcome queries.
type ToolOutcomeQuerier interface {
	AggregateSuccessByToolSequence(ctx context.Context, since time.Time) ([]ToolSequenceSuccess, error)
}

// ToolSequenceAgg represents an aggregated tool call sequence from routing logs.
type ToolSequenceAgg struct {
	ToolSequence   json.RawMessage
	Frequency      int
	AvgHops        float64
	SampleSessions []uuid.UUID
	// Intent is extracted from the planner_output.
	Intent string
}

// ToolSequenceSuccess represents success rate data for a tool sequence.
type ToolSequenceSuccess struct {
	ToolSequence json.RawMessage
	Total        int
	Successes    int
}

// DiscoverCandidates scans recent routing logs and tool outcomes to find
// high-frequency, high-success tool call patterns that could become skills.
func (d *DiscoveryDeps) DiscoverCandidates(ctx context.Context, since time.Time) ([]SkillCandidate, error) {
	if d.RoutingLogQuerier == nil || d.CandidateRepo == nil {
		return nil, fmt.Errorf("discovery deps not configured")
	}

	// Step 1: Aggregate tool sequences from routing logs.
	seqAggs, err := d.RoutingLogQuerier.AggregateToolSequences(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("aggregate tool sequences: %w", err)
	}

	// Step 2: Get success rates from tool outcomes.
	var successMap map[string]ToolSequenceSuccess
	if d.ToolOutcomeQuerier != nil {
		successes, err := d.ToolOutcomeQuerier.AggregateSuccessByToolSequence(ctx, since)
		if err == nil {
			successMap = make(map[string]ToolSequenceSuccess, len(successes))
			for _, s := range successes {
				successMap[string(s.ToolSequence)] = s
			}
		}
	}

	// Step 3: Build candidates.
	var candidates []SkillCandidate
	for _, agg := range seqAggs {
		if agg.Frequency < 3 {
			continue // too few occurrences
		}
		c := SkillCandidate{
			IntentPattern:  agg.Intent,
			ToolSequence:   agg.ToolSequence,
			Frequency:      agg.Frequency,
			AvgHops:        agg.AvgHops,
			SampleSessions: mustJSON(agg.SampleSessions),
			Status:         "pending",
		}
		// Calculate success rate from tool outcomes if available.
		if successMap != nil {
			if s, ok := successMap[string(agg.ToolSequence)]; ok && s.Total > 0 {
				c.SuccessRate = float64(s.Successes) / float64(s.Total)
			} else {
				c.SuccessRate = 0.8 // default assumption when no outcome data
			}
		} else {
			c.SuccessRate = 0.8
		}

		// Auto-approve if criteria are met.
		if c.SuccessRate > 0.95 && c.Frequency > 10 {
			c.Status = "auto_approved"
		}

		candidates = append(candidates, c)
	}

	// Step 4: Persist candidates (upsert by tool_sequence).
	var result []SkillCandidate
	for i := range candidates {
		if err := d.CandidateRepo.UpsertByToolSequence(ctx, &candidates[i]); err != nil {
			logx.Warn("skill discovery: upsert candidate", "err", err)
			continue
		}
		result = append(result, candidates[i])
	}

	// Step 5: Auto-approve eligible candidates.
	for i := range result {
		if result[i].Status == "auto_approved" {
			if err := d.autoApprove(ctx, &result[i]); err != nil {
				logx.Warn("skill discovery: auto-approve", "err", err, "candidate", result[i].ID)
			}
		}
	}

	return result, nil
}

// autoApprove creates a正式技能 from an auto-approved candidate.
func (d *DiscoveryDeps) autoApprove(ctx context.Context, c *SkillCandidate) error {
	if d.SkillRepo == nil {
		return nil
	}

	// Parse tool sequence to extract tool domains.
	var tools []string
	if err := json.Unmarshal(c.ToolSequence, &tools); err != nil {
		return err
	}

	// Build a skill name from the tool sequence.
	name := "auto_" + shortHash(string(c.ToolSequence))
	label := "自动发现: " + truncateStr(c.IntentPattern, 30)
	description := fmt.Sprintf("从 %d 次成功对话中自动发现的工具调用模式（成功率 %.0f%%）",
		c.Frequency, c.SuccessRate*100)

	// Create triggers from the intent pattern.
	triggers := []SkillTrigger{}
	if c.IntentPattern != "" {
		triggers = append(triggers, SkillTrigger{Type: "intent", Pattern: c.IntentPattern})
	}

	_, err := d.SkillRepo.Create(ctx, nil, uuid.Nil, CreateSkillInput{
		Name:           name,
		Label:          label,
		Description:    description,
		PromptTemplate: "",
		ToolDomains:    tools,
		Triggers:       triggers,
	})
	return err
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	if b == nil {
		return json.RawMessage("[]")
	}
	return b
}

func shortHash(s string) string {
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)[:8]
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
