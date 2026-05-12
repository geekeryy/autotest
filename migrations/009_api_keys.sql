-- +goose Up
create table if not exists api_keys (
    id              uuid        primary key default gen_random_uuid(),
    name            text        not null,
    token_hash      text        not null unique,
    token_prefix    text        not null,
    token_suffix    text        not null,
    scopes          text[]      not null default array['specs:import']::text[],
    created_by      uuid        not null references users (id) on delete cascade,
    enabled         boolean     not null default true,
    expires_at      timestamptz,
    last_used_at    timestamptz,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    deleted_at      timestamptz
);

create index if not exists idx_api_keys_active
    on api_keys (token_hash)
    where deleted_at is null and enabled = true;

create index if not exists idx_api_keys_created_by
    on api_keys (created_by, created_at desc)
    where deleted_at is null;

-- +goose Down
drop index if exists idx_api_keys_created_by;
drop index if exists idx_api_keys_active;
drop table if exists api_keys;
