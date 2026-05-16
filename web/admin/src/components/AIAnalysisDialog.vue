<!--
  AIAnalysisDialog
  =================
  Lightweight presentation-only dialog used by the "AI 智能分析" feature
  (see docs/requirements.md). It does NOT call the AI itself: the parent
  component is expected to await the analysis API (analyzeRunFailure /
  analyzeSpecChanges) and pass `text` / `model` / `elapsedMillis` /
  `loading` / `error` in via props. This keeps the dialog reusable for any
  future analysis action without dragging in provider/prompt loading.

  Why a separate component instead of reusing AIGenerateDialog?
    - AIGenerateDialog owns provider + prompt loading and auto-runs on open
      (with intent capture for assertion). Analysis flows already have the
      result by the time the dialog is shown, so reusing it would add
      avoidable round-trips and tangled state.
-->
<template>
  <el-dialog
    v-model="visible"
    :title="title || 'AI 智能分析'"
    width="780px"
    align-center
    destroy-on-close
    class="ai-analysis-dialog"
  >
    <div v-if="loading" class="ai-analysis-loading">
      <el-icon class="ai-spin"><Loading /></el-icon>
      <span>AI 分析中，请稍候…</span>
    </div>

    <el-alert
      v-else-if="error"
      type="error"
      show-icon
      :closable="false"
      :title="error"
    />

    <el-alert
      v-else-if="info"
      type="info"
      show-icon
      :closable="false"
      :title="info"
    />

    <div v-else-if="text" class="ai-analysis-result">
      <div class="ai-analysis-meta">
        <span>模型：<strong>{{ model || '-' }}</strong></span>
        <span class="ai-analysis-meta-sep">·</span>
        <span>耗时：<strong>{{ elapsedMillis || 0 }} ms</strong></span>
      </div>
      <div class="ai-analysis-body">
        <MarkdownView :source="text" />
      </div>
    </div>

    <el-empty v-else description="暂无分析结果" :image-size="48" />

    <template #footer>
      <el-button v-if="text && !loading" @click="copyText">复制</el-button>
      <el-button v-if="!loading && allowRetry" type="primary" plain @click="$emit('retry')">重新分析</el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script>
import { Loading } from '@element-plus/icons-vue'
import MarkdownView from './MarkdownView.vue'

export default {
  name: 'AIAnalysisDialog',
  components: { Loading, MarkdownView },
  props: {
    modelValue: { type: Boolean, default: false },
    title: { type: String, default: '' },
    loading: { type: Boolean, default: false },
    text: { type: String, default: '' },
    model: { type: String, default: '' },
    elapsedMillis: { type: Number, default: 0 },
    error: { type: String, default: '' },
    info: { type: String, default: '' },
    allowRetry: { type: Boolean, default: true }
  },
  emits: ['update:modelValue', 'retry'],
  computed: {
    visible: {
      get() {
        return this.modelValue
      },
      set(val) {
        this.$emit('update:modelValue', val)
      }
    }
  },
  methods: {
    async copyText() {
      if (!this.text) return
      try {
        await navigator.clipboard.writeText(this.text)
        this.$message.success('已复制到剪贴板')
      } catch (_) {
        this.$message.warning('当前环境不支持剪贴板，请手动选择复制')
      }
    }
  }
}
</script>

<style scoped>
.ai-analysis-loading {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 36px 0;
  justify-content: center;
  color: var(--app-secondary-text, #909399);
  font-size: var(--app-font-size-base);
}

.ai-spin {
  font-size: calc(var(--app-font-size-base) + 8px);
  animation: ai-analysis-rotating 1s linear infinite;
}

@keyframes ai-analysis-rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.ai-analysis-result {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ai-analysis-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text, #909399);
}

.ai-analysis-meta-sep {
  opacity: 0.6;
}

.ai-analysis-meta strong {
  color: var(--el-text-color-primary, #303133);
  font-weight: 500;
}

.ai-analysis-body {
  padding: 14px 16px;
  background: var(--el-fill-color-lighter, #fafafa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 6px;
  max-height: 60vh;
  overflow: auto;
}
</style>
