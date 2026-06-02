package aimemory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository persists AI memories.
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a new memory.
func (r *Repository) Create(ctx context.Context, input CreateMemoryInput) (*Memory, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var projectID any
	if input.ProjectID != nil && *input.ProjectID != uuid.Nil {
		projectID = *input.ProjectID
	}
	var userID any
	if input.UserID != nil && *input.UserID != uuid.Nil {
		userID = *input.UserID
	}
	var expiresAt any
	if input.ExpiresAt != nil {
		expiresAt = *input.ExpiresAt
	}
	confidence := input.Confidence
	if confidence <= 0 {
		confidence = 0.8
	}
	row := r.DB.QueryRow(ctx, `
		insert into ai_memories (project_id, user_id, type, category, key, content, source, confidence, expires_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning id, project_id, user_id, type, category, key, content, source, confidence, ref_count, last_used_at, expires_at, created_at, updated_at
	`, projectID, userID, input.Type, input.Category, input.Key, input.Content, input.Source, confidence, expiresAt)
	return scanMemory(row)
}

// List returns memories for a user/project combination.
func (r *Repository) List(ctx context.Context, projectID, userID uuid.UUID, opts ListOptions) ([]Memory, int, error) {
	if r.DB == nil {
		return nil, 0, fmt.Errorf("database unavailable")
	}
	// Build query
	where := []string{"deleted_at is null"}
	args := []any{}
	argIdx := 1

	if projectID != uuid.Nil {
		where = append(where, fmt.Sprintf("(project_id = $%d OR project_id IS NULL)", argIdx))
		args = append(args, projectID)
		argIdx++
	}
	if userID != uuid.Nil {
		where = append(where, fmt.Sprintf("(user_id = $%d OR user_id IS NULL)", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if opts.Type != "" {
		where = append(where, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, opts.Type)
		argIdx++
	}
	if opts.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, opts.Category)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM ai_memories WHERE %s", whereClause)
	err := r.DB.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count memories: %w", err)
	}

	// Fetch records
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, project_id, user_id, type, category, key, content, source, confidence, ref_count, last_used_at, expires_at, created_at, updated_at
		FROM ai_memories
		WHERE %s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	out := []Memory{}
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// Get fetches a single memory by ID.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Memory, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, user_id, type, category, key, content, source, confidence, ref_count, last_used_at, expires_at, created_at, updated_at
		FROM ai_memories
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	m, err := scanMemory(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("memory 不存在")
		}
		return nil, err
	}
	return m, nil
}

// Update modifies an existing memory.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, input UpdateMemoryInput) (*Memory, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	setClauses := []string{"updated_at = now()"}
	args := []any{}
	argIdx := 1

	if input.Content != nil {
		setClauses = append(setClauses, fmt.Sprintf("content = $%d", argIdx))
		args = append(args, *input.Content)
		argIdx++
	}
	if input.Confidence != nil {
		setClauses = append(setClauses, fmt.Sprintf("confidence = $%d", argIdx))
		args = append(args, *input.Confidence)
		argIdx++
	}

	setClause := strings.Join(setClauses, ", ")
	args = append(args, id)

	row := r.DB.QueryRow(ctx, fmt.Sprintf(`
		UPDATE ai_memories
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, project_id, user_id, type, category, key, content, source, confidence, ref_count, last_used_at, expires_at, created_at, updated_at
	`, setClause, argIdx), args...)
	return scanMemory(row)
}

// Delete soft-deletes a memory.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE ai_memories
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory 不存在")
	}
	return nil
}

// Touch increments ref_count and updates last_used_at.
func (r *Repository) Touch(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		UPDATE ai_memories
		SET ref_count = ref_count + 1, last_used_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// Prune removes expired and low-confidence memories.
func (r *Repository) Prune(ctx context.Context, maxAge time.Duration, minConfidence float64) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		DELETE FROM ai_memories
		WHERE (expires_at IS NOT NULL AND expires_at < now())
		   OR (type = 'auto' AND ref_count = 0 AND last_used_at IS NULL AND created_at < now() - INTERVAL '30 days')
	`)
	if err != nil {
		return 0, fmt.Errorf("prune memories: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// CreateMemoryInput is the internal projection for creating a memory.
type CreateMemoryInput struct {
	ProjectID  *uuid.UUID
	UserID     *uuid.UUID
	Type       string
	Category   string
	Key        string
	Content    string
	Source     string
	Confidence float64
	ExpiresAt  *time.Time
}

// UpdateMemoryInput is the internal projection for updating a memory.
type UpdateMemoryInput struct {
	Content    *string
	Confidence *float64
}

func scanMemory(row rowScanner) (*Memory, error) {
	var m Memory
	var projectID pgtype.UUID
	var userID pgtype.UUID
	var lastUsedAt pgtype.Timestamptz
	var expiresAt pgtype.Timestamptz
	if err := row.Scan(
		&m.ID, &projectID, &userID, &m.Type, &m.Category, &m.Key, &m.Content,
		&m.Source, &m.Confidence, &m.RefCount, &lastUsedAt, &expiresAt,
		&m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if projectID.Valid {
		id := uuid.UUID(projectID.Bytes)
		m.ProjectID = &id
	}
	if userID.Valid {
		id := uuid.UUID(userID.Bytes)
		m.UserID = &id
	}
	if lastUsedAt.Valid {
		m.LastUsedAt = &lastUsedAt.Time
	}
	if expiresAt.Valid {
		m.ExpiresAt = &expiresAt.Time
	}
	return &m, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}
