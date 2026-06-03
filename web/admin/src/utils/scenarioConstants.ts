/**
 * 场景编排常量定义
 * 从 ScenarioEditor.vue 中提取，供多个组件共享
 */

export interface StepTypeOption {
  value: string
  label: string
  icon?: string
}

export const STEP_TYPES: StepTypeOption[] = [
  { value: 'api', label: 'API 请求' },
  { value: 'database', label: '数据库查询' },
  { value: 'script', label: '脚本' },
  { value: 'for', label: 'For 循环' },
  { value: 'condition', label: '条件分支' },
]

export interface ConditionOperatorOption {
  value: string
  label: string
}

export const CONDITION_OPERATORS: ConditionOperatorOption[] = [
  { value: 'equals', label: '等于' },
  { value: 'not_equals', label: '不等于' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'gt', label: '大于' },
  { value: 'gte', label: '大于等于' },
  { value: 'lt', label: '小于' },
  { value: 'lte', label: '小于等于' },
  { value: 'exists', label: '存在' },
  { value: 'not_exists', label: '不存在' },
  { value: 'truthy', label: '为真' },
  { value: 'falsy', label: '为假' },
  { value: 'is_empty', label: '为空' },
  { value: 'is_not_empty', label: '不为空' },
]

export interface HttpMethodOption {
  value: string
  label: string
  color?: string
}

export const HTTP_METHODS: HttpMethodOption[] = [
  { value: 'GET', label: 'GET', color: '#61affe' },
  { value: 'POST', label: 'POST', color: '#49cc90' },
  { value: 'PUT', label: 'PUT', color: '#fca130' },
  { value: 'PATCH', label: 'PATCH', color: '#50e3c2' },
  { value: 'DELETE', label: 'DELETE', color: '#f93e3e' },
  { value: 'HEAD', label: 'HEAD', color: '#9012fe' },
  { value: 'OPTIONS', label: 'OPTIONS', color: '#0d5aa7' },
]

/** 运行结果面板默认高度 */
export const RUN_RESULT_PANEL_DEFAULT_HEIGHT = 320

/** 步骤侧边栏默认宽度 */
export const STEP_SIDEBAR_DEFAULT = 260

/** 步骤侧边栏最小宽度 */
export const STEP_SIDEBAR_MIN = 180

/** 步骤侧边栏最大宽度 */
export const STEP_SIDEBAR_MAX = 560
