package evalagent

import (
	"context"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// Repository handles ai_eval_reports persistence.
type Repository struct {
	store.Repository
}

// NewRepository constructs a Repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts an evaluation report.
func (r *Repository) Create(ctx context.Context, report *EvalReport) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	return r.DB.QueryRow(ctx, `
		INSERT INTO ai_eval_reports (period_start, period_end, routing_accuracy, tool_efficiency,
		                             user_satisfaction, tool_success_rate, skill_coverage, suggestions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, report.PeriodStart, report.PeriodEnd, report.RoutingAccuracy, report.ToolEfficiency,
		report.UserSatisfaction, report.ToolSuccessRate, report.SkillCoverage, report.Suggestions,
	).Scan(&report.ID, &report.CreatedAt)
}

// List returns reports ordered by creation time.
func (r *Repository) List(ctx context.Context, limit int) ([]EvalReport, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.DB.Query(ctx, `
		SELECT id, period_start, period_end, routing_accuracy, tool_efficiency,
		       user_satisfaction, tool_success_rate, skill_coverage, suggestions, created_at
		FROM ai_eval_reports
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvalReport
	for rows.Next() {
		var r EvalReport
		if err := rows.Scan(&r.ID, &r.PeriodStart, &r.PeriodEnd, &r.RoutingAccuracy,
			&r.ToolEfficiency, &r.UserSatisfaction, &r.ToolSuccessRate,
			&r.SkillCoverage, &r.Suggestions, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get returns a single report by ID.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*EvalReport, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var report EvalReport
	err := r.DB.QueryRow(ctx, `
		SELECT id, period_start, period_end, routing_accuracy, tool_efficiency,
		       user_satisfaction, tool_success_rate, skill_coverage, suggestions, created_at
		FROM ai_eval_reports WHERE id = $1
	`, id).Scan(&report.ID, &report.PeriodStart, &report.PeriodEnd, &report.RoutingAccuracy,
		&report.ToolEfficiency, &report.UserSatisfaction, &report.ToolSuccessRate,
		&report.SkillCoverage, &report.Suggestions, &report.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// AggregateFeedback returns total and positive feedback count since the given time.
func (r *Repository) AggregateFeedback(ctx context.Context, since time.Time) (total, positive int, err error) {
	if r.DB == nil {
		return 0, 0, fmt.Errorf("database unavailable")
	}
	err = r.DB.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE deleted_at IS NULL),
		       COUNT(*) FILTER (WHERE deleted_at IS NULL AND rating = 1)
		FROM ai_feedback
		WHERE created_at >= $1
	`, since).Scan(&total, &positive)
	return
}

// AggregateOutcomes returns total, success count, and average hops since the given time.
func (r *Repository) AggregateOutcomes(ctx context.Context, since time.Time) (total, successes int, avgHops float64, err error) {
	if r.DB == nil {
		return 0, 0, 0, fmt.Errorf("database unavailable")
	}
	err = r.DB.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE success), COALESCE(AVG(hop_index), 0)
		FROM ai_tool_outcomes
		WHERE created_at >= $1
	`, since).Scan(&total, &successes, &avgHops)
	return
}

// AggregateRoutingAccuracy returns total routing logs and those with matching intent-domain alignment.
func (r *Repository) AggregateRoutingAccuracy(ctx context.Context, since time.Time) (total, correct int, err error) {
	if r.DB == nil {
		return 0, 0, fmt.Errorf("database unavailable")
	}
	// A routing decision is "correct" when outcome is 'success'.
	err = r.DB.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE outcome = 'success')
		FROM ai_routing_logs
		WHERE created_at >= $1
	`, since).Scan(&total, &correct)
	return
}
