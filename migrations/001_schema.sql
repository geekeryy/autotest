-- +goose Up
create extension if not exists pgcrypto;

-- ── 核心业务表 ────────────────────────────────────────────────────────────────

create table if not exists projects (
    id          uuid        primary key default gen_random_uuid(),
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists services (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists environments (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    service_id  uuid        references services(id) on delete cascade,
    name        text        not null,
    base_url    text        not null default '',
    variables   jsonb       not null default '{}'::jsonb,
    auth        jsonb       not null default '{}'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists service_environment_configs (
    id             uuid        primary key default gen_random_uuid(),
    project_id     uuid        not null references projects(id) on delete cascade,
    service_id     uuid        not null references services(id) on delete cascade,
    environment_id uuid        not null references environments(id) on delete cascade,
    base_url       text        not null,
    variables      jsonb       not null default '{}'::jsonb,
    auth           jsonb       not null default '{}'::jsonb,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    deleted_at     timestamptz
);

create table if not exists api_specs (
    id                  uuid        primary key default gen_random_uuid(),
    service_id          uuid        not null references services(id) on delete cascade,
    version             integer     not null,
    content_hash        text        not null,
    raw_content         bytea       not null,
    normalized_snapshot jsonb       not null default '{}'::jsonb,
    status              text        not null default 'imported',
    created_at          timestamptz not null default now(),
    deleted_at          timestamptz,
    unique (service_id, version),
    unique (service_id, content_hash)
);

create table if not exists api_endpoints (
    id              uuid        primary key default gen_random_uuid(),
    service_id      uuid        not null references services(id) on delete cascade,
    spec_id         uuid        not null references api_specs(id) on delete cascade,
    method          text        not null,
    path            text        not null,
    operation_id    text        not null default '',
    summary         text        not null default '',
    tags            text[]      not null default '{}',
    request_schema  jsonb       not null default '{}'::jsonb,
    response_schema jsonb       not null default '{}'::jsonb,
    fingerprint     text        not null,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    deleted_at      timestamptz,
    unique (service_id, method, path)
);

create table if not exists test_cases (
    id                     uuid        primary key default gen_random_uuid(),
    project_id             uuid        not null references projects(id) on delete cascade,
    service_id             uuid        not null references services(id) on delete cascade,
    endpoint_id            uuid        references api_endpoints(id) on delete set null,
    source                 text        not null check (source in ('auto', 'manual', 'derived')),
    name                   text        not null,
    method                 text        not null,
    path                   text        not null,
    fingerprint            text        not null unique,
    parent_case_id         uuid        references test_cases(id) on delete set null,
    generation_rule_id     text        not null default '',
    request                jsonb       not null default '{}'::jsonb,
    assertions             jsonb       not null default '[]'::jsonb,
    status                 text        not null default 'active' check (status in ('draft', 'active', 'disabled')),
    last_response_snapshot jsonb       not null default '{}'::jsonb,
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now(),
    deleted_at             timestamptz
);

create table if not exists test_case_steps (
    id            uuid        primary key default gen_random_uuid(),
    test_case_id  uuid        not null references test_cases(id) on delete cascade,
    step_order    integer     not null default 1,
    name          text        not null,
    request       jsonb       not null default '{}'::jsonb,
    assertions    jsonb       not null default '[]'::jsonb,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz,
    unique (test_case_id, step_order)
);

-- ── 业务数据源与 SQL 参数源 ────────────────────────────────────────────────────

create table if not exists business_data_sources (
    id              uuid        primary key default gen_random_uuid(),
    project_id      uuid        not null references projects(id) on delete cascade,
    name            text        not null,
    description     text        not null default '',
    driver          text        not null default 'postgres' check (driver in ('postgres')),
    host            text        not null,
    port            integer     not null default 5432,
    database_name   text        not null,
    username        text        not null default '',
    password_secret text        not null default '',
    ssl_mode        text        not null default 'disable',
    enabled         boolean     not null default true,
    options         jsonb       not null default '{}'::jsonb,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    deleted_at      timestamptz
);

create table if not exists sql_parameter_sources (
    id             uuid        primary key default gen_random_uuid(),
    project_id     uuid        not null references projects(id) on delete cascade,
    service_id     uuid        not null references services(id) on delete cascade,
    data_source_id uuid        not null references business_data_sources(id) on delete restrict,
    name           text        not null,
    description    text        not null default '',
    sql_text       text        not null,
    input_params   jsonb       not null default '[]'::jsonb,
    source_key     text        not null default '',
    enabled        boolean     not null default true,
    timeout_millis integer     not null default 3000,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    deleted_at     timestamptz
);

-- ── 场景编排 ──────────────────────────────────────────────────────────────────

create table if not exists test_scenarios (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    service_id  uuid        not null references services(id) on delete cascade,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists test_scenario_steps (
    id                   uuid        primary key default gen_random_uuid(),
    scenario_id          uuid        not null references test_scenarios(id) on delete cascade,
    test_case_id         uuid        references test_cases(id) on delete cascade,
    step_order           integer     not null default 1,
    step_seq             integer     not null default 0,
    name                 text        not null default '',
    enabled              boolean     not null default true,
    step_type            text        not null default 'api',
    config               jsonb       not null default '{}'::jsonb,
    request_override     jsonb       not null default '{}'::jsonb,
    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now(),
    deleted_at           timestamptz,
    unique (scenario_id, step_order),
    constraint test_scenario_steps_step_type_check
        check (step_type in ('api', 'database', 'script', 'for', 'condition')),
    constraint test_scenario_steps_api_case_check
        check (step_type <> 'api' or test_case_id is not null)
);

create table if not exists test_suites (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    service_id  uuid        not null references services(id) on delete cascade,
    name        text        not null,
    kind        text        not null check (kind in ('manual', 'auto', 'mixed')),
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists test_suite_items (
    id           uuid        primary key default gen_random_uuid(),
    suite_id     uuid        not null references test_suites(id) on delete cascade,
    test_case_id uuid        not null references test_cases(id) on delete cascade,
    item_order   integer     not null default 1,
    source       text        not null check (source in ('manual_added', 'auto_generated', 'auto_selected')),
    enabled      boolean     not null default true,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    deleted_at   timestamptz,
    unique (suite_id, test_case_id)
);

create table if not exists test_runs (
    id                            uuid        primary key default gen_random_uuid(),
    project_id                    uuid        not null references projects(id) on delete cascade,
    service_id                    uuid        not null references services(id) on delete cascade,
    suite_id                      uuid        references test_suites(id) on delete set null,
    scenario_id                   uuid        references test_scenarios(id) on delete set null,
    environment_id                uuid        references environments(id) on delete set null,
    service_environment_config_id uuid        references service_environment_configs(id) on delete set null,
    name                          text        not null default '',
    status                        text        not null default 'queued' check (status in ('queued', 'running', 'passed', 'failed', 'cancelled')),
    variables                     jsonb       not null default '{}'::jsonb,
    snapshot                      jsonb       not null default '{}'::jsonb,
    started_at                    timestamptz,
    finished_at                   timestamptz,
    created_at                    timestamptz not null default now(),
    deleted_at                    timestamptz
);

create table if not exists test_run_results (
    id                         uuid        primary key default gen_random_uuid(),
    run_id                     uuid        not null references test_runs(id) on delete cascade,
    test_case_id               uuid        references test_cases(id) on delete cascade,
    step_id                    uuid,
    status                     text        not null check (status in ('passed', 'failed', 'error')),
    duration_millis            bigint      not null default 0,
    request_snapshot           jsonb       not null default '{}'::jsonb,
    response_snapshot          jsonb       not null default '{}'::jsonb,
    assertions                 jsonb       not null default '[]'::jsonb,
    parameter_source_snapshots jsonb       not null default '[]'::jsonb,
    error                      text        not null default '',
    created_at                 timestamptz not null default now(),
    deleted_at                 timestamptz
);

-- ── 认证与权限表 ──────────────────────────────────────────────────────────────

create table if not exists users (
    id            uuid        primary key default gen_random_uuid(),
    username      text        not null unique,
    password_hash text        not null,
    display_name  text        not null default '',
    email         text        not null default '',
    avatar_jpeg   bytea,
    active        boolean     not null default true,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create table if not exists roles (
    id          uuid        primary key default gen_random_uuid(),
    code        text        not null unique,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table if not exists permissions (
    id          uuid        primary key default gen_random_uuid(),
    code        text        not null unique,
    name        text        not null,
    description text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table if not exists user_roles (
    user_id    uuid        not null references users(id) on delete cascade,
    role_id    uuid        not null references roles(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (user_id, role_id)
);

create table if not exists role_permissions (
    role_id       uuid        not null references roles(id) on delete cascade,
    permission_id uuid        not null references permissions(id) on delete cascade,
    created_at    timestamptz not null default now(),
    primary key (role_id, permission_id)
);

create table if not exists project_members (
    project_id uuid not null references projects(id) on delete cascade,
    user_id    uuid not null references users(id) on delete cascade,
    role       text not null default 'developer',
    created_at timestamptz not null default now(),
    primary key (project_id, user_id),
    constraint project_member_role_valid check (role in ('owner', 'developer', 'viewer'))
);

-- ── Mock Server ───────────────────────────────────────────────────────────────

create table if not exists mock_servers (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    name        text        not null,
    description text        not null default '',
    port        integer     not null check (port between 1 and 65535),
    auto_start  boolean     not null default false,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists mock_routes (
    id                uuid        primary key default gen_random_uuid(),
    mock_server_id    uuid        not null references mock_servers(id) on delete cascade,
    method            text        not null,
    path              text        not null,
    priority          integer     not null default 0,
    enabled           boolean     not null default true,
    request_match     jsonb       not null default '{}'::jsonb,
    response_status   integer     not null default 200 check (response_status between 100 and 599),
    response_headers  jsonb       not null default '{}'::jsonb,
    response_body     text        not null default '',
    response_body_type text       not null default 'json' check (response_body_type in ('json', 'text', 'raw')),
    delay_millis      integer     not null default 0 check (delay_millis >= 0),
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now(),
    deleted_at        timestamptz
);

create table if not exists mock_value_sets (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    key         text        not null,
    name        text        not null,
    description text        not null default '',
    values      jsonb       not null default '[]'::jsonb,
    weights     jsonb       not null default '[]'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

-- ── AI 提供商、提示词与会话 ─────────────────────────────────────────────────────

create table if not exists ai_providers (
    id            uuid        primary key default gen_random_uuid(),
    project_id    uuid        not null references projects(id) on delete cascade,
    name          text        not null,
    provider_type text        not null check (provider_type in
                    ('deepseek', 'xiaomi', 'openai', 'anthropic', 'kimi', 'ollama')),
    base_url      text        not null default '',
    api_key       text        not null default '',
    default_model text        not null default '',
    extra_config  jsonb       not null default '{}'::jsonb,
    enabled       boolean     not null default true,
    is_default    boolean     not null default false,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz
);

create table if not exists project_ai_prompts (
    id            uuid        primary key default gen_random_uuid(),
    project_id    uuid        not null references projects(id) on delete cascade,
    provider_id   uuid        references ai_providers(id) on delete set null,
    action        text        not null check (action in (
        'generate_params',
        'generate_assertion',
        'generate_case_data',
        'analyze_failure',
        'analyze_spec_changes',
        'raw'
    )),
    name          text        not null default '',
    system_prompt text        not null default '',
    default_model text        not null default '',
    enabled       boolean     not null default true,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    deleted_at    timestamptz
);

create table if not exists ai_sessions (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    user_id     uuid        not null references users(id) on delete cascade,
    title       text        not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists ai_messages (
    id                uuid        primary key default gen_random_uuid(),
    session_id        uuid        not null references ai_sessions(id) on delete cascade,
    seq               integer     not null,
    role              text        not null check (role in ('system', 'user', 'assistant', 'tool')),
    content           text        not null default '',
    reasoning_content text        not null default '',
    tool_call_id      text,
    tool_calls        jsonb,
    attachments       jsonb,
    status            text        not null default 'final'
                      check (status in ('final', 'pending_confirm', 'rejected')),
    model             text,
    elapsed_millis    integer,
    created_at        timestamptz not null default now()
);

-- ── 测试数据表与 API Key ───────────────────────────────────────────────────────

create table if not exists test_data_tables (
    id          uuid        primary key default gen_random_uuid(),
    project_id  uuid        not null references projects(id) on delete cascade,
    key         text        not null,
    name        text        not null,
    description text        not null default '',
    columns     jsonb       not null default '[]'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    deleted_at  timestamptz
);

create table if not exists test_data_rows (
    id         uuid        primary key default gen_random_uuid(),
    table_id   uuid        not null references test_data_tables(id) on delete cascade,
    row_index  int         not null default 0,
    values     jsonb       not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists api_keys (
    id           uuid        primary key default gen_random_uuid(),
    name         text        not null,
    token_hash   text        not null unique,
    token_prefix text        not null,
    token_suffix text        not null,
    scopes       text[]      not null default array['specs:import']::text[],
    created_by   uuid        not null references users(id) on delete cascade,
    enabled      boolean     not null default true,
    expires_at   timestamptz,
    last_used_at timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    deleted_at   timestamptz
);

create table if not exists script_library_templates (
    id uuid primary key default gen_random_uuid(),
    project_id uuid references projects (id) on delete cascade,
    name text not null,
    category text not null default '自定义',
    description text,
    code text not null,
    scopes jsonb not null default '["assertion", "scenario"]'::jsonb,
    is_builtin boolean not null default false,
    builtin_key text,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_script_library_templates_builtin_project
        check (
                (is_builtin = true and project_id is null)
                or (is_builtin = false and project_id is not null)
            )
);

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '状态码与 JSON 业务码',
       '响应断言',
       '校验 HTTP 状态码与常见 body.code；按需改路径与期望值',
       $c1$pm.test('HTTP 状态码为 200', () => {
  pm.response.to.have.status(200);
});

pm.test('业务码为 0', () => {
  const body = pm.response.json();
  pm.expect(body).to.have.property('code');
  pm.expect(body.code).to.equal(0);
});$c1$,
       '["assertion"]'::jsonb,
       true,
       'assert-status-json-code',
       10
where not exists (select 1 from script_library_templates where builtin_key = 'assert-status-json-code');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       'JSON 字段存在与类型',
       '响应断言',
       '检查 data.id 存在且为字符串；修改 data 路径',
       $c2$pm.test('data.id 为字符串', () => {
  const body = pm.response.json();
  pm.expect(body.data).to.exist;
  pm.expect(body.data.id).to.be.a('string').and.not.empty;
});$c2$,
       '["assertion"]'::jsonb,
       true,
       'assert-json-field',
       20
where not exists (select 1 from script_library_templates where builtin_key = 'assert-json-field');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '响应头包含',
       '响应断言',
       '校验响应头；修改名称与可选期望值',
       $c3$pm.test('Content-Type 存在', () => {
  pm.response.to.have.header('content-type');
});

pm.test('Authorization 格式', () => {
  pm.response.to.have.header('authorization', /Bearer\s+\S+/);
});$c3$,
       '["assertion"]'::jsonb,
       true,
       'assert-header',
       30
where not exists (select 1 from script_library_templates where builtin_key = 'assert-header');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '响应时间上限',
       '响应断言',
       '毫秒上限可按需调整',
       $c4$pm.test('响应时间不超过 500ms', () => {
  pm.expect(pm.response.responseTime).to.be.below(500);
});$c4$,
       '["assertion"]'::jsonb,
       true,
       'assert-response-time',
       40
where not exists (select 1 from script_library_templates where builtin_key = 'assert-response-time');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '写入场景变量',
       '场景脚本',
       'pm.variables.set 供后续步骤 {{变量名}} 使用',
       $c5$// 将常量或拼接结果写入场景变量（按需改名）
pm.variables.set('myToken', 'REPLACE_WITH_TOKEN');
pm.variables.set('requestId', String(Date.now()));

console.log('已设置 myToken / requestId');$c5$,
       '["scenario"]'::jsonb,
       true,
       'scenario-set-vars',
       50
where not exists (select 1 from script_library_templates where builtin_key = 'scenario-set-vars');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '从上一步结果取字段',
       '场景脚本',
       '配合 {{$steps[N].body.xxx}} 占位；将 N 改为实际步骤序号',
       $c6$// 假设上一步 API 已将 body 写入变量 token（或通过模板引用解析）
// 若上游步骤号为 1，可在 SQL/路径中使用 {{$steps[1].body.data.id}}

const raw = pm.variables.get('upstreamPayload'); // 若上游 pm.variables.set 过
if (raw) {
  try {
    const o = JSON.parse(raw);
    pm.variables.set('extractedId', String(o.id || ''));
  } catch (e) {
    console.error('解析 upstreamPayload 失败', e.message);
  }
}$c6$,
       '["scenario"]'::jsonb,
       true,
       'scenario-from-prev-step',
       60
where not exists (select 1 from script_library_templates where builtin_key = 'scenario-from-prev-step');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '脚本内断言块',
       '场景脚本',
       '场景脚本无 pm.response，用 assert() 与 pm.test 组合（无 pm.expect）',
       $c7$pm.test('变量已配置', () => {
  const v = pm.variables.get('必填变量名');
  assert(v && String(v).length > 0, '必填变量名不能为空');
});

pm.test('数值为正', () => {
  const n = Number(pm.variables.get('某数字变量'));
  assert(!isNaN(n) && n > 0, '某数字变量须为大于 0 的数字');
});$c7$,
       '["scenario"]'::jsonb,
       true,
       'scenario-pm-test',
       70
where not exists (select 1 from script_library_templates where builtin_key = 'scenario-pm-test');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '打印变量调试',
       '场景脚本',
       '输出到步骤 stdout，便于运行结果中查看',
       $c8$console.log('当前变量 token =', pm.variables.get('token'));
console.log('environment 同名读取 =', pm.environment.get('token'));$c8$,
       '["scenario"]'::jsonb,
       true,
       'scenario-console-debug',
       80
where not exists (select 1 from script_library_templates where builtin_key = 'scenario-console-debug');

insert into script_library_templates (
    project_id, name, category, description, code, scopes, is_builtin, builtin_key, sort_order
)
select null,
       '布尔条件断言',
       '通用',
       'assert / pm.test 任选；断言脚本与场景脚本均可用',
       $c9$pm.test('自定义条件', () => {
  assert(pm.variables.get('flag') === 'true', 'flag 应为 true');
});$c9$,
       '["assertion","scenario"]'::jsonb,
       true,
       'common-assert-boolean',
       90
where not exists (select 1 from script_library_templates where builtin_key = 'common-assert-boolean');

-- ── 兼容旧库（表已存在时 CREATE TABLE 不会补列）────────────────────────────────

alter table users
    add column if not exists avatar_jpeg bytea;

alter table mock_servers
    add column if not exists auto_start boolean not null default false;

alter table project_ai_prompts
    add column if not exists provider_id uuid references ai_providers (id) on delete set null;

alter table project_ai_prompts
    drop constraint if exists project_ai_prompts_action_check;

alter table project_ai_prompts
    add constraint project_ai_prompts_action_check
    check (action in (
        'generate_params',
        'generate_assertion',
        'generate_case_data',
        'analyze_failure',
        'analyze_spec_changes',
        'raw'
    ));

alter table ai_messages
    add column if not exists reasoning_content text not null default '';

alter table ai_messages
    add column if not exists attachments jsonb;

alter table ai_messages
    add column if not exists usage_details jsonb;

-- ── 索引 ──────────────────────────────────────────────────────────────────────

create index if not exists idx_projects_active
    on projects(created_at desc) where deleted_at is null;

create index if not exists idx_services_project
    on services(project_id);
create index if not exists idx_services_project_active
    on services(project_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_services_project_name_active
    on services(project_id, name) where deleted_at is null;

create index if not exists idx_environments_project_active
    on environments(project_id, created_at desc) where deleted_at is null;
create index if not exists idx_environments_service_active
    on environments(service_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_environments_service_name_active
    on environments(service_id, name) where deleted_at is null;

create unique index if not exists idx_service_environment_configs_active
    on service_environment_configs(service_id, environment_id) where deleted_at is null;
create index if not exists idx_service_environment_configs_project_active
    on service_environment_configs(project_id, service_id, environment_id) where deleted_at is null;

create index if not exists idx_api_specs_service
    on api_specs(service_id);
create index if not exists idx_api_specs_service_active
    on api_specs(service_id, version desc) where deleted_at is null;

create index if not exists idx_api_endpoints_service
    on api_endpoints(service_id);
create index if not exists idx_api_endpoints_service_active
    on api_endpoints(service_id, path, method) where deleted_at is null;

create index if not exists idx_test_cases_service_source
    on test_cases(service_id, source);
create index if not exists idx_test_cases_project_service_active
    on test_cases(project_id, service_id, created_at desc) where deleted_at is null;
create unique index if not exists idx_test_cases_auto_generation_active_unique
    on test_cases(project_id, service_id, endpoint_id, generation_rule_id)
    where deleted_at is null
      and source = 'auto'
      and endpoint_id is not null
      and generation_rule_id <> '';

create index if not exists idx_test_cases_parent_active
    on test_cases(parent_case_id, created_at asc)
    where deleted_at is null and parent_case_id is not null;

-- 006 迁移后 project_id 已移除；仅在列仍存在时创建项目级索引
do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'business_data_sources'
          and column_name = 'project_id'
    ) then
        execute $idx$
            create index if not exists idx_business_data_sources_project_list_active
                on business_data_sources(project_id, created_at desc)
                where deleted_at is null
        $idx$;
        execute $idx$
            create unique index if not exists idx_business_data_sources_project_name_active
                on business_data_sources(project_id, name)
                where deleted_at is null
        $idx$;
    end if;
end $$;

create index if not exists idx_sql_parameter_sources_scope_active
    on sql_parameter_sources(project_id, service_id, created_at desc)
    where deleted_at is null;

create index if not exists idx_sql_parameter_sources_data_source_active
    on sql_parameter_sources(data_source_id)
    where deleted_at is null;

create unique index if not exists idx_sql_parameter_sources_scope_name_active
    on sql_parameter_sources(project_id, service_id, name)
    where deleted_at is null;

create unique index if not exists idx_sql_parameter_sources_scope_key_active
    on sql_parameter_sources(project_id, service_id, source_key)
    where deleted_at is null and source_key <> '';

create index if not exists idx_test_suite_items_suite_order
    on test_suite_items(suite_id, item_order);
create index if not exists idx_test_suite_items_suite_active
    on test_suite_items(suite_id, item_order, created_at) where deleted_at is null;

create index if not exists idx_test_suites_project_service_active
    on test_suites(project_id, service_id, created_at desc) where deleted_at is null;

create index if not exists idx_test_runs_project_status
    on test_runs(project_id, status);
create index if not exists idx_test_runs_project_active
    on test_runs(project_id, created_at desc) where deleted_at is null;
create index if not exists idx_test_runs_scenario_finished
    on test_runs(scenario_id, finished_at desc)
    where deleted_at is null and scenario_id is not null;
create index if not exists idx_test_runs_scenario_finished
    on test_runs(scenario_id, finished_at desc)
    where deleted_at is null and scenario_id is not null;

create index if not exists idx_test_run_results_run
    on test_run_results(run_id);
create index if not exists idx_test_run_results_run_active
    on test_run_results(run_id, created_at desc) where deleted_at is null;

create index if not exists idx_user_roles_role
    on user_roles(role_id);
create index if not exists idx_role_permissions_permission
    on role_permissions(permission_id);

create index if not exists project_members_user_id_idx on project_members(user_id);

create index if not exists idx_script_library_templates_project_id
    on script_library_templates (project_id);

create unique index if not exists idx_script_library_templates_builtin_key
    on script_library_templates (builtin_key)
    where builtin_key is not null;

create index if not exists idx_mock_servers_project_active
    on mock_servers(project_id, created_at desc) where deleted_at is null;

create unique index if not exists idx_mock_servers_project_name_active
    on mock_servers(project_id, name) where deleted_at is null;

create unique index if not exists idx_mock_servers_port_active
    on mock_servers(port) where deleted_at is null;

create index if not exists idx_mock_servers_auto_start_active
    on mock_servers(auto_start)
    where deleted_at is null and auto_start = true;

create index if not exists idx_mock_routes_server_active
    on mock_routes(mock_server_id, method, path, priority desc, created_at asc)
    where deleted_at is null;

create unique index if not exists ux_mock_value_sets_project_key
    on mock_value_sets(project_id, key)
    where deleted_at is null;

create index if not exists idx_mock_value_sets_project_active
    on mock_value_sets(project_id, created_at desc)
    where deleted_at is null;

do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'ai_providers'
          and column_name = 'project_id'
    ) then
        execute $idx$
            create unique index if not exists idx_ai_providers_project_name_active
                on ai_providers(project_id, name) where deleted_at is null
        $idx$;
        execute $idx$
            create unique index if not exists idx_ai_providers_project_default_active
                on ai_providers(project_id) where is_default and deleted_at is null
        $idx$;
        execute $idx$
            create index if not exists idx_ai_providers_project_active
                on ai_providers(project_id, created_at desc) where deleted_at is null
        $idx$;
    end if;

    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public'
          and table_name = 'project_ai_prompts'
          and column_name = 'project_id'
    ) then
        execute $idx$
            create unique index if not exists project_ai_prompts_action_idx
                on project_ai_prompts(project_id, action) where deleted_at is null
        $idx$;
        execute $idx$
            create index if not exists idx_project_ai_prompts_project_provider
                on project_ai_prompts(project_id, provider_id)
                where deleted_at is null
        $idx$;
    end if;
end $$;

create index if not exists idx_ai_sessions_owner_active
    on ai_sessions(project_id, user_id, updated_at desc)
    where deleted_at is null;

create unique index if not exists ux_ai_messages_session_seq
    on ai_messages(session_id, seq);

create index if not exists idx_ai_messages_pending
    on ai_messages(session_id, created_at desc)
    where status = 'pending_confirm';

create unique index if not exists idx_test_data_tables_project_key_active
    on test_data_tables(project_id, key) where deleted_at is null;

create index if not exists idx_test_data_tables_project_active
    on test_data_tables(project_id, created_at desc) where deleted_at is null;

create index if not exists idx_test_data_rows_table_index
    on test_data_rows(table_id, row_index);

create index if not exists idx_api_keys_active
    on api_keys(token_hash)
    where deleted_at is null and enabled = true;

create index if not exists idx_api_keys_created_by
    on api_keys(created_by, created_at desc)
    where deleted_at is null;

-- +goose Down
drop table if exists script_library_templates;
drop table if exists api_keys;
drop table if exists test_data_rows;
drop table if exists test_data_tables;
drop table if exists ai_messages;
drop table if exists ai_sessions;
drop table if exists project_ai_prompts;
drop table if exists ai_providers;
drop table if exists mock_value_sets;
drop table if exists mock_routes;
drop table if exists mock_servers;
drop table if exists project_members;
drop table if exists role_permissions;
drop table if exists user_roles;
drop table if exists permissions;
drop table if exists roles;
drop table if exists users;
drop table if exists test_run_results;
drop table if exists test_runs;
drop table if exists test_suite_items;
drop table if exists test_suites;
drop table if exists test_scenario_steps;
drop table if exists test_scenarios;
drop table if exists sql_parameter_sources;
drop table if exists business_data_sources;
drop table if exists test_case_steps;
drop table if exists test_cases;
drop table if exists api_endpoints;
drop table if exists api_specs;
drop table if exists service_environment_configs;
drop table if exists environments;
drop table if exists services;
drop table if exists projects;
