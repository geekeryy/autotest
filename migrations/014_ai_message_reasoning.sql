-- +goose Up
-- DeepSeek / Xiaomi thinking mode can require reasoning_content to be
-- replayed on tool-call continuation, so keep it separate from user-visible
-- assistant content.
alter table if exists ai_messages
    add column if not exists reasoning_content text not null default '';

-- +goose Down
alter table if exists ai_messages
    drop column if exists reasoning_content;
