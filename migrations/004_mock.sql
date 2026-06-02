-- +goose Up
-- Mock Server 功能扩展：重定向支持 + 访问日志

-- ── Mock 路由支持 HTTP 重定向响应 ──────────────────────────────────────────────

ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS response_mode TEXT NOT NULL DEFAULT 'body'
        CHECK (response_mode IN ('body', 'redirect'));

ALTER TABLE mock_routes
    ADD COLUMN IF NOT EXISTS redirect_location TEXT NOT NULL DEFAULT '';

-- ── Mock 访问日志 ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS mock_access_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    mock_server_id  UUID        NOT NULL REFERENCES mock_servers(id) ON DELETE CASCADE,
    mock_route_id   UUID        REFERENCES mock_routes(id) ON DELETE SET NULL,
    method          TEXT        NOT NULL,
    path            TEXT        NOT NULL,
    query           TEXT        NOT NULL DEFAULT '',
    client_ip       TEXT        NOT NULL DEFAULT '',
    matched         BOOLEAN     NOT NULL DEFAULT false,
    response_status INTEGER     NOT NULL,
    duration_ms     INTEGER     NOT NULL,
    error_message   TEXT        NOT NULL DEFAULT '',
    request_headers JSONB       NOT NULL DEFAULT '{}'::JSONB,
    request_body    TEXT        NOT NULL DEFAULT '',
    response_headers JSONB      NOT NULL DEFAULT '{}'::JSONB,
    response_body   TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mock_access_logs_server_created
    ON mock_access_logs(mock_server_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS mock_access_logs;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS redirect_location;
ALTER TABLE mock_routes DROP COLUMN IF EXISTS response_mode;
