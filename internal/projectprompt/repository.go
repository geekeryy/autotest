package projectprompt

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository persists per-project AI prompt configurations.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// List returns all active prompts for the given project.
func (r *Repository) List(ctx context.Context, projectID uuid.UUID) ([]ProjectPrompt, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, project_id, action, name, system_prompt, default_model, provider_id, enabled, created_at, updated_at
		from project_ai_prompts
		where project_id = $1 and deleted_at is null
		order by created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project ai prompts: %w", err)
	}
	defer rows.Close()

	var out []ProjectPrompt
	for rows.Next() {
		p, err := scanPrompt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []ProjectPrompt{}, nil
	}
	return out, nil
}

// GetByAction returns the prompt override for a specific action within a project.
func (r *Repository) GetByAction(ctx context.Context, projectID uuid.UUID, action string) (*ProjectPrompt, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, action, name, system_prompt, default_model, provider_id, enabled, created_at, updated_at
		from project_ai_prompts
		where project_id = $1 and action = $2 and deleted_at is null
	`, projectID, action)
	p, err := scanPrompt(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// Create inserts a new prompt record.
func (r *Repository) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*ProjectPrompt, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var providerAny any
	if input.ProviderID != nil {
		providerAny = *input.ProviderID
	}
	row := r.DB.QueryRow(ctx, `
		insert into project_ai_prompts (project_id, action, name, system_prompt, default_model, provider_id, enabled)
		select p.id, $2, $3, $4, $5, $7, $6
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, action, name, system_prompt, default_model, provider_id, enabled, created_at, updated_at
	`, projectID, input.Action, input.Name, input.SystemPrompt, input.DefaultModel, enabled, providerAny)
	p, err := scanPrompt(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("project not found or inaccessible")
		}
		return nil, err
	}
	return p, nil
}

// Update mutates an existing prompt record.
func (r *Repository) Update(ctx context.Context, projectID, promptID uuid.UUID, input UpdateInput) (*ProjectPrompt, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var providerAny any
	if input.ProviderID != nil {
		providerAny = *input.ProviderID
	}
	row := r.DB.QueryRow(ctx, `
		update project_ai_prompts
		set name = $3,
		    system_prompt = $4,
		    default_model = $5,
		    provider_id = $7,
		    enabled = $6,
		    updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
		returning id, project_id, action, name, system_prompt, default_model, provider_id, enabled, created_at, updated_at
	`, projectID, promptID, input.Name, input.SystemPrompt, input.DefaultModel, enabled, providerAny)
	p, err := scanPrompt(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// Delete soft-deletes a prompt record.
func (r *Repository) Delete(ctx context.Context, projectID, promptID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update project_ai_prompts
		set deleted_at = now(), updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, promptID)
	if err != nil {
		return fmt.Errorf("delete project ai prompt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ProviderExistsForProject reports whether providerID is a non-soft-deleted provider for projectID.
func (r *Repository) ProviderExistsForProject(ctx context.Context, projectID, providerID uuid.UUID) (bool, error) {
	if r.DB == nil {
		return false, fmt.Errorf("database unavailable")
	}
	var exists bool
	err := r.DB.QueryRow(ctx, `
		select exists (
			select 1 from ai_providers
			where id = $2 and project_id = $1 and deleted_at is null
		)
	`, projectID, providerID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPrompt(row rowScanner) (*ProjectPrompt, error) {
	var p ProjectPrompt
	var prov pgtype.UUID
	if err := row.Scan(
		&p.ID, &p.ProjectID, &p.Action, &p.Name,
		&p.SystemPrompt, &p.DefaultModel, &prov, &p.Enabled,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if prov.Valid {
		u := uuid.UUID(prov.Bytes)
		p.ProviderID = &u
	}
	return &p, nil
}
