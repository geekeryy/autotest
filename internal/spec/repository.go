package spec

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
)

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateSpec(ctx context.Context, projectID, serviceID uuid.UUID, result *ImportResult, raw []byte) (*APISpec, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		insert into api_specs (service_id, version, content_hash, raw_content, normalized_snapshot, status)
		select
			s.id,
			coalesce((select max(version) + 1 from api_specs where service_id = $2 and deleted_at is null), 1),
			$3,
			$4,
			$5,
			'imported'
		from services s
		join projects p on p.id = s.project_id and p.id = $1 and p.deleted_at is null
		where s.id = $2 and s.deleted_at is null
		on conflict (service_id, content_hash)
		do update set
			raw_content = excluded.raw_content,
			normalized_snapshot = excluded.normalized_snapshot,
			status = excluded.status,
			deleted_at = null
		returning id, service_id, version, content_hash, raw_content, normalized_snapshot, status, created_at
	`, projectID, serviceID, result.Hash, raw, result.Snapshot)

	var apiSpec APISpec
	if err := row.Scan(&apiSpec.ID, &apiSpec.ServiceID, &apiSpec.Version, &apiSpec.ContentHash, &apiSpec.RawContent, &apiSpec.NormalizedSnapshot, &apiSpec.Status, &apiSpec.CreatedAt); err != nil {
		return nil, fmt.Errorf("create api spec: %w", err)
	}
	return &apiSpec, nil
}

func (r *Repository) ListSpecs(ctx context.Context, projectID, serviceID uuid.UUID) ([]APISpec, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	rows, err := r.DB.Query(ctx, `
		select sp.id, sp.service_id, sp.version, sp.content_hash, sp.raw_content,
			sp.normalized_snapshot, sp.status, sp.created_at
		from api_specs sp
		join services s on s.id = sp.service_id and s.deleted_at is null
		join projects p on p.id = s.project_id and p.deleted_at is null
		where p.id = $1 and sp.service_id = $2 and sp.deleted_at is null
		order by sp.version desc
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list api specs: %w", err)
	}
	defer rows.Close()

	var specs []APISpec
	for rows.Next() {
		var apiSpec APISpec
		if err := rows.Scan(&apiSpec.ID, &apiSpec.ServiceID, &apiSpec.Version, &apiSpec.ContentHash, &apiSpec.RawContent, &apiSpec.NormalizedSnapshot, &apiSpec.Status, &apiSpec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api spec: %w", err)
		}
		specs = append(specs, apiSpec)
	}
	return specs, rows.Err()
}

func (r *Repository) SyncEndpoints(ctx context.Context, projectID, serviceID, specID uuid.UUID, endpoints []Endpoint) ([]Endpoint, int, int, error) {
	if r.DB == nil {
		return nil, 0, 0, fmt.Errorf("database unavailable")
	}

	synced := make([]Endpoint, 0, len(endpoints))
	var created, updated int
	for _, endpoint := range endpoints {
		endpoint.ServiceID = serviceID
		endpoint.SpecID = specID

		row := r.DB.QueryRow(ctx, `
			insert into api_endpoints (
				service_id, spec_id, method, path, operation_id, summary,
				tags, request_schema, response_schema, fingerprint
			)
			select s.id, sp.id, upper($4), $5, $6, $7, $8, $9, $10, $11
			from services s
			join projects p on p.id = s.project_id and p.id = $1 and p.deleted_at is null
			join api_specs sp on sp.id = $3 and sp.service_id = s.id and sp.deleted_at is null
			where s.id = $2 and s.deleted_at is null
			on conflict (service_id, method, path)
			do update set
				spec_id = excluded.spec_id,
				operation_id = excluded.operation_id,
				summary = excluded.summary,
				tags = excluded.tags,
				request_schema = excluded.request_schema,
				response_schema = excluded.response_schema,
				fingerprint = excluded.fingerprint,
				deleted_at = null,
				updated_at = now()
			returning id, service_id, spec_id, method, path, operation_id, summary, tags,
				request_schema, response_schema, fingerprint, created_at, updated_at,
				(xmax = 0) AS inserted
		`, projectID, serviceID, specID, endpoint.Method, endpoint.Path, endpoint.OperationID, endpoint.Summary, endpoint.Tags, endpoint.RequestSchema, endpoint.ResponseSchema, endpoint.Fingerprint)

		var saved Endpoint
		var inserted bool
		if err := row.Scan(
			&saved.ID,
			&saved.ServiceID,
			&saved.SpecID,
			&saved.Method,
			&saved.Path,
			&saved.OperationID,
			&saved.Summary,
			&saved.Tags,
			&saved.RequestSchema,
			&saved.ResponseSchema,
			&saved.Fingerprint,
			&saved.CreatedAt,
			&saved.UpdatedAt,
			&inserted,
		); err != nil {
			return nil, 0, 0, fmt.Errorf("sync endpoint %s %s: %w", endpoint.Method, endpoint.Path, err)
		}
		if inserted {
			created++
		} else {
			updated++
		}
		synced = append(synced, saved)
	}
	return synced, created, updated, nil
}

// GetEndpointRequestSchema returns only the request_schema for the given endpoint.
// It satisfies the testcase.EndpointSchemaGetter interface without creating an import cycle.
func (r *Repository) GetEndpointRequestSchema(ctx context.Context, endpointID uuid.UUID) (json.RawMessage, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var schema json.RawMessage
	row := r.DB.QueryRow(ctx, `
		select request_schema from api_endpoints where id = $1 and deleted_at is null
	`, endpointID)
	if err := row.Scan(&schema); err != nil {
		return nil, fmt.Errorf("get endpoint request schema: %w", err)
	}
	return schema, nil
}

func (r *Repository) GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*Endpoint, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	row := r.DB.QueryRow(ctx, `
		select e.id, e.service_id, e.spec_id, e.method, e.path, e.operation_id, e.summary, e.tags,
			e.request_schema, e.response_schema, e.fingerprint, e.created_at, e.updated_at
		from api_endpoints e
		where e.id = $1 and e.deleted_at is null
	`, endpointID)

	var endpoint Endpoint
	if err := row.Scan(
		&endpoint.ID,
		&endpoint.ServiceID,
		&endpoint.SpecID,
		&endpoint.Method,
		&endpoint.Path,
		&endpoint.OperationID,
		&endpoint.Summary,
		&endpoint.Tags,
		&endpoint.RequestSchema,
		&endpoint.ResponseSchema,
		&endpoint.Fingerprint,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get endpoint by id: %w", err)
	}
	return &endpoint, nil
}

func (r *Repository) ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]Endpoint, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	rows, err := r.DB.Query(ctx, `
		select e.id, e.service_id, e.spec_id, e.method, e.path, e.operation_id, e.summary, e.tags,
			e.request_schema, e.response_schema, e.fingerprint, e.created_at, e.updated_at
		from api_endpoints e
		join services s on s.id = e.service_id and s.deleted_at is null
		join projects p on p.id = s.project_id and p.deleted_at is null
		where p.id = $1 and e.service_id = $2 and e.deleted_at is null
		order by e.path asc, e.method asc
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		var endpoint Endpoint
		if err := rows.Scan(
			&endpoint.ID,
			&endpoint.ServiceID,
			&endpoint.SpecID,
			&endpoint.Method,
			&endpoint.Path,
			&endpoint.OperationID,
			&endpoint.Summary,
			&endpoint.Tags,
			&endpoint.RequestSchema,
			&endpoint.ResponseSchema,
			&endpoint.Fingerprint,
			&endpoint.CreatedAt,
			&endpoint.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}
