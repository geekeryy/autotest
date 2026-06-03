/**
 * 管理相关 API
 * 包括：用户、角色、权限、API Key、审计日志、通知
 */
import request, { asList } from './request'

// ── 用户管理 ──────────────────────────────────────────────────
export const listUsers = () =>
  request.get('/users')

export const createUser = (data: Record<string, unknown>) =>
  request.post('/users', data)

export const updateUser = (id: string, data: Record<string, unknown>) =>
  request.put(`/users/${id}`, data)

export const deleteUser = (id: string) =>
  request.delete(`/users/${id}`)

// ── 角色管理 ──────────────────────────────────────────────────
export const listRoles = () =>
  request.get('/roles')

export const createRole = (data: Record<string, unknown>) =>
  request.post('/roles', data)

export const updateRole = (id: string, data: Record<string, unknown>) =>
  request.put(`/roles/${id}`, data)

export const deleteRole = (id: string) =>
  request.delete(`/roles/${id}`)

export const setRolePermissions = (id: string, data: Record<string, unknown>) =>
  request.put(`/roles/${id}/permissions`, data)

// ── 权限管理 ──────────────────────────────────────────────────
export const listPermissions = () =>
  request.get('/permissions')

export const createPermission = (data: Record<string, unknown>) =>
  request.post('/permissions', data)

// ── API Key ──────────────────────────────────────────────────
export const listApiKeys = () =>
  request.get('/api-keys').then(asList())

export const createApiKey = (data: Record<string, unknown>) =>
  request.post('/api-keys', data)

export const updateApiKey = (id: string, data: Record<string, unknown>) =>
  request.patch(`/api-keys/${id}`, data)

export const rotateApiKey = (id: string) =>
  request.post(`/api-keys/${id}/rotate`)

export const deleteApiKey = (id: string) =>
  request.delete(`/api-keys/${id}`)

// ── 审计日志 ──────────────────────────────────────────────────
export const listAuditLogs = (params: Record<string, unknown> = {}) =>
  request.get('/audit-logs', { params })

// ── 通知 ──────────────────────────────────────────────────────
export const listNotifications = (params: Record<string, unknown> = {}) =>
  request.get('/notifications', { params })

export const markNotificationRead = (id: string) =>
  request.patch(`/notifications/${id}/read`)

export const markAllNotificationsRead = () =>
  request.post('/notifications/read-all')

export const clearAllNotifications = () =>
  request.post('/notifications/clear-all')
