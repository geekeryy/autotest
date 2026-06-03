<template>
  <el-form :model="config" class="scenario-step-extra-form" label-width="96px" size="small">
    <el-form-item label="数据源">
      <el-select
        :model-value="config.dbDataSourceId"
        placeholder="选择数据源"
        filterable
        style="width: 100%"
        @update:model-value="update('dbDataSourceId', $event)"
      >
        <el-option
          v-for="ds in dataSources"
          :key="ds.id"
          :label="ds.name"
          :value="ds.id"
        />
      </el-select>
    </el-form-item>

    <el-form-item label="SQL">
      <SqlEditor
        :model-value="config.dbSQL"
        language="sql"
        placeholder="SELECT * FROM users WHERE id = {{userId}}"
        style="height: 200px"
        @update:model-value="update('dbSQL', $event)"
      />
    </el-form-item>

    <el-form-item label="输入参数">
      <el-input
        :model-value="config.dbInputParamsText"
        type="textarea"
        :rows="3"
        placeholder='[{"name": "userId", "value": "{{$steps[1].response.body.id}}"}]'
        @update:model-value="update('dbInputParamsText', $event)"
      />
    </el-form-item>

    <el-form-item label="超时(ms)">
      <el-input-number
        :model-value="config.dbTimeoutMillis"
        :min="1000"
        :max="30000"
        :step="1000"
        @update:model-value="update('dbTimeoutMillis', $event)"
      />
    </el-form-item>

    <el-form-item label="输出格式">
      <div class="step-ref-hint">
        查询结果可通过 <code>{{$steps[N].rows}}</code>（全部行）、
        <code>{{$steps[N].firstRow.fieldName}}</code>（第一行某字段）引用
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup>
import SqlEditor from '@/components/SqlEditor.vue'

const props = defineProps({
  config: { type: Object, required: true },
  dataSources: { type: Array, default: () => [] },
})

const emit = defineEmits(['update:config'])

function update(key, value) {
  emit('update:config', { ...props.config, [key]: value })
}
</script>

<style scoped>
.step-ref-hint {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.6;
}
.step-ref-hint code {
  background: var(--app-code-bg);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}
</style>
