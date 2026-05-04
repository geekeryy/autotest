package project

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDatabaseUnavailable = errors.New("database unavailable")
var ErrProjectNotFound = errors.New("project not found")
var ErrServiceNotFound = errors.New("service not found")
var ErrEnvironmentNotFound = errors.New("environment not found")

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	row := r.DB.QueryRow(ctx, `
		insert into projects (name, description)
		values ($1, $2)
		returning id, name, description, created_at, updated_at
	`, input.Name, input.Description)

	var p Project
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &p, nil
}

func (r *Repository) ListProjects(ctx context.Context) ([]Project, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select id, name, description, created_at, updated_at
		from projects
		where deleted_at is null
		order by created_at desc
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *Repository) DeleteProject(ctx context.Context, projectID uuid.UUID) error {
	if r.DB == nil {
		return ErrDatabaseUnavailable
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete project: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		update projects
		set deleted_at = now(), updated_at = now()
		where id = $1 and deleted_at is null
	`, projectID)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}

	statements := []string{
		`update test_run_results
		 set deleted_at = now()
		 where deleted_at is null
		   and run_id in (select id from test_runs where project_id = $1)`,
		`update test_suite_items
		 set deleted_at = now(), updated_at = now()
		 where deleted_at is null
		   and (
		     suite_id in (select id from test_suites where project_id = $1)
		     or test_case_id in (select id from test_cases where project_id = $1)
		   )`,
		`update test_case_steps
		 set deleted_at = now(), updated_at = now()
		 where deleted_at is null
		   and test_case_id in (select id from test_cases where project_id = $1)`,
		`update test_runs
		 set deleted_at = now()
		 where project_id = $1 and deleted_at is null`,
		`update test_suites
		 set deleted_at = now(), updated_at = now()
		 where project_id = $1 and deleted_at is null`,
		`update test_cases
		 set deleted_at = now(), updated_at = now()
		 where project_id = $1 and deleted_at is null`,
		`update api_endpoints
		 set deleted_at = now(), updated_at = now()
		 where deleted_at is null
		   and service_id in (select id from services where project_id = $1)`,
		`update api_specs
		 set deleted_at = now()
		 where deleted_at is null
		   and service_id in (select id from services where project_id = $1)`,
		`update environments
		 set deleted_at = now(), updated_at = now()
		 where project_id = $1 and deleted_at is null`,
		`update services
		 set deleted_at = now(), updated_at = now()
		 where project_id = $1 and deleted_at is null`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, projectID); err != nil {
			return fmt.Errorf("delete project children: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete project: %w", err)
	}
	return nil
}

func (r *Repository) CreateService(ctx context.Context, projectID uuid.UUID, input CreateServiceInput) (*Service, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	row := r.DB.QueryRow(ctx, `
		insert into services (project_id, name, description)
		select id, $2, $3
		from projects
		where id = $1 and deleted_at is null
		returning id, project_id, name, description, created_at, updated_at
	`, projectID, input.Name, input.Description)

	var svc Service
	if err := row.Scan(&svc.ID, &svc.ProjectID, &svc.Name, &svc.Description, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}
	return &svc, nil
}

func (r *Repository) UpdateService(ctx context.Context, projectID, serviceID uuid.UUID, input UpdateServiceInput) (*Service, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	row := r.DB.QueryRow(ctx, `
		update services s
		set name = $3, description = $4, updated_at = now()
		from projects p
		where s.project_id = $1
		  and s.id = $2
		  and p.id = s.project_id
		  and p.deleted_at is null
		  and s.deleted_at is null
		returning s.id, s.project_id, s.name, s.description, s.created_at, s.updated_at
	`, projectID, serviceID, input.Name, input.Description)

	var svc Service
	if err := row.Scan(&svc.ID, &svc.ProjectID, &svc.Name, &svc.Description, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrServiceNotFound
		}
		return nil, fmt.Errorf("update service: %w", err)
	}
	return &svc, nil
}

func (r *Repository) DeleteService(ctx context.Context, projectID, serviceID uuid.UUID) error {
	if r.DB == nil {
		return ErrDatabaseUnavailable
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete service: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		update services s
		set deleted_at = now(), updated_at = now()
		from projects p
		where s.project_id = $1
		  and s.id = $2
		  and p.id = s.project_id
		  and p.deleted_at is null
		  and s.deleted_at is null
	`, projectID, serviceID)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}

	statements := []string{
		`update test_run_results
		 set deleted_at = now()
		 where deleted_at is null
		   and run_id in (select id from test_runs where service_id = $1)`,
		`update test_suite_items
		 set deleted_at = now(), updated_at = now()
		 where deleted_at is null
		   and (
		     suite_id in (select id from test_suites where service_id = $1)
		     or test_case_id in (select id from test_cases where service_id = $1)
		   )`,
		`update test_case_steps
		 set deleted_at = now(), updated_at = now()
		 where deleted_at is null
		   and test_case_id in (select id from test_cases where service_id = $1)`,
		`update test_runs
		 set deleted_at = now()
		 where service_id = $1 and deleted_at is null`,
		`update environments
		 set deleted_at = now(), updated_at = now()
		 where service_id = $1 and deleted_at is null`,
		`update test_suites
		 set deleted_at = now(), updated_at = now()
		 where service_id = $1 and deleted_at is null`,
		`update test_cases
		 set deleted_at = now(), updated_at = now()
		 where service_id = $1 and deleted_at is null`,
		`update api_endpoints
		 set deleted_at = now(), updated_at = now()
		 where service_id = $1 and deleted_at is null`,
		`update api_specs
		 set deleted_at = now()
		 where service_id = $1 and deleted_at is null`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, serviceID); err != nil {
			return fmt.Errorf("delete service children: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete service: %w", err)
	}
	return nil
}

func (r *Repository) ListServices(ctx context.Context, projectID uuid.UUID) ([]Service, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select s.id, s.project_id, s.name, s.description, s.created_at, s.updated_at
		from services s
		join projects p on p.id = s.project_id and p.deleted_at is null
		where s.project_id = $1 and s.deleted_at is null
		order by s.created_at desc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var svc Service
		if err := rows.Scan(&svc.ID, &svc.ProjectID, &svc.Name, &svc.Description, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

func (r *Repository) CreateEnvironment(ctx context.Context, projectID uuid.UUID, input CreateEnvironmentInput) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(input.Variables) == 0 {
		input.Variables = []byte(`{}`)
	}
	if len(input.Auth) == 0 {
		input.Auth = []byte(`{}`)
	}

	row := r.DB.QueryRow(ctx, `
		insert into environments (project_id, name, base_url, variables, auth)
		select id, $2, $3, $4, $5
		from projects
		where id = $1 and deleted_at is null
		returning id, project_id, service_id, name, base_url, variables, auth, created_at, updated_at
	`, projectID, input.Name, input.BaseURL, input.Variables, input.Auth)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}
	return &env, nil
}

func (r *Repository) UpdateEnvironment(ctx context.Context, projectID, environmentID uuid.UUID, input UpdateEnvironmentInput) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(input.Variables) == 0 {
		input.Variables = []byte(`{}`)
	}
	if len(input.Auth) == 0 {
		input.Auth = []byte(`{}`)
	}

	row := r.DB.QueryRow(ctx, `
		update environments e
		set name = $3, base_url = $4, variables = $5, auth = $6, updated_at = now()
		from projects p
		where e.project_id = $1
		  and e.id = $2
		  and p.id = e.project_id
		  and p.deleted_at is null
		  and e.deleted_at is null
		returning e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
	`, projectID, environmentID, input.Name, input.BaseURL, input.Variables, input.Auth)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("update environment: %w", err)
	}
	return &env, nil
}

func (r *Repository) DeleteEnvironment(ctx context.Context, projectID, environmentID uuid.UUID) error {
	if r.DB == nil {
		return ErrDatabaseUnavailable
	}

	tag, err := r.DB.Exec(ctx, `
		update environments e
		set deleted_at = now(), updated_at = now()
		from projects p
		where e.project_id = $1
		  and e.id = $2
		  and p.id = e.project_id
		  and p.deleted_at is null
		  and e.deleted_at is null
	`, projectID, environmentID)
	if err != nil {
		return fmt.Errorf("delete environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEnvironmentNotFound
	}
	return nil
}

func (r *Repository) ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
		from environments e
		join projects p on p.id = e.project_id and p.deleted_at is null
		where e.project_id = $1 and e.deleted_at is null
		order by e.created_at desc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	defer rows.Close()

	var environments []Environment
	for rows.Next() {
		var env Environment
		if err := scanEnvironment(rows, &env); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		environments = append(environments, env)
	}
	return environments, rows.Err()
}

func (r *Repository) GetEnvironment(ctx context.Context, projectID, environmentID uuid.UUID) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	row := r.DB.QueryRow(ctx, `
		select e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
		from environments e
		join projects p on p.id = e.project_id and p.deleted_at is null
		where e.project_id = $1 and e.id = $2 and e.deleted_at is null
	`, projectID, environmentID)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("get environment: %w", err)
	}
	return &env, nil
}

func (r *Repository) CreateServiceEnvironment(ctx context.Context, projectID, serviceID uuid.UUID, input CreateEnvironmentInput) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(input.Variables) == 0 {
		input.Variables = []byte(`{}`)
	}
	if len(input.Auth) == 0 {
		input.Auth = []byte(`{}`)
	}

	row := r.DB.QueryRow(ctx, `
		insert into environments (project_id, service_id, name, base_url, variables, auth)
		select p.id, s.id, $3, $4, $5, $6
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, service_id, name, base_url, variables, auth, created_at, updated_at
	`, projectID, serviceID, input.Name, input.BaseURL, input.Variables, input.Auth)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		return nil, fmt.Errorf("create service environment: %w", err)
	}
	return &env, nil
}

func (r *Repository) UpdateServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID, input UpdateEnvironmentInput) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(input.Variables) == 0 {
		input.Variables = []byte(`{}`)
	}
	if len(input.Auth) == 0 {
		input.Auth = []byte(`{}`)
	}

	row := r.DB.QueryRow(ctx, `
		update environments e
		set name = $4, base_url = $5, variables = $6, auth = $7, updated_at = now()
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where e.project_id = p.id
		  and e.service_id = s.id
		  and e.id = $3
		  and p.id = $1
		  and p.deleted_at is null
		  and e.deleted_at is null
		returning e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
	`, projectID, serviceID, environmentID, input.Name, input.BaseURL, input.Variables, input.Auth)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("update service environment: %w", err)
	}
	return &env, nil
}

func (r *Repository) DeleteServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID) error {
	if r.DB == nil {
		return ErrDatabaseUnavailable
	}

	tag, err := r.DB.Exec(ctx, `
		update environments e
		set deleted_at = now(), updated_at = now()
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where e.project_id = p.id
		  and e.service_id = s.id
		  and e.id = $3
		  and p.id = $1
		  and p.deleted_at is null
		  and e.deleted_at is null
	`, projectID, serviceID, environmentID)
	if err != nil {
		return fmt.Errorf("delete service environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEnvironmentNotFound
	}
	return nil
}

func (r *Repository) ListServiceEnvironments(ctx context.Context, projectID, serviceID uuid.UUID) ([]Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
		from environments e
		join projects p on p.id = e.project_id and p.deleted_at is null
		join services s on s.id = e.service_id and s.project_id = p.id and s.deleted_at is null
		where e.project_id = $1 and e.service_id = $2 and e.deleted_at is null
		order by e.created_at desc
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service environments: %w", err)
	}
	defer rows.Close()

	var environments []Environment
	for rows.Next() {
		var env Environment
		if err := scanEnvironment(rows, &env); err != nil {
			return nil, fmt.Errorf("scan service environment: %w", err)
		}
		environments = append(environments, env)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if environments == nil {
		return []Environment{}, nil
	}
	return environments, nil
}

func (r *Repository) GetServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID) (*Environment, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	row := r.DB.QueryRow(ctx, `
		select e.id, e.project_id, e.service_id, e.name, e.base_url, e.variables, e.auth, e.created_at, e.updated_at
		from environments e
		join projects p on p.id = e.project_id and p.deleted_at is null
		join services s on s.id = e.service_id and s.project_id = p.id and s.deleted_at is null
		where e.project_id = $1
		  and e.service_id = $2
		  and e.id = $3
		  and e.deleted_at is null
	`, projectID, serviceID, environmentID)

	var env Environment
	if err := scanEnvironment(row, &env); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("get service environment: %w", err)
	}
	return &env, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEnvironment(row rowScanner, env *Environment) error {
	var serviceID *uuid.UUID
	if err := row.Scan(
		&env.ID,
		&env.ProjectID,
		&serviceID,
		&env.Name,
		&env.BaseURL,
		&env.Variables,
		&env.Auth,
		&env.CreatedAt,
		&env.UpdatedAt,
	); err != nil {
		return err
	}
	if serviceID != nil {
		env.ServiceID = *serviceID
	}
	return nil
}
