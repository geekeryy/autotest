import { reactive } from 'vue'
import { getMe } from './api'

export const authState = reactive({
  user: null,
  loaded: false
})

export async function loadCurrentUser() {
  const user = await getMe()
  authState.user = user
  authState.loaded = true
  return user
}

export function resetCurrentUser() {
  authState.user = null
  authState.loaded = false
}

export function permissionCodes() {
  return (authState.user?.permissions || []).map((item) => item.code)
}

export function hasPermission(code) {
  if (!code) return true
  return permissionCodes().includes(code)
}

export function hasAnyPermission(codes) {
  if (!codes?.length) return true
  return codes.some((code) => hasPermission(code))
}

export function routePermissionAllowed(meta) {
  if (meta?.permissions?.length) {
    return hasAnyPermission(meta.permissions)
  }
  return hasPermission(meta?.permission)
}
