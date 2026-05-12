<template>
  <div class="assertion-editor">
    <div
      v-for="(row, index) in rows"
      :key="index"
      class="assertion-row"
      :class="{ 'assertion-row--script': row.type === 'script' }"
    >
      <!-- 顶部控制行 -->
      <div class="assertion-controls">
        <el-select
          v-model="row.type"
          class="type-select"
          size="small"
          @change="onTypeChange(row)"
        >
          <el-option label="状态码" value="status_code" />
          <el-option label="JSONPath" value="jsonpath" />
          <el-option label="响应头" value="header" />
          <el-option label="Body 包含" value="body_contains" />
          <el-option label="响应时间" value="response_time" />
          <el-option label="JS 脚本" value="script" />
        </el-select>

        <!-- status_code -->
        <template v-if="row.type === 'status_code'">
          <span class="field-label">期望状态码</span>
          <el-input-number
            v-model="row.expected"
            class="status-code-input"
            size="small"
            :min="100"
            :max="599"
            :step="1"
          />
        </template>

        <!-- header -->
        <template v-else-if="row.type === 'header'">
          <el-input
            v-model="row.name"
            class="name-input"
            size="small"
            placeholder="Header 名称"
          />
          <el-select v-model="row.op" class="op-select" size="small">
            <el-option v-for="op in headerOps" :key="op.value" :label="op.label" :value="op.value" />
          </el-select>
          <el-input
            v-if="needsExpected(row)"
            v-model="row.expected"
            class="expected-input"
            size="small"
            placeholder="期望值"
          />
        </template>

        <!-- jsonpath -->
        <template v-else-if="row.type === 'jsonpath'">
          <el-input
            v-model="row.path"
            class="path-input"
            size="small"
            placeholder="$.data.id"
          />
          <el-select v-model="row.op" class="op-select" size="small">
            <el-option v-for="op in jsonpathOps" :key="op.value" :label="op.label" :value="op.value" />
          </el-select>
          <el-input
            v-if="needsExpected(row)"
            v-model="row.expected"
            class="expected-input"
            size="small"
            placeholder="期望值（数字直接填 0，字符串加引号 &quot;text&quot;）"
          />
        </template>

        <!-- body_contains -->
        <template v-else-if="row.type === 'body_contains'">
          <el-select v-model="row.op" class="op-select-sm" size="small">
            <el-option label="包含" value="contains" />
            <el-option label="不包含" value="not_contains" />
            <el-option label="正则匹配" value="matches" />
            <el-option label="等于" value="eq" />
          </el-select>
          <el-input
            v-model="row.value"
            class="value-input"
            size="small"
            placeholder="期望文本"
          />
        </template>

        <!-- response_time -->
        <template v-else-if="row.type === 'response_time'">
          <span class="field-label">响应时间</span>
          <el-select v-model="row.op" class="op-select-sm" size="small">
            <el-option label="≤" value="lte" />
            <el-option label="<" value="lt" />
            <el-option label="≥" value="gte" />
            <el-option label=">" value="gt" />
          </el-select>
          <el-input-number
            v-model="row.expected"
            class="rt-input"
            size="small"
            :min="1"
            :step="100"
          />
          <span class="ms-label">ms</span>
        </template>

        <!-- script placeholder in top row -->
        <template v-else-if="row.type === 'script'">
          <span class="script-hint">JavaScript (Postman 兼容 pm API)</span>
          <ScriptLibraryPicker variant="assertion" class="script-library-inline" @append="appendScriptSnippet(row, $event)" />
          <el-tooltip content="AI 生成断言脚本" placement="top">
            <el-button
              size="small"
              type="primary"
              plain
              class="script-library-inline ai-sparkle-btn"
              :loading="aiBusyRow === row"
              :disabled="!!aiBusyRow && aiBusyRow !== row"
              :aria-label="'AI 生成断言脚本'"
              @click="openAIGenerator(row)"
            >
              <AiSparkleIcon v-if="aiBusyRow !== row" />
            </el-button>
          </el-tooltip>
        </template>

        <el-button
          link
          type="danger"
          size="small"
          class="delete-btn"
          @click="removeRow(index)"
        >删除</el-button>
      </div>

      <!-- 脚本编辑区（仅 script 类型） -->
      <div v-if="row.type === 'script'" class="script-editor-wrap">
        <el-input
          v-model="row.script"
          type="textarea"
          class="script-editor"
          :autosize="{ minRows: 5, maxRows: 16 }"
          placeholder="pm.test('状态码 200', () => pm.response.to.have.status(200));
pm.test('业务码为 0', () => {
  const body = pm.response.json();
  pm.expect(body.code).to.equal(0);
  pm.expect(body.data.id).to.be.a('string').and.not.empty;
});"
        />
        <div class="script-api-tip">
          <code>pm.response</code> · <code>pm.expect(val).to.equal(x)</code> · <code>pm.variables.get(key)</code> · <code>assert(cond, msg)</code>
        </div>
      </div>
    </div>

    <el-button link type="primary" size="small" class="add-btn" @click="addRow">
      + 新增断言
    </el-button>

    <AIGenerateDialog
      v-model="aiDialogVisible"
      action="generate_assertion"
      :context="aiContext"
      @apply="onAIApply"
      @generation-settled="aiBusyRow = null"
    />
  </div>
</template>

<script>
import ScriptLibraryPicker from './ScriptLibraryPicker.vue'
import AIGenerateDialog from './AIGenerateDialog.vue'
import AiSparkleIcon from './icons/AiSparkleIcon.vue'

const NO_EXPECTED_OPS = new Set([
  'exists', 'not_exists', 'is_null', 'is_not_null', 'is_empty', 'is_not_empty'
])

const HEADER_OPS = [
  { value: 'exists', label: '存在' },
  { value: 'not_exists', label: '不存在' },
  { value: 'eq', label: '等于' },
  { value: 'ne', label: '不等于' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'matches', label: '正则匹配' },
  { value: 'is_empty', label: '为空' },
  { value: 'is_not_empty', label: '不为空' }
]

const JSONPATH_OPS = [
  { value: 'exists', label: '存在' },
  { value: 'not_exists', label: '不存在' },
  { value: 'is_null', label: '为 null' },
  { value: 'is_not_null', label: '不为 null' },
  { value: 'eq', label: '等于 (=)' },
  { value: 'ne', label: '不等于 (≠)' },
  { value: 'gt', label: '大于 (>)' },
  { value: 'gte', label: '大于等于 (≥)' },
  { value: 'lt', label: '小于 (<)' },
  { value: 'lte', label: '小于等于 (≤)' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'matches', label: '正则匹配' },
  { value: 'is_empty', label: '为空' },
  { value: 'is_not_empty', label: '不为空' }
]

function defaultRow(type) {
  switch (type) {
    case 'status_code':
      return { type: 'status_code', expected: 200 }
    case 'header':
      return { type: 'header', name: '', op: 'exists', expected: '' }
    case 'jsonpath':
      return { type: 'jsonpath', path: '', op: 'exists', expected: '' }
    case 'body_contains':
      return { type: 'body_contains', op: 'contains', value: '' }
    case 'response_time':
      return { type: 'response_time', op: 'lte', expected: 500 }
    case 'script':
      return { type: 'script', script: '' }
    default:
      return { type: 'status_code', expected: 200 }
  }
}

/** Convert UI row to backend assertion spec JSON object. */
function rowToSpec(row) {
  switch (row.type) {
    case 'status_code':
      return { type: 'status_code', expected: Number(row.expected) || 200 }
    case 'header':
      return {
        type: 'header',
        name: row.name || '',
        op: row.op || 'exists',
        ...(needsExpectedCheck(row) ? { expected: row.expected || '' } : {})
      }
    case 'jsonpath': {
      const spec = { type: 'jsonpath', path: row.path || '$', op: row.op || 'exists' }
      if (needsExpectedCheck(row)) {
        spec.expected = parseExpected(row.expected)
      }
      return spec
    }
    case 'body_contains':
      return { type: 'body_contains', op: row.op || 'contains', value: row.value || '' }
    case 'response_time':
      return { type: 'response_time', op: row.op || 'lte', expected: Number(row.expected) || 500 }
    case 'script':
      return { type: 'script', script: row.script || '' }
    default:
      return row
  }
}

function needsExpectedCheck(row) {
  return !NO_EXPECTED_OPS.has(row.op)
}

/** Try to parse expected value as JSON; fall back to string. */
function parseExpected(val) {
  if (val === '' || val === null || val === undefined) return val
  const trimmed = String(val).trim()
  // Try numeric
  if (trimmed !== '' && !isNaN(Number(trimmed)) && trimmed !== '') {
    const n = Number(trimmed)
    if (String(n) === trimmed) return n
  }
  // Try JSON (boolean, null, or quoted string)
  try {
    return JSON.parse(trimmed)
  } catch {
    return trimmed
  }
}

/** Convert backend assertion spec to UI row. */
function specToRow(spec) {
  if (!spec || !spec.type) return null
  switch (spec.type) {
    case 'status_code':
      return { type: 'status_code', expected: spec.expected ?? 200 }
    case 'header':
      return { type: 'header', name: spec.name || '', op: spec.op || 'exists', expected: spec.expected ?? '' }
    case 'jsonpath':
      return {
        type: 'jsonpath',
        path: spec.path || '',
        op: spec.op || 'exists',
        expected: spec.expected !== undefined ? String(
          typeof spec.expected === 'object' ? JSON.stringify(spec.expected) : spec.expected
        ) : ''
      }
    case 'body_contains':
      return { type: 'body_contains', op: spec.op || 'contains', value: spec.value || '' }
    case 'response_time':
      return { type: 'response_time', op: spec.op || 'lte', expected: spec.expected ?? 500 }
    case 'script':
      return { type: 'script', script: spec.script || '' }
    default:
      return { ...spec }
  }
}

export default {
  name: 'AssertionEditor',
  components: { ScriptLibraryPicker, AIGenerateDialog, AiSparkleIcon },
  props: {
    modelValue: {
      type: Array,
      default: () => []
    },
    aiResponseSnapshot: {
      type: Object,
      default: () => null
    }
  },
  emits: ['update:modelValue'],
  data() {
    return {
      rows: [],
      /** 避免 v-model 回写时整表替换 rows，否则 el-select 选完后弹层无法收起 */
      skipModelValueApply: false,
      headerOps: HEADER_OPS,
      jsonpathOps: JSONPATH_OPS,
      aiDialogVisible: false,
      aiTargetRow: null,
      aiBusyRow: null
    }
  },
  watch: {
    aiDialogVisible(val) {
      if (!val) this.aiBusyRow = null
    },
    modelValue: {
      immediate: true,
      handler(val) {
        if (this.skipModelValueApply) {
          this.skipModelValueApply = false
          return
        }
        const specs = Array.isArray(val) ? val : []
        this.rows = specs.map(specToRow).filter(Boolean)
      }
    },
    rows: {
      deep: true,
      handler() {
        this.skipModelValueApply = true
        this.$emit('update:modelValue', this.rows.map(rowToSpec))
      }
    }
  },
  methods: {
    needsExpected(row) {
      return !NO_EXPECTED_OPS.has(row.op)
    },
    onTypeChange(row) {
      const fresh = defaultRow(row.type)
      Object.assign(row, fresh)
    },
    addRow() {
      this.rows.push(defaultRow('status_code'))
    },
    removeRow(index) {
      this.rows.splice(index, 1)
    },
    appendScriptSnippet(row, code) {
      const cur = row.script || ''
      const next = cur.trim() ? `${cur.trim()}\n\n${code}` : code
      row.script = next
    },
    openAIGenerator(row) {
      this.aiTargetRow = row
      this.aiBusyRow = row
      this.aiDialogVisible = true
    },
    onAIApply(payload) {
      const code = (payload?.text || '').trim()
      if (!code || !this.aiTargetRow) return
      this.appendScriptSnippet(this.aiTargetRow, code)
      this.aiTargetRow = null
    }
  },
  computed: {
    aiContext() {
      const snapshot = this.aiResponseSnapshot || null
      return {
        existingAssertions: this.rows,
        responseSnapshot: snapshot
      }
    }
  }
}
</script>

<style scoped>
.assertion-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.assertion-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid var(--app-border-color, #e4e7ed);
  border-radius: 8px;
  background: var(--app-card-bg, #fafafa);
}

.assertion-row--script {
  background: #f8faff;
}

.assertion-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.type-select {
  width: 120px;
  flex-shrink: 0;
}

.name-input,
.path-input {
  width: 180px;
  flex-shrink: 0;
}

.op-select {
  width: 140px;
  flex-shrink: 0;
}

.op-select-sm {
  width: 110px;
  flex-shrink: 0;
}

.expected-input,
.value-input {
  flex: 1;
  min-width: 120px;
}

.status-code-input {
  width: 120px;
}

.rt-input {
  width: 120px;
}

.ms-label,
.field-label {
  font-size: var(--app-font-size-small, 13px);
  color: var(--app-secondary-text, #909399);
  white-space: nowrap;
}

.script-hint {
  flex: 1;
  font-size: var(--app-font-size-small, 13px);
  color: var(--app-secondary-text, #909399);
  font-style: italic;
}

.script-library-inline {
  flex-shrink: 0;
}

/* AI 生成按钮 .ai-sparkle-btn 已在 styles/global.css 统一管理样式，本文件不再重复 */

.delete-btn {
  margin-left: auto;
  flex-shrink: 0;
}

.script-editor-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.script-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.55;
}

.script-api-tip {
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text, #909399);
  line-height: 1.4;
}

.script-api-tip code {
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(64, 158, 255, 0.08);
  font-size: var(--app-font-size-small);
  color: var(--el-color-primary, #409eff);
}

.add-btn {
  align-self: flex-start;
  margin-top: 2px;
}
</style>
