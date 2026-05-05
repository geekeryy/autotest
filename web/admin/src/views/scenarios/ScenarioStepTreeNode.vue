<template>
  <template v-if="node.type === 'leaf'">
    <ScenarioStepRow :step="node.step" :depth="node.depth" />
  </template>

  <template v-else-if="node.type === 'for'">
    <div class="control-flow-wrap control-flow-wrap--loop">
      <ScenarioStepRow
        :step="node.step"
        :depth="node.depth"
        extra-row-class="control-flow-parent"
      />
      <div
        class="control-flow-frame control-flow-frame--loop"
        :style="{ '--control-parent-depth': node.depth }"
      >
        <ScenarioStepTreeNode v-for="ch in node.children" :key="ch.key" :node="ch" />
        <div class="control-frame-border-add-anchor">
          <button
            type="button"
            class="control-frame-border-add control-frame-border-add--loop"
            aria-label="向循环体内新增步骤"
            @click.stop="api.startNewControlChildDraft(node.step, 'body')"
          >
            <el-icon class="control-frame-border-add-icon"><Plus /></el-icon>
          </button>
        </div>
      </div>
    </div>
  </template>

  <template v-else-if="node.type === 'condition'">
    <div class="control-flow-wrap control-flow-wrap--condition">
      <ScenarioStepRow
        :step="node.step"
        :depth="node.depth"
        extra-row-class="control-flow-parent"
      />
      <div
        class="control-flow-branch-stack"
        :style="{ '--control-parent-depth': node.depth }"
      >
        <div
          v-for="branch in node.branchFrames"
          :key="branch.key"
          class="control-flow-frame control-flow-frame--then"
        >
          <ScenarioStepTreeNode v-for="ch in branch.children" :key="ch.key" :node="ch" />
          <div class="control-frame-border-add-anchor">
            <button
              type="button"
              class="control-frame-border-add control-frame-border-add--then"
              :aria-label="`向条件分支 ${branch.branchIndex + 1} 新增步骤`"
              @click.stop="api.startNewControlChildDraft(node.step, branch.branchIndex)"
            >
              <el-icon class="control-frame-border-add-icon"><Plus /></el-icon>
            </button>
          </div>
        </div>
        <div class="control-flow-frame control-flow-frame--else">
          <ScenarioStepTreeNode v-for="ch in node.elseChildren" :key="ch.key" :node="ch" />
          <div class="control-frame-border-add-anchor">
            <button
              type="button"
              class="control-frame-border-add control-frame-border-add--else"
              aria-label="向否则分支新增步骤"
              @click.stop="api.startNewControlChildDraft(node.step, 'else')"
            >
              <el-icon class="control-frame-border-add-icon"><Plus /></el-icon>
            </button>
          </div>
        </div>
      </div>
    </div>
  </template>
</template>

<script>
import { Plus } from '@element-plus/icons-vue'
import ScenarioStepRow from './ScenarioStepRow.vue'

export default {
  name: 'ScenarioStepTreeNode',
  components: {
    ScenarioStepRow,
    Plus
  },
  inject: ['scenarioStepListApi'],
  props: {
    node: { type: Object, required: true }
  },
  computed: {
    api() {
      return this.scenarioStepListApi
    }
  }
}
</script>
