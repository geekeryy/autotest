package testcase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateManual(ctx context.Context, input CreateManualInput) (*TestCase, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if len(input.Request) == 0 {
		input.Request = []byte(`{}`)
	}
	if len(input.Assertions) == 0 {
		input.Assertions = []byte(`[]`)
	}

	fingerprint := NewFingerprint(input.ProjectID.String(), input.ServiceID.String(), string(SourceManual), input.Method, input.Path, input.Name, CompactJSON(input.Request))
	row := r.DB.QueryRow(ctx, `
		insert into test_cases (
			project_id, service_id, endpoint_id, source, name, method, path,
			fingerprint, request, assertions, status
		)
		select p.id, s.id, $3, $4, $5, upper($6), $7, $8, $9, $10, $11
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, service_id, endpoint_id, source, name, method, path,
			fingerprint, generation_rule_id, request, assertions, last_response_snapshot, status,
			created_at, updated_at
	`, input.ProjectID, input.ServiceID, input.EndpointID, SourceManual, input.Name, input.Method, input.Path, fingerprint, input.Request, input.Assertions, StatusActive)

	return scanCase(row)
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]TestCase, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	query := `
		select tc.id, tc.project_id, tc.service_id, tc.endpoint_id, tc.source, tc.name,
			tc.method, tc.path, tc.fingerprint, tc.generation_rule_id, tc.request,
			tc.assertions, tc.last_response_snapshot, tc.status, tc.created_at, tc.updated_at
		from test_cases tc
		join projects p on p.id = tc.project_id and p.deleted_at is null
		join services s on s.id = tc.service_id and s.deleted_at is null
	`
	var args []any
	where := []string{"tc.deleted_at is null"}
	if filter.ProjectID.String() != "00000000-0000-0000-0000-000000000000" {
		args = append(args, filter.ProjectID)
		where = append(where, fmt.Sprintf("tc.project_id = $%d", len(args)))
	}
	if filter.ServiceID.String() != "00000000-0000-0000-0000-000000000000" {
		args = append(args, filter.ServiceID)
		where = append(where, fmt.Sprintf("tc.service_id = $%d", len(args)))
	}
	query += " where " + strings.Join(where, " and ")
	query += " order by tc.created_at desc"

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list test cases: %w", err)
	}
	defer rows.Close()

	var cases []TestCase
	for rows.Next() {
		tc, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, *tc)
	}
	return cases, rows.Err()
}

func (r *Repository) Get(ctx context.Context, testCaseID uuid.UUID) (*TestCase, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		select tc.id, tc.project_id, tc.service_id, tc.endpoint_id, tc.source, tc.name,
			tc.method, tc.path, tc.fingerprint, tc.generation_rule_id, tc.request,
			tc.assertions, tc.last_response_snapshot, tc.status, tc.created_at, tc.updated_at
		from test_cases tc
		join projects p on p.id = tc.project_id and p.deleted_at is null
		join services s on s.id = tc.service_id and s.deleted_at is null
		where tc.id = $1 and tc.deleted_at is null
	`, testCaseID)

	tc, err := scanCase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTestCaseNotFound
		}
		return nil, err
	}
	return tc, nil
}

func (r *Repository) UpsertGenerated(ctx context.Context, draft Draft) (*TestCase, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if draft.Source == "" {
		draft.Source = SourceAuto
	}
	if draft.Status == "" {
		draft.Status = StatusActive
	}
	if len(draft.Request) == 0 {
		draft.Request = []byte(`{}`)
	}
	if len(draft.Assertions) == 0 {
		draft.Assertions = []byte(`[]`)
	}
	if draft.Fingerprint == "" {
		draft.Fingerprint = NewFingerprint(
			draft.ProjectID.String(),
			draft.ServiceID.String(),
			draft.Source.String(),
			draft.GenerationRuleID,
			draft.Method,
			draft.Path,
			CompactJSON(draft.Request),
		)
	}
	if draft.Source == SourceAuto && draft.EndpointID != nil && *draft.EndpointID != uuid.Nil && draft.GenerationRuleID != "" {
		tc, err := r.updateGeneratedByIdentity(ctx, draft)
		if err == nil {
			return tc, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	row := r.DB.QueryRow(ctx, `
		insert into test_cases (
			project_id, service_id, endpoint_id, source, name, method, path,
			fingerprint, generation_rule_id, request, assertions, status
		)
		select p.id, s.id, $3, $4, $5, upper($6), $7, $8, $9, $10, $11, $12
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		on conflict (fingerprint)
		do update set
			name = excluded.name,
			endpoint_id = excluded.endpoint_id,
			request = excluded.request,
			assertions = excluded.assertions,
			status = excluded.status,
			deleted_at = null,
			updated_at = now()
		returning id, project_id, service_id, endpoint_id, source, name, method, path,
			fingerprint, generation_rule_id, request, assertions, last_response_snapshot, status,
			created_at, updated_at
	`, draft.ProjectID, draft.ServiceID, draft.EndpointID, draft.Source, draft.Name, draft.Method, draft.Path, draft.Fingerprint, draft.GenerationRuleID, draft.Request, draft.Assertions, draft.Status)

	return scanCase(row)
}

func (r *Repository) updateGeneratedByIdentity(ctx context.Context, draft Draft) (*TestCase, error) {
	row := r.DB.QueryRow(ctx, `
		with ranked as (
			select id,
				row_number() over (
					order by (fingerprint = $9) desc, updated_at desc, created_at desc, id desc
				) as rn
			from test_cases
			where project_id = $1
			  and service_id = $2
			  and endpoint_id = $3
			  and source = $4
			  and generation_rule_id = $5
			  and deleted_at is null
		),
		archived as (
			update test_cases
			set deleted_at = now(), updated_at = now()
			where id in (select id from ranked where rn > 1)
		)
		update test_cases
		set name = $6,
			method = upper($7),
			path = $8,
			fingerprint = $9,
			request = $10,
			assertions = $11,
			status = $12,
			updated_at = now()
		where id = (select id from ranked where rn = 1)
		returning id, project_id, service_id, endpoint_id, source, name, method, path,
			fingerprint, generation_rule_id, request, assertions, last_response_snapshot, status,
			created_at, updated_at
	`, draft.ProjectID, draft.ServiceID, *draft.EndpointID, draft.Source, draft.GenerationRuleID, draft.Name, draft.Method, draft.Path, draft.Fingerprint, draft.Request, draft.Assertions, draft.Status)

	return scanCase(row)
}

func (r *Repository) SaveRunSnapshot(ctx context.Context, testCaseID uuid.UUID, request json.RawMessage, response json.RawMessage) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if len(request) == 0 {
		request = []byte(`{}`)
	}
	if len(response) == 0 {
		response = []byte(`{}`)
	}

	tag, err := r.DB.Exec(ctx, `
		update test_cases
		set request = $2,
			last_response_snapshot = $3,
			updated_at = now()
		where id = $1 and deleted_at is null
	`, testCaseID, request, response)
	if err != nil {
		return fmt.Errorf("save case run snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTestCaseNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCase(row rowScanner) (*TestCase, error) {
	var tc TestCase
	if err := row.Scan(
		&tc.ID,
		&tc.ProjectID,
		&tc.ServiceID,
		&tc.EndpointID,
		&tc.Source,
		&tc.Name,
		&tc.Method,
		&tc.Path,
		&tc.Fingerprint,
		&tc.GenerationRuleID,
		&tc.Request,
		&tc.Assertions,
		&tc.LastResponse,
		&tc.Status,
		&tc.CreatedAt,
		&tc.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan test case: %w", err)
	}
	return &tc, nil
}

func (s Source) String() string {
	return string(s)
}
