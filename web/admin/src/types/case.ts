/**
 * 用例管理相关类型定义
 * 与后端 internal/case/model.go 对齐
 */

import type { UUID, HttpMethod } from './common'

/** 用例来源 */
export type CaseSource = 'auto' | 'manual' | 'derived'

/** 用例状态 */
export type CaseStatus = 'active' | 'draft' | 'archived'

/** 测试用例 */
export interface TestCase {
  id: UUID
  projectId: UUID
  serviceId: UUID
  endpointId?: UUID
  parentCaseId?: UUID
  source: CaseSource
  name: string
  method: HttpMethod
  path: string
  fingerprint?: string
  generationRuleId?: string
  request: RequestDefinition
  assertions: Assertion[]
  lastResponseSnapshot?: Record<string, unknown>
  status: CaseStatus
  createdAt: string
  updatedAt: string
}

/** 请求定义 */
export interface RequestDefinition {
  method: HttpMethod
  path: string
  headers?: Record<string, string>
  query?: Record<string, string>
  variables?: Record<string, string>
  body?: unknown
  security?: Record<string, unknown>
}

/** 断言定义 (用例级别) */
export interface Assertion {
  type: AssertionType
  expression?: string
  operator?: string
  expected?: unknown
  script?: string
}

/** 断言类型 */
export type AssertionType =
  | 'status_code' | 'jsonpath' | 'header' | 'body_contains'
  | 'response_time' | 'script' | 'schema'

/** 创建手动用例输入 */
export interface CreateManualInput {
  projectId: UUID
  serviceId: UUID
  endpointId?: UUID
  name: string
  method: HttpMethod
  path: string
  request: RequestDefinition
  assertions?: Assertion[]
}

/** 创建派生用例输入 */
export interface CreateSavedInput {
  name: string
  method: HttpMethod
  path: string
  request: RequestDefinition
  assertions?: Assertion[]
}

/** 更新用例输入 (PATCH) */
export interface PatchCaseInput {
  name?: string
  assertions?: Assertion[]
}

/** 用例列表筛选 */
export interface CaseListFilter {
  projectId: UUID
  serviceId: UUID
}

/** 生成的参数 */
export interface GeneratedParams {
  query: Record<string, string>
  path: Record<string, string>
  body: unknown
  dependencies?: FieldDependencyInfo[]
}

/** 字段依赖信息 */
export interface FieldDependencyInfo {
  fieldPath: string
  dependsOn: string
  suggestion: string
}

/** 推断的断言 (AI 智能断言推断) */
export interface InferredAssertion {
  type: AssertionType
  expression?: string
  operator?: string
  expected?: unknown
  confidence: number
  source: 'schema' | 'history' | 'semantic'
  reason: string
}

/** 断言推断请求 */
export interface InferAssertionsInput {
  sampleCount?: number
  sources?: ('schema' | 'history' | 'semantic')[]
}

/** 断言推断响应 */
export interface InferAssertionsOutput {
  assertions: InferredAssertion[]
  summary: {
    totalAssertions: number
    bySource: Record<string, number>
    historySamples: number
    hasSchema: boolean
  }
}

/** 批量应用断言请求 */
export interface ApplyAssertionsInput {
  assertions: InferredAssertion[]
  mode: 'replace' | 'append'
}
