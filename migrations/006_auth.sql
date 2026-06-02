-- +goose Up
-- 认证与权限体系：GitHub OAuth、认证提供商、审计日志

-- ── GitHub OAuth 支持 ──────────────────────────────────────────────────────────

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'local';

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS github_id BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS users_github_id_key ON users(github_id) WHERE github_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_active_auth_provider ON users(active, auth_provider);

-- ── 强制改密码标记 ─────────────────────────────────────────────────────────────

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

-- ── OAuth 认证提供商 ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS auth_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_type   TEXT NOT NULL CHECK (provider_type IN ('github', 'gitlab', 'google')),
    name            TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    client_id       TEXT NOT NULL DEFAULT '',
    client_secret   TEXT NOT NULL DEFAULT '',
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    extra_config    JSONB NOT NULL DEFAULT '{}',
    auto_register   BOOLEAN NOT NULL DEFAULT true,
    default_active  BOOLEAN NOT NULL DEFAULT false,
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_auth_providers_enabled
    ON auth_providers(sort_order ASC, created_at ASC)
    WHERE deleted_at IS NULL AND enabled;

-- ── 用户外部身份关联 ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS user_external_identities (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL,
    external_id   TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_type, external_id)
);

CREATE INDEX IF NOT EXISTS idx_user_external_identities_user
    ON user_external_identities(user_id);

-- 将已有 github_id 迁移到外部身份表
INSERT INTO user_external_identities (user_id, provider_type, external_id)
SELECT id, 'github', github_id::TEXT
FROM users
WHERE github_id IS NOT NULL
ON CONFLICT (provider_type, external_id) DO NOTHING;

-- ── OAuth 路由标识 Slug ────────────────────────────────────────────────────────

ALTER TABLE auth_providers ADD COLUMN IF NOT EXISTS slug TEXT;

UPDATE auth_providers
SET slug = provider_type || '-' || substr(replace(id::TEXT, '-', ''), 1, 8)
WHERE slug IS NULL OR slug = '';

ALTER TABLE auth_providers ALTER COLUMN slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_providers_slug
    ON auth_providers(slug)
    WHERE deleted_at IS NULL;

-- ── OAuth 回调 URL ─────────────────────────────────────────────────────────────

ALTER TABLE auth_providers ADD COLUMN IF NOT EXISTS callback_url TEXT NOT NULL DEFAULT '';

-- ── 可信前端域名（从全局表迁移到 auth_providers）───────────────────────────────

-- 先创建临时全局表用于迁移
CREATE TABLE IF NOT EXISTS oauth_trusted_origins (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    origin     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (origin)
);

ALTER TABLE auth_providers
    ADD COLUMN IF NOT EXISTS trusted_frontend_origins TEXT[] NOT NULL DEFAULT '{}';

UPDATE auth_providers ap
SET trusted_frontend_origins = sub.origins
FROM (
    SELECT coalesce(array_agg(origin ORDER BY origin), '{}'::TEXT[]) AS origins
    FROM oauth_trusted_origins
) sub
WHERE ap.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM oauth_trusted_origins);

DROP TABLE IF EXISTS oauth_trusted_origins;

-- ── 审计日志 ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    action          TEXT        NOT NULL,
    success         BOOLEAN     NOT NULL,
    actor_user_id   UUID        REFERENCES users(id) ON DELETE SET NULL,
    actor_username  TEXT        NOT NULL DEFAULT '',
    resource        TEXT        NOT NULL DEFAULT '',
    client_ip       TEXT        NOT NULL DEFAULT '',
    user_agent      TEXT        NOT NULL DEFAULT '',
    detail          JSONB       NOT NULL DEFAULT '{}'::JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
    ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created
    ON audit_logs(action, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS user_external_identities;
DROP TABLE IF EXISTS auth_providers;
ALTER TABLE users DROP COLUMN IF EXISTS must_change_password;
ALTER TABLE users DROP COLUMN IF EXISTS github_id;
ALTER TABLE users DROP COLUMN IF EXISTS auth_provider;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
