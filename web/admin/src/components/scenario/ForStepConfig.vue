<template>
  <el-form :model="config" class="scenario-step-extra-form" label-width="112px" size="small">
    <el-form-item label="循环模式">
      <el-radio-group :model-value="config.forMode" @update:model-value="update('forMode', $event)">
        <el-radio value="count">固定次数</el-radio>
        <el-radio value="array">遍历数组</el-radio>
      </el-radio-group>
    </el-form-item>

    <el-form-item v-if="config.forMode === 'count'" label="循环次数">
      <el-input
        :model-value="config.forCountExpression"
        placeholder="如 5 或 {{myVar}}"
        @update:model-value="update('forCountExpression', $event)"
      />
    </el-form-item>

    <el-form-item v-else label="数组表达式">
      <el-input
        :model-value="config.forItemsExpression"
        placeholder="如 [1,2,3] 或 {{$steps[1].response.body.list}}"
        @update:model-value="update('forItemsExpression', $event)"
      />
    </el-form-item>

    <el-form-item label="循环变量名">
      <el-input
        :model-value="config.forItemVar"
        placeholder="item"
        style="width: 120px"
        @update:model-value="update('forItemVar', $event)"
      />
      <span style="margin: 0 8px; color: var(--app-secondary-text)">索引变量</span>
      <el-input
        :model-value="config.forIndexVar"
        placeholder="index"
        style="width: 120px"
        @update:model-value="update('forIndexVar', $event)"
      />
    </el-form-item>

    <el-form-item label="最大迭代次数">
      <el-input-number
        :model-value="config.forMaxIterations"
        :min="1"
        :max="1000"
        @update:model-value="update('forMaxIterations', $event)"
      />
    </el-form-item>
  </el-form>
</template>

<script setup>
const props = defineProps({
  config: { type: Object, required: true },
})

const emit = defineEmits(['update:config'])

function update(key, value) {
  emit('update:config', { ...props.config, [key]: value })
}
</script>
