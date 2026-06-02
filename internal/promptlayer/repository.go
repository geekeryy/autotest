package promptlayer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists prompt layers.
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create inserts a new prompt layer version.
func (r *Repository) Create(ctx context.Context, input CreateLayerInput) (*PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	// Get next version for this name
	version, err := r.getNextVersion(ctx, input.Name)
	if err != nil {
		return nil, err
	}

	row := r.DB.QueryRow(ctx, `
		INSERT INTO ai_prompt_layers (name, domain, content, version)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, domain, content, version, enabled, created_at, updated_at
	`, input.Name, input.Domain, input.Content, version)
	return scanLayer(row)
}

// List returns layers with optional filtering.
func (r *Repository) List(ctx context.Context, opts ListLayersOptions) ([]PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	argIdx := 1

	if opts.Domain != "" {
		where = append(where, fmt.Sprintf("domain = $%d", argIdx))
		args = append(args, opts.Domain)
		argIdx++
	}
	if opts.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *opts.Enabled)
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
		SELECT id, name, domain, content, version, enabled, created_at, updated_at
		FROM ai_prompt_layers
		WHERE %s
		ORDER BY name, version DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list prompt layers: %w", err)
	}
	defer rows.Close()

	out := []PromptLayer{}
	for rows.Next() {
		l, err := scanLayer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

// Get fetches a single layer by ID.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, name, domain, content, version, enabled, created_at, updated_at
		FROM ai_prompt_layers
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	l, err := scanLayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("prompt layer 不存在")
		}
		return nil, err
	}
	return l, nil
}

// GetLatestVersion returns the latest enabled version of a layer by name.
func (r *Repository) GetLatestVersion(ctx context.Context, name string) (*PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, name, domain, content, version, enabled, created_at, updated_at
		FROM ai_prompt_layers
		WHERE name = $1 AND enabled = true AND deleted_at IS NULL
		ORDER BY version DESC
		LIMIT 1
	`, name)
	l, err := scanLayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("prompt layer 不存在")
		}
		return nil, err
	}
	return l, nil
}

// GetByVersion returns a specific version of a layer.
func (r *Repository) GetByVersion(ctx context.Context, name string, version int) (*PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, name, domain, content, version, enabled, created_at, updated_at
		FROM ai_prompt_layers
		WHERE name = $1 AND version = $2 AND deleted_at IS NULL
	`, name, version)
	l, err := scanLayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("prompt layer 不存在")
		}
		return nil, err
	}
	return l, nil
}

// Update modifies a layer (creates a new version).
func (r *Repository) Update(ctx context.Context, name string, input UpdateLayerInput) (*PromptLayer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	// Get current latest version
	current, err := r.GetLatestVersion(ctx, name)
	if err != nil {
		return nil, err
	}

	// Create new version
	newContent := current.Content
	if input.Content != nil {
		newContent = *input.Content
	}
	newEnabled := current.Enabled
	if input.Enabled != nil {
		newEnabled = *input.Enabled
	}

	row := r.DB.QueryRow(ctx, `
		INSERT INTO ai_prompt_layers (name, domain, content, version, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, domain, content, version, enabled, created_at, updated_at
	`, name, current.Domain, newContent, current.Version+1, newEnabled)
	return scanLayer(row)
}

// Delete soft-deletes a layer.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE ai_prompt_layers
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete prompt layer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prompt layer 不存在")
	}
	return nil
}

// EnableVersion enables a specific version and disables others atomically.
func (r *Repository) EnableVersion(ctx context.Context, name string, version int) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE ai_prompt_layers
		SET enabled = (version = $2), updated_at = now()
		WHERE name = $1 AND deleted_at IS NULL
	`, name, version)
	if err != nil {
		return fmt.Errorf("enable layer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prompt layer 不存在")
	}
	// Verify the target version was actually enabled (it might not exist)
	var enabled bool
	err = r.DB.QueryRow(ctx, `
		SELECT enabled FROM ai_prompt_layers
		WHERE name = $1 AND version = $2 AND deleted_at IS NULL
	`, name, version).Scan(&enabled)
	if err != nil {
		return fmt.Errorf("prompt layer version 不存在")
	}
	if !enabled {
		return fmt.Errorf("prompt layer version 不存在")
	}
	return nil
}

func (r *Repository) getNextVersion(ctx context.Context, name string) (int, error) {
	var version int
	err := r.DB.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM ai_prompt_layers
		WHERE name = $1 AND deleted_at IS NULL
	`, name).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("get next version: %w", err)
	}
	return version, nil
}

func scanLayer(row rowScanner) (*PromptLayer, error) {
	var l PromptLayer
	if err := row.Scan(&l.ID, &l.Name, &l.Domain, &l.Content, &l.Version, &l.Enabled, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

// --- Experiment methods ---

// CreateExperiment inserts a new A/B test experiment.
func (r *Repository) CreateExperiment(ctx context.Context, input CreateExperimentInput) (*PromptExperiment, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var variantA, variantB *uuid.UUID
	if input.VariantAID != "" {
		id, err := uuid.Parse(input.VariantAID)
		if err != nil {
			return nil, fmt.Errorf("invalid variantAId: %w", err)
		}
		variantA = &id
	}
	if input.VariantBID != "" {
		id, err := uuid.Parse(input.VariantBID)
		if err != nil {
			return nil, fmt.Errorf("invalid variantBId: %w", err)
		}
		variantB = &id
	}
	split := input.TrafficSplit
	if split <= 0 || split >= 1 {
		split = 0.5
	}
	metric := input.Metric
	if metric == "" {
		metric = "success_rate"
	}

	e := &PromptExperiment{}
	err := r.DB.QueryRow(ctx, `
		INSERT INTO ai_prompt_experiments (domain, layer_name, variant_a_id, variant_b_id, traffic_split, metric)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, domain, layer_name, variant_a_id, variant_b_id, traffic_split, metric,
		          variant_a_val, variant_b_val, status, started_at, ended_at
	`, input.Domain, input.LayerName, variantA, variantB, split, metric).Scan(
		&e.ID, &e.Domain, &e.LayerName, &e.VariantAID, &e.VariantBID,
		&e.TrafficSplit, &e.Metric, &e.VariantAVal, &e.VariantBVal,
		&e.Status, &e.StartedAt, &e.EndedAt,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// ListExperiments returns all experiments with the given status.
func (r *Repository) ListExperiments(ctx context.Context, status string) ([]PromptExperiment, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	q := `SELECT id, domain, layer_name, variant_a_id, variant_b_id, traffic_split, metric,
	             variant_a_val, variant_b_val, status, started_at, ended_at
	      FROM ai_prompt_experiments WHERE deleted_at IS NULL`
	args := []any{}
	if status != "" {
		q += ` AND status = $1`
		args = append(args, status)
	}
	q += ` ORDER BY started_at DESC`
	rows, err := r.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PromptExperiment
	for rows.Next() {
		var e PromptExperiment
		if err := rows.Scan(&e.ID, &e.Domain, &e.LayerName, &e.VariantAID, &e.VariantBID,
			&e.TrafficSplit, &e.Metric, &e.VariantAVal, &e.VariantBVal,
			&e.Status, &e.StartedAt, &e.EndedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StopExperiment stops a running experiment.
func (r *Repository) StopExperiment(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE ai_prompt_experiments SET status = 'stopped', ended_at = now()
		WHERE id = $1 AND status = 'running' AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("experiment not found or already stopped")
	}
	return nil
}
