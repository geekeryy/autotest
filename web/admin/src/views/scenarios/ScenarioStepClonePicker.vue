<template>
  <el-dialog
    v-model="dialogVisible"
    title="克隆已有步骤"
    width="780px"
    align-center
    destroy-on-close
    class="clone-step-dialog"
    @close="onClose"
  >
    <div class="clone-step-body">
      <div class="clone-step-row">
        <span class="clone-step-row-label">源场景</span>
        <el-select
          v-model="selectedScenarioId"
          filterable
          placeholder="选择当前项目下的源场景"
          class="clone-scenario-select"
          :loading="loadingScenarios"
          @change="onScenarioChange"
        >
          <el-option-group
            v-for="group in groupedScenarios"
            :key="group.serviceId || '__nogroup__'"
            :label="group.label"
          >
            <el-option
              v-for="sc in group.scenarios"
              :key="sc.id"
              :value="sc.id"
              :label="`${sc.name} · ${group.label}`"
            >
              <span class="clone-option-name">{{ sc.name }}</span>
              <span class="clone-option-meta">{{ group.label }}<span v-if="sc.id === currentScenarioId" class="clone-option-current">（当前场景）</span></span>
            </el-option>
          </el-option-group>
        </el-select>
      </div>

      <div class="clone-step-tree-wrap">
        <div class="clone-step-tree-title">
          <span>选择要克隆的步骤</span>
          <span class="clone-step-tree-hint">
            选中后会连同 For 循环体 / 条件分支下的子步骤一起深拷贝
          </span>
        </div>
        <div v-if="loadingSteps" class="clone-step-loading">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>正在加载场景步骤…</span>
        </div>
        <div v-else-if="!selectedScenarioId" class="clone-step-empty">
          请先在上方选择一个场景
        </div>
        <el-empty
          v-else-if="!stepTreeData.length"
          description="该场景暂无可克隆的步骤"
          :image-size="60"
        />
        <el-tree
          v-else
          ref="treeRef"
          :data="stepTreeData"
          node-key="seq"
          highlight-current
          :default-expand-all="true"
          :expand-on-click-node="false"
          @current-change="onCurrentNodeChange"
        >
          <template #default="{ data }">
            <div class="clone-step-node" :class="{ 'is-control': data.isControl }">
              <el-tag size="small" :type="stepTagType(data.stepType)" effect="plain">
                #{{ data.seq }} · {{ stepTypeLabel(data.stepType) }}
              </el-tag>
              <span class="clone-step-node-name" :title="data.label">{{ data.label }}</span>
              <span v-if="data.branchHint" class="clone-step-node-hint">{{ data.branchHint }}</span>
            </div>
          </template>
        </el-tree>
      </div>
    </div>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button
        type="primary"
        :disabled="!canConfirm"
        :loading="confirming"
        @click="onConfirm"
      >克隆到当前场景末尾</el-button>
    </template>
  </el-dialog>
</template>

<script>
import { Loading } from '@element-plus/icons-vue'
import { listScenarios, listScenarioSteps, listServices } from '../../api'

const STEP_TYPE_LABELS = {
  api: 'API',
  database: '数据库',
  script: '脚本',
  for: 'For循环',
  condition: '条件分支'
}

const STEP_TYPE_TAG = {
  api: 'success',
  database: 'warning',
  script: 'info',
  for: 'primary',
  condition: 'warning'
}

function parseConfig(raw) {
  if (raw == null) return {}
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw)
    } catch {
      return {}
    }
  }
  return raw
}

function normalizeSeqList(values) {
  const seen = new Set()
  const out = []
  for (const v of values || []) {
    const n = Number(v)
    if (!Number.isFinite(n) || n <= 0) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out
}

function normalizeConditionCfg(cfg) {
  const legacy = {
    left: cfg.left || '',
    operator: cfg.operator || 'equals',
    right: cfg.right || '',
    stepSeqs: normalizeSeqList(cfg.thenStepSeqs)
  }
  const branches = Array.isArray(cfg.branches) && cfg.branches.length
    ? cfg.branches.map((b) => ({
        left: b?.left || '',
        operator: b?.operator || 'equals',
        right: b?.right || '',
        stepSeqs: normalizeSeqList(b?.stepSeqs)
      }))
    : [legacy]
  return {
    branches,
    elseStepSeqs: normalizeSeqList(cfg.elseStepSeqs)
  }
}

export default {
  name: 'ScenarioStepClonePicker',
  components: { Loading },
  props: {
    modelValue: { type: Boolean, default: false },
    projectId: { type: String, default: '' },
    currentScenarioId: { type: String, default: '' },
    cases: { type: Array, default: () => [] }
  },
  emits: ['update:modelValue', 'confirm'],
  data() {
    return {
      loadingScenarios: false,
      loadingSteps: false,
      confirming: false,
      services: [],
      scenarios: [],
      selectedScenarioId: '',
      selectedSteps: [],
      selectedSeq: null
    }
  },
  computed: {
    dialogVisible: {
      get() {
        return this.modelValue
      },
      set(v) {
        this.$emit('update:modelValue', v)
      }
    },
    serviceMap() {
      const m = new Map()
      for (const s of this.services) {
        m.set(String(s.id), s.name || s.id)
      }
      return m
    },
    groupedScenarios() {
      const groups = new Map()
      for (const sc of this.scenarios) {
        const sid = sc.serviceId ? String(sc.serviceId) : ''
        const key = sid || '__none__'
        if (!groups.has(key)) {
          groups.set(key, {
            serviceId: sid,
            label: sid ? (this.serviceMap.get(sid) || '未知服务') : '未关联服务',
            scenarios: []
          })
        }
        groups.get(key).scenarios.push(sc)
      }
      const arr = Array.from(groups.values())
      arr.sort((a, b) => a.label.localeCompare(b.label, 'zh-Hans-CN'))
      for (const g of arr) {
        g.scenarios.sort((a, b) => (a.name || '').localeCompare(b.name || '', 'zh-Hans-CN'))
      }
      return arr
    },
    stepBySeq() {
      const m = new Map()
      for (const step of this.selectedSteps) {
        const seq = Number(step.stepSeq)
        if (Number.isFinite(seq) && seq > 0) m.set(seq, step)
      }
      return m
    },
    stepTreeData() {
      return this.buildTree()
    },
    canConfirm() {
      return Boolean(this.selectedScenarioId && this.selectedSeq && this.stepBySeq.has(this.selectedSeq))
    }
  },
  watch: {
    modelValue(open) {
      if (open) {
        this.initOnOpen()
      } else {
        this.resetState()
      }
    }
  },
  methods: {
    async initOnOpen() {
      this.resetState()
      if (!this.projectId) {
        this.$message.warning('请先选择项目后再克隆步骤')
        this.dialogVisible = false
        return
      }
      this.loadingScenarios = true
      try {
        const [services, scenarios] = await Promise.all([
          listServices(this.projectId).catch(() => []),
          listScenarios({ projectId: this.projectId }).catch(() => [])
        ])
        this.services = Array.isArray(services) ? services : []
        this.scenarios = Array.isArray(scenarios) ? scenarios : []
        if (this.currentScenarioId && this.scenarios.some((s) => s.id === this.currentScenarioId)) {
          this.selectedScenarioId = this.currentScenarioId
          await this.loadStepsForScenario(this.currentScenarioId)
        }
      } finally {
        this.loadingScenarios = false
      }
    },
    resetState() {
      this.selectedScenarioId = ''
      this.selectedSteps = []
      this.selectedSeq = null
      this.confirming = false
    },
    async onScenarioChange(scenarioId) {
      this.selectedSeq = null
      this.selectedSteps = []
      if (!scenarioId) return
      await this.loadStepsForScenario(scenarioId)
    },
    async loadStepsForScenario(scenarioId) {
      this.loadingSteps = true
      try {
        const steps = await listScenarioSteps(scenarioId)
        this.selectedSteps = Array.isArray(steps) ? steps : []
      } catch {
        this.selectedSteps = []
      } finally {
        this.loadingSteps = false
      }
    },
    onCurrentNodeChange(data) {
      this.selectedSeq = data && Number.isFinite(Number(data.seq)) ? Number(data.seq) : null
    },
    stepTypeLabel(stepType) {
      return STEP_TYPE_LABELS[stepType] || 'API'
    },
    stepTagType(stepType) {
      return STEP_TYPE_TAG[stepType] || 'success'
    },
    findCaseName(caseId) {
      if (caseId == null || caseId === '') return ''
      const sid = String(caseId)
      const tc = this.cases.find((c) => String(c.id) === sid)
      return tc?.name || ''
    },
    stepDisplayName(step) {
      const custom = (step?.name || '').trim()
      if (custom) return custom
      const t = step?.stepType || 'api'
      if (t === 'api') return this.findCaseName(step.testCaseId) || '(未关联接口)'
      return STEP_TYPE_LABELS[t] || 'API'
    },
    buildTree() {
      const steps = [...this.selectedSteps].sort((a, b) => (a.stepOrder || 0) - (b.stepOrder || 0))
      if (!steps.length) return []
      const seqMap = this.stepBySeq
      const controlled = new Set()
      for (const step of steps) {
        for (const seq of this.childSeqsOf(step)) controlled.add(seq)
      }

      const buildNode = (step, path = new Set(), branchHint = '') => {
        const seq = Number(step.stepSeq)
        const isControl = step.stepType === 'for' || step.stepType === 'condition'
        const node = {
          seq,
          stepType: step.stepType || 'api',
          isControl,
          label: this.stepDisplayName(step),
          branchHint,
          children: []
        }
        if (!Number.isFinite(seq) || seq <= 0 || path.has(seq)) return node
        const nextPath = new Set(path)
        nextPath.add(seq)

        if (step.stepType === 'for') {
          const cfg = parseConfig(step.config)
          for (const childSeq of normalizeSeqList(cfg.bodyStepSeqs)) {
            const child = seqMap.get(childSeq)
            if (child) node.children.push(buildNode(child, nextPath, '循环体'))
          }
        } else if (step.stepType === 'condition') {
          const norm = normalizeConditionCfg(parseConfig(step.config))
          norm.branches.forEach((br, idx) => {
            for (const childSeq of normalizeSeqList(br.stepSeqs)) {
              const child = seqMap.get(childSeq)
              if (child) node.children.push(buildNode(child, nextPath, `分支 ${idx + 1}`))
            }
          })
          for (const childSeq of norm.elseStepSeqs) {
            const child = seqMap.get(childSeq)
            if (child) node.children.push(buildNode(child, nextPath, '否则'))
          }
        }
        return node
      }

      const roots = []
      for (const step of steps) {
        const seq = Number(step.stepSeq)
        if (Number.isFinite(seq) && controlled.has(seq)) continue
        roots.push(buildNode(step, new Set(), ''))
      }
      if (!roots.length) {
        for (const step of steps) roots.push(buildNode(step, new Set(), ''))
      }
      return roots
    },
    childSeqsOf(step) {
      const cfg = parseConfig(step?.config)
      if (step?.stepType === 'for') return normalizeSeqList(cfg.bodyStepSeqs)
      if (step?.stepType === 'condition') {
        const norm = normalizeConditionCfg(cfg)
        const all = []
        for (const b of norm.branches) all.push(...normalizeSeqList(b.stepSeqs))
        all.push(...norm.elseStepSeqs)
        return normalizeSeqList(all)
      }
      return []
    },
    onClose() {
      this.resetState()
    },
    onConfirm() {
      if (!this.canConfirm) return
      const sourceStep = this.stepBySeq.get(this.selectedSeq)
      if (!sourceStep) {
        this.$message.warning('未找到选中的源步骤')
        return
      }
      this.confirming = true
      this.$emit('confirm', {
        sourceScenarioId: this.selectedScenarioId,
        sourceStep,
        sourceSteps: this.selectedSteps
      })
    },
    finishConfirm() {
      this.confirming = false
      this.dialogVisible = false
    }
  }
}
</script>

<style scoped>
.clone-step-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.clone-step-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.clone-step-row-label {
  flex: 0 0 auto;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.clone-scenario-select {
  flex: 1 1 auto;
  min-width: 0;
}
.clone-option-name {
  font-weight: 500;
  margin-right: 8px;
}
.clone-option-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.clone-option-current {
  margin-left: 4px;
  color: var(--el-color-primary);
}
.clone-step-tree-wrap {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--el-bg-color-page);
  max-height: 360px;
  overflow: auto;
}
.clone-step-tree-title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  font-size: 13px;
  color: var(--el-text-color-primary);
}
.clone-step-tree-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.clone-step-loading,
.clone-step-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 28px 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.clone-step-node {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.clone-step-node-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.clone-step-node-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
.clone-step-node.is-control .clone-step-node-name {
  font-weight: 500;
}
</style>
