import request from './request'

/** 列表接口若返回 JSON null，规范为空数组，避免表格等对 null 异常 */
function asList(data) {
  return Array.isArray(data) ? data : []
}

export const login = (data) => request.post('/auth/login', data)
export const logout = () => request.post('/auth/logout')
export const getMe = () => request.get('/auth/me')

export const listProjects = () => request.get('/projects')
export const createProject = (data) => request.post('/projects', data)
export const deleteProject = (id) => request.delete(`/projects/${id}`)
export const listServices = (projectId) => request.get(`/projects/${projectId}/services`)
export const createService = (projectId, data) => request.post(`/projects/${projectId}/services`, data)
export const updateService = (projectId, serviceId, data) => request.put(`/projects/${projectId}/services/${serviceId}`, data)
export const deleteService = (projectId, serviceId) => request.delete(`/projects/${projectId}/services/${serviceId}`)
export const listServiceEnvironments = (projectId, serviceId) =>
  request.get(`/projects/${projectId}/services/${serviceId}/environments`).then(asList)
export const createServiceEnvironment = (projectId, serviceId, data) => request.post(`/projects/${projectId}/services/${serviceId}/environments`, data)
export const updateServiceEnvironment = (projectId, serviceId, environmentId, data) => request.put(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`, data)
export const deleteServiceEnvironment = (projectId, serviceId, environmentId) => request.delete(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`)
export const listEnvironments = (projectId) => request.get(`/projects/${projectId}/environments`)
export const createEnvironment = (projectId, data) => request.post(`/projects/${projectId}/environments`, data)
export const updateEnvironment = (projectId, environmentId, data) => request.put(`/projects/${projectId}/environments/${environmentId}`, data)
export const deleteEnvironment = (projectId, environmentId) => request.delete(`/projects/${projectId}/environments/${environmentId}`)

export const importSpec = (projectId, serviceId, content) =>
  request.post(`/projects/${projectId}/services/${serviceId}/specs/import`, content, {
    headers: { 'Content-Type': 'application/yaml' }
  })
export const listSpecs = (projectId, serviceId) => request.get(`/projects/${projectId}/services/${serviceId}/specs`)
export const listEndpoints = (projectId, serviceId) => request.get(`/projects/${projectId}/services/${serviceId}/endpoints`)

export const listCases = (params = {}) => request.get('/cases', { params })
export const getCase = (caseId) => request.get(`/cases/${caseId}`)
export const createCase = (data) => request.post('/cases', data)
export const generateCaseParams = (caseId) => request.get(`/cases/${caseId}/generate-params`)
export const runCase = (caseId, data) => request.post(`/cases/${caseId}/run`, data)
export const getRunResult = (runId) => request.get(`/runs/${runId}/result`)

export const listSuites = (params = {}) => request.get('/suites', { params })
export const createSuite = (data) => request.post('/suites', data)
export const listSuiteItems = (suiteId) => request.get(`/suites/${suiteId}/items`)
export const addSuiteItem = (suiteId, data) => request.post(`/suites/${suiteId}/items`, data)

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
