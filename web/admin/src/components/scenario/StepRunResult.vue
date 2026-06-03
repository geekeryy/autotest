<template>
  <div class="step-run-result">
    <div class="result-header">
      <span class="result-status" :class="statusClass">
        <el-icon v-if="status === 'passed'"><CircleCheck /></el-icon>
        <el-icon v-else-if="status === 'failed'"><CircleClose /></el-icon>
        <el-icon v-else><WarningFilled /></el-icon>
        {{ statusLabel }}
      </span>
      <span class="result-duration">{{ durationMs }}ms</span>
    </div>

    <el-tabs v-model="activeTab" class="result-tabs">
      <el-tab-pane label="响应体" name="body">
        <div class="result-body">
          <pre v-if="bodyText" class="json-preview">{{ formattedBody }}</pre>
          <el-empty v-else description="无响应数据" :image-size="40" />
        </div>
      </el-tab-pane>

      <el-tab-pane label="响应头" name="headers">
        <div v-if="headers && Object.keys(headers).length" class="result-headers">
          <div v-for="(value, key) in headers" :key="key" class="header-row">
            <span class="header-key">{{ key }}</span>
            <span class="header-value">{{ value }}</span>
          </div>
        </div>
        <el-empty v-else description="无响应头" :image-size="40" />
      </el-tab-pane>

      <el-tab-pane label="断言" name="assertions">
        <div v-if="assertions && assertions.length" class="result-assertions">
          <div
            v-for="(a, idx) in assertions"
            :key="idx"
            class="assertion-row"
            :class="{ passed: a.passed, failed: !a.passed }"
          >
            <el-icon v-if="a.passed"><CircleCheck /></el-icon>
            <el-icon v-else><CircleClose /></el-icon>
            <span class="assertion-name">{{ a.type }} {{ a.expression || '' }}</span>
            <span v-if="a.message" class="assertion-message">{{ a.message }}</span>
          </div>
        </div>
        <el-empty v-else description="无断言" :image-size="40" />
      </el-tab-pane>

      <el-tab-pane v-if="error" label="错误" name="error">
        <div class="result-error">
          <pre>{{ error }}</pre>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { CircleCheck, CircleClose, WarningFilled } from '@element-plus/icons-vue'

const props = defineProps({
  status: { type: String, default: '' },
  durationMs: { type: Number, default: 0 },
  body: { type: [String, Object], default: '' },
  headers: { type: Object, default: () => ({}) },
  assertions: { type: Array, default: () => [] },
  error: { type: String, default: '' },
})

const activeTab = ref('body')

const statusClass = computed(() => ({
  'status-passed': props.status === 'passed',
  'status-failed': props.status === 'failed',
  'status-error': props.status === 'error',
}))

const statusLabel = computed(() => {
  const map = { passed: '通过', failed: '失败', error: '错误' }
  return map[props.status] || props.status
})

const bodyText = computed(() => {
  if (typeof props.body === 'string') return props.body
  try { return JSON.stringify(props.body, null, 2) } catch { return String(props.body) }
})

const formattedBody = computed(() => {
  try { return JSON.stringify(JSON.parse(bodyText.value), null, 2) } catch { return bodyText.value }
})
</script>

<style scoped>
.step-run-result {
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  overflow: hidden;
}
.result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--app-code-bg);
  border-bottom: 1px solid var(--app-border-color);
}
.result-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 600;
}
.status-passed { color: #67c23a; }
.status-failed { color: #f56c6c; }
.status-error { color: #e6a23c; }
.result-duration {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}
.result-tabs {
  padding: 0 16px;
}
.json-preview {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 300px;
  overflow-y: auto;
}
.result-headers {
  padding: 8px 0;
}
.header-row {
  display: flex;
  padding: 4px 0;
  border-bottom: 1px solid var(--app-border-color);
}
.header-key {
  font-weight: 600;
  min-width: 180px;
  color: var(--app-primary-color);
}
.assertion-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}
.assertion-row.passed { color: #67c23a; }
.assertion-row.failed { color: #f56c6c; }
.assertion-message {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}
.result-error pre {
  color: #f56c6c;
  font-size: 12px;
  white-space: pre-wrap;
}
</style>
