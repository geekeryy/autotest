<template>
  <div class="response-viewer">
    <!-- 响应概览标签 -->
    <div class="response-overview-tags" aria-label="响应概览">
      <el-tag v-if="result?.status" effect="plain" size="small" :type="statusTagType">
        {{ result.status }}
      </el-tag>
      <el-tag v-if="result?.durationMs != null" effect="plain" size="small" type="info">
        {{ result.durationMs }}ms
      </el-tag>
      <el-tag v-if="result?.error" effect="plain" type="danger" size="small">
        {{ result.error }}
      </el-tag>
    </div>

    <!-- 响应详情 Tab -->
    <el-tabs v-model="activeTab" class="response-detail-tabs">
      <el-tab-pane label="响应体" name="body">
        <div v-if="bodyText" class="response-body">
          <pre class="json-preview">{{ formattedBody }}</pre>
        </div>
        <el-empty v-else description="无响应数据" :image-size="40" />
      </el-tab-pane>

      <el-tab-pane label="响应头" name="headers">
        <div v-if="headers && Object.keys(headers).length" class="response-headers">
          <div v-for="(value, key) in headers" :key="key" class="header-row">
            <span class="header-key">{{ key }}</span>
            <span class="header-value">{{ value }}</span>
          </div>
        </div>
        <el-empty v-else description="无响应头" :image-size="40" />
      </el-tab-pane>

      <el-tab-pane label="断言结果" name="assertions">
        <div v-if="assertionResults && assertionResults.length" class="assertion-results">
          <div
            v-for="(a, idx) in assertionResults"
            :key="idx"
            class="assertion-row"
            :class="{ passed: a.passed, failed: !a.passed }"
          >
            <el-icon v-if="a.passed"><CircleCheck /></el-icon>
            <el-icon v-else><CircleClose /></el-icon>
            <span class="assertion-desc">
              <span v-if="a.name" class="assertion-name">{{ a.name }}</span>
              <code v-else-if="a.expression" class="assertion-path">{{ a.expression }}</code>
              <span v-if="a.message" class="assertion-message"> — {{ a.message }}</span>
            </span>
          </div>
        </div>
        <el-empty v-else description="无断言结果" :image-size="40" />
      </el-tab-pane>

      <el-tab-pane label="请求快照" name="request">
        <div v-if="requestSnapshot" class="request-snapshot">
          <pre class="json-preview">{{ formatJSON(requestSnapshot) }}</pre>
        </div>
        <el-empty v-else description="无请求快照" :image-size="40" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { CircleCheck, CircleClose } from '@element-plus/icons-vue'

const props = defineProps({
  result: { type: Object, default: null },
})

const activeTab = ref('body')

const statusTagType = computed(() => {
  const s = props.result?.status
  if (s >= 200 && s < 300) return 'success'
  if (s >= 400) return 'danger'
  return 'warning'
})

const bodyText = computed(() => {
  const body = props.result?.response?.body
  if (!body) return ''
  if (typeof body === 'string') return body
  try { return JSON.stringify(body, null, 2) } catch { return String(body) }
})

const formattedBody = computed(() => {
  try { return JSON.stringify(JSON.parse(bodyText.value), null, 2) } catch { return bodyText.value }
})

const headers = computed(() => props.result?.response?.headers || {})

const assertionResults = computed(() => props.result?.assertions || [])

const requestSnapshot = computed(() => props.result?.request || null)

function formatJSON(obj) {
  try { return JSON.stringify(obj, null, 2) } catch { return String(obj) }
}
</script>

<style scoped>
.response-viewer {
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  overflow: hidden;
}
.response-overview-tags {
  display: flex;
  gap: 8px;
  padding: 8px 16px;
  background: var(--app-code-bg);
  border-bottom: 1px solid var(--app-border-color);
}
.response-detail-tabs {
  padding: 0 16px;
}
.json-preview {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 400px;
  overflow-y: auto;
}
.response-headers {
  padding: 8px 0;
}
.header-row {
  display: flex;
  padding: 4px 0;
  border-bottom: 1px solid var(--app-border-color);
}
.header-key {
  font-weight: 600;
  min-width: 200px;
  color: var(--app-primary-color);
}
.header-value {
  word-break: break-all;
}
.assertion-results {
  padding: 8px 0;
}
.assertion-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 0;
}
.assertion-row.passed { color: #67c23a; }
.assertion-row.failed { color: #f56c6c; }
.assertion-desc {
  flex: 1;
}
.assertion-name {
  font-weight: 600;
}
.assertion-path {
  background: var(--app-code-bg);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}
.assertion-message {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}
</style>
