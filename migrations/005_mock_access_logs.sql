-- +goose Up
create table if not exists mock_access_logs (
    id              uuid        primary key default gen_random_uuid(),
    project_id      uuid        not null references projects(id) on delete cascade,
    mock_server_id  uuid        not null references mock_servers(id) on delete cascade,
    mock_route_id   uuid        references mock_routes(id) on delete set null,
    method          text        not null,
    path            text        not null,
    query           text        not null default '',
    client_ip       text        not null default '',
    matched         boolean     not null default false,
    response_status integer     not null,
    duration_ms     integer     not null,
    error_message   text        not null default '',
    request_headers jsonb       not null default '{}'::jsonb,
    request_body    text        not null default '',
    response_headers jsonb      not null default '{}'::jsonb,
    response_body   text        not null default '',
    created_at      timestamptz not null default now()
);

create index if not exists idx_mock_access_logs_server_created
    on mock_access_logs(mock_server_id, created_at desc);

-- +goose Down
drop table if exists mock_access_logs;
