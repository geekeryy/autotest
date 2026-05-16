-- +goose Up
-- 全局 AI 助理浮窗与未来对话型 AI 入口共用的会话/消息表。
-- 每条会话按 user × project 隔离：跨用户的查询/写入由 application 层禁止。
create table if not exists ai_sessions (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects (id) on delete cascade,
    user_id     uuid        not null references users (id) on delete cascade,
    title       text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create index if not exists idx_ai_sessions_owner_active
    on ai_sessions (project_id, user_id, updated_at desc)
    where deleted_at is null;

-- 每条消息覆盖 LLM 对话的四种 role：system / user / assistant / tool。
-- seq 在 session 内单调递增，便于前端做增量渲染与冲突检查；
-- status 控制 mutating 工具的 human-in-the-loop 流程：
--   - 'final'           普通消息（默认）
--   - 'pending_confirm' assistant 消息且 tool_calls 中含 mutating 调用，等待用户确认
--   - 'rejected'        mutating 调用被用户拒绝；之后会再追加一条 role=tool 的 final 消息记录拒绝原因
-- tool_calls 保存 OpenAI 风格的工具调用元数据数组；tool 消息则使用 tool_call_id 反向引用。
create table if not exists ai_messages (
    id              uuid        primary key default gen_random_uuid(),
    session_id      uuid        not null references ai_sessions (id) on delete cascade,
    seq             integer     not null,
    role            text        not null check (role in ('system','user','assistant','tool')),
    content         text        not null default '',
    tool_call_id    text,
    tool_calls      jsonb,
    status          text        not null default 'final'
                    check (status in ('final','pending_confirm','rejected')),
    model           text,
    elapsed_millis  integer,
    created_at      timestamptz not null default now()
);

create unique index if not exists ux_ai_messages_session_seq
    on ai_messages (session_id, seq);

create index if not exists idx_ai_messages_pending
    on ai_messages (session_id, created_at desc)
    where status = 'pending_confirm';

-- +goose Down
drop index if exists idx_ai_messages_pending;
drop index if exists ux_ai_messages_session_seq;
drop table if exists ai_messages;
drop index if exists idx_ai_sessions_owner_active;
drop table if exists ai_sessions;
