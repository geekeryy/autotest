<template>
  <div class="request-editor">
    <!-- 请求行：Method + Path -->
    <div class="request-line">
      <el-select
        :model-value="method"
        class="method-select"
        @update:model-value="$emit('update:method', $event)"
      >
        <el-option
          v-for="m in methods"
          :key="m.value"
          :label="m.label"
          :value="m.value"
        >
          <span :style="{ color: m.color, fontWeight: 600 }">{{ m.label }}</span>
        </el-option>
      </el-select>
      <el-input
        :model-value="path"
        class="path-input"
        placeholder="/api/path"
        @update:model-value="$emit('update:path', $event)"
      />
      <el-button
        type="primary"
        :loading="running"
        @click="$emit('run')"
      >
        发送
      </el-button>
      <el-button @click="$emit('save')">保存</el-button>
    </div>

    <!-- 参数编辑 Tabs -->
    <el-tabs :model-value="activeTab" class="request-tabs" @update:model-value="$emit('update:activeTab', $event)">
      <el-tab-pane label="Query 参数" name="query">
        <slot name="query" />
      </el-tab-pane>

      <el-tab-pane label="请求头" name="headers">
        <slot name="headers" />
      </el-tab-pane>

      <el-tab-pane v-if="hasBody" label="请求体" name="body">
        <slot name="body" />
      </el-tab-pane>

      <el-tab-pane label="认证" name="auth">
        <slot name="auth" />
      </el-tab-pane>

      <el-tab-pane label="断言" name="assertions">
        <slot name="assertions" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
const props = defineProps({
  method: { type: String, default: 'GET' },
  path: { type: String, default: '' },
  activeTab: { type: String, default: 'query' },
  running: { type: Boolean, default: false },
  methods: {
    type: Array,
    default: () => [
      { value: 'GET', label: 'GET', color: '#61affe' },
      { value: 'POST', label: 'POST', color: '#49cc90' },
      { value: 'PUT', label: 'PUT', color: '#fca130' },
      { value: 'PATCH', label: 'PATCH', color: '#50e3c2' },
      { value: 'DELETE', label: 'DELETE', color: '#f93e3e' },
    ],
  },
})

defineEmits(['update:method', 'update:path', 'update:activeTab', 'run', 'save'])

const hasBody = ['POST', 'PUT', 'PATCH'].includes(props.method)
</script>

<style scoped>
.request-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.request-line {
  display: flex;
  gap: 8px;
  align-items: center;
}
.method-select {
  width: 120px;
}
.path-input {
  flex: 1;
}
.request-tabs {
  margin-top: 8px;
}
</style>
