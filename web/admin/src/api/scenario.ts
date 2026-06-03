/**
 * 场景编排相关 API
 */
import request, { asList } from './request'
import type {
  Scenario,
  ScenarioStep,
  ScenarioListFilter,
  CreateScenarioInput,
  UpdateScenarioInput,
  UpsertStepInput,
  ReorderStepsInput,
} from '@/types/scenario'

// 场景 CRUD
export const listScenarios = (params: ScenarioListFilter): Promise<Scenario[]> =>
  request.get('/scenarios', { params }).then(asList<Scenario>())

export const getScenario = (scenarioId: string): Promise<Scenario> =>
  request.get(`/scenarios/${scenarioId}`)

export const createScenario = (data: CreateScenarioInput): Promise<Scenario> =>
  request.post('/scenarios', data)

export const updateScenario = (id: string, data: UpdateScenarioInput): Promise<Scenario> =>
  request.put(`/scenarios/${id}`, data)

export const deleteScenario = (id: string) =>
  request.delete(`/scenarios/${id}`)

// 步骤管理
export const listSteps = (scenarioId: string): Promise<ScenarioStep[]> =>
  request.get(`/scenarios/${scenarioId}/steps`).then(asList<ScenarioStep>())

export const listScenarioSteps = (scenarioId: string): Promise<ScenarioStep[]> =>
  request.get(`/scenarios/${scenarioId}/steps`).then(asList<ScenarioStep>())

export const upsertStep = (scenarioId: string, input: UpsertStepInput): Promise<ScenarioStep> =>
  request.put(`/scenarios/${scenarioId}/steps/${input.stepOrder}`, input)

export const upsertScenarioStep = (scenarioId: string, stepOrder: number, data: Record<string, unknown>): Promise<ScenarioStep> =>
  request.put(`/scenarios/${scenarioId}/steps/${stepOrder}`, data)

export const deleteStep = (scenarioId: string, stepId: string) =>
  request.delete(`/scenarios/${scenarioId}/steps/${stepId}`)

export const deleteScenarioStep = (scenarioId: string, stepId: string) =>
  request.delete(`/scenarios/${scenarioId}/steps/${stepId}`)

export const reorderSteps = (scenarioId: string, data: ReorderStepsInput) =>
  request.put(`/scenarios/${scenarioId}/steps/reorder`, data)

export const reorderScenarioSteps = (scenarioId: string, data: ReorderStepsInput) =>
  request.put(`/scenarios/${scenarioId}/steps/reorder`, data)

// 运行
export const runScenario = (scenarioId: string, data: Record<string, unknown>) =>
  request.post(`/scenarios/${scenarioId}/run`, data)

// 运行记录（报告相关 API 在 report.ts 中）
