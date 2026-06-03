// Global AI assistant store
// =========================
//
// Conversation state is split into panes: floating `panel` and workspace `w0`..`w5`
// (up to 6 side-by-side chats on the AI assistant page).

import { reactive } from 'vue'
import {
  createAISession,
  deleteAISession,
  getAISession,
  listAIProviderModels,
  listAIProviders,
  listAIProviderTypes,
  listAISessions,
  renameAISession,
} from '../api'
import { streamPostSSE } from '../utils/aiStream'
import {
  providerDefaultModel,
  providerSupportsThinking,
  providerSupportsWebSearch,
} from '../utils/aiProvider'

const STORAGE_KEY = 'autotest.ai-assistant.sessionId'
const SETTINGS_KEY = 'autotest.ai-assistant.settings'
const SPLIT_MODE_KEY = 'autotest.ai-assistant.splitMode'
const FOCUSED_PANE_KEY = 'autotest.ai-assistant.focusedPane'
const WORKSPACE_PANES_KEY = 'autotest.ai-assistant.workspacePaneIds'

export const MAX_WORKSPACE_PANES = 6
export const WORKSPACE_PANE_PREFIX = 'w'

const LEGACY_WORKSPACE_MAP = { left: 'w0', right: 'w1' }
const LEGACY_WORKSPACE_IDS = ['left', 'right']

export function workspacePaneId(index) {
  return `${WORKSPACE_PANE_PREFIX}${index}`
}

export function isWorkspacePaneId(paneId) {
  return /^w\d+$/.test(String(paneId || ''))
}

function allWorkspacePaneSlots() {
  return Array.from({ length: MAX_WORKSPACE_PANES }, (_, i) => workspacePaneId(i))
}

export const PANE_IDS = ['panel', ...allWorkspacePaneSlots()]

export function getWorkspacePaneIds() {
  return [...assistantState.workspacePaneIds]
}


function createPaneState() {
  return {
    activeSessionId: '',
    messages: [],
    pendingCalls: [],
    streaming: false,
    error: '',
    abortController: null,
    requestSeq: 0,
    selectedProviderId: '',
    selectedModel: '',
    thinkingEnabled: true,
    webSearchEnabled: false,
    debugEnabled: false,
    remoteModelList: [],
    modelsWarning: '',
    genJobProgress: null,
  }
}

function createInitialPanes() {
  const panes = { panel: createPaneState() }
  for (const id of allWorkspacePaneSlots()) {
    panes[id] = createPaneState()
  }
  return panes
}

function migrateLegacyPaneId(paneId) {
  return LEGACY_WORKSPACE_MAP[paneId] || paneId
}

function readActiveId(projectId, paneId = 'panel') {
  if (!projectId) return ''
  const id = migrateLegacyPaneId(paneId)
  try {
    const legacy = window.localStorage.getItem(`${STORAGE_KEY}:${projectId}`)
    const key = `${STORAGE_KEY}:${projectId}:${id}`
    let raw = window.localStorage.getItem(key)
    if (!raw && LEGACY_WORKSPACE_MAP[paneId]) {
      raw = window.localStorage.getItem(`${STORAGE_KEY}:${projectId}:${paneId}`)
    }
    if (!raw && id === 'panel') raw = legacy
    return raw || ''
  } catch {
    return ''
  }
}

function writeActiveId(projectId, paneId, sessionId) {
  if (!projectId) return
  const id = migrateLegacyPaneId(paneId)
  try {
    const key = `${STORAGE_KEY}:${projectId}:${id}`
    if (sessionId) window.localStorage.setItem(key, sessionId)
    else window.localStorage.removeItem(key)
    if (id === 'panel') {
      const legacyKey = `${STORAGE_KEY}:${projectId}`
      if (sessionId) window.localStorage.setItem(legacyKey, sessionId)
      else window.localStorage.removeItem(legacyKey)
    }
  } catch {
    // localStorage may be blocked
  }
}

function readSettings(projectId, paneId) {
  if (!projectId) return {}
  const id = migrateLegacyPaneId(paneId)
  try {
    let raw = window.localStorage.getItem(`${SETTINGS_KEY}:${projectId}:${id}`)
    if (!raw && LEGACY_WORKSPACE_MAP[paneId]) {
      raw = window.localStorage.getItem(`${SETTINGS_KEY}:${projectId}:${paneId}`)
    }
    if (!raw && id === 'w0') {
      raw = window.localStorage.getItem(`${SETTINGS_KEY}:${projectId}`)
    }
    return raw ? JSON.parse(raw) || {} : {}
  } catch {
    return {}
  }
}

function writePaneSettings(paneId) {
  if (!assistantState.projectId) return
  const pane = getPane(paneId)
  if (!pane) return
  const id = migrateLegacyPaneId(paneId)
  try {
    window.localStorage.setItem(
      `${SETTINGS_KEY}:${assistantState.projectId}:${id}`,
      JSON.stringify({
        providerId: pane.selectedProviderId || '',
        model: pane.selectedModel || '',
        thinkingEnabled: !!pane.thinkingEnabled,
        webSearchEnabled: !!pane.webSearchEnabled,
        debugEnabled: !!pane.debugEnabled,
      })
    )
  } catch {
    // ignore
  }
}

function readSplitModeLegacy() {
  try {
    return window.localStorage.getItem(SPLIT_MODE_KEY) === '1'
  } catch {
    return false
  }
}

function writeSplitModeFlag(enabled) {
  try {
    window.localStorage.setItem(SPLIT_MODE_KEY, enabled ? '1' : '0')
  } catch {
    // ignore
  }
}

function readWorkspacePaneIds() {
  try {
    const raw = window.localStorage.getItem(WORKSPACE_PANES_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed) && parsed.length > 0) {
        const valid = parsed.filter((id) => isWorkspacePaneId(id)).slice(0, MAX_WORKSPACE_PANES)
        if (valid.length) return valid
      }
    }
  } catch {
    // ignore
  }
  if (readSplitModeLegacy()) return ['w0', 'w1']
  return ['w0']
}

function writeWorkspacePaneIds(ids) {
  try {
    window.localStorage.setItem(WORKSPACE_PANES_KEY, JSON.stringify(ids))
  } catch {
    // ignore
  }
}

function readFocusedPane() {
  try {
    const raw = window.localStorage.getItem(FOCUSED_PANE_KEY)
    if (!raw) return 'w0'
    if (raw === 'left') return 'w0'
    if (raw === 'right') return 'w1'
    if (isWorkspacePaneId(raw) || raw === 'panel') return raw
    return 'w0'
  } catch {
    return 'w0'
  }
}

function writeFocusedPane(paneId) {
  try {
    window.localStorage.setItem(FOCUSED_PANE_KEY, migrateLegacyPaneId(paneId))
  } catch {
    // ignore
  }
}

const initialWorkspacePaneIds = readWorkspacePaneIds()

export const assistantState = reactive({
  open: false,
  projectId: '',
  sessions: [],
  hasMoreSessions: false,
  loadingMoreSessions: false,
  providers: [],
  providerTypes: [],
  pageContext: null,
  workspacePaneIds: initialWorkspacePaneIds,
  focusedPaneId: readFocusedPane(),
  panes: createInitialPanes(),
})

Object.defineProperty(assistantState, 'splitMode', {
  get() {
    return assistantState.workspacePaneIds.length > 1
  },
  enumerable: true,
  configurable: true,
})

const inFlightCallIds = new Set()

export function getPane(paneId = 'panel') {
  const id = migrateLegacyPaneId(paneId)
  return assistantState.panes[id] || assistantState.panes.panel
}

export function anyPaneStreaming(paneIds = PANE_IDS) {
  return paneIds.some((id) => getPane(id).streaming)
}

export function paneSessionIds(paneIds = assistantState.workspacePaneIds) {
  return paneIds.map((id) => getPane(id).activeSessionId).filter(Boolean)
}

export function isSessionOpenInPane(sessionId, paneId) {
  return !!sessionId && getPane(paneId).activeSessionId === sessionId
}

export function isSessionOpenInOtherPane(sessionId, paneId) {
  if (!sessionId) return false
  return assistantState.workspacePaneIds.some(
    (id) => id !== migrateLegacyPaneId(paneId) && isSessionOpenInPane(sessionId, id)
  )
}

/** @deprecated use isSessionOpenInOtherPane */
export function isSessionOpenInOtherWorkspacePane(sessionId, paneId) {
  return isSessionOpenInOtherPane(sessionId, paneId)
}

/** Stable 1-based index from slot id w0..w5 (not workspacePaneIds position). */
export function workspacePaneSlotNumber(paneId) {
  const m = String(migrateLegacyPaneId(paneId) || '').match(/^w(\d+)$/)
  if (!m) return 0
  const n = Number(m[1])
  if (!Number.isFinite(n) || n < 0 || n >= MAX_WORKSPACE_PANES) return 0
  return n + 1
}

export function workspacePaneLabel(paneId) {
  const num = workspacePaneSlotNumber(paneId)
  return num > 0 ? `对话 ${num}` : ''
}

export function workspacePaneTag(paneId) {
  const num = workspacePaneSlotNumber(paneId)
  return num > 0 ? String(num) : ''
}

export function canRemoveWorkspacePane(paneId) {
  const id = migrateLegacyPaneId(paneId)
  const ids = assistantState.workspacePaneIds
  if (!ids.includes(id)) return false
  if (ids.length <= 1) return false
  const pane = getPane(id)
  if (pane?.streaming) return false
  return true
}

export function setWorkspaceSplitMode(enabled) {
  const on = !!enabled
  if (on) {
    if (assistantState.workspacePaneIds.length < 2) {
      assistantState.workspacePaneIds = ['w0', 'w1']
    }
  } else {
    assistantState.workspacePaneIds = ['w0']
    setFocusedPane('w0')
  }
  writeWorkspacePaneIds(assistantState.workspacePaneIds)
  writeSplitModeFlag(on)
  if (on && !assistantState.workspacePaneIds.includes(assistantState.focusedPaneId)) {
    setFocusedPane(assistantState.workspacePaneIds[0])
  }
}

/** @deprecated use setWorkspaceSplitMode */
export function setSplitMode(enabled) {
  setWorkspaceSplitMode(enabled)
}

export function setFocusedPane(paneId) {
  const id = migrateLegacyPaneId(paneId)
  if (!assistantState.workspacePaneIds.includes(id) && id !== 'panel') return
  assistantState.focusedPaneId = id
  writeFocusedPane(id)
}

export function addWorkspacePane() {
  if (assistantState.workspacePaneIds.length >= MAX_WORKSPACE_PANES) return false
  const used = new Set(assistantState.workspacePaneIds)
  const next = allWorkspacePaneSlots().find((id) => !used.has(id))
  if (!next) return false
  assistantState.workspacePaneIds = [...assistantState.workspacePaneIds, next]
  writeWorkspacePaneIds(assistantState.workspacePaneIds)
  writeSplitModeFlag(true)
  return true
}

export function removeWorkspacePane(paneId) {
  const id = migrateLegacyPaneId(paneId)
  const ids = assistantState.workspacePaneIds
  if (!ids.includes(id)) return false
  if (ids.length <= 1) return false
  const pane = getPane(id)
  if (pane.streaming) return false

  assistantState.workspacePaneIds = ids.filter((p) => p !== id)
  writeWorkspacePaneIds(assistantState.workspacePaneIds)
  writeSplitModeFlag(assistantState.workspacePaneIds.length > 1)
  resetPane(pane)

  if (assistantState.focusedPaneId === id) {
    setFocusedPane(assistantState.workspacePaneIds[0] || 'w0')
  }
  return true
}

function resetPane(pane) {
  cancelStreamingPane(pane)
  pane.activeSessionId = ''
  pane.messages = []
  pane.pendingCalls = []
  pane.error = ''
  pane.genJobProgress = null
}

function cancelStreamingPane(pane) {
  if (pane.abortController) {
    try {
      pane.abortController.abort()
    } catch {
      // ignore
    }
  }
  pane.abortController = null
  pane.streaming = false
}

function cancelAllStreaming() {
  for (const id of PANE_IDS) {
    cancelStreamingPane(getPane(id))
  }
}

async function applyProviderSettingsToPane(paneId) {
  if (!assistantState.projectId) return
  const pane = getPane(paneId)
  const settings = readSettings(assistantState.projectId, paneId)
  const available = assistantState.providers.filter((p) => p.enabled !== false)
  const savedProvider = available.find((p) => p.id === settings.providerId)
  const fallbackProvider = available.find((p) => p.isDefault) || available[0] || null
  const provider = savedProvider || fallbackProvider
  pane.selectedProviderId = provider?.id || ''
  pane.selectedModel = pickInitialModel(provider, settings)
  pane.thinkingEnabled = pickInitialThinking(provider, settings)
  pane.webSearchEnabled = pickInitialWebSearch(provider, settings)
  pane.debugEnabled = pickInitialDebug(settings)
  await refreshPaneProviderModels(paneId, provider?.id)
  writePaneSettings(paneId)
}

export async function bindProject(projectId) {
  if (assistantState.projectId === projectId) return
  cancelAllStreaming()
  assistantState.projectId = projectId
  assistantState.sessions = []
  for (const id of PANE_IDS) {
    resetPane(getPane(id))
  }
  assistantState.providers = []
  assistantState.providerTypes = []
  if (!projectId) return
  await refreshAssistantProviders()
  await refreshSessions()
  const restoreIds = [...PANE_IDS, ...LEGACY_WORKSPACE_IDS]
  const seen = new Set()
  for (const paneId of restoreIds) {
    const canonical = migrateLegacyPaneId(paneId)
    if (seen.has(canonical)) continue
    seen.add(canonical)
    const desired = readActiveId(projectId, paneId)
    if (desired && assistantState.sessions.some((s) => s.id === desired)) {
      await selectSession(desired, canonical, { restoreOnly: true })
    }
  }
}

export async function refreshAssistantProviders() {
  if (!assistantState.projectId) return
  try {
    const [providers, providerTypes] = await Promise.all([
      listAIProviders(),
      listAIProviderTypes(),
    ])
    assistantState.providers = Array.isArray(providers) ? providers : []
    assistantState.providerTypes = Array.isArray(providerTypes) ? providerTypes : []
  } catch {
    assistantState.providers = []
    assistantState.providerTypes = []
  }

  await Promise.all([
    applyProviderSettingsToPane('panel'),
    ...allWorkspacePaneSlots().map((id) => applyProviderSettingsToPane(id)),
  ])
}

export function setPaneProvider(paneId, providerId) {
  const pane = getPane(paneId)
  const provider = assistantState.providers.find((p) => p.id === providerId) || null
  pane.selectedProviderId = provider?.id || ''
  pane.selectedModel = providerDefaultModel(provider, assistantState.providerTypes)
  pane.thinkingEnabled = providerSupportsThinking(provider)
  pane.webSearchEnabled = providerSupportsWebSearch(provider)
  void refreshPaneProviderModels(paneId, provider?.id)
  writePaneSettings(paneId)
}

/** @deprecated use setPaneProvider(paneId, ...) */
export function setAssistantProvider(providerId, paneId = 'panel') {
  setPaneProvider(paneId, providerId)
}

export async function refreshPaneProviderModels(paneId, providerId) {
  const pane = getPane(paneId)
  const pid = providerId || pane.selectedProviderId
  pane.remoteModelList = []
  pane.modelsWarning = ''
  if (!pid || !assistantState.projectId) return
  try {
    const res = await listAIProviderModels(pid)
    const ids = Array.isArray(res?.models) ? res.models.map((m) => String(m?.id || '').trim()).filter(Boolean) : []
    pane.remoteModelList = ids
    pane.modelsWarning = res?.warning ? String(res.warning) : ''
  } catch {
    pane.remoteModelList = []
  }
}

/** @deprecated use refreshPaneProviderModels */
export async function refreshProviderModels(providerId, paneId = 'panel') {
  await refreshPaneProviderModels(paneId, providerId)
}

export function setPaneModel(paneId, model) {
  const pane = getPane(paneId)
  pane.selectedModel = String(model || '').trim()
  writePaneSettings(paneId)
}

export function setAssistantModel(model, paneId = 'panel') {
  setPaneModel(paneId, model)
}

export function setPaneThinking(paneId, enabled) {
  const pane = getPane(paneId)
  pane.thinkingEnabled = !!enabled
  writePaneSettings(paneId)
}

export function setAssistantThinking(enabled, paneId = 'panel') {
  setPaneThinking(paneId, enabled)
}

export function setPaneWebSearch(paneId, enabled) {
  const pane = getPane(paneId)
  pane.webSearchEnabled = !!enabled
  writePaneSettings(paneId)
}

export function setAssistantWebSearch(enabled, paneId = 'panel') {
  setPaneWebSearch(paneId, enabled)
}

export function setPaneDebug(paneId, enabled) {
  const pane = getPane(paneId)
  pane.debugEnabled = !!enabled
  writePaneSettings(paneId)
}

export function setAssistantDebug(enabled, paneId = 'panel') {
  setPaneDebug(paneId, enabled)
}

export function bindPage(snapshot) {
  if (snapshot && typeof snapshot === 'object' && Object.keys(snapshot).length > 0) {
    assistantState.pageContext = { ...snapshot }
  } else {
    assistantState.pageContext = null
  }
}

export function enrichPage(patch) {
  if (!patch || typeof patch !== 'object') return
  const base = assistantState.pageContext || {}
  assistantState.pageContext = { ...base, ...patch }
}

export function clearPage() {
  assistantState.pageContext = null
}

export function openAssistant() {
  assistantState.open = true
}

export function closeAssistant() {
  assistantState.open = false
}

export function toggleAssistant() {
  assistantState.open = !assistantState.open
}

export async function refreshSessions() {
  if (!assistantState.projectId) return
  try {
    const res = await listAISessions(assistantState.projectId, { limit: 20, offset: 0 })
    if (Array.isArray(res)) {
      // 兼容旧接口直接返回数组
      assistantState.sessions = res
      assistantState.hasMoreSessions = false
    } else {
      assistantState.sessions = Array.isArray(res?.sessions) ? res.sessions : []
      assistantState.hasMoreSessions = !!res?.hasMore
    }
  } catch {
    assistantState.sessions = []
    assistantState.hasMoreSessions = false
  }
}

export async function loadMoreSessions() {
  if (!assistantState.projectId || assistantState.loadingMoreSessions || !assistantState.hasMoreSessions) return
  assistantState.loadingMoreSessions = true
  try {
    const res = await listAISessions(assistantState.projectId, { limit: 20, offset: assistantState.sessions.length })
    if (Array.isArray(res)) {
      assistantState.sessions = [...assistantState.sessions, ...res]
      assistantState.hasMoreSessions = false
    } else {
      const more = Array.isArray(res?.sessions) ? res.sessions : []
      assistantState.sessions = [...assistantState.sessions, ...more]
      assistantState.hasMoreSessions = !!res?.hasMore
    }
  } finally {
    assistantState.loadingMoreSessions = false
  }
}

/** Conversation turns visible in UI (excludes system messages). */
function conversationMessageCount(pane) {
  return pane.messages.filter((m) => m.role !== 'system').length
}

/** True when the pane already has an active session with no user/assistant/tool turns. */
export function isPaneSessionEmpty(paneId = 'panel') {
  const pane = getPane(paneId)
  if (!pane.activeSessionId || pane.streaming) return false
  if (pane.pendingCalls.length > 0) return false
  return conversationMessageCount(pane) === 0
}

export async function newSession(title = '', paneId = 'panel') {
  if (!assistantState.projectId) return null
  if (isPaneSessionEmpty(paneId)) {
    const pane = getPane(paneId)
    return assistantState.sessions.find((s) => s.id === pane.activeSessionId) || { id: pane.activeSessionId }
  }
  const created = await createAISession(assistantState.projectId, { title })
  assistantState.sessions = [created, ...assistantState.sessions]
  await selectSession(created.id, paneId)
  return created
}

export async function selectSession(sessionId, paneId = 'panel', options = {}) {
  const pane = getPane(paneId)
  if (!sessionId) return
  if (sessionId === pane.activeSessionId) {
    writeActiveId(assistantState.projectId, paneId, sessionId)
    return
  }
  if (!options.restoreOnly) {
    cancelStreamingPane(pane)
  }
  pane.activeSessionId = sessionId
  writeActiveId(assistantState.projectId, paneId, sessionId)
  await refreshPaneSession(paneId)
}

export async function refreshPaneSession(paneId = 'panel') {
  const pane = getPane(paneId)
  if (!assistantState.projectId || !pane.activeSessionId) {
    pane.messages = []
    pane.pendingCalls = []
    return
  }
  const payload = await getAISession(assistantState.projectId, pane.activeSessionId)
  pane.messages = Array.isArray(payload?.messages) ? payload.messages : []
  rebuildPendingFromMessages(pane)
}

export async function renameSession(sessionId, title) {
  if (!assistantState.projectId || !sessionId) return
  const updated = await renameAISession(assistantState.projectId, sessionId, { title })
  assistantState.sessions = assistantState.sessions.map((s) => (s.id === sessionId ? updated : s))
}

export async function removeSession(sessionId) {
  if (!assistantState.projectId || !sessionId) return
  await deleteAISession(assistantState.projectId, sessionId)
  assistantState.sessions = assistantState.sessions.filter((s) => s.id !== sessionId)
  for (const paneId of PANE_IDS) {
    const pane = getPane(paneId)
    if (pane.activeSessionId === sessionId) {
      pane.activeSessionId = ''
      pane.messages = []
      pane.pendingCalls = []
      writeActiveId(assistantState.projectId, paneId, '')
    }
  }
}

export async function sendMessage(text, paneId = 'panel', images = []) {
  const message = String(text || '').trim()
  const imgs = (Array.isArray(images) ? images : [])
    .filter((item) => item && String(item.url || '').trim())
    .map((item) => ({ url: String(item.url).trim(), name: String(item.name || '').trim() || undefined }))
  const pane = getPane(paneId)
  if ((!message && imgs.length === 0) || !assistantState.projectId) return
  if (pane.streaming) return

  if (!pane.activeSessionId) {
    await newSession('', paneId)
  }
  if (!pane.activeSessionId) return

  const optimisticId = `optimistic-user-${Date.now()}`
  if (message || imgs.length) {
    pane.messages = [
      ...pane.messages.filter((m) => !m.optimistic),
      {
        id: optimisticId,
        seq: Number.MAX_SAFE_INTEGER - 2,
        role: 'user',
        content: message,
        attachments: imgs.length ? imgs : undefined,
        status: 'final',
        optimistic: true,
      },
    ]
  }

  await streamWith(
    `/projects/${assistantState.projectId}/ai/chat/stream`,
    {
      sessionId: pane.activeSessionId,
      providerId: pane.selectedProviderId || undefined,
      model: pane.selectedModel || undefined,
      thinkingEnabled: pane.thinkingEnabled,
      reasoningEffort: pane.thinkingEnabled ? 'high' : undefined,
      webSearchEnabled: pane.webSearchEnabled,
      debugEnabled: pane.debugEnabled,
      userMessage: message,
      images: imgs.length ? imgs : undefined,
      pageContext: assistantState.pageContext || undefined,
    },
    paneId
  )
}

export function clearGenJobProgress(paneId = 'panel') {
  const pane = getPane(paneId)
  pane.genJobProgress = null
}

function normalizeGenJobProgress(raw) {
  if (!raw || typeof raw !== 'object') return null
  const src = raw.genJob || raw.gen_job || raw.progress || raw
  if (!src || typeof src !== 'object') return null
  return {
    jobId: src.jobId || src.job_id || src.id || '',
    status: src.status || 'running',
    round: src.round ?? src.currentRound ?? src.current_round,
    maxRounds: src.maxRounds ?? src.max_rounds,
    passRate: src.passRate ?? src.pass_rate,
    totalScenarios: src.totalScenarios ?? src.total_scenarios,
    passedScenarios: src.passedScenarios ?? src.passed_scenarios,
    repairSummary: src.repairSummary ?? src.repair_summary ?? src.summary ?? '',
    repairDetails: src.repairDetails ?? src.repair_details ?? src.details,
    phase: src.phase || src.message || '',
  }
}

export async function resolvePendingCall(callId, decision, paneId = 'panel') {
  const pane = getPane(paneId)
  if (!assistantState.projectId || !pane.activeSessionId) return
  if (pane.streaming) return
  if (!callId || inFlightCallIds.has(`${paneId}:${callId}`)) return
  inFlightCallIds.add(`${paneId}:${callId}`)
  try {
    const body = {
      sessionId: pane.activeSessionId,
      approve: !!decision?.approve,
      reason: decision?.reason || '',
      providerId: pane.selectedProviderId || undefined,
      model: pane.selectedModel || undefined,
      thinkingEnabled: pane.thinkingEnabled,
      reasoningEffort: pane.thinkingEnabled ? 'high' : undefined,
      webSearchEnabled: pane.webSearchEnabled,
      debugEnabled: pane.debugEnabled,
      pageContext: assistantState.pageContext || undefined,
    }
    if (decision?.argumentOverrides && typeof decision.argumentOverrides === 'object') {
      body.toolArguments = decision.argumentOverrides
    }
    await streamWith(
      `/projects/${assistantState.projectId}/ai/tool-calls/${callId}/confirm`,
      body,
      paneId
    )
  } finally {
    inFlightCallIds.delete(`${paneId}:${callId}`)
  }
}

function pickInitialModel(provider, settings) {
  if (!provider) return ''
  if (settings?.providerId === provider.id && typeof settings.model === 'string' && settings.model.trim()) {
    return settings.model.trim()
  }
  return providerDefaultModel(provider, assistantState.providerTypes)
}

function pickInitialThinking(provider, settings) {
  if (!providerSupportsThinking(provider)) return false
  if (settings?.providerId === provider.id && typeof settings.thinkingEnabled === 'boolean') {
    return settings.thinkingEnabled
  }
  return true
}

function pickInitialWebSearch(provider, settings) {
  if (!providerSupportsWebSearch(provider)) return false
  if (settings?.providerId === provider.id && typeof settings.webSearchEnabled === 'boolean') {
    return settings.webSearchEnabled
  }
  return false
}

function pickInitialDebug(settings) {
  return !!settings?.debugEnabled
}

function rebuildPendingFromMessages(pane) {
  const pending = []
  for (const msg of pane.messages) {
    if (msg.status !== 'pending_confirm') continue
    const calls = parseCalls(msg.toolCalls)
    for (const call of calls) {
      if (call.requiresConfirm ?? call.mutating) pending.push(call)
    }
  }
  pane.pendingCalls = pending
}

function parseCalls(raw) {
  if (!raw) return []
  if (Array.isArray(raw)) return raw
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

/** Parse tool_calls JSON from an assistant message (for inline confirm UI). */
export function parseAssistantToolCalls(raw) {
  return parseCalls(raw)
}

function isPlaceholderSessionTitle(title) {
  const t = String(title || '').trim()
  if (!t) return true
  return ['新会话', '新对话', '未命名会话', '未命名'].includes(t)
}

async function streamWith(url, body, paneId = 'panel') {
  const pane = getPane(paneId)
  const controller = new AbortController()
  pane.abortController = controller
  pane.streaming = true
  pane.error = ''
  const requestSeq = ++pane.requestSeq
  const showThinking = !!body?.thinkingEnabled

  let placeholderId = `streaming-${paneId}-${requestSeq}`
  let placeholder = null

  const setPlaceholderThinking = (target, active, elapsedMillis = 0) => {
    if (!target) return
    if (active) {
      target.thinking = true
      if (!target.thinkingStartedAt) target.thinkingStartedAt = Date.now()
      return
    }
    if (target.thinking) {
      target.thinkingElapsedMillis = Number(elapsedMillis) || Math.max(0, Date.now() - (target.thinkingStartedAt || Date.now()))
    }
    target.thinking = false
  }

  const ensurePlaceholder = (thinking = false) => {
    if (placeholder) {
      if (thinking) setPlaceholderThinking(placeholder, true)
      return placeholder
    }
    placeholder = {
      id: placeholderId,
      seq: -1,
      role: 'assistant',
      content: '',
      status: 'final',
      streaming: true,
      thinking: false,
      thinkingStartedAt: 0,
      thinkingElapsedMillis: 0,
      thinkingContent: '',
    }
    if (thinking) setPlaceholderThinking(placeholder, true)
    pane.messages = [...pane.messages, placeholder]
    return placeholder
  }

  ensurePlaceholder(showThinking)

  try {
    await streamPostSSE(
      url,
      body,
      (event) => {
        if (requestSeq !== pane.requestSeq) return
        switch (event.kind) {
          case 'message':
            handleMessage(pane, event.message, placeholderId)
            placeholder = null
            break
          case 'thinking': {
            const p = ensurePlaceholder()
            setPlaceholderThinking(p, !!event.thinking?.active, event.thinking?.elapsedMillis)
            if (event.thinking?.content) {
              p.thinkingContent = (p.thinkingContent || '') + event.thinking.content
            }
            pane.messages = [...pane.messages]
            break
          }
          case 'usage': {
            if (!pane.debugEnabled || !event.usage) break
            const p = ensurePlaceholder()
            p.usageDetails = normalizeUsageDetails(event.usage)
            pane.messages = [...pane.messages]
            break
          }
          case 'text': {
            const p = ensurePlaceholder()
            setPlaceholderThinking(p, false)
            p.content += event.text || ''
            pane.messages = [...pane.messages]
            break
          }
          case 'tool_call':
            break
          case 'tool_result':
            break
          case 'pending_confirm':
            if (event.toolCall) {
              const exists = pane.pendingCalls.some((c) => c.id === event.toolCall.id)
              if (!exists) pane.pendingCalls = [...pane.pendingCalls, event.toolCall]
            }
            break
          case 'gen_job_progress': {
            const progress = normalizeGenJobProgress(event)
            if (progress) {
              pane.genJobProgress = progress
            }
            break
          }
          case 'error':
            pane.error = event.error || '对话发生未知错误'
            break
          case 'session': {
            const sid = String(event.session?.id || '')
            if (!sid) break
            const patch = { ...event.session, id: sid }
            const idx = assistantState.sessions.findIndex((s) => String(s.id) === sid)
            if (idx >= 0) {
              const next = [...assistantState.sessions]
              next[idx] = { ...next[idx], ...patch }
              assistantState.sessions = next
            } else {
              assistantState.sessions = [patch, ...assistantState.sessions]
            }
            break
          }
          case 'done':
            if (placeholder) {
              setPlaceholderThinking(placeholder, false)
              pane.messages = [...pane.messages]
            }
            if (pane.genJobProgress) {
              const st = String(pane.genJobProgress.status || '').toLowerCase()
              if (st === 'running' || !st) {
                pane.genJobProgress = { ...pane.genJobProgress, status: 'completed' }
              }
            }
            break
        }
      },
      { signal: controller.signal }
    )
  } catch (err) {
    if (err?.name !== 'AbortError') {
      pane.error = err?.message || '对话请求失败'
    }
  } finally {
    if (requestSeq === pane.requestSeq) {
      if (placeholder) {
        pane.messages = pane.messages.filter((m) => m.id !== placeholderId)
      }
      pane.streaming = false
      pane.abortController = null
      if (pane.activeSessionId) {
        const sid = pane.activeSessionId
        const sess = assistantState.sessions.find((s) => s.id === sid)
        if (isPlaceholderSessionTitle(sess?.title)) {
          refreshSessions().catch(() => {})
        }
      }
    }
  }
}

function normalizeUsageDetails(raw) {
  if (!raw) return null
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(String(raw))
  } catch {
    return null
  }
}

function handleMessage(pane, msg, placeholderId) {
  if (!msg || !msg.id) return
  const prior = pane.messages.find((m) => m.id === placeholderId)
  const usageDetails = normalizeUsageDetails(msg.usageDetails) || prior?.usageDetails || null
  const thinkingContent = prior?.thinkingContent || msg.reasoningContent || ''
  const merged = { ...msg, usageDetails, thinkingContent }
  const filtered = pane.messages.filter(
    (m) => m.id !== placeholderId && m.id !== msg.id && !m.optimistic
  )
  filtered.push(merged)
  filtered.sort(compareMessages)
  pane.messages = filtered

  if (msg.role === 'tool' && msg.toolCallId) {
    pane.pendingCalls = pane.pendingCalls.filter((c) => c.id !== msg.toolCallId)
  }
  if (
    msg.role === 'assistant'
    && (msg.status === 'final' || msg.status === 'pending_confirm' || msg.status === 'rejected')
  ) {
    rebuildPendingFromMessages(pane)
  }
}

function compareMessages(a, b) {
  const sa = Number.isFinite(a.seq) ? a.seq : Number.MAX_SAFE_INTEGER
  const sb = Number.isFinite(b.seq) ? b.seq : Number.MAX_SAFE_INTEGER
  return sa - sb
}
