<template>
  <el-drawer
    :model-value="visible"
    :title="drawerTitle"
    direction="rtl"
    size="380px"
    append-to-body
    destroy-on-close
    @close="onClose"
  >
    <div v-loading="loading" class="session-detail">
      <template v-if="detail">
        <section class="session-detail__section">
          <h3 class="session-detail__heading">会话信息</h3>
          <dl class="session-detail__dl">
            <dt>标题</dt>
            <dd>{{ detail.session?.title || '新对话' }}</dd>
            <dt>创建</dt>
            <dd>{{ formatTime(detail.session?.createdAt) }}</dd>
            <dt>最近活跃</dt>
            <dd>{{ formatTime(detail.session?.updatedAt) }}</dd>
            <dt v-if="conversationSpan">对话跨度</dt>
            <dd v-if="conversationSpan">{{ conversationSpan }}</dd>
          </dl>
        </section>

        <section class="session-detail__section">
          <h3 class="session-detail__heading">对话统计</h3>
          <div class="session-detail__stat-grid">
            <div v-for="item in conversationItems" :key="item.label" class="session-detail__stat-card">
              <span class="session-detail__stat-value">{{ item.value }}</span>
              <span class="session-detail__stat-label">{{ item.label }}</span>
            </div>
          </div>
          <p v-if="modelsLabel" class="session-detail__note">使用模型：{{ modelsLabel }}</p>
        </section>

        <section class="session-detail__section">
          <h3 class="session-detail__heading">Token 消耗汇总</h3>
          <p v-if="detail.tokenUsage?.hint && !detail.tokenUsage?.hasUsageData" class="session-detail__hint">
            {{ detail.tokenUsage.hint }}
          </p>
          <template v-else-if="detail.tokenUsage?.hasUsageData">
            <div class="session-detail__stat-grid">
              <div v-for="item in tokenItems" :key="item.label" class="session-detail__stat-card">
                <span class="session-detail__stat-value">{{ item.value }}</span>
                <span class="session-detail__stat-label">{{ item.label }}</span>
              </div>
            </div>
            <p class="session-detail__note">
              共 {{ fmt(detail.tokenUsage.roundsWithUsage) }} 轮 assistant 调用含用量记录
            </p>
          </template>
        </section>
      </template>
      <p v-else-if="!loading && error" class="session-detail__error">{{ error }}</p>
    </div>
  </el-drawer>
</template>

<script>
import { getAISessionStats } from '../api'
import { buildCacheStatItems } from '../utils/aiCacheUsage'
import { formatSessionTime } from '../utils/useRelativeTime'

export default {
  name: 'AISessionDetailDrawer',
  props: {
    modelValue: { type: Boolean, default: false },
    projectId: { type: String, default: '' },
    sessionId: { type: String, default: '' },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      loading: false,
      error: '',
      detail: null,
    }
  },
  computed: {
    visible: {
      get() {
        return this.modelValue
      },
      set(value) {
        this.$emit('update:modelValue', value)
      },
    },
    drawerTitle() {
      const title = this.detail?.session?.title || '会话详情'
      return `会话详情 · ${title}`
    },
    conv() {
      return this.detail?.conversation || {}
    },
    conversationItems() {
      const c = this.conv
      return [
        { label: '消息总数', value: this.fmt(c.totalMessages) },
        { label: '用户消息', value: this.fmt(c.userMessages) },
        { label: '助理回复', value: this.fmt(c.assistantMessages) },
        { label: '工具结果', value: this.fmt(c.toolMessages) },
        { label: '工具调用', value: this.fmt(c.toolCallCount) },
        { label: '待确认', value: this.fmt(c.pendingConfirmMessages) },
        { label: '助理耗时', value: this.formatDuration(c.totalElapsedMillis) },
      ].filter((item) => item.value !== '-' || item.label === '消息总数')
    },
    tokenItems() {
      const t = this.detail?.tokenUsage || {}
      const items = [
        { label: '输入 Token', value: this.fmt(t.promptTokens) },
        { label: '输出 Token', value: this.fmt(t.completionTokens) },
        { label: '合计 Token', value: this.fmt(t.totalTokens) },
      ]
      items.push(...buildCacheStatItems(t, (n) => this.fmt(n)))
      if (t.reasoningTokens) items.push({ label: '推理 Token', value: this.fmt(t.reasoningTokens) })
      if (t.usageElapsedMillis) {
        items.push({ label: '记录耗时', value: this.formatDuration(t.usageElapsedMillis) })
      }
      return items
    },
    modelsLabel() {
      const models = this.conv.modelsUsed
      if (!Array.isArray(models) || !models.length) return ''
      return models.join('、')
    },
    conversationSpan() {
      const first = this.conv.firstMessageAt
      const last = this.conv.lastMessageAt
      if (!first || !last) return ''
      const a = new Date(first).getTime()
      const b = new Date(last).getTime()
      if (!Number.isFinite(a) || !Number.isFinite(b) || b < a) return ''
      return this.formatDuration(b - a)
    },
  },
  watch: {
    visible(value) {
      if (value) this.load()
      else {
        this.detail = null
        this.error = ''
      }
    },
    sessionId() {
      if (this.visible) this.load()
    },
  },
  methods: {
    onClose() {
      this.visible = false
    },
    async load() {
      if (!this.projectId || !this.sessionId) {
        this.error = '缺少项目或会话 ID'
        return
      }
      this.loading = true
      this.error = ''
      try {
        this.detail = await getAISessionStats(this.projectId, this.sessionId)
      } catch (e) {
        this.detail = null
        this.error = e?.message || '加载会话详情失败'
      } finally {
        this.loading = false
      }
    },
    fmt(n) {
      const v = Number(n)
      if (!Number.isFinite(v)) return '-'
      return v.toLocaleString('zh-CN')
    },
    formatTime(value) {
      if (!value) return '-'
      return formatSessionTime(value)
    },
    formatDuration(ms) {
      const v = Number(ms)
      if (!Number.isFinite(v) || v <= 0) return '-'
      if (v < 1000) return `${Math.round(v)} ms`
      if (v < 60000) return `${(v / 1000).toFixed(1)} s`
      const m = Math.floor(v / 60000)
      const s = Math.round((v % 60000) / 1000)
      return s ? `${m} 分 ${s} 秒` : `${m} 分`
    },
  },
}
</script>

<style scoped>
.session-detail {
  min-height: 120px;
  padding: 0 4px 24px;
}

.session-detail__section {
  margin-bottom: 24px;
}

.session-detail__heading {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-color);
}

.session-detail__dl {
  display: grid;
  grid-template-columns: 72px 1fr;
  gap: 8px 12px;
  margin: 0;
  font-size: 13px;
}

.session-detail__dl dt {
  margin: 0;
  color: var(--app-text-muted);
}

.session-detail__dl dd {
  margin: 0;
  color: var(--app-text-color);
  word-break: break-word;
}

.session-detail__stat-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.session-detail__stat-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--app-surface-subtle);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.session-detail__stat-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--app-text-color);
  line-height: 1.2;
}

.session-detail__stat-label {
  font-size: 12px;
  color: var(--app-text-muted);
}

.session-detail__note,
.session-detail__hint {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--app-text-muted);
}

.session-detail__hint {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--el-color-warning-light-9, #fdf6ec);
  color: var(--el-color-warning-dark-2, #b88230);
}

.session-detail__error {
  margin: 0;
  font-size: 13px;
  color: var(--el-color-danger);
}
</style>
