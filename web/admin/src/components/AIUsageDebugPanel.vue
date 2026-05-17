<template>
  <div v-if="usage" class="ai-usage-debug">
    <div class="ai-usage-debug__head">
      <span class="ai-usage-debug__title">Token 明细</span>
      <span v-if="hopLabel" class="ai-usage-debug__hop">{{ hopLabel }}</span>
      <span v-if="usage.model" class="ai-usage-debug__model">{{ usage.model }}</span>
    </div>
    <dl class="ai-usage-debug__grid">
      <template v-if="usage.promptTokens != null">
        <dt>输入</dt>
        <dd>{{ fmt(usage.promptTokens) }}</dd>
      </template>
      <template v-if="usage.completionTokens != null">
        <dt>输出</dt>
        <dd>{{ fmt(usage.completionTokens) }}</dd>
      </template>
      <template v-if="usage.totalTokens != null">
        <dt>合计</dt>
        <dd>{{ fmt(usage.totalTokens) }}</dd>
      </template>
      <template v-if="cacheHitTotal">
        <dt>缓存命中</dt>
        <dd>{{ fmt(cacheHitTotal) }}</dd>
      </template>
      <template v-if="cacheMissTotal">
        <dt>缓存未命中</dt>
        <dd>{{ fmt(cacheMissTotal) }}</dd>
      </template>
      <template v-if="cacheWriteTotal">
        <dt>缓存写入</dt>
        <dd>{{ fmt(cacheWriteTotal) }}</dd>
      </template>
      <template v-if="usage.reasoningTokens">
        <dt>推理 Token</dt>
        <dd>{{ fmt(usage.reasoningTokens) }}</dd>
      </template>
      <template v-if="usage.elapsedMillis">
        <dt>耗时</dt>
        <dd>{{ (usage.elapsedMillis / 1000).toFixed(2) }}s</dd>
      </template>
    </dl>
    <details v-if="showRaw" class="ai-usage-debug__raw">
      <summary>上游原始 usage</summary>
      <pre>{{ rawJson }}</pre>
    </details>
  </div>
</template>

<script>
import { cacheHitTokens, cacheMissTokens, cacheWriteTokens } from '../utils/aiCacheUsage'

export default {
  name: 'AIUsageDebugPanel',
  props: {
    usage: { type: Object, default: null },
  },
  computed: {
    hopLabel() {
      const hop = Number(this.usage?.hop)
      if (!hop) return ''
      return `第 ${hop} 轮`
    },
    cacheHitTotal() {
      return cacheHitTokens(this.usage)
    },
    cacheMissTotal() {
      return cacheMissTokens(this.usage)
    },
    cacheWriteTotal() {
      return cacheWriteTokens(this.usage)
    },
    showRaw() {
      return !!this.usage?.providerRaw
    },
    rawJson() {
      const raw = this.usage?.providerRaw
      if (!raw) return ''
      if (typeof raw === 'string') {
        try {
          return JSON.stringify(JSON.parse(raw), null, 2)
        } catch {
          return raw
        }
      }
      try {
        return JSON.stringify(raw, null, 2)
      } catch {
        return String(raw)
      }
    },
  },
  methods: {
    fmt(n) {
      const v = Number(n)
      if (!Number.isFinite(v)) return '-'
      return v.toLocaleString('zh-CN')
    },
  },
}
</script>

<style scoped>
.ai-usage-debug {
  margin-top: 8px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px dashed var(--el-border-color, #dcdfe6);
  background: var(--el-fill-color-lighter, #fafafa);
  font-size: 12px;
  color: var(--el-text-color-secondary, #606266);
}

.ai-usage-debug__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.ai-usage-debug__title {
  font-weight: 600;
  color: var(--el-text-color-primary, #303133);
}

.ai-usage-debug__hop,
.ai-usage-debug__model {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--el-fill-color, #f0f2f5);
  font-size: 11px;
}

.ai-usage-debug__grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 12px;
  margin: 0;
}

.ai-usage-debug__grid dt {
  margin: 0;
  color: var(--el-text-color-placeholder, #a8abb2);
}

.ai-usage-debug__grid dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
}

.ai-usage-debug__raw {
  margin-top: 8px;
}

.ai-usage-debug__raw summary {
  cursor: pointer;
  color: var(--el-color-primary, #409eff);
}

.ai-usage-debug__raw pre {
  margin: 6px 0 0;
  padding: 8px;
  border-radius: 6px;
  background: var(--el-fill-color, #f5f7fa);
  overflow-x: auto;
  font-size: 11px;
  line-height: 1.4;
}
</style>
