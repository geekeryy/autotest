package testprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PGRepository implements Repository using PostgreSQL.
type PGRepository struct {
	store.Repository
}

func NewPGRepository(repo store.Repository) *PGRepository {
	return &PGRepository{Repository: repo}
}

func (r *PGRepository) Get(ctx context.Context, projectID uuid.UUID) (*Profile, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, profile_data, updated_at
		FROM project_test_profiles
		WHERE project_id = $1
	`, projectID)

	var p Profile
	var data []byte
	err := row.Scan(&p.ID, &p.ProjectID, &data, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get test profile: %w", err)
	}

	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal test profile: %w", err)
	}
	return &p, nil
}

func (r *PGRepository) Upsert(ctx context.Context, profile *Profile) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal test profile: %w", err)
	}

	_, err = r.DB.Exec(ctx, `
		INSERT INTO project_test_profiles (id, project_id, profile_data, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id)
		DO UPDATE SET profile_data = EXCLUDED.profile_data, updated_at = EXCLUDED.updated_at
	`, profile.ID, profile.ProjectID, data, profile.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert test profile: %w", err)
	}
	return nil
}
