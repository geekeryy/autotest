-- +goose Up
-- 服务绑定的 MCP 专用 API Key（由平台在接入配置时自动创建）

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS mcp_api_key_id uuid REFERENCES api_keys(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE services DROP COLUMN IF EXISTS mcp_api_key_id;
