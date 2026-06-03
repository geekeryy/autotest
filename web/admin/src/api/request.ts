/**
 * Axios 实例和请求/响应拦截器
 * 替代 api/request.js，TypeScript 化
 */
import axios, { type AxiosInstance, type InternalAxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { apiBaseURL } from '@/utils/apiBase'
import { clearToken } from '@/utils/storage'

const TOKEN_KEY = 'autotest_admin_token'

function getToken(): string | null {
  try { return localStorage.getItem(TOKEN_KEY) } catch { return null }
}

export const apiClient: AxiosInstance = axios.create({
  baseURL: apiBaseURL(),
  timeout: 20000,
})

// 请求拦截器：自动注入 Authorization
apiClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：统一错误处理
apiClient.interceptors.response.use(
  (response: AxiosResponse) => response.data,
  (error) => {
    const status = error.response?.status
    const message = error.response?.data?.error || error.message || '请求失败'

    if (status === 403) {
      const msg = String(message).toLowerCase()
      const pending = msg.includes('pending') || String(message).includes('审核')
      if (pending) {
        if (router.currentRoute.value.path !== '/pending-approval') {
          void router.replace('/pending-approval')
        }
        ElMessage.error(message)
        return Promise.reject(error)
      }
      const mustChange = msg.includes('change password') || String(message).includes('修改密码')
      if (mustChange) {
        if (router.currentRoute.value.path !== '/change-password') {
          void router.replace('/change-password')
        }
        ElMessage.error('首次登录须修改密码')
        return Promise.reject(error)
      }
    }

    if (status === 401) {
      clearToken()
      if (router.currentRoute.value.path !== '/login') {
        void router.replace('/login')
      }
    }

    ElMessage.error(String(message))
    return Promise.reject(error)
  }
)

/** 响应拦截器已解包为 response.data */
export interface ApiClient {
  get<T = unknown>(url: string, config?: Parameters<AxiosInstance['get']>[1]): Promise<T>
  post<T = unknown>(url: string, data?: unknown, config?: Parameters<AxiosInstance['post']>[2]): Promise<T>
  put<T = unknown>(url: string, data?: unknown, config?: Parameters<AxiosInstance['post']>[2]): Promise<T>
  patch<T = unknown>(url: string, data?: unknown, config?: Parameters<AxiosInstance['patch']>[2]): Promise<T>
  delete<T = unknown>(url: string, config?: Parameters<AxiosInstance['delete']>[1]): Promise<T>
}

/** 列表接口若返回 JSON null，规范为空数组（用于 Promise.then） */
export function asList<T>() {
  return (data: unknown): T[] => (Array.isArray(data) ? (data as T[]) : [])
}

const request = apiClient as unknown as ApiClient

export default request
