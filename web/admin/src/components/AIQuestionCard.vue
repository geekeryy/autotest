<!--
  AIQuestionCard
  ==============
  Interactive question card rendered in the action bar for `ask_question` tool
  calls. Supports single-select, multi-select, and text input modes.
-->
<template>
  <div class="ai-question">
    <div class="ai-question__header">
      <el-icon class="ai-question__icon"><ChatDotRound /></el-icon>
      <span class="ai-question__text">{{ question }}</span>
    </div>

    <!-- Single select: click to auto-submit -->
    <div v-if="type === 'single_select'" class="ai-question__options">
      <button
        v-for="opt in options"
        :key="opt.value"
        type="button"
        class="ai-question__option"
        :class="{ 'ai-question__option--selected': selectedSingle === opt.value }"
        :disabled="disabled || submitting"
        @click="onSingleSelect(opt)"
      >
        <span class="ai-question__option-label">{{ opt.label }}</span>
        <span v-if="opt.description" class="ai-question__option-desc">{{ opt.description }}</span>
      </button>
    </div>

    <!-- Multi select: checkboxes + confirm button -->
    <div v-else-if="type === 'multi_select'" class="ai-question__options">
      <label
        v-for="opt in options"
        :key="opt.value"
        class="ai-question__option ai-question__option--checkable"
        :class="{ 'ai-question__option--checked': selectedMulti.has(opt.value) }"
      >
        <el-checkbox
          :model-value="selectedMulti.has(opt.value)"
          :disabled="disabled || submitting"
          @change="(val) => toggleMulti(opt.value, val)"
        />
        <span class="ai-question__option-label">{{ opt.label }}</span>
        <span v-if="opt.description" class="ai-question__option-desc">{{ opt.description }}</span>
      </label>
      <div class="ai-question__multi-actions">
        <el-button
          size="small"
          type="primary"
          :disabled="disabled || submitting || (!isOptional && selectedMulti.size === 0)"
          :loading="submitting"
          @click="onMultiSubmit"
        >
          确认选择
        </el-button>
      </div>
    </div>

    <!-- Text input -->
    <div v-else-if="type === 'text_input'" class="ai-question__input-area">
      <el-input
        v-model="textValue"
        type="textarea"
        :rows="3"
        :placeholder="placeholder || '请输入...'"
        :disabled="disabled || submitting"
        resize="none"
        @keydown.ctrl.enter.prevent="onTextSubmit"
      />
      <div class="ai-question__input-actions">
        <span class="ai-question__input-hint">Ctrl+Enter 提交</span>
        <el-button
          size="small"
          type="primary"
          :disabled="disabled || submitting || (!isOptional && !textValue.trim())"
          :loading="submitting"
          @click="onTextSubmit"
        >
          提交
        </el-button>
      </div>
    </div>

    <!-- Skip button for optional questions -->
    <div v-if="isOptional" class="ai-question__skip">
      <button
        type="button"
        class="ai-question__skip-btn"
        :disabled="disabled || submitting"
        @click="onSkip"
      >
        跳过此问题
      </button>
    </div>
  </div>
</template>

<script>
import { ChatDotRound } from '@element-plus/icons-vue'

function parseArgs(raw) {
  if (raw == null) return {}
  try {
    return typeof raw === 'string' ? JSON.parse(raw) : { ...raw }
  } catch {
    return {}
  }
}

export default {
  name: 'AIQuestionCard',
  components: { ChatDotRound },
  props: {
    call: { type: Object, required: true },
    disabled: { type: Boolean, default: false },
  },
  emits: ['decide'],
  data() {
    return {
      selectedSingle: null,
      selectedMulti: new Set(),
      textValue: '',
      submitting: false,
    }
  },
  computed: {
    parsed() {
      return parseArgs(this.call?.arguments)
    },
    question() {
      return this.parsed.question || '请选择'
    },
    type() {
      return this.parsed.type || 'single_select'
    },
    options() {
      return Array.isArray(this.parsed.options) ? this.parsed.options : []
    },
    isOptional() {
      return this.parsed.required === false
    },
    placeholder() {
      return this.parsed.placeholder || ''
    },
  },
  methods: {
    async onSingleSelect(opt) {
      this.selectedSingle = opt.value
      await this.submitAnswer(opt.value)
    },
    toggleMulti(value, checked) {
      const next = new Set(this.selectedMulti)
      if (checked) next.add(value)
      else next.delete(value)
      this.selectedMulti = next
    },
    async onMultiSubmit() {
      await this.submitAnswer([...this.selectedMulti])
    },
    async onTextSubmit() {
      if (!this.textValue.trim() && !this.isOptional) return
      await this.submitAnswer(this.textValue.trim())
    },
    async onSkip() {
      await this.submitAnswer(null)
    },
    async submitAnswer(answer) {
      this.submitting = true
      try {
        this.$emit('decide', {
          callId: this.call.id,
          approve: true,
          reason: '',
          argumentOverrides: { answer },
        })
      } finally {
        this.submitting = false
      }
    },
  },
}
</script>

<style scoped>
.ai-question {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--el-border-color-light, #e4e7ed);
  border-radius: 8px;
  background: var(--el-fill-color-blank, #fff);
}

.ai-question__header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.ai-question__icon {
  font-size: 18px;
  color: var(--el-color-primary, #409eff);
  flex-shrink: 0;
  margin-top: 1px;
}

.ai-question__text {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary, #303133);
  line-height: 1.5;
}

.ai-question__options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ai-question__option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--el-border-color-light, #e4e7ed);
  border-radius: 6px;
  background: var(--el-fill-color-blank, #fff);
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
  text-align: left;
  width: 100%;
}

.ai-question__option:hover:not(:disabled) {
  border-color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-question__option:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ai-question__option--selected {
  border-color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-question__option--checkable {
  cursor: pointer;
}

.ai-question__option--checked {
  border-color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-question__option-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary, #303133);
  flex: 1;
}

.ai-question__option-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  line-height: 1.4;
}

.ai-question__multi-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

.ai-question__input-area {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-question__input-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ai-question__input-hint {
  font-size: 11px;
  color: var(--el-text-color-placeholder, #a8abb2);
}

.ai-question__skip {
  display: flex;
  justify-content: flex-start;
}

.ai-question__skip-btn {
  border: none;
  background: transparent;
  color: var(--el-text-color-secondary, #909399);
  font-size: 12px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: color 0.2s ease, background 0.2s ease;
}

.ai-question__skip-btn:hover:not(:disabled) {
  color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.ai-question__skip-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
