-- +goose Up
-- 业务数据源、AI 提供商、Prompt 管理改为全局平台资源（不再按项目隔离）

-- ── business_data_sources：去 project_id ─────────────────────────────────────

do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'business_data_sources'
          and column_name = 'project_id'
    ) then
        update business_data_sources ds
        set name = ds.name || ' [' || left(p.name, 40) || ']'
        from projects p
        where ds.project_id = p.id
          and ds.deleted_at is null
          and exists (
            select 1
            from business_data_sources ds2
            where ds2.id <> ds.id
              and ds2.name = ds.name
              and ds2.deleted_at is null
          );

        alter table business_data_sources drop constraint if exists business_data_sources_project_id_fkey;
        drop index if exists idx_business_data_sources_project_name_active;
        drop index if exists idx_business_data_sources_project_list_active;
        alter table business_data_sources drop column project_id;
    end if;
end $$;

create unique index if not exists idx_business_data_sources_name_active
    on business_data_sources(name) where deleted_at is null;
create index if not exists idx_business_data_sources_list_active
    on business_data_sources(created_at desc) where deleted_at is null;

-- ── ai_providers：去 project_id ───────────────────────────────────────────────

do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'ai_providers'
          and column_name = 'project_id'
    ) then
        update ai_providers ap
        set name = ap.name || ' [' || left(p.name, 40) || ']'
        from projects p
        where ap.project_id = p.id
          and ap.deleted_at is null
          and exists (
            select 1
            from ai_providers ap2
            where ap2.id <> ap.id
              and ap2.name = ap.name
              and ap2.deleted_at is null
          );

        alter table ai_providers drop constraint if exists ai_providers_project_id_fkey;
        drop index if exists idx_ai_providers_project_name_active;
        drop index if exists idx_ai_providers_project_default_active;
        drop index if exists idx_ai_providers_project_active;
        alter table ai_providers drop column project_id;
    end if;
end $$;

-- 全局仅保留一个默认提供商：其余 is_default 清零
with ranked as (
    select id,
           row_number() over (order by is_default desc, updated_at desc) as rn
    from ai_providers
    where deleted_at is null and is_default
)
update ai_providers
set is_default = false, updated_at = now()
where id in (select id from ranked where rn > 1);

create unique index if not exists idx_ai_providers_name_active
    on ai_providers(name) where deleted_at is null;
create unique index if not exists idx_ai_providers_default_active
    on ai_providers((true)) where is_default and deleted_at is null;
create index if not exists idx_ai_providers_list_active
    on ai_providers(created_at desc) where deleted_at is null;

-- ── project_ai_prompts：去 project_id，按 action 全局唯一 ─────────────────────

do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'project_ai_prompts'
          and column_name = 'project_id'
    ) then
        update project_ai_prompts pap
        set deleted_at = now(), updated_at = now()
        where pap.deleted_at is null
          and pap.id not in (
            select distinct on (action) id
            from project_ai_prompts
            where deleted_at is null
            order by action, updated_at desc
          );

        alter table project_ai_prompts drop constraint if exists project_ai_prompts_project_id_fkey;
        drop index if exists project_ai_prompts_action_idx;
        drop index if exists idx_project_ai_prompts_project_provider;
        alter table project_ai_prompts drop column project_id;
    end if;
end $$;

create unique index if not exists idx_project_ai_prompts_action_active
    on project_ai_prompts(action) where deleted_at is null;
create index if not exists idx_project_ai_prompts_provider
    on project_ai_prompts(provider_id) where deleted_at is null;

-- +goose Down
-- 不可逆：全局资源迁移无法安全回滚
