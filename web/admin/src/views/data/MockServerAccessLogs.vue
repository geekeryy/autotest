<template>
  <div class="page-card mock-server-access-logs">
    <div class="page-header access-logs-header">
      <div>
        <el-button link type="primary" @click="goBack">← 返回 Mock Server</el-button>
        <h2 class="page-title">访问日志</h2>
        <p v-if="server" class="page-subtitle">
          <span>{{ server.name || '未命名 Server' }}</span>
          <el-tag :type="isRunning(server) ? 'success' : 'info'" size="small">{{ serverStatusLabel(server) }}</el-tag>
          <span v-if="accessAddress(server) !== '-'" class="access-address">{{ accessAddress(server) }}</span>
        </p>
        <p v-else-if="!loadingServer" class="page-subtitle">记录 Mock 运行时每次 HTTP 访问，便于排查规则匹配与重定向问题。</p>
      </div>
      <div class="header-actions">
        <el-switch
          v-model="accessLogAutoRefresh"
          active-text="自动刷新"
          :disabled="!server || !isRunning(server)"
          @change="onAccessLogAutoRefreshChange"
        />
        <el-button :disabled="!projectId || !serverId" :loading="loadingAccessLogs" @click="loadAccessLogs">刷新</el-button>
        <el-button
          type="danger"
          plain
          :disabled="!projectId || !serverId"
          :loading="clearingAccessLogs"
          @click="confirmClearAccessLogs"
        >
          清空日志
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再查看访问日志。"
    />

    <el-card v-else shadow="never" class="section-card" v-loading="loadingServer">
      <div class="access-log-filters">
        <el-input
          v-model="accessLogFilterStatusFrom"
          clearable
          placeholder="状态码起"
          style="width: 110px"
          @change="onAccessLogFilterChange"
        />
        <el-input
          v-model="accessLogFilterStatusTo"
          clearable
          placeholder="状态码止"
          style="width: 110px"
          @change="onAccessLogFilterChange"
        />
        <el-select
          v-model="accessLogFilterMatched"
          clearable
          placeholder="是否匹配"
          style="width: 120px"
          @change="onAccessLogFilterChange"
        >
          <el-option label="已匹配" value="true" />
          <el-option label="未匹配" value="false" />
        </el-select>
        <el-date-picker
          v-model="accessLogFilterRange"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          value-format="YYYY-MM-DDTHH:mm:ss[Z]"
          @change="onAccessLogFilterChange"
        />
      </div>

      <el-table
        :data="accessLogs"
        border
        row-key="id"
        v-loading="loadingAccessLogs"
        class="access-log-table"
        row-class-name="access-log-row-clickable"
        @row-click="openAccessLogDetail"
      >
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatAccessLogTime(row.createdAt || row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="方法" width="92">
          <template #default="{ row }">
            <el-tag :type="methodTagType(accessLogMethod(row))" size="small">{{ accessLogMethod(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ accessLogPath(row) }}</template>
        </el-table-column>
        <el-table-column label="匹配规则" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ accessLogRouteLabel(row) }}</template>
        </el-table-column>
        <el-table-column label="状态码" width="90">
          <template #default="{ row }">{{ accessLogStatus(row) }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="90">
          <template #default="{ row }">{{ accessLogDuration(row) }}</template>
        </el-table-column>
        <el-table-column label="错误" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ accessLogError(row) || '—' }}</template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="accessLogPage"
          v-model:page-size="accessLogPageSize"
          :total="accessLogTotal"
          layout="total, prev, pager, next, sizes"
          :page-sizes="[10, 20, 50]"
          @current-change="loadAccessLogs"
          @size-change="onAccessLogPageSizeChange"
        />
      </div>
      <el-empty v-if="!loadingAccessLogs && !accessLogs.length" description="暂无访问日志" />
    </el-card>

    <el-drawer v-model="accessLogDrawerVisible" title="访问日志详情" size="640px">
      <template v-if="selectedAccessLog">
        <el-descriptions :column="1" border size="small" class="access-log-detail-meta">
          <el-descriptions-item label="时间">{{ formatAccessLogTime(selectedAccessLog.createdAt || selectedAccessLog.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="方法">{{ accessLogMethod(selectedAccessLog) }}</el-descriptions-item>
          <el-descriptions-item label="路径">{{ accessLogPath(selectedAccessLog) }}</el-descriptions-item>
          <el-descriptions-item label="Query">{{ accessLogQuery(selectedAccessLog) || '—' }}</el-descriptions-item>
          <el-descriptions-item label="客户端 IP">{{ accessLogClientIP(selectedAccessLog) || '—' }}</el-descriptions-item>
          <el-descriptions-item label="匹配规则">{{ accessLogRouteLabel(selectedAccessLog) }}</el-descriptions-item>
          <el-descriptions-item label="状态码">{{ accessLogStatus(selectedAccessLog) }}</el-descriptions-item>
          <el-descriptions-item label="耗时">{{ accessLogDuration(selectedAccessLog) }}</el-descriptions-item>
          <el-descriptions-item label="错误">{{ accessLogError(selectedAccessLog) || '—' }}</el-descriptions-item>
        </el-descriptions>
        <div class="access-log-detail-block">
          <div class="form-section-header"><span>请求头</span></div>
          <pre class="detail-pre">{{ formatJSONValue(accessLogRequestHeaders(selectedAccessLog)) }}</pre>
        </div>
        <div class="access-log-detail-block">
          <div class="form-section-header"><span>请求 Body</span></div>
          <pre class="detail-pre">{{ accessLogRequestBody(selectedAccessLog) || '—' }}</pre>
        </div>
        <div class="access-log-detail-block">
          <div class="form-section-header"><span>响应头</span></div>
          <pre class="detail-pre">{{ formatJSONValue(accessLogResponseHeaders(selectedAccessLog)) }}</pre>
        </div>
        <div class="access-log-detail-block">
          <div class="form-section-header"><span>响应 Body</span></div>
          <pre class="detail-pre">{{ accessLogResponseBody(selectedAccessLog) || '—' }}</pre>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script>
import { clearMockAccessLogs, listMockAccessLogs, listMockServers } from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { formatMockServerAccessURL } from '../../utils/mockServer'

export default {
  name: 'MockServerAccessLogs',
  data() {
    return {
      server: null,
      loadingServer: false,
      accessLogs: [],
      accessLogTotal: 0,
      accessLogPage: 1,
      accessLogPageSize: 20,
      loadingAccessLogs: false,
      accessLogsFetching: false,
      clearingAccessLogs: false,
      accessLogAutoRefresh: false,
      accessLogRefreshTimer: null,
      accessLogFilterStatusFrom: '',
      accessLogFilterStatusTo: '',
      accessLogFilterMatched: '',
      accessLogFilterRange: null,
      accessLogDrawerVisible: false,
      selectedAccessLog: null
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    serverId() {
      return this.$route.params.serverId || ''
    }
  },
  watch: {
    projectId() {
      this.refresh()
    },
    serverId() {
      this.resetAccessLogFilters()
      this.stopAccessLogAutoRefresh()
      this.refresh()
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.refresh()
  },
  beforeUnmount() {
    this.stopAccessLogAutoRefresh()
  },
  methods: {
    goBack() {
      this.$router.push('/mock-servers')
    },
    async refresh() {
      await this.loadServer()
      this.syncAccessLogAutoRefresh()
      await this.loadAccessLogs()
    },
    async loadServer() {
      this.server = null
      if (!this.projectId || !this.serverId) return

      this.loadingServer = true
      try {
        const servers = await listMockServers(this.projectId)
        this.server = servers.find((item) => item.id === this.serverId) || null
        if (!this.server) {
          this.$message.warning('未找到该 Mock Server')
        }
      } finally {
        this.loadingServer = false
      }
    },
    serverPort(row) {
      return row?.port ?? row?.listenPort ?? row?.listen_port ?? ''
    },
    isRunning(row) {
      if (typeof row?.status?.running === 'boolean') return row.status.running
      if (typeof row?.running === 'boolean') return row.running
      if (typeof row?.isRunning === 'boolean') return row.isRunning
      const status = String(row?.status || row?.runtimeStatus || row?.runtime_status || '').toLowerCase()
      return ['running', 'started', 'active'].includes(status)
    },
    serverStatusLabel(row) {
      return this.isRunning(row) ? '运行中' : '已停止'
    },
    accessAddress(row) {
      return formatMockServerAccessURL(this.serverPort(row))
    },
    methodTagType(method) {
      const types = {
        GET: 'success',
        POST: 'primary',
        PUT: 'warning',
        PATCH: 'warning',
        DELETE: 'danger'
      }
      return types[method] || 'info'
    },
    formatJSONValue(value, fallback = '{}') {
      if (value == null || value === '') return fallback
      if (typeof value === 'string') {
        try {
          return JSON.stringify(JSON.parse(value), null, 2)
        } catch {
          return value
        }
      }
      return JSON.stringify(value, null, 2)
    },
    resetAccessLogFilters() {
      this.accessLogPage = 1
      this.accessLogFilterStatusFrom = ''
      this.accessLogFilterStatusTo = ''
      this.accessLogFilterMatched = ''
      this.accessLogFilterRange = null
    },
    onAccessLogFilterChange() {
      this.accessLogPage = 1
      this.loadAccessLogs()
    },
    onAccessLogPageSizeChange() {
      this.accessLogPage = 1
      this.loadAccessLogs()
    },
    buildAccessLogParams() {
      const params = {
        limit: this.accessLogPageSize,
        offset: (this.accessLogPage - 1) * this.accessLogPageSize
      }
      const statusFrom = Number(String(this.accessLogFilterStatusFrom || '').trim())
      if (Number.isInteger(statusFrom) && statusFrom >= 100 && statusFrom <= 599) {
        params.statusFrom = statusFrom
      }
      const statusTo = Number(String(this.accessLogFilterStatusTo || '').trim())
      if (Number.isInteger(statusTo) && statusTo >= 100 && statusTo <= 599) {
        params.statusTo = statusTo
      }
      if (this.accessLogFilterMatched === 'true' || this.accessLogFilterMatched === 'false') {
        params.matched = this.accessLogFilterMatched
      }
      if (this.accessLogFilterRange?.[0]) {
        params.from = this.accessLogFilterRange[0]
      }
      if (this.accessLogFilterRange?.[1]) {
        params.to = this.accessLogFilterRange[1]
      }
      return params
    },
    async loadAccessLogs(options = {}) {
      const silent = !!options.silent
      if (!this.projectId || !this.serverId) {
        this.accessLogs = []
        this.accessLogTotal = 0
        return
      }
      if (this.accessLogsFetching) {
        if (silent) return
      }
      this.accessLogsFetching = true
      if (!silent) {
        this.accessLogs = []
        this.loadingAccessLogs = true
      }
      try {
        const res = await listMockAccessLogs(this.projectId, this.serverId, this.buildAccessLogParams())
        this.accessLogs = res?.items || []
        this.accessLogTotal = res?.total ?? 0
      } finally {
        this.accessLogsFetching = false
        if (!silent) {
          this.loadingAccessLogs = false
        }
      }
    },
    onAccessLogAutoRefreshChange(enabled) {
      if (enabled && this.server && this.isRunning(this.server)) {
        this.startAccessLogAutoRefresh()
      } else {
        this.stopAccessLogAutoRefresh()
      }
    },
    syncAccessLogAutoRefresh() {
      const running = !!(this.server && this.isRunning(this.server))
      this.accessLogAutoRefresh = running
      if (running) {
        this.startAccessLogAutoRefresh()
      } else {
        this.stopAccessLogAutoRefresh()
      }
    },
    startAccessLogAutoRefresh() {
      this.stopAccessLogAutoRefresh()
      this.accessLogRefreshTimer = window.setInterval(() => {
        this.loadAccessLogs({ silent: true })
      }, 5000)
    },
    stopAccessLogAutoRefresh() {
      if (this.accessLogRefreshTimer) {
        window.clearInterval(this.accessLogRefreshTimer)
        this.accessLogRefreshTimer = null
      }
    },
    async confirmClearAccessLogs() {
      if (!this.projectId || !this.serverId) return
      try {
        await this.$confirm('确定清空当前 Mock Server 的全部访问日志？此操作不可恢复。', '清空访问日志', {
          type: 'warning',
          confirmButtonText: '清空',
          cancelButtonText: '取消'
        })
      } catch {
        return
      }
      this.clearingAccessLogs = true
      try {
        await clearMockAccessLogs(this.projectId, this.serverId)
        this.$message.success('访问日志已清空')
        this.accessLogPage = 1
        await this.loadAccessLogs()
      } finally {
        this.clearingAccessLogs = false
      }
    },
    openAccessLogDetail(row) {
      this.selectedAccessLog = row
      this.accessLogDrawerVisible = true
    },
    formatAccessLogTime(value) {
      if (!value) return '—'
      const date = new Date(value)
      if (Number.isNaN(date.getTime())) return String(value)
      return date.toLocaleString()
    },
    accessLogMethod(row) {
      return String(row?.method || 'GET').toUpperCase()
    },
    accessLogPath(row) {
      return row?.path || '/'
    },
    accessLogQuery(row) {
      return row?.query ?? row?.rawQuery ?? ''
    },
    accessLogClientIP(row) {
      return row?.clientIp ?? row?.client_ip ?? ''
    },
    accessLogStatus(row) {
      return row?.responseStatus ?? row?.response_status ?? '—'
    },
    accessLogDuration(row) {
      const ms = row?.durationMs ?? row?.duration_ms
      if (ms == null || ms === '') return '—'
      return `${ms} ms`
    },
    accessLogError(row) {
      return row?.errorMessage ?? row?.error_message ?? ''
    },
    accessLogRouteLabel(row) {
      if (row?.matched === false) return '未匹配'
      const method = row?.routeMethod || row?.route_method
      const path = row?.routePath || row?.route_path
      if (method && path) return `${method} ${path}`
      if (path) return path
      return row?.matched ? '已匹配' : '未匹配'
    },
    accessLogRequestHeaders(row) {
      return row?.requestHeaders ?? row?.request_headers ?? {}
    },
    accessLogRequestBody(row) {
      return row?.requestBody ?? row?.request_body ?? ''
    },
    accessLogResponseHeaders(row) {
      return row?.responseHeaders ?? row?.response_headers ?? {}
    },
    accessLogResponseBody(row) {
      return row?.responseBody ?? row?.response_body ?? ''
    }
  }
}
</script>

<style scoped>
.mock-server-access-logs {
  width: 100%;
  max-width: none;
  box-sizing: border-box;
}

.access-logs-header {
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
}

.page-title {
  margin: 8px 0 0;
  font-size: 20px;
}

.page-subtitle {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.access-address {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.header-actions {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.project-hint {
  margin-bottom: 16px;
}

.section-card {
  width: 100%;
}

.access-log-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.access-log-table {
  width: 100%;
  margin-bottom: 12px;
}

.access-log-detail-meta {
  margin-bottom: 16px;
}

.access-log-detail-block {
  margin-bottom: 16px;
}

.form-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 20px 0 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--app-border-color);
  color: var(--app-text-color);
  font-weight: 600;
}

.detail-pre {
  max-height: 260px;
  overflow: auto;
  margin: 0;
  padding: 10px;
  border: 1px solid var(--app-border-color);
  border-radius: 6px;
  background: var(--app-code-bg);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

:deep(.access-log-row-clickable) {
  cursor: pointer;
}
</style>
