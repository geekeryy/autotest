-- +goose Up
-- GitHub OAuth：本地密码可空、记录登录方式与 GitHub 用户 ID

alter table users alter column password_hash drop not null;

alter table users
    add column if not exists auth_provider text not null default 'local';

alter table users
    add column if not exists github_id bigint;

create unique index if not exists users_github_id_key on users (github_id) where github_id is not null;

create index if not exists idx_users_active_auth_provider on users (active, auth_provider);

-- +goose Down
drop index if exists idx_users_active_auth_provider;
drop index if exists users_github_id_key;
alter table users drop column if exists github_id;
alter table users drop column if exists auth_provider;
alter table users alter column password_hash set not null;
