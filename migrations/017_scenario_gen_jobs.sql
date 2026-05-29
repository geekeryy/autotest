-- +goose Up
create table if not exists scenario_gen_jobs (
    id              uuid        primary key default gen_random_uuid(),
    project_id      uuid        not null references projects(id) on delete cascade,
    service_id      uuid        not null references services(id) on delete cascade,
    environment_id  uuid        not null,
    config          jsonb       not null default '{}',
    status          text        not null default 'pending',
    rounds          integer     not null default 0,
    result          jsonb,
    created_by      uuid        references users(id) on delete set null,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);

create index if not exists idx_scenario_gen_jobs_project_created
    on scenario_gen_jobs(project_id, created_at desc);

-- +goose Down
drop table if exists scenario_gen_jobs;
