-- +goose Up
-- 扩展 project_ai_prompts.action 的 CHECK 约束，加入 AI 智能分析新增的两个 action：
--   - analyze_failure：AI 分析失败原因（运行控制台 / 场景运行结果）
--   - analyze_spec_changes：AI 分析 OpenAPI/Swagger 变更影响
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

-- +goose Down
alter table project_ai_prompts
    drop constraint if exists project_ai_prompts_action_check;

alter table project_ai_prompts
    add constraint project_ai_prompts_action_check
    check (action in (
        'generate_params',
        'generate_assertion',
        'generate_case_data',
        'raw'
    ));
