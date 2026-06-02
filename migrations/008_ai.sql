-- +goose Up
-- AI 功能全部：路由日志、场景生成、助理设置、检查点、记忆、技能、提示词层、限流、反馈、工具结果、技能候选、提示词实验、评估报告

-- ── AI 路由日志 ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_routing_logs (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id     UUID        NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    message_seq    INTEGER,
    planner_output JSONB,
    router_output  JSONB,
    per_hop        JSONB,
    outcome        TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_routing_logs_session_created
    ON ai_routing_logs(session_id, created_at DESC);

-- ── 场景生成任务 ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS scenario_gen_jobs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id      UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id  UUID        NOT NULL,
    config          JSONB       NOT NULL DEFAULT '{}',
    status          TEXT        NOT NULL DEFAULT 'pending',
    rounds          INTEGER     NOT NULL DEFAULT 0,
    result          JSONB,
    created_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scenario_gen_jobs_project_created
    ON scenario_gen_jobs(project_id, created_at DESC);

-- ── AI 助理平台级运行时配置 ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_assistant_settings (
    id         INT         PRIMARY KEY CHECK (id = 1),
    config     JSONB       NOT NULL DEFAULT '{}'::JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ai_assistant_settings (id, config)
VALUES (1, '{}'::JSONB)
ON CONFLICT (id) DO NOTHING;

-- ── AI 检查点 ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_checkpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    message_seq INT NOT NULL,
    state       JSONB NOT NULL DEFAULT '{}',
    label       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_checkpoints_session ON ai_checkpoints(session_id);

-- ── AI 记忆 ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_memories (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID REFERENCES projects(id) ON DELETE CASCADE,
    user_id      UUID,
    type         TEXT NOT NULL CHECK (type IN ('auto', 'project', 'global')),
    category     TEXT NOT NULL DEFAULT 'general',
    key          TEXT NOT NULL,
    content      TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'auto',
    confidence   REAL NOT NULL DEFAULT 0.8,
    ref_count    INT NOT NULL DEFAULT 0,
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_memories_project_user ON ai_memories(project_id, user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_memories_type ON ai_memories(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_memories_expires ON ai_memories(expires_at) WHERE expires_at IS NOT NULL AND deleted_at IS NULL;

-- ── AI 技能 ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_skills (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    label           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    prompt_template TEXT NOT NULL,
    tool_domains    JSONB DEFAULT '[]',
    input_schema    JSONB,
    require_confirm BOOLEAN NOT NULL DEFAULT false,
    triggers        JSONB DEFAULT '[]',
    created_by      UUID,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_skills_project ON ai_skills(project_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_skills_name ON ai_skills(name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skills_unique_project ON ai_skills(name, project_id)
    WHERE project_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skills_unique_global ON ai_skills(name)
    WHERE project_id IS NULL AND deleted_at IS NULL;

-- ── AI 提示词层 ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_prompt_layers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    domain     TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL,
    version    INT NOT NULL DEFAULT 1,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(name, version)
);

CREATE INDEX IF NOT EXISTS idx_ai_prompt_layers_name ON ai_prompt_layers(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_prompt_layers_domain ON ai_prompt_layers(domain) WHERE deleted_at IS NULL AND domain != '';

-- ── AI 限流 ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_rate_limit_rules (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    key        TEXT NOT NULL,
    "limit"    INT NOT NULL DEFAULT 60,
    "window"   INT NOT NULL DEFAULT 60,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_rate_limit_entries (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key        TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_entries_expires
    ON ai_rate_limit_entries(expires_at);
CREATE INDEX IF NOT EXISTS idx_rate_limit_entries_key_expires
    ON ai_rate_limit_entries(key, expires_at);

INSERT INTO ai_rate_limit_rules (name, key, "limit", "window") VALUES
    ('ai_chat_per_user', 'ai:chat', 60, 60),
    ('ai_generate_per_user', 'ai:generate', 30, 60),
    ('ai_analyze_per_user', 'ai:analyze', 30, 60),
    ('ai_scenario_gen_per_user', 'ai:scenario_gen', 10, 60),
    ('ai_background_per_user', 'ai:background', 5, 300)
ON CONFLICT (name) DO NOTHING;

-- ── AI 反馈 ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_feedback (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    message_seq INTEGER NOT NULL,
    rating      SMALLINT NOT NULL,
    correction  TEXT NOT NULL DEFAULT '',
    intent      TEXT NOT NULL DEFAULT '',
    domains     JSONB NOT NULL DEFAULT '[]',
    tools_used  JSONB NOT NULL DEFAULT '[]',
    created_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_feedback_session
    ON ai_feedback(session_id, message_seq) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_feedback_created
    ON ai_feedback(created_at DESC) WHERE deleted_at IS NULL;

-- ── AI 工具执行结果 ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_tool_outcomes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    message_seq INTEGER NOT NULL DEFAULT 0,
    tool_name   TEXT NOT NULL,
    domain      TEXT NOT NULL DEFAULT '',
    intent      TEXT NOT NULL DEFAULT '',
    success     BOOLEAN NOT NULL,
    error_type  TEXT NOT NULL DEFAULT '',
    hop_index   INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_tool_outcomes_tool
    ON ai_tool_outcomes(tool_name, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_tool_outcomes_session
    ON ai_tool_outcomes(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_tool_outcomes_created
    ON ai_tool_outcomes(created_at DESC);

-- ── AI 技能候选（自动发现）─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_skill_candidates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intent_pattern  TEXT NOT NULL DEFAULT '',
    tool_sequence   JSONB NOT NULL DEFAULT '[]',
    frequency       INTEGER NOT NULL DEFAULT 1,
    success_rate    REAL NOT NULL DEFAULT 0,
    avg_hops        REAL NOT NULL DEFAULT 0,
    sample_sessions JSONB NOT NULL DEFAULT '[]',
    status          TEXT NOT NULL DEFAULT 'pending',
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at     TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_skill_candidates_status
    ON ai_skill_candidates(status) WHERE deleted_at IS NULL;

-- ── AI 提示词 A/B 实验 ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_prompt_experiments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain        TEXT NOT NULL DEFAULT '',
    layer_name    TEXT NOT NULL DEFAULT '',
    variant_a_id  UUID REFERENCES ai_prompt_layers(id) ON DELETE SET NULL,
    variant_b_id  UUID REFERENCES ai_prompt_layers(id) ON DELETE SET NULL,
    traffic_split REAL NOT NULL DEFAULT 0.5,
    metric        TEXT NOT NULL DEFAULT 'success_rate',
    variant_a_val REAL NOT NULL DEFAULT 0,
    variant_b_val REAL NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'running',
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at      TIMESTAMPTZ,
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_prompt_experiments_status
    ON ai_prompt_experiments(status) WHERE deleted_at IS NULL;

-- ── AI 评估报告 ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_eval_reports (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    period_start      TIMESTAMPTZ NOT NULL,
    period_end        TIMESTAMPTZ NOT NULL,
    routing_accuracy  REAL NOT NULL DEFAULT 0,
    tool_efficiency   REAL NOT NULL DEFAULT 0,
    user_satisfaction REAL NOT NULL DEFAULT 0,
    tool_success_rate REAL NOT NULL DEFAULT 0,
    skill_coverage    REAL NOT NULL DEFAULT 0,
    suggestions       JSONB NOT NULL DEFAULT '[]',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_eval_reports_created
    ON ai_eval_reports(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_eval_reports;
DROP TABLE IF EXISTS ai_prompt_experiments;
DROP TABLE IF EXISTS ai_skill_candidates;
DROP TABLE IF EXISTS ai_tool_outcomes;
DROP TABLE IF EXISTS ai_feedback;
DROP TABLE IF EXISTS ai_rate_limit_entries;
DROP TABLE IF EXISTS ai_rate_limit_rules;
DROP TABLE IF EXISTS ai_prompt_layers;
DROP TABLE IF EXISTS ai_skills;
DROP TABLE IF EXISTS ai_memories;
DROP TABLE IF EXISTS ai_checkpoints;
DROP TABLE IF EXISTS ai_assistant_settings;
DROP TABLE IF EXISTS scenario_gen_jobs;
DROP TABLE IF EXISTS ai_routing_logs;
