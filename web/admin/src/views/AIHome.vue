<template>
  <div class="ai-home">
    <header class="ai-home__header">
      <div class="ai-home__header-left">
        <BrandLogo class="ai-home__logo" />
        <span class="ai-home__brand">Autotest</span>
      </div>
      <div class="ai-home__header-right">
        <div v-if="currentProjectId" class="ai-home__project">
          <span class="ai-home__project-label">项目</span>
          <el-select
            v-model="projectId"
            class="ai-home__project-select"
            placeholder="请选择项目"
            filterable
            :loading="projectState.loading"
          >
            <el-option v-for="p in projectState.projects" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </div>
        <el-button type="primary" plain @click="goConsole">
          <el-icon><Monitor /></el-icon>
          <span>进入控制台</span>
        </el-button>
        <el-dropdown trigger="click" @command="onUserMenuCommand">
          <el-avatar :size="36" :src="avatarSrc" :style="avatarBgStyle" class="ai-home__avatar">
            {{ userInitials }}
          </el-avatar>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>{{ userName }}</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <WorkspaceLayout
      class="ai-home__workspace"
      :breakpoint="900"
      narrow-mode="drawer"
      drawer-trigger-label="会话列表"
      sidebar-width="260px"
    >
      <template #sidebar>
        <aside class="ai-home__sidebar" :class="{ 'ai-home__sidebar--disabled': !currentProjectId }">
          <el-button
            class="ai-home__new-chat"
            :disabled="!currentProjectId || workspaceBusy"
            @click="onNewSession"
          >
            <el-icon><Plus /></el-icon>
            <span>新对话</span>
          </el-button>
          <div class="ai-home__session-list" v-loading="sessionsLoading">
            <p v-if="!currentProjectId" class="ai-home__session-empty">请先在顶栏选择项目</p>
            <p v-else-if="!state.sessions.length" class="ai-home__session-empty">暂无历史会话</p>
            <SessionListItem
              v-for="session in state.sessions"
              :key="session.id"
              :session="session"
              :active="isSessionActive(session.id)"
              :focused="isSessionActive(session.id)"
              @select="() => onSelectSession(session.id)"
              @detail="onSessionDetail"
              @rename="onRenameSession"
              @delete="onDeleteSession"
            />
          </div>
        </aside>
      </template>

      <div v-if="currentProjectId" class="ai-home__main">
        <AIScenarioGenLauncher
          v-if="genLauncherOpen"
          @close="genLauncherOpen = false"
          @submit="onGenLauncherSubmit"
        />
        <GlobalAIAssistant
          layout="page"
          pane-id="w0"
          variant="home"
          :prompt-suggestions="homePromptSuggestions"
          @prompt-action="onPromptAction"
        />
      </div>
      <div v-else class="ai-home__empty">
        <div class="ai-home__empty-icon">
          <el-icon :size="52"><ChatLineRound /></el-icon>
        </div>
        <p class="ai-home__empty-title">选择项目后开始对话</p>
        <p class="ai-home__empty-desc">在顶栏选择测试项目，即可使用 AI 生成场景、查询接口与编排测试。</p>
      </div>
    </WorkspaceLayout>

    <AISessionDetailDrawer
      v-model="sessionDetailVisible"
      :project-id="currentProjectId"
      :session-id="sessionDetailId"
    />
  </div>
</template>

<script>
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Monitor, Plus } from '@element-plus/icons-vue'
import { logout } from '../api'
import { authState, resetCurrentUser } from '../auth'
import BrandLogo from '../components/BrandLogo.vue'
import GlobalAIAssistant from '../components/GlobalAIAssistant.vue'
import AISessionDetailDrawer from '../components/AISessionDetailDrawer.vue'
import AIScenarioGenLauncher from '../components/AIScenarioGenLauncher.vue'
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
  sendMessage,
} from '../stores/aiAssistant'
import { clearToken } from '../utils/storage'
import { loadGlobalProjects, projectState, setCurrentProjectId } from '../utils/currentProject'
import { getAvatarBgStyle, getInitials } from '../utils/useInitialsColor'

const GEN_PROMPT_KEY = '__gen_scenarios__'

const HOME_PROMPT_SUGGESTIONS = [
  '列出当前项目下所有场景',
  '查询某个接口的请求模板与断言',
  { label: '生成并验证测试场景', action: GEN_PROMPT_KEY },
  '帮我生成一个登录后下单的测试场景',
]

export default {
  name: 'AIHome',
  components: {
    BrandLogo,
    GlobalAIAssistant,
    AISessionDetailDrawer,
    AIScenarioGenLauncher,
    SessionListItem,
    WorkspaceLayout,
    ChatLineRound,
    Monitor,
    Plus,
  },
  data() {
    return {
      sessionsLoading: false,
      sessionDetailVisible: false,
      sessionDetailId: '',
      genLauncherOpen: false,
      homePromptSuggestions: HOME_PROMPT_SUGGESTIONS,
    }
  },
  computed: {
    state() {
      return assistantState
    },
    projectState() {
      return projectState
    },
    currentProjectId() {
      return projectState.currentProjectId
    },
    projectId: {
      get() {
        return projectState.currentProjectId
      },
      set(value) {
        setCurrentProjectId(value)
      },
    },
    workspaceBusy() {
      return anyPaneStreaming(['w0'])
    },
    userName() {
      return authState.user?.displayName || authState.user?.username || '管理员'
    },
    avatarSrc() {
      const u = authState.user
      return u?.avatarUrl ? String(u.avatarUrl) : ''
    },
    avatarBgStyle() {
      const u = authState.user
      return getAvatarBgStyle(u?.id || u?.username, !!this.avatarSrc)
    },
    userInitials() {
      const u = authState.user
      return getInitials(u?.displayName || u?.username, '管')
    },
  },
  watch: {
    currentProjectId: {
      immediate: true,
      handler(projectId) {
        this.bindCurrentProject(projectId)
      },
    },
  },
  created() {
    loadGlobalProjects()
  },
  methods: {
    goConsole() {
      this.$router.push('/dashboard')
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
    isSessionActive(sessionId) {
      return isSessionOpenInPane(sessionId, 'w0')
    },
    async onNewSession() {
      await newSession('', 'w0')
    },
    async onSelectSession(id) {
      await selectSession(id, 'w0')
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
        // cancelled
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
    onPromptAction(action) {
      if (action === GEN_PROMPT_KEY) {
        this.genLauncherOpen = true
      }
    },
    async onGenLauncherSubmit(config) {
      this.genLauncherOpen = false
      const message = [
        '请调用 generate_and_verify_scenarios 工具，为我生成并验证测试场景。',
        `serviceId: "${config.serviceId}"`,
        `environmentId: "${config.environmentId}"`,
        `autoRepair: ${config.autoRepair}`,
        `maxRounds: ${config.maxRounds}`,
        config.loginCredentials && Object.keys(config.loginCredentials).length
          ? `loginCredentials: ${JSON.stringify(config.loginCredentials)}`
          : 'loginCredentials: 请在确认面板填写（若已有 saved_case 候选可先 list_scenario_login_hints）',
        config.serviceName ? `（服务：${config.serviceName}，环境：${config.environmentName}）` : '',
      ].filter(Boolean).join('\n')
      await sendMessage(message, 'w0')
    },
    async onUserMenuCommand(cmd) {
      if (cmd !== 'logout') return
      try {
        await logout()
      } finally {
        clearToken()
        resetCurrentUser()
        this.$router.replace('/login')
      }
    },
  },
}
</script>

<style scoped>
.ai-home {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--app-surface-subtle, #f7f8fa);
  overflow: hidden;
}

.ai-home__header {
  flex-shrink: 0;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 20px;
  background: var(--app-card-bg, #fff);
  border-bottom: 1px solid var(--app-border-color, #e8eaed);
}

.ai-home__header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ai-home__logo {
  --brand-logo-size: 28px;
}

.ai-home__brand {
  font-size: 16px;
  font-weight: 700;
  color: var(--app-text-color);
  letter-spacing: 0.02em;
}

.ai-home__header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.ai-home__project {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.ai-home__project-label {
  font-size: 13px;
  color: var(--app-secondary-text);
  white-space: nowrap;
}

.ai-home__project-select {
  width: 200px;
}

.ai-home__avatar {
  cursor: pointer;
}

.ai-home__workspace {
  flex: 1 1 auto;
  min-height: 0;
}

.ai-home__sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #e8eaed;
}

.ai-home__sidebar--disabled {
  opacity: 0.92;
}

.ai-home__new-chat {
  margin: 14px 12px 10px;
  width: calc(100% - 24px);
  height: 40px;
  border-radius: 10px;
  border: 1px solid #dce0e6;
  background: #fff;
  font-weight: 500;
  justify-content: flex-start;
  gap: 8px;
  padding-left: 14px;
}

.ai-home__session-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 0 8px 8px;
}

.ai-home__session-empty {
  margin: 12px 8px;
  font-size: 13px;
  color: var(--app-text-muted);
}

.ai-home__main {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}

.ai-home__main :deep(.ai-panel-host--page) {
  flex: 1 1 auto;
  min-height: 0;
}

.ai-home__main :deep(.gen-launcher) {
  position: absolute;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
}

.ai-home__empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 32px;
  text-align: center;
}

.ai-home__empty-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  color: var(--el-color-primary);
  background: #fff;
  box-shadow: 0 8px 32px rgba(15, 23, 42, 0.06);
}

.ai-home__empty-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.ai-home__empty-desc {
  margin: 0;
  max-width: 380px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--app-text-muted);
}

@media (max-width: 768px) {
  .ai-home__header {
    flex-wrap: wrap;
    height: auto;
    padding: 10px 12px;
  }

  .ai-home__header-right {
    flex-wrap: wrap;
    width: 100%;
    justify-content: flex-end;
  }

  .ai-home__project-select {
    width: 160px;
  }
}
</style>
