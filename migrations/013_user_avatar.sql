-- +goose Up
-- 用户头像：压缩后的 JPEG 字节，仅用于管理后台展示；通过 JSON data URL 下发（Bearer 场景下 img 无法带鉴权头）。
alter table users add column if not exists avatar_jpeg bytea null;

-- +goose Down
alter table users drop column if exists avatar_jpeg;
