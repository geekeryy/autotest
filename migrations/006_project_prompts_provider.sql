-- +goose Up
alter table project_ai_prompts
    add column if not exists provider_id uuid null
        references ai_providers (id) on delete set null;

create index if not exists idx_project_ai_prompts_project_provider
    on project_ai_prompts (project_id, provider_id)
    where deleted_at is null;

-- +goose Down
drop index if exists idx_project_ai_prompts_project_provider;
alter table project_ai_prompts drop column if exists provider_id;
