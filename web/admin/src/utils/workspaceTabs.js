// 多 Tab 工作区（场景编排 / 运行控制台）的 Tab 列表持久化助手。
// 存储结构（按 projectId+serviceId 维度独立保存）：
//   { "<projectId>:<serviceId>": { tabs: [...], activeTabId: '' } }
// tabs 元素允许携带任意可序列化字段（如 name/method/path），由调用方约定。

export const SCENARIO_WORKSPACE_KEY = 'autotest.scenarioWorkspaceByProjectService'
export const RUN_CONSOLE_WORKSPACE_KEY = 'autotest.runConsoleWorkspaceByProjectService'

const EMPTY_STATE = Object.freeze({ tabs: [], activeTabId: '' })

function scopeId(projectId, serviceId) {
  if (!projectId || !serviceId) return ''
  return `${projectId}:${serviceId}`
}

function readBucket(storageKey) {
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function writeBucket(storageKey, bucket) {
  try {
    localStorage.setItem(storageKey, JSON.stringify(bucket))
  } catch {
    // ignore storage failures
  }
}

export function readTabState(storageKey, projectId, serviceId) {
  const id = scopeId(projectId, serviceId)
  if (!id) return { tabs: [], activeTabId: '' }
  const bucket = readBucket(storageKey)
  const entry = bucket[id]
  if (!entry || typeof entry !== 'object') return { tabs: [], activeTabId: '' }
  const tabs = Array.isArray(entry.tabs) ? entry.tabs.filter((t) => t && t.id) : []
  const activeTabId = typeof entry.activeTabId === 'string' ? entry.activeTabId : ''
  return { tabs, activeTabId }
}

export function writeTabState(storageKey, projectId, serviceId, state) {
  const id = scopeId(projectId, serviceId)
  if (!id) return
  const tabs = Array.isArray(state?.tabs) ? state.tabs.filter((t) => t && t.id) : []
  const activeTabId = typeof state?.activeTabId === 'string' ? state.activeTabId : ''
  const bucket = readBucket(storageKey)
  if (!tabs.length && !activeTabId) {
    if (bucket[id]) {
      delete bucket[id]
      writeBucket(storageKey, bucket)
    }
    return
  }
  bucket[id] = { tabs, activeTabId }
  writeBucket(storageKey, bucket)
}

export function clearTabState(storageKey, projectId, serviceId) {
  const id = scopeId(projectId, serviceId)
  if (!id) return
  const bucket = readBucket(storageKey)
  if (!bucket[id]) return
  delete bucket[id]
  writeBucket(storageKey, bucket)
}

// 工具：把 EMPTY_STATE 暴露给只读消费者，避免重复构造。
export function emptyTabState() {
  return { ...EMPTY_STATE }
}
