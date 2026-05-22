<template>
  <div class="page-card project-management">
    <div class="page-header project-mgmt-header">
      <div>
        <h2 class="page-title">项目管理</h2>
        <p class="page-subtitle">左侧选择项目（与顶部全局项目同步）；右侧维护该项目下的服务与环境。业务数据源、AI 提供商与 Prompt 管理已移至「平台资源」。</p>
      </div>
    </div>

    <el-container class="project-mgmt-shell">
      <el-aside width="300px" class="project-mgmt-aside">
        <ProjectListPanel />
      </el-aside>
      <el-main class="project-mgmt-main">
        <ServiceEnvironment :key="projectPanelKey" embedded />
      </el-main>
    </el-container>
  </div>
</template>

<script>
import ProjectListPanel from './ProjectListPanel.vue'
import ServiceEnvironment from './ServiceEnvironment.vue'
import { projectState } from '../../utils/currentProject'

export default {
  name: 'ProjectManagement',
  components: {
    ProjectListPanel,
    ServiceEnvironment,
  },
  computed: {
    projectPanelKey() {
      return projectState.currentProjectId || '__no_project__'
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
  padding: 16px 20px;
}
</style>
