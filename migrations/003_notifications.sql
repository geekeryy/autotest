-- +goose Up
create table if not exists notifications (
    id          uuid        primary key default gen_random_uuid(),
    user_id     uuid        not null references users(id) on delete cascade,
    type        text        not null,
    title       text        not null,
    body        text        not null,
    payload     jsonb       not null default '{}'::jsonb,
    read_at     timestamptz,
    created_at  timestamptz not null default now()
);

create index if not exists idx_notifications_user_created
    on notifications(user_id, created_at desc);

-- +goose Down
drop table if exists notifications;
