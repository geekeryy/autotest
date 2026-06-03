/**
 * 全局应用状态 Store
 * 整合认证、外观主题、全局通知等
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User } from '@/types/api'
import { apiClient } from '@/api/request'

// ── Token 管理 ──────────────────────────────────────────────────
const TOKEN_KEY = 'autotest_admin_token'

function readToken(): string | null {
  try { return localStorage.getItem(TOKEN_KEY) } catch { return null }
}

// ── Store ──────────────────────────────────────────────────────────
export const useAppStore = defineStore('app', () => {
  // 状态
  const token = ref<string | null>(readToken())
  const currentUser = ref<User | null>(null)
  const sidebarCollapsed = ref(false)

  // 计算属性
  const isLoggedIn = computed(() => !!token.value)

  // 认证方法
  function setToken(newToken: string | null) {
    token.value = newToken
    try {
      if (newToken) {
        localStorage.setItem(TOKEN_KEY, newToken)
      } else {
        localStorage.removeItem(TOKEN_KEY)
      }
    } catch { /* ignore */ }
  }

  function logout() {
    setToken(null)
    currentUser.value = null
    window.location.href = '/login'
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return {
    token,
    currentUser,
    sidebarCollapsed,
    isLoggedIn,
    setToken,
    logout,
    toggleSidebar,
  }
})
