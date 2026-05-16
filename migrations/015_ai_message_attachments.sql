-- +goose Up
-- 用户多模态消息附件（如小米图片理解）；仅存本轮 user 消息的 image data URL / 外链。
alter table ai_messages
    add column if not exists attachments jsonb;

-- +goose Down
alter table ai_messages drop column if exists attachments;
