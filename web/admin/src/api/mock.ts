/**
 * Mock Server 相关 API
 */
import request, { asList } from './request'

// Mock Server
export const listMockServers = (projectId: string) =>
  request.get(`/projects/${projectId}/mock-servers`).then(asList())

export const createMockServer = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/mock-servers`, data)

export const updateMockServer = (projectId: string, serverId: string, data: Record<string, unknown>) =>
  request.put(`/projects/${projectId}/mock-servers/${serverId}`, data)

export const deleteMockServer = (projectId: string, serverId: string) =>
  request.delete(`/projects/${projectId}/mock-servers/${serverId}`)

export const startMockServer = (projectId: string, serverId: string) =>
  request.post(`/projects/${projectId}/mock-servers/${serverId}/start`)

export const stopMockServer = (projectId: string, serverId: string) =>
  request.post(`/projects/${projectId}/mock-servers/${serverId}/stop`)

// Mock Routes
export const listMockRoutes = (projectId: string, serverId: string) =>
  request.get(`/projects/${projectId}/mock-servers/${serverId}/routes`).then(asList())

export const createMockRoute = (projectId: string, serverId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/mock-servers/${serverId}/routes`, data)

export const updateMockRoute = (projectId: string, serverId: string, routeId: string, data: Record<string, unknown>) =>
  request.put(`/projects/${projectId}/mock-servers/${serverId}/routes/${routeId}`, data)

export const deleteMockRoute = (projectId: string, serverId: string, routeId: string) =>
  request.delete(`/projects/${projectId}/mock-servers/${serverId}/routes/${routeId}`)

// Mock 访问日志
export const listMockAccessLogs = (projectId: string, serverId: string, params: Record<string, unknown> = {}) =>
  request.get(`/projects/${projectId}/mock-servers/${serverId}/access-logs`, { params })

export const clearMockAccessLogs = (projectId: string, serverId: string) =>
  request.delete(`/projects/${projectId}/mock-servers/${serverId}/access-logs`)

// Mock Value Sets
export const listMockValueSets = (projectId: string) =>
  request.get(`/projects/${projectId}/mock-value-sets`).then(asList())

export const createMockValueSet = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/mock-value-sets`, data)

export const updateMockValueSet = (projectId: string, setId: string, data: Record<string, unknown>) =>
  request.put(`/projects/${projectId}/mock-value-sets/${setId}`, data)

export const deleteMockValueSet = (projectId: string, setId: string) =>
  request.delete(`/projects/${projectId}/mock-value-sets/${setId}`)
