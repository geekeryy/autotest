<template>
  <el-alert class="list-load-error" type="error" show-icon :closable="false">
    <template #title>
      <span class="list-load-error__text">{{ message }}</span>
      <el-button type="primary" link @click="$emit('retry')">重试</el-button>
    </template>
  </el-alert>
</template>

<script>
export default {
  name: 'ListLoadError',
  props: {
    message: {
      type: String,
      default: '加载失败，请重试',
    },
  },
  emits: ['retry'],
}
</script>
