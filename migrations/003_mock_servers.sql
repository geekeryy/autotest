-- +goose Up
create table if not exists mock_servers (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    name        text        not null,
    description text        not null default '',
    port        integer     not null check (port between 1 and 65535),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists mock_routes (
    id               uuid        primary key default gen_random_uuid(),
    mock_server_id   uuid        not null references mock_servers(id) on delete cascade,
    method           text        not null,
    path             text        not null,
    priority         integer     not null default 0,
    enabled          boolean     not null default true,
    request_match    jsonb       not null default '{}'::jsonb,
    response_status  integer     not null default 200 check (response_status between 100 and 599),
    response_headers jsonb       not null default '{}'::jsonb,
    response_body    text        not null default '',
    response_body_type text      not null default 'json' check (response_body_type in ('json', 'text', 'raw')),
    delay_millis     integer     not null default 0 check (delay_millis >= 0),
    created_at       timestamptz not null default now(),
    updated_at       timestamptz not null default now(),
    deleted_at       timestamptz
);

create index if not exists idx_mock_servers_project_active
    on mock_servers(project_id, created_at desc) where deleted_at is null;

create unique index if not exists idx_mock_servers_project_name_active
    on mock_servers(project_id, name) where deleted_at is null;

create unique index if not exists idx_mock_servers_port_active
    on mock_servers(port) where deleted_at is null;

create index if not exists idx_mock_routes_server_active
    on mock_routes(mock_server_id, method, path, priority desc, created_at asc)
    where deleted_at is null;

-- +goose Down
drop table if exists mock_routes;
drop table if exists mock_servers;
