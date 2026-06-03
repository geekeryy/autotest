/**
 * AI 相关 API
 * 包括：提供商、会话、记忆、技能、Prompt Layer、速率限制
 */
import request, { asList } from './request'
import type { AIProviderConfig, AISession } from '@/types/ai'

// ── AI 提供商 ──────────────────────────────────────────────────
export const listAIProviderTypes = () =>
  request.get('/ai-provider-types').then(asList())

export const listAIProviders = (): Promise<AIProviderConfig[]> =>
  request.get('/ai-providers').then(asList<AIProviderConfig>())

export const createAIProvider = (data: Record<string, unknown>): Promise<AIProviderConfig> =>
  request.post('/ai-providers', data)

export const updateAIProvider = (providerId: string, data: Record<string, unknown>): Promise<AIProviderConfig> =>
  request.put(`/ai-providers/${providerId}`, data)

export const deleteAIProvider = (providerId: string) =>
  request.delete(`/ai-providers/${providerId}`)

export const testAIProvider = (providerId: string) =>
  request.post(`/ai-providers/${providerId}/test`, undefined, { timeout: 45000 })

export const listAIProviderModels = (providerId: string): Promise<string[]> =>
  request.get(`/ai-providers/${providerId}/models`, { timeout: 30000 })

export const discoverAIProviderModels = (data: Record<string, unknown>) =>
  request.post('/ai-providers/models/discover', data, { timeout: 30000 })

// ── AI 助理设置 ──────────────────────────────────────────────────
export const getAIAssistantSettings = () =>
  request.get('/ai-assistant-settings')

export const updateAIAssistantSettings = (data: Record<string, unknown>) =>
  request.put('/ai-assistant-settings', data)

export const getScenarioLoginHints = (projectId: string, serviceId: string, environmentId?: string) =>
  request.get(`/projects/${projectId}/services/${serviceId}/scenario-login-hints`, {
    params: environmentId ? { environmentId } : undefined,
  })

export const aiChat = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/ai/chat`, data, { timeout: 120000 })

// ── AI 分析 ──────────────────────────────────────────────────────
export const analyzeRunFailure = (runId: string) =>
  request.post(`/runs/${runId}/ai/analyze-failure`, {}, { timeout: 120000 })

export const analyzeSpecChanges = (projectId: string, serviceId: string, specId: string) =>
  request.post(
    `/projects/${projectId}/services/${serviceId}/specs/${specId}/ai/analyze-changes`,
    {},
    { timeout: 120000 }
  )

// ── AI 会话 ──────────────────────────────────────────────────────
export const listAISessions = (projectId: string, params?: { limit?: number; offset?: number }): Promise<AISession[]> =>
  request.get(`/projects/${projectId}/ai/sessions`, { params })

export const createAISession = (projectId: string, data: Record<string, unknown> = {}): Promise<AISession> =>
  request.post(`/projects/${projectId}/ai/sessions`, data)

export const getAISession = (projectId: string, sessionId: string) =>
  request.get(`/projects/${projectId}/ai/sessions/${sessionId}`)

// 兼容 ai store 中直接调用 sessionId 的场景
export const getAISessionById = (sessionId: string) =>
  request.get(`/ai/sessions/${sessionId}`)

export const getAISessionStats = (projectId: string, sessionId: string) =>
  request.get(`/projects/${projectId}/ai/sessions/${sessionId}/stats`)

export const getAIProjectTokenUsage = (projectId: string) =>
  request.get(`/projects/${projectId}/ai/sessions/token-usage`)

export const getAIMyTokenUsage = () =>
  request.get('/me/ai/token-usage')

export const renameAISession = (projectId: string, sessionId: string, data: Record<string, unknown>) =>
  request.patch(`/projects/${projectId}/ai/sessions/${sessionId}`, data)

export const deleteAISession = (projectId: string, sessionId: string) =>
  request.delete(`/projects/${projectId}/ai/sessions/${sessionId}`)

// ── 项目级 AI Prompts ──────────────────────────────────────────
export const listProjectAIPrompts = () =>
  request.get('/ai-prompts').then(asList())

export const createProjectAIPrompt = (data: Record<string, unknown>) =>
  request.post('/ai-prompts', data)

export const updateProjectAIPrompt = (promptId: string, data: Record<string, unknown>) =>
  request.put(`/ai-prompts/${promptId}`, data)

export const deleteProjectAIPrompt = (promptId: string) =>
  request.delete(`/ai-prompts/${promptId}`)

// ── 脚本库 ──────────────────────────────────────────────────────
export const listScriptLibraryTemplates = (projectId: string) =>
  request.get('/script-library-templates', { params: { projectId } }).then(asList())

export const createScriptLibraryTemplate = (data: Record<string, unknown>) =>
  request.post('/script-library-templates', data)

export const updateScriptLibraryTemplate = (id: string, data: Record<string, unknown>) =>
  request.put(`/script-library-templates/${id}`, data)

export const deleteScriptLibraryTemplate = (id: string) =>
  request.delete(`/script-library-templates/${id}`)

// ── AI Memory ──────────────────────────────────────────────────
export const listAIMemories = (params: Record<string, unknown> = {}) =>
  request.get('/ai/memories', { params })

export const saveAIMemory = (data: Record<string, unknown>) =>
  request.post('/ai/memories', data)

export const recallAIMemory = (data: Record<string, unknown>) =>
  request.post('/ai/memories/recall', data)

export const getAIMemory = (id: string) =>
  request.get(`/ai/memories/${id}`)

export const updateAIMemory = (id: string, data: Record<string, unknown>) =>
  request.put(`/ai/memories/${id}`, data)

export const deleteAIMemory = (id: string) =>
  request.delete(`/ai/memories/${id}`)

export const listProjectAIMemories = (projectId: string, params: Record<string, unknown> = {}) =>
  request.get(`/projects/${projectId}/ai/memories`, { params })

export const saveProjectAIMemory = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/ai/memories`, data)

export const recallProjectAIMemory = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/ai/memories/recall`, data)

// ── AI Skills ──────────────────────────────────────────────────
export const listAISkills = (params: Record<string, unknown> = {}) =>
  request.get('/ai/skills', { params }).then(asList())

export const createAISkill = (data: Record<string, unknown>) =>
  request.post('/ai/skills', data)

export const matchAISkills = (data: Record<string, unknown>) =>
  request.post('/ai/skills/match', data).then(asList())

export const getAISkill = (id: string) =>
  request.get(`/ai/skills/${id}`)

export const updateAISkill = (id: string, data: Record<string, unknown>) =>
  request.put(`/ai/skills/${id}`, data)

export const deleteAISkill = (id: string) =>
  request.delete(`/ai/skills/${id}`)

// ── Prompt Layers ──────────────────────────────────────────────
export const listPromptLayers = (params: Record<string, unknown> = {}) =>
  request.get('/ai/prompt-layers', { params }).then(asList())

export const createPromptLayer = (data: Record<string, unknown>) =>
  request.post('/ai/prompt-layers', data)

export const getPromptLayer = (id: string) =>
  request.get(`/ai/prompt-layers/${id}`)

export const enablePromptLayerVersion = (id: string) =>
  request.put(`/ai/prompt-layers/${id}/enable`)

export const deletePromptLayer = (id: string) =>
  request.delete(`/ai/prompt-layers/${id}`)

export const getDomainPrompt = (domain: string) =>
  request.get(`/ai/prompt-layers/domain/${domain}`)

export const getLatestPromptLayer = (name: string) =>
  request.get(`/ai/prompt-layers/by-name/${name}/latest`)

export const updatePromptLayerByName = (name: string, data: Record<string, unknown>) =>
  request.put(`/ai/prompt-layers/by-name/${name}`, data)

// ── Rate Limits ──────────────────────────────────────────────
export const listRateLimitRules = () =>
  request.get('/ai/rate-limits/rules').then(asList())

export const createRateLimitRule = (data: Record<string, unknown>) =>
  request.post('/ai/rate-limits/rules', data)

export const cleanupRateLimitEntries = () =>
  request.post('/ai/rate-limits/cleanup')
