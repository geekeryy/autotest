<template>
  <div class="project-run-history page-card">
    <div class="page-header">
      <div>
        <h2 class="page-title">运行记录</h2>
        <p class="page-subtitle">当前项目下全部场景测试运行</p>
      </div>
    </div>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="filter-bar">
          <span>运行列表</span>
          <div class="filter-actions">
            <el-select
              v-model="filterScenarioId"
              clearable
              filterable
              placeholder="场景"
              style="width: 200px"
              @change="onFilterChange"
            >
              <el-option v-for="sc in scenarios" :key="sc.id" :label="sc.name" :value="sc.id" />
            </el-select>
            <el-select v-model="filterStatus" clearable placeholder="状态" style="width: 120px" @change="onFilterChange">
              <el-option label="通过" value="passed" />
              <el-option label="失败" value="failed" />
            </el-select>
            <el-date-picker
              v-model="filterRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始"
              end-placeholder="结束"
              value-format="YYYY-MM-DD"
              @change="onFilterChange"
            />
          </div>
        </div>
      </template>
      <el-table
        v-loading="loading"
        :data="runs"
        border
        size="small"
        class="run-history-table"
        row-class-name="run-row-clickable"
        @row-click="onRowClick"
      >
        <el-table-column prop="name" label="运行名称" min-width="140" show-overflow-tooltip />
        <el-table-column label="场景" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ scenarioName(row) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="环境" width="120">
          <template #default="{ row }">{{ envName(row) }}</template>
        </el-table-column>
        <el-table-column label="开始时间" width="170">
          <template #default="{ row }">{{ formatTime(row.startedAt || row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="结束时间" width="170">
          <template #default="{ row }">{{ formatTime(row.finishedAt) }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="100">
          <template #default="{ row }">{{ formatRunDuration(row) }}</template>
        </el-table-column>
        <el-table-column label="步骤通过" width="90" align="center">
          <template #default="{ row }">{{ stepCount(row, 'passed') }}</template>
        </el-table-column>
        <el-table-column label="步骤失败" width="90" align="center">
          <template #default="{ row }">{{ stepCount(row, 'failed') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openReport(row.id)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next, sizes"
          :page-sizes="[10, 20, 50]"
          @current-change="loadRuns"
          @size-change="onPageSizeChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script>
import { listProjectRuns, listScenarios } from '../../api'
import { projectState } from '../../utils/currentProject'

export default {
  name: 'ProjectRunHistory',
  data() {
    return {
      runs: [],
      scenarios: [],
      total: 0,
      page: 1,
      pageSize: 20,
      loading: false,
      filterStatus: '',
      filterScenarioId: '',
      filterRange: null
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    }
  },
  watch: {
    projectId: {
      immediate: true,
      handler() {
        this.refresh()
      }
    }
  },
  methods: {
    async refresh() {
      if (!this.projectId) return
      await Promise.all([this.loadScenarios(), this.loadRuns()])
    },
    async loadScenarios() {
      try {
        this.scenarios = await listScenarios({ projectId: this.projectId })
      } catch {
        this.scenarios = []
      }
    },
    onFilterChange() {
      this.page = 1
      this.loadRuns()
    },
    onPageSizeChange() {
      this.page = 1
      this.loadRuns()
    },
    async loadRuns() {
      if (!this.projectId) return
      this.loading = true
      try {
        const params = {
          page: this.page,
          pageSize: this.pageSize
        }
        if (this.filterStatus) params.status = this.filterStatus
        if (this.filterScenarioId) params.scenarioId = this.filterScenarioId
        if (this.filterRange?.[0]) params.from = `${this.filterRange[0]}T00:00:00Z`
        if (this.filterRange?.[1]) params.to = `${this.filterRange[1]}T23:59:59Z`
        const res = await listProjectRuns(this.projectId, params)
        this.runs = res?.items || []
        this.total = res?.total ?? 0
      } finally {
        this.loading = false
      }
    },
    parseSnapshot(row) {
      try {
        return typeof row.snapshot === 'string' ? JSON.parse(row.snapshot) : row.snapshot || {}
      } catch {
        return {}
      }
    },
    scenarioName(row) {
      const snap = this.parseSnapshot(row)
      return snap.scenarioName || '—'
    },
    envName(row) {
      const snap = this.parseSnapshot(row)
      return snap.environmentName || '—'
    },
    stepCount(row, field) {
      const summary = row.summary
      if (!summary) return '—'
      const n = summary[field]
      return n != null ? n : '—'
    },
    statusLabel(status) {
      if (status === 'passed') return '通过'
      if (status === 'failed') return '失败'
      if (status === 'running') return '运行中'
      if (status === 'queued') return '排队中'
      return status || '—'
    },
    statusTagType(status) {
      if (status === 'passed') return 'success'
      if (status === 'failed') return 'danger'
      if (status === 'running') return 'warning'
      return 'info'
    },
    formatTime(t) {
      if (!t) return '—'
      return new Date(t).toLocaleString()
    },
    formatRunDuration(row) {
      if (!row.startedAt || !row.finishedAt) return '—'
      const ms = new Date(row.finishedAt) - new Date(row.startedAt)
      if (ms < 1000) return `${ms} ms`
      return `${(ms / 1000).toFixed(2)} s`
    },
    openReport(runId) {
      this.$router.push(`/runs/${runId}`)
    },
    onRowClick(row) {
      if (row?.id) this.openReport(row.id)
    }
  }
}
</script>

<style scoped>
.project-run-history {
  width: 100%;
  max-width: none;
  box-sizing: border-box;
}
.page-header {
  margin-bottom: 0;
}
.page-title {
  margin: 0;
  font-size: 20px;
}
.page-subtitle {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}
.section-card {
  width: 100%;
  margin-bottom: 0;
}
.run-history-table {
  width: 100%;
}
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.filter-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.pagination-wrap {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
:deep(.run-row-clickable) {
  cursor: pointer;
}
</style>
