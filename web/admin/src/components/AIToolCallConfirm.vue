<!--
  AIToolCallConfirm
  =================
  Renders a single pending mutating tool call (e.g. update_case_assertions)
  with the model's proposed arguments and approve/reject controls. Emits
  `decide` with the user's choice; the parent component is responsible for
  forwarding to the store.

  Note: we do not validate `arguments` here — the backend tool's own JSON
  schema gate handles malformed payloads. The UI's job is to make the
  proposed change reviewable, not to second-guess the model.
-->
<template>
  <div class="tool-call-confirm" :class="{ 'tool-call-confirm--inline': inline }">
    <button
      type="button"
      class="tool-call-confirm-head"
      :aria-expanded="argsExpanded"
      @click="argsExpanded = !argsExpanded"
    >
      <el-icon class="tool-chevron" :class="{ 'tool-chevron--expanded': argsExpanded }">
        <ArrowRight />
      </el-icon>
      <span class="tool-call-confirm-title">即将执行写操作：{{ toolDisplayName }}</span>
    </button>
    <pre v-show="argsExpanded" class="tool-call-confirm-args">{{ prettyArgs }}</pre>
    <div v-if="rejectExpanded" class="tool-call-confirm-reason">
      <el-input
        v-model="reason"
        type="textarea"
        :rows="2"
        placeholder="说明拒绝原因（可选，将作为下一轮上下文提供给 AI）"
      />
    </div>
    <div class="tool-call-confirm-actions">
      <el-button v-if="!rejectExpanded" :disabled="disabled" size="small" @click="onToggleReject">拒绝</el-button>
      <template v-else>
        <el-button size="small" @click="onCancelReject">取消</el-button>
        <el-button size="small" type="danger" :loading="busy" :disabled="disabled" @click="onReject">
          确认拒绝
        </el-button>
      </template>
      <el-button size="small" type="primary" :loading="busy" :disabled="disabled" @click="onApprove">
        允许执行
      </el-button>
    </div>
  </div>
</template>

<script>
import { ArrowRight } from '@element-plus/icons-vue'
import { getToolDisplayName } from '../utils/aiToolLabels'

export default {
  name: 'AIToolCallConfirm',
  components: { ArrowRight },
  props: {
    /** @type {{ id: string, name: string, arguments: any, mutating?: boolean }} */
    call: { type: Object, required: true },
    disabled: { type: Boolean, default: false },
    inline: { type: Boolean, default: false },
  },
  emits: ['decide'],
  data() {
    return {
      reason: '',
      rejectExpanded: false,
      argsExpanded: false,
      busy: false,
    }
  },
  computed: {
    toolDisplayName() {
      return getToolDisplayName(this.call?.name)
    },
    prettyArgs() {
      const raw = this.call?.arguments
      if (raw == null) return '{}'
      try {
        const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
        return JSON.stringify(parsed, null, 2)
      } catch {
        return String(raw)
      }
    },
  },
  methods: {
    onToggleReject() {
      this.rejectExpanded = true
    },
    onCancelReject() {
      this.rejectExpanded = false
      this.reason = ''
    },
    async onApprove() {
      this.busy = true
      try {
        await this.emitDecision(true, '')
      } finally {
        this.busy = false
      }
    },
    async onReject() {
      this.busy = true
      try {
        await this.emitDecision(false, this.reason)
      } finally {
        this.busy = false
      }
    },
    async emitDecision(approve, reason) {
      return new Promise((resolve) => {
        this.$emit('decide', { callId: this.call.id, approve, reason })
        resolve()
      })
    },
  },
}
</script>

<style scoped>
.tool-call-confirm {
  background: var(--el-fill-color-blank, #fff);
  border: 1px solid var(--el-color-warning, #e6a23c);
  border-radius: 8px;
  padding: 6px 10px 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tool-call-confirm--inline {
  border-color: var(--app-border-color);
  box-shadow: 0 1px 0 rgba(15, 23, 42, 0.04);
}

.tool-call-confirm-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-height: 32px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--el-text-color-primary, #303133);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
}

.tool-call-confirm-head:hover {
  opacity: 0.9;
}

.tool-chevron {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  transition: transform 0.15s ease;
}

.tool-chevron--expanded {
  transform: rotate(90deg);
}

.tool-call-confirm-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.tool-call-confirm-args {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 6px;
  padding: 6px 8px;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--el-text-color-regular, #606266);
}

.tool-call-confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
</style>
