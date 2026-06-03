/**
 * AI 助理状态 Store
 * 替代 stores/aiAssistant.js (30KB)，使用 Pinia + TypeScript 重构
 *
 * 管理多窗格 AI 对话状态：
 * - panel: 浮动面板
 * - w0..w5: 工作区窗格（最多 6 个并行聊天）
 */
import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import type { UUID } from '@/types/common'
import type { AIMessage, PendingConfirmEvent, TokenUsage } from '@/types/ai'
import {
  createAISession,
  listAISessions,
  getAISession,
  deleteAISession,
  renameAISession,
  listAIProviders,
  listAIProviderModels,
} from '@/api/ai'
import { streamPostSSE } from '@/utils/aiStream'
import {
  providerDefaultModel,
  providerSupportsThinking,
  providerSupportsWebSearch,
} from '@/utils/aiProvider'

// ── 常量 ──────────────────────────────────────────────────────────
const SESSION_KEY = 'autotest.ai-assistant.sessionId'
const SETTINGS_KEY = 'autotest.ai-assistant.settings'
const SPLIT_MODE_KEY = 'autotest.ai-assistant.splitMode'
const FOCUSED_PANE_KEY = 'autotest.ai-assistant.focusedPane'
const WORKSPACE_PANES_KEY = 'autotest.ai-assistant.workspacePaneIds'

export const MAX_WORKSPACE_PANES = 6
export const WORKSPACE_PANE_PREFIX = 'w'

// ── 类型 ──────────────────────────────────────────────────────────
export type PaneId = 'panel' | `w${number}`

export interface PaneState {
  activeSessionId: string
  messages: AIMessage[]
  pendingCalls: PendingCall[]
  streaming: boolean
  error: string
  abortController: AbortController | null
  requestSeq: number
  selectedProviderId: string
  selectedModel: string
  thinkingEnabled: boolean
  webSearchEnabled: boolean
  debugEnabled: boolean
  remoteModelList: string[]
  modelsWarning: string
  genJobProgress: GenJobProgress | null
}

export interface PendingCall {
  toolCallId: string
  name: string
  arguments: Record<string, unknown>
  description: string
}

export interface GenJobProgress {
  jobId: string
  phase: string
  progress: number
  total: number
  message: string
}

export interface ProviderInfo {
  id: string
  name: string
  provider: string
  model: string
  isDefault: boolean
  enabled: boolean
}

// ── 辅助函数 ──────────────────────────────────────────────────────
function workspacePaneId(index: number): `w${number}` {
  return `w${index}`
}

function isWorkspacePaneId(paneId: string): boolean {
  return /^w\d+$/.test(paneId)
}

function allWorkspacePaneSlots(): `w${number}`[] {
  return Array.from({ length: MAX_WORKSPACE_PANES }, (_, i) => workspacePaneId(i))
}

export const PANE_IDS: PaneId[] = ['panel', ...allWorkspacePaneSlots()]

function createPaneState(): PaneState {
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

// ── Store ──────────────────────────────────────────────────────────
export const useAIStore = defineStore('ai', () => {
  // 窗格状态
  const panes = reactive<Record<PaneId, PaneState>>({
    panel: createPaneState(),
    ...Object.fromEntries(allWorkspacePaneSlots().map(id => [id, createPaneState()])) as Record<`w${number}`, PaneState>,
  })

  // 全局状态
  const focusedPane = ref<PaneId>(
    (localStorage.getItem(FOCUSED_PANE_KEY) as PaneId) || 'panel'
  )
  const splitMode = ref<1 | 2 | 3 | 6>(
    Number(localStorage.getItem(SPLIT_MODE_KEY)) as 1 | 2 | 3 | 6 || 1
  )
  const workspacePaneIds = ref<string[]>(
    JSON.parse(localStorage.getItem(WORKSPACE_PANES_KEY) || '["w0"]')
  )
  const sessions = ref<Array<{ id: string; title: string }>>([])
  const providers = ref<ProviderInfo[]>([])

  // 计算属性
  const activePane = computed(() => panes[focusedPane.value])

  const activeSessionId = computed({
    get: () => activePane.value.activeSessionId,
    set: (id: string) => { activePane.value.activeSessionId = id },
  })

  // ── 窗格管理 ──────────────────────────────────────────────────
  function getPane(paneId: PaneId): PaneState {
    return panes[paneId]
  }

  function setFocusedPane(paneId: PaneId) {
    focusedPane.value = paneId
    localStorage.setItem(FOCUSED_PANE_KEY, paneId)
  }

  function setSplitMode(mode: 1 | 2 | 3 | 6) {
    splitMode.value = mode
    localStorage.setItem(SPLIT_MODE_KEY, String(mode))
    // 确保工作区窗格数量足够
    while (workspacePaneIds.value.length < mode) {
      const nextIdx = workspacePaneIds.value.length
      const id = workspacePaneId(nextIdx)
      if (!workspacePaneIds.value.includes(id)) {
        workspacePaneIds.value.push(id)
      }
    }
    localStorage.setItem(WORKSPACE_PANES_KEY, JSON.stringify(workspacePaneIds.value))
  }

  function addWorkspacePane(): string | null {
    if (workspacePaneIds.value.length >= MAX_WORKSPACE_PANES) return null
    for (let i = 0; i < MAX_WORKSPACE_PANES; i++) {
      const id = workspacePaneId(i)
      if (!workspacePaneIds.value.includes(id)) {
        workspacePaneIds.value.push(id)
        localStorage.setItem(WORKSPACE_PANES_KEY, JSON.stringify(workspacePaneIds.value))
        return id
      }
    }
    return null
  }

  function removeWorkspacePane(paneId: string) {
    const idx = workspacePaneIds.value.indexOf(paneId)
    if (idx < 0) return
    workspacePaneIds.value.splice(idx, 1)
    localStorage.setItem(WORKSPACE_PANES_KEY, JSON.stringify(workspacePaneIds.value))
    // 如果删除的是当前聚焦的窗格，切换到第一个窗格
    if (focusedPane.value === paneId) {
      setFocusedPane(workspacePaneIds.value[0] as PaneId || 'panel')
    }
  }

  // ── 会话管理 ──────────────────────────────────────────────────
  async function loadSessions(projectId: string) {
    try {
      sessions.value = await listAISessions(projectId)
    } catch {
      sessions.value = []
    }
  }

  async function createSession(projectId: string, paneId?: PaneId): Promise<string | null> {
    try {
      const session = await createAISession(projectId)
      const targetPane = paneId || focusedPane.value
      panes[targetPane].activeSessionId = session.id
      panes[targetPane].messages = []
      // 持久化
      saveSessionId(targetPane, session.id)
      return session.id
    } catch {
      return null
    }
  }

  async function switchSession(paneId: PaneId, sessionId: string, projectId: string) {
    panes[paneId].activeSessionId = sessionId
    panes[paneId].messages = []
    panes[paneId].error = ''
    panes[paneId].pendingCalls = []
    saveSessionId(paneId, sessionId)
    // 加载历史消息
    try {
      const session = await getAISession(projectId, sessionId) as { messages?: AIMessage[] }
      if (session?.messages) {
        panes[paneId].messages = session.messages
      }
    } catch { /* ignore */ }
  }

  async function removeSession(projectId: string, sessionId: string) {
    try {
      await deleteAISession(projectId, sessionId)
      // 清理所有引用该会话的窗格
      for (const paneId of PANE_IDS) {
        if (panes[paneId].activeSessionId === sessionId) {
          panes[paneId].activeSessionId = ''
          panes[paneId].messages = []
          saveSessionId(paneId, '')
        }
      }
    } catch { /* ignore */ }
  }

  // ── 消息发送 ──────────────────────────────────────────────────
  async function sendMessage(paneId: PaneId, content: string) {
    const pane = panes[paneId]
    if (pane.streaming || !content.trim()) return

    // 如果没有活跃会话，先创建
    if (!pane.activeSessionId) {
      // 需要 projectId，从 projectStore 获取
      // 这里通过 import 避免循环依赖
      const { useProjectStore } = await import('./project')
      const projectStore = useProjectStore()
      if (!projectStore.currentProjectId) return
      const sessionId = await createSession(projectStore.currentProjectId, paneId)
      if (!sessionId) return
    }

    // 添加用户消息
    const userMessage: AIMessage = {
      id: crypto.randomUUID(),
      sessionId: pane.activeSessionId,
      role: 'user',
      content,
      createdAt: new Date().toISOString(),
    }
    pane.messages.push(userMessage)

    // 开始流式请求
    pane.streaming = true
    pane.error = ''
    pane.abortController = new AbortController()
    pane.requestSeq++

    const currentSeq = pane.requestSeq
    let assistantContent = ''
    let thinkingContent = ''

    try {
      await streamPostSSE(
        `/api/ai/sessions/${pane.activeSessionId}/chat`,
        { message: content },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (event: any) => {
          if (pane.requestSeq !== currentSeq) return

          switch (event.kind) {
            case 'text':
              assistantContent += event.text || ''
              updateAssistantMessage(paneId, assistantContent, thinkingContent)
              break
            case 'thinking':
              thinkingContent += event.text || ''
              updateAssistantMessage(paneId, assistantContent, thinkingContent)
              break
            case 'tool_call':
            case 'tool_result':
              break
            case 'pending_confirm':
              if (event.toolCall) {
                pane.pendingCalls.push({
                  toolCallId: event.toolCall.toolCallId,
                  name: event.toolCall.name,
                  arguments: event.toolCall.arguments,
                  description: event.toolCall.description || '',
                })
              }
              break
            case 'done':
              finalizeAssistantMessage(paneId, assistantContent, thinkingContent, event.tokenUsage)
              break
            case 'error':
              pane.error = event.error || 'Unknown error'
              break
          }
        },
        { signal: pane.abortController.signal }
      )
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== 'AbortError') {
        pane.error = err.message
      }
    } finally {
      pane.streaming = false
      pane.abortController = null
    }
  }

  function updateAssistantMessage(paneId: PaneId, content: string, thinking: string) {
    const pane = panes[paneId]
    const lastMsg = pane.messages[pane.messages.length - 1]
    if (lastMsg && lastMsg.role === 'assistant' && !lastMsg.toolResult) {
      lastMsg.content = content
      lastMsg.thinking = thinking || undefined
    } else {
      pane.messages.push({
        id: crypto.randomUUID(),
        sessionId: pane.activeSessionId,
        role: 'assistant',
        content,
        thinking: thinking || undefined,
        createdAt: new Date().toISOString(),
      })
    }
  }

  function finalizeAssistantMessage(paneId: PaneId, content: string, thinking: string, tokenUsage?: TokenUsage) {
    const pane = panes[paneId]
    const lastMsg = pane.messages[pane.messages.length - 1]
    if (lastMsg && lastMsg.role === 'assistant') {
      lastMsg.content = content
      lastMsg.thinking = thinking || undefined
      lastMsg.tokenUsage = tokenUsage
    }
  }

  // ── 工具调用确认 ──────────────────────────────────────────────
  async function confirmToolCall(paneId: PaneId, toolCallId: string, approved: boolean) {
    const pane = panes[paneId]
    const idx = pane.pendingCalls.findIndex(c => c.toolCallId === toolCallId)
    if (idx < 0) return

    pane.pendingCalls.splice(idx, 1)

    try {
      await fetch(`/api/ai/tool-calls/${toolCallId}/confirm`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('autotest_admin_token')}`,
        },
        body: JSON.stringify({ approve: approved }),
      })
    } catch { /* ignore */ }
  }

  // ── 中止请求 ──────────────────────────────────────────────────
  function abortStream(paneId: PaneId) {
    const pane = panes[paneId]
    if (pane.abortController) {
      pane.abortController.abort()
      pane.streaming = false
      pane.abortController = null
    }
  }

  // ── Provider 管理 ──────────────────────────────────────────────
  async function loadProviders() {
    try {
      providers.value = await listAIProviders()
    } catch {
      providers.value = []
    }
  }

  function setProvider(paneId: PaneId, providerId: string) {
    panes[paneId].selectedProviderId = providerId
    // 找到 provider 的默认模型
    const provider = providers.value.find(p => p.id === providerId)
    if (provider) {
      panes[paneId].selectedModel = provider.model
    }
  }

  function setModel(paneId: PaneId, model: string) {
    panes[paneId].selectedModel = model
  }

  // ── 持久化 ──────────────────────────────────────────────────────
  function saveSessionId(paneId: PaneId, sessionId: string) {
    try {
      const key = `${SESSION_KEY}.${paneId}`
      if (sessionId) localStorage.setItem(key, sessionId)
      else localStorage.removeItem(key)
    } catch { /* ignore */ }
  }

  function loadSessionId(paneId: PaneId): string {
    try { return localStorage.getItem(`${SESSION_KEY}.${paneId}`) || '' } catch { return '' }
  }

  // ── 初始化 ──────────────────────────────────────────────────────
  function initPane(paneId: PaneId) {
    const savedId = loadSessionId(paneId)
    if (savedId) {
      panes[paneId].activeSessionId = savedId
    }
  }

  // 初始化所有窗格
  for (const paneId of PANE_IDS) {
    initPane(paneId)
  }

  return {
    // 状态
    panes,
    focusedPane,
    splitMode,
    workspacePaneIds,
    sessions,
    providers,

    // 计算属性
    activePane,
    activeSessionId,

    // 窗格管理
    getPane,
    setFocusedPane,
    setSplitMode,
    addWorkspacePane,
    removeWorkspacePane,

    // 会话管理
    loadSessions,
    createSession,
    switchSession,
    removeSession,

    // 消息
    sendMessage,
    abortStream,

    // 工具调用确认
    confirmToolCall,

    // Provider
    loadProviders,
    setProvider,
    setModel,
  }
})
