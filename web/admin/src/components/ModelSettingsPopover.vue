<template>
  <el-popover
    :placement="placement"
    :width="360"
    trigger="click"
    popper-class="ai-model-settings-popper"
    :popper-style="{ padding: '10px 12px' }"
    :disabled="disabled"
  >
    <template #reference>
      <slot name="reference" />
    </template>
    <div class="ai-model-settings">
      <div class="ai-model-settings-picker" role="group" aria-label="模型选择">
        <div class="ai-model-settings-picker__controls">
          <el-select
            :model-value="state.selectedProviderId"
            class="ai-model-settings-select ai-model-settings-select--provider"
            size="small"
            placeholder="提供商"
            filterable
            :teleported="false"
            popper-class="ai-assistant-model-dropdown"
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
          <span class="ai-model-settings-picker__divider" aria-hidden="true" />
          <el-select
            :model-value="state.selectedModel"
            class="ai-model-settings-select ai-model-settings-select--model"
            size="small"
            placeholder="模型"
            filterable
            allow-create
            default-first-option
            :teleported="false"
            popper-class="ai-assistant-model-dropdown"
            :disabled="controlsDisabled || !state.selectedProviderId"
            @change="onModelChange"
          >
            <el-option v-for="model in modelOptions" :key="model" :label="model" :value="model" />
          </el-select>
        </div>
        <el-tooltip content="刷新模型列表" placement="top">
          <button
            type="button"
            class="ai-model-settings-refresh"
            :disabled="!state.selectedProviderId || controlsDisabled"
            aria-label="刷新模型列表"
            @click="onRefreshModels"
          >
            <el-icon><Refresh /></el-icon>
          </button>
        </el-tooltip>
      </div>

      <p v-if="state.modelsWarning" class="ai-model-settings-hint ai-model-settings-hint--warn">
        {{ state.modelsWarning }}
      </p>
      <p v-else-if="state.remoteModelList.length" class="ai-model-settings-hint">
        已加载 {{ state.remoteModelList.length }} 个模型
      </p>

      <div class="ai-model-settings-debug">
        <el-switch
          :model-value="state.debugEnabled"
          inline-prompt
          active-text="开"
          inactive-text="关"
          :disabled="controlsDisabled"
          @change="onDebugChange"
        />
        <span class="ai-model-settings-debug-hint">Debug：在对话中显示每轮 Token 与缓存详情</span>
      </div>
    </div>
  </el-popover>
</template>

<script>
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import {
  assistantState,
  getPane,
  refreshPaneProviderModels,
  setPaneDebug,
  setPaneModel,
  setPaneProvider,
} from '../stores/aiAssistant'
import { collectModelOptionIds, formatProviderLabel } from '../utils/aiProvider'

export default {
  name: 'ModelSettingsPopover',
  components: { Refresh },
  props: {
    paneId: { type: String, default: 'panel' },
    disabled: { type: Boolean, default: false },
    busy: { type: Boolean, default: false },
    placement: { type: String, default: 'top-start' },
  },
  computed: {
    pane() {
      return getPane(this.paneId)
    },
    state() {
      return {
        ...assistantState,
        selectedProviderId: this.pane.selectedProviderId,
        selectedModel: this.pane.selectedModel,
        debugEnabled: this.pane.debugEnabled,
        remoteModelList: this.pane.remoteModelList,
        modelsWarning: this.pane.modelsWarning,
      }
    },
    controlsDisabled() {
      return this.disabled || this.busy
    },
    providerOptions() {
      return assistantState.providers.filter((p) => p.enabled !== false)
    },
    modelOptions() {
      const provider = this.providerOptions.find((p) => p.id === this.pane.selectedProviderId)
      return collectModelOptionIds({
        remoteModelList: this.pane.remoteModelList,
        selectedModel: this.pane.selectedModel,
        defaultModel: provider?.defaultModel,
      })
    },
  },
  methods: {
    providerLabel(provider) {
      return formatProviderLabel(provider)
    },
    onProviderChange(value) {
      setPaneProvider(this.paneId, value)
    },
    onModelChange(value) {
      setPaneModel(this.paneId, value)
    },
    onDebugChange(value) {
      setPaneDebug(this.paneId, value)
    },
    async onRefreshModels() {
      const id = this.pane.selectedProviderId
      if (!id) return
      await refreshPaneProviderModels(this.paneId, id)
      const n = this.pane.remoteModelList.length
      if (n) ElMessage.success(`已加载 ${n} 个模型`)
      else if (this.pane.modelsWarning) ElMessage.warning(this.pane.modelsWarning)
      else ElMessage.warning('未获取到模型')
    },
  },
}
</script>

<style scoped>
.ai-model-settings {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ai-model-settings-picker {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 4px 6px 4px 8px;
  border-radius: 10px;
  border: 1px solid var(--app-border-color, var(--el-border-color-lighter, #e4e7ed));
  background: var(--app-surface-subtle, var(--el-fill-color-lighter, #f5f7fa));
}

.ai-model-settings-picker__controls {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  min-width: 0;
}

.ai-model-settings-picker__divider {
  flex-shrink: 0;
  width: 1px;
  height: 14px;
  margin: 0 2px;
  background: var(--app-border-color, var(--el-border-color, #dcdfe6));
  opacity: 0.85;
}

.ai-model-settings-select {
  flex: 1 1 0;
  min-width: 0;
}

.ai-model-settings-select--provider {
  flex: 0 1 38%;
  max-width: 42%;
}

.ai-model-settings-select--model {
  flex: 1 1 0;
}

.ai-model-settings-select :deep(.el-select__wrapper) {
  width: 100%;
  min-height: 26px;
  padding: 0 6px;
  font-size: 12px;
  box-shadow: none;
  border: none;
  border-radius: 6px;
  background: transparent;
}

.ai-model-settings-select :deep(.el-select__wrapper:hover) {
  background: color-mix(in srgb, var(--app-card-bg, #fff) 55%, transparent);
}

.ai-model-settings-select :deep(.el-select__wrapper.is-focused) {
  background: var(--app-card-bg, #fff);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--el-color-primary) 35%, transparent);
}

.ai-model-settings-select :deep(.el-select__placeholder),
.ai-model-settings-select :deep(.el-select__selected-item) {
  font-size: 12px;
}

.ai-model-settings-select :deep(.el-select__placeholder) {
  color: var(--app-text-muted, var(--el-text-color-placeholder, #a8abb2));
}

.ai-model-settings-select :deep(.el-select__selected-item) {
  font-weight: 500;
}

.ai-model-settings-refresh {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-muted, var(--el-text-color-secondary, #909399));
  font-size: 14px;
  cursor: pointer;
  transition: color 0.15s ease, background 0.15s ease;
}

.ai-model-settings-refresh:hover:not(:disabled) {
  color: var(--el-color-primary, #409eff);
  background: color-mix(in srgb, var(--el-color-primary) 10%, transparent);
}

.ai-model-settings-refresh:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.ai-model-settings-hint {
  margin: 0;
  font-size: 11px;
  color: var(--app-text-muted, var(--el-text-color-secondary, #909399));
  line-height: 1.4;
}

.ai-model-settings-hint--warn {
  color: var(--el-color-warning);
}

.ai-model-settings-debug {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding-top: 2px;
}

.ai-model-settings-debug-hint {
  font-size: 11px;
  color: var(--app-text-muted, var(--el-text-color-secondary, #909399));
  line-height: 1.4;
}
</style>

<style>
/* 下拉不 teleport 到 body，避免点选选项被 popover 判为外部点击而关闭 */
.ai-model-settings-popper.el-popover {
  overflow: visible;
}
</style>
