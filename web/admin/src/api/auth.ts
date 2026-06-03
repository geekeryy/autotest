/**
 * 认证相关 API
 */
import request, { asList } from './request'

// 登录/登出
export const login = (data: { username: string; password: string }) =>
  request.post('/auth/login', data)

export const logout = () => request.post('/auth/logout')

export const changePassword = (data: { oldPassword: string; newPassword: string }) =>
  request.post('/auth/change-password', data)

export const getMe = () => request.get('/auth/me')

export const getGitHubOAuthStatus = () => request.get('/auth/github/status')

// OAuth 登录方式
export const listLoginProviders = () =>
  request.get('/auth/login-providers').then(asList())

export const listAuthProviderTypes = () =>
  request.get('/auth-provider-types').then(asList())

export const listAuthProviders = () =>
  request.get('/auth-providers').then(asList())

export const createAuthProvider = (data: Record<string, unknown>) =>
  request.post('/auth-providers', data)

export const updateAuthProvider = (providerId: string, data: Record<string, unknown>) =>
  request.put(`/auth-providers/${providerId}`, data)

export const deleteAuthProvider = (providerId: string) =>
  request.delete(`/auth-providers/${providerId}`)

export const testAuthProvider = (providerId: string) =>
  request.post(`/auth-providers/${providerId}/test`)
