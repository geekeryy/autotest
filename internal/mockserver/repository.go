package mockserver

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists mock server configuration and routes.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// CreateServer creates a mock server inside a project.
func (r *Repository) CreateServer(ctx context.Context, projectID uuid.UUID, input MockServerInput) (*MockServer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into mock_servers (project_id, name, description, port)
		select p.id, $2, $3, $4
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, name, description, port, created_at, updated_at
	`, projectID, input.Name, input.Description, input.Port)
	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("project not found or inaccessible")
		}
		return nil, fmt.Errorf("create mock server: %w", err)
	}
	return server, nil
}

// ListServers returns active mock servers in a project.
func (r *Repository) ListServers(ctx context.Context, projectID uuid.UUID) ([]MockServer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select ms.id, ms.project_id, ms.name, ms.description, ms.port, ms.created_at, ms.updated_at
		from mock_servers ms
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where ms.project_id = $1 and ms.deleted_at is null
		order by ms.created_at desc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list mock servers: %w", err)
	}
	defer rows.Close()
	return scanServers(rows)
}

// GetServer returns a mock server by project and server ID.
func (r *Repository) GetServer(ctx context.Context, projectID, serverID uuid.UUID) (*MockServer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select ms.id, ms.project_id, ms.name, ms.description, ms.port, ms.created_at, ms.updated_at
		from mock_servers ms
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where ms.project_id = $1 and ms.id = $2 and ms.deleted_at is null
	`, projectID, serverID)
	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMockServerNotFound
		}
		return nil, err
	}
	return server, nil
}

// UpdateServer updates a mock server in a project.
func (r *Repository) UpdateServer(ctx context.Context, projectID, serverID uuid.UUID, input MockServerInput) (*MockServer, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update mock_servers ms
		set name = $3,
			description = $4,
			port = $5,
			updated_at = now()
		from projects p
		where ms.project_id = $1
		  and ms.id = $2
		  and p.id = ms.project_id
		  and p.deleted_at is null
		  and ms.deleted_at is null
		returning ms.id, ms.project_id, ms.name, ms.description, ms.port, ms.created_at, ms.updated_at
	`, projectID, serverID, input.Name, input.Description, input.Port)
	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMockServerNotFound
		}
		return nil, fmt.Errorf("update mock server: %w", err)
	}
	return server, nil
}

// DeleteServer soft-deletes a mock server in a project.
func (r *Repository) DeleteServer(ctx context.Context, projectID, serverID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update mock_servers
		set deleted_at = now(), updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, serverID)
	if err != nil {
		return fmt.Errorf("delete mock server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMockServerNotFound
	}
	return nil
}

// CreateRoute creates a mock route for a project-owned server.
func (r *Repository) CreateRoute(ctx context.Context, projectID, serverID uuid.UUID, input MockRouteInput) (*MockRoute, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into mock_routes (
			mock_server_id, method, path, priority, enabled, request_match,
			response_status, response_headers, response_body, response_body_type, delay_millis
		)
		select ms.id, upper($3), $4, $5, $6, $7, $8, $9, $10, $11, $12
		from mock_servers ms
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where ms.project_id = $1 and ms.id = $2 and ms.deleted_at is null
		returning id, mock_server_id, method, path, priority, enabled, request_match,
			response_status, response_headers, response_body, response_body_type,
			delay_millis, created_at, updated_at
	`, projectID, serverID, input.Method, input.Path, input.Priority, boolValue(input.Enabled, true),
		defaultJSON(input.RequestMatch, "{}"), input.ResponseStatus, defaultJSON(input.ResponseHeaders, "{}"),
		input.ResponseBody, input.ResponseBodyType, input.DelayMillis)
	route, err := scanRoute(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMockServerNotFound
		}
		return nil, err
	}
	return route, nil
}

// ListRoutes returns active routes for a project-owned server.
func (r *Repository) ListRoutes(ctx context.Context, projectID, serverID uuid.UUID) ([]MockRoute, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select mr.id, mr.mock_server_id, mr.method, mr.path, mr.priority, mr.enabled, mr.request_match,
			mr.response_status, mr.response_headers, mr.response_body, mr.response_body_type,
			mr.delay_millis, mr.created_at, mr.updated_at
		from mock_routes mr
		join mock_servers ms on ms.id = mr.mock_server_id and ms.deleted_at is null
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where ms.project_id = $1 and ms.id = $2 and mr.deleted_at is null
		order by mr.priority desc, mr.created_at asc
	`, projectID, serverID)
	if err != nil {
		return nil, fmt.Errorf("list mock routes: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// ListEnabledRoutesForServer returns enabled runtime routes for a running mock server.
func (r *Repository) ListEnabledRoutesForServer(ctx context.Context, serverID uuid.UUID) ([]MockRoute, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select mr.id, mr.mock_server_id, mr.method, mr.path, mr.priority, mr.enabled, mr.request_match,
			mr.response_status, mr.response_headers, mr.response_body, mr.response_body_type,
			mr.delay_millis, mr.created_at, mr.updated_at
		from mock_routes mr
		join mock_servers ms on ms.id = mr.mock_server_id and ms.deleted_at is null
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where mr.mock_server_id = $1
		  and mr.deleted_at is null
		  and mr.enabled
		order by mr.priority desc, mr.created_at asc
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list runtime mock routes: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// UpdateRoute updates a mock route for a project-owned server.
func (r *Repository) UpdateRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID, input MockRouteInput) (*MockRoute, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update mock_routes mr
		set method = upper($4),
			path = $5,
			priority = $6,
			enabled = $7,
			request_match = $8,
			response_status = $9,
			response_headers = $10,
			response_body = $11,
			response_body_type = $12,
			delay_millis = $13,
			updated_at = now()
		from mock_servers ms
		join projects p on p.id = ms.project_id and p.deleted_at is null
		where mr.id = $3
		  and mr.mock_server_id = ms.id
		  and ms.project_id = $1
		  and ms.id = $2
		  and ms.deleted_at is null
		  and mr.deleted_at is null
		returning mr.id, mr.mock_server_id, mr.method, mr.path, mr.priority, mr.enabled, mr.request_match,
			mr.response_status, mr.response_headers, mr.response_body, mr.response_body_type,
			mr.delay_millis, mr.created_at, mr.updated_at
	`, projectID, serverID, routeID, input.Method, input.Path, input.Priority, boolValue(input.Enabled, true),
		defaultJSON(input.RequestMatch, "{}"), input.ResponseStatus, defaultJSON(input.ResponseHeaders, "{}"),
		input.ResponseBody, input.ResponseBodyType, input.DelayMillis)
	route, err := scanRoute(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMockRouteNotFound
		}
		return nil, fmt.Errorf("update mock route: %w", err)
	}
	return route, nil
}

// DeleteRoute soft-deletes a mock route for a project-owned server.
func (r *Repository) DeleteRoute(ctx context.Context, projectID, serverID, routeID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update mock_routes mr
		set deleted_at = now(), updated_at = now()
		from mock_servers ms
		where mr.id = $3
		  and mr.mock_server_id = ms.id
		  and ms.project_id = $1
		  and ms.id = $2
		  and ms.deleted_at is null
		  and mr.deleted_at is null
	`, projectID, serverID, routeID)
	if err != nil {
		return fmt.Errorf("delete mock route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMockRouteNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanServer(row rowScanner) (*MockServer, error) {
	var server MockServer
	if err := row.Scan(
		&server.ID, &server.ProjectID, &server.Name, &server.Description,
		&server.Port, &server.CreatedAt, &server.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &server, nil
}

func scanServers(rows pgx.Rows) ([]MockServer, error) {
	var servers []MockServer
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mock server: %w", err)
		}
		servers = append(servers, *server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if servers == nil {
		return []MockServer{}, nil
	}
	return servers, nil
}

func scanRoute(row rowScanner) (*MockRoute, error) {
	var route MockRoute
	if err := row.Scan(
		&route.ID, &route.MockServerID, &route.Method, &route.Path, &route.Priority,
		&route.Enabled, &route.RequestMatch, &route.ResponseStatus, &route.ResponseHeaders,
		&route.ResponseBody, &route.ResponseBodyType, &route.DelayMillis,
		&route.CreatedAt, &route.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &route, nil
}

func scanRoutes(rows pgx.Rows) ([]MockRoute, error) {
	var routes []MockRoute
	for rows.Next() {
		route, err := scanRoute(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mock route: %w", err)
		}
		routes = append(routes, *route)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if routes == nil {
		return []MockRoute{}, nil
	}
	return routes, nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultJSON(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}
