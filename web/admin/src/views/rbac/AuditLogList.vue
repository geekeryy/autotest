<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">审计日志</h2>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>

    <p class="page-hint">
      记录用户登录成功与失败事件，包括本地密码与 OAuth 方式。本地密码登录失败时会记录尝试使用的密码，仅供管理员排查。
    </p>

    <div class="filters">
      <el-select v-model="filters.success" clearable placeholder="结果" style="width: 120px" @change="onFilterChange">
        <el-option label="成功" value="true" />
        <el-option label="失败" value="false" />
      </el-select>
      <el-input
        v-model="filters.username"
        clearable
        placeholder="用户名"
        style="width: 180px"
        @keyup.enter="onFilterChange"
        @clear="onFilterChange"
      />
      <el-button type="primary" @click="onFilterChange">查询</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-table v-loading="loading" :data="items" border>
      <el-table-column label="时间" min-width="170">
        <template #default="{ row }">
          {{ formatDateTime(row.createdAt) }}
        </template>
      </el-table-column>
      <el-table-column prop="actorUsername" label="用户" min-width="140">
        <template #default="{ row }">
          <span>{{ row.actorUsername || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="方式" min-width="120">
        <template #default="{ row }">
          {{ methodLabel(row.resource) }}
        </template>
      </el-table-column>
      <el-table-column label="结果" width="90">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" size="small">
            {{ row.success ? '成功' : '失败' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="140">
        <template #default="{ row }">
          <span v-if="row.success" class="muted">-</span>
          <span v-else>{{ failureReasonLabel(row.detail) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="尝试密码" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.success || !row.detail?.attemptedPassword" class="muted">-</span>
          <code v-else>{{ row.detail.attemptedPassword }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="clientIp" label="IP" min-width="130" />
      <el-table-column label="User-Agent" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="muted">{{ row.userAgent || '-' }}</span>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="onPageSizeChange"
      />
    </div>
  </div>
</template>

<script>
import { listAuditLogs } from '../../api'
import ListLoadError from '../../components/ListLoadError.vue'
import { formatDateTime } from '../../utils/datetime'

const FAILURE_REASON_LABELS = {
  user_not_found: '用户不存在',
  account_inactive: '账号未启用',
  password_not_set: '未设置密码',
  invalid_password: '密码错误',
  oauth_disabled: 'OAuth 未配置',
  invalid_state: 'OAuth state 无效',
  missing_code: '缺少授权码',
  exchange_failed: '授权码交换失败',
  profile_failed: '获取用户信息失败',
  registration_disabled: '禁止自动注册',
  invalid_frontend_url: '前端地址无效',
  oauth_failed: 'OAuth 失败',
  token_issue_failed: '签发令牌失败'
}

export default {
  name: 'AuditLogList',
  components: { ListLoadError },
  data() {
    return {
      loading: false,
      loadError: '',
      items: [],
      total: 0,
      page: 1,
      pageSize: 50,
      filters: {
        success: '',
        username: ''
      }
    }
  },
  mounted() {
    this.load()
  },
  methods: {
    formatDateTime,
    methodLabel(resource) {
      if (!resource) return '-'
      if (resource === 'local') return '本地密码'
      if (resource.startsWith('oauth:')) return `OAuth (${resource.slice(6)})`
      return resource
    },
    failureReasonLabel(detail) {
      const reason = detail?.reason
      if (!reason) return '-'
      return FAILURE_REASON_LABELS[reason] || reason
    },
    onFilterChange() {
      this.page = 1
      this.load()
    },
    onPageSizeChange() {
      this.page = 1
      this.load()
    },
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        const params = {
          action: 'auth.login',
          limit: this.pageSize,
          offset: (this.page - 1) * this.pageSize
        }
        if (this.filters.success !== '') params.success = this.filters.success
        if (this.filters.username.trim()) params.username = this.filters.username.trim()
        const resp = await listAuditLogs(params)
        this.items = resp.items || []
        this.total = resp.total ?? 0
      } catch (e) {
        this.loadError = e?.message || '加载审计日志失败'
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.page-hint {
  margin: 0 0 16px;
  color: var(--app-secondary-text);
  line-height: 1.6;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.muted {
  color: var(--app-secondary-text);
}
</style>
