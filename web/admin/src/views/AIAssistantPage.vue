<template>
  <WorkspaceLayout
    class="ds-chat-page"
    :breakpoint="900"
    narrow-mode="drawer"
    drawer-trigger-label="会话列表"
    sidebar-width="260px"
  >
    <template #sidebar>
    <aside class="ds-sidebar" :class="{ 'ds-sidebar--disabled': !currentProjectId }">
      <div class="ds-sidebar__brand">
        <span class="ds-sidebar__logo" aria-hidden="true">
          <el-icon :size="20"><ChatLineRound /></el-icon>
        </span>
        <span class="ds-sidebar__title">AI 助理</span>
      </div>

      <el-button
        class="ds-new-chat"
        :disabled="!currentProjectId || workspaceBusy"
        @click="onNewSession"
      >
        <el-icon><Plus /></el-icon>
        <span>在{{ focusedPaneLabel }}开启新对话</span>
      </el-button>

      <div class="ds-session-list" v-loading="sessionsLoading">
        <p v-if="!currentProjectId" class="ds-session-empty">请先在顶栏选择项目</p>
        <p v-else-if="!state.sessions.length" class="ds-session-empty">暂无历史会话</p>
        <SessionListItem
          v-for="session in state.sessions"
          :key="session.id"
          :session="session"
          :active="isSessionActive(session.id)"
          :focused="isSessionInFocusedPane(session.id)"
          :tags="sessionPaneTags(session.id)"
          @select="() => onSelectSession(session.id)"
          @detail="onSessionDetail"
          @rename="onRenameSession"
          @delete="onDeleteSession"
        />
      </div>

      <div class="ds-sidebar__footer">
        <div class="ds-split-control">
          <span class="ds-split-control__label">分屏对话</span>
          <el-switch
            :model-value="splitMode"
            inline-prompt
            active-text="开"
            inactive-text="关"
            :disabled="!currentProjectId"
            @change="onSplitModeChange"
          />
        </div>
        <ModelSettingsPopover :disabled="!currentProjectId" :busy="workspaceBusy">
          <template #reference>
            <button type="button" class="ds-settings-btn" :disabled="!currentProjectId">
              <el-icon><Setting /></el-icon>
              <span>模型设置</span>
            </button>
          </template>
        </ModelSettingsPopover>
      </div>
    </aside>
    </template>

    <template v-if="currentProjectId">
        <el-tabs
          v-if="showSplitTabs"
          v-model="narrowPaneTab"
          class="ds-split-tabs"
          @tab-change="onNarrowPaneTabChange"
        >
          <el-tab-pane label="对话 A" name="left" />
          <el-tab-pane label="对话 B" name="right" />
        </el-tabs>
        <div class="ds-split" :class="{ 'ds-split--dual': splitMode && !showSplitTabs }">
          <section
            v-show="!showSplitTabs || narrowPaneTab === 'left'"
            class="ds-pane"
            :class="{ 'ds-pane--focused': focusedPaneId === 'left' }"
            @mousedown="focusPane('left')"
          >
            <GlobalAIAssistant
              layout="page"
              pane-id="left"
              :pane-focused="focusedPaneId === 'left'"
            />
          </section>
          <div v-if="splitMode && !showSplitTabs" class="ds-split-divider" aria-hidden="true" />
          <section
            v-show="splitMode && (!showSplitTabs || narrowPaneTab === 'right')"
            class="ds-pane"
            :class="{ 'ds-pane--focused': focusedPaneId === 'right' }"
            @mousedown="focusPane('right')"
          >
            <GlobalAIAssistant
              layout="page"
              pane-id="right"
              :pane-focused="focusedPaneId === 'right'"
            />
          </section>
        </div>
    </template>
    <div v-else class="ds-main-empty">
      <div class="ds-main-empty__icon" aria-hidden="true">
        <el-icon :size="48"><ChatLineRound /></el-icon>
      </div>
      <p class="ds-main-empty__title">选择项目后开始对话</p>
      <p class="ds-main-empty__desc">在页面右上角选择测试项目，即可使用 AI 助理查询与编排。</p>
    </div>
    <AISessionDetailDrawer
      v-model="sessionDetailVisible"
      :project-id="currentProjectId"
      :session-id="sessionDetailId"
    />
  </WorkspaceLayout>
</template>

<script>
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Plus, Setting } from '@element-plus/icons-vue'
import GlobalAIAssistant from '../components/GlobalAIAssistant.vue'
import ModelSettingsPopover from '../components/ModelSettingsPopover.vue'
import AISessionDetailDrawer from '../components/AISessionDetailDrawer.vue'
import SessionListItem from '../components/SessionListItem.vue'
import WorkspaceLayout from '../components/WorkspaceLayout.vue'
import {
  anyPaneStreaming,
  assistantState,
  bindProject,
  isSessionOpenInPane,
  newSession,
  removeSession,
  renameSession,
  selectSession,
  setFocusedPane,
  setSplitMode,
  WORKSPACE_PANE_IDS,
} from '../stores/aiAssistant'
import { projectState } from '../utils/currentProject'

export default {
  name: 'AIAssistantPage',
  components: {
    GlobalAIAssistant,
    ModelSettingsPopover,
    AISessionDetailDrawer,
    SessionListItem,
    WorkspaceLayout,
    ChatLineRound,
    Plus,
    Setting,
  },
  data() {
    return {
      sessionsLoading: false,
      sessionDetailVisible: false,
      sessionDetailId: '',
      viewportWidth: typeof window !== 'undefined' ? window.innerWidth : 1200,
      narrowPaneTab: 'left',
    }
  },
  computed: {
    state() {
      return assistantState
    },
    currentProjectId() {
      return projectState.currentProjectId
    },
    isNarrowViewport() {
      return this.viewportWidth < 900
    },
    showSplitTabs() {
      return this.splitMode && this.isNarrowViewport
    },
    splitMode() {
      return assistantState.splitMode
    },
    focusedPaneId() {
      return assistantState.focusedPaneId
    },
    focusedPaneLabel() {
      return this.focusedPaneId === 'right' ? '对话 B' : '对话 A'
    },
    workspaceBusy() {
      return anyPaneStreaming(WORKSPACE_PANE_IDS)
    },
  },
  watch: {
    currentProjectId: {
      immediate: true,
      handler(projectId) {
        this.bindCurrentProject(projectId)
      },
    },
    focusedPaneId(value) {
      if (value === 'left' || value === 'right') this.narrowPaneTab = value
    },
  },
  mounted() {
    this.onViewportResize = () => {
      this.viewportWidth = window.innerWidth
    }
    window.addEventListener('resize', this.onViewportResize)
    this.onViewportResize()
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.onViewportResize)
  },
  methods: {
    onNarrowPaneTabChange(name) {
      if (name === 'left' || name === 'right') setFocusedPane(name)
    },
    async bindCurrentProject(projectId) {
      if (!projectId) return
      this.sessionsLoading = true
      try {
        await bindProject(projectId)
      } finally {
        this.sessionsLoading = false
      }
    },
    onSplitModeChange(value) {
      setSplitMode(!!value)
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
    focusPane(paneId) {
      setFocusedPane(paneId)
    },
    isSessionActive(sessionId) {
      if (!sessionId) return false
      if (this.splitMode) {
        return isSessionOpenInPane(sessionId, 'left') || isSessionOpenInPane(sessionId, 'right')
      }
      return isSessionOpenInPane(sessionId, 'left')
    },
    isSessionInFocusedPane(sessionId) {
      return isSessionOpenInPane(sessionId, this.focusedPaneId)
    },
    sessionPaneTags(sessionId) {
      const tags = []
      if (this.splitMode) {
        if (isSessionOpenInPane(sessionId, 'left')) tags.push('A')
        if (isSessionOpenInPane(sessionId, 'right')) tags.push('B')
      }
      return tags
    },
    async onNewSession() {
      await newSession('', this.focusedPaneId)
    },
    async onSelectSession(id) {
      const otherPane = this.focusedPaneId === 'left' ? 'right' : 'left'
      if (this.splitMode && isSessionOpenInPane(id, otherPane)) {
        ElMessage.warning('该会话已在另一分屏中打开，请先切换或关闭另一侧')
        return
      }
      await selectSession(id, this.focusedPaneId)
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
  },
}
</script>

<style scoped>
.ds-chat-page {
  display: flex;
  height: calc(100vh - var(--el-header-height, 60px));
  margin: -20px;
  background: var(--app-surface-subtle);
  overflow: hidden;
}

.ds-sidebar {
  flex: 1 1 auto;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #e8eaed;
  min-height: 0;
}

.ds-sidebar--disabled {
  opacity: 0.92;
}

.ds-sidebar__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 16px 12px;
}

.ds-sidebar__logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ds-sidebar__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text-color);
}

.ds-new-chat {
  margin: 0 12px 12px;
  width: calc(100% - 24px);
  height: 40px;
  border-radius: 10px;
  border: 1px solid #dce0e6;
  background: #fff;
  color: var(--app-text-color);
  font-weight: 500;
  justify-content: flex-start;
  gap: 8px;
  padding-left: 14px;
}

.ds-new-chat:hover:not(:disabled) {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9, #f5f9ff);
  color: var(--el-color-primary);
}

.ds-session-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 0 8px 8px;
}

.ds-session-empty {
  margin: 12px 8px;
  font-size: 13px;
  color: var(--app-text-muted);
  line-height: 1.5;
}

.ds-session-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  margin-bottom: 2px;
  padding: 10px 36px 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s ease;
}

.ds-session-item:hover {
  background: var(--app-surface-subtle);
}

.ds-session-item.active {
  background: var(--app-focus-ring);
}

.ds-session-item__title {
  width: 100%;
  font-size: 14px;
  font-weight: 500;
  color: var(--app-text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ds-session-item__meta {
  font-size: 12px;
  color: var(--app-text-muted);
}

.ds-session-item__delete {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s ease, background 0.15s ease, color 0.15s ease;
}

.ds-session-item:hover .ds-session-item__delete,
.ds-session-item.active .ds-session-item__delete {
  opacity: 1;
}

.ds-session-item__delete:hover {
  background: #fee;
  color: var(--el-color-danger);
}

.ds-sidebar__footer {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px 14px;
  border-top: 1px solid #eef0f3;
}

.ds-split-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--app-surface-subtle);
  border: 1px solid #eef0f3;
}

.ds-split-control__label {
  font-size: 13px;
  font-weight: 500;
  color: var(--app-text-color);
}

.ds-settings-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-muted);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.ds-settings-btn:hover:not(:disabled) {
  background: var(--app-surface-subtle);
  color: var(--app-text-color);
}

.ds-settings-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ds-model-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ds-model-settings-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ds-model-settings-label {
  font-size: 12px;
  color: var(--app-text-muted);
}

.ds-model-settings-model {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ds-model-settings-model :deep(.el-select) {
  flex: 1;
}

.ds-main {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--app-surface-subtle);
}

.ds-split {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ds-split--dual {
  flex-direction: row;
}

.ds-pane {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--app-surface-subtle);
  outline: 2px solid transparent;
  outline-offset: -2px;
  transition: outline-color 0.15s ease;
}

.ds-pane--focused {
  outline-color: color-mix(in srgb, var(--el-color-primary) 40%, transparent);
}

.ds-split-divider {
  flex: 0 0 1px;
  align-self: stretch;
  background: #e8eaed;
}

.ds-split-tabs {
  flex: 0 0 auto;
  padding: 0 12px;
  background: var(--app-card-bg);
  border-bottom: 1px solid var(--app-border-color);
}

.ds-split--dual .ds-pane {
  flex: 1 1 0;
  width: 50%;
}

.ds-main :deep(.ai-panel-host--page) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
}

.ds-session-item__title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.ds-session-item__tags {
  display: inline-flex;
  flex-shrink: 0;
  gap: 4px;
}

.ds-session-item__tag {
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--el-color-primary);
  background: var(--app-focus-ring);
}

.ds-session-item--focused.active {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--el-color-primary) 35%, transparent);
}

.ds-main-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 32px;
  text-align: center;
}

.ds-main-empty__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 88px;
  height: 88px;
  border-radius: 50%;
  color: var(--el-color-primary);
  background: #fff;
  box-shadow: 0 8px 32px rgba(15, 23, 42, 0.06);
}

.ds-main-empty__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--app-text-color);
}

.ds-main-empty__desc {
  margin: 0;
  max-width: 360px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--app-text-muted);
}
</style>
