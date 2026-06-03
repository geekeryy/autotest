<template>
  <aside class="step-dialog-sidebar" :style="sidebarStyle">
    <div class="step-sidebar-title">
      <span>已添加步骤</span>
      <button type="button" class="step-sidebar-add" @click="$emit('add-step')">
        <el-icon class="step-sidebar-add-icon"><Plus /></el-icon>
        <span>新增步骤</span>
      </button>
    </div>
    <div class="step-case-list">
      <div ref="sortableListRef" class="step-sortable-root">
        <slot />
      </div>
      <el-empty v-if="!steps.length" description="暂无已添加步骤" :image-size="60" />
    </div>
  </aside>

  <div
    class="step-sidebar-resizer"
    role="separator"
    :aria-valuenow="sidebarWidth"
    aria-valuemin="180"
    aria-valuemax="560"
    aria-orientation="vertical"
    aria-label="调整步骤列表宽度"
    tabindex="0"
    @mousedown.prevent="$emit('start-resize')"
    @keydown.left.prevent="$emit('nudge', -16)"
    @keydown.right.prevent="$emit('nudge', 16)"
  />
</template>

<script setup>
import { ref } from 'vue'
import { Plus } from '@element-plus/icons-vue'

const props = defineProps({
  steps: { type: Array, default: () => [] },
  sidebarWidth: { type: Number, default: 260 },
})

defineEmits(['add-step', 'start-resize', 'nudge'])

const sortableListRef = ref(null)

const sidebarStyle = { width: `${props.sidebarWidth}px`, flexShrink: 0 }

defineExpose({ sortableListRef })
</script>

<style scoped>
.step-dialog-sidebar {
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--app-border-color);
  background: var(--app-card-bg);
  overflow: hidden;
}
.step-sidebar-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-weight: 600;
  font-size: var(--app-font-size-base);
  border-bottom: 1px solid var(--app-border-color);
}
.step-sidebar-add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px dashed var(--app-border-color);
  border-radius: 6px;
  background: transparent;
  color: var(--app-primary-color);
  cursor: pointer;
  font-size: var(--app-font-size-small);
  transition: border-color 0.2s;
}
.step-sidebar-add:hover {
  border-color: var(--app-primary-color);
}
.step-case-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}
.step-sidebar-resizer {
  width: 4px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.2s;
  flex-shrink: 0;
}
.step-sidebar-resizer:hover {
  background: var(--app-primary-color);
}
</style>
