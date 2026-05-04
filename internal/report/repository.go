package report

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrRunNotFound = errors.New("test run not found")

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateRun(ctx context.Context, input CreateRunInput) (*Run, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if len(input.Variables) == 0 {
		input.Variables = []byte(`{}`)
	}
	if len(input.Snapshot) == 0 {
		input.Snapshot = []byte(`{}`)
	}

	row := r.DB.QueryRow(ctx, `
		insert into test_runs (
			name, project_id, service_id, suite_id, environment_id, status,
			variables, snapshot, started_at
		)
		select $1, p.id, s.id, $4, e.id, $6, $7, $8, now()
		from projects p
		join services s on s.project_id = p.id and s.id = $3 and s.deleted_at is null
		join environments e on e.project_id = p.id and e.service_id = s.id and e.id = $5 and e.deleted_at is null
		where p.id = $2 and p.deleted_at is null
		returning id, name, project_id, service_id, suite_id, environment_id, status,
			variables, snapshot, started_at, finished_at, created_at
	`, input.Name, input.ProjectID, input.ServiceID, input.SuiteID, input.EnvironmentID, RunRunning, input.Variables, input.Snapshot)

	run, err := scanRun(row)
	if err != nil {
		return nil, fmt.Errorf("create test run: %w", err)
	}
	return run, nil
}

func (r *Repository) FinishRun(ctx context.Context, runID uuid.UUID, status RunStatus) (*Run, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		update test_runs
		set status = $2, finished_at = now()
		where id = $1 and deleted_at is null
		returning id, name, project_id, service_id, suite_id, environment_id, status,
			variables, snapshot, started_at, finished_at, created_at
	`, runID, status)

	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("finish test run: %w", err)
	}
	return run, nil
}

func (r *Repository) GetRun(ctx context.Context, runID uuid.UUID) (*Run, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		select id, name, project_id, service_id, suite_id, environment_id, status,
			variables, snapshot, started_at, finished_at, created_at
		from test_runs
		where id = $1 and deleted_at is null
	`, runID)

	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("get test run: %w", err)
	}
	return run, nil
}

func (r *Repository) ListResults(ctx context.Context, runID uuid.UUID) ([]Result, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	rows, err := r.DB.Query(ctx, `
		select rr.id, rr.run_id, rr.test_case_id, rr.step_id, rr.status,
			rr.duration_millis, rr.request_snapshot, rr.response_snapshot,
			rr.assertions, rr.error, rr.created_at
		from test_run_results rr
		join test_runs tr on tr.id = rr.run_id and tr.deleted_at is null
		where rr.run_id = $1 and rr.deleted_at is null
		order by rr.created_at asc
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list run results: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		result, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, rows.Err()
}

func (r *Repository) SaveResult(ctx context.Context, result Result) (*Result, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		insert into test_run_results (
			run_id, test_case_id, step_id, status, duration_millis,
			request_snapshot, response_snapshot, assertions, error
		)
		select tr.id, tc.id, $3, $4, $5, $6, $7, $8, $9
		from test_runs tr
		join test_cases tc on tc.id = $2 and tc.deleted_at is null
		where tr.id = $1 and tr.deleted_at is null
		returning id, run_id, test_case_id, step_id, status, duration_millis,
			request_snapshot, response_snapshot, assertions, error, created_at
	`, result.RunID, result.TestCaseID, result.StepID, result.Status, result.DurationMillis, result.RequestSnapshot, result.ResponseSnapshot, result.Assertions, result.Error)

	saved, err := scanResult(row)
	if err != nil {
		return nil, fmt.Errorf("save run result: %w", err)
	}
	return saved, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (*Run, error) {
	var run Run
	if err := row.Scan(
		&run.ID,
		&run.Name,
		&run.ProjectID,
		&run.ServiceID,
		&run.SuiteID,
		&run.EnvironmentID,
		&run.Status,
		&run.Variables,
		&run.Snapshot,
		&run.StartedAt,
		&run.FinishedAt,
		&run.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &run, nil
}

func scanResult(row rowScanner) (*Result, error) {
	var result Result
	if err := row.Scan(
		&result.ID,
		&result.RunID,
		&result.TestCaseID,
		&result.StepID,
		&result.Status,
		&result.DurationMillis,
		&result.RequestSnapshot,
		&result.ResponseSnapshot,
		&result.Assertions,
		&result.Error,
		&result.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan run result: %w", err)
	}
	return &result, nil
}
