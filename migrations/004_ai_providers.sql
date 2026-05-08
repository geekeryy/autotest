-- +goose Up
create table if not exists ai_providers (
    id            uuid        primary key default gen_random_uuid(),
    project_id    uuid        not null references projects(id) on delete cascade,
    name          text        not null,
    provider_type text        not null check (provider_type in
                    ('deepseek', 'xiaomi', 'openai', 'anthropic', 'kimi', 'ollama')),
    base_url      text        not null default '',
    api_key       text        not null default '',
    default_model text        not null default '',
    extra_config  jsonb       not null default '{}'::jsonb,
    enabled       boolean     not null default true,
    is_default    boolean     not null default false,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz
);

create unique index if not exists idx_ai_providers_project_name_active
    on ai_providers (project_id, name) where deleted_at is null;

create unique index if not exists idx_ai_providers_project_default_active
    on ai_providers (project_id) where is_default and deleted_at is null;

create index if not exists idx_ai_providers_project_active
    on ai_providers (project_id, created_at desc) where deleted_at is null;

-- +goose Down
drop table if exists ai_providers;
