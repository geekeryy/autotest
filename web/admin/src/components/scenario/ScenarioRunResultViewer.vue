<template>
  <div class="scenario-run-result-viewer">
    <el-collapse v-model="openItems" class="scenario-run-result-viewer__collapse">
    <el-collapse-item v-for="(sr, si) in normalizedResults" :key="si" :name="stepCollapseName(si)">
      <template #title>
        <div class="step-run-result-title">
          <el-icon class="step-run-icon" :class="'step-run-icon--' + outcomeMeta(sr).key">
            <component :is="outcomeMeta(sr).Icon" />
          </el-icon>
          <span class="step-run-result-title__seq">#{{ stepSeq(sr) }}</span>
          <span class="step-run-result-title__name">{{ stepLabel(sr) }}</span>
          <span
            v-if="stepInlineErrorText(sr)"
            class="step-run-result-title__error"
            :title="stepInlineErrorText(sr)"
          >{{ stepInlineErrorText(sr) }}</span>
          <span v-if="sr.result?.durationMillis != null" class="step-run-result-title__duration">
            {{ formatDuration(sr.result.durationMillis) }}
          </span>
        </div>
      </template>
      <div class="step-result-detail">
        <el-tabs v-model="detailTabs[si]" class="step-result-info-tabs" type="border-card">
          <el-tab-pane label="详情" name="detail">
            <div class="step-result-detail-grid">
              <section class="step-result-snapshot-panel">
                <div class="step-result-panel-head">
                  <span>响应</span>
                  <el-tag v-if="httpStatus(sr)" :type="httpStatusType(sr)" size="small" effect="plain">
                    {{ httpStatus(sr) }}
                  </el-tag>
                </div>
                <JsonSnapshotViewer :lines="jsonLines(si, 'response', sr.result?.responseSnapshot)" @toggle="toggleJson(si, 'response', $event)" />
              </section>
              <section class="step-result-snapshot-panel">
                <div class="step-result-panel-head">
                  <span>请求</span>
                  <el-tag v-if="requestMethod(sr)" size="small" effect="plain">{{ requestMethod(sr) }}</el-tag>
                </div>
                <JsonSnapshotViewer :lines="jsonLines(si, 'request', sr.result?.requestSnapshot)" @toggle="toggleJson(si, 'request', $event)" />
              </section>
            </div>
          </el-tab-pane>
          <el-tab-pane label="断言" name="assertions">
            <el-table v-if="assertionsFor(sr).length" :data="assertionsFor(sr)" border size="small">
              <el-table-column label="类型" width="100">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain">{{ assertionTypeLabel(row.type) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="名称 / 路径" min-width="140">
                <template #default="{ row }">
                  <span v-if="row.name">{{ row.name }}</span>
                  <code v-else class="assertion-path">{{ row.path || row.headerName || '' }}</code>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="80">
                <template #default="{ row }">
                  <el-tag :type="row.passed ? 'success' : 'danger'" size="small">
                    {{ row.passed ? '通过' : '失败' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="message" label="详情" min-width="180" />
            </el-table>
            <el-empty v-else description="暂无断言结果" :image-size="42" />
          </el-tab-pane>
          <el-tab-pane v-if="sr.output && Object.keys(sr.output).length" label="步骤输出" name="output">
            <JsonSnapshotViewer :lines="jsonLines(si, 'output', sr.output)" @toggle="toggleJson(si, 'output', $event)" />
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script>
import {
  CircleCheck,
  CircleClose,
  QuestionFilled,
  Remove,
  WarningFilled
} from '@element-plus/icons-vue'
import { jsonLinesForSnapshot } from '../../utils/jsonViewer'
import JsonSnapshotViewer from './JsonSnapshotViewer.vue'

const ASSERTION_TYPE_LABELS = {
  status_code: '状态码',
  jsonpath: 'JSONPath',
  header: '响应头',
  body_contains: 'Body',
  response_time: '响应时间',
  script: 'JS 脚本'
}

export default {
  name: 'ScenarioRunResultViewer',
  components: { JsonSnapshotViewer },
  props: {
    stepResults: { type: Array, default: () => [] },
    caseNameResolver: { type: Function, default: null },
    defaultExpandFailed: { type: Boolean, default: false }
  },
  data() {
    return {
      openItems: [],
      detailTabs: {},
      jsonCollapsed: {}
    }
  },
  computed: {
    allExpanded() {
      const n = this.normalizedResults.length
      if (n === 0) return false
      // Element Plus collapse 会把 activeNames 规范为字符串，与数字混用会导致 UI 未展开但 openItems 非空
      const open = new Set((this.openItems || []).map((name) => String(name)))
      for (let i = 0; i < n; i++) {
        if (!open.has(String(i))) return false
      }
      return true
    },
    normalizedResults() {
      return (this.stepResults || []).map((sr) => {
        const step = sr.step || {}
        const result = sr.result || {}
        return {
          step: {
            stepSeq: step.stepSeq ?? step.step_seq,
            stepOrder: step.stepOrder,
            name: step.name,
            stepType: step.stepType || 'api',
            testCaseId: step.testCaseId
          },
          result: {
            ...result,
            assertions: result.assertions || sr.assertions || []
          },
          output: sr.output,
          stepErrors: sr.stepErrors
        }
      })
    }
  },
  watch: {
    stepResults: {
      immediate: true,
      handler() {
        const tabs = {}
        this.normalizedResults.forEach((_, i) => {
          tabs[i] = 'detail'
        })
        this.detailTabs = tabs
        if (this.defaultExpandFailed) {
          this.openItems = this.defaultFailedOpenIndices()
        } else {
          this.openItems = []
        }
        // 按钮默认展示「全部展开」，不因 defaultExpandFailed 自动展开而改写父级状态
      }
    }
  },
  methods: {
    stepCollapseName(si) {
      return String(si)
    },
    formatDuration(ms) {
      if (ms == null) return '-'
      const value = Number(ms || 0)
      if (value < 1000) return `${value} ms`
      return `${(value / 1000).toFixed(2)} s`
    },
    stepSeq(sr) {
      const n = sr.step?.stepSeq
      return n != null && n !== '' ? n : '?'
    },
    stepLabel(sr) {
      const custom = (sr.step?.name || '').trim()
      if (custom) return custom
      if (sr.step?.stepType === 'api' && sr.step?.testCaseId && this.caseNameResolver) {
        return this.caseNameResolver(sr.step.testCaseId) || 'API'
      }
      const types = { database: '数据库', script: '脚本', for: 'For循环', condition: '条件分支' }
      return types[sr.step?.stepType] || sr.step?.stepType || '步骤'
    },
    outcomeMeta(sr) {
      let key = 'unknown'
      if (sr.output?.skipped === true) key = 'skipped'
      else if (sr.result?.status === 'passed') key = 'passed'
      else if (sr.result?.status === 'failed') key = 'failed'
      else if (sr.result?.status === 'error') key = 'error'
      const icons = {
        passed: CircleCheck,
        failed: CircleClose,
        error: WarningFilled,
        skipped: Remove,
        unknown: QuestionFilled
      }
      return { key, Icon: icons[key] || QuestionFilled }
    },
    stepInlineErrorText(sr) {
      const { key } = this.outcomeMeta(sr)
      if (key === 'passed') return ''
      const parts = []
      if (key === 'skipped' && sr.output?.reason) {
        parts.push(String(sr.output.reason))
      }
      if (sr.result?.error) parts.push(String(sr.result.error))
      if (sr.stepErrors?.length) {
        parts.push(`步骤错误：${sr.stepErrors.join('; ')}`)
      }
      return parts.join(' · ')
    },
    parseSnapshot(snapshot) {
      if (snapshot == null || snapshot === '') return null
      try {
        return typeof snapshot === 'string' ? JSON.parse(snapshot) : snapshot
      } catch {
        return null
      }
    },
    httpStatus(sr) {
      const o = this.parseSnapshot(sr.result?.responseSnapshot)
      return o?.statusCode
    },
    httpStatusType(sr) {
      const code = Number(this.httpStatus(sr))
      if (code >= 200 && code < 300) return 'success'
      if (code >= 400) return 'danger'
      return 'warning'
    },
    requestMethod(sr) {
      const req = this.parseSnapshot(sr.result?.requestSnapshot)
      return req?.method || req?.type || ''
    },
    assertionsFor(sr) {
      const a = sr.result?.assertions
      return Array.isArray(a) ? a : []
    },
    assertionTypeLabel(type) {
      return ASSERTION_TYPE_LABELS[type] || type || '-'
    },
    collapseKey(si, kind) {
      return `${si}:${kind}`
    },
    jsonLines(si, kind, snapshot) {
      return jsonLinesForSnapshot(snapshot, this.collapseKey(si, kind), this.jsonCollapsed)
    },
    toggleJson(si, kind, path) {
      const key = this.collapseKey(si, kind)
      const current = { ...(this.jsonCollapsed[key] || {}) }
      if (current[path]) delete current[path]
      else current[path] = true
      this.jsonCollapsed = { ...this.jsonCollapsed, [key]: current }
    },
    defaultFailedOpenIndices() {
      return this.normalizedResults
        .map((sr, i) => (this.outcomeMeta(sr).key !== 'passed' ? String(i) : null))
        .filter((i) => i != null)
    },
    emitExpandState() {
      this.$emit('expand-state', { allExpanded: this.allExpanded })
    },
    expandAll() {
      this.openItems = this.normalizedResults.map((_, i) => String(i))
    },
    collapseAll() {
      this.openItems = []
    },
    toggleExpandAll() {
      if (this.allExpanded) this.collapseAll()
      else this.expandAll()
      this.$nextTick(() => this.emitExpandState())
    }
  }
}
</script>

<style scoped>
.step-run-result-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}
.step-run-result-title__name {
  flex-shrink: 0;
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.step-run-result-title__error {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--el-color-danger);
  font-size: 12px;
}
.step-run-result-title__duration {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: auto;
}
.step-run-icon--passed { color: var(--el-color-success); }
.step-run-icon--failed { color: var(--el-color-danger); }
.step-run-icon--error { color: var(--el-color-warning); }
.step-result-detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.step-result-panel-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-weight: 600;
}
@media (max-width: 900px) {
  .step-result-detail-grid { grid-template-columns: 1fr; }
}
</style>
