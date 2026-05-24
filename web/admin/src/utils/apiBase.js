const API_PREFIX = '/api/v1'

/** 本地 dev 默认 API 地址，与 vite.config.js proxy target 一致。 */
const DEV_API_ORIGIN = 'http://localhost:8080'

function normalizeOrigin(origin) {
  return String(origin).replace(/\/$/, '')
}

/**
 * API 公网 origin（OAuth 回调等需 IdP 登记的绝对 URL）。
 * 优先 VITE_API_BASE_URL；dev 未配置时用 localhost:8080；生产同域部署用当前页面 origin。
 */
export function apiPublicOrigin() {
  const envOrigin = import.meta.env.VITE_API_BASE_URL
  if (envOrigin) return normalizeOrigin(envOrigin)
  if (import.meta.env.DEV) return DEV_API_ORIGIN
  return window.location.origin
}

/** API 根路径：dev 始终用相对路径（走 Vite 代理）；生产用 VITE_API_BASE_URL + /api/v1。 */
export function apiBaseURL() {
  if (import.meta.env.DEV) return API_PREFIX
  const origin = import.meta.env.VITE_API_BASE_URL
  if (!origin) return API_PREFIX
  return `${apiPublicOrigin()}${API_PREFIX}`
}

/** OAuth 回调 URL，由 API 域名 + slug 组成；保存登录方式时提交给后端持久化。 */
export function oauthCallbackUrl(slug) {
  const normalized = String(slug || '').trim().toLowerCase()
  if (!normalized) return ''
  return `${apiPublicOrigin()}${API_PREFIX}/auth/oauth/${normalized}/callback`
}
