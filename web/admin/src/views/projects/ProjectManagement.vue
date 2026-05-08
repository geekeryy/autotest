<template>
  <div class="page-card project-management">
    <div class="page-header project-mgmt-header">
      <div>
        <h2 class="page-title">项目管理</h2>
        <p class="page-subtitle">左侧选择项目（与顶部全局项目同步）；右侧维护该项目下的服务与环境、业务数据源。</p>
      </div>
    </div>

    <el-container class="project-mgmt-shell">
      <el-aside width="300px" class="project-mgmt-aside">
        <ProjectListPanel />
      </el-aside>
      <el-main class="project-mgmt-main">
        <el-tabs v-model="activeTab" class="project-mgmt-tabs" @tab-change="onTabChange">
          <el-tab-pane label="服务与环境" name="services" lazy>
            <ServiceEnvironment :key="projectPanelKey" embedded />
          </el-tab-pane>
          <el-tab-pane label="业务数据源" name="dataSources" lazy>
            <DataSourceList :key="projectPanelKey" embedded />
          </el-tab-pane>
          <el-tab-pane label="AI 提供商" name="aiProviders" lazy>
            <AIProviderList :key="projectPanelKey" embedded />
          </el-tab-pane>
          <el-tab-pane label="Prompt 管理" name="prompts" lazy>
            <ProjectPromptList :key="projectPanelKey" embedded />
          </el-tab-pane>
        </el-tabs>
      </el-main>
    </el-container>
  </div>
</template>

<script>
import ProjectListPanel from './ProjectListPanel.vue'
import ServiceEnvironment from './ServiceEnvironment.vue'
import DataSourceList from '../data/DataSourceList.vue'
import AIProviderList from '../platform/AIProviderList.vue'
import ProjectPromptList from './ProjectPromptList.vue'
import { projectState } from '../../utils/currentProject'

export default {
  name: 'ProjectManagement',
  components: {
    ProjectListPanel,
    ServiceEnvironment,
    DataSourceList,
    AIProviderList,
    ProjectPromptList,
  },
  data() {
    return {
      activeTab: 'services',
    }
  },
  computed: {
    /** 随当前全局项目切换强制重建右侧面板，避免 lazy Tab / 树懒加载残留旧项目状态 */
    projectPanelKey() {
      return projectState.currentProjectId || '__no_project__'
    },
  },
  watch: {
    '$route.query.tab': {
      immediate: true,
      handler() {
        this.syncTabFromRoute()
      },
    },
  },
  methods: {
    normalizeTab(raw) {
      const t = typeof raw === 'string' ? raw : ''
      if (t === 'dataSources') return 'dataSources'
      if (t === 'aiProviders') return 'aiProviders'
      if (t === 'prompts') return 'prompts'
      return 'services'
    },
    syncTabFromRoute() {
      this.activeTab = this.normalizeTab(this.$route.query.tab)
    },
    onTabChange(name) {
      const q = { ...this.$route.query }
      if (name === 'services') {
        delete q.tab
      } else {
        q.tab = name
      }
      this.$router.replace({ path: '/projects', query: q })
    },
  },
}
</script>

<style scoped>
.page-subtitle {
  margin: 8px 0 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.project-mgmt-header {
  margin-bottom: 12px;
}

.project-mgmt-shell {
  min-height: 420px;
  align-items: stretch;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  overflow: hidden;
  background: var(--el-bg-color);
}

.project-mgmt-aside {
  margin: 0;
  padding: 0;
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-blank);
}

.project-mgmt-main {
  margin: 0;
  padding: 12px 16px 16px;
  min-width: 0;
}

.project-mgmt-tabs {
  margin-top: 0;
}

.project-mgmt-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}
</style>
