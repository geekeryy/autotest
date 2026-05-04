<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">概览</h2>
      <el-button @click="load">
        <el-icon><Refresh /></el-icon>
        <span>刷新</span>
      </el-button>
    </div>

    <el-row :gutter="16">
      <el-col :span="6" v-for="card in cards" :key="card.title">
        <el-card class="metric">
          <div class="metric-value">{{ card.value }}</div>
          <div class="metric-title">{{ card.title }}</div>
        </el-card>
      </el-col>
    </el-row>

    <div class="page-card quick-start">
      <h3>快速开始</h3>
      <el-steps :active="1" finish-status="success">
        <el-step title="创建项目" description="维护系统或业务线" />
        <el-step title="添加服务与环境" description="为每个服务配置环境和 baseUrl" />
        <el-step title="导入 OpenAPI" description="生成接口和基础用例" />
        <el-step title="创建测试集" description="组合用例后执行" />
      </el-steps>
    </div>
  </div>
</template>

<script>
import { listCases, listProjects, listSuites } from '../api'

export default {
  name: 'Dashboard',
  data() {
    return {
      loading: false,
      counts: {
        projects: 0,
        cases: 0,
        suites: 0
      }
    }
  },
  computed: {
    cards() {
      return [
        { title: '项目数', value: this.counts.projects },
        { title: '测试用例', value: this.counts.cases },
        { title: '测试集', value: this.counts.suites },
        { title: '运行状态', value: 'MVP' }
      ]
    }
  },
  created() {
    this.load()
  },
  methods: {
    async load() {
      this.loading = true
      try {
        const [projects, cases, suites] = await Promise.all([listProjects(), listCases(), listSuites()])
        this.counts.projects = projects.length
        this.counts.cases = cases.length
        this.counts.suites = suites.length
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
.metric {
  margin-bottom: 16px;
}

.metric-value {
  font-size: var(--app-font-size-metric);
  font-weight: 700;
  color: var(--app-primary-color);
}

.metric-title {
  color: var(--app-secondary-text);
  margin-top: 8px;
}

.quick-start {
  margin-top: 8px;
}
</style>
