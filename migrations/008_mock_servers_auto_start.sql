-- +goose Up
alter table mock_servers
    add column if not exists auto_start boolean not null default false;

create index if not exists idx_mock_servers_auto_start_active
    on mock_servers (auto_start)
    where deleted_at is null and auto_start = true;

-- +goose Down
drop index if exists idx_mock_servers_auto_start_active;
alter table mock_servers drop column if exists auto_start;
