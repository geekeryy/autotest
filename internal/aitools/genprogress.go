package aitools

import "context"

type genJobProgressKey struct{}
type progressSinkKey struct{}

// GenJobProgress is streamed to SSE clients during scenario generate-and-verify jobs.
type GenJobProgress struct {
	JobID     string             `json:"jobId"`
	Status    string             `json:"status"`
	Round     int                `json:"round"`
	MaxRounds int                `json:"maxRounds"`
	Phase     string             `json:"phase"`
	PassRate  float64            `json:"passRate"`
	Message   string             `json:"message,omitempty"`
	Repairs   []GenRepairSummary `json:"repairs,omitempty"`
}

// GenRepairSummary describes one auto-repair action in a gen job round.
type GenRepairSummary struct {
	ScenarioID string `json:"scenarioId"`
	StepID     string `json:"stepId,omitempty"`
	Category   string `json:"category"`
	Action     string `json:"action"`
	Detail     string `json:"detail,omitempty"`
}

// GenJobProgressSink emits progress events during async gen jobs.
type GenJobProgressSink func(GenJobProgress) error

// WithGenJobProgress attaches a progress sink to ctx for tool/agent execution.
func WithGenJobProgress(ctx context.Context, sink GenJobProgressSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, genJobProgressKey{}, sink)
}

// GenJobProgressFrom returns the sink stamped on ctx, if any.
func GenJobProgressFrom(ctx context.Context) GenJobProgressSink {
	return GenJobProgressSinkFrom(ctx)
}

// GenJobProgressSinkFrom returns the sink stamped on ctx, if any.
func GenJobProgressSinkFrom(ctx context.Context) GenJobProgressSink {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(genJobProgressKey{}).(GenJobProgressSink)
	return s
}

// EmitGenJobProgress invokes the typed sink when present.
func EmitGenJobProgress(ctx context.Context, p GenJobProgress) {
	sink := GenJobProgressSinkFrom(ctx)
	if sink != nil {
		_ = sink(p)
	}
}

// ProgressSink receives generic progress events.
type ProgressSink func(kind string, payload any) error

// WithProgressSink attaches a generic sink for tool implementations.
func WithProgressSink(ctx context.Context, sink ProgressSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, progressSinkKey{}, sink)
}

// ProgressSinkFrom returns the generic sink from ctx.
func ProgressSinkFrom(ctx context.Context) ProgressSink {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(progressSinkKey{}).(ProgressSink)
	return s
}

// EmitProgress invokes the generic sink when present.
func EmitProgress(ctx context.Context, kind string, payload any) {
	sink := ProgressSinkFrom(ctx)
	if sink == nil {
		return
	}
	_ = sink(kind, payload)
}
