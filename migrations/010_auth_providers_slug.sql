-- +goose Up
-- OAuth 公开路由标识 slug（login/callback URL 使用 slug 而非 UUID）

alter table auth_providers add column if not exists slug text;

-- 为已有记录回填：同类型第一条用 provider_type，其余追加 -2、-3…
with numbered as (
    select
        id,
        provider_type,
        row_number() over (partition by provider_type order by created_at asc, id asc) as rn
    from auth_providers
    where deleted_at is null
)
update auth_providers ap
set slug = case
    when n.rn = 1 then n.provider_type
    else n.provider_type || '-' || n.rn::text
end
from numbered n
where ap.id = n.id;

-- 软删或未参与分组的行用 provider_type + id 前缀兜底
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
