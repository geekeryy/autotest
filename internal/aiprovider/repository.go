package aiprovider

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists platform-wide AI provider configurations.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a new provider record. When IsDefault is true, all other defaults are cleared first.
func (r *Repository) Create(ctx context.Context, input CreateInput) (*providerRow, error) {
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
			where is_default = true and deleted_at is null
		`); err != nil {
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
			name, provider_type, base_url, api_key, default_model,
			extra_config, enabled, is_default
		)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
		returning id, name, provider_type, base_url, api_key, default_model,
		          extra_config, enabled, is_default, created_at, updated_at
	`, input.Name, input.ProviderType, input.BaseURL, input.APIKey, input.DefaultModel,
		extra, enabled, input.IsDefault)

	out, err := scanProvider(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create ai provider: %w", err)
	}
	return out, nil
}

// Update mutates an existing provider record. If apiKey is nil the previous key is preserved.
func (r *Repository) Update(ctx context.Context, providerID uuid.UUID, input UpdateInput) (*providerRow, error) {
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
			where is_default = true and id <> $1 and deleted_at is null
		`, providerID); err != nil {
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
		set name = $2,
		    provider_type = $3,
		    base_url = $4,
		    api_key = case when $10::boolean then ap.api_key else coalesce($5::text, '') end,
		    default_model = $6,
		    extra_config = $7::jsonb,
		    enabled = $8,
		    is_default = $9,
		    updated_at = now()
		where ap.id = $1
		  and ap.deleted_at is null
		returning ap.id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		          ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		          ap.created_at, ap.updated_at
	`, providerID, input.Name, input.ProviderType, input.BaseURL, apiKeyArg,
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
func (r *Repository) Delete(ctx context.Context, providerID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update ai_providers
		set deleted_at = now(), is_default = false, updated_at = now()
		where id = $1 and deleted_at is null
	`, providerID)
	if err != nil {
		return fmt.Errorf("delete ai provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// Get fetches a provider record (including its plaintext key for client construction).
func (r *Repository) Get(ctx context.Context, providerID uuid.UUID) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select ap.id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		       ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		       ap.created_at, ap.updated_at
		from ai_providers ap
		where ap.id = $1 and ap.deleted_at is null
	`, providerID)
	out, err := scanProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

// GetDefaultProvider returns the provider flagged as is_default. When none is
// explicitly defaulted we fall back to the oldest enabled provider.
func (r *Repository) GetDefaultProvider(ctx context.Context) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select ap.id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		       ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		       ap.created_at, ap.updated_at
		from ai_providers ap
		where ap.deleted_at is null and ap.enabled = true
		order by ap.is_default desc, ap.created_at asc
		limit 1
	`)
	out, err := scanProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

// List returns all active platform providers.
func (r *Repository) List(ctx context.Context) ([]providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select ap.id, ap.name, ap.provider_type, ap.base_url, ap.api_key,
		       ap.default_model, ap.extra_config, ap.enabled, ap.is_default,
		       ap.created_at, ap.updated_at
		from ai_providers ap
		where ap.deleted_at is null
		order by ap.is_default desc, ap.created_at asc
	`)
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
		&p.ID, &p.Name, &p.ProviderType, &p.BaseURL, &p.APIKey,
		&p.DefaultModel, &p.ExtraConfig, &p.Enabled, &p.IsDefault,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}
