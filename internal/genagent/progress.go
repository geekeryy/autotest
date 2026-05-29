package genagent

import (
	"context"

	"autotest/internal/aitools"
)

// Progress is streamed to SSE clients during a gen job.
type Progress struct {
	JobID     string          `json:"jobId"`
	Status    string          `json:"status"`
	Round     int             `json:"round"`
	MaxRounds int             `json:"maxRounds"`
	Phase     string          `json:"phase"`
	PassRate  float64         `json:"passRate"`
	Message   string          `json:"message,omitempty"`
	Repairs   []RepairSummary `json:"repairs,omitempty"`
}

func emitProgress(ctx context.Context, p Progress) {
	repairs := make([]aitools.GenRepairSummary, 0, len(p.Repairs))
	for _, r := range p.Repairs {
		repairs = append(repairs, aitools.GenRepairSummary{
			ScenarioID: r.ScenarioID.String(),
			StepID:     r.StepID.String(),
			Category:   r.Category,
			Action:     r.Action,
			Detail:     r.Detail,
		})
	}
	aitools.EmitGenJobProgress(ctx, aitools.GenJobProgress{
		JobID: p.JobID, Status: p.Status, Round: p.Round, MaxRounds: p.MaxRounds,
		Phase: p.Phase, PassRate: p.PassRate, Message: p.Message, Repairs: repairs,
	})
}

// WithProgressBridge copies aitools gen job sink into ctx for async goroutines.
func WithProgressBridge(ctx context.Context) context.Context {
	if sink := aitools.GenJobProgressSinkFrom(ctx); sink != nil {
		return aitools.WithGenJobProgress(context.Background(), sink)
	}
	if sink := aitools.ProgressSinkFrom(ctx); sink != nil {
		return aitools.WithProgressSink(context.Background(), sink)
	}
	return ctx
}
