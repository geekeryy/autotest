package testprofile

import (
	"context"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// PGDataSource implements DataSource using direct SQL queries. This avoids
// import cycles with the case/spec/report packages while keeping queries simple.
type PGDataSource struct {
	store.Repository
}

func NewPGDataSource(repo store.Repository) *PGDataSource {
	return &PGDataSource{Repository: repo}
}

func (ds *PGDataSource) ListEndpoints(ctx context.Context, projectID uuid.UUID) ([]EndpointInfo, error) {
	if ds.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := ds.DB.Query(ctx, `
		SELECT e.id, e.method, e.path, COALESCE(e.operation_id, ''),
		       COALESCE(e.summary, ''), e.tags,
		       COALESCE(e.request_schema, '{}'), COALESCE(e.response_schema, '{}')
		FROM api_endpoints e
		JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
		WHERE s.project_id = $1
		ORDER BY e.path, e.method
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints for profile: %w", err)
	}
	defer rows.Close()

	var endpoints []EndpointInfo
	for rows.Next() {
		var ep EndpointInfo
		
		if err := rows.Scan(&ep.ID, &ep.Method, &ep.Path, &ep.OperationID,
			&ep.Summary, &ep.Tags, &ep.RequestSchema, &ep.ResponseSchema); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}

func (ds *PGDataSource) ListCases(ctx context.Context, projectID uuid.UUID) ([]CaseInfo, error) {
	if ds.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := ds.DB.Query(ctx, `
		SELECT c.id, c.method, c.path, c.source, c.request,
		       COALESCE(c.assertions, '[]'), COALESCE(c.last_response_snapshot, 'null'),
		       (c.source = 'manual' AND c.assertions IS NOT NULL AND c.assertions != '[]' AND c.assertions != 'null') AS has_manual_assertions,
		       COALESCE(run_stats.run_count, 0),
		       COALESCE(run_stats.success_count, 0)
		FROM test_cases c
		LEFT JOIN (
			SELECT test_case_id,
			       COUNT(*) AS run_count,
			       COUNT(*) FILTER (WHERE status = 'passed') AS success_count
			FROM test_run_results
			GROUP BY test_case_id
		) run_stats ON run_stats.test_case_id = c.id
		WHERE c.project_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.path, c.method
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list cases for profile: %w", err)
	}
	defer rows.Close()

	var cases []CaseInfo
	for rows.Next() {
		var c CaseInfo
		if err := rows.Scan(&c.ID, &c.Method, &c.Path, &c.Source, &c.Request,
			&c.Assertions, &c.LastResponseSnapshot, &c.HasManualAssertions,
			&c.RunCount, &c.SuccessCount); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

func (ds *PGDataSource) ListSuccessfulResults(ctx context.Context, projectID uuid.UUID, limit int) ([]RunResultInfo, error) {
	if ds.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := ds.DB.Query(ctx, `
		SELECT r.test_case_id, c.method, c.path, r.status,
		       COALESCE(r.request_snapshot, '{}'), COALESCE(r.response_snapshot, '{}')
		FROM test_run_results r
		JOIN test_cases c ON c.id = r.test_case_id
		JOIN test_runs tr ON tr.id = r.run_id
		WHERE tr.project_id = $1 AND r.status = 'passed'
		ORDER BY r.created_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list successful results for profile: %w", err)
	}
	defer rows.Close()

	var results []RunResultInfo
	for rows.Next() {
		var r RunResultInfo
		if err := rows.Scan(&r.TestCaseID, &r.Method, &r.Path, &r.Status,
			&r.RequestSnapshot, &r.ResponseSnapshot); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
