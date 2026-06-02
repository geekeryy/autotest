<template>
  <div
    class="ai-thinking-block"
    :class="{
      'ai-thinking-block--active': active,
      'ai-thinking-block--expanded': expanded,
    }"
  >
    <button
      type="button"
      class="ai-thinking-block-head"
      :aria-expanded="expanded"
      @click="toggle"
    >
      <el-icon
        class="ai-thinking-chevron"
        :class="{ 'ai-thinking-chevron--expanded': expanded }"
      >
        <ArrowRight />
      </el-icon>
      <span v-if="active" class="ai-thinking-orb" />
      <span class="ai-thinking-block-label">
        <template v-if="active">正在深度思考</template>
        <template v-else>已思考 {{ elapsedLabel }}</template>
      </span>
      <span v-if="elapsedLabel" class="ai-thinking-block-elapsed">{{
        elapsedLabel
      }}</span>
    </button>
    <div v-show="expanded" class="ai-thinking-block-body">
      <MarkdownView v-if="content" :source="content" />
      <span v-else-if="active" class="ai-thinking-block-placeholder"
        >思考中...</span
      >
    </div>
  </div>
</template>

<script>
import { ArrowRight } from '@element-plus/icons-vue'
import MarkdownView from './MarkdownView.vue'

export default {
  name: 'AIThinkingBlock',
  components: { ArrowRight, MarkdownView },
  props: {
    active: { type: Boolean, default: false },
    content: { type: String, default: '' },
    elapsedMillis: { type: Number, default: 0 },
    startedAt: { type: Number, default: 0 },
  },
  data() {
    return {
      expanded: this.active,
      nowTick: Date.now(),
      tickTimer: null,
    }
  },
  computed: {
    elapsedLabel() {
      const started = Number(this.startedAt) || 0
      let elapsed = Number(this.elapsedMillis) || 0
      if (this.active && started > 0) {
        elapsed = Math.max(0, this.nowTick - started)
      }
      const seconds = Math.floor(elapsed / 1000)
      if (seconds <= 0) return ''
      return `${seconds}s`
    },
  },
  watch: {
    active(val) {
      if (val) {
        this.expanded = true
        this.startTick()
      } else {
        this.stopTick()
      }
    },
  },
  mounted() {
    if (this.active) this.startTick()
  },
  beforeUnmount() {
    this.stopTick()
  },
  methods: {
    toggle() {
      this.expanded = !this.expanded
    },
    startTick() {
      this.stopTick()
      this.tickTimer = window.setInterval(() => {
        this.nowTick = Date.now()
      }, 1000)
    },
    stopTick() {
      if (this.tickTimer) {
        window.clearInterval(this.tickTimer)
        this.tickTimer = null
      }
    },
  },
}
</script>

<style scoped>
.ai-thinking-block {
  background: linear-gradient(
    135deg,
    var(--el-color-primary-light-9, #ecf5ff),
    var(--el-fill-color-light, #f5f7fa)
  );
  border: 1px solid
    color-mix(
      in srgb,
      var(--el-color-primary, #409eff) 20%,
      var(--el-border-color-lighter, #ebeef5)
    );
  border-radius: 8px;
  overflow: hidden;
}

.ai-thinking-block-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 32px;
  padding: 4px 10px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-size: 12px;
  color: var(--el-text-color-regular, #606266);
}

.ai-thinking-block-head:hover {
  background: rgba(0, 0, 0, 0.03);
}

.ai-thinking-chevron {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  transition: transform 0.15s ease;
}

.ai-thinking-chevron--expanded {
  transform: rotate(90deg);
}

.ai-thinking-orb {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 0 rgba(64, 158, 255, 0.35);
  animation: thinking-pulse 1.4s ease-in-out infinite;
}

.ai-thinking-block-label {
  font-weight: 500;
}

.ai-thinking-block-elapsed {
  color: var(--el-text-color-placeholder, #a8abb2);
  font-variant-numeric: tabular-nums;
  margin-left: auto;
}

.ai-thinking-block-body {
  border-top: 1px solid
    color-mix(
      in srgb,
      var(--el-color-primary, #409eff) 10%,
      var(--el-border-color-extra-light, #f2f6fc)
    );
  padding: 8px 12px 10px 24px;
  max-height: 400px;
  overflow-y: auto;
}

.ai-thinking-block-body :deep(.markdown-body) {
  background: transparent;
  padding: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-secondary, #606266);
}

.ai-thinking-block-placeholder {
  color: var(--el-text-color-placeholder, #a8abb2);
  font-size: 13px;
}

@keyframes thinking-pulse {
  0% {
    transform: scale(0.85);
    box-shadow: 0 0 0 0 rgba(64, 158, 255, 0.35);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 8px rgba(64, 158, 255, 0);
  }
  100% {
    transform: scale(0.85);
    box-shadow: 0 0 0 0 rgba(64, 158, 255, 0);
  }
}
</style>
