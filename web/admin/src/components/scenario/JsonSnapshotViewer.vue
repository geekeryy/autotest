<template>
  <div v-if="lines.length" class="json-viewer code-view">
    <div
      v-for="line in lines"
      :key="line.id"
      class="json-line"
      :style="{ paddingLeft: `${line.depth * 16}px` }"
    >
      <span
        v-if="line.type === 'open'"
        class="json-toggle"
        @click="$emit('toggle', line.path)"
      />
      <span
        v-else-if="line.type === 'collapsed'"
        class="json-toggle is-collapsed"
        @click="$emit('toggle', line.path)"
      />
      <span v-else class="json-toggle-ph" />
      <span v-if="line.key != null" class="json-key">"{{ line.key }}"<span class="json-colon">: </span></span>
      <template v-if="line.type === 'primitive'">
        <span class="json-value" :class="`json-${line.valueType}`">{{ line.displayValue }}</span>
      </template>
      <template v-else-if="line.type === 'open'">
        <span class="json-bracket">{{ line.bracket }}</span>
      </template>
      <template v-else-if="line.type === 'close'">
        <span class="json-bracket">{{ line.bracket }}</span>
      </template>
      <template v-else-if="line.type === 'collapsed'">
        <span class="json-bracket">{{ line.open }}</span>
        <span class="json-collapsed-preview"> {{ line.count }} </span>
        <span class="json-bracket">{{ line.close }}</span>
      </template>
      <template v-else-if="line.type === 'empty-object'">
        <span class="json-bracket">{}</span>
      </template>
      <template v-else-if="line.type === 'empty-array'">
        <span class="json-bracket">[]</span>
      </template>
      <span v-if="line.hasComma" class="json-comma">,</span>
    </div>
  </div>
  <el-empty v-else description="暂无数据" :image-size="42" />
</template>

<script>
export default {
  name: 'JsonSnapshotViewer',
  props: {
    lines: { type: Array, default: () => [] }
  },
  emits: ['toggle']
}
</script>

<style scoped>
.json-viewer {
  min-height: 96px;
  max-height: 320px;
  overflow: auto;
  padding: 10px 0;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 10px;
  background: var(--app-code-bg, var(--el-fill-color-light));
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small, 12px);
  line-height: 1.7;
}

.json-line {
  display: flex;
  align-items: baseline;
  gap: 0;
  white-space: pre;
  padding-right: 10px;
}

.json-toggle,
.json-toggle-ph {
  flex: 0 0 14px;
  width: 14px;
  height: 14px;
  position: relative;
  top: 1px;
}

.json-toggle {
  cursor: pointer;
  border-radius: 2px;
  flex-shrink: 0;
}

.json-toggle:hover {
  background: var(--el-fill-color);
}

.json-toggle::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  border: 3.5px solid transparent;
  border-top: 4.5px solid #94a3b8;
  border-bottom: 0;
  transform: translate(-50%, -20%);
  transition: transform 0.12s;
}

.json-toggle.is-collapsed::before {
  transform: translate(-50%, -50%) rotate(-90deg);
}

.json-key {
  color: #1e40af;
}

.json-colon,
.json-bracket,
.json-comma {
  color: var(--el-text-color-regular);
}

.json-value.json-string {
  color: #16a34a;
}

.json-value.json-number {
  color: #c2410c;
}

.json-value.json-boolean {
  color: #7c3aed;
}

.json-value.json-null {
  color: #94a3b8;
}

.json-collapsed-preview {
  color: #94a3b8;
  font-style: italic;
}
</style>
