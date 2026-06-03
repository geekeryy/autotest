/**
 * AI 相关类型定义
 * 与后端 internal/aiprovider/ 和 internal/aitools/ 对齐
 */

import type { UUID } from './common'

// ── 会话与消息 ──────────────────────────────────────────────────

/** AI 会话 */
export interface AISession {
  id: UUID
  projectId: UUID
  title: string
  provider: string
  model: string
  createdAt: string
  updatedAt: string
}

/** 消息角色 */
export type MessageRole = 'user' | 'assistant' | 'system' | 'tool'

/** AI 消息 */
export interface AIMessage {
  id: UUID
  sessionId: UUID
  role: MessageRole
  content: string
  toolCalls?: ToolCall[]
  toolResult?: ToolResult
  thinking?: string
  tokenUsage?: TokenUsage
  createdAt: string
}

/** 工具调用 */
export interface ToolCall {
  id: string
  name: string
  arguments: Record<string, unknown>
  status: ToolCallStatus
}

/** 工具调用状态 */
export type ToolCallStatus = 'pending' | 'running' | 'completed' | 'failed' | 'confirmed' | 'rejected'

/** 工具执行结果 */
export interface ToolResult {
  toolCallId: string
  output: string
  isError: boolean
}

/** Token 用量 */
export interface TokenUsage {
  inputTokens: number
  outputTokens: number
  cacheReadTokens?: number
  cacheWriteTokens?: number
}

// ── SSE 流式事件 ──────────────────────────────────────────────────

/** SSE 事件类型 */
export type SSEEventType =
  | 'text_delta'
  | 'thinking_delta'
  | 'tool_call'
  | 'tool_result'
  | 'pending_confirm'
  | 'confirmed'
  | 'rejected'
  | 'done'
  | 'error'
  | 'heartbeat'

/** SSE 事件 */
export interface SSEEvent {
  type: SSEEventType
  data: unknown
}

/** 文本增量事件 */
export interface TextDeltaEvent {
  type: 'text_delta'
  delta: string
}

/** 思考增量事件 */
export interface ThinkingDeltaEvent {
  type: 'thinking_delta'
  delta: string
}

/** 工具调用事件 */
export interface ToolCallEvent {
  type: 'tool_call'
  toolCallId: string
  name: string
  arguments: Record<string, unknown>
}

/** 工具结果事件 */
export interface ToolResultEvent {
  type: 'tool_result'
  toolCallId: string
  output: string
  isError: boolean
}

/** 待确认事件 */
export interface PendingConfirmEvent {
  type: 'pending_confirm'
  toolCallId: string
  name: string
  arguments: Record<string, unknown>
  description: string
}

/** 完成事件 */
export interface DoneEvent {
  type: 'done'
  tokenUsage?: TokenUsage
}

/** 错误事件 */
export interface ErrorEvent {
  type: 'error'
  error: string
}

// ── AI 助理窗格 ──────────────────────────────────────────────────

/** AI 助理窗格状态 */
export interface AIPane {
  id: string
  sessionId: UUID | null
  messages: AIMessage[]
  isStreaming: boolean
  provider: string
  model: string
  thinkingEnabled: boolean
  webSearchEnabled: boolean
  pendingConfirm: PendingConfirmEvent | null
  inputValue: string
}

/** 分屏模式 */
export type SplitMode = 1 | 2 | 3 | 6

// ── AI 提供商 ──────────────────────────────────────────────────

/** AI 提供商配置 */
export interface AIProviderConfig {
  id: UUID
  name: string
  provider: AIProviderType
  model: string
  apiKey?: string
  baseURL?: string
  isDefault: boolean
  enabled: boolean
  config: Record<string, unknown>
}

/** AI 提供商类型 */
export type AIProviderType =
  | 'deepseek' | 'openai' | 'anthropic' | 'ollama'
  | 'kimi' | 'xiaomi' | 'custom'

// ── AI 场景生成 ──────────────────────────────────────────────────

/** AI 场景生成配置 */
export interface AIScenarioGenConfig {
  serviceId: string
  coverage: CoverageLevel
  includeAuth: boolean
  maxScenarios?: number
}

/** 覆盖级别 */
export type CoverageLevel = 'happy_path' | 'boundary' | 'error' | 'full'

/** AI 场景生成进度 */
export interface AIScenarioGenProgress {
  phase: 'planning' | 'generating' | 'verifying' | 'fixing' | 'done'
  totalScenarios: number
  completedScenarios: number
  currentScenario?: string
  errors: string[]
}

/** AI 场景生成结果 */
export interface AIScenarioGenResult {
  scenarios: GeneratedScenario[]
  summary: {
    totalScenarios: number
    totalSteps: number
    coverageRate: number
  }
}

/** AI 生成的场景 */
export interface GeneratedScenario {
  name: string
  description: string
  steps: GeneratedStep[]
  preview: string
}

/** AI 生成的步骤 */
export interface GeneratedStep {
  stepType: string
  name: string
  config: Record<string, unknown>
  requestOverride?: Record<string, unknown>
}

// ── 自然语言编排 ──────────────────────────────────────────────────

/** 自然语言编排请求 */
export interface NLOrchestratorInput {
  description: string
  serviceId: string
  environmentId?: string
}

/** 自然语言编排结果 */
export interface NLOrchestratorResult {
  scenarioId: string
  scenarioName: string
  steps: GeneratedStep[]
  preview: string
}

// ── 测试数据工厂 ──────────────────────────────────────────────────

/** 测试数据生成请求 */
export interface GenerateTestDataInput {
  endpointId: string
  scenario: 'normal' | 'boundary' | 'stress'
  count: number
  locale?: 'zh_CN' | 'en_US'
  constraints?: Record<string, unknown>
}

/** 测试数据生成结果 */
export interface GenerateTestDataOutput {
  rows: Record<string, unknown>[]
  metadata: {
    totalCount: number
    generatedAt: string
    locale: string
  }
}

// ── 覆盖生成 ──────────────────────────────────────────────────

/** 覆盖生成请求 */
export interface GenerateCoverageInput {
  serviceId: string
  dimensions: CoverageDimension[]
}

/** 覆盖维度 */
export type CoverageDimension =
  | 'happy_path' | 'required_missing' | 'type_mismatch'
  | 'edge_cases' | 'business_error'

/** 覆盖生成结果 */
export interface GenerateCoverageOutput {
  cases: GeneratedCase[]
  summary: Record<CoverageDimension, number>
}

/** AI 生成的用例 */
export interface GeneratedCase {
  endpointId: string
  endpointPath: string
  method: string
  dimension: CoverageDimension
  name: string
  request: Record<string, unknown>
  assertions: Record<string, unknown>[]
  description: string
}
