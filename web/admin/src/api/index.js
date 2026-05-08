import request from './request'

/** 列表接口若返回 JSON null，规范为空数组，避免表格等对 null 异常 */
function asList(data) {
  return Array.isArray(data) ? data : []
}

export const login = (data) => request.post('/auth/login', data)
export const logout = () => request.post('/auth/logout')
export const getMe = () => request.get('/auth/me')

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
export const listDataSources = (params = {}) => request.get('/data-sources', { params }).then(asList)
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
