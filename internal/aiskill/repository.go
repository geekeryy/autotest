package aiskill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository persists AI skills.
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a new skill.
func (r *Repository) Create(ctx context.Context, projectID *uuid.UUID, createdBy uuid.UUID, input CreateSkillInput) (*Skill, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var projID any
	if projectID != nil && *projectID != uuid.Nil {
		projID = *projectID
	}
	var toolDomains any
	if len(input.ToolDomains) > 0 {
		toolDomains = input.ToolDomains
	}
	var inputSchema any
	if len(input.InputSchema) > 0 {
		inputSchema = input.InputSchema
	}
	var triggers any
	if len(input.Triggers) > 0 {
		triggers = input.Triggers
	}
	requireConfirm := false
	if input.RequireConfirm != nil {
		requireConfirm = *input.RequireConfirm
	}

	row := r.DB.QueryRow(ctx, `
		INSERT INTO ai_skills (project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by, enabled, created_at, updated_at
	`, projID, input.Name, input.Label, input.Description, input.PromptTemplate, toolDomains, inputSchema, requireConfirm, triggers, createdBy)
	return scanSkill(row)
}

// List returns skills with optional filtering.
func (r *Repository) List(ctx context.Context, opts ListSkillsOptions) ([]Skill, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	argIdx := 1

	if opts.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *opts.Enabled)
		argIdx++
	}
	if opts.ProjectID != nil && *opts.ProjectID != uuid.Nil {
		where = append(where, fmt.Sprintf("(project_id = $%d OR project_id IS NULL)", argIdx))
		args = append(args, *opts.ProjectID)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by, enabled, created_at, updated_at
		FROM ai_skills
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	out := []Skill{}
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// Get fetches a single skill by ID.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Skill, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by, enabled, created_at, updated_at
		FROM ai_skills
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	s, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("skill 不存在")
		}
		return nil, err
	}
	return s, nil
}

// GetByName fetches a skill by name.
func (r *Repository) GetByName(ctx context.Context, name string, projectID *uuid.UUID) (*Skill, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var projID any
	if projectID != nil && *projectID != uuid.Nil {
		projID = *projectID
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by, enabled, created_at, updated_at
		FROM ai_skills
		WHERE name = $1 AND (project_id = $2 OR project_id IS NULL) AND deleted_at IS NULL
	`, name, projID)
	s, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("skill 不存在")
		}
		return nil, err
	}
	return s, nil
}

// Update modifies an existing skill.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, input UpdateSkillInput) (*Skill, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	setClauses := []string{"updated_at = now()"}
	args := []any{}
	argIdx := 1

	if input.Label != nil {
		setClauses = append(setClauses, fmt.Sprintf("label = $%d", argIdx))
		args = append(args, *input.Label)
		argIdx++
	}
	if input.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *input.Description)
		argIdx++
	}
	if input.PromptTemplate != nil {
		setClauses = append(setClauses, fmt.Sprintf("prompt_template = $%d", argIdx))
		args = append(args, *input.PromptTemplate)
		argIdx++
	}
	if input.ToolDomains != nil {
		setClauses = append(setClauses, fmt.Sprintf("tool_domains = $%d", argIdx))
		args = append(args, input.ToolDomains)
		argIdx++
	}
	if input.InputSchema != nil {
		setClauses = append(setClauses, fmt.Sprintf("input_schema = $%d", argIdx))
		args = append(args, input.InputSchema)
		argIdx++
	}
	if input.RequireConfirm != nil {
		setClauses = append(setClauses, fmt.Sprintf("require_confirm = $%d", argIdx))
		args = append(args, *input.RequireConfirm)
		argIdx++
	}
	if input.Triggers != nil {
		setClauses = append(setClauses, fmt.Sprintf("triggers = $%d", argIdx))
		args = append(args, input.Triggers)
		argIdx++
	}
	if input.Enabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *input.Enabled)
		argIdx++
	}

	setClause := strings.Join(setClauses, ", ")
	args = append(args, id)

	row := r.DB.QueryRow(ctx, fmt.Sprintf(`
		UPDATE ai_skills
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, project_id, name, label, description, prompt_template, tool_domains, input_schema, require_confirm, triggers, created_by, enabled, created_at, updated_at
	`, setClause, argIdx), args...)
	return scanSkill(row)
}

// Delete soft-deletes a skill.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE ai_skills
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("skill 不存在")
	}
	return nil
}

func scanSkill(row rowScanner) (*Skill, error) {
	var s Skill
	var projectID pgtype.UUID
	var createdBy pgtype.UUID
	var toolDomains []byte
	var inputSchema []byte
	var triggers []byte
	if err := row.Scan(
		&s.ID, &projectID, &s.Name, &s.Label, &s.Description, &s.PromptTemplate,
		&toolDomains, &inputSchema, &s.RequireConfirm, &triggers,
		&createdBy, &s.Enabled, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if projectID.Valid {
		id := uuid.UUID(projectID.Bytes)
		s.ProjectID = &id
	}
	if createdBy.Valid {
		id := uuid.UUID(createdBy.Bytes)
		s.CreatedBy = &id
	}
	if len(toolDomains) > 0 {
		if err := json.Unmarshal(toolDomains, &s.ToolDomains); err != nil {
			return nil, fmt.Errorf("unmarshal tool_domains: %w", err)
		}
	}
	if len(inputSchema) > 0 {
		s.InputSchema = inputSchema
	}
	if len(triggers) > 0 {
		if err := json.Unmarshal(triggers, &s.Triggers); err != nil {
			return nil, fmt.Errorf("unmarshal triggers: %w", err)
		}
	}
	return &s, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}
