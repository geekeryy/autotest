package aifeedback

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Service wraps the feedback repository.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SubmitFeedback records user feedback. It auto-fills intent/domains/toolsUsed
// from the routing log for the matching session+messageSeq when available.
func (s *Service) SubmitFeedback(ctx context.Context, userID uuid.UUID, input CreateFeedbackInput) (*Feedback, error) {
	if input.SessionID == uuid.Nil {
		return nil, ErrSessionRequired
	}
	if input.Rating < -1 || input.Rating > 1 {
		return nil, ErrInvalidRating
	}

	// TODO: look up intent/domains/toolsUsed from ai_routing_logs
	// For now, submit with empty metadata; the caller can enrich later.
	domains := json.RawMessage("[]")
	toolsUsed := json.RawMessage("[]")
	intent := ""

	return s.repo.Create(ctx, userID, input, intent, domains, toolsUsed)
}

// List returns feedback matching filters.
func (s *Service) List(ctx context.Context, opts ListFeedbackOptions) ([]Feedback, error) {
	return s.repo.List(ctx, opts)
}

// GetStats returns aggregated feedback stats since the given time.
func (s *Service) GetStats(ctx context.Context, since time.Time) ([]FeedbackStats, error) {
	return s.repo.AggregateByIntentDomain(ctx, since)
}

// Errors.
var (
	ErrSessionRequired = &FeedbackError{"sessionId 不能为空"}
	ErrInvalidRating   = &FeedbackError{"rating 必须是 -1, 0 或 1"}
)

type FeedbackError struct {
	msg string
}

func (e *FeedbackError) Error() string { return e.msg }
