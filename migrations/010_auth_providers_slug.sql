-- +goose Up
-- OAuth 公开路由标识 slug（login/callback URL 使用 slug 而非 UUID）

alter table auth_providers add column if not exists slug text;

-- 仅为尚无 slug 的记录回填；绝不覆盖已有 slug（含管理端已配置值）
update auth_providers
set slug = provider_type || '-' || substr(replace(id::text, '-', ''), 1, 8)
where slug is null or slug = '';

alter table auth_providers alter column slug set not null;

create unique index if not exists idx_auth_providers_slug
    on auth_providers (slug)
    where deleted_at is null;

-- +goose Down
drop index if exists idx_auth_providers_slug;
alter table auth_providers drop column if exists slug;
