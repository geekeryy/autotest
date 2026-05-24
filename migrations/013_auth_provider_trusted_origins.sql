-- +goose Up
-- 可信前端域名迁移至 auth_providers，移除全局 oauth_trusted_origins

alter table auth_providers
    add column if not exists trusted_frontend_origins text[] not null default '{}';

-- 将既有全局可信域名复制到所有未删除的登录方式
update auth_providers ap
set trusted_frontend_origins = sub.origins
from (
    select coalesce(array_agg(origin order by origin), '{}'::text[]) as origins
    from oauth_trusted_origins
) sub
where ap.deleted_at is null
  and exists (select 1 from oauth_trusted_origins);

drop table if exists oauth_trusted_origins;

-- +goose Down
create table if not exists oauth_trusted_origins (
    id         uuid primary key default gen_random_uuid(),
    origin     text not null,
    created_at timestamptz not null default now(),
    unique (origin)
);

create index if not exists idx_oauth_trusted_origins_origin
    on oauth_trusted_origins (origin);

insert into oauth_trusted_origins (origin)
select distinct unnest(trusted_frontend_origins)
from auth_providers
where deleted_at is null
on conflict (origin) do nothing;

alter table auth_providers drop column if exists trusted_frontend_origins;
