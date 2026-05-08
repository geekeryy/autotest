<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">概览</h2>
      <el-button @click="load">
        <el-icon><Refresh /></el-icon>
        <span>刷新</span>
      </el-button>
    </div>

    <p v-if="!currentProjectId" class="context-hint">在顶部选择项目后，接口、场景与数据源数量将按当前项目统计。</p>

    <el-row :gutter="16" v-loading="loading" class="metric-row">
      <el-col :span="6" v-for="card in cards" :key="card.title">
        <el-card class="metric" :style="{ borderTopColor: card.color }">
          <div class="metric-icon" :style="{ color: card.color }">
            <el-icon :size="28"><component :is="card.icon" /></el-icon>
          </div>
          <div class="metric-value" :style="{ color: card.color }">{{ card.value }}</div>
          <div class="metric-title">{{ card.title }}</div>
          <div v-if="card.hint" class="metric-hint">{{ card.hint }}</div>
        </el-card>
      </el-col>
    </el-row>

    <div class="page-card shortcuts">
      <h3 class="section-title">快捷入口</h3>
      <p class="shortcuts-hint">根据本机记录的点击次数，优先展示使用最多的 4 个入口；次数仅在当前浏览器保存。</p>
      <el-row :gutter="12">
        <el-col v-for="item in topShortcuts" :key="item.key" :xs="12" :sm="8" :md="6" class="shortcut-col">
          <router-link :to="item.to" class="shortcut-link" @click="onShortcutClick(item.key)">
            <el-card shadow="hover" class="shortcut-card">
              <el-icon class="shortcut-icon" :size="22"><component :is="item.icon" /></el-icon>
              <div class="shortcut-title">{{ item.title }}</div>
              <div class="shortcut-desc">{{ item.desc }}</div>
            </el-card>
          </router-link>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script>
import {
  Connection,
  Document,
  Folder,
  Monitor,
  Refresh,
  Setting,
  Tickets,
  Upload,
  DataAnalysis
} from '@element-plus/icons-vue'
import { listCases, listDataSources, listProjects, listScenarios } from '../api'
import { hasPermission } from '../auth'
import { incrementShortcutClick, rankShortcutsByClicks } from '../utils/dashboardShortcutClicks'
import { projectState } from '../utils/currentProject'

const SHORTCUTS = [
  { key: 'projects', title: '项目管理', desc: '项目、成员与资源', to: '/projects', permission: 'projects:read', icon: Folder },
  {
    key: 'services',
    title: '服务与环境',
    desc: 'Base URL 与认证',
    to: { path: '/projects', query: { tab: 'services' } },
    permission: 'projects:read',
    icon: Setting
  },
  { key: 'cases', title: '接口列表', desc: 'API 请求模板', to: '/cases', permission: 'cases:read', icon: Document },
  { key: 'spec', title: 'OpenAPI 导入', desc: '同步 Swagger 定义', to: '/spec-import', permission: 'specs:import', icon: Upload },
  { key: 'run', title: '运行控制台', desc: '调试与保存用例', to: '/run-console', permission: 'cases:read', icon: Monitor },
  { key: 'scenarios', title: '场景编排', desc: '多步骤联调与回归', to: '/scenarios', permission: 'cases:read', icon: Connection },
  { key: 'sql', title: 'SQL 参数源', desc: '动态参数与预览', to: '/sql-parameter-sources', permission: 'projects:read', icon: Tickets }
]

const SHORTCUT_ORDER_KEYS = SHORTCUTS.map((s) => s.key)

export default {
  name: 'Dashboard',
  components: {
    Refresh,
    Folder,
    Document,
    Connection,
    Tickets
  },
  data() {
    return {
      loading: false,
      counts: {
        projects: 0,
        cases: 0,
        scenarios: 0,
        dataSources: 0
      },
      /** 用于点击后触发「按次数排序」的 computed 刷新 */
      shortcutClickRevision: 0
    }
  },
  computed: {
    currentProjectId() {
      return projectState.currentProjectId
    },
    visibleShortcuts() {
      return SHORTCUTS.filter((s) => hasPermission(s.permission))
    },
    topShortcuts() {
      this.shortcutClickRevision
      return rankShortcutsByClicks(this.visibleShortcuts, SHORTCUT_ORDER_KEYS).slice(0, 4)
    },
    cards() {
      const scoped = !!this.currentProjectId
      return [
        { title: '项目数', value: this.counts.projects, hint: '全部项目', icon: 'Folder', color: '#2563eb' },
        { title: '接口数', value: this.counts.cases, hint: scoped ? '当前项目' : '全部可见', icon: 'Document', color: '#14b8a6' },
        { title: '场景数', value: this.counts.scenarios, hint: scoped ? '当前项目' : '全部可见', icon: 'Connection', color: '#f97316' },
        {
          title: '业务数据源',
          value: scoped ? this.counts.dataSources : '—',
          hint: scoped ? '当前项目' : '请先选择项目',
          icon: 'Tickets',
          color: '#8b5cf6'
        }
      ]
    }
  },
  watch: {
    currentProjectId() {
      this.load()
    }
  },
  created() {
    this.load()
  },
  methods: {
    onShortcutClick(key) {
      incrementShortcutClick(key)
      this.shortcutClickRevision++
    },
    async load() {
      this.loading = true
      try {
        const projects = await listProjects()
        this.counts.projects = projects.length

        const pid = this.currentProjectId
        const projectParams = pid ? { projectId: pid } : {}

        const [cases, scenarios, dataSources] = await Promise.all([
          listCases(projectParams),
          listScenarios(projectParams),
          pid ? listDataSources({ projectId: pid }) : Promise.resolve([])
        ])

        this.counts.cases = Array.isArray(cases) ? cases.length : 0
        this.counts.scenarios = Array.isArray(scenarios) ? scenarios.length : 0
        this.counts.dataSources = Array.isArray(dataSources) ? dataSources.length : 0
      } catch {
        // 错误已在全局请求拦截器中提示
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.context-hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
}

.metric-row {
  margin-bottom: 8px;
}

.metric {
  margin-bottom: 16px;
  border-top: 3px solid transparent;
  transition: border-top-color 0.2s ease;
}

.metric-icon {
  margin-bottom: 10px;
}

.metric-value {
  font-size: var(--app-font-size-metric);
  font-weight: 700;
}

.metric-title {
  color: var(--app-secondary-text);
  margin-top: 8px;
}

.metric-hint {
  margin-top: 4px;
  font-size: var(--el-font-size-extra-small);
  color: var(--el-text-color-placeholder);
}

.shortcuts {
  margin-top: 8px;
}

.section-title {
  margin: 0 0 8px;
  font-size: var(--el-font-size-medium);
  font-weight: 600;
}

.shortcuts-hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
}

.shortcut-col {
  margin-bottom: 12px;
}

.shortcut-link {
  display: block;
  text-decoration: none;
  color: inherit;
}

.shortcut-card {
  height: 100%;
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.shortcut-card:hover {
  border-color: var(--app-primary-color);
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.shortcut-icon {
  color: var(--app-primary-color);
  margin-bottom: 8px;
}

.shortcut-title {
  font-weight: 600;
  margin-bottom: 4px;
}

.shortcut-desc {
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
  line-height: 1.4;
}
</style>
