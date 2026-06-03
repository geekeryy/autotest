<template>
  <el-select
    :model-value="modelValue"
    class="step-type-select"
    :disabled="disabled"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <el-option
      v-for="item in stepTypes"
      :key="item.value"
      :label="item.label"
      :value="item.value"
    />
  </el-select>
</template>

<script setup>
defineProps({
  modelValue: { type: String, default: 'api' },
  disabled: { type: Boolean, default: false },
  stepTypes: {
    type: Array,
    default: () => [
      { value: 'api', label: 'API 请求' },
      { value: 'database', label: '数据库查询' },
      { value: 'script', label: '脚本' },
      { value: 'for', label: 'For 循环' },
      { value: 'condition', label: '条件分支' },
    ],
  },
})

defineEmits(['update:modelValue'])
</script>
