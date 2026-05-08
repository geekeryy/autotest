-- +goose Up
create table if not exists project_ai_prompts (
    id            uuid        primary key default gen_random_uuid(),
    project_id    uuid        not null references projects(id) on delete cascade,
    action        text        not null check (action in ('generate_params','generate_assertion','generate_case_data','raw')),
    name          text        not null default '',
    system_prompt text        not null default '',
    default_model text        not null default '',
    enabled       boolean     not null default true,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz
);
create unique index if not exists project_ai_prompts_action_idx
    on project_ai_prompts(project_id, action) where deleted_at is null;
-- +goose Down
drop table if exists project_ai_prompts;
