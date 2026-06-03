<template>
  <el-form :model="config" class="scenario-step-extra-form" label-width="112px" size="small">
    <el-form-item label="条件分支">
      <div v-for="(branch, idx) in config.conditionBranches" :key="idx" class="condition-branch-row">
        <span class="branch-label">IF {{ idx > 0 ? 'ELSE IF' : '' }}</span>
        <el-input
          v-model="branch.left"
          placeholder="变量或表达式"
          style="width: 180px"
        />
        <el-select v-model="branch.operator" style="width: 130px">
          <el-option
            v-for="op in conditionOperators"
            :key="op.value"
            :label="op.label"
            :value="op.value"
          />
        </el-select>
        <el-input
          v-model="branch.right"
          placeholder="比较值"
          style="width: 140px"
        />
        <el-button
          v-if="config.conditionBranches.length > 1"
          type="danger"
          text
          @click="removeBranch(idx)"
        >
          删除
        </el-button>
      </div>
      <el-button type="primary" text @click="addBranch">
        + 添加分支
      </el-button>
    </el-form-item>
  </el-form>
</template>

<script setup>
import { CONDITION_OPERATORS } from '@/utils/scenarioConstants'

const props = defineProps({
  config: { type: Object, required: true },
})

const emit = defineEmits(['update:config'])
const conditionOperators = CONDITION_OPERATORS

function addBranch() {
  const branches = [...(props.config.conditionBranches || [])]
  branches.push({ left: '', operator: 'equals', right: '', stepSeqs: [] })
  emit('update:config', { ...props.config, conditionBranches: branches })
}

function removeBranch(idx) {
  const branches = [...(props.config.conditionBranches || [])]
  branches.splice(idx, 1)
  emit('update:config', { ...props.config, conditionBranches: branches })
}
</script>

<style scoped>
.condition-branch-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.branch-label {
  font-weight: 600;
  color: var(--app-secondary-text);
  min-width: 70px;
}
</style>
