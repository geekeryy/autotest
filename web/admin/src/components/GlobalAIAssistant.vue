<!--
  GlobalAIAssistant
  =================
  Right-side panel AI assistant, integrated into the AdminLayout.
  Supports drag-to-resize width, styled after Cursor's AI panel.

  State is fully owned by `stores/aiAssistant.js`. This component is
  responsible for:
    - Rendering the conversation timeline.
    - Capturing user input and forwarding to the store.
    - Surfacing pending mutating tool calls so the user can approve or
      reject them.
    - Drag handle for adjustable width.

  Visibility rules: show whenever a project is selected and panel is open.
-->
<template>
  <component
    :is="rootWrapper"
    v-bind="rootWrapperProps"
    class="ai-panel-host"
    :class="{
      'ai-panel-host--page': isPageLayout,
      'ai-panel-host--narrow': isPanelLayout && viewportNarrow,
    }"
    :style="panelHostStyle"
  >
    <div
      v-if="visible"
      class="ai-panel"
      :class="{ 'ai-panel--page': isPageLayout }"
    >
      <!-- Drag resize handle (panel only) -->
      <div
        v-if="isPanelLayout"
        class="ai-resize-handle"
        @mousedown="onResizeStart"
        :class="{ 'ai-resize-handle--active': resizing }"
      />

      <!-- Header：仅侧栏浮窗模式显示顶栏 -->
      <header v-if="isPanelLayout" class="ai-header">
        <div class="ai-header-left">
          <el-icon class="ai-header-icon"><ChatLineRound /></el-icon>
          <span class="ai-header-title">AI 助理</span>
        </div>
        <div class="ai-header-actions">
          <el-tooltip content="新建会话" placement="bottom">
            <button class="ai-icon-btn" @click="onNewSession">
              <el-icon><Plus /></el-icon>
            </button>
          </el-tooltip>
          <el-popover placement="bottom-end" :width="280" trigger="click" :popper-style="{ padding: '6px' }">
            <template #reference>
              <button class="ai-icon-btn" :title="'会话历史 (' + state.sessions.length + ')'">
                <el-icon><Operation /></el-icon>
              </button>
            </template>
            <div class="ai-history">
              <div v-if="!state.sessions.length" class="ai-history-empty">暂无历史会话</div>
              <SessionListItem
                v-for="session in state.sessions"
                :key="session.id"
                :session="session"
                variant="history"
                :active="session.id === state.activeSessionId"
                @select="() => onSelectSession(session.id)"
                @detail="onSessionDetail"
                @rename="onRenameSession"
                @delete="onDeleteSession"
              />
            </div>
          </el-popover>
          <ModelSettingsPopover
            pane-id="panel"
            placement="bottom-end"
            :busy="state.streaming || state.pendingCalls.length > 0"
          >
            <template #reference>
              <button
                type="button"
                class="ai-icon-btn"
                :disabled="state.streaming || state.pendingCalls.length > 0"
                :title="modelSettingsSummary"
              >
                <el-icon><Setting /></el-icon>
              </button>
            </template>
          </ModelSettingsPopover>
          <el-tooltip content="关闭" placement="bottom">
            <button class="ai-icon-btn" @click="close">
              <el-icon><Close /></el-icon>
            </button>
          </el-tooltip>
        </div>
      </header>

      <div
        v-if="isPageLayout && paneBarLabel"
        class="ai-pane-bar"
        :class="{ 'ai-pane-bar--focused': paneFocused }"
      >
        <span class="ai-pane-bar__title">{{ paneBarLabel }}</span>
        <div class="ai-pane-bar__actions">
          <el-tooltip content="关闭此屏" placement="bottom" :show-after="400">
            <button
              type="button"
              class="ai-pane-bar__btn ai-pane-bar__btn--close"
              :disabled="!canRemoveThisPane"
              aria-label="关闭此屏"
              @click="onRemoveThisPane"
            >
              <el-icon><Close /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip
            v-if="showAddPane"
            content="添加分屏"
            placement="bottom"
            :show-after="400"
          >
            <button
              type="button"
              class="ai-pane-bar__btn ai-pane-bar__btn--add"
              :disabled="!canAddPane"
              aria-label="添加分屏"
              @click="onAddPaneClick"
            >
              <el-icon><Plus /></el-icon>
            </button>
          </el-tooltip>
        </div>
      </div>
      <div
        v-else-if="isPageLayout && showAddPane"
        class="ai-pane-add-fallback"
      >
        <el-tooltip content="添加分屏" placement="bottom" :show-after="400">
          <button
            type="button"
            class="ai-pane-bar__btn ai-pane-bar__btn--add"
            :disabled="!canAddPane"
            aria-label="添加分屏"
            @click="onAddPaneClick"
          >
            <el-icon><Plus /></el-icon>
          </button>
        </el-tooltip>
      </div>

      <div v-if="pageContextHint && isPanelLayout" class="ai-page-context">
        <el-icon><Location /></el-icon>
        <span class="ai-page-context-label">当前上下文：</span>
        <span class="ai-page-context-value" :title="pageContextHint">{{ pageContextHint }}</span>
      </div>

      <div class="ai-chat-stage">
      <!-- Messages body（可滚动） -->
      <section class="ai-body" ref="bodyEl">
        <div
          v-if="visibleMessages.length === 0 && !state.streaming"
          class="ai-empty"
          :class="{ 'ai-empty--page': isPageLayout }"
        >
          <div class="ai-empty-icon" :class="{ 'ai-empty-icon--page': isPageLayout }">
            <el-icon :size="isPageLayout ? 56 : 48"><ChatLineRound /></el-icon>
          </div>
          <p class="ai-empty-title">{{ isPageLayout ? '今天有什么可以帮你？' : 'AI 助理' }}</p>
          <p class="ai-empty-desc">
            {{
              isPageLayout
                ? '我可以查询接口与场景、生成编排步骤、调整断言；写操作会先请你确认。'
                : '我可以帮你查询用例、场景、接口定义，并按需调整测试断言。'
            }}
          </p>
          <p v-if="!isPageLayout" class="ai-empty-hint">写工具调用会先暂停并请你确认</p>
          <div v-if="isPageLayout" class="ai-empty-prompts">
            <button
              v-for="prompt in pagePromptSuggestions"
              :key="prompt"
              type="button"
              class="ai-empty-prompt"
              @click="applyPrompt(prompt)"
            >
              {{ prompt }}
            </button>
          </div>
        </div>

        <template v-for="item in displayTimeline" :key="item.key">
          <div
            v-if="item.kind === 'toolGroup'"
            class="ai-tool-group"
            :class="{ 'ai-tool-group--expanded': isToolGroupExpanded(item.key) }"
          >
            <button
              type="button"
              class="ai-tool-group-head"
              :aria-expanded="isToolGroupExpanded(item.key)"
              @click="toggleToolGroup(item.key)"
            >
              <el-icon class="ai-tool-chevron" :class="{ 'ai-tool-chevron--expanded': isToolGroupExpanded(item.key) }">
                <ArrowRight />
              </el-icon>
              <span class="ai-tool-group-summary">{{ toolGroupSummary(item.messages) }}</span>
            </button>
            <div v-show="isToolGroupExpanded(item.key)" class="ai-tool-group-body">
              <div
                v-for="toolMsg in item.messages"
                :key="toolMsg.id"
                class="ai-tool-row"
                :class="{ 'ai-tool-row--expanded': isToolMsgExpanded(toolMsg.id) }"
              >
                <button
                  type="button"
                  class="ai-tool-row-head"
                  :aria-expanded="isToolMsgExpanded(toolMsg.id)"
                  @click="toggleToolMsg(toolMsg.id)"
                >
                  <el-icon class="ai-tool-chevron" :class="{ 'ai-tool-chevron--expanded': isToolMsgExpanded(toolMsg.id) }">
                    <ArrowRight />
                  </el-icon>
                  <span class="ai-tool-row-title">{{ toolMessageTitle(toolMsg) }}</span>
                </button>
                <pre v-show="isToolMsgExpanded(toolMsg.id)" class="ai-tool-row-body">{{ formatToolContent(toolMsg.content) }}</pre>
              </div>
            </div>
          </div>

          <div
            v-else-if="item.kind === 'toolSingle'"
            class="ai-tool-row ai-tool-row--standalone"
            :class="{ 'ai-tool-row--expanded': isToolMsgExpanded(item.msg.id) }"
          >
            <button
              type="button"
              class="ai-tool-row-head"
              :aria-expanded="isToolMsgExpanded(item.msg.id)"
              @click="toggleToolMsg(item.msg.id)"
            >
              <el-icon class="ai-tool-chevron" :class="{ 'ai-tool-chevron--expanded': isToolMsgExpanded(item.msg.id) }">
                <ArrowRight />
              </el-icon>
              <span class="ai-tool-row-title">{{ toolMessageTitle(item.msg) }}</span>
            </button>
            <pre v-show="isToolMsgExpanded(item.msg.id)" class="ai-tool-row-body">{{ formatToolContent(item.msg.content) }}</pre>
          </div>

          <div
            v-else
            class="ai-message"
            :class="['ai-message--' + item.msg.role, item.msg.status === 'pending_confirm' ? 'ai-message--pending' : '']"
          >
            <template v-if="item.msg.role === 'user'">
              <div class="ai-message-user-row">
                <div class="ai-user-bubble">
                  <div v-if="userMessageImages(item.msg).length" class="ai-user-images">
                    <img
                      v-for="(img, idx) in userMessageImages(item.msg)"
                      :key="idx"
                      class="ai-user-image"
                      :src="img.url"
                      :alt="img.name || '图片'"
                      loading="lazy"
                    />
                  </div>
                  <div v-if="item.msg.content" class="ai-text ai-text--user">{{ item.msg.content }}</div>
                </div>
                <el-avatar
                  class="ai-message-user-avatar"
                  :size="32"
                  :src="userAvatarSrc"
                  :style="userAvatarBgStyle"
                >
                  {{ userAvatarInitials }}
                </el-avatar>
              </div>
            </template>
            <template v-else>
              <div class="ai-message-header">
                <span class="ai-role" :class="'ai-role--' + item.msg.role">{{ roleLabel(item.msg.role) }}</span>
                <span v-if="item.msg.model" class="ai-model">{{ item.msg.model }}</span>
                <span v-if="item.msg.streaming" class="ai-streaming-indicator">
                  <span class="ai-streaming-cursor" />
                </span>
              </div>
              <div v-if="item.msg.role === 'assistant' && item.msg.thinking" class="ai-thinking-state">
                <span class="ai-thinking-orb" />
                <span class="ai-thinking-label">正在深度思考</span>
                <span v-if="formatThinkingElapsed(item.msg)" class="ai-thinking-elapsed">
                  {{ formatThinkingElapsed(item.msg) }}
                </span>
              </div>
              <div
                v-else-if="item.msg.role === 'assistant' && item.msg.streaming && !item.msg.content"
                class="ai-replying-state"
              >
                <span class="ai-streaming-dot" />
                <span class="ai-replying-label">AI 正在回复…</span>
              </div>
              <MarkdownView v-if="item.msg.role === 'assistant' && item.msg.content" :source="item.msg.content" />
              <AIUsageDebugPanel
                v-if="item.msg.role === 'assistant' && state.debugEnabled && item.msg.usageDetails"
                :usage="item.msg.usageDetails"
              />
            </template>
          </div>
        </template>


        <div v-if="state.pendingCalls.length" class="ai-pending-block">
          <div class="ai-pending-title">
            <el-icon><Warning /></el-icon>
            <span>{{ state.pendingCalls.length }} 项删除操作等待确认</span>
          </div>
          <AIToolCallConfirm
            v-for="call in state.pendingCalls"
            :key="call.id"
            :call="call"
            :disabled="state.streaming"
            @decide="onDecide"
          />
        </div>

        <div v-if="state.error" class="ai-error">
          <el-icon><CircleClose /></el-icon>
          <span>{{ state.error }}</span>
        </div>
      </section>

      <!-- Composer -->
      <footer class="ai-composer">
        <div
          class="ai-composer-box"
          :class="{ 'ai-composer-box--with-toolbar': composerHasToolbar }"
        >
          <div
            v-if="composerHasToolbar"
            class="ai-composer-toolbar"
          >
            <el-tooltip v-if="supportsThinking" :content="thinkingTooltip" placement="top">
              <button
                type="button"
                class="ai-composer-toggle-btn"
                :class="{ 'ai-composer-toggle-btn--active': state.thinkingEnabled }"
                :disabled="state.streaming || state.pendingCalls.length > 0"
                :aria-pressed="state.thinkingEnabled"
                aria-label="深度思考"
                @click="onToggleThinking"
              >
                <el-icon><Cpu /></el-icon>
              </button>
            </el-tooltip>
            <el-tooltip v-if="supportsWebSearch" :content="webSearchTooltip" placement="top">
              <button
                type="button"
                class="ai-composer-toggle-btn"
                :class="{ 'ai-composer-toggle-btn--active': state.webSearchEnabled }"
                :disabled="state.streaming || state.pendingCalls.length > 0"
                :aria-pressed="state.webSearchEnabled"
                aria-label="联网搜索"
                @click="onToggleWebSearch"
              >
                <el-icon><Search /></el-icon>
              </button>
            </el-tooltip>
            <el-tooltip
              v-if="supportsMultimodal"
              :content="multimodalTooltip"
              placement="top"
            >
              <button
                type="button"
                class="ai-composer-toggle-btn"
                :disabled="state.streaming || state.pendingCalls.length > 0"
                aria-label="添加图片"
                @click="onPickImages"
              >
                <el-icon><Picture /></el-icon>
              </button>
            </el-tooltip>
          </div>
          <div v-if="pendingImages.length" class="ai-composer-attachments">
            <div
              v-for="(img, idx) in pendingImages"
              :key="idx"
              class="ai-composer-attachment"
            >
              <img :src="img.url" :alt="img.name || '待发送图片'" class="ai-composer-attachment-thumb" />
              <button
                type="button"
                class="ai-composer-attachment-remove"
                aria-label="移除图片"
                @click="removePendingImage(idx)"
              >
                <el-icon><Close /></el-icon>
              </button>
            </div>
          </div>
          <input
            ref="imageInput"
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            multiple
            class="ai-image-input-hidden"
            @change="onImagesSelected"
          />
          <div class="ai-composer-input-row">
            <textarea
              ref="composerInput"
              v-model="draft"
              class="ai-composer-input"
              :placeholder="composerPlaceholder"
              :disabled="state.streaming || state.pendingCalls.length > 0"
              @keydown.enter.meta.exact.prevent="onEnter"
              @input="autoResize"
              rows="2"
            />
            <button
              class="ai-send-btn"
              :disabled="!canSend || state.streaming || state.pendingCalls.length > 0"
              @click="onSend"
            >
              <el-icon v-if="state.streaming"><Loading /></el-icon>
              <el-icon v-else><Promotion /></el-icon>
            </button>
          </div>
        </div>
        <div class="ai-composer-footer" :class="{ 'ai-composer-footer--page': isPageLayout }">
          <span class="ai-composer-hint" v-if="state.pendingCalls.length">先处理上方等待确认的删除操作</span>
          <span class="ai-composer-hint" v-else>Enter 换行 · {{ sendShortcutHint }} 发送</span>
          <ModelSettingsPopover
            v-if="isPageLayout"
            :pane-id="paneId"
            placement="top-end"
            :busy="state.streaming || state.pendingCalls.length > 0"
          >
            <template #reference>
              <button
                type="button"
                class="ai-composer-settings-btn"
                :disabled="state.streaming || state.pendingCalls.length > 0"
                :title="modelSettingsSummary"
                aria-label="模型设置"
              >
                <el-icon><Setting /></el-icon>
              </button>
            </template>
          </ModelSettingsPopover>
        </div>
      </footer>
      </div>
    </div>
    <AISessionDetailDrawer
      v-model="sessionDetailVisible"
      :project-id="currentProjectId"
      :session-id="sessionDetailId"
    />
  </component>
</template>

<script>
import { nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowRight,
  ChatLineRound,
  CircleClose,
  Close,
  Cpu,
  Delete,
  Search,
  Picture,
  Loading,
  Location,
  Operation,
  Plus,
  Promotion,
  Setting,
  Warning,
} from '@element-plus/icons-vue'
import MarkdownView from './MarkdownView.vue'
import AIToolCallConfirm from './AIToolCallConfirm.vue'
import AIUsageDebugPanel from './AIUsageDebugPanel.vue'
import AISessionDetailDrawer from './AISessionDetailDrawer.vue'
import ModelSettingsPopover from './ModelSettingsPopover.vue'
import SessionListItem from './SessionListItem.vue'
import {
  assistantState,
  bindProject,
  closeAssistant,
  getPane,
  newSession,
  refreshAssistantProviders,
  removeSession,
  renameSession,
  resolvePendingCall,
  selectSession,
  sendMessage,
  setPaneThinking,
  setPaneWebSearch,
  canRemoveWorkspacePane,
  isWorkspacePaneId,
  removeWorkspacePane,
  workspacePaneLabel,
} from '../stores/aiAssistant'
import { authState } from '../auth'
import { projectState } from '../utils/currentProject'
import { getToolDisplayName } from '../utils/aiToolLabels'
import { parseMessageAttachments, pickImageAttachments } from '../utils/aiImageAttachment'
import {
  collectModelOptionIds,
  formatProviderLabel,
  imageUploadTooltip,
  providerHasImageModel,
  providerSupportsThinking,
  providerSupportsWebSearch,
} from '../utils/aiProvider'

const MIN_WIDTH = 480
const MAX_WIDTH = 880
const DEFAULT_WIDTH = 560
const WIDTH_STORAGE_KEY = 'autotest.ai-assistant.panelWidth'
const PANEL_NARROW_MQ = '(max-width: 1279px)'
const COMPOSER_INPUT_MIN_HEIGHT = 56

const PAGE_PROMPT_SUGGESTIONS = [
  '列出当前项目下所有场景',
  '帮我生成一个登录后下单的测试场景',
  '查询某个接口的请求模板与断言',
]

const WORKSPACE_PANE_PATTERN = /^w[0-5]$/

export default {
  name: 'GlobalAIAssistant',
  components: {
    MarkdownView,
    AIToolCallConfirm,
    AIUsageDebugPanel,
    AISessionDetailDrawer,
    ModelSettingsPopover,
    SessionListItem,
  },
  props: {
    layout: {
      type: String,
      default: 'panel',
      validator: (v) => v === 'panel' || v === 'page',
    },
    paneId: {
      type: String,
      default: 'panel',
      validator: (v) => v === 'panel' || WORKSPACE_PANE_PATTERN.test(v),
    },
    paneTitle: {
      type: String,
      default: '',
    },
    paneFocused: {
      type: Boolean,
      default: false,
    },
    showAddPane: {
      type: Boolean,
      default: false,
    },
    canAddPane: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['add-pane'],
  data() {
    return {
      draft: '',
      pendingImages: [],
      panelWidth: this.loadWidth(),
      viewportNarrow: false,
      panelNarrowMq: null,
      resizing: false,
      expandedToolMsgIds: {},
      expandedToolGroupIds: {},
      nowTick: Date.now(),
      thinkingTimer: null,
      sessionDetailVisible: false,
      sessionDetailId: '',
    }
  },
  computed: {
    pane() {
      return getPane(this.paneId)
    },
    state() {
      const pane = this.pane
      return {
        ...assistantState,
        activeSessionId: pane.activeSessionId,
        messages: pane.messages,
        pendingCalls: pane.pendingCalls,
        streaming: pane.streaming,
        error: pane.error,
        selectedProviderId: pane.selectedProviderId,
        selectedModel: pane.selectedModel,
        thinkingEnabled: pane.thinkingEnabled,
        webSearchEnabled: pane.webSearchEnabled,
        debugEnabled: pane.debugEnabled,
        remoteModelList: pane.remoteModelList,
        modelsWarning: pane.modelsWarning,
      }
    },
    isPageLayout() {
      return this.layout === 'page'
    },
    isPanelLayout() {
      return this.layout === 'panel'
    },
    rootWrapper() {
      return this.isPanelLayout ? 'transition' : 'div'
    },
    rootWrapperProps() {
      return this.isPanelLayout ? { name: 'ai-slide' } : {}
    },
    panelHostStyle() {
      if (this.isPageLayout) return {}
      if (!this.visible) {
        return {
          width: '0px',
          flex: '0 0 0px',
          minWidth: '0px',
          maxWidth: '0px',
          overflow: 'hidden',
          '--ai-panel-width': '0px',
        }
      }
      if (this.viewportNarrow) {
        const w = this.panelWidth
        return {
          width: `min(100vw, ${w}px)`,
          maxWidth: '100vw',
          flex: 'none',
          minWidth: 0,
          '--ai-panel-width': `min(100vw, ${w}px)`,
        }
      }
      const w = this.panelWidth
      return {
        width: `${w}px`,
        flex: `0 0 ${w}px`,
        minWidth: `${w}px`,
        maxWidth: `${w}px`,
        '--ai-panel-width': `${w}px`,
      }
    },
    pagePromptSuggestions() {
      return PAGE_PROMPT_SUGGESTIONS
    },
    composerPlaceholder() {
      return this.isPageLayout ? '给 AI 助理发送消息' : '向 AI 助理提问...'
    },
    visible() {
      if (!projectState.currentProjectId) return false
      return this.isPageLayout || assistantState.open
    },
    visibleMessages() {
      return this.pane.messages.filter((m) => m.role !== 'system')
    },
    paneBarLabel() {
      if (this.paneTitle) return this.paneTitle
      if (isWorkspacePaneId(this.paneId)) return workspacePaneLabel(this.paneId)
      return ''
    },
    canRemoveThisPane() {
      return canRemoveWorkspacePane(this.paneId)
    },
    displayTimeline() {
      const items = []
      const msgs = this.visibleMessages
      let i = 0
      while (i < msgs.length) {
        const msg = msgs[i]
        if (msg.role !== 'tool') {
          items.push({ kind: 'message', key: msg.id, msg })
          i += 1
          continue
        }
        const tools = []
        while (i < msgs.length && msgs[i].role === 'tool') {
          tools.push(msgs[i])
          i += 1
        }
        if (tools.length === 1) {
          items.push({ kind: 'toolSingle', key: tools[0].id, msg: tools[0] })
        } else {
          items.push({ kind: 'toolGroup', key: `group-${tools[0].id}`, messages: tools })
        }
      }
      return items
    },
    currentProjectId() {
      return projectState.currentProjectId
    },
    sendShortcutHint() {
      if (typeof navigator === 'undefined') return '⌘+Enter'
      return /Mac|iPhone|iPad|iPod/i.test(navigator.userAgent) ? '⌘+Enter' : 'Win+Enter'
    },
    userAvatarSrc() {
      const u = authState.user
      return u?.avatarUrl ? String(u.avatarUrl) : ''
    },
    userAvatarHueBackground() {
      const raw = String(authState.user?.id || authState.user?.username || 'x')
      let h = 0
      for (let i = 0; i < raw.length; i++) {
        h = raw.charCodeAt(i) + ((h << 5) - h)
      }
      const hue = Math.abs(h) % 360
      return `hsl(${hue}, 42%, 44%)`
    },
    userAvatarBgStyle() {
      if (this.userAvatarSrc) return {}
      return { backgroundColor: this.userAvatarHueBackground }
    },
    userAvatarInitials() {
      const u = authState.user
      const display = (u?.displayName || '').trim()
      const username = (u?.username || '').trim()
      const name = display || username || '管'
      if (/[\u4e00-\u9fff]/.test(name)) {
        return name.length >= 2 ? name.slice(0, 2) : name.slice(0, 1)
      }
      const parts = name.split(/[\s._-]+/).filter(Boolean)
      if (parts.length >= 2) {
        return (parts[0][0] + parts[1][0]).toUpperCase()
      }
      return name.slice(0, 2).toUpperCase()
    },
    pageContextHint() {
      // Render a short, human-friendly hint about what the AI is
      // currently aware of. We prefer named labels (scenarioName,
      // caseName) over raw IDs, falling back to a generic path string.
      const ctx = assistantState.pageContext
      if (!ctx || typeof ctx !== 'object') return ''
      const parts = []
      if (ctx.scenarioName || ctx.scenarioId) {
        parts.push(`场景: ${ctx.scenarioName || ctx.scenarioId.slice(0, 8)}`)
      }
      if (ctx.caseName || ctx.caseId) {
        parts.push(`用例: ${ctx.caseName || ctx.caseId.slice(0, 8)}`)
      }
      if (!parts.length && (ctx.serviceName || ctx.serviceId)) {
        parts.push(`服务: ${ctx.serviceName || ctx.serviceId.slice(0, 8)}`)
      }
      if (!parts.length && ctx.path) {
        parts.push(ctx.path)
      }
      return parts.join(' · ')
    },
    providerOptions() {
      return assistantState.providers.filter((p) => p.enabled !== false)
    },
    selectedProvider() {
      return this.providerOptions.find((p) => p.id === this.state.selectedProviderId) || null
    },
    selectedProviderMeta() {
      const provider = this.selectedProvider
      if (!provider) return null
      return assistantState.providerTypes.find((t) => t.type === provider.providerType) || null
    },
    modelOptions() {
      return collectModelOptionIds({
        remoteModelList: this.state.remoteModelList,
        selectedModel: this.state.selectedModel,
        defaultModel: this.selectedProvider?.defaultModel,
      })
    },
    supportsThinking() {
      return providerSupportsThinking(this.selectedProvider)
    },
    supportsWebSearch() {
      return providerSupportsWebSearch(this.selectedProvider)
    },
    supportsMultimodal() {
      return providerHasImageModel(this.selectedProvider)
    },
    multimodalTooltip() {
      return imageUploadTooltip(this.selectedProvider)
    },
    composerHasToolbar() {
      return this.supportsThinking || this.supportsWebSearch || this.supportsMultimodal
    },
    canSend() {
      return !!this.draft.trim() || this.pendingImages.length > 0
    },
    modelSettingsSummary() {
      const provider = this.selectedProvider
        ? formatProviderLabel(this.selectedProvider)
        : '未选提供商'
      const model = String(this.state.selectedModel || '').trim() || '未选模型'
      return `${provider} · ${model}`
    },
    thinkingTooltip() {
      if (!this.supportsThinking) return ''
      return this.state.thinkingEnabled
        ? '深度思考已开启：模型将进行更充分推理（响应可能更慢）'
        : '深度思考已关闭：点击开启更充分推理'
    },
    webSearchTooltip() {
      if (!this.supportsWebSearch) return ''
      return this.state.webSearchEnabled
        ? '联网搜索已开启：可检索互联网实时信息（按次计费）'
        : '联网搜索已关闭：点击开启实时检索'
    },
  },
  watch: {
    'state.messages'() {
      this.$nextTick(() => this.scrollToBottom())
    },
    'state.streaming'(value) {
      if (!value) this.$nextTick(() => this.scrollToBottom())
    },
    currentProjectId: {
      immediate: true,
      handler(value) {
        if (this.paneId !== 'panel') return
        if (!this.visible) return
        this.preparePanel(value)
      },
    },
    visible(value) {
      if (value) {
        this.preparePanel(this.currentProjectId)
        this.$nextTick(() => this.autoResize())
      }
    },
    'state.activeSessionId'() {
      this.expandedToolMsgIds = {}
    },
  },
  mounted() {
    this.thinkingTimer = window.setInterval(() => {
      this.nowTick = Date.now()
    }, 1000)
    if (typeof window !== 'undefined' && window.matchMedia) {
      this.panelNarrowMq = window.matchMedia(PANEL_NARROW_MQ)
      this.viewportNarrow = this.panelNarrowMq.matches
      this._onPanelNarrowMq = (e) => {
        this.viewportNarrow = e.matches
      }
      this.panelNarrowMq.addEventListener('change', this._onPanelNarrowMq)
    }
  },
  beforeUnmount() {
    if (this.thinkingTimer) {
      window.clearInterval(this.thinkingTimer)
      this.thinkingTimer = null
    }
    if (this.panelNarrowMq && this._onPanelNarrowMq) {
      this.panelNarrowMq.removeEventListener('change', this._onPanelNarrowMq)
    }
  },
  methods: {
    async preparePanel(projectId) {
      if (this.paneId !== 'panel' || !projectId) return
      try {
        await bindProject(projectId)
      } catch (err) {
        if (err && import.meta.env.DEV) console.warn('bindProject failed', err)
      }
      try {
        await refreshAssistantProviders()
      } catch (err) {
        if (err && import.meta.env.DEV) console.warn('refreshAssistantProviders failed', err)
      }
    },
    close() {
      closeAssistant()
    },
    onRemoveThisPane() {
      if (!canRemoveWorkspacePane(this.paneId)) return
      removeWorkspacePane(this.paneId)
    },
    onAddPaneClick() {
      if (!this.canAddPane) return
      this.$emit('add-pane')
    },
    applyPrompt(text) {
      this.draft = text
      this.$nextTick(() => {
        this.autoResize()
        this.$refs.composerInput?.focus()
      })
    },
    async onSend() {
      const text = this.draft.trim()
      const images = this.pendingImages.map((img) => ({ url: img.url, name: img.name }))
      if (!text && !images.length) return
      this.draft = ''
      this.pendingImages = []
      this.$nextTick(() => this.autoResize())
      await sendMessage(text, this.paneId, images)
    },
    onPickImages() {
      this.$refs.imageInput?.click()
    },
    async onImagesSelected(event) {
      const input = event?.target
      try {
        const picked = await pickImageAttachments(this.pendingImages.length, input?.files)
        this.pendingImages = [...this.pendingImages, ...picked]
      } catch (err) {
        ElMessage.warning(err?.message || '图片添加失败')
      } finally {
        if (input) input.value = ''
      }
    },
    removePendingImage(index) {
      this.pendingImages = this.pendingImages.filter((_, i) => i !== index)
    },
    userMessageImages(msg) {
      return parseMessageAttachments(msg?.attachments)
    },
    onEnter() {
      this.onSend()
    },
    async onNewSession() {
      await newSession('')
    },
    async onSelectSession(id) {
      await selectSession(id)
    },
    onSessionDetail(session) {
      if (!session?.id) return
      this.sessionDetailId = session.id
      this.sessionDetailVisible = true
    },
    async onRenameSession(session) {
      if (!session?.id) return
      try {
        const { value } = await ElMessageBox.prompt('请输入新的会话名称', '重命名会话', {
          confirmButtonText: '保存',
          cancelButtonText: '取消',
          inputValue: session.title || '',
          inputPattern: /\S/,
          inputErrorMessage: '名称不能为空',
        })
        const title = String(value || '').trim()
        if (!title) return
        await renameSession(session.id, title)
        ElMessage.success('会话已重命名')
      } catch {
        // 用户取消
      }
    },
    async onDeleteSession(session) {
      try {
        await ElMessageBox.confirm(`确定删除会话「${session.title || '未命名'}」吗？`, '删除会话', {
          confirmButtonText: '删除',
          cancelButtonText: '取消',
          type: 'warning',
        })
      } catch {
        return
      }
      await removeSession(session.id)
    },
    async onDecide({ callId, approve, reason }) {
      await resolvePendingCall(callId, { approve, reason }, this.paneId)
    },
    onToggleThinking() {
      setPaneThinking(this.paneId, !this.state.thinkingEnabled)
    },
    onToggleWebSearch() {
      setPaneWebSearch(this.paneId, !this.state.webSearchEnabled)
    },
    providerLabel(provider) {
      return formatProviderLabel(provider)
    },
    scrollToBottom() {
      const el = this.$refs.bodyEl
      if (!el) return
      el.scrollTop = el.scrollHeight
    },
    roleLabel(role) {
      switch (role) {
        case 'assistant': return 'AI'
        case 'tool': return '工具'
        default: return role
      }
    },
    formatToolContent(raw) {
      if (!raw) return ''
      try {
        const parsed = JSON.parse(raw)
        return JSON.stringify(parsed, null, 2)
      } catch {
        return raw
      }
    },
    isToolMsgExpanded(msgId) {
      return !!this.expandedToolMsgIds[msgId]
    },
    toggleToolMsg(msgId) {
      this.expandedToolMsgIds = {
        ...this.expandedToolMsgIds,
        [msgId]: !this.expandedToolMsgIds[msgId],
      }
    },
    isToolGroupExpanded(groupKey) {
      return !!this.expandedToolGroupIds[groupKey]
    },
    toggleToolGroup(groupKey) {
      this.expandedToolGroupIds = {
        ...this.expandedToolGroupIds,
        [groupKey]: !this.expandedToolGroupIds[groupKey],
      }
    },
    toolGroupSummary(messages) {
      const names = (messages || []).map((m) => this.toolMessageTitle(m))
      const n = names.length
      if (!n) return '工具调用'
      if (n <= 3) return `连续调用 ${n} 个工具：${names.join('、')}`
      return `连续调用 ${n} 个工具：${names.slice(0, 2).join('、')} 等`
    },
    resolveToolCallName(callId) {
      if (!callId) return ''
      for (let i = this.pane.messages.length - 1; i >= 0; i -= 1) {
        const m = this.pane.messages[i]
        if (m.role !== 'assistant') continue
        const calls = this.parseToolCalls(m.toolCalls)
        for (const call of calls) {
          if (call.id === callId) return call.name || ''
        }
      }
      return ''
    },
    parseToolCalls(raw) {
      if (!raw) return []
      if (Array.isArray(raw)) return raw
      try {
        const parsed = JSON.parse(raw)
        return Array.isArray(parsed) ? parsed : []
      } catch {
        return []
      }
    },
    toolMessageTitle(msg) {
      const name = this.resolveToolCallName(msg.toolCallId)
      if (!name) return '工具结果'
      return getToolDisplayName(name)
    },
    formatThinkingElapsed(msg) {
      if (!msg) return ''
      const started = Number(msg.thinkingStartedAt) || 0
      let elapsed = Number(msg.thinkingElapsedMillis) || 0
      if (msg.thinking && started > 0) {
        elapsed = Math.max(0, this.nowTick - started)
      }
      const seconds = Math.floor(elapsed / 1000)
      return seconds > 0 ? `${seconds}s` : ''
    },
    autoResize() {
      const el = this.$refs.composerInput
      if (!el) return
      el.style.height = 'auto'
      el.style.height = `${Math.max(COMPOSER_INPUT_MIN_HEIGHT, Math.min(el.scrollHeight, 150))}px`
    },
    // Width persistence
    loadWidth() {
      try {
        const saved = parseInt(window.localStorage.getItem(WIDTH_STORAGE_KEY), 10)
        if (saved >= MIN_WIDTH && saved <= MAX_WIDTH) return saved
      } catch {}
      return DEFAULT_WIDTH
    },
    saveWidth() {
      try {
        window.localStorage.setItem(WIDTH_STORAGE_KEY, String(this.panelWidth))
      } catch {}
    },
    // Drag resize
    onResizeStart(e) {
      if (this.viewportNarrow) return
      e.preventDefault()
      this.resizing = true
      const startX = e.clientX
      const startWidth = this.panelWidth

      const onMove = (ev) => {
        const delta = startX - ev.clientX
        this.panelWidth = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, startWidth + delta))
      }

      const onUp = () => {
        this.resizing = false
        this.saveWidth()
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
      }

      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
    },
  },
}
</script>

<style scoped>
.ai-panel-host {
  flex: 0 0 auto;
  align-self: stretch;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  z-index: 4;
}

.ai-panel-host--narrow {
  position: fixed;
  inset: 0;
  left: auto;
  z-index: 2100;
  box-shadow: -8px 0 32px rgba(15, 23, 42, 0.18);
}

.ai-panel-host--page {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  width: 100%;
  z-index: auto;
}

.ai-panel {
  flex: 1 1 auto;
  width: 100%;
  height: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: var(--app-card-bg, #fff);
  border-left: 1px solid var(--el-border-color-lighter, #ebeef5);
  box-shadow: -10px 0 32px rgba(15, 23, 42, 0.08);
  position: relative;
  overflow: hidden;
}

.ai-panel-host--page .ai-panel {
  box-shadow: none;
}

.ai-panel--page {
  flex: 1 1 auto;
  width: 100%;
  height: 100%;
  min-height: 0;
  border-left: none;
  border-radius: 0;
  background: var(--app-surface-subtle);
}

.ai-header,
.ai-page-context {
  flex-shrink: 0;
}

.ai-chat-stage {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ai-panel-host--page {
  position: relative;
}

.ai-pane-bar {
  flex-shrink: 0;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid #eef0f3;
  background: #fff;
}

.ai-pane-bar--focused {
  box-shadow: inset 0 -2px 0 var(--el-color-primary);
}

.ai-pane-bar__title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: #1f2329;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-pane-bar__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.ai-pane-bar__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-muted, #909399);
  font-size: 15px;
  cursor: pointer;
  transition:
    color 0.15s ease,
    background 0.15s ease,
    border-color 0.15s ease;
}

.ai-pane-bar__btn:hover:not(:disabled) {
  background: var(--el-fill-color-light, #f5f7fa);
  border-color: var(--app-border-color, #e2e8f0);
  color: var(--el-text-color-primary, #303133);
}

.ai-pane-bar__btn--close:hover:not(:disabled) {
  color: var(--el-color-danger, #f56c6c);
  background: color-mix(in srgb, var(--el-color-danger, #f56c6c) 10%, transparent);
  border-color: color-mix(in srgb, var(--el-color-danger, #f56c6c) 22%, transparent);
}

.ai-pane-bar__btn--add:hover:not(:disabled) {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9, #f5f9ff);
  border-color: var(--el-color-primary-light-5);
}

.ai-pane-bar__btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.ai-pane-add-fallback {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 20;
}

.ai-panel--page .ai-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  max-width: 820px;
  width: 100%;
  margin: 0 auto;
  padding: 24px 20px 16px;
  background: transparent;
  -webkit-overflow-scrolling: touch;
}

.ai-panel--page .ai-composer {
  flex-shrink: 0;
  padding: 12px 20px 20px;
  border-top: none;
  background: var(--app-surface-subtle);
  box-shadow: 0 -8px 24px rgba(15, 23, 42, 0.04);
}

.ai-panel--page .ai-composer-box {
  max-width: 820px;
  margin: 0 auto;
  background: #fff;
  border: 1px solid #e3e6eb;
  border-radius: 20px;
  box-shadow: 0 4px 24px rgba(15, 23, 42, 0.06);
}

.ai-panel--page .ai-composer-box:focus-within {
  border-color: color-mix(in srgb, var(--el-color-primary) 45%, #e3e6eb);
  box-shadow:
    0 4px 24px rgba(15, 23, 42, 0.06),
    0 0 0 3px color-mix(in srgb, var(--el-color-primary) 12%, transparent);
}

.ai-panel--page .ai-composer-input-row {
  min-height: 64px;
  padding: 12px 14px;
}

.ai-panel--page .ai-send-btn {
  width: 40px;
  height: 40px;
  border-radius: 50%;
}

.ai-panel--page .ai-composer-footer {
  max-width: 820px;
  margin: 8px auto 0;
  padding: 0 2px;
}

.ai-composer-footer--page {
  justify-content: space-between;
  gap: 8px;
}

.ai-composer-footer--page .ai-composer-hint {
  flex: 1;
  min-width: 0;
  text-align: left;
}

.ai-composer-settings-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-muted, #909399);
  font-size: 16px;
  cursor: pointer;
  transition: color 0.15s ease, background 0.15s ease;
}

.ai-composer-settings-btn:hover:not(:disabled) {
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 10%, transparent);
}

.ai-composer-settings-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.ai-panel--page .ai-message--assistant :deep(.markdown-view) {
  background: transparent;
  padding: 4px 0;
}

.ai-panel--page .ai-text--user {
  background: var(--el-color-primary);
}

/* Slide transition */
.ai-slide-enter-active,
.ai-slide-leave-active {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s ease;
}

.ai-slide-enter-from,
.ai-slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

.ai-panel-host--narrow.ai-slide-enter-from,
.ai-panel-host--narrow.ai-slide-leave-to {
  transform: translateX(100%);
}

/* Resize handle */
.ai-resize-handle {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 5px;
  cursor: col-resize;
  z-index: 10;
  transition: background 0.15s ease;
}

.ai-resize-handle:hover,
.ai-resize-handle--active {
  background: var(--el-color-primary, #409eff);
}

.ai-resize-handle::after {
  content: '';
  position: absolute;
  left: -3px;
  right: -3px;
  top: 0;
  bottom: 0;
}

/* Header */
.ai-header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
  background: var(--app-card-bg, #fff);
  min-height: 48px;
}

.ai-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.ai-header-icon {
  font-size: 18px;
  color: var(--el-color-primary, #409eff);
  flex-shrink: 0;
}

.ai-header-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary, #303133);
  white-space: nowrap;
}

.ai-streaming-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-color-primary, #409eff);
  animation: pulse-dot 1.2s ease-in-out infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.ai-header-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.ai-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--el-text-color-regular, #606266);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  font-size: 16px;
}

.ai-icon-btn:hover {
  background: var(--el-fill-color-light, #f5f7fa);
  color: var(--el-text-color-primary, #303133);
}

.ai-icon-btn--active {
  color: var(--el-color-primary, #409eff);
}

.ai-icon-btn--active:hover {
  color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-icon-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

/* History */
.ai-history {
  max-height: 320px;
  overflow-y: auto;
}

.ai-history-empty {
  padding: 16px;
  color: var(--el-text-color-placeholder, #a8abb2);
  text-align: center;
  font-size: 13px;
}

.ai-history-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.ai-history-row:hover {
  background: var(--el-fill-color-light, #f5f7fa);
}

.ai-history-row.active {
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-history-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary, #303133);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-history-meta {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--el-text-color-secondary, #909399);
}

.ai-history-delete {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--el-text-color-placeholder, #a8abb2);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  font-size: 14px;
}

.ai-history-delete:hover {
  background: var(--el-color-danger-light-9, #fef0f0);
  color: var(--el-color-danger, #f56c6c);
}

/* Body */
.ai-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  -webkit-overflow-scrolling: touch;
}

.ai-panel:not(.ai-panel--page) .ai-body {
  padding: 18px 18px 16px;
  gap: 14px;
}

.ai-panel:not(.ai-panel--page) .ai-message--assistant :deep(.markdown-view) {
  max-width: min(100%, 76ch);
}

.ai-panel:not(.ai-panel--page) .ai-user-bubble {
  max-width: min(78ch, calc(100% - 48px));
}

.ai-panel:not(.ai-panel--page) .ai-composer {
  padding: 12px 18px 16px;
}

@media (max-width: 767px) {
  .ai-resize-handle {
    display: none;
  }
}

.ai-page-context {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
  background: var(--el-fill-color-light, #f5f7fa);
  font-size: 12px;
  color: var(--el-text-color-regular, #606266);
  overflow: hidden;
}

.ai-page-context-label {
  color: var(--el-text-color-secondary, #909399);
  white-space: nowrap;
}

.ai-page-context-value {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ai-body::-webkit-scrollbar {
  width: 6px;
}

.ai-body::-webkit-scrollbar-track {
  background: transparent;
}

.ai-body::-webkit-scrollbar-thumb {
  background: var(--el-border-color, #dcdfe6);
  border-radius: 3px;
}

.ai-body::-webkit-scrollbar-thumb:hover {
  background: var(--el-text-color-placeholder, #a8abb2);
}

/* Empty state */
.ai-empty {
  flex: 1 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  text-align: center;
  min-height: min(100%, 320px);
}

.ai-empty-icon {
  color: var(--el-border-color, #dcdfe6);
  margin-bottom: 8px;
}

.ai-empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary, #303133);
  margin: 0;
}

.ai-empty-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary, #909399);
  margin: 0;
  line-height: 1.6;
}

.ai-empty-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder, #a8abb2);
  margin: 0;
}

.ai-empty--page {
  flex: 1 0 auto;
  justify-content: center;
  min-height: min(100%, calc(100vh - 320px));
  padding: 24px 16px 32px;
  box-sizing: border-box;
}

.ai-empty-icon--page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  margin-bottom: 16px;
  border-radius: 50%;
  color: var(--el-color-primary);
  background: #fff;
  box-shadow: 0 8px 32px rgba(15, 23, 42, 0.06);
}

.ai-empty--page .ai-empty-title {
  font-size: 28px;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: #1f2329;
}

.ai-empty--page .ai-empty-desc {
  font-size: 14px;
  color: var(--app-text-muted);
  max-width: 420px;
}

.ai-empty-prompts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 28px;
  width: 100%;
  max-width: 560px;
}

.ai-empty-prompt {
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid #e8eaed;
  background: #fff;
  color: #1f2329;
  font-size: 14px;
  line-height: 1.45;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background 0.15s ease,
    box-shadow 0.15s ease;
}

.ai-empty-prompt:hover {
  border-color: color-mix(in srgb, var(--el-color-primary) 35%, #e8eaed);
  background: #fff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.06);
}

@media (max-width: 640px) {
  .ai-empty-prompts {
    grid-template-columns: 1fr;
  }
}

/* Messages */
.ai-message {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ai-message-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
}

.ai-role {
  font-weight: 600;
  font-size: 12px;
}

.ai-role--user {
  color: var(--el-color-primary, #409eff);
}

.ai-role--assistant {
  color: var(--el-text-color-primary, #303133);
}

.ai-role--tool {
  color: var(--el-color-warning, #e6a23c);
}

.ai-model {
  font-size: 11px;
  color: var(--el-text-color-placeholder, #a8abb2);
  background: var(--el-fill-color-light, #f5f7fa);
  padding: 1px 6px;
  border-radius: 4px;
}

.ai-streaming-indicator {
  display: inline-flex;
  align-items: center;
}

.ai-streaming-cursor {
  display: inline-block;
  width: 2px;
  height: 14px;
  background: var(--el-color-primary, #409eff);
  animation: blink-cursor 0.8s step-end infinite;
  border-radius: 1px;
}

@keyframes blink-cursor {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

.ai-thinking-state,
.ai-replying-state {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: fit-content;
  max-width: 100%;
  padding: 8px 12px;
  border-radius: 12px 12px 12px 2px;
  background: linear-gradient(135deg, var(--el-color-primary-light-9, #ecf5ff), var(--el-fill-color-light, #f5f7fa));
  color: var(--el-text-color-secondary, #606266);
  font-size: 13px;
  line-height: 1.4;
}

.ai-replying-label {
  font-weight: 500;
}

.ai-thinking-orb {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 0 rgba(64, 158, 255, 0.35);
  animation: thinking-pulse 1.4s ease-in-out infinite;
}

.ai-thinking-label {
  font-weight: 500;
}

.ai-thinking-elapsed {
  color: var(--el-text-color-placeholder, #a8abb2);
  font-variant-numeric: tabular-nums;
}

@keyframes thinking-pulse {
  0% { transform: scale(0.85); box-shadow: 0 0 0 0 rgba(64, 158, 255, 0.35); }
  70% { transform: scale(1); box-shadow: 0 0 0 8px rgba(64, 158, 255, 0); }
  100% { transform: scale(0.85); box-shadow: 0 0 0 0 rgba(64, 158, 255, 0); }
}

.ai-text--user {
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--el-color-primary, #409eff) 94%, #fff),
    color-mix(in srgb, var(--el-color-primary, #409eff) 82%, #7c3aed)
  );
  color: #fff;
  border-radius: 18px 18px 6px 18px;
  padding: 11px 15px;
  max-width: 100%;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 14px;
  line-height: 1.6;
  box-shadow: 0 8px 22px rgba(37, 99, 235, 0.16);
}

.ai-message--assistant :deep(.markdown-view) {
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid color-mix(in srgb, var(--el-border-color-lighter, #ebeef5) 70%, transparent);
  border-radius: 18px 18px 18px 6px;
  padding: 12px 15px;
  font-size: 14px;
  line-height: 1.6;
}

.ai-message--pending {
  border-left: 3px solid var(--el-color-warning, #e6a23c);
  padding-left: 12px;
}

.ai-user-bubble {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  max-width: min(78ch, calc(100% - 48px));
  min-width: 0;
}

.ai-user-images {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.ai-user-image {
  max-width: 200px;
  max-height: 160px;
  border-radius: 10px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.25);
}

.ai-message-user-row {
  display: flex;
  flex-direction: row;
  justify-content: flex-end;
  align-items: flex-end;
  gap: 8px;
  width: 100%;
}

.ai-composer-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 12px 0;
}

.ai-composer-attachment {
  position: relative;
  width: 56px;
  height: 56px;
}

.ai-composer-attachment-thumb {
  width: 100%;
  height: 100%;
  border-radius: 8px;
  object-fit: cover;
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.ai-composer-attachment-remove {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 20px;
  height: 20px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: var(--el-text-color-primary, #303133);
  color: #fff;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}

.ai-image-input-hidden {
  display: none;
}

.ai-message-user-avatar {
  flex-shrink: 0;
  color: #fff;
  font-size: 12px;
}

/* Tool calls */
.ai-tool-chevron {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  transition: transform 0.15s ease;
}

.ai-tool-chevron--expanded {
  transform: rotate(90deg);
}

.ai-tool-group {
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 8px;
  overflow: hidden;
}

.ai-tool-group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 32px;
  padding: 0 10px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-size: 12px;
  color: var(--el-text-color-regular, #606266);
}

.ai-tool-group-head:hover {
  background: var(--el-fill-color, #ebeef5);
}

.ai-tool-group-summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-tool-group-body {
  border-top: 1px solid var(--el-border-color-extra-light, #f2f6fc);
}

.ai-tool-row {
  border-top: 1px solid var(--el-border-color-extra-light, #f2f6fc);
}

.ai-tool-row:first-child {
  border-top: none;
}

.ai-tool-row-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 32px;
  padding: 0 10px 0 20px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-size: 12px;
  color: var(--el-text-color-regular, #606266);
}

.ai-tool-row-head:hover {
  background: var(--el-fill-color, #ebeef5);
}

.ai-tool-row-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.ai-tool-row-body {
  margin: 0;
  padding: 8px 12px 10px 32px;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  max-height: 200px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--el-text-color-regular, #606266);
  line-height: 1.5;
  border-top: 1px solid var(--el-border-color-extra-light, #f2f6fc);
  background: var(--el-fill-color-blank, #fff);
}

.ai-tool-row--standalone {
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 8px;
  overflow: hidden;
}

.ai-tool-row--standalone .ai-tool-row-head {
  padding-left: 10px;
}

/* Pending block */
.ai-pending-block {
  background: rgba(230, 162, 60, 0.06);
  border: 1px solid rgba(230, 162, 60, 0.3);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ai-pending-title {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--el-color-warning, #b88230);
  font-weight: 600;
  font-size: 13px;
}

/* Error */
.ai-error {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(245, 108, 108, 0.08);
  color: var(--el-color-danger, #f56c6c);
  border: 1px solid rgba(245, 108, 108, 0.2);
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 13px;
}

/* Composer */
.ai-composer {
  flex-shrink: 0;
  border-top: 1px solid var(--el-border-color-lighter, #ebeef5);
  padding: 12px 16px;
  background: var(--app-card-bg, #fff);
  z-index: 2;
}

.ai-composer-box {
  position: relative;
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 12px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.ai-composer-box:focus-within {
  border-color: var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 3px rgba(64, 158, 255, 0.1);
}

.ai-composer-toolbar {
  position: absolute;
  top: 8px;
  right: 10px;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 4px;
}

.ai-composer-toggle-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--el-text-color-secondary, #909399);
  font-size: 16px;
  cursor: pointer;
  transition: color 0.15s ease, background 0.15s ease;
}

.ai-composer-toggle-btn:hover:not(:disabled) {
  background: var(--el-fill-color, #ebeef5);
  color: var(--el-text-color-primary, #303133);
}

.ai-composer-toggle-btn--active {
  background: var(--el-color-primary-light-9, #ecf5ff);
  color: var(--el-color-primary, #409eff);
}

.ai-composer-toggle-btn--active:hover:not(:disabled) {
  background: var(--el-color-primary-light-8, #d9ecff);
  color: var(--el-color-primary, #409eff);
}

.ai-composer-toggle-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.ai-composer-input-row {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 10px 12px;
  min-height: 76px;
}

.ai-composer-box--with-toolbar .ai-composer-input-row {
  padding-top: 36px;
}

.ai-composer-input {
  flex: 1;
  min-width: 0;
  min-height: 56px;
  border: none;
  background: transparent;
  resize: none;
  outline: none;
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-primary, #303133);
  font-family: inherit;
  max-height: 150px;
}

.ai-composer-input::placeholder {
  color: var(--el-text-color-placeholder, #a8abb2);
}

.ai-composer-input:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.ai-send-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 8px;
  background: var(--el-color-primary, #409eff);
  color: #fff;
  cursor: pointer;
  transition: background 0.15s ease, opacity 0.15s ease;
  font-size: 16px;
}

.ai-send-btn:hover:not(:disabled) {
  background: var(--el-color-primary-light-3, #79bbff);
}

.ai-send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.ai-composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 6px;
  padding: 0 2px;
}

.ai-composer-hint {
  font-size: 11px;
  color: var(--el-text-color-placeholder, #a8abb2);
}
</style>

<style>
.ai-assistant-model-dropdown.el-popper {
  min-width: 360px;
}
</style>
