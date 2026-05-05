<template>
  <div
    class="step-case-row"
    :class="[rowClass, extraRowClass]"
    :data-step-id="step.id"
    :style="{ '--step-depth': depth }"
  >
    <button
      type="button"
      class="step-seq-badge"
      :class="{ 'step-seq-badge--disabled': !step.enabled }"
      :aria-pressed="step.enabled"
      :aria-label="seqToggleAriaLabel"
      :title="seqBadgeTitle"
      @click.stop="api.onStepEnabledChange(step, !step.enabled)"
    >
      #{{ step.stepSeq }}
    </button>
    <button
      type="button"
      class="step-case-item"
      :class="{ active: api.isPanelStepActive(step) }"
      @click="api.selectPanelStep(step)"
    >
      <span class="step-case-main">
        <el-tag class="step-type-tag" size="small" :type="api.stepTypeTagType(step.stepType)">
          {{ api.stepTypeLabel(step.stepType) }}
        </el-tag>
        <span class="step-case-title">{{ api.stepDisplayName(step) }}</span>
      </span>
    </button>
    <el-icon class="step-delete-icon" @click.stop="api.confirmDeleteStep(step)"><Delete /></el-icon>
  </div>
</template>

<script>
import { Delete } from '@element-plus/icons-vue'

export default {
  name: 'ScenarioStepRow',
  components: { Delete },
  inject: ['scenarioStepListApi'],
  props: {
    step: { type: Object, required: true },
    depth: { type: Number, required: true },
    extraRowClass: { type: [String, Object, Array], default: null }
  },
  computed: {
    api() {
      return this.scenarioStepListApi
    },
    rowClass() {
      return this.api.leafRowClass(this.depth)
    },
    seqBadgeTitle() {
      const n = this.step.stepSeq
      const ref = `{{$steps[${n}].xxx}}`
      const state = this.step.enabled ? '点击停用' : '点击启用'
      return `步骤序号 #${n}，引用输出 ${ref}；${state}`
    },
    seqToggleAriaLabel() {
      const n = this.step.stepSeq
      return this.step.enabled ? `步骤 ${n} 已启用，点击停用` : `步骤 ${n} 已停用，点击启用`
    }
  }
}
</script>
