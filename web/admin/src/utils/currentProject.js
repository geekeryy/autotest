import { reactive } from 'vue'
import { listProjects } from '../api'

const STORAGE_KEY = 'autotest.currentProjectId'

let loadingPromise = null

function readStoredProjectId() {
  try {
    return localStorage.getItem(STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

function storeProjectId(projectId) {
  try {
    if (projectId) {
      localStorage.setItem(STORAGE_KEY, projectId)
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // Ignore storage failures so project switching still works in memory.
  }
}

export const projectState = reactive({
  projects: [],
  currentProjectId: readStoredProjectId(),
  loading: false,
  loaded: false
})

export function setCurrentProjectId(projectId) {
  projectState.currentProjectId = projectId || ''
  storeProjectId(projectState.currentProjectId)
}

export function getCurrentProject() {
  return projectState.projects.find((project) => project.id === projectState.currentProjectId) || null
}

export async function loadGlobalProjects(options = {}) {
  if (projectState.loaded && !options.force) return projectState.projects
  if (loadingPromise) return loadingPromise

  projectState.loading = true
  loadingPromise = listProjects()
    .then((projects) => {
      projectState.projects = projects
      const currentExists = projects.some((project) => project.id === projectState.currentProjectId)
      setCurrentProjectId(currentExists ? projectState.currentProjectId : projects[0]?.id || '')
      projectState.loaded = true
      return projects
    })
    .catch(() => {
      // 错误提示由 axios 拦截器处理；保持可再次拉取
      projectState.loaded = false
      projectState.projects = []
      setCurrentProjectId('')
      return []
    })
    .finally(() => {
      projectState.loading = false
      loadingPromise = null
    })

  return loadingPromise
}

export function refreshGlobalProjects() {
  return loadGlobalProjects({ force: true })
}
