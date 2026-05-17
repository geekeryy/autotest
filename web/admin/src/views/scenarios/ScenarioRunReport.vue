<template>
  <div v-loading="loading" class="scenario-run-report page-card">
    <div class="page-header">
      <div>
        <el-button link type="primary" @click="goBack">← 返回</el-button>
        <h2 class="page-title">{{ report?.run?.name || '场景运行报告' }}</h2>
        <p v-if="report" class="page-subtitle">
          <el-tag :type="report.run.status === 'passed' ? 'success' : 'danger'" size="small">
            {{ report.run.status === 'passed' ? '通过' : '失败' }}
          </el-tag>
          <span v-if="envName"> · 环境 {{ envName }}</span>
          <span v-if="scenarioName"> · 场景 {{ scenarioName }}</span>
        </p>
      </div>
      <div class="header-actions">
        <el-button @click="download('json')">导出 JSON</el-button>
        <el-button @click="download('html')">导出 HTML</el-button>
        <el-tooltip v-if="canAnalyze" content="AI 分析失败原因">
          <el-button type="warning" plain :loading="aiLoading" @click="openAI">AI 分析</el-button>
        </el-tooltip>
      </div>
    </div>

    <el-row v-if="report" :gutter="12" class="summary-row">
      <el-col :xs="12" :sm="6">
        <el-card shadow="never"><div class="stat-label">总耗时</div><div class="stat-value">{{ summaryDuration }}</div></el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="never"><div class="stat-label">通过步骤</div><div class="stat-value">{{ report.summary.passed }}</div></el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="never"><div class="stat-label">失败步骤</div><div class="stat-value">{{ report.summary.failed }}</div></el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="never"><div class="stat-label">错误步骤</div><div class="stat-value">{{ report.summary.error }}</div></el-card>
      </el-col>
    </el-row>

    <el-card v-if="report" shadow="never" class="step-results-card">
      <template #header>
        <div class="step-results-card-header">
          <span>步骤结果</span>
          <el-tooltip v-if="report.stepResults?.length" :content="stepExpandToggleLabel" placement="top">
            <el-button
              size="small"
              :icon="stepExpandToggleIcon"
              :aria-label="stepExpandToggleLabel"
              @click="toggleStepResultsExpand"
            />
          </el-tooltip>
        </div>
      </template>
      <ScenarioRunResultViewer
        ref="stepResultsViewer"
        :step-results="report.stepResults"
        @expand-state="onStepExpandState"
      />
    </el-card>

    <AIAnalysisDialog
      v-model="aiVisible"
      :loading="aiLoading"
      :text="aiText"
      :error="aiError"
      :model="aiModel"
      :elapsed-millis="aiElapsed"
      @retry="runAI"
    />
  </div>
</template>

<script>
import { Expand, Fold } from '@element-plus/icons-vue'
import { analyzeRunFailure, exportScenarioRun, getScenarioRunReport } from '../../api'
import AIAnalysisDialog from '../../components/AIAnalysisDialog.vue'
import ScenarioRunResultViewer from '../../components/scenario/ScenarioRunResultViewer.vue'

export default {
  name: 'ScenarioRunReport',
  components: { ScenarioRunResultViewer, AIAnalysisDialog },
  data() {
    return {
      Expand,
      Fold,
      stepResultsAllExpanded: false,
      loading: false,
      report: null,
      aiVisible: false,
      aiLoading: false,
      aiText: '',
      aiError: '',
      aiModel: '',
      aiElapsed: 0
    }
  },
  computed: {
    runId() {
      return this.$route.params.runId
    },
    envName() {
      return this.report?.runSnapshot?.environmentName || ''
    },
    scenarioName() {
      return this.report?.runSnapshot?.scenarioName || ''
    },
    summaryDuration() {
      const ms = this.report?.summary?.durationMillis
      if (ms == null) return '-'
      if (ms < 1000) return `${ms} ms`
      return `${(ms / 1000).toFixed(2)} s`
    },
    canAnalyze() {
      if (!this.report?.run?.id) return false
      if (this.report.run.status !== 'passed') return true
      return (this.report.stepResults || []).some((sr) => sr.result?.status && sr.result.status !== 'passed')
    },
    stepExpandToggleLabel() {
      return this.stepResultsAllExpanded ? '全部折叠' : '全部展开'
    },
    stepExpandToggleIcon() {
      return this.stepResultsAllExpanded ? Fold : Expand
    }
  },
  watch: {
    runId: { immediate: true, handler: 'load' }
  },
  methods: {
    async load() {
      if (!this.runId) return
      this.loading = true
      try {
        this.report = await getScenarioRunReport(this.runId)
      } catch (e) {
        this.$message.error(e?.response?.data?.error || '加载报告失败')
        this.report = null
      } finally {
        this.loading = false
      }
    },
    goBack() {
      this.$router.push('/reports/runs')
    },
    onStepExpandState({ allExpanded }) {
      this.stepResultsAllExpanded = !!allExpanded
    },
    toggleStepResultsExpand() {
      this.$refs.stepResultsViewer?.toggleExpandAll()
    },
    async download(format) {
      try {
        const blob = await exportScenarioRun(this.runId, format)
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `scenario-run-${this.runId}.${format}`
        a.click()
        URL.revokeObjectURL(url)
      } catch (e) {
        this.$message.error(e?.response?.data?.error || '导出失败')
      }
    },
    openAI() {
      this.aiVisible = true
      this.runAI()
    },
    async runAI() {
      this.aiLoading = true
      this.aiText = ''
      this.aiError = ''
      try {
        const resp = await analyzeRunFailure(this.runId)
        this.aiText = resp?.text || ''
        this.aiModel = resp?.model || ''
        this.aiElapsed = Number(resp?.elapsedMillis) || 0
        if (!this.aiText) this.aiError = 'AI 未返回内容'
      } catch (e) {
        this.aiError = e?.response?.data?.error || e?.message || 'AI 分析失败'
      } finally {
        this.aiLoading = false
      }
    }
  }
}
</script>

<style scoped>
.scenario-run-report { padding: 16px; }
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.header-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.page-subtitle { margin: 8px 0 0; color: var(--el-text-color-secondary); font-size: 14px; }
.summary-row { margin-bottom: 16px; }
.stat-label { font-size: 12px; color: var(--el-text-color-secondary); }
.stat-value { font-size: 20px; font-weight: 600; margin-top: 4px; }
.step-results-card-header {
  display: flex;
  align-items: center;
  gap: 6px;
}
.step-results-card-header > span {
  font-weight: 600;
}
</style>
