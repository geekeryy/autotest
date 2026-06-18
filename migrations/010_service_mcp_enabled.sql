-- +goose Up
-- 服务级 MCP 自动化开关（管理后台「服务与环境」配置）

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS mcp_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE services DROP COLUMN IF EXISTS mcp_enabled;
