package aiskill

import (
	"context"
	"encoding/json"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// SkillCandidate represents an auto-discovered skill pattern from routing logs.
type SkillCandidate struct {
	ID             uuid.UUID       `json:"id"`
	IntentPattern  string          `json:"intentPattern"`
	ToolSequence   json.RawMessage `json:"toolSequence"`
	Frequency      int             `json:"frequency"`
	SuccessRate    float64         `json:"successRate"`
	AvgHops        float64         `json:"avgHops"`
	SampleSessions json.RawMessage `json:"sampleSessions"`
	Status         string          `json:"status"` // pending / auto_approved / approved / rejected
	GeneratedAt    time.Time       `json:"generatedAt"`
	ReviewedAt     *time.Time      `json:"reviewedAt,omitempty"`
}

// CandidateRepository handles ai_skill_candidates persistence.
type CandidateRepository struct {
	store.Repository
}

// NewCandidateRepository constructs a CandidateRepository.
func NewCandidateRepository(repo store.Repository) *CandidateRepository {
	return &CandidateRepository{Repository: repo}
}

// List returns candidates matching the given status filter.
func (r *CandidateRepository) List(ctx context.Context, status string, limit int) ([]SkillCandidate, error) {
	q := `SELECT id, intent_pattern, tool_sequence, frequency, success_rate, avg_hops,
	             sample_sessions, status, generated_at, reviewed_at
	      FROM ai_skill_candidates WHERE deleted_at IS NULL`
	args := []any{}
	idx := 1
	if status != "" {
		q += ` AND status = $` + itoa(idx)
		args = append(args, status)
		idx++
	}
	q += ` ORDER BY frequency DESC, success_rate DESC`
	if limit > 0 {
		q += ` LIMIT $` + itoa(idx)
		args = append(args, limit)
	}
	rows, err := r.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SkillCandidate
	for rows.Next() {
		var c SkillCandidate
		if err := rows.Scan(&c.ID, &c.IntentPattern, &c.ToolSequence, &c.Frequency,
			&c.SuccessRate, &c.AvgHops, &c.SampleSessions, &c.Status,
			&c.GeneratedAt, &c.ReviewedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create inserts a new candidate.
func (r *CandidateRepository) Create(ctx context.Context, c *SkillCandidate) error {
	return r.DB.QueryRow(ctx, `
		INSERT INTO ai_skill_candidates (intent_pattern, tool_sequence, frequency, success_rate, avg_hops, sample_sessions, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, generated_at
	`, c.IntentPattern, c.ToolSequence, c.Frequency, c.SuccessRate, c.AvgHops, c.SampleSessions, c.Status).Scan(&c.ID, &c.GeneratedAt)
}

// UpdateStatus changes the status of a candidate.
func (r *CandidateRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE ai_skill_candidates SET status = $2, reviewed_at = now() WHERE id = $1
	`, id, status)
	return err
}

// UpsertByToolSequence inserts or updates a candidate based on tool_sequence uniqueness.
// If a candidate with the same tool_sequence exists, it increments frequency and
// recalculates success_rate. Otherwise it inserts a new row.
func (r *CandidateRepository) UpsertByToolSequence(ctx context.Context, c *SkillCandidate) error {
	return r.DB.QueryRow(ctx, `
		INSERT INTO ai_skill_candidates (intent_pattern, tool_sequence, frequency, success_rate, avg_hops, sample_sessions, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tool_sequence) WHERE deleted_at IS NULL
		DO UPDATE SET
			frequency = ai_skill_candidates.frequency + EXCLUDED.frequency,
			success_rate = (ai_skill_candidates.success_rate * ai_skill_candidates.frequency + EXCLUDED.success_rate * EXCLUDED.frequency)
			              / (ai_skill_candidates.frequency + EXCLUDED.frequency),
			avg_hops = (ai_skill_candidates.avg_hops * ai_skill_candidates.frequency + EXCLUDED.avg_hops * EXCLUDED.frequency)
			          / (ai_skill_candidates.frequency + EXCLUDED.frequency),
			sample_sessions = (ai_skill_candidates.sample_sessions || EXCLUDED.sample_sessions)::jsonb
		RETURNING id, generated_at
	`, c.IntentPattern, c.ToolSequence, c.Frequency, c.SuccessRate, c.AvgHops, c.SampleSessions, c.Status).Scan(&c.ID, &c.GeneratedAt)
}

func itoa(i int) string {
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if s == "" {
		s = "0"
	}
	return s
}
