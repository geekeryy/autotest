-- +goose Up
create extension if not exists pgcrypto;

-- ── 核心业务表 ────────────────────────────────────────────────────────────────

create table if not exists projects (
    id          uuid        primary key default gen_random_uuid(),
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists services (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists environments (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    service_id  uuid        references services(id) on delete cascade,
    name        text        not null,
    base_url    text        not null default '',
    variables   jsonb       not null default '{}'::jsonb,
    auth        jsonb       not null default '{}'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists service_environment_configs (
    id             uuid        primary key default gen_random_uuid(),
    project_id     uuid        not null references projects(id) on delete cascade,
    service_id     uuid        not null references services(id) on delete cascade,
    environment_id uuid        not null references environments(id) on delete cascade,
    base_url       text        not null,
    variables      jsonb       not null default '{}'::jsonb,
    auth           jsonb       not null default '{}'::jsonb,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    deleted_at     timestamptz
);

create table if not exists api_specs (
    id                  uuid        primary key default gen_random_uuid(),
    service_id          uuid        not null references services(id) on delete cascade,
    version             integer     not null,
    content_hash        text        not null,
    raw_content         bytea       not null,
    normalized_snapshot jsonb       not null default '{}'::jsonb,
    status              text        not null default 'imported',
    created_at          timestamptz not null default now(),
    deleted_at          timestamptz,
    unique (service_id, version),
    unique (service_id, content_hash)
);

create table if not exists api_endpoints (
    id              uuid        primary key default gen_random_uuid(),
    service_id      uuid        not null references services(id) on delete cascade,
    spec_id         uuid        not null references api_specs(id) on delete cascade,
    method          text        not null,
    path            text        not null,
    operation_id    text        not null default '',
    summary         text        not null default '',
    tags            text[]      not null default '{}',
    request_schema  jsonb       not null default '{}'::jsonb,
    response_schema jsonb       not null default '{}'::jsonb,
    fingerprint     text        not null,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    deleted_at      timestamptz,
    unique (service_id, method, path)
);

create table if not exists test_cases (
    id                     uuid        primary key default gen_random_uuid(),
    project_id             uuid        not null references projects(id) on delete cascade,
    service_id             uuid        not null references services(id) on delete cascade,
    endpoint_id            uuid        references api_endpoints(id) on delete set null,
    source                 text        not null check (source in ('auto', 'manual', 'derived')),
    name                   text        not null,
    method                 text        not null,
    path                   text        not null,
    fingerprint            text        not null unique,
    generation_rule_id     text        not null default '',
    request                jsonb       not null default '{}'::jsonb,
    assertions             jsonb       not null default '[]'::jsonb,
    status                 text        not null default 'active' check (status in ('draft', 'active', 'disabled')),
    last_response_snapshot jsonb       not null default '{}'::jsonb,
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now(),
    deleted_at             timestamptz
);

create table if not exists test_case_steps (
    id            uuid        primary key default gen_random_uuid(),
    test_case_id  uuid        not null references test_cases(id) on delete cascade,
    step_order    integer     not null default 1,
    name          text        not null,
    request       jsonb       not null default '{}'::jsonb,
    assertions    jsonb       not null default '[]'::jsonb,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz,
    unique (test_case_id, step_order)
);

create table if not exists test_suites (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    service_id  uuid        not null references services(id) on delete cascade,
    name        text        not null,
    kind        text        not null check (kind in ('manual', 'auto', 'mixed')),
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists test_suite_items (
    id           uuid        primary key default gen_random_uuid(),
    suite_id     uuid        not null references test_suites(id) on delete cascade,
    test_case_id uuid        not null references test_cases(id) on delete cascade,
    item_order   integer     not null default 1,
    source       text        not null check (source in ('manual_added', 'auto_generated', 'auto_selected')),
    enabled      boolean     not null default true,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    deleted_at   timestamptz,
    unique (suite_id, test_case_id)
);

create table if not exists test_runs (
    id                            uuid        primary key default gen_random_uuid(),
    project_id                    uuid        not null references projects(id) on delete cascade,
    service_id                    uuid        not null references services(id) on delete cascade,
    suite_id                      uuid        references test_suites(id) on delete set null,
    environment_id                uuid        references environments(id) on delete set null,
    service_environment_config_id uuid        references service_environment_configs(id) on delete set null,
    name                          text        not null default '',
    status                        text        not null default 'queued' check (status in ('queued', 'running', 'passed', 'failed', 'cancelled')),
    variables                     jsonb       not null default '{}'::jsonb,
    snapshot                      jsonb       not null default '{}'::jsonb,
    started_at                    timestamptz,
    finished_at                   timestamptz,
    created_at                    timestamptz not null default now(),
    deleted_at                    timestamptz
);

create table if not exists test_run_results (
    id                uuid        primary key default gen_random_uuid(),
    run_id            uuid        not null references test_runs(id) on delete cascade,
    test_case_id      uuid        not null references test_cases(id) on delete cascade,
    step_id           uuid        references test_case_steps(id) on delete set null,
    status            text        not null check (status in ('passed', 'failed', 'error')),
    duration_millis   bigint      not null default 0,
    request_snapshot  jsonb       not null default '{}'::jsonb,
    response_snapshot jsonb       not null default '{}'::jsonb,
    assertions        jsonb       not null default '[]'::jsonb,
    error             text        not null default '',
    created_at        timestamptz not null default now(),
    deleted_at        timestamptz
);

-- ── 认证与权限表 ──────────────────────────────────────────────────────────────

create table if not exists users (
    id            uuid        primary key default gen_random_uuid(),
    username      text        not null unique,
    password_hash text        not null,
    display_name  text        not null default '',
    email         text        not null default '',
    active        boolean     not null default true,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create table if not exists roles (
    id          uuid        primary key default gen_random_uuid(),
    code        text        not null unique,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table if not exists permissions (
    id          uuid        primary key default gen_random_uuid(),
    code        text        not null unique,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table if not exists user_roles (
    user_id    uuid        not null references users(id) on delete cascade,
    role_id    uuid        not null references roles(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (user_id, role_id)
);

create table if not exists role_permissions (
    role_id       uuid        not null references roles(id) on delete cascade,
    permission_id uuid        not null references permissions(id) on delete cascade,
    created_at    timestamptz not null default now(),
    primary key (role_id, permission_id)
);

-- ── 索引 ──────────────────────────────────────────────────────────────────────

create index if not exists idx_projects_active
    on projects(created_at desc) where deleted_at is null;

create index if not exists idx_services_project
    on services(project_id);
create index if not exists idx_services_project_active
    on services(project_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_services_project_name_active
    on services(project_id, name) where deleted_at is null;

create index if not exists idx_environments_project_active
    on environments(project_id, created_at desc) where deleted_at is null;
create index if not exists idx_environments_service_active
    on environments(service_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_environments_service_name_active
    on environments(service_id, name) where deleted_at is null;

create unique index if not exists idx_service_environment_configs_active
    on service_environment_configs(service_id, environment_id) where deleted_at is null;
create index if not exists idx_service_environment_configs_project_active
    on service_environment_configs(project_id, service_id, environment_id) where deleted_at is null;

create index if not exists idx_api_specs_service
    on api_specs(service_id);
create index if not exists idx_api_specs_service_active
    on api_specs(service_id, version desc) where deleted_at is null;

create index if not exists idx_api_endpoints_service
    on api_endpoints(service_id);
create index if not exists idx_api_endpoints_service_active
    on api_endpoints(service_id, path, method) where deleted_at is null;

create index if not exists idx_test_cases_service_source
    on test_cases(service_id, source);
create index if not exists idx_test_cases_project_service_active
    on test_cases(project_id, service_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_test_cases_auto_generation_active_unique
    on test_cases(project_id, service_id, endpoint_id, generation_rule_id)
    where deleted_at is null
      and source = 'auto'
      and endpoint_id is not null
      and generation_rule_id <> '';

create index if not exists idx_test_suite_items_suite_order
    on test_suite_items(suite_id, item_order);
create index if not exists idx_test_suite_items_suite_active
    on test_suite_items(suite_id, item_order, created_at) where deleted_at is null;

create index if not exists idx_test_suites_project_service_active
    on test_suites(project_id, service_id, created_at desc) where deleted_at is null;

create index if not exists idx_test_runs_project_status
    on test_runs(project_id, status);
create index if not exists idx_test_runs_project_active
    on test_runs(project_id, created_at desc) where deleted_at is null;

create index if not exists idx_test_run_results_run
    on test_run_results(run_id);
create index if not exists idx_test_run_results_run_active
    on test_run_results(run_id, created_at desc) where deleted_at is null;

create index if not exists idx_user_roles_role
    on user_roles(role_id);
create index if not exists idx_role_permissions_permission
    on role_permissions(permission_id);

-- +goose Down
drop table if exists role_permissions;
drop table if exists user_roles;
drop table if exists permissions;
drop table if exists roles;
drop table if exists users;
drop table if exists test_run_results;
drop table if exists test_runs;
drop table if exists test_suite_items;
drop table if exists test_suites;
drop table if exists test_case_steps;
drop table if exists test_cases;
drop table if exists api_endpoints;
drop table if exists api_specs;
drop table if exists service_environment_configs;
drop table if exists environments;
drop table if exists services;
drop table if exists projects;
