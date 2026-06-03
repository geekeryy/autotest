/**
 * 场景编排相关类型定义
 * 与后端 internal/scenario/model.go 对齐
 */

import type { UUID } from './common'

/** 步骤类型 */
export type StepType = 'api' | 'database' | 'script' | 'for' | 'condition'

/** 场景 */
export interface Scenario {
  id: UUID
  projectId: UUID
  serviceId: UUID
  name: string
  description?: string
  createdAt: string
  updatedAt: string
}

/** 场景步骤 */
export interface ScenarioStep {
  id: UUID
  scenarioId: UUID
  testCaseId?: UUID
  stepSeq: number
  stepOrder: number
  stepType: StepType
  name?: string
  enabled: boolean
  config?: Record<string, unknown>
  requestOverride?: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

/** 创建场景输入 */
export interface CreateScenarioInput {
  projectId: UUID
  serviceId: UUID
  name: string
  description?: string
}

/** 更新场景输入 */
export interface UpdateScenarioInput {
  name?: string
  description?: string
}

/** Upsert 步骤输入 */
export interface UpsertStepInput {
  testCaseId?: UUID
  stepOrder: number
  stepType: StepType
  name: string
  enabled?: boolean
  config: Record<string, unknown>
  requestOverride?: Record<string, unknown>
  attachUnderParent?: AttachUnderParentInput
}

/** 控制流步骤挂载 */
export interface AttachUnderParentInput {
  parentStepOrder: number
  kind: 'forLoopBody' | 'conditionElse' | 'conditionBranchIndex'
  branchIndex?: number
}

/** 重排序步骤输入 */
export interface ReorderStepsInput {
  stepIds: UUID[]
}

/** 场景列表筛选 */
export interface ScenarioListFilter {
  projectId: UUID
  serviceId: UUID
}

// ── 步骤配置类型 ──────────────────────────────────────────────────

/** For 循环步骤配置 */
export interface ForStepConfig {
  mode: 'count' | 'array'
  count?: number
  countExpression?: string
  itemsExpression?: string
  itemName?: string
  indexName?: string
  bodyStepSeqs?: number[]
  maxIterations?: number
}

/** 条件表达式 */
export interface ConditionExpression {
  source: string
  operator: ComparisonOperator
  value?: unknown
}

/** 条件分支 */
export interface ConditionBranch {
  expression: ConditionExpression
  stepSeqs: number[]
}

/** Condition 条件步骤配置 */
export interface ConditionStepConfig {
  branches: ConditionBranch[]
  elseStepSeqs?: number[]
  thenStepSeqs?: number[]
}

/** 比较运算符 */
export type ComparisonOperator =
  | 'equals' | 'not_equals' | 'contains' | 'not_contains'
  | 'gt' | 'gte' | 'lt' | 'lte'
  | 'exists' | 'not_exists' | 'truthy' | 'falsy'
  | 'is_empty' | 'is_not_empty'

/** API 步骤配置 */
export interface ApiStepConfig {
  testCaseId?: UUID
  assertions?: Assertion[]
}

/** 脚本步骤配置 */
export interface ScriptStepConfig {
  script: string
  timeout?: number
}

/** 数据库步骤配置 */
export interface DatabaseStepConfig {
  dataSourceId: UUID
  sql: string
  params?: Record<string, unknown>
}

/** 变量提取规则 */
export interface StepExtract {
  variableName: string
  source: 'response.body' | 'response.header' | 'response.status' | 'stdout' | 'stdoutJson'
  expression: string
}

/** 断言 */
export interface Assertion {
  type: AssertionType
  expression?: string
  operator?: ComparisonOperator
  expected?: unknown
  script?: string
}

/** 断言类型 */
export type AssertionType =
  | 'status_code' | 'jsonpath' | 'header' | 'body_contains'
  | 'response_time' | 'script' | 'schema'

// ── 运行相关类型 ──────────────────────────────────────────────────

/** 运行状态 */
export type RunStatus = 'queued' | 'running' | 'passed' | 'failed'

/** 结果状态 */
export type ResultStatus = 'passed' | 'failed' | 'error'

/** 运行记录 */
export interface Run {
  id: UUID
  name?: string
  projectId: UUID
  serviceId: UUID
  suiteId?: UUID
  scenarioId?: UUID
  environmentId?: UUID
  status: RunStatus
  variables?: Record<string, unknown>
  snapshot?: Record<string, unknown>
  startedAt?: string
  finishedAt?: string
  createdAt: string
}

/** 步骤执行结果 */
export interface RunResult {
  id: UUID
  runId: UUID
  testCaseId: UUID
  stepId?: UUID
  status: ResultStatus
  durationMillis: number
  requestSnapshot?: Record<string, unknown>
  responseSnapshot?: Record<string, unknown>
  assertions?: AssertionResult[]
  error?: string
  createdAt: string
}

/** 断言结果 */
export interface AssertionResult {
  type: string
  expression?: string
  operator?: string
  expected?: unknown
  actual?: unknown
  passed: boolean
  message?: string
}

/** 场景运行报告 */
export interface ScenarioRunReport {
  run: Run
  summary: RunSummary
  stepResults: StepRunResult[]
}

/** 运行汇总 */
export interface RunSummary {
  totalSteps: number
  passed: number
  failed: number
  errors: number
  durationMillis: number
}

/** 步骤运行结果 */
export interface StepRunResult {
  stepOrder: number
  stepSeq: number
  stepType: StepType
  stepName?: string
  status: ResultStatus
  durationMillis: number
  requestSnapshot?: Record<string, unknown>
  responseSnapshot?: Record<string, unknown>
  assertions?: AssertionResult[]
  extracts?: Record<string, unknown>
  error?: string
}

/** 运行列表条目 */
export interface RunListEntry {
  id: UUID
  name?: string
  scenarioId?: UUID
  scenarioName?: string
  environmentId?: UUID
  environmentName?: string
  status: RunStatus
  totalSteps: number
  passedSteps: number
  failedSteps: number
  durationMillis: number
  startedAt?: string
  finishedAt?: string
  createdAt: string
}

/** 每日运行趋势 */
export interface DailyRunTrend {
  date: string
  total: number
  passed: number
  failed: number
}

/** 失败步骤统计 */
export interface FailedStepStat {
  stepName: string
  stepType: StepType
  failCount: number
  lastFailAt: string
}

/** 运行统计 */
export interface ScenarioRunStats {
  dailyTrend: DailyRunTrend[]
  failedSteps: FailedStepStat[]
}
