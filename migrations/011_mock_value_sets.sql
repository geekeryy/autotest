-- +goose Up
create table if not exists mock_value_sets (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects (id) on delete cascade,
    key         text        not null,
    name        text        not null,
    description text        not null default '',
    values      jsonb       not null default '[]'::jsonb,
    weights     jsonb       not null default '[]'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create unique index if not exists ux_mock_value_sets_project_key
    on mock_value_sets (project_id, key)
    where deleted_at is null;

create index if not exists idx_mock_value_sets_project_active
    on mock_value_sets (project_id, created_at desc)
    where deleted_at is null;

-- +goose Down
drop index if exists idx_mock_value_sets_project_active;
drop index if exists ux_mock_value_sets_project_key;
drop table if exists mock_value_sets;
