<!--
  AIActionBar
  ===========
  Bottom action bar displayed above the composer when there are pending
  tool calls requiring user confirmation.

  Three display modes:
  1. Question cards (ask_question) — interactive single/multi select or text input
  2. Aggregated quick-action chips for simple tool calls
  3. Complex tool calls (scenario gen) shown individually with expandable forms
-->
<template>
  <div class="ai-action-bar">
    <!-- Question cards: shown one at a time (sequential) -->
    <template v-if="currentQuestion">
      <AIQuestionCard
        :call="currentQuestion"
        :disabled="disabled"
        @decide="onQuestionDecide"
      />
      <div v-if="remainingQuestions.length" class="ai-action-bar__queue">
        还有 {{ remainingQuestions.length }} 个问题等待回答
      </div>
    </template>

    <!-- Non-question calls: aggregated options -->
    <template v-else>
      <div v-if="simpleGroups.length" class="ai-action-bar__options">
        <button
          v-for="group in simpleGroups"
          :key="group.key"
          type="button"
          class="ai-action-bar__option"
          :class="{ 'ai-action-bar__option--loading': batchLoadingKey === group.key }"
          :disabled="disabled || !!batchLoadingKey"
          @click="onBatchApprove(group)"
        >
          <el-icon class="ai-action-bar__option-icon"><CircleCheck /></el-icon>
          <span class="ai-action-bar__option-label">{{ group.label }}</span>
          <span class="ai-action-bar__option-count">{{ group.calls.length }}</span>
        </button>
      </div>

      <div
        v-for="call in complexCalls"
        :key="call.id"
        class="ai-action-bar__item"
      >
        <div class="ai-action-bar__row">
          <div class="ai-action-bar__info">
            <el-icon class="ai-action-bar__warn"><Warning /></el-icon>
            <span class="ai-action-bar__tool-name">{{ getToolDisplayName(call.name) }}</span>
          </div>
          <div class="ai-action-bar__actions">
            <el-button size="small" :disabled="disabled" @click="onRejectSingle(call)">
              拒绝
            </el-button>
            <el-button
              size="small"
              type="primary"
              :disabled="disabled"
              @click="toggleExpand(call.id)"
            >
              {{ expandedIds[call.id] ? '收起' : '查看详情' }}
            </el-button>
          </div>
        </div>
        <div v-if="expandedIds[call.id]" class="ai-action-bar__complex">
          <AIScenarioGenConfirm
            inline
            :call="call"
            :disabled="disabled"
            @decide="$emit('decide', $event)"
          />
        </div>
      </div>

      <div class="ai-action-bar__footer">
        <button
          v-if="simpleCalls.length > 1"
          type="button"
          class="ai-action-bar__link-btn"
          @click="showDetails = !showDetails"
        >
          {{ showDetails ? '收起详情' : '查看操作详情' }}
        </button>
        <div class="ai-action-bar__spacer" />
        <el-button
          v-if="nonQuestionCalls.length > 1"
          size="small"
          :disabled="disabled || !!batchLoadingKey"
          @click="onRejectAll"
        >
          全部拒绝
        </el-button>
      </div>

      <div v-if="showDetails && simpleCalls.length" class="ai-action-bar__detail-list">
        <div v-for="call in simpleCalls" :key="call.id" class="ai-action-bar__detail-item">
          <span class="ai-action-bar__detail-name">{{ getToolDisplayName(call.name) }}</span>
          <pre class="ai-action-bar__detail-args">{{ formatArgs(call) }}</pre>
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import { Warning, CircleCheck } from '@element-plus/icons-vue'
import { getToolDisplayName } from '../utils/aiToolLabels'
import AIScenarioGenConfirm from './AIScenarioGenConfirm.vue'
import AIQuestionCard from './AIQuestionCard.vue'

const QUESTION_TOOL = 'ask_question'
const COMPLEX_TOOLS = new Set(['generate_and_verify_scenarios', 'generate_coverage_scenarios'])

const OP_CATEGORIES = [
  { key: 'delete', label: '执行全部删除操作', match: (n) => n.startsWith('delete_') },
  { key: 'create', label: '执行全部创建操作', match: (n) => n.startsWith('create_') },
  { key: 'update', label: '执行全部更新操作', match: (n) => n.startsWith('update_') },
  { key: 'reorder', label: '执行全部排序操作', match: (n) => n.startsWith('reorder_') },
  { key: 'generate', label: '执行全部生成操作', match: (n) => n.startsWith('generate_') },
  { key: 'other', label: '执行全部操作', match: () => true },
]

function categorize(name) {
  for (const cat of OP_CATEGORIES) {
    if (cat.match(name)) return cat
  }
  return OP_CATEGORIES[OP_CATEGORIES.length - 1]
}

export default {
  name: 'AIActionBar',
  components: { Warning, CircleCheck, AIScenarioGenConfirm, AIQuestionCard },
  props: {
    pendingCalls: { type: Array, required: true },
    disabled: { type: Boolean, default: false },
  },
  emits: ['decide', 'batch-decide'],
  data() {
    return {
      expandedIds: {},
      showDetails: false,
      batchLoadingKey: null,
    }
  },
  computed: {
    questionCalls() {
      return this.pendingCalls.filter((c) => c.name === QUESTION_TOOL)
    },
    nonQuestionCalls() {
      return this.pendingCalls.filter((c) => c.name !== QUESTION_TOOL)
    },
    currentQuestion() {
      return this.questionCalls[0] || null
    },
    remainingQuestions() {
      return this.questionCalls.slice(1)
    },
    simpleCalls() {
      return this.nonQuestionCalls.filter((c) => !COMPLEX_TOOLS.has(c.name))
    },
    complexCalls() {
      return this.nonQuestionCalls.filter((c) => COMPLEX_TOOLS.has(c.name))
    },
    simpleGroups() {
      if (this.simpleCalls.length <= 1) return []
      const map = new Map()
      for (const call of this.simpleCalls) {
        const cat = categorize(call.name)
        if (!map.has(cat.key)) {
          map.set(cat.key, { key: cat.key, label: cat.label, calls: [] })
        }
        map.get(cat.key).calls.push(call)
      }
      const groups = [...map.values()]
      if (groups.length === 1) {
        groups[0].label = '执行全部操作'
      }
      return groups
    },
  },
  methods: {
    getToolDisplayName,
    formatArgs(call) {
      const raw = call?.arguments
      if (raw == null) return '{}'
      try {
        const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
        return JSON.stringify(parsed, null, 2)
      } catch {
        return String(raw)
      }
    },
    toggleExpand(callId) {
      this.expandedIds = { ...this.expandedIds, [callId]: !this.expandedIds[callId] }
    },
    onQuestionDecide(decision) {
      this.$emit('decide', decision)
    },
    onBatchApprove(group) {
      this.batchLoadingKey = group.key
      const callIds = group.calls.map((c) => c.id)
      this.$emit('batch-decide', { callIds, approve: true })
    },
    onRejectSingle(call) {
      this.$emit('decide', { callId: call.id, approve: false, reason: '' })
    },
    onRejectAll() {
      const callIds = this.nonQuestionCalls.map((c) => c.id)
      this.$emit('batch-decide', { callIds, approve: false })
    },
    resetBatchLoading() {
      this.batchLoadingKey = null
    },
  },
}
</script>

<style scoped>
.ai-action-bar {
  flex-shrink: 0;
  border-top: 1px solid var(--el-border-color-lighter, #ebeef5);
  background: var(--el-fill-color-blank, #fff);
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ai-action-bar__queue {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  text-align: center;
  padding: 2px 0;
}

.ai-action-bar__options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ai-action-bar__option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 30%, var(--el-border-color-lighter));
  border-radius: 10px;
  background: color-mix(in srgb, var(--el-color-primary-light-9) 60%, var(--el-fill-color-blank));
  color: var(--el-color-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
  white-space: nowrap;
}

.ai-action-bar__option:hover:not(:disabled) {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.12);
}

.ai-action-bar__option:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ai-action-bar__option--loading {
  opacity: 0.7;
  pointer-events: none;
}

.ai-action-bar__option-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.ai-action-bar__option-label {
  flex: 1;
}

.ai-action-bar__option-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--el-color-primary) 15%, transparent);
  font-size: 11px;
  font-weight: 600;
}

.ai-action-bar__item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--el-color-warning) 40%, var(--el-border-color-lighter));
  border-radius: 10px;
  background: color-mix(in srgb, var(--el-color-warning-light-9) 50%, var(--el-fill-color-blank));
}

.ai-action-bar__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.ai-action-bar__info {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex-shrink: 0;
}

.ai-action-bar__warn {
  font-size: 16px;
  color: var(--el-color-warning);
  flex-shrink: 0;
}

.ai-action-bar__tool-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
}

.ai-action-bar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  flex: 1;
  justify-content: flex-end;
}

.ai-action-bar__complex {
  margin-top: 4px;
}

.ai-action-bar__footer {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-action-bar__spacer {
  flex: 1;
}

.ai-action-bar__link-btn {
  border: none;
  background: transparent;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}

.ai-action-bar__link-btn:hover {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.ai-action-bar__detail-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 4px 0 0;
}

.ai-action-bar__detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ai-action-bar__detail-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.ai-action-bar__detail-args {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Courier New', monospace;
  font-size: 12px;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 6px 8px;
  max-height: 160px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--el-text-color-regular);
}
</style>
