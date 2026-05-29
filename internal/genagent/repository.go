package genagent

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PGRepository persists jobs in scenario_gen_jobs.
type PGRepository struct {
	store.Repository
}

func NewPGRepository(repo store.Repository) *PGRepository {
	return &PGRepository{Repository: repo}
}

func (r *PGRepository) Create(ctx context.Context, cfg RunConfig) (*Job, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rawCfg, _ := json.Marshal(cfg)
	var createdBy *uuid.UUID
	if cfg.CreatedBy != uuid.Nil {
		createdBy = &cfg.CreatedBy
	}
	row := r.DB.QueryRow(ctx, `
		insert into scenario_gen_jobs (project_id, service_id, environment_id, config, status, created_by)
		values ($1, $2, $3, $4, $5, $6)
		returning id, project_id, service_id, environment_id, config, status, rounds, result, created_by, created_at, updated_at
	`, cfg.ProjectID, cfg.ServiceID, cfg.EnvironmentID, rawCfg, JobPending, createdBy)
	return scanJob(row)
}

func (r *PGRepository) Update(ctx context.Context, jobID uuid.UUID, status JobStatus, rounds int, result json.RawMessage) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		update scenario_gen_jobs
		set status = $2, rounds = $3, result = $4, updated_at = now()
		where id = $1
	`, jobID, status, rounds, nullableJSON(result))
	return err
}

func (r *PGRepository) Get(ctx context.Context, projectID, jobID uuid.UUID) (*Job, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, service_id, environment_id, config, status, rounds, result, created_by, created_at, updated_at
		from scenario_gen_jobs
		where id = $1 and project_id = $2
	`, jobID, projectID)
	return scanJob(row)
}

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	var result []byte
	var createdBy *uuid.UUID
	err := row.Scan(
		&j.ID, &j.ProjectID, &j.ServiceID, &j.EnvironmentID,
		&j.Config, &j.Status, &j.Rounds, &result, &createdBy,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		j.Result = json.RawMessage(result)
	}
	j.CreatedBy = createdBy
	return &j, nil
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
