-- +goose Up
-- Mock Route 支持 Schema 自动生成模式 + AI 预生成缓存池 + 录制回放

-- 新增 response_mode 选项 'schema' 和 'record' 和 'replay'
ALTER TABLE mock_routes DROP CONSTRAINT IF EXISTS mock_routes_response_mode_check;
ALTER TABLE mock_routes
    ADD CONSTRAINT mock_routes_response_mode_check
    CHECK (response_mode IN ('body', 'redirect', 'schema', 'record', 'replay'));

-- Schema 字段：存储 JSON Schema，用于 schema 模式自动生成响应
ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS response_schema JSONB NOT NULL DEFAULT '{}'::JSONB;

-- AI 预生成的数据池
ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS ai_generated_pool JSONB NOT NULL DEFAULT '[]'::JSONB;

ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS ai_generated_at TIMESTAMPTZ;

-- 录制目标 URL（record 模式使用）
ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS record_target_url TEXT NOT NULL DEFAULT '';

-- ── Mock 录制数据 ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS mock_recordings (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    mock_server_id  UUID        NOT NULL REFERENCES mock_servers(id) ON DELETE CASCADE,
    mock_route_id   UUID        NOT NULL REFERENCES mock_routes(id) ON DELETE CASCADE,
    method          TEXT        NOT NULL,
    path            TEXT        NOT NULL,
    query_hash      TEXT        NOT NULL DEFAULT '',
    request_body    JSONB       NOT NULL DEFAULT '{}'::JSONB,
    status_code     INTEGER     NOT NULL,
    headers         JSONB       NOT NULL DEFAULT '{}'::JSONB,
    body            JSONB       NOT NULL DEFAULT '{}'::JSONB,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    hit_count       INTEGER     NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mock_recordings_match
    ON mock_recordings(mock_server_id, method, path, query_hash);

CREATE INDEX IF NOT EXISTS idx_mock_recordings_route
    ON mock_recordings(mock_server_id, mock_route_id);

-- +goose Down
DROP TABLE IF EXISTS mock_recordings;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS record_target_url;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS ai_generated_at;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS ai_generated_pool;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS response_schema;
ALTER TABLE mock_routes DROP CONSTRAINT IF EXISTS mock_routes_response_mode_check;
ALTER TABLE mock_routes
    ADD CONSTRAINT mock_routes_response_mode_check
    CHECK (response_mode IN ('body', 'redirect'));
