package evalagent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EvalReport represents a self-evaluation report for the AI assistant.
type EvalReport struct {
	ID               uuid.UUID       `json:"id"`
	PeriodStart      time.Time       `json:"periodStart"`
	PeriodEnd        time.Time       `json:"periodEnd"`
	RoutingAccuracy  float64         `json:"routingAccuracy"`
	ToolEfficiency   float64         `json:"toolEfficiency"`
	UserSatisfaction float64         `json:"userSatisfaction"`
	ToolSuccessRate  float64         `json:"toolSuccessRate"`
	SkillCoverage    float64         `json:"skillCoverage"`
	Suggestions      json.RawMessage `json:"suggestions"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// EvalDeps holds dependencies for running evaluations.
type EvalDeps struct {
	// FeedbackQuerier provides feedback aggregation.
	FeedbackQuerier FeedbackQuerier
	// OutcomeQuerier provides tool outcome aggregation.
	OutcomeQuerier OutcomeQuerier
	// RoutingQuerier provides routing log aggregation.
	RoutingQuerier RoutingQuerier
	// Repo persists evaluation reports.
	Repo *Repository
}

// FeedbackQuerier abstracts feedback queries.
type FeedbackQuerier interface {
	AggregateFeedback(ctx interface{}, since time.Time) (total, positive int, err error)
}

// OutcomeQuerier abstracts tool outcome queries.
type OutcomeQuerier interface {
	AggregateOutcomes(ctx interface{}, since time.Time) (total, successes int, avgHops float64, err error)
}

// RoutingQuerier abstracts routing log queries.
type RoutingQuerier interface {
	AggregateRoutingAccuracy(ctx interface{}, since time.Time) (total, correct int, err error)
}
