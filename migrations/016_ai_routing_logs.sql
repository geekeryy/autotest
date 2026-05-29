-- +goose Up
create table if not exists ai_routing_logs (
    id             uuid        primary key default gen_random_uuid(),
    session_id     uuid        not null references ai_sessions(id) on delete cascade,
    message_seq    integer,
    planner_output jsonb,
    router_output  jsonb,
    per_hop        jsonb,
    outcome        text,
    created_at     timestamptz not null default now()
);

create index if not exists idx_ai_routing_logs_session_created
    on ai_routing_logs(session_id, created_at desc);

-- +goose Down
drop table if exists ai_routing_logs;
