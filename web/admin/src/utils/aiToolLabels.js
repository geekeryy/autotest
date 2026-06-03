/** 内置 AI 工具英文名 → 界面中文名（与 internal/aitools/builtin 注册名一致） */
export const AI_TOOL_DISPLAY_NAMES = {
  list_services: '列出服务',
  list_endpoints: '列出接口',
  get_endpoint: '查询接口定义',
  list_environments: '列出环境',
  list_cases: '列出用例',
  get_case: '查询用例',
  create_case_from_endpoint: '创建用例模板',
  update_case_assertions: '更新用例断言',
  list_scenarios: '列出场景',
  get_scenario: '查询场景',
  create_scenario_with_steps: '创建场景及步骤',
  add_scenario_step: '添加场景步骤',
  update_scenario_step: '更新场景步骤',
  delete_scenario: '删除场景',
  delete_scenario_step: '删除场景步骤',
  delete_scenarios: '批量删除场景',
  reorder_scenario_steps: '重排场景步骤',
  generate_and_verify_scenarios: '生成并验证场景',
  generate_coverage_scenarios: '生成覆盖场景',
  list_scenario_login_hints: '发现场景登录凭据',
  ask_question: '向你提问',
}

/**
 * @param {string} [name]
 * @returns {string}
 */
export function getToolDisplayName(name) {
  const key = String(name || '').trim()
  if (!key) return '工具'
  return AI_TOOL_DISPLAY_NAMES[key] || key
}
