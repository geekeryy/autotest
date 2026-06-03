/**
 * 认证管理 Composable
 * 替代 utils/storage.js + permission.js 的认证逻辑
 */
import { ref, computed } from 'vue'
import type { User } from '@/types/api'

const TOKEN_KEY = 'autotest_admin_token'

export function useAuth() {
  const token = ref<string | null>(readToken())
  const currentUser = ref<User | null>(null)

  const isLoggedIn = computed(() => !!token.value)

  function readToken(): string | null {
    try { return localStorage.getItem(TOKEN_KEY) } catch { return null }
  }

  function setToken(newToken: string | null) {
    token.value = newToken
    try {
      if (newToken) localStorage.setItem(TOKEN_KEY, newToken)
      else localStorage.removeItem(TOKEN_KEY)
    } catch { /* ignore */ }
  }

  function clearToken() {
    setToken(null)
    currentUser.value = null
  }

  function logout() {
    clearToken()
    window.location.href = '/login'
  }

  return {
    token,
    currentUser,
    isLoggedIn,
    setToken,
    clearToken,
    logout,
  }
}
