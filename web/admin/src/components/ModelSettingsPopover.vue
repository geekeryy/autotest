<template>
  <el-popover
    :placement="placement"
    :width="300"
    trigger="click"
    popper-class="ai-model-settings-popper"
    :popper-style="{ padding: '12px' }"
    :disabled="disabled"
  >
    <template #reference>
      <slot name="reference" />
    </template>
    <div class="ai-model-settings">
      <div class="ai-model-settings-row">
        <span class="ai-model-settings-label">提供商</span>
        <el-select
          :model-value="state.selectedProviderId"
          class="ai-provider-select"
          size="small"
          placeholder="选择提供商"
          filterable
          :disabled="controlsDisabled"
          @change="onProviderChange"
        >
          <el-option
            v-for="provider in providerOptions"
            :key="provider.id"
            :label="providerLabel(provider)"
            :value="provider.id"
          />
        </el-select>
      </div>
      <div class="ai-model-settings-row ai-model-settings-row--model">
        <span class="ai-model-settings-label">模型</span>
        <div class="ai-model-settings-model">
          <el-select
            :model-value="state.selectedModel"
            class="ai-model-select"
            size="small"
            placeholder="选择或输入模型"
            filterable
            allow-create
            default-first-option
            :disabled="controlsDisabled || !state.selectedProviderId"
            @change="onModelChange"
          >
            <el-option v-for="model in modelOptions" :key="model" :label="model" :value="model" />
          </el-select>
          <el-button
            link
            type="primary"
            size="small"
            :disabled="!state.selectedProviderId || controlsDisabled"
            @click="onRefreshModels"
          >
            刷新
          </el-button>
        </div>
      </div>
      <p v-if="state.modelsWarning" class="ai-model-settings-hint ai-model-settings-hint--warn">
        {{ state.modelsWarning }}
      </p>
      <p v-else-if="state.remoteModelList.length" class="ai-model-settings-hint">
        已加载 {{ state.remoteModelList.length }} 个模型，点击下拉查看
      </p>
    </div>
  </el-popover>
</template>

<script>
import { ElMessage } from 'element-plus'
import {
  assistantState,
  refreshProviderModels,
  setAssistantModel,
  setAssistantProvider,
} from '../stores/aiAssistant'

export default {
  name: 'ModelSettingsPopover',
  props: {
    disabled: { type: Boolean, default: false },
    busy: { type: Boolean, default: false },
    placement: { type: String, default: 'top-start' },
  },
  computed: {
    state() {
      return assistantState
    },
    controlsDisabled() {
      return this.disabled || this.busy
    },
    providerOptions() {
      return assistantState.providers.filter((p) => p.enabled !== false)
    },
    modelOptions() {
      const values = new Set()
      for (const model of assistantState.remoteModelList || []) {
        if (String(model || '').trim()) values.add(String(model).trim())
      }
      const provider = this.providerOptions.find((p) => p.id === assistantState.selectedProviderId)
      if (provider?.defaultModel) values.add(provider.defaultModel)
      if (assistantState.selectedModel) values.add(assistantState.selectedModel)
      return Array.from(values).sort()
    },
  },
  methods: {
    providerLabel(provider) {
      if (!provider) return ''
      const suffix = provider.isDefault ? ' · 默认' : ''
      return `${provider.name || provider.providerType}${suffix}`
    },
    onProviderChange(value) {
      setAssistantProvider(value)
    },
    onModelChange(value) {
      setAssistantModel(value)
    },
    async onRefreshModels() {
      const id = assistantState.selectedProviderId
      if (!id) return
      await refreshProviderModels(id)
      const n = assistantState.remoteModelList.length
      if (n) ElMessage.success(`已加载 ${n} 个模型`)
      else if (assistantState.modelsWarning) ElMessage.warning(assistantState.modelsWarning)
      else ElMessage.warning('未获取到模型')
    },
  },
}
</script>

<style scoped>
.ai-model-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-model-settings-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ai-model-settings-label {
  font-size: var(--app-font-size-small);
  color: var(--app-text-muted);
}

.ai-model-settings-model {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-model-settings-model :deep(.el-select) {
  flex: 1;
}

.ai-model-settings-hint {
  margin: 0;
  font-size: var(--app-font-size-small);
  color: var(--app-text-muted);
  line-height: 1.45;
}

.ai-model-settings-hint--warn {
  color: var(--el-color-warning);
}
</style>
