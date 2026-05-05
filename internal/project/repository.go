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
var ErrMemberNotFound = errors.New("project member not found")
var ErrMemberAlreadyExists = errors.New("user is already a member of this project")

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateProject(ctx context.Context, input CreateProjectInput, createdBy uuid.UUID) (*Project, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create project: %w", err)
	}
	defer tx.Rollback(ctx)

	var p Project
	if err := tx.QueryRow(ctx, `
		insert into projects (name, description)
		values ($1, $2)
		returning id, name, description, created_at, updated_at
	`, input.Name, input.Description).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into project_members (project_id, user_id, role)
		values ($1, $2, 'owner')
	`, p.ID, createdBy); err != nil {
		return nil, fmt.Errorf("add creator as owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create project: %w", err)
	}
	return &p, nil
}

// ListProjects returns all non-deleted projects. For admin use only.
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

// ListProjectsForUser returns projects where the user has membership, including the user's role.
func (r *Repository) ListProjectsForUser(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select p.id, p.name, p.description, pm.role, p.created_at, p.updated_at
		from projects p
		join project_members pm on pm.project_id = p.id and pm.user_id = $1
		where p.deleted_at is null
		order by p.created_at desc
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects for user: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var role ProjectRole
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &role, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.MyRole = &role
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *Repository) GetMembership(ctx context.Context, projectID, userID uuid.UUID) (*ProjectMember, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	var m ProjectMember
	err := r.DB.QueryRow(ctx, `
		select pm.project_id, pm.user_id, u.username, u.display_name, pm.role, pm.created_at
		from project_members pm
		join users u on u.id = pm.user_id
		where pm.project_id = $1 and pm.user_id = $2
	`, projectID, userID).Scan(&m.ProjectID, &m.UserID, &m.Username, &m.DisplayName, &m.Role, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return &m, nil
}

func (r *Repository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	rows, err := r.DB.Query(ctx, `
		select pm.project_id, pm.user_id, u.username, u.display_name, pm.role, pm.created_at
		from project_members pm
		join users u on u.id = pm.user_id
		where pm.project_id = $1
		order by pm.created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Username, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *Repository) AddMember(ctx context.Context, projectID uuid.UUID, input AddMemberInput) (*ProjectMember, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	var m ProjectMember
	err := r.DB.QueryRow(ctx, `
		insert into project_members (project_id, user_id, role)
		values ($1, $2, $3)
		returning project_id, user_id, role, created_at
	`, projectID, input.UserID, input.Role).Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}

	if err := r.DB.QueryRow(ctx, `select username, display_name from users where id = $1`, input.UserID).
		Scan(&m.Username, &m.DisplayName); err != nil {
		return nil, fmt.Errorf("load member user: %w", err)
	}
	return &m, nil
}

func (r *Repository) UpdateMember(ctx context.Context, projectID, userID uuid.UUID, input UpdateMemberInput) (*ProjectMember, error) {
	if r.DB == nil {
		return nil, ErrDatabaseUnavailable
	}

	tag, err := r.DB.Exec(ctx, `
		update project_members set role = $3
		where project_id = $1 and user_id = $2
	`, projectID, userID, input.Role)
	if err != nil {
		return nil, fmt.Errorf("update member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrMemberNotFound
	}
	return r.GetMembership(ctx, projectID, userID)
}

func (r *Repository) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	if r.DB == nil {
		return ErrDatabaseUnavailable
	}

	tag, err := r.DB.Exec(ctx, `
		delete from project_members where project_id = $1 and user_id = $2
	`, projectID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
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
