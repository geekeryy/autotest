<template>
  <el-form :model="config" class="scenario-step-extra-form" label-width="96px" size="small">
    <el-form-item label="脚本">
      <div class="scenario-script-field">
        <div class="scenario-script-toolbar">
          <el-button size="small" @click="$emit('insert-script-library')">
            从脚本库插入
          </el-button>
          <el-button size="small" @click="$emit('ai-generate-script')">
            <AiSparkleIcon style="margin-right: 4px" /> AI 生成
          </el-button>
          <span class="scenario-script-toolbar-hint">
            从脚本库插入模板或调用 AI 生成；追加到编辑区后可改占位参数
          </span>
        </div>
        <SqlEditor
          :model-value="config.scriptSource"
          language="javascript"
          placeholder="// Postman 风格 pm API：pm.test / pm.expect / pm.variables.get/set"
          style="height: 300px"
          @update:model-value="$emit('update:config', { ...config, scriptSource: $event })"
        />
      </div>
    </el-form-item>

    <el-form-item label="超时(ms)">
      <el-input-number
        :model-value="config.scriptTimeoutMillis"
        :min="1000"
        :max="60000"
        :step="1000"
        @update:model-value="$emit('update:config', { ...config, scriptTimeoutMillis: $event })"
      />
    </el-form-item>

    <el-form-item label="输出格式">
      <div class="step-ref-hint">
        脚本可通过 <code>pm.variables.set('key', value)</code> 设置变量，
        后续步骤通过 <code>{{$steps[N].stdout}}</code> 或 <code>{{$steps[N].stdoutJson.key}}</code> 引用
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup>
import SqlEditor from '@/components/SqlEditor.vue'
import AiSparkleIcon from '@/components/icons/AiSparkleIcon.vue'

defineProps({
  config: { type: Object, required: true },
})

defineEmits(['update:config', 'insert-script-library', 'ai-generate-script'])
</script>

<style scoped>
.scenario-script-field {
  width: 100%;
}
.scenario-script-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.scenario-script-toolbar-hint {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}
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
