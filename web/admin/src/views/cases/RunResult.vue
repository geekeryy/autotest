<template>
  <div class="result-page">
    <div class="result-header">
      <div>
        <el-button link type="primary" @click="$router.push('/cases')">返回用例列表</el-button>
        <h2>{{ run?.name || '运行结果' }}</h2>
        <p v-if="run">Run ID: {{ run.id }}</p>
      </div>
      <div class="result-header-right">
        <div class="response-overview-tags" aria-label="响应概览">
          <el-tag :type="responseStatusType" effect="plain" size="small">{{ responseSnapshot.statusCode || '-' }}</el-tag>
          <el-tag v-if="result?.error" effect="plain" type="danger" size="small" class="response-msg-tag">
            <span class="response-msg-tag-text">{{ result.error }}</span>
          </el-tag>
        </div>
        <el-button v-if="result?.testCaseId" @click="$router.push(`/run-console/${result.testCaseId}`)">再次运行</el-button>
      </div>
    </div>

    <el-row v-loading="loading" :gutter="16">
      <el-col :xs="24" :sm="8">
        <el-card class="summary-card">
          <span>运行状态</span>
          <el-tag :type="statusType(run?.status)">{{ run?.status || '-' }}</el-tag>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card class="summary-card">
          <span>结果状态</span>
          <el-tag :type="statusType(result?.status)">{{ result?.status || '-' }}</el-tag>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card class="summary-card">
          <span>耗时</span>
          <strong>{{ result?.durationMillis || 0 }} ms</strong>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="result-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="请求" name="request">
          <div class="section-grid">
            <div class="section-block full">
              <h3>URL</h3>
              <div class="url-line">
                <el-tag>{{ requestSnapshot.method || '-' }}</el-tag>
                <span>{{ requestSnapshot.url || '-' }}</span>
              </div>
            </div>
            <div class="section-block">
              <h3>Params</h3>
              <el-table :data="requestQueryRows" border>
                <el-table-column prop="key" label="名称" />
                <el-table-column prop="value" label="值" />
              </el-table>
            </div>
            <div class="section-block">
              <h3>Headers</h3>
              <el-table :data="headersToRows(requestSnapshot.headers)" border>
                <el-table-column prop="key" label="名称" />
                <el-table-column prop="value" label="值" />
              </el-table>
            </div>
            <div class="section-block full">
              <h3>Body</h3>
              <pre class="code-view">{{ formatBody(requestSnapshot.body) }}</pre>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="响应" name="response">
          <div class="section-grid">
            <div class="section-block full">
              <h3>Body</h3>
              <pre class="code-view">{{ formatBody(responseSnapshot.body) }}</pre>
            </div>
            <div class="section-block full">
              <el-tabs v-model="responseDetailTab" class="response-detail-tabs" type="card">
                <el-tab-pane label="响应头" name="headers">
                  <el-table :data="headersToRows(responseSnapshot.headers)" border>
                    <el-table-column prop="key" label="名称" />
                    <el-table-column prop="value" label="值" />
                  </el-table>
                </el-tab-pane>
                <el-tab-pane label="实际请求" name="curl">
                  <pre class="code-view code-view--curl">{{ requestCurlCommand }}</pre>
                </el-tab-pane>
              </el-tabs>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="断言" name="assertions">
          <el-table :data="assertions" border>
            <el-table-column prop="type" label="类型" width="160" />
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="row.passed ? 'success' : 'danger'">{{ row.passed ? '通过' : '失败' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="信息" min-width="260" />
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script>
import { getRunResult } from '../../api'
import { buildCurlFromRequestSnapshot } from '../../utils/curl'

export default {
  name: 'RunResult',
  data() {
    return {
      loading: false,
      activeTab: this.initialTab(),
      responseDetailTab: 'headers',
      payload: null
    }
  },
  computed: {
    run() {
      return this.payload?.run
    },
    result() {
      return this.payload?.result
    },
    requestSnapshot() {
      return this.result?.requestSnapshot || {}
    },
    responseSnapshot() {
      return this.result?.responseSnapshot || {}
    },
    assertions() {
      return Array.isArray(this.result?.assertions) ? this.result.assertions : []
    },
    requestQueryRows() {
      const url = this.requestSnapshot.url
      if (!url) return []
      try {
        const parsed = new URL(url)
        return Array.from(parsed.searchParams.entries()).map(([key, value]) => ({ key, value }))
      } catch (error) {
        return []
      }
    },
    responseStatusType() {
      const code = Number(this.responseSnapshot.statusCode)
      if (code >= 200 && code < 300) return 'success'
      if (code >= 400) return 'danger'
      return 'warning'
    },
    requestCurlCommand() {
      return buildCurlFromRequestSnapshot(this.requestSnapshot)
    }
  },
  async created() {
    this.loading = true
    try {
      this.payload = await getRunResult(this.$route.params.runID)
    } catch {
      // 错误已在全局请求拦截器中提示
    } finally {
      this.loading = false
    }
  },
  methods: {
    initialTab() {
      const tab = this.$route.query.tab
      return ['request', 'response', 'assertions'].includes(tab) ? tab : 'request'
    },
    statusType(status) {
      if (status === 'passed') return 'success'
      if (status === 'failed' || status === 'error') return 'danger'
      return 'warning'
    },
    headersToRows(headers) {
      return Object.entries(headers || {}).map(([key, value]) => ({
        key,
        value: Array.isArray(value) ? value.join(', ') : String(value)
      }))
    },
    formatBody(value) {
      if (value == null || value === '') return ''
      if (typeof value !== 'string') return JSON.stringify(value, null, 2)
      try {
        return JSON.stringify(JSON.parse(value), null, 2)
      } catch (error) {
        return value
      }
    }
  }
}
</script>

<style scoped>
.result-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.result-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.result-header-right {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

.response-overview-tags {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.response-msg-tag-text {
  display: inline-block;
  max-width: min(320px, 42vw);
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}

.result-header h2 {
  margin: 6px 0;
  font-size: var(--app-font-size-title);
}

.result-header p {
  margin: 0;
  color: var(--app-secondary-text);
}

.summary-card :deep(.el-card__body) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 54px;
}

.result-card {
  border-radius: 12px;
}

.section-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.section-block.full {
  grid-column: 1 / -1;
}

.section-block h3 {
  margin: 0 0 10px;
  font-size: var(--app-font-size-base);
}

.url-line {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 10px 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-card-bg);
  word-break: break-all;
}

.code-view {
  min-height: 240px;
  max-height: 520px;
  margin: 0;
  overflow: auto;
  padding: 14px;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-code-bg);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.6;
  white-space: pre-wrap;
}

.response-detail-tabs {
  width: 100%;
}

.response-detail-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.code-view--curl {
  min-height: 160px;
}

@media (max-width: 900px) {
  .section-grid {
    grid-template-columns: 1fr;
  }
}
</style>
