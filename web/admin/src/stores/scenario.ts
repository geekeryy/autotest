/**
 * 场景状态 Store
 * 管理场景列表、编辑中的步骤、Tab 状态等
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UUID } from '@/types/common'
import type {
  Scenario,
  ScenarioStep,
  ScenarioListFilter,
  CreateScenarioInput,
  UpdateScenarioInput,
  UpsertStepInput,
  ReorderStepsInput,
} from '@/types/scenario'
import * as scenarioApi from '@/api/scenario'

// ── Tab 持久化 ──────────────────────────────────────────────────
const TABS_KEY = 'autotest.scenario.openTabs'
const ACTIVE_TAB_KEY = 'autotest.scenario.activeTab'

interface ScenarioTab {
  id: string
  scenarioId: UUID
  name: string
}

function loadTabs(projectId: string, serviceId: string): ScenarioTab[] {
  try {
    const key = `${TABS_KEY}.${projectId}.${serviceId}`
    return JSON.parse(localStorage.getItem(key) || '[]')
  } catch { return [] }
}

function saveTabs(projectId: string, serviceId: string, tabs: ScenarioTab[]) {
  try {
    const key = `${TABS_KEY}.${projectId}.${serviceId}`
    localStorage.setItem(key, JSON.stringify(tabs))
  } catch { /* ignore */ }
}

function loadActiveTab(projectId: string, serviceId: string): string | null {
  try {
    const key = `${ACTIVE_TAB_KEY}.${projectId}.${serviceId}`
    return localStorage.getItem(key)
  } catch { return null }
}

function saveActiveTab(projectId: string, serviceId: string, tabId: string | null) {
  try {
    const key = `${ACTIVE_TAB_KEY}.${projectId}.${serviceId}`
    if (tabId) localStorage.setItem(key, tabId)
    else localStorage.removeItem(key)
  } catch { /* ignore */ }
}

// ── Store ──────────────────────────────────────────────────────────
export const useScenarioStore = defineStore('scenario', () => {
  // 状态
  const scenarios = ref<Scenario[]>([])
  const openTabs = ref<ScenarioTab[]>([])
  const activeTabId = ref<string | null>(null)
  const editingSteps = ref<Map<UUID, ScenarioStep[]>>(new Map())
  const loading = ref(false)
  const currentFilter = ref<ScenarioListFilter | null>(null)

  // 计算属性
  const activeScenario = computed(() => {
    if (!activeTabId.value) return null
    const tab = openTabs.value.find(t => t.id === activeTabId.value)
    return tab ? scenarios.value.find(s => s.id === tab.scenarioId) || null : null
  })

  const activeSteps = computed(() => {
    if (!activeScenario.value) return []
    return editingSteps.value.get(activeScenario.value.id) || []
  })

  // ── 场景 CRUD ──────────────────────────────────────────────────
  async function loadScenarios(filter: ScenarioListFilter) {
    currentFilter.value = filter
    loading.value = true
    try {
      scenarios.value = await scenarioApi.listScenarios(filter)
    } catch {
      scenarios.value = []
    } finally {
      loading.value = false
    }
  }

  async function createScenario(input: CreateScenarioInput): Promise<Scenario | null> {
    try {
      const scenario = await scenarioApi.createScenario(input)
      scenarios.value.unshift(scenario)
      return scenario
    } catch {
      return null
    }
  }

  async function updateScenario(id: UUID, input: UpdateScenarioInput): Promise<Scenario | null> {
    try {
      const scenario = await scenarioApi.updateScenario(id, input)
      const idx = scenarios.value.findIndex(s => s.id === id)
      if (idx >= 0) scenarios.value[idx] = scenario
      // 更新 Tab 名称
      const tab = openTabs.value.find(t => t.scenarioId === id)
      if (tab) tab.name = scenario.name
      return scenario
    } catch {
      return null
    }
  }

  async function deleteScenario(id: UUID): Promise<boolean> {
    try {
      await scenarioApi.deleteScenario(id)
      scenarios.value = scenarios.value.filter(s => s.id !== id)
      // 关闭对应的 Tab
      closeTabByScenarioId(id)
      // 清除步骤缓存
      editingSteps.value.delete(id)
      return true
    } catch {
      return false
    }
  }

  // ── 步骤管理 ──────────────────────────────────────────────────
  async function loadSteps(scenarioId: UUID) {
    try {
      const steps = await scenarioApi.listSteps(scenarioId)
      editingSteps.value.set(scenarioId, steps)
      return steps
    } catch {
      editingSteps.value.set(scenarioId, [])
      return []
    }
  }

  async function saveStep(scenarioId: UUID, input: UpsertStepInput): Promise<ScenarioStep | null> {
    try {
      const step = await scenarioApi.upsertStep(scenarioId, input)
      // 更新本地缓存
      const steps = editingSteps.value.get(scenarioId) || []
      const idx = steps.findIndex(s => s.id === step.id)
      if (idx >= 0) steps[idx] = step
      else steps.push(step)
      steps.sort((a, b) => a.stepOrder - b.stepOrder)
      editingSteps.value.set(scenarioId, steps)
      return step
    } catch {
      return null
    }
  }

  async function removeStep(scenarioId: UUID, stepId: UUID): Promise<boolean> {
    try {
      await scenarioApi.deleteStep(scenarioId, stepId)
      const steps = editingSteps.value.get(scenarioId) || []
      editingSteps.value.set(scenarioId, steps.filter(s => s.id !== stepId))
      return true
    } catch {
      return false
    }
  }

  async function reorderSteps(scenarioId: UUID, input: ReorderStepsInput): Promise<boolean> {
    try {
      await scenarioApi.reorderSteps(scenarioId, input)
      // 重新加载步骤
      await loadSteps(scenarioId)
      return true
    } catch {
      return false
    }
  }

  // ── Tab 管理 ──────────────────────────────────────────────────
  function openTab(scenario: Scenario) {
    const existing = openTabs.value.find(t => t.scenarioId === scenario.id)
    if (existing) {
      activeTabId.value = existing.id
    } else {
      const tab: ScenarioTab = {
        id: `tab-${scenario.id}`,
        scenarioId: scenario.id,
        name: scenario.name,
      }
      openTabs.value.push(tab)
      activeTabId.value = tab.id
    }
    // 持久化
    if (currentFilter.value) {
      saveTabs(currentFilter.value.projectId, currentFilter.value.serviceId, openTabs.value)
      saveActiveTab(currentFilter.value.projectId, currentFilter.value.serviceId, activeTabId.value)
    }
    // 加载步骤
    loadSteps(scenario.id)
  }

  function closeTab(tabId: string) {
    const idx = openTabs.value.findIndex(t => t.id === tabId)
    if (idx < 0) return
    openTabs.value.splice(idx, 1)
    // 如果关闭的是当前激活的 Tab，切换到相邻 Tab
    if (activeTabId.value === tabId) {
      activeTabId.value = openTabs.value[Math.min(idx, openTabs.value.length - 1)]?.id || null
    }
    // 持久化
    if (currentFilter.value) {
      saveTabs(currentFilter.value.projectId, currentFilter.value.serviceId, openTabs.value)
      saveActiveTab(currentFilter.value.projectId, currentFilter.value.serviceId, activeTabId.value)
    }
  }

  function closeTabByScenarioId(scenarioId: UUID) {
    const tab = openTabs.value.find(t => t.scenarioId === scenarioId)
    if (tab) closeTab(tab.id)
  }

  function setActiveTab(tabId: string) {
    activeTabId.value = tabId
    if (currentFilter.value) {
      saveActiveTab(currentFilter.value.projectId, currentFilter.value.serviceId, tabId)
    }
    // 加载步骤
    const tab = openTabs.value.find(t => t.id === tabId)
    if (tab) loadSteps(tab.scenarioId)
  }

  // ── 初始化 ──────────────────────────────────────────────────────
  function initTabs(projectId: string, serviceId: string) {
    openTabs.value = loadTabs(projectId, serviceId)
    const savedActive = loadActiveTab(projectId, serviceId)
    if (savedActive && openTabs.value.some(t => t.id === savedActive)) {
      activeTabId.value = savedActive
    } else {
      activeTabId.value = openTabs.value[0]?.id || null
    }
    // 加载当前 Tab 的步骤
    if (activeTabId.value) {
      const tab = openTabs.value.find(t => t.id === activeTabId.value)
      if (tab) loadSteps(tab.scenarioId)
    }
  }

  return {
    // 状态
    scenarios,
    openTabs,
    activeTabId,
    editingSteps,
    loading,
    currentFilter,

    // 计算属性
    activeScenario,
    activeSteps,

    // 场景 CRUD
    loadScenarios,
    createScenario,
    updateScenario,
    deleteScenario,

    // 步骤管理
    loadSteps,
    saveStep,
    removeStep,
    reorderSteps,

    // Tab 管理
    openTab,
    closeTab,
    setActiveTab,
    initTabs,
  }
})
