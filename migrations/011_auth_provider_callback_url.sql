-- +goose Up
-- OAuth 回调 URL 由管理端根据 VITE_API_BASE_URL + slug 生成并持久化

alter table auth_providers add column if not exists callback_url text not null default '';

-- +goose Down
alter table auth_providers drop column if exists callback_url;
