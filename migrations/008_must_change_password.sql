-- +goose Up
alter table users
    add column if not exists must_change_password boolean not null default false;

-- +goose Down
alter table users drop column if exists must_change_password;
