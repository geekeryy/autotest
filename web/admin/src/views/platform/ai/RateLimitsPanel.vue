<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;">
      <span style="color:#909399;font-size:13px;">AI 操作的速率限制规则。修改后立即生效。</span>
      <el-button @click="cleanup" :loading="cleaning">清理过期记录</el-button>
    </div>

    <el-table :data="rules" v-loading="loading" stripe>
      <el-table-column prop="name" label="规则名" width="220" />
      <el-table-column prop="key" label="Key" width="180" />
      <el-table-column prop="limit" label="限制次数" width="100" align="center" />
      <el-table-column label="窗口" width="100" align="center">
        <template #default="{ row }">{{ row.window }}s</template>
      </el-table-column>
      <el-table-column label="状态" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
import { listRateLimitRules, cleanupRateLimitEntries } from '../../../api'

export default {
  name: 'RateLimitsPanel',
  data() {
    return { rules: [], loading: false, cleaning: false }
  },
  mounted() { this.load() },
  methods: {
    async load() {
      this.loading = true
      try {
        this.rules = await listRateLimitRules()
      } finally { this.loading = false }
    },
    async cleanup() {
      this.cleaning = true
      try {
        const res = await cleanupRateLimitEntries()
        this.$message.success(`已清理 ${res.deleted || 0} 条过期记录`)
      } finally { this.cleaning = false }
    }
  }
}
</script>
