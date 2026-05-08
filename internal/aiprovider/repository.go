package aiprovider

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists AI provider configurations per project.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a new provider record. When IsDefault is true, all other defaults in the project are cleared first.
func (r *Repository) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create ai provider: %w", err)
	}
	defer tx.Rollback(ctx)

	if input.IsDefault {
		if _, err := tx.Exec(ctx, `
			update ai_providers
			set is_default = false, updated_at = now()
			where project_id = $1 and is_default = true and deleted_at is null
		`, projectID); err != nil {
			return nil, fmt.Errorf("clear previous default ai provider: %w", err)
		}
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	extra := input.ExtraConfig
	if len(extra) == 0 {
		extra = []byte("{}")
	}

	row := tx.QueryRow(ctx, `
		insert into ai_providers (
			project_id, name, provider_type, base_url, api_key, default_model,
			extra_config, enabled, is_default
		)
		select p.id, $2, $3, $4, $5, $6, $7::jsonb, $8, $9
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, name, provider_type, base_url, api_key, default_model,
		          extra_config, enabled, is_default, created_at, updated_at
	`, projectID, input.Name, input.ProviderType, input.BaseURL, input.APIKey, input.DefaultModel,
		extra, enabled, input.IsDefault)

	out, err := scanProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("project not found or inaccessible")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create ai provider: %w", err)
	}
	return out, nil
}

// Update mutates an existing provider record. If apiKey is nil the previous key is preserved.
func (r *Repository) Update(ctx context.Context, projectID, providerID uuid.UUID, input UpdateInput) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update ai provider: %w", err)
	}
	defer tx.Rollback(ctx)

	if input.IsDefault {
		if _, err := tx.Exec(ctx, `
			update ai_providers
			set is_default = false, updated_at = now()
			where project_id = $1 and is_default = true and id <> $2 and deleted_at is null
		`, projectID, providerID); err != nil {
			return nil, fmt.Errorf("clear previous default ai provider: %w", err)
		}
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	extra := input.ExtraConfig
	if len(extra) == 0 {
		extra = []byte("{}")
	}

	var apiKeyArg any
	keepAPIKey := input.APIKey == nil
	if !keepAPIKey {
		apiKeyArg = *input.APIKey
	}

	row := tx.QueryRow(ctx, `
		update ai_providers ap
		set name = $3,
		    provider_type = $4,
		    base_url = $5,
		    api_key = case when $11::boolean then ap.api_key else coalesce($6::text, '') end,
		    default_model = $7,
		    extra_config = $8::jsonb,
		    enabled = $9,
		    is_default = $10,
		    updated_at = now()
		from projects p
		where ap.project_id = $1
		  and ap.id = $2
		  and p.id = ap.project_id
		  and p.deleted_at is null
		  and ap.deleted_at is null
		returning ap.id, ap.project_id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		          ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		          ap.created_at, ap.updated_at
	`, projectID, providerID, input.Name, input.ProviderType, input.BaseURL, apiKeyArg,
		input.DefaultModel, extra, enabled, input.IsDefault, keepAPIKey)

	out, err := scanProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update ai provider: %w", err)
	}
	return out, nil
}

// Delete soft-deletes a provider record.
func (r *Repository) Delete(ctx context.Context, projectID, providerID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update ai_providers
		set deleted_at = now(), is_default = false, updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, providerID)
	if err != nil {
		return fmt.Errorf("delete ai provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// Get fetches a provider record (including its plaintext key for client construction).
func (r *Repository) Get(ctx context.Context, projectID, providerID uuid.UUID) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select ap.id, ap.project_id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		       ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		       ap.created_at, ap.updated_at
		from ai_providers ap
		join projects p on p.id = ap.project_id and p.deleted_at is null
		where ap.project_id = $1 and ap.id = $2 and ap.deleted_at is null
	`, projectID, providerID)
	out, err := scanProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

// ListByProject returns all active providers for a project.
func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select ap.id, ap.project_id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		       ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		       ap.created_at, ap.updated_at
		from ai_providers ap
		join projects p on p.id = ap.project_id and p.deleted_at is null
		where ap.project_id = $1 and ap.deleted_at is null
		order by ap.is_default desc, ap.created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list ai providers: %w", err)
	}
	defer rows.Close()

	var out []providerRow
	for rows.Next() {
		row, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []providerRow{}, nil
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProvider(row rowScanner) (*providerRow, error) {
	var p providerRow
	if err := row.Scan(
		&p.ID, &p.ProjectID, &p.Name, &p.ProviderType, &p.BaseURL, &p.APIKey,
		&p.DefaultModel, &p.ExtraConfig, &p.Enabled, &p.IsDefault,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}
