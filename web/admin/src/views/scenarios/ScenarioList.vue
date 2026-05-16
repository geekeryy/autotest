<template>
  <WorkspaceLayout class="scenario-workspace" sidebar-width="260px" :breakpoint="1100" narrow-mode="stack">
    <template #sidebar>
    <div class="scenario-sidebar">
      <div class="sidebar-header">
        <el-select
          v-model="serviceId"
          class="service-select"
          :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
          :disabled="!projectId"
          filterable
          @change="onServiceChange"
        >
          <el-option v-for="svc in services" :key="svc.id" :label="svc.name" :value="svc.id" />
        </el-select>
        <el-button
          type="primary"
          size="small"
          :disabled="!serviceId"
          @click="openCreateDialog"
        >新建</el-button>
      </div>

      <div class="scenario-list">
        <div
          v-for="sc in scenarios"
          :key="sc.id"
          class="scenario-item"
          :class="{ active: activeTabId === sc.id, 'has-tab': tabs.some((t) => t.id === sc.id) }"
          @click="openScenario(sc)"
        >
          <span class="scenario-name">{{ sc.name }}</span>
          <el-dropdown trigger="click" @command="(cmd) => handleScenarioCommand(cmd, sc)" @click.stop>
            <el-icon class="scenario-action"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">编辑信息</el-dropdown-item>
                <el-dropdown-item command="delete" class="danger-item">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div v-if="!scenarios.length && serviceId" class="empty-tip">暂无场景，点击新建</div>
        <div v-if="!serviceId" class="empty-tip">请先选择服务</div>
      </div>
    </div>
    </template>

    <div class="scenario-main">
      <template v-if="tabs.length">
        <el-tabs
          v-model="activeTabId"
          type="card"
          closable
          class="scenario-tabs"
          @tab-change="onTabChange"
          @tab-remove="closeTab"
        >
          <el-tab-pane v-for="t in tabs" :key="t.id" :name="t.id">
            <template #label>
              <span class="scenario-tab-label" :title="t.name">{{ t.name }}</span>
            </template>
          </el-tab-pane>
        </el-tabs>

        <ScenarioEditor
          v-for="t in tabs"
          :key="t.id"
          v-show="t.id === activeTabId"
          :scenario="scenarioById(t.id)"
          :active="t.id === activeTabId"
          :cases="cases"
          :service-endpoints="serviceEndpoints"
          :environments="environments"
          :data-sources="dataSources"
          :project-id="projectId"
          :service-id="serviceId"
        />
      </template>

      <div v-else class="placeholder">
        <el-empty :description="serviceId ? '请从左侧选择场景开始编辑' : '请先选择服务'" />
      </div>
    </div>

    <!-- 新建/编辑场景对话框 -->
    <el-dialog v-model="scenarioDialogVisible" :title="editingScenario ? '编辑场景' : '新建场景'" width="480px">
      <el-form :model="scenarioForm" label-width="70px">
        <el-form-item label="名称" required>
          <el-input v-model="scenarioForm.name" placeholder="场景名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="scenarioForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="scenarioDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitScenario">保存</el-button>
      </template>
    </el-dialog>
  </WorkspaceLayout>
</template>

<script>
import { MoreFilled } from '@element-plus/icons-vue'
import {
  createScenario,
  deleteScenario,
  listCases,
  listDataSources,
  listEndpoints,
  listScenarios,
  listServiceEnvironments,
  listServices,
  updateScenario
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { enrichPage } from '../../stores/aiAssistant'
import {
  SCENARIO_STEP_WORKSPACE_KEY,
  SCENARIO_WORKSPACE_KEY,
  buildScopeKey,
  clearTabStateByScopeKey,
  readTabState,
  writeTabState
} from '../../utils/workspaceTabs'
import WorkspaceLayout from '../../components/WorkspaceLayout.vue'
import ScenarioEditor from './ScenarioEditor.vue'

export default {
  name: 'ScenarioList',
  components: {
    MoreFilled,
    ScenarioEditor,
    WorkspaceLayout,
  },

  data() {
    return {
      services: [],
      serviceId: '',
      serviceEndpoints: [],
      scenarios: [],
      cases: [],
      environments: [],
      dataSources: [],

      tabs: [],
      activeTabId: '',

      scenarioDialogVisible: false,
      editingScenario: null,
      scenarioForm: { name: '', description: '' },

      loadSeq: 0,
      _persistTimer: null,
      _restoringScope: false,
      _initializing: true
    }
  },

  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    scopeKey() {
      return this.projectId && this.serviceId ? `${this.projectId}:${this.serviceId}` : ''
    }
  },

  watch: {
    projectId(newVal, oldVal) {
      if (this._initializing || newVal === oldVal) return
      this.loadServices()
    },
    scopeKey(newKey, oldKey) {
      // 跨 scope 切换：先持久化旧 scope，清空内存 tabs；新 scope 由 onServiceChange 末尾还原
      if (this._initializing || newKey === oldKey) return
      if (oldKey) {
        const [oldProject, oldService] = oldKey.split(':')
        writeTabState(SCENARIO_WORKSPACE_KEY, oldProject, oldService, {
          tabs: this.tabs.map((t) => ({ id: t.id, name: t.name })),
          activeTabId: this.activeTabId
        })
      }
      this.tabs = []
      this.activeTabId = ''
    },
    '$route.params.scenarioID'(id) {
      if (this._initializing) return
      if (!id) return
      const sc = this.scenarios.find((s) => s.id === id)
      if (sc) {
        this.openScenario(sc, { sync: false })
        enrichPage({ scenarioName: sc.name, serviceId: this.serviceId })
      }
    },
    serviceId(id) {
      enrichPage({ serviceId: id || undefined })
    }
  },

  async created() {
    this._initializing = true
    try {
      await loadGlobalProjects()
      await this.loadServices()
    } finally {
      this._initializing = false
    }
  },

  beforeUnmount() {
    if (this._persistTimer) {
      clearTimeout(this._persistTimer)
      this._persistTimer = null
    }
    // 退出页面时把当前 scope 落盘
    if (this.scopeKey) {
      const [p, s] = this.scopeKey.split(':')
      writeTabState(SCENARIO_WORKSPACE_KEY, p, s, {
        tabs: this.tabs.map((t) => ({ id: t.id, name: t.name })),
        activeTabId: this.activeTabId
      })
    }
  },

  methods: {
    async loadServices() {
      const seq = ++this.loadSeq
      this.serviceId = ''
      this.scenarios = []
      this.cases = []
      this.environments = []
      this.serviceEndpoints = []
      this.dataSources = []
      this.tabs = []
      this.activeTabId = ''
      this.services = this.projectId ? await listServices(this.projectId) : []
      if (seq !== this.loadSeq) return
      if (this.services[0]) {
        this.serviceId = this.services[0].id
        await this.onServiceChange()
      }
    },

    async onServiceChange() {
      const seq = ++this.loadSeq
      this.scenarios = []
      this.cases = []
      this.environments = []
      this.serviceEndpoints = []
      this.dataSources = []
      this.tabs = []
      this.activeTabId = ''
      if (!this.projectId || !this.serviceId) return
      const params = { projectId: this.projectId, serviceId: this.serviceId }
      const [scenarios, cases, envs, endpointsRaw] = await Promise.all([
        listScenarios(params),
        listCases(params),
        listServiceEnvironments(this.projectId, this.serviceId),
        listEndpoints(this.projectId, this.serviceId)
      ])
      if (seq !== this.loadSeq) return
      this.scenarios = scenarios
      this.cases = cases
      this.serviceEndpoints = Array.isArray(endpointsRaw) ? endpointsRaw : []
      this.environments = envs
      await this.loadDataSources()
      if (seq !== this.loadSeq) return
      this.restoreTabsForScope()
      // 处理 URL 中的 :scenarioID（覆盖还原结果）
      const urlId = this.$route.params.scenarioID
      if (urlId) {
        const sc = this.scenarios.find((s) => s.id === urlId)
        if (sc) {
          this.openScenario(sc, { sync: false })
          enrichPage({ scenarioName: sc.name, serviceId: this.serviceId })
        }
      }
    },

    async loadDataSources() {
      if (!this.projectId) {
        this.dataSources = []
        return
      }
      this.dataSources = await listDataSources({ projectId: this.projectId })
    },

    scenarioById(id) {
      return this.scenarios.find((s) => s.id === id) || { id, name: '' }
    },

    openScenario(sc, options = {}) {
      if (!sc?.id) return
      const existing = this.tabs.find((t) => t.id === sc.id)
      if (!existing) {
        this.tabs.push({ id: sc.id, name: sc.name })
      } else if (existing.name !== sc.name) {
        existing.name = sc.name
      }
      this.activeTabId = sc.id
      if (options.sync !== false) this.syncRoute(sc.id)
      this.persistTabsForScope()
    },

    closeTab(scenarioId) {
      const idx = this.tabs.findIndex((t) => t.id === scenarioId)
      if (idx < 0) return
      const wasActive = this.activeTabId === scenarioId
      this.tabs.splice(idx, 1)
      if (wasActive) {
        const next = this.tabs[idx] || this.tabs[idx - 1]
        this.activeTabId = next?.id || ''
        this.syncRoute(this.activeTabId)
      }
      this.clearStepTabsForScenario(scenarioId)
      this.persistTabsForScope()
    },

    // 关闭场景 Tab 或删除场景时，必须清掉该场景在浏览器本地保存的步骤 Tab 列表，
    // 否则下次再次打开同一场景时会还原已被用户主动关闭的旧步骤 Tab。
    // ScenarioEditor 在 beforeUnmount 还会做一次兜底持久化，这里用 nextTick 延后到卸载之后再清。
    clearStepTabsForScenario(scenarioId) {
      if (!scenarioId) return
      const scopeKey = buildScopeKey(this.projectId, this.serviceId, scenarioId)
      if (!scopeKey) return
      this.$nextTick(() => {
        clearTabStateByScopeKey(SCENARIO_STEP_WORKSPACE_KEY, scopeKey)
      })
    },

    onTabChange(activeId) {
      this.syncRoute(activeId)
      this.persistTabsForScope()
    },

    syncRoute(activeId) {
      const target = activeId ? `/scenarios/${activeId}` : '/scenarios'
      if (this.$route.path !== target) {
        this.$router.replace(target).catch(() => {})
      }
    },

    restoreTabsForScope() {
      if (!this.scopeKey) return
      this._restoringScope = true
      try {
        const stored = readTabState(SCENARIO_WORKSPACE_KEY, this.projectId, this.serviceId)
        const existingIds = new Set(this.scenarios.map((s) => s.id))
        const restored = stored.tabs
          .filter((t) => existingIds.has(t.id))
          .map((t) => {
            const live = this.scenarios.find((s) => s.id === t.id)
            return { id: t.id, name: live?.name || t.name }
          })
        this.tabs = restored
        this.activeTabId = existingIds.has(stored.activeTabId)
          ? stored.activeTabId
          : (restored[0]?.id || '')
        if (this.activeTabId) this.syncRoute(this.activeTabId)
      } finally {
        this._restoringScope = false
      }
    },

    persistTabsForScope() {
      if (this._restoringScope || !this.scopeKey) return
      if (this._persistTimer) clearTimeout(this._persistTimer)
      this._persistTimer = setTimeout(() => {
        this._persistTimer = null
        if (!this.scopeKey) return
        writeTabState(SCENARIO_WORKSPACE_KEY, this.projectId, this.serviceId, {
          tabs: this.tabs.map((t) => ({ id: t.id, name: t.name })),
          activeTabId: this.activeTabId
        })
      }, 150)
    },

    openCreateDialog() {
      this.editingScenario = null
      this.scenarioForm = { name: '', description: '' }
      this.scenarioDialogVisible = true
    },

    handleScenarioCommand(cmd, sc) {
      if (cmd === 'edit') {
        this.editingScenario = sc
        this.scenarioForm = { name: sc.name, description: sc.description || '' }
        this.scenarioDialogVisible = true
      } else if (cmd === 'delete') {
        this.$confirm(`确认删除场景「${sc.name}」？此操作不可恢复。`, '删除场景', {
          type: 'warning',
          confirmButtonText: '删除',
          confirmButtonClass: 'el-button--danger'
        }).then(async () => {
          await deleteScenario(sc.id)
          this.$message.success('场景已删除')
          this.scenarios = this.scenarios.filter((s) => s.id !== sc.id)
          if (this.tabs.some((t) => t.id === sc.id)) {
            this.closeTab(sc.id)
          } else {
            this.clearStepTabsForScenario(sc.id)
          }
        }).catch(() => {})
      }
    },

    async submitScenario() {
      if (!this.scenarioForm.name.trim()) {
        this.$message.warning('请输入场景名称')
        return
      }
      if (this.editingScenario) {
        const updated = await updateScenario(this.editingScenario.id, this.scenarioForm)
        const idx = this.scenarios.findIndex((s) => s.id === updated.id)
        if (idx !== -1) this.scenarios.splice(idx, 1, updated)
        const tab = this.tabs.find((t) => t.id === updated.id)
        if (tab && tab.name !== updated.name) {
          tab.name = updated.name
          this.persistTabsForScope()
        }
      } else {
        const sc = await createScenario({
          ...this.scenarioForm,
          projectId: this.projectId,
          serviceId: this.serviceId
        })
        this.scenarios.push(sc)
        this.openScenario(sc)
      }
      this.$message.success('保存成功')
      this.scenarioDialogVisible = false
    }
  }
}
</script>

<style scoped>
.scenario-workspace {
  height: 100%;
  overflow: hidden;
}

/* Left sidebar */
.scenario-sidebar {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--el-bg-color);
}

.sidebar-header {
  display: flex;
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid var(--el-border-color-light);
  align-items: center;
}

.service-select {
  flex: 1;
  min-width: 0;
}

.scenario-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.scenario-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  cursor: pointer;
  border-radius: 4px;
  margin: 0 6px 2px;
  transition: background 0.15s;
}

.scenario-item:hover {
  background: var(--el-fill-color-light);
}

.scenario-item.has-tab {
  font-weight: 600;
}

.scenario-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.scenario-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--app-font-size-base);
}

.scenario-action {
  opacity: 0.45;
  cursor: pointer;
  color: var(--el-text-color-secondary);
}

.scenario-item:hover .scenario-action {
  opacity: 1;
}

/* Main area */
.scenario-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

.scenario-tabs {
  flex: 0 0 auto;
  padding: 8px 12px 0;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
}

.scenario-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.scenario-tabs :deep(.el-tabs__content) {
  display: none;
}

.scenario-tab-label {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}

.placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-tip {
  text-align: center;
  color: var(--el-text-color-secondary);
  font-size: var(--app-font-size-small);
  padding: 24px;
}

:deep(.danger-item) {
  color: var(--el-color-danger);
}
</style>
