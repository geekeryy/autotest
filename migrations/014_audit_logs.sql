-- +goose Up
create table if not exists audit_logs (
    id              uuid        primary key default gen_random_uuid(),
    action          text        not null,
    success         boolean     not null,
    actor_user_id   uuid        references users(id) on delete set null,
    actor_username  text        not null default '',
    resource        text        not null default '',
    client_ip       text        not null default '',
    user_agent      text        not null default '',
    detail          jsonb       not null default '{}'::jsonb,
    created_at      timestamptz not null default now()
);

create index if not exists idx_audit_logs_created_at
    on audit_logs(created_at desc);

create index if not exists idx_audit_logs_action_created
    on audit_logs(action, created_at desc);

-- +goose Down
drop table if exists audit_logs;
