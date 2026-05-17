// Global AI assistant store
// =========================
//
// Conversation state is split into panes so the workspace page can run two
// chats side-by-side while the floating panel keeps its own pane (`panel`).

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

const STORAGE_KEY = 'autotest.ai-assistant.sessionId'
const SETTINGS_KEY = 'autotest.ai-assistant.settings'
const SPLIT_MODE_KEY = 'autotest.ai-assistant.splitMode'
const FOCUSED_PANE_KEY = 'autotest.ai-assistant.focusedPane'

export const PANE_IDS = ['panel', 'left', 'right']
export const WORKSPACE_PANE_IDS = ['left', 'right']

function createPaneState() {
  return {
    activeSessionId: '',
    messages: [],
    pendingCalls: [],
    streaming: false,
    error: '',
    abortController: null,
    requestSeq: 0,
  }
}

function readActiveId(projectId, paneId = 'panel') {
  if (!projectId) return ''
  try {
    const legacy = window.localStorage.getItem(`${STORAGE_KEY}:${projectId}`)
    const key = `${STORAGE_KEY}:${projectId}:${paneId}`
    const raw = window.localStorage.getItem(key) || (paneId === 'panel' ? legacy : '')
    return raw || ''
  } catch {
    return ''
  }
}

function writeActiveId(projectId, paneId, id) {
  if (!projectId) return
  try {
    const key = `${STORAGE_KEY}:${projectId}:${paneId}`
    if (id) window.localStorage.setItem(key, id)
    else window.localStorage.removeItem(key)
    if (paneId === 'panel') {
      const legacyKey = `${STORAGE_KEY}:${projectId}`
      if (id) window.localStorage.setItem(legacyKey, id)
      else window.localStorage.removeItem(legacyKey)
    }
  } catch {
    // localStorage may be blocked
  }
}

function readSettings(projectId) {
  if (!projectId) return {}
  try {
    const raw = window.localStorage.getItem(`${SETTINGS_KEY}:${projectId}`)
    return raw ? JSON.parse(raw) || {} : {}
  } catch {
    return {}
  }
}

function writeSettings() {
  if (!assistantState.projectId) return
  try {
    window.localStorage.setItem(`${SETTINGS_KEY}:${assistantState.projectId}`, JSON.stringify({
      providerId: assistantState.selectedProviderId || '',
      model: assistantState.selectedModel || '',
      thinkingEnabled: !!assistantState.thinkingEnabled,
      webSearchEnabled: !!assistantState.webSearchEnabled,
      debugEnabled: !!assistantState.debugEnabled,
    }))
  } catch {
    // ignore
  }
}

function readSplitMode() {
  try {
    return window.localStorage.getItem(SPLIT_MODE_KEY) === '1'
  } catch {
    return false
  }
}

function writeSplitMode(value) {
  try {
    window.localStorage.setItem(SPLIT_MODE_KEY, value ? '1' : '0')
  } catch {
    // ignore
  }
}

function readFocusedPane() {
  try {
    const raw = window.localStorage.getItem(FOCUSED_PANE_KEY)
    return raw === 'right' ? 'right' : 'left'
  } catch {
    return 'left'
  }
}

function writeFocusedPane(paneId) {
  try {
    window.localStorage.setItem(FOCUSED_PANE_KEY, paneId === 'right' ? 'right' : 'left')
  } catch {
    // ignore
  }
}

export const assistantState = reactive({
  open: false,
  projectId: '',
  sessions: [],
  providers: [],
  providerTypes: [],
  remoteModelList: [],
  modelsWarning: '',
  selectedProviderId: '',
  selectedModel: '',
  thinkingEnabled: true,
  webSearchEnabled: false,
  debugEnabled: false,
  pageContext: null,
  splitMode: readSplitMode(),
  focusedPaneId: readFocusedPane(),
  panes: {
    panel: createPaneState(),
    left: createPaneState(),
    right: createPaneState(),
  },
})

const inFlightCallIds = new Set()

export function getPane(paneId = 'panel') {
  return assistantState.panes[paneId] || assistantState.panes.panel
}

export function anyPaneStreaming(paneIds = PANE_IDS) {
  return paneIds.some((id) => getPane(id).streaming)
}

export function paneSessionIds(paneIds = WORKSPACE_PANE_IDS) {
  return paneIds.map((id) => getPane(id).activeSessionId).filter(Boolean)
}

export function isSessionOpenInPane(sessionId, paneId) {
  return !!sessionId && getPane(paneId).activeSessionId === sessionId
}

export function setSplitMode(enabled) {
  assistantState.splitMode = !!enabled
  writeSplitMode(assistantState.splitMode)
  if (assistantState.splitMode && assistantState.focusedPaneId !== 'left' && assistantState.focusedPaneId !== 'right') {
    setFocusedPane('left')
  }
}

export function setFocusedPane(paneId) {
  if (paneId !== 'left' && paneId !== 'right') return
  assistantState.focusedPaneId = paneId
  writeFocusedPane(paneId)
}

function resetPane(pane) {
  cancelStreamingPane(pane)
  pane.activeSessionId = ''
  pane.messages = []
  pane.pendingCalls = []
  pane.error = ''
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
  assistantState.selectedProviderId = ''
  assistantState.selectedModel = ''
  assistantState.thinkingEnabled = true
  assistantState.webSearchEnabled = false
  assistantState.debugEnabled = false
  if (!projectId) return
  await refreshAssistantProviders()
  await refreshSessions()
  for (const paneId of PANE_IDS) {
    const desired = readActiveId(projectId, paneId)
    if (desired && assistantState.sessions.some((s) => s.id === desired)) {
      await selectSession(desired, paneId, { restoreOnly: true })
    }
  }
}

export async function refreshAssistantProviders() {
  if (!assistantState.projectId) return
  const settings = readSettings(assistantState.projectId)
  try {
    const [providers, providerTypes] = await Promise.all([
      listAIProviders(assistantState.projectId),
      listAIProviderTypes(),
    ])
    assistantState.providers = Array.isArray(providers) ? providers : []
    assistantState.providerTypes = Array.isArray(providerTypes) ? providerTypes : []
  } catch {
    assistantState.providers = []
    assistantState.providerTypes = []
  }

  const available = assistantState.providers.filter((p) => p.enabled !== false)
  const savedProvider = available.find((p) => p.id === settings.providerId)
  const fallbackProvider = available.find((p) => p.isDefault) || available[0] || null
  const provider = savedProvider || fallbackProvider
  assistantState.selectedProviderId = provider?.id || ''
  assistantState.selectedModel = pickInitialModel(provider, settings)
  assistantState.thinkingEnabled = pickInitialThinking(provider, settings)
  assistantState.webSearchEnabled = pickInitialWebSearch(provider, settings)
  assistantState.debugEnabled = pickInitialDebug(settings)
  await refreshProviderModels(provider?.id)
  writeSettings()
}

export function setAssistantProvider(providerId) {
  const provider = assistantState.providers.find((p) => p.id === providerId) || null
  assistantState.selectedProviderId = provider?.id || ''
  assistantState.selectedModel = providerDefaultModel(provider)
  assistantState.thinkingEnabled = providerSupportsThinking(provider)
  assistantState.webSearchEnabled = providerSupportsWebSearch(provider)
  void refreshProviderModels(provider?.id)
  writeSettings()
}

export async function refreshProviderModels(providerId) {
  assistantState.remoteModelList = []
  assistantState.modelsWarning = ''
  if (!providerId || !assistantState.projectId) return
  try {
    const res = await listAIProviderModels(assistantState.projectId, providerId)
    const ids = Array.isArray(res?.models) ? res.models.map((m) => String(m?.id || '').trim()).filter(Boolean) : []
    assistantState.remoteModelList = ids
    assistantState.modelsWarning = res?.warning ? String(res.warning) : ''
  } catch {
    assistantState.remoteModelList = []
  }
}

export function setAssistantModel(model) {
  assistantState.selectedModel = String(model || '').trim()
  writeSettings()
}

export function setAssistantThinking(enabled) {
  assistantState.thinkingEnabled = !!enabled
  writeSettings()
}

export function setAssistantWebSearch(enabled) {
  assistantState.webSearchEnabled = !!enabled
  writeSettings()
}

export function setAssistantDebug(enabled) {
  assistantState.debugEnabled = !!enabled
  writeSettings()
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
    const items = await listAISessions(assistantState.projectId)
    assistantState.sessions = Array.isArray(items) ? items : []
  } catch {
    assistantState.sessions = []
  }
}

export async function newSession(title = '', paneId = 'panel') {
  if (!assistantState.projectId) return null
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

  await streamWith(
    `/projects/${assistantState.projectId}/ai/chat/stream`,
    {
      sessionId: pane.activeSessionId,
      providerId: assistantState.selectedProviderId || undefined,
      model: assistantState.selectedModel || undefined,
      thinkingEnabled: assistantState.thinkingEnabled,
      reasoningEffort: assistantState.thinkingEnabled ? 'high' : undefined,
      webSearchEnabled: assistantState.webSearchEnabled,
      debugEnabled: assistantState.debugEnabled,
      userMessage: message,
      images: imgs.length ? imgs : undefined,
      pageContext: assistantState.pageContext || undefined,
    },
    paneId
  )
}

export async function resolvePendingCall(callId, decision, paneId = 'panel') {
  const pane = getPane(paneId)
  if (!assistantState.projectId || !pane.activeSessionId) return
  if (pane.streaming) return
  if (!callId || inFlightCallIds.has(`${paneId}:${callId}`)) return
  inFlightCallIds.add(`${paneId}:${callId}`)
  pane.pendingCalls = pane.pendingCalls.filter((c) => c.id !== callId)
  try {
    await streamWith(
      `/projects/${assistantState.projectId}/ai/tool-calls/${callId}/confirm`,
      {
        sessionId: pane.activeSessionId,
        approve: !!decision?.approve,
        reason: decision?.reason || '',
        providerId: assistantState.selectedProviderId || undefined,
        model: assistantState.selectedModel || undefined,
        thinkingEnabled: assistantState.thinkingEnabled,
        reasoningEffort: assistantState.thinkingEnabled ? 'high' : undefined,
        webSearchEnabled: assistantState.webSearchEnabled,
        debugEnabled: assistantState.debugEnabled,
        pageContext: assistantState.pageContext || undefined,
      },
      paneId
    )
  } finally {
    inFlightCallIds.delete(`${paneId}:${callId}`)
  }
}

function providerMeta(provider) {
  if (!provider) return null
  return assistantState.providerTypes.find((t) => t.type === provider.providerType) || null
}

function providerDefaultModel(provider) {
  if (!provider) return ''
  return provider.defaultModel || providerMeta(provider)?.defaultModel || ''
}

function providerSupportsThinking(provider) {
  const t = String(provider?.providerType || '').toLowerCase()
  return t === 'deepseek' || t === 'xiaomi'
}

function providerSupportsWebSearch(provider) {
  return String(provider?.providerType || '').toLowerCase() === 'xiaomi'
}

function pickInitialModel(provider, settings) {
  if (!provider) return ''
  if (settings?.providerId === provider.id && typeof settings.model === 'string' && settings.model.trim()) {
    return settings.model.trim()
  }
  return providerDefaultModel(provider)
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
      if (call.mutating) pending.push(call)
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
  const selectedProvider = assistantState.providers.find((p) => p.id === assistantState.selectedProviderId) || null
  const showThinking = !!body?.thinkingEnabled && providerSupportsThinking(selectedProvider)

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
            pane.messages = [...pane.messages]
            break
          }
          case 'usage': {
            if (!assistantState.debugEnabled || !event.usage) break
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
  const merged = { ...msg, usageDetails }
  const filtered = pane.messages.filter((m) => m.id !== placeholderId && m.id !== msg.id)
  filtered.push(merged)
  filtered.sort(compareMessages)
  pane.messages = filtered

  if (msg.role === 'tool' && msg.toolCallId) {
    pane.pendingCalls = pane.pendingCalls.filter((c) => c.id !== msg.toolCallId)
  }
  if (msg.role === 'assistant' && msg.status === 'final') {
    rebuildPendingFromMessages(pane)
  }
}

function compareMessages(a, b) {
  const sa = Number.isFinite(a.seq) ? a.seq : Number.MAX_SAFE_INTEGER
  const sb = Number.isFinite(b.seq) ? b.seq : Number.MAX_SAFE_INTEGER
  return sa - sb
}
