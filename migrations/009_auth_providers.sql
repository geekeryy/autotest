-- +goose Up
-- 可配置 OAuth 登录方式与用户外部身份绑定

create table if not exists auth_providers (
    id              uuid primary key default gen_random_uuid(),
    provider_type   text not null check (provider_type in ('github', 'gitlab', 'google')),
    name            text not null,
    enabled         boolean not null default true,
    client_id       text not null default '',
    client_secret   text not null default '',
    scopes          text[] not null default '{}',
    extra_config    jsonb not null default '{}',
    auto_register   boolean not null default true,
    default_active  boolean not null default false,
    sort_order      int not null default 0,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    deleted_at      timestamptz
);

create index if not exists idx_auth_providers_enabled
    on auth_providers (sort_order asc, created_at asc)
    where deleted_at is null and enabled;

create table if not exists user_external_identities (
    id              uuid primary key default gen_random_uuid(),
    user_id         uuid not null references users (id) on delete cascade,
    provider_type   text not null,
    external_id     text not null,
    created_at      timestamptz not null default now(),
    unique (provider_type, external_id)
);

create index if not exists idx_user_external_identities_user
    on user_external_identities (user_id);

insert into user_external_identities (user_id, provider_type, external_id)
select id, 'github', github_id::text
from users
where github_id is not null
on conflict (provider_type, external_id) do nothing;

-- +goose Down
drop table if exists user_external_identities;
drop table if exists auth_providers;
