package authprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists auth_providers records.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

const providerSelectColumns = `
	id, slug, provider_type, name, enabled, client_id, client_secret, scopes,
	extra_config, auto_register, default_active, sort_order, callback_url,
	trusted_frontend_origins, created_at, updated_at
`

func (r *Repository) Create(ctx context.Context, input CreateInput) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	autoRegister := true
	if input.AutoRegister != nil {
		autoRegister = *input.AutoRegister
	}
	defaultActive := false
	if input.DefaultActive != nil {
		defaultActive = *input.DefaultActive
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	scopes := input.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	extra := input.ExtraConfig
	if len(extra) == 0 {
		extra = []byte("{}")
	}
	trustedOrigins := input.TrustedFrontendOrigins
	if trustedOrigins == nil {
		trustedOrigins = []string{}
	}

	row := r.DB.QueryRow(ctx, `
		insert into auth_providers (
			provider_type, slug, name, enabled, client_id, client_secret, scopes,
			extra_config, auto_register, default_active, sort_order, callback_url,
			trusted_frontend_origins
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13)
		returning `+providerSelectColumns+`
	`, input.ProviderType, input.Slug, input.Name, enabled, input.ClientID, input.ClientSecret,
		scopes, extra, autoRegister, defaultActive, sortOrder, input.CallbackURL, trustedOrigins)
	return scanProviderRow(row)
}

func (r *Repository) Update(ctx context.Context, providerID uuid.UUID, input UpdateInput) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	autoRegister := true
	if input.AutoRegister != nil {
		autoRegister = *input.AutoRegister
	}
	defaultActive := false
	if input.DefaultActive != nil {
		defaultActive = *input.DefaultActive
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	scopes := input.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	extra := input.ExtraConfig
	if len(extra) == 0 {
		extra = []byte("{}")
	}
	trustedOrigins := input.TrustedFrontendOrigins
	if trustedOrigins == nil {
		trustedOrigins = []string{}
	}

	keepSecret := input.ClientSecret == nil
	var secretArg any
	if !keepSecret {
		secretArg = *input.ClientSecret
	}

	row := r.DB.QueryRow(ctx, `
		update auth_providers ap
		set slug = $2,
		    name = $3,
		    client_id = $4,
		    client_secret = case when $11::boolean then ap.client_secret else coalesce($5::text, '') end,
		    scopes = $6,
		    extra_config = $7::jsonb,
		    auto_register = $8,
		    default_active = $9,
		    enabled = $10,
		    sort_order = $12,
		    callback_url = $13,
		    trusted_frontend_origins = $14,
		    updated_at = now()
		where ap.id = $1 and ap.deleted_at is null
		returning `+providerSelectColumns+`
	`, providerID, input.Slug, input.Name, input.ClientID, secretArg, scopes, extra,
		autoRegister, defaultActive, enabled, keepSecret, sortOrder, input.CallbackURL, trustedOrigins)

	out, err := scanProviderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, providerID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update auth_providers
		set deleted_at = now(), updated_at = now()
		where id = $1 and deleted_at is null
	`, providerID)
	if err != nil {
		return fmt.Errorf("delete auth provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select `+providerSelectColumns+`
		from auth_providers
		where deleted_at is null
		order by sort_order asc, created_at asc
	`)
	if err != nil {
		return nil, fmt.Errorf("list auth providers: %w", err)
	}
	defer rows.Close()
	return scanProviderRows(rows)
}

func (r *Repository) ListEnabled(ctx context.Context) ([]providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select `+providerSelectColumns+`
		from auth_providers
		where deleted_at is null and enabled = true
		order by sort_order asc, created_at asc
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled auth providers: %w", err)
	}
	defer rows.Close()
	return scanProviderRows(rows)
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select `+providerSelectColumns+`
		from auth_providers
		where slug = $1 and deleted_at is null
	`, slug)
	out, err := scanProviderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) SlugTaken(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	if r.DB == nil {
		return false, fmt.Errorf("database unavailable")
	}
	var exists bool
	if excludeID != nil {
		err := r.DB.QueryRow(ctx, `
			select exists(
				select 1 from auth_providers
				where deleted_at is null and slug = $1 and id <> $2
			)
		`, slug, *excludeID).Scan(&exists)
		return exists, err
	}
	err := r.DB.QueryRow(ctx, `
		select exists(
			select 1 from auth_providers
			where deleted_at is null and slug = $1
		)
	`, slug).Scan(&exists)
	return exists, err
}

func (r *Repository) GetByID(ctx context.Context, providerID uuid.UUID) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select `+providerSelectColumns+`
		from auth_providers
		where id = $1 and deleted_at is null
	`, providerID)
	out, err := scanProviderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) HasEnabledProviderType(ctx context.Context, providerType string) (bool, error) {
	if r.DB == nil {
		return false, fmt.Errorf("database unavailable")
	}
	var exists bool
	err := r.DB.QueryRow(ctx, `
		select exists(
			select 1 from auth_providers
			where deleted_at is null and enabled = true and provider_type = $1
		)
	`, providerType).Scan(&exists)
	return exists, err
}

func (r *Repository) FirstEnabledByType(ctx context.Context, providerType string) (*providerRow, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select `+providerSelectColumns+`
		from auth_providers
		where deleted_at is null and enabled = true and provider_type = $1
		order by sort_order asc, created_at asc
		limit 1
	`, providerType)
	out, err := scanProviderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProviderRow(row rowScanner) (*providerRow, error) {
	var p providerRow
	var extra []byte
	if err := row.Scan(
		&p.ID, &p.Slug, &p.ProviderType, &p.Name, &p.Enabled, &p.ClientID, &p.ClientSecret,
		&p.Scopes, &extra, &p.AutoRegister, &p.DefaultActive, &p.SortOrder, &p.CallbackURL,
		&p.TrustedFrontendOrigins, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan auth provider: %w", err)
	}
	p.ExtraConfig = extra
	return &p, nil
}

func scanProviderRows(rows pgx.Rows) ([]providerRow, error) {
	var out []providerRow
	for rows.Next() {
		var p providerRow
		var extra []byte
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.ProviderType, &p.Name, &p.Enabled, &p.ClientID, &p.ClientSecret,
			&p.Scopes, &extra, &p.AutoRegister, &p.DefaultActive, &p.SortOrder, &p.CallbackURL,
			&p.TrustedFrontendOrigins, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan auth provider row: %w", err)
		}
		p.ExtraConfig = extra
		out = append(out, p)
	}
	return out, rows.Err()
}

func parseExtraConfig(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err == nil {
		return m
	}
	return out
}

func rowToResolved(row *providerRow) ResolvedProvider {
	return ResolvedProvider{
		ID:                     row.ID.String(),
		Slug:                   row.Slug,
		ProviderType:           row.ProviderType,
		Name:                   row.Name,
		ClientID:               row.ClientID,
		ClientSecret:           row.ClientSecret,
		Scopes:                 row.Scopes,
		ExtraConfig:            parseExtraConfig(row.ExtraConfig),
		CallbackURL:            row.CallbackURL,
		TrustedFrontendOrigins: append([]string(nil), row.TrustedFrontendOrigins...),
		AutoRegister:           row.AutoRegister,
		DefaultActive:          row.DefaultActive,
		Source:                 SourceDB,
	}
}
