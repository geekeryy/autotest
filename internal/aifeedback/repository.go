package aifeedback

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// Repository handles ai_feedback persistence.
type Repository struct {
	store.Repository
}

// NewRepository constructs a Repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a feedback record.
func (r *Repository) Create(ctx context.Context, userID uuid.UUID, input CreateFeedbackInput, intent string, domains, toolsUsed json.RawMessage) (*Feedback, error) {
	var f Feedback
	err := r.DB.QueryRow(ctx, `
		INSERT INTO ai_feedback (session_id, message_seq, rating, correction, intent, domains, tools_used, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, session_id, message_seq, rating, correction, intent, domains, tools_used, created_by, created_at
	`, input.SessionID, input.MessageSeq, input.Rating, input.Correction, intent, domains, toolsUsed, userID).Scan(
		&f.ID, &f.SessionID, &f.MessageSeq, &f.Rating, &f.Correction, &f.Intent, &f.Domains, &f.ToolsUsed, &f.CreatedBy, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// List returns feedback matching the filters.
func (r *Repository) List(ctx context.Context, opts ListFeedbackOptions) ([]Feedback, error) {
	q := `SELECT id, session_id, message_seq, rating, correction, intent, domains, tools_used, created_by, created_at
	      FROM ai_feedback WHERE deleted_at IS NULL`
	args := []any{}
	idx := 1

	if opts.SessionID != nil {
		q += ` AND session_id = $` + fmt.Sprint(idx)
		args = append(args, *opts.SessionID)
		idx++
	}
	if opts.Rating != nil {
		q += ` AND rating = $` + fmt.Sprint(idx)
		args = append(args, *opts.Rating)
		idx++
	}
	if opts.Since != nil {
		q += ` AND created_at >= $` + fmt.Sprint(idx)
		args = append(args, *opts.Since)
		idx++
	}
	q += ` ORDER BY created_at DESC`
	if opts.Limit > 0 {
		q += ` LIMIT $` + fmt.Sprint(idx)
		args = append(args, opts.Limit)
		idx++
	}
	if opts.Offset > 0 {
		q += ` OFFSET $` + fmt.Sprint(idx)
		args = append(args, opts.Offset)
		idx++
	}

	rows, err := r.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Feedback
	for rows.Next() {
		var f Feedback
		if err := rows.Scan(&f.ID, &f.SessionID, &f.MessageSeq, &f.Rating, &f.Correction, &f.Intent, &f.Domains, &f.ToolsUsed, &f.CreatedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AggregateByIntentDomain computes success rates grouped by intent and domain.
func (r *Repository) AggregateByIntentDomain(ctx context.Context, since time.Time) ([]FeedbackStats, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT intent, d.domain,
		       COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE rating = 1) AS positive,
		       COUNT(*) FILTER (WHERE rating = -1) AS negative
		FROM ai_feedback, jsonb_array_elements_text(domains) AS d(domain)
		WHERE deleted_at IS NULL AND created_at >= $1 AND intent != ''
		GROUP BY intent, d.domain
		ORDER BY total DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FeedbackStats
	for rows.Next() {
		var s FeedbackStats
		if err := rows.Scan(&s.Intent, &s.Domain, &s.Total, &s.Positive, &s.Negative); err != nil {
			return nil, err
		}
		if s.Total > 0 {
			s.Rate = float64(s.Positive) / float64(s.Total)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
