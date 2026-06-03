/**
 * 报告和运行记录相关 API
 */
import request from './request'
import type { ScenarioRunReport, ScenarioRunStats } from '@/types/scenario'

export const listProjectRuns = (projectId: string, params: Record<string, unknown> = {}) =>
  request.get(`/projects/${projectId}/runs`, { params })

export const listScenarioRuns = (scenarioId: string, params: Record<string, unknown> = {}) =>
  request.get(`/scenarios/${scenarioId}/runs`, { params })

export const getScenarioRunStats = (scenarioId: string, params: Record<string, unknown> = {}): Promise<ScenarioRunStats> =>
  request.get(`/scenarios/${scenarioId}/runs/stats`, { params })

export const getScenarioRunReport = (runId: string): Promise<ScenarioRunReport> =>
  request.get(`/runs/${runId}/report`)

export const exportScenarioRun = (runId: string, format: 'json' | 'html' = 'json') =>
  request.get(`/runs/${runId}/export`, { params: { format }, responseType: 'blob' })

export const getRunResult = (runId: string) =>
  request.get(`/runs/${runId}/result`)
