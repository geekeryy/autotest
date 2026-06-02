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
      <router-link to="/" class="ds-sidebar__brand">
        <span class="ds-sidebar__logo" aria-hidden="true">
          <el-icon :size="20"><ChatLineRound /></el-icon>
        </span>
        <span class="ds-sidebar__title">AI 助理</span>
      </router-link>

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
    </aside>
    </template>

    <template v-if="currentProjectId">
        <div class="ds-workspace">
          <el-tabs
            v-if="showSplitTabs"
            v-model="narrowPaneTab"
            class="ds-split-tabs"
            @tab-change="onNarrowPaneTabChange"
          >
            <el-tab-pane
              v-for="paneId in workspacePaneIds"
              :key="paneId"
              :label="workspacePaneLabel(paneId)"
              :name="paneId"
            />
          </el-tabs>
          <div
            class="ds-split"
            :class="splitContainerClasses"
          >
          <template v-for="(paneId, index) in workspacePaneIds" :key="paneId">
            <section
              v-show="!showSplitTabs || narrowPaneTab === paneId"
              class="ds-pane"
              :class="{ 'ds-pane--focused': focusedPaneId === paneId }"
              @mousedown="focusPane(paneId)"
            >
              <GlobalAIAssistant
                layout="page"
                :pane-id="paneId"
                :pane-focused="focusedPaneId === paneId"
                :show-add-pane="paneId === lastWorkspacePaneId"
                :can-add-pane="canAddPane"
                @add-pane="onAddPane"
              />
            </section>
            <div
              v-if="splitMode && !showSplitTabs && index < workspacePaneIds.length - 1"
              class="ds-split-divider"
              aria-hidden="true"
            />
          </template>
          </div>
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
import { ChatLineRound, Plus } from '@element-plus/icons-vue'
import GlobalAIAssistant from '../components/GlobalAIAssistant.vue'
import AISessionDetailDrawer from '../components/AISessionDetailDrawer.vue'
import SessionListItem from '../components/SessionListItem.vue'
import WorkspaceLayout from '../components/WorkspaceLayout.vue'
import {
  MAX_WORKSPACE_PANES,
  addWorkspacePane,
  anyPaneStreaming,
  assistantState,
  bindProject,
  isSessionOpenInOtherPane,
  isSessionOpenInPane,
  newSession,
  removeSession,
  renameSession,
  selectSession,
  setFocusedPane,
  workspacePaneLabel,
  workspacePaneTag,
} from '../stores/aiAssistant'
import { projectState } from '../utils/currentProject'

export default {
  name: 'AIAssistantPage',
  components: {
    GlobalAIAssistant,
    AISessionDetailDrawer,
    SessionListItem,
    WorkspaceLayout,
    ChatLineRound,
    Plus,
  },
  data() {
    return {
      sessionsLoading: false,
      sessionDetailVisible: false,
      sessionDetailId: '',
      viewportWidth: typeof window !== 'undefined' ? window.innerWidth : 1200,
      narrowPaneTab: 'w0',
    }
  },
  computed: {
    state() {
      return assistantState
    },
    currentProjectId() {
      return projectState.currentProjectId
    },
    workspacePaneIds() {
      return assistantState.workspacePaneIds
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
      return workspacePaneLabel(this.focusedPaneId) || workspacePaneLabel('w0')
    },
    workspaceBusy() {
      return anyPaneStreaming(this.workspacePaneIds)
    },
    canAddPane() {
      return !!this.currentProjectId && this.workspacePaneIds.length < MAX_WORKSPACE_PANES
    },
    lastWorkspacePaneId() {
      const ids = this.workspacePaneIds
      return ids.length ? ids[ids.length - 1] : ''
    },
    splitContainerClasses() {
      return {
        'ds-split--multi': this.splitMode && !this.showSplitTabs,
        'ds-split--row': true,
      }
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
      if (this.workspacePaneIds.includes(value)) this.narrowPaneTab = value
    },
    workspacePaneIds: {
      immediate: true,
      handler(ids) {
        if (ids.includes(this.focusedPaneId)) {
          this.narrowPaneTab = this.focusedPaneId
        } else if (ids.length) {
          this.narrowPaneTab = ids[0]
        }
      },
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
    workspacePaneLabel,
    onNarrowPaneTabChange(name) {
      if (this.workspacePaneIds.includes(name)) setFocusedPane(name)
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
    onAddPane() {
      if (addWorkspacePane()) {
        const lastId = this.workspacePaneIds[this.workspacePaneIds.length - 1]
        setFocusedPane(lastId)
      }
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
      return this.workspacePaneIds.some((id) => isSessionOpenInPane(sessionId, id))
    },
    isSessionInFocusedPane(sessionId) {
      return isSessionOpenInPane(sessionId, this.focusedPaneId)
    },
    sessionPaneTags(sessionId) {
      const tags = []
      this.workspacePaneIds.forEach((paneId) => {
        if (isSessionOpenInPane(sessionId, paneId)) {
          const tag = workspacePaneTag(paneId)
          if (tag) tags.push(tag)
        }
      })
      return tags
    },
    async onNewSession() {
      await newSession('', this.focusedPaneId)
    },
    async onSelectSession(id) {
      if (isSessionOpenInOtherPane(id, this.focusedPaneId)) {
        ElMessage.warning('该会话已在其他分屏中打开，请先切换或关闭对应分屏')
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
  text-decoration: none;
  color: inherit;
  cursor: pointer;
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

.ds-workspace {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ds-split {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ds-split--row {
  flex-direction: row;
  align-items: stretch;
}

.ds-split--multi {
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

.ds-split--multi .ds-pane {
  flex: 1 1 0;
  width: 0;
}

.ds-main :deep(.ai-panel-host--page) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
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
