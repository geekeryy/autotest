<template>
  <div v-if="lines.length" class="json-viewer snapshot-pre">
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
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  max-height: 320px;
  overflow: auto;
  background: #0f172a;
  color: #e2e8f0;
  padding: 8px;
  border-radius: 6px;
}
.json-line { line-height: 1.45; white-space: pre; }
.json-toggle {
  display: inline-block;
  width: 12px;
  cursor: pointer;
  margin-right: 4px;
}
.json-toggle::before { content: '▼'; font-size: 9px; }
.json-toggle.is-collapsed::before { content: '▶'; }
.json-toggle-ph { display: inline-block; width: 16px; }
.json-key { color: #7dd3fc; }
.json-value.json-string { color: #86efac; }
.json-value.json-number { color: #fcd34d; }
.json-value.json-boolean { color: #f9a8d4; }
.json-value.json-null { color: #94a3b8; }
</style>
