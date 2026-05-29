import request from './request'

/** 列表接口若返回 JSON null，规范为空数组，避免表格等对 null 异常 */
function asList(data) {
  return Array.isArray(data) ? data : []
}

export const login = (data) => request.post('/auth/login', data)
export const logout = () => request.post('/auth/logout')
export const changePassword = (data) => request.post('/auth/change-password', data)
export const getMe = () => request.get('/auth/me')
export const getGitHubOAuthStatus = () => request.get('/auth/github/status')

/** 登录页可用 OAuth 方式（公开，无需 JWT） */
export const listLoginProviders = () => request.get('/auth/login-providers').then(asList)
export const listAuthProviderTypes = () => request.get('/auth-provider-types').then(asList)
export const listAuthProviders = () => request.get('/auth-providers').then(asList)
export const createAuthProvider = (data) => request.post('/auth-providers', data)
export const updateAuthProvider = (providerId, data) =>
  request.put(`/auth-providers/${providerId}`, data)
export const deleteAuthProvider = (providerId) =>
  request.delete(`/auth-providers/${providerId}`)
export const testAuthProvider = (providerId) =>
  request.post(`/auth-providers/${providerId}/test`)

export const listProjects = () => request.get('/projects').then(asList)
export const createProject = (data) => request.post('/projects', data)
export const deleteProject = (id) => request.delete(`/projects/${id}`)
export const listProjectMembers = (projectId) => request.get(`/projects/${projectId}/members`).then(asList)
export const addProjectMember = (projectId, data) => request.post(`/projects/${projectId}/members`, data)
export const updateProjectMember = (projectId, userId, data) => request.put(`/projects/${projectId}/members/${userId}`, data)
export const removeProjectMember = (projectId, userId) => request.delete(`/projects/${projectId}/members/${userId}`)
export const listServices = (projectId) => request.get(`/projects/${projectId}/services`).then(asList)
export const createService = (projectId, data) => request.post(`/projects/${projectId}/services`, data)
export const updateService = (projectId, serviceId, data) => request.put(`/projects/${projectId}/services/${serviceId}`, data)
export const deleteService = (projectId, serviceId) => request.delete(`/projects/${projectId}/services/${serviceId}`)
export const listServiceEnvironments = (projectId, serviceId) =>
  request.get(`/projects/${projectId}/services/${serviceId}/environments`).then(asList)
export const createServiceEnvironment = (projectId, serviceId, data) => request.post(`/projects/${projectId}/services/${serviceId}/environments`, data)
export const updateServiceEnvironment = (projectId, serviceId, environmentId, data) => request.put(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`, data)
export const deleteServiceEnvironment = (projectId, serviceId, environmentId) => request.delete(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`)
export const listEnvironments = (projectId) => request.get(`/projects/${projectId}/environments`).then(asList)
export const createEnvironment = (projectId, data) => request.post(`/projects/${projectId}/environments`, data)
export const updateEnvironment = (projectId, environmentId, data) => request.put(`/projects/${projectId}/environments/${environmentId}`, data)
export const deleteEnvironment = (projectId, environmentId) => request.delete(`/projects/${projectId}/environments/${environmentId}`)

export const importSpec = (projectId, serviceId, content) =>
  request.post(`/projects/${projectId}/services/${serviceId}/specs/import`, content, {
    headers: { 'Content-Type': 'application/yaml' }
  })
export const listSpecs = (projectId, serviceId) => request.get(`/projects/${projectId}/services/${serviceId}/specs`)
export const listEndpoints = (projectId, serviceId) => request.get(`/projects/${projectId}/services/${serviceId}/endpoints`)

export const listCases = (params = {}) => request.get('/cases', { params }).then(asList)
export const getCase = (caseId) => request.get(`/cases/${caseId}`)
export const patchCase = (caseId, data) => request.patch(`/cases/${caseId}`, data)
export const createCase = (data) => request.post('/cases', data)
export const listSavedCases = (caseId) => request.get(`/cases/${caseId}/saved-cases`).then(asList)
export const createSavedCase = (caseId, data) => request.post(`/cases/${caseId}/saved-cases`, data)
export const deleteSavedCase = (caseId, savedCaseId) => request.delete(`/cases/${caseId}/saved-cases/${savedCaseId}`)
export const generateCaseParams = (caseId) => request.get(`/cases/${caseId}/generate-params`)
export const runCase = (caseId, data) => request.post(`/cases/${caseId}/run`, data)
export const getRunResult = (runId) => request.get(`/runs/${runId}/result`)

/**
 * @typedef {Object} DataSource
 * @property {string} id
 * @property {string} projectId
 * @property {string} serviceId
 * @property {string} environmentId
 * @property {string} name
 * @property {string} driver
 * @property {string=} dsn
 * @property {Object=} config
 */
export const listDataSources = () => request.get('/data-sources').then(asList)
export const createDataSource = (data) => request.post('/data-sources', data)
export const updateDataSource = (id, data) => request.put(`/data-sources/${id}`, data)
export const deleteDataSource = (id) => request.delete(`/data-sources/${id}`)
export const testDataSource = (id) => request.post(`/data-sources/${id}/test`)
export const getDataSourceSchema = (id) => request.get(`/data-sources/${id}/schema`).then(asList)

/**
 * @typedef {Object} SQLParameterSource
 * @property {string} id
 * @property {string} projectId
 * @property {string} serviceId
 * @property {string} dataSourceId
 * @property {string} key
 * @property {string} name
 * @property {string} sql
 * @property {Object=} inputParams
 */
export const listSQLParameterSources = (params = {}) => request.get('/sql-parameter-sources', { params }).then(asList)
export const createSQLParameterSource = (data) => request.post('/sql-parameter-sources', data)
export const updateSQLParameterSource = (id, data) => request.put(`/sql-parameter-sources/${id}`, data)
export const deleteSQLParameterSource = (id) => request.delete(`/sql-parameter-sources/${id}`)
export const previewSQLParameterSource = (id, data = {}) => request.post(`/sql-parameter-sources/${id}/preview`, data)
/** 新增/编辑弹窗内根据当前表单测试 SQL（无需已保存的记录） */
export const previewSQLParameterSourceDraft = (data) => request.post('/sql-parameter-sources/preview', data)

// 项目级命名值集合（mock value sets）— 与脚本库、AI 提供商同级，
// 服务于 `{{$mock.set.<key>}}` 模板族。viewer 可读，developer 可写，
// 后端通过 RequireProjectRole 控制。
export const listMockValueSets = (projectId) => request.get(`/projects/${projectId}/mock-value-sets`).then(asList)
export const createMockValueSet = (projectId, data) => request.post(`/projects/${projectId}/mock-value-sets`, data)
export const updateMockValueSet = (projectId, setId, data) => request.put(`/projects/${projectId}/mock-value-sets/${setId}`, data)
export const deleteMockValueSet = (projectId, setId) => request.delete(`/projects/${projectId}/mock-value-sets/${setId}`)

export const listMockServers = (projectId) => request.get(`/projects/${projectId}/mock-servers`).then(asList)
export const createMockServer = (projectId, data) => request.post(`/projects/${projectId}/mock-servers`, data)
export const updateMockServer = (projectId, serverId, data) => request.put(`/projects/${projectId}/mock-servers/${serverId}`, data)
export const deleteMockServer = (projectId, serverId) => request.delete(`/projects/${projectId}/mock-servers/${serverId}`)
export const startMockServer = (projectId, serverId) => request.post(`/projects/${projectId}/mock-servers/${serverId}/start`)
export const stopMockServer = (projectId, serverId) => request.post(`/projects/${projectId}/mock-servers/${serverId}/stop`)
export const listMockRoutes = (projectId, serverId) =>
  request.get(`/projects/${projectId}/mock-servers/${serverId}/routes`).then(asList)
export const createMockRoute = (projectId, serverId, data) => request.post(`/projects/${projectId}/mock-servers/${serverId}/routes`, data)
export const updateMockRoute = (projectId, serverId, routeId, data) =>
  request.put(`/projects/${projectId}/mock-servers/${serverId}/routes/${routeId}`, data)
export const deleteMockRoute = (projectId, serverId, routeId) =>
  request.delete(`/projects/${projectId}/mock-servers/${serverId}/routes/${routeId}`)
export const listMockAccessLogs = (projectId, serverId, params = {}) =>
  request.get(`/projects/${projectId}/mock-servers/${serverId}/access-logs`, { params })
export const clearMockAccessLogs = (projectId, serverId) =>
  request.delete(`/projects/${projectId}/mock-servers/${serverId}/access-logs`)

// Scenario orchestration
export const listScenarios = (params = {}) => request.get('/scenarios', { params }).then(asList)
export const createScenario = (data) => request.post('/scenarios', data)
export const updateScenario = (id, data) => request.put(`/scenarios/${id}`, data)
export const deleteScenario = (id) => request.delete(`/scenarios/${id}`)
export const listScenarioSteps = (scenarioId) => request.get(`/scenarios/${scenarioId}/steps`).then(asList)
export const reorderScenarioSteps = (scenarioId, data) =>
  request.put(`/scenarios/${scenarioId}/steps/reorder`, data)
export const upsertScenarioStep = (scenarioId, stepOrder, data) =>
  request.put(`/scenarios/${scenarioId}/steps/${stepOrder}`, data)
export const deleteScenarioStep = (scenarioId, stepId) => request.delete(`/scenarios/${scenarioId}/steps/${stepId}`)
export const runScenario = (scenarioId, data) => request.post(`/scenarios/${scenarioId}/run`, data)

export const getScenario = (scenarioId) => request.get(`/scenarios/${scenarioId}`)
export const listScenarioRuns = (scenarioId, params = {}) =>
  request.get(`/scenarios/${scenarioId}/runs`, { params })
export const listProjectRuns = (projectId, params = {}) =>
  request.get(`/projects/${projectId}/runs`, { params })
export const getScenarioRunStats = (scenarioId, params = {}) =>
  request.get(`/scenarios/${scenarioId}/runs/stats`, { params })
export const getScenarioRunReport = (runId) => request.get(`/runs/${runId}/report`)
export const exportScenarioRun = (runId, format = 'json') =>
  request.get(`/runs/${runId}/export`, { params: { format }, responseType: 'blob' })

// AI providers (platform-wide) and AI invocation
export const listAIProviderTypes = () => request.get('/ai-provider-types').then(asList)
export const listAIProviders = () => request.get('/ai-providers').then(asList)
export const createAIProvider = (data) => request.post('/ai-providers', data)
export const updateAIProvider = (providerId, data) =>
  request.put(`/ai-providers/${providerId}`, data)
export const deleteAIProvider = (providerId) =>
  request.delete(`/ai-providers/${providerId}`)
export const testAIProvider = (providerId) =>
  request.post(`/ai-providers/${providerId}/test`, undefined, { timeout: 45000 })
export const listAIProviderModels = (providerId) =>
  request.get(`/ai-providers/${providerId}/models`, { timeout: 30000 })
export const discoverAIProviderModels = (data) =>
  request.post('/ai-providers/models/discover', data, { timeout: 30000 })

export const getAIAssistantSettings = () => request.get('/ai-assistant-settings')
export const updateAIAssistantSettings = (data) => request.put('/ai-assistant-settings', data)
export const getScenarioLoginHints = (projectId, serviceId, environmentId) =>
  request.get(`/projects/${projectId}/services/${serviceId}/scenario-login-hints`, {
    params: environmentId ? { environmentId } : undefined,
  })
export const aiChat = (projectId, data) =>
  request.post(`/projects/${projectId}/ai/chat`, data, { timeout: 120000 })

// AI 智能分析：基于一次运行的请求/响应/断言失败结果调用 AI 分析失败原因
export const analyzeRunFailure = (runId) =>
  request.post(`/runs/${runId}/ai/analyze-failure`, {}, { timeout: 120000 })

// AI 智能分析：基于本次 spec 与同 service 上一版本的结构化 diff，结合受影响的接口模板/场景步骤，调用 AI 分析变更影响
export const analyzeSpecChanges = (projectId, serviceId, specId) =>
  request.post(
    `/projects/${projectId}/services/${serviceId}/specs/${specId}/ai/analyze-changes`,
    {},
    { timeout: 120000 }
  )

// AI 助理浮窗：会话与消息持久化按 (project, user) 隔离；
// 真正的对话流通过 `utils/aiStream.js` 的 fetch + SSE，确认写工具调用同样走 SSE。
export const listAISessions = (projectId) =>
  request.get(`/projects/${projectId}/ai/sessions`).then(asList)
export const createAISession = (projectId, data = {}) =>
  request.post(`/projects/${projectId}/ai/sessions`, data)
export const getAISession = (projectId, sessionId) =>
  request.get(`/projects/${projectId}/ai/sessions/${sessionId}`)
export const getAISessionStats = (projectId, sessionId) =>
  request.get(`/projects/${projectId}/ai/sessions/${sessionId}/stats`)
export const getAIProjectTokenUsage = (projectId) =>
  request.get(`/projects/${projectId}/ai/sessions/token-usage`)
export const getAIMyTokenUsage = () => request.get('/me/ai/token-usage')
export const renameAISession = (projectId, sessionId, data) =>
  request.patch(`/projects/${projectId}/ai/sessions/${sessionId}`, data)
export const deleteAISession = (projectId, sessionId) =>
  request.delete(`/projects/${projectId}/ai/sessions/${sessionId}`)

// Platform AI Prompts (global prompt config)
export const listProjectAIPrompts = () => request.get('/ai-prompts').then(asList)
export const createProjectAIPrompt = (data) => request.post('/ai-prompts', data)
export const updateProjectAIPrompt = (promptId, data) =>
  request.put(`/ai-prompts/${promptId}`, data)
export const deleteProjectAIPrompt = (promptId) =>
  request.delete(`/ai-prompts/${promptId}`)

/** @param {string} projectId */
export const listScriptLibraryTemplates = (projectId) =>
  request.get('/script-library-templates', { params: { projectId } }).then(asList)
export const createScriptLibraryTemplate = (data) => request.post('/script-library-templates', data)
export const updateScriptLibraryTemplate = (id, data) => request.put(`/script-library-templates/${id}`, data)
export const deleteScriptLibraryTemplate = (id) => request.delete(`/script-library-templates/${id}`)

export const listUsers = () => request.get('/users')
export const createUser = (data) => request.post('/users', data)
export const updateUser = (id, data) => request.put(`/users/${id}`, data)
export const deleteUser = (id) => request.delete(`/users/${id}`)

export const listRoles = () => request.get('/roles')
export const createRole = (data) => request.post('/roles', data)
export const updateRole = (id, data) => request.put(`/roles/${id}`, data)
export const deleteRole = (id) => request.delete(`/roles/${id}`)
export const setRolePermissions = (id, data) => request.put(`/roles/${id}/permissions`, data)

export const listPermissions = () => request.get('/permissions')
export const createPermission = (data) => request.post('/permissions', data)

// API Keys（需 apikeys:manage；列表与变更仅针对当前用户本人创建的 Key；明文 token 仅在创建/重置响应里返回一次）
export const listApiKeys = () => request.get('/api-keys').then(asList)
export const createApiKey = (data) => request.post('/api-keys', data)
export const updateApiKey = (id, data) => request.patch(`/api-keys/${id}`, data)
export const rotateApiKey = (id) => request.post(`/api-keys/${id}/rotate`)
export const deleteApiKey = (id) => request.delete(`/api-keys/${id}`)

export const listAuditLogs = (params = {}) => request.get('/audit-logs', { params })

export const listTestDataTables = (projectId) =>
  request.get(`/projects/${projectId}/test-data-tables`).then(asList)
export const getTestDataTable = (projectId, tableId) =>
  request.get(`/projects/${projectId}/test-data-tables/${tableId}`)
export const createTestDataTable = (projectId, data) =>
  request.post(`/projects/${projectId}/test-data-tables`, data)
export const updateTestDataTable = (projectId, tableId, data) =>
  request.put(`/projects/${projectId}/test-data-tables/${tableId}`, data)
export const deleteTestDataTable = (projectId, tableId) =>
  request.delete(`/projects/${projectId}/test-data-tables/${tableId}`)
export const listTestDataRows = (projectId, tableId) =>
  request.get(`/projects/${projectId}/test-data-tables/${tableId}/rows`).then(asList)
export const replaceTestDataRows = (projectId, tableId, rows) =>
  request.put(`/projects/${projectId}/test-data-tables/${tableId}/rows`, { rows })
/**
 * 批量生成 N 行测试数据。AI 列由项目 Prompt 管理（generate_case_data）配置，
 * 因此请求体不再接受 providerId。
 * @param {string} projectId
 * @param {string} tableId
 * @param {{ rowCount: number, preserveManual?: boolean }} data
 */
export const generateTestDataRows = (projectId, tableId, data) =>
  request.post(`/projects/${projectId}/test-data-tables/${tableId}/rows/generate`, {
    rowCount: Number(data?.rowCount) || 1,
    preserveManual: !!data?.preserveManual,
  }, { timeout: 120000 })
/**
 * 按列重新生成单行单元格。AI 列同上由项目 Prompt 管理控制，请求体不再接受 providerId。
 * @param {string} projectId
 * @param {string} tableId
 * @param {string} rowId
 * @param {{ columnKeys: string[] }} data
 */
export const regenerateTestDataCells = (projectId, tableId, rowId, data) =>
  request.post(`/projects/${projectId}/test-data-tables/${tableId}/rows/${rowId}/regenerate`, {
    columnKeys: Array.isArray(data?.columnKeys) ? data.columnKeys.slice() : [],
  }, { timeout: 120000 })
export const listMockHelpers = () => request.get('/test-data/mock-helpers').then(asList)

export const listNotifications = (params = {}) => request.get('/notifications', { params })
export const markNotificationRead = (id) => request.patch(`/notifications/${id}/read`)
export const markAllNotificationsRead = () => request.post('/notifications/read-all')
export const clearAllNotifications = () => request.post('/notifications/clear-all')
