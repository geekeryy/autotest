package mockset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository persists project-scoped mock value sets in PostgreSQL.
type Repository struct {
	store.Repository
}

// NewRepository constructs a Repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// ListByProject returns all active mock value sets for the project, ordered
// by key ascending so the management UI presents a stable list.
func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]ValueSet, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, project_id, key, name, description, values, weights, created_at, updated_at
		from mock_value_sets
		where project_id = $1 and deleted_at is null
		order by key asc, created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list mock value sets: %w", err)
	}
	defer rows.Close()

	out := []ValueSet{}
	for rows.Next() {
		set, err := scanValueSet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *set)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetByKey returns a single active set keyed by (projectID, key). Returns
// ErrNotFound when no active row matches.
func (r *Repository) GetByKey(ctx context.Context, projectID uuid.UUID, key string) (*ValueSet, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, key, name, description, values, weights, created_at, updated_at
		from mock_value_sets
		where project_id = $1 and key = $2 and deleted_at is null
	`, projectID, key)
	set, err := scanValueSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return set, nil
}

// Get returns a single active set by id within the given project.
func (r *Repository) Get(ctx context.Context, projectID, setID uuid.UUID) (*ValueSet, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, key, name, description, values, weights, created_at, updated_at
		from mock_value_sets
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, setID)
	set, err := scanValueSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return set, nil
}

// Create inserts a new active record.
func (r *Repository) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*ValueSet, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	valuesJSON, err := json.Marshal(input.Values)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}
	weightsJSON, err := json.Marshal(input.Weights)
	if err != nil {
		return nil, fmt.Errorf("marshal weights: %w", err)
	}
	row := r.DB.QueryRow(ctx, `
		insert into mock_value_sets (project_id, key, name, description, values, weights)
		select p.id, $2, $3, $4, $5::jsonb, $6::jsonb
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, key, name, description, values, weights, created_at, updated_at
	`, projectID, input.Key, input.Name, input.Description, valuesJSON, weightsJSON)
	set, err := scanValueSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("project not found or inaccessible")
		}
		return nil, mapPGError(err)
	}
	return set, nil
}

// Update mutates an existing active record. key 不可变，故无 key 字段。
func (r *Repository) Update(ctx context.Context, projectID, setID uuid.UUID, input UpdateInput) (*ValueSet, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	valuesJSON, err := json.Marshal(input.Values)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}
	weightsJSON, err := json.Marshal(input.Weights)
	if err != nil {
		return nil, fmt.Errorf("marshal weights: %w", err)
	}
	row := r.DB.QueryRow(ctx, `
		update mock_value_sets
		set name = $3,
		    description = $4,
		    values = $5::jsonb,
		    weights = $6::jsonb,
		    updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
		returning id, project_id, key, name, description, values, weights, created_at, updated_at
	`, projectID, setID, input.Name, input.Description, valuesJSON, weightsJSON)
	set, err := scanValueSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, mapPGError(err)
	}
	return set, nil
}

// Delete soft-deletes a record so callers may immediately re-create with the
// same key (the unique partial index excludes soft-deleted rows).
func (r *Repository) Delete(ctx context.Context, projectID, setID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update mock_value_sets
		set deleted_at = now(), updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, setID)
	if err != nil {
		return fmt.Errorf("delete mock value set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanValueSet(row rowScanner) (*ValueSet, error) {
	var (
		set         ValueSet
		valuesRaw   []byte
		weightsRaw  []byte
		description *string
	)
	if err := row.Scan(
		&set.ID, &set.ProjectID, &set.Key, &set.Name, &description,
		&valuesRaw, &weightsRaw, &set.CreatedAt, &set.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if description != nil {
		set.Description = *description
	}
	if len(valuesRaw) > 0 {
		if err := json.Unmarshal(valuesRaw, &set.Values); err != nil {
			return nil, fmt.Errorf("decode mock value set values: %w", err)
		}
	}
	if set.Values == nil {
		set.Values = []json.RawMessage{}
	}
	if len(weightsRaw) > 0 {
		if err := json.Unmarshal(weightsRaw, &set.Weights); err != nil {
			return nil, fmt.Errorf("decode mock value set weights: %w", err)
		}
	}
	return &set, nil
}

// mapPGError converts unique-constraint violations on the (project_id, key)
// partial index into ErrKeyConflict so the handler can return 409 Conflict.
func mapPGError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "mock_value_sets_project_key") {
		return ErrKeyConflict
	}
	return err
}
