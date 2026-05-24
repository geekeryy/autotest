-- +goose Up
-- OAuth 登录 frontendUrl 校验与 CORS 白名单

create table if not exists oauth_trusted_origins (
    id         uuid primary key default gen_random_uuid(),
    origin     text not null,
    created_at timestamptz not null default now(),
    unique (origin)
);

create index if not exists idx_oauth_trusted_origins_origin
    on oauth_trusted_origins (origin);

-- +goose Down
drop table if exists oauth_trusted_origins;
