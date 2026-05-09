-- +goose Up
create table if not exists test_data_tables (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects (id) on delete cascade,
    key         text        not null,
    name        text        not null,
    description text        not null default '',
    columns     jsonb       not null default '[]'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create unique index if not exists idx_test_data_tables_project_key_active
    on test_data_tables (project_id, key) where deleted_at is null;

create index if not exists idx_test_data_tables_project_active
    on test_data_tables (project_id, created_at desc) where deleted_at is null;

create table if not exists test_data_rows (
    id         uuid        primary key default gen_random_uuid(),
    table_id   uuid        not null references test_data_tables (id) on delete cascade,
    row_index  int         not null default 0,
    values     jsonb       not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_test_data_rows_table_index
    on test_data_rows (table_id, row_index);

-- +goose Down
drop table if exists test_data_rows;
drop table if exists test_data_tables;
