package aifeedback

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Feedback represents a user's feedback on an AI assistant message.
type Feedback struct {
	ID         uuid.UUID       `json:"id"`
	SessionID  uuid.UUID       `json:"sessionId"`
	MessageSeq int             `json:"messageSeq"`
	Rating     int             `json:"rating"` // 1=thumbs up, -1=thumbs down, 0=neutral
	Correction string          `json:"correction,omitempty"`
	Intent     string          `json:"intent,omitempty"`
	Domains    json.RawMessage `json:"domains,omitempty"`
	ToolsUsed  json.RawMessage `json:"toolsUsed,omitempty"`
	CreatedBy  *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// CreateFeedbackInput is the payload for submitting feedback.
type CreateFeedbackInput struct {
	SessionID  uuid.UUID `json:"sessionId"`
	MessageSeq int       `json:"messageSeq"`
	Rating     int       `json:"rating"`
	Correction string    `json:"correction,omitempty"`
}

// ListFeedbackOptions filters feedback queries.
type ListFeedbackOptions struct {
	SessionID *uuid.UUID
	Rating    *int
	Since     *time.Time
	Limit     int
	Offset    int
}

// FeedbackStats aggregates feedback by intent and domain.
type FeedbackStats struct {
	Intent   string  `json:"intent"`
	Domain   string  `json:"domain"`
	Total    int     `json:"total"`
	Positive int     `json:"positive"`
	Negative int     `json:"negative"`
	Rate     float64 `json:"rate"`
}

// ToolOutcomeStats aggregates tool call outcomes.
type ToolOutcomeStats struct {
	ToolName    string  `json:"toolName"`
	Domain      string  `json:"domain"`
	Total       int     `json:"total"`
	Successes   int     `json:"successes"`
	Failures    int     `json:"failures"`
	SuccessRate float64 `json:"successRate"`
}
