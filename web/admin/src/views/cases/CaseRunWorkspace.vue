<template>
  <div class="run-workspace">
    <aside class="api-sidebar" :style="sidebarStyle">
      <div class="sidebar-header">
        <div>
          <h2>API 列表</h2>
        </div>
        <el-button :loading="treeLoading" circle text title="刷新 API 列表" @click="loadTree">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>

      <el-select
        v-model="selectedServiceId"
        class="workspace-service-select"
        filterable
        placeholder="选择服务"
        :disabled="!projectId || !services.length || treeLoading"
        @change="onWorkspaceServiceChange"
      >
        <el-option v-for="s in services" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>

      <el-input v-model="treeKeyword" class="tree-search" clearable placeholder="搜索接口或用例" prefix-icon="Search" />

      <el-skeleton v-if="treeLoading" :rows="8" animated />
      <el-empty v-else-if="!projectId" description="请先在顶部选择项目" />
      <el-empty v-else-if="projectId && !services.length" description="当前项目暂无服务" />
      <el-empty v-else-if="!treeData.length" description="当前服务暂无 API 或用例" />
      <el-tree
        v-else
        class="api-tree"
        :data="filteredTreeData"
        :props="treeProps"
        node-key="id"
        default-expand-all
        highlight-current
        empty-text="未匹配到 API 或用例"
        @node-click="handleTreeNodeClick"
      >
        <template #default="{ data }">
          <span class="tree-node" :class="{ 'is-case': data.type === 'case', 'is-tag': data.type === 'tag', 'is-empty': data.disabled }">
            <span class="tree-node-main">
              <el-tag v-if="data.method" class="method-tag" size="small" :type="methodType(data.method)">
                {{ data.method }}
              </el-tag>
              <span class="tree-node-title">{{ data.label }}</span>
            </span>
            <span v-if="data.caseCount" class="tree-node-count">{{ data.caseCount }}</span>
          </span>
        </template>
      </el-tree>
    </aside>

    <div
      v-show="!layoutNarrow"
      class="sidebar-resizer"
      role="separator"
      :aria-valuenow="sidebarWidthPx"
      aria-valuemin="200"
      aria-valuemax="720"
      aria-orientation="vertical"
      aria-label="调整 API 列表宽度"
      tabindex="0"
      @mousedown.prevent="startSidebarResize"
      @keydown.left.prevent="nudgeSidebar(-16)"
      @keydown.right.prevent="nudgeSidebar(16)"
    />

    <section class="run-console">
      <div class="console-header">
        <div>
          <h2>运行控制台</h2>
          <p>从左侧 API 树打开多个用例后，可在横向标签中切换运行。</p>
        </div>
        <div id="run-console-environment-control" class="console-controls"></div>
      </div>

      <el-empty v-if="!tabs.length" class="console-empty" description="请选择左侧 API 下的用例开始运行" />
      <template v-else>
        <el-tabs
          v-model="activeCaseId"
          class="console-tabs"
          type="card"
          closable
          @tab-change="syncRoute"
          @tab-remove="closeTab"
        >
          <el-tab-pane v-for="tab in tabs" :key="tab.id" :name="tab.id">
            <template #label>
              <span class="run-tab-label">
                <span class="tab-title">{{ tab.name }}</span>
              </span>
            </template>
          </el-tab-pane>
        </el-tabs>

        <div class="console-panels">
          <CaseRun
            v-for="tab in tabs"
            :key="tab.id"
            v-show="tab.id === activeCaseId"
            :case-id="tab.id"
            :active="tab.id === activeCaseId"
            embedded
            @status-change="updateTabStatus(tab.id, $event)"
          />
        </div>
      </template>
    </section>
  </div>
</template>

<script>
import { getCase, listCases, listEndpoints, listServices } from '../../api'
import { loadGlobalProjects, projectState, setCurrentProjectId } from '../../utils/currentProject'
import { persistServiceId, readStoredServiceId } from '../../utils/serviceSelection'
import CaseRun from './CaseRun.vue'

const SIDEBAR_WIDTH_STORAGE_KEY = 'autotest.runConsoleApiSidebarWidth'
const SIDEBAR_MIN = 200
const SIDEBAR_MAX = 720
const SIDEBAR_DEFAULT = 320
const LAYOUT_NARROW_MQ = '(max-width: 1100px)'

function emptySummary() {
  return { running: false, runStatus: '', resultStatus: '', durationMillis: null }
}

export default {
  name: 'CaseRunWorkspace',
  components: { CaseRun },
  data() {
    return {
      initializing: false,
      treeLoading: false,
      treeKeyword: '',
      services: [],
      selectedServiceId: '',
      treeData: [],
      treeProps: { label: 'label', children: 'children', disabled: 'disabled' },
      tabs: [],
      activeCaseId: '',
      loadSeq: 0,
      sidebarWidthPx: SIDEBAR_DEFAULT,
      layoutNarrow: false,
      layoutMediaQuery: null,
      _sidebarResize: null
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    currentProjectName() {
      const project = projectState.projects.find((item) => item.id === this.projectId)
      return project?.name || '未选择项目'
    },
    filteredTreeData() {
      const keyword = this.treeKeyword.trim().toLowerCase()
      if (!keyword) return this.treeData
      return this.filterTree(this.treeData, keyword)
    },
    sidebarStyle() {
      if (this.layoutNarrow) return {}
      const w = this.sidebarWidthPx
      return {
        width: `${w}px`,
        flex: `0 0 ${w}px`
      }
    }
  },
  watch: {
    projectId(newValue, oldValue) {
      if (this.initializing || newValue === oldValue) return
      this.tabs = []
      this.activeCaseId = ''
      this.loadTree()
      this.syncRoute('')
    },
    '$route.params.caseID'(caseID) {
      if (!caseID || this.tabs.some((tab) => tab.id === caseID)) {
        if (caseID) this.activeCaseId = caseID
        return
      }
      this.openCaseById(caseID, { sync: false })
    }
  },
  async created() {
    this.initializing = true
    try {
      await loadGlobalProjects()
      const initialCaseId = this.$route.params.caseID
      let initialCase = null
      if (initialCaseId) {
        try {
          initialCase = await getCase(initialCaseId)
        } catch {
          // 用例不存在或无权限时已在拦截器中提示，仍加载 API 树
        }
        if (initialCase?.projectId && initialCase.projectId !== this.projectId) {
          setCurrentProjectId(initialCase.projectId)
        }
      }
      await this.loadTree()
      if (initialCase) this.openCase(initialCase, { sync: false })
    } finally {
      this.initializing = false
    }
    this.initSidebarWidth()
    this.initLayoutMediaQuery()
  },
  beforeUnmount() {
    this.detachSidebarResizeListeners()
    if (this.layoutMediaQuery) {
      this.layoutMediaQuery.removeEventListener('change', this.onLayoutMediaChange)
      this.layoutMediaQuery = null
    }
  },
  methods: {
    initSidebarWidth() {
      try {
        const raw = localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)
        const w = raw != null ? Number.parseInt(raw, 10) : NaN
        if (!Number.isNaN(w)) {
          this.sidebarWidthPx = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, w))
        }
      } catch {
        /* ignore */
      }
    },
    persistSidebarWidth() {
      try {
        localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, String(this.sidebarWidthPx))
      } catch {
        /* ignore */
      }
    },
    initLayoutMediaQuery() {
      if (typeof window === 'undefined' || !window.matchMedia) return
      this.layoutMediaQuery = window.matchMedia(LAYOUT_NARROW_MQ)
      this.layoutNarrow = this.layoutMediaQuery.matches
      this.layoutMediaQuery.addEventListener('change', this.onLayoutMediaChange)
    },
    onLayoutMediaChange() {
      if (this.layoutMediaQuery) this.layoutNarrow = this.layoutMediaQuery.matches
    },
    clampSidebarWidth(value) {
      return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, value))
    },
    nudgeSidebar(delta) {
      if (this.layoutNarrow) return
      this.sidebarWidthPx = this.clampSidebarWidth(this.sidebarWidthPx + delta)
      this.persistSidebarWidth()
    },
    startSidebarResize(downEvent) {
      if (this.layoutNarrow) return
      const startX = downEvent.clientX
      const startW = this.sidebarWidthPx
      const onMove = (e) => {
        const dx = e.clientX - startX
        this.sidebarWidthPx = this.clampSidebarWidth(startW + dx)
      }
      const onUp = () => {
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        this.persistSidebarWidth()
      }
      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      this._sidebarResize = { onMove, onUp }
    },
    detachSidebarResizeListeners() {
      if (!this._sidebarResize) return
      document.removeEventListener('mousemove', this._sidebarResize.onMove)
      document.removeEventListener('mouseup', this._sidebarResize.onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      this._sidebarResize = null
    },
    async loadTree() {
      const projectId = this.projectId
      const seq = ++this.loadSeq
      if (!projectId) {
        this.services = []
        this.selectedServiceId = ''
        this.treeData = []
        return
      }

      this.treeLoading = true
      try {
        const services = await listServices(projectId)
        if (seq !== this.loadSeq) return
        this.services = services

        let serviceId = this.selectedServiceId
        if (!services.some((s) => s.id === serviceId)) {
          const stored = readStoredServiceId(projectId)
          serviceId = services.some((s) => s.id === stored) ? stored : services[0]?.id || ''
        }
        this.selectedServiceId = serviceId
        if (this.selectedServiceId) persistServiceId(projectId, this.selectedServiceId)

        await this.loadTreeDataForService(projectId, this.selectedServiceId, seq)
      } catch {
        if (seq !== this.loadSeq) return
        this.services = []
        this.selectedServiceId = ''
        this.treeData = []
      } finally {
        if (seq === this.loadSeq) this.treeLoading = false
      }
    },
    async loadTreeDataForService(projectId, serviceId, seq) {
      if (!serviceId) {
        this.treeData = []
        return
      }
      let endpoints
      let cases
      try {
        ;[endpoints, cases] = await Promise.all([
          listEndpoints(projectId, serviceId),
          listCases({ projectId, serviceId })
        ])
      } catch {
        if (seq !== this.loadSeq) return
        this.treeData = []
        return
      }
      if (seq !== this.loadSeq) return
      const service = this.services.find((s) => s.id === serviceId) || { id: serviceId, name: '' }
      this.treeData = this.buildServiceTreeContent({ service, endpoints, cases })
    },
    async onWorkspaceServiceChange(serviceId) {
      if (this.projectId && serviceId) persistServiceId(this.projectId, serviceId)
      const seq = ++this.loadSeq
      this.treeLoading = true
      try {
        await this.loadTreeDataForService(this.projectId, serviceId, seq)
      } finally {
        if (seq === this.loadSeq) this.treeLoading = false
      }
    },
    buildServiceTreeContent({ service, endpoints, cases }) {
      const usedCaseIds = new Set()
      const endpointNodes = endpoints.map((endpoint) => {
        const endpointCases = cases.filter((item) => {
          const byId = item.endpointId && item.endpointId === endpoint.id
          const bySignature = item.method === endpoint.method && item.path === endpoint.path
          return byId || bySignature
        })
        endpointCases.forEach((item) => usedCaseIds.add(item.id))
        const caseNodes = endpointCases.map((item) => this.buildCaseNode(item))

        if (caseNodes.length === 1) {
          const testCase = endpointCases[0]
          const baseSearch = `${endpoint.method} ${endpoint.path} ${endpoint.summary || ''} ${endpoint.operationId || ''} ${(endpoint.tags || []).join(' ')}`
          return {
            id: `case:${testCase.id}`,
            type: 'case',
            label: endpoint.summary || endpoint.path,
            method: endpoint.method,
            tagName: this.endpointTagName(endpoint),
            searchText: `${baseSearch} ${testCase.name} ${testCase.method} ${testCase.path} ${testCase.source || ''}`,
            caseCount: 1,
            case: testCase
          }
        }

        return {
          id: `endpoint:${endpoint.id}`,
          type: 'endpoint',
          label: endpoint.summary || endpoint.path,
          method: endpoint.method,
          tagName: this.endpointTagName(endpoint),
          searchText: `${endpoint.method} ${endpoint.path} ${endpoint.summary || ''} ${endpoint.operationId || ''} ${(endpoint.tags || []).join(' ')}`,
          caseCount: caseNodes.length,
          children: caseNodes.length ? caseNodes : [this.buildEmptyNode(endpoint.id)]
        }
      })

      const standaloneCases = cases.filter((item) => !usedCaseIds.has(item.id))
      if (standaloneCases.length) {
        endpointNodes.push({
          id: `standalone:${service.id}`,
          type: 'standalone',
          label: '未关联接口',
          tagName: '未关联接口',
          searchText: '未关联接口 manual standalone',
          caseCount: standaloneCases.length,
          children: standaloneCases.map((item) => this.buildCaseNode(item))
        })
      }
      return this.groupEndpointNodesByTag(service.id, endpointNodes)
    },
    endpointTagName(endpoint) {
      const tags = Array.isArray(endpoint.tags) ? endpoint.tags : []
      return tags.map((item) => String(item || '').trim()).find(Boolean) || '未分组'
    },
    groupEndpointNodesByTag(serviceId, endpointNodes) {
      const groups = new Map()
      for (const node of endpointNodes) {
        const tagName = node.tagName || '未分组'
        if (!groups.has(tagName)) {
          groups.set(tagName, {
            id: `tag:${serviceId}:${tagName}`,
            type: 'tag',
            label: tagName,
            searchText: tagName,
            caseCount: 0,
            children: []
          })
        }
        const group = groups.get(tagName)
        group.children.push(node)
        group.searchText = `${group.searchText} ${node.searchText || node.label || ''}`
        group.caseCount += node.caseCount || 0
      }
      return Array.from(groups.values()).sort((left, right) => left.label.localeCompare(right.label, 'zh-CN'))
    },
    buildCaseNode(testCase) {
      return {
        id: `case:${testCase.id}`,
        type: 'case',
        label: testCase.name,
        method: testCase.method,
        searchText: `${testCase.name} ${testCase.method} ${testCase.path} ${testCase.source || ''}`,
        case: testCase
      }
    },
    buildEmptyNode(endpointId) {
      return {
        id: `endpoint-empty:${endpointId}`,
        type: 'empty',
        label: '暂无可运行用例',
        disabled: true,
        searchText: '暂无可运行用例'
      }
    },
    handleTreeNodeClick(data) {
      if (data.disabled) return
      if (data.type === 'case') {
        this.openCase(data.case)
      }
    },
    async openCaseById(caseId, options = {}) {
      const existed = this.tabs.find((tab) => tab.id === caseId)
      if (existed) {
        this.activeCaseId = caseId
        if (options.sync !== false) this.syncRoute(caseId)
        return
      }
      const testCase = await getCase(caseId)
      if (testCase.projectId && testCase.projectId !== this.projectId) {
        setCurrentProjectId(testCase.projectId)
        await this.loadTree()
      }
      this.openCase(testCase, options)
    },
    openCase(testCase, options = {}) {
      if (!testCase?.id) return
      const existed = this.tabs.find((tab) => tab.id === testCase.id)
      if (!existed) {
        this.tabs.push({
          id: testCase.id,
          name: testCase.name,
          method: testCase.method,
          path: testCase.path,
          summary: emptySummary()
        })
      }
      this.activeCaseId = testCase.id
      if (options.sync !== false) this.syncRoute(testCase.id)
    },
    closeTab(caseId) {
      const index = this.tabs.findIndex((tab) => tab.id === caseId)
      if (index < 0) return
      const wasActive = this.activeCaseId === caseId
      this.tabs.splice(index, 1)
      if (wasActive) {
        const next = this.tabs[index] || this.tabs[index - 1]
        this.activeCaseId = next?.id || ''
        this.syncRoute(this.activeCaseId)
      }
    },
    updateTabStatus(caseId, summary) {
      const tab = this.tabs.find((item) => item.id === caseId)
      if (!tab) return
      tab.summary = { ...tab.summary, ...summary }
    },
    syncRoute(caseId) {
      const target = caseId ? `/run-console/${caseId}` : '/run-console'
      if (this.$route.path !== target) {
        this.$router.replace(target)
      }
    },
    filterTree(nodes, keyword) {
      return nodes.reduce((items, node) => {
        const children = node.children ? this.filterTree(node.children, keyword) : []
        const matched = String(node.searchText || node.label || '').toLowerCase().includes(keyword)
        if (matched || children.length) {
          items.push({ ...node, children })
        }
        return items
      }, [])
    },
    methodType(method) {
      const value = String(method || '').toUpperCase()
      if (value === 'GET') return 'success'
      if (value === 'POST') return 'primary'
      if (value === 'DELETE') return 'danger'
      if (value === 'PUT' || value === 'PATCH') return 'warning'
      return 'info'
    },
    metricClass(summary) {
      if (summary.running || summary.runStatus === 'running') return 'is-running'
      if (summary.resultStatus === 'passed' || summary.runStatus === 'passed') return 'is-success'
      if (['failed', 'error'].includes(summary.resultStatus) || ['failed', 'error'].includes(summary.runStatus)) return 'is-danger'
      return 'is-idle'
    },
    statusIcon(summary) {
      if (summary.running || summary.runStatus === 'running') return 'Loading'
      if (summary.resultStatus === 'passed' || summary.runStatus === 'passed') return 'CircleCheck'
      if (['failed', 'error'].includes(summary.resultStatus) || ['failed', 'error'].includes(summary.runStatus)) return 'CircleClose'
      return 'Clock'
    },
    statusLabel(summary) {
      if (summary.running || summary.runStatus === 'running') return '运行中'
      const status = summary.resultStatus || summary.runStatus
      const labels = { passed: '通过', failed: '失败', error: '错误', pending: '等待中' }
      return labels[status] || '未运行'
    },
    formatDuration(ms) {
      if (ms == null) return ''
      const value = Number(ms || 0)
      if (value < 1000) return `${value} ms`
      return `${(value / 1000).toFixed(2)} s`
    }
  }
}
</script>

<style scoped>
.run-workspace {
  display: flex;
  flex-direction: row;
  align-items: stretch;
  gap: 0;
  /* 随主内容区高度填充；过长时由 el-main 滚动，不再用 100vh 估算撑高整页 */
  min-height: 100%;
  padding: 2px;
  background: #f3f6fb;
}

.api-sidebar,
.run-console {
  min-height: 0;
  border: 1px solid color-mix(in srgb, var(--app-border-color) 72%, #64748b);
  border-radius: 12px;
  background: var(--app-card-bg);
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.06);
}

.api-sidebar {
  display: flex;
  flex-direction: column;
  padding: 14px;
  flex: 0 0 320px;
  width: 320px;
  background:
    linear-gradient(180deg, rgba(248, 250, 252, 0.9), rgba(255, 255, 255, 0.98)),
    var(--app-card-bg);
}

.sidebar-resizer {
  flex: 0 0 10px;
  margin: 0 4px;
  align-self: stretch;
  border-radius: 6px;
  cursor: col-resize;
  background: transparent;
  touch-action: none;
}

.sidebar-resizer:hover,
.sidebar-resizer:focus-visible {
  background: color-mix(in srgb, var(--el-color-primary) 18%, transparent);
  outline: none;
}

.sidebar-resizer:focus-visible {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--el-color-primary) 45%, transparent);
}

.sidebar-header,
.console-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.console-controls {
  display: flex;
  min-height: 32px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: flex-end;
}

.sidebar-header h2,
.console-header h2 {
  margin: 0 0 6px;
  font-size: var(--app-font-size-title);
}

.sidebar-header p,
.console-header p {
  margin: 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.workspace-service-select {
  width: 100%;
}

.tree-search {
  margin: 14px 0;
}

.api-tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px;
  border: 1px solid color-mix(in srgb, var(--app-border-color) 78%, #94a3b8);
  border-radius: 10px;
  background: rgba(248, 250, 252, 0.72);
}

.api-tree :deep(.el-tree-node__content) {
  min-height: 34px;
  border-radius: 8px;
}

.api-tree :deep(.el-tree-node__content:hover) {
  background: rgba(59, 130, 246, 0.08);
}

.api-tree :deep(.is-current > .el-tree-node__content) {
  background: rgba(59, 130, 246, 0.13);
}

.tree-node,
.tree-node-main,
.run-tab-label,
.tab-status,
.tab-duration {
  display: inline-flex;
  align-items: center;
}

.tree-node {
  width: 100%;
  min-width: 0;
  justify-content: space-between;
  gap: 8px;
}

.tree-node-main {
  flex: 1 1 0;
  min-width: 0;
  gap: 6px;
}

.tree-node-title,
.tab-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-node.is-case {
  cursor: pointer;
}

.tree-node.is-tag .tree-node-title {
  color: #334155;
  font-weight: 700;
}

.tree-node.is-empty {
  color: var(--app-secondary-text);
  cursor: default;
}

.tree-node-count {
  flex-shrink: 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

/* 需压过 .el-tag.el-tag--* .el-tag--small（3 个类 + 后加载），放在 .api-tree 下提高特异性 */
.api-tree :deep(.method-tag.el-tag.el-tag--small) {
  --el-tag-font-size: 10px;
  flex: 0 0 auto;
  font-weight: 600;
  font-size: 10px;
  line-height: 1;
  height: 18px;
  min-height: 18px;
  padding: 0 5px;
  border-radius: 3px;
  box-sizing: border-box;
}

.run-console {
  display: flex;
  min-width: 0;
  flex: 1 1 auto;
  flex-direction: column;
  padding: 14px 14px 16px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.92)),
    var(--app-card-bg);
}

.console-empty {
  flex: 1;
}

.console-tabs {
  margin-top: 12px;
  padding: 8px 8px 0;
  border: 1px solid color-mix(in srgb, var(--app-border-color) 76%, #94a3b8);
  border-bottom: 0;
  border-radius: 12px 12px 0 0;
  background: rgba(241, 245, 249, 0.82);
}

.console-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.console-tabs :deep(.el-tabs__item) {
  border-color: color-mix(in srgb, var(--app-border-color) 68%, #94a3b8);
  background: rgba(255, 255, 255, 0.76);
}

.console-tabs :deep(.el-tabs__item.is-active) {
  background: #ffffff;
  box-shadow: 0 -2px 8px rgba(15, 23, 42, 0.08);
}

.console-tabs :deep(.el-tabs__content) {
  display: none;
}

.run-tab-label {
  max-width: 240px;
}

.tab-title {
  max-width: 220px;
  font-weight: 600;
}

.console-panels {
  min-height: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--app-border-color) 76%, #94a3b8);
  border-radius: 0 0 12px 12px;
  background: #ffffff;
  box-shadow: inset 0 1px 0 rgba(15, 23, 42, 0.04);
}

.console-panels :deep(.request-card),
.console-panels :deep(.result-card) {
  border: 1px solid color-mix(in srgb, var(--app-border-color) 70%, #64748b);
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.05);
}

.console-panels :deep(.result-card) {
  background: linear-gradient(180deg, #ffffff, rgba(248, 250, 252, 0.92));
}

@media (max-width: 1100px) {
  .run-workspace {
    flex-direction: column;
    gap: 12px;
  }

  .api-sidebar {
    width: 100% !important;
    flex: 0 0 auto !important;
    max-height: 420px;
  }

  .console-header {
    flex-direction: column;
  }

  .console-controls {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
