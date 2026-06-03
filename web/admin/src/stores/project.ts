/**
 * 项目状态 Store
 * 替代 utils/currentProject.js 的 reactive 单例模式
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Project } from '@/types/api'
import { listProjects } from '@/api/project'

const STORAGE_KEY = 'autotest.currentProjectId'

function readStoredProjectId(): string {
  try { return localStorage.getItem(STORAGE_KEY) || '' } catch { return '' }
}

function storeProjectId(id: string) {
  try {
    if (id) localStorage.setItem(STORAGE_KEY, id)
    else localStorage.removeItem(STORAGE_KEY)
  } catch { /* ignore */ }
}

export const useProjectStore = defineStore('project', () => {
  // 状态
  const projects = ref<Project[]>([])
  const currentProjectId = ref<string>(readStoredProjectId())
  const loading = ref(false)
  const loaded = ref(false)

  // 计算属性
  const currentProject = computed(() =>
    projects.value.find(p => p.id === currentProjectId.value) || null
  )

  // 方法
  function setCurrentProject(id: string) {
    currentProjectId.value = id || ''
    storeProjectId(currentProjectId.value)
  }

  async function loadProjects(force = false): Promise<Project[]> {
    if (loaded.value && !force) return projects.value
    loading.value = true
    try {
      const result = await listProjects()
      projects.value = result
      // 验证当前项目是否仍然存在
      const exists = result.some(p => p.id === currentProjectId.value)
      setCurrentProject(exists ? currentProjectId.value : result[0]?.id || '')
      loaded.value = true
      return result
    } catch {
      loaded.value = false
      projects.value = []
      setCurrentProject('')
      return []
    } finally {
      loading.value = false
    }
  }

  function refreshProjects() {
    return loadProjects(true)
  }

  return {
    projects,
    currentProjectId,
    currentProject,
    loading,
    loaded,
    setCurrentProject,
    loadProjects,
    refreshProjects,
  }
})
