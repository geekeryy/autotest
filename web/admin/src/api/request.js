import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import { apiBaseURL } from '../utils/apiBase'
import { clearToken, getToken } from '../utils/storage'

const request = axios.create({
  baseURL: apiBaseURL(),
  timeout: 20000
})

request.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status
    const message = error.response?.data?.error || error.message || '请求失败'
    if (status === 403) {
      const msg = String(message).toLowerCase()
      const pending = msg.includes('pending') || String(message).includes('审核')
      if (pending) {
        if (router.currentRoute.value.path !== '/pending-approval') {
          router.replace('/pending-approval')
        }
        ElMessage.error(message)
        return Promise.reject(error)
      }
      const mustChange = msg.includes('change password') || String(message).includes('修改密码')
      if (mustChange) {
        if (router.currentRoute.value.path !== '/change-password') {
          router.replace('/change-password')
        }
        ElMessage.error('首次登录须修改密码')
        return Promise.reject(error)
      }
    }
    if (status === 401) {
      clearToken()
      if (router.currentRoute.value.path !== '/login') {
        router.replace('/login')
      }
    }
    ElMessage.error(message)
    return Promise.reject(error)
  }
)

export default request
