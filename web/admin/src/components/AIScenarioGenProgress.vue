<!--
  AIScenarioGenProgress
  =====================
  Streamed progress card for generate_and_verify_scenarios async jobs.
  Consumes genJobProgress from aiAssistant store (SSE kind: gen_job_progress).
-->
<template>
  <div class="gen-progress" :class="'gen-progress--' + statusClass">
    <div class="gen-progress__head">
      <el-icon v-if="isRunning" class="gen-progress__spin"><Loading /></el-icon>
      <el-icon v-else-if="statusClass === 'completed'" class="gen-progress__icon gen-progress__icon--ok"><CircleCheck /></el-icon>
      <el-icon v-else-if="statusClass === 'failed'" class="gen-progress__icon gen-progress__icon--fail"><CircleClose /></el-icon>
      <span class="gen-progress__title">{{ titleText }}</span>
      <el-tag v-if="progress.round" size="small" type="info" effect="plain">
        第 {{ progress.round }} / {{ progress.maxRounds || '?' }} 轮
      </el-tag>
    </div>

    <div v-if="hasPassRate" class="gen-progress__metrics">
      <div class="gen-progress__metric">
        <span class="gen-progress__metric-label">通过率</span>
        <span class="gen-progress__metric-value">{{ passRateText }}</span>
      </div>
      <div v-if="progress.totalScenarios != null" class="gen-progress__metric">
        <span class="gen-progress__metric-label">场景</span>
        <span class="gen-progress__metric-value">
          {{ progress.passedScenarios ?? 0 }} / {{ progress.totalScenarios }}
        </span>
      </div>
    </div>

    <el-progress
      v-if="hasPassRate"
      :percentage="passRatePercent"
      :status="progressBarStatus"
      :stroke-width="8"
      :show-text="false"
      class="gen-progress__bar"
    />

    <p v-if="progress.phase" class="gen-progress__phase">{{ progress.phase }}</p>
    <p v-if="progress.repairSummary" class="gen-progress__summary">{{ progress.repairSummary }}</p>

    <ul v-if="repairDetails.length" class="gen-progress__details">
      <li v-for="(item, idx) in repairDetails" :key="idx">{{ item }}</li>
    </ul>
  </div>
</template>

<script>
import { CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'

export default {
  name: 'AIScenarioGenProgress',
  components: { CircleCheck, CircleClose, Loading },
  props: {
    progress: { type: Object, required: true },
  },
  computed: {
    statusClass() {
      const s = String(this.progress?.status || 'running').toLowerCase()
      if (s === 'completed' || s === 'done' || s === 'success') return 'completed'
      if (s === 'failed' || s === 'error') return 'failed'
      return 'running'
    },
    isRunning() {
      return this.statusClass === 'running'
    },
    hasPassRate() {
      return this.progress?.passRate != null || (
        this.progress?.totalScenarios != null && this.progress.totalScenarios > 0
      )
    },
    passRatePercent() {
      if (this.progress?.passRate != null) {
        const n = Number(this.progress.passRate)
        if (Number.isFinite(n)) return Math.min(100, Math.max(0, n <= 1 ? n * 100 : n))
      }
      const total = Number(this.progress?.totalScenarios)
      const passed = Number(this.progress?.passedScenarios)
      if (Number.isFinite(total) && total > 0 && Number.isFinite(passed)) {
        return Math.round((passed / total) * 100)
      }
      return 0
    },
    passRateText() {
      return `${Math.round(this.passRatePercent)}%`
    },
    progressBarStatus() {
      if (this.statusClass === 'completed') return 'success'
      if (this.statusClass === 'failed') return 'exception'
      return undefined
    },
    titleText() {
      const map = {
        running: '场景生成与验证进行中',
        completed: '场景生成与验证已完成',
        failed: '场景生成与验证失败',
      }
      return map[this.statusClass] || map.running
    },
    repairDetails() {
      const raw = this.progress?.repairDetails || this.progress?.details
      if (!raw) return []
      if (Array.isArray(raw)) return raw.map((d) => String(d))
      return [String(raw)]
    },
  },
}
</script>

<style scoped>
.gen-progress {
  margin: 12px 0;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--app-border-color, #e8eaed);
  background: var(--app-card-bg, #fff);
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.04);
}

.gen-progress--running {
  border-color: color-mix(in srgb, var(--el-color-primary) 35%, var(--app-border-color));
}

.gen-progress--completed {
  border-color: color-mix(in srgb, var(--el-color-success) 40%, var(--app-border-color));
}

.gen-progress--failed {
  border-color: color-mix(in srgb, var(--el-color-danger) 40%, var(--app-border-color));
}

.gen-progress__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.gen-progress__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-color);
}

.gen-progress__spin {
  color: var(--el-color-primary);
  animation: gen-spin 1s linear infinite;
}

@keyframes gen-spin {
  to { transform: rotate(360deg); }
}

.gen-progress__icon--ok {
  color: var(--el-color-success);
}

.gen-progress__icon--fail {
  color: var(--el-color-danger);
}

.gen-progress__metrics {
  display: flex;
  gap: 20px;
  margin-bottom: 8px;
}

.gen-progress__metric {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.gen-progress__metric-label {
  font-size: 12px;
  color: var(--app-text-muted);
}

.gen-progress__metric-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-color);
}

.gen-progress__bar {
  margin-bottom: 8px;
}

.gen-progress__phase,
.gen-progress__summary {
  margin: 0 0 6px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--app-secondary-text);
}

.gen-progress__details {
  margin: 8px 0 0;
  padding-left: 18px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--app-secondary-text);
}
</style>
