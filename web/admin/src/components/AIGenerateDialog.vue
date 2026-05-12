<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="720px"
    align-center
    destroy-on-close
    @close="onClose"
  >
    <el-alert
      v-if="!projectId"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再使用 AI 生成"
    />

    <el-alert
      v-else-if="!loading && !providers.length"
      type="warning"
      show-icon
      :closable="false"
      title="当前项目尚未配置 AI 提供商"
      class="ai-no-provider"
    >
      <template #default>
        请前往
        <router-link to="/ai-providers" class="ai-link" @click="visible = false">「平台资源 / AI 提供商」</router-link>
        新增并启用至少一个提供商。
      </template>
    </el-alert>

    <div v-if="projectId && providers.length && needsIntentCapture && !result && !running" class="ai-intent-block">
      <div class="ai-intent-label">
        测试意图 <span class="ai-intent-required">*</span>
      </div>
      <p class="ai-intent-hint">用中文说明希望脚本校验的要点（状态码、关键 JSON 字段、业务规则等）。</p>
      <el-input
        v-model="form.intent"
        type="textarea"
        :autosize="{ minRows: 4, maxRows: 12 }"
        maxlength="2000"
        show-word-limit
        placeholder="例如：断言 HTTP 200；body.code 为 0；data.id 为非空字符串"
      />
    </div>

    <div v-if="projectId && providers.length && running && !result" class="ai-generating">
      <el-icon class="ai-spin"><Loading /></el-icon>
      <span>AI 生成中，请稍候…</span>
    </div>

    <div v-if="result" class="ai-result">
      <div class="ai-result-meta">
        模型：<span>{{ result.model || '-' }}</span>
        ·
        耗时：<span>{{ result.elapsedMillis || 0 }} ms</span>
        <el-tag v-if="result.parseWarnings" type="warning" size="small" class="ai-warning">{{ result.parseWarnings }}</el-tag>
      </div>
      <el-input
        :model-value="resultDisplay"
        type="textarea"
        :autosize="{ minRows: 8, maxRows: 18 }"
        readonly
        class="ai-result-text"
      />
    </div>

    <template #footer>
      <template v-if="running">
        <el-button :loading="true" disabled>生成中…</el-button>
      </template>
      <template v-else-if="result">
        <el-button @click="run">重新生成</el-button>
        <el-button v-if="action !== 'generate_assertion'" @click="copyText">复制</el-button>
        <el-button type="primary" :disabled="!canApply" @click="apply">应用</el-button>
      </template>
      <template v-else>
        <el-button @click="visible = false">关闭</el-button>
        <el-button
          v-if="needsIntentCapture && projectId && providers.length"
          type="primary"
          :disabled="!form.providerId"
          @click="run"
        >
          开始生成
        </el-button>
      </template>
    </template>
  </el-dialog>
</template>

<script>
import { Loading } from '@element-plus/icons-vue'
import { aiChat, listAIProviderTypes, listAIProviders, listProjectAIPrompts } from '../api'
import { projectState } from '../utils/currentProject'

const ACTION_TITLES = {
  generate_params: 'AI 生成请求参数',
  generate_assertion: 'AI 生成断言脚本',
  generate_case_data: 'AI 生成测试数据行',
  raw: 'AI 生成'
}

export default {
  name: 'AIGenerateDialog',
  components: { Loading },
  props: {
    modelValue: { type: Boolean, default: false },
    action: { type: String, default: 'raw' },
    title: { type: String, default: '' },
    context: { type: [Object, Array, String, Number, Boolean, null], default: null }
  },
  emits: ['update:modelValue', 'apply', 'generation-settled'],
  data() {
    return {
      providers: [],
      providerTypes: [],
      loading: false,
      running: false,
      result: null,
      form: {
        providerId: '',
        model: '',
        intent: ''
      }
    }
  },
  computed: {
    visible: {
      get() {
        return this.modelValue
      },
      set(val) {
        this.$emit('update:modelValue', val)
      }
    },
    projectId() {
      return projectState.currentProjectId || ''
    },
    dialogTitle() {
      return this.title || ACTION_TITLES[this.action] || 'AI 生成'
    },
    resultDisplay() {
      if (!this.result) return ''
      if (this.result.parsed) {
        try {
          return JSON.stringify(this.result.parsed, null, 2)
        } catch (_) {
          // fallthrough
        }
      }
      return this.result.text || ''
    },
    canApply() {
      return !!this.result && (this.result.parsed || this.result.text)
    },
    /** 断言/脚本类生成必须先由用户填写意图，不自动发起首次请求 */
    needsIntentCapture() {
      return this.action === 'generate_assertion'
    }
  },
  watch: {
    visible(val) {
      if (val) this.onOpen()
    }
  },
  methods: {
    typeLabel(type) {
      const meta = this.providerTypes.find((t) => t.type === type)
      return meta?.label || type
    },
    async ensureTypes() {
      if (this.providerTypes.length) return
      try {
        this.providerTypes = await listAIProviderTypes()
      } catch (_) {
        this.providerTypes = []
      }
    },
    async onOpen() {
      this.result = null
      this.form.model = ''
      this.form.intent = ''
      await this.ensureTypes()
      if (!this.projectId) {
        this.providers = []
        this.$emit('generation-settled')
        return
      }
      this.loading = true
      try {
        this.providers = await listAIProviders(this.projectId)
      } catch (_) {
        this.providers = []
      } finally {
        this.loading = false
      }
      if (!this.providers.length) {
        this.$emit('generation-settled')
        return
      }

      const def = this.providers.find((p) => p.isDefault && p.enabled) || this.providers.find((p) => p.enabled)
      let providerId = def ? def.id : (this.providers[0]?.id || '')

      try {
        const promptConfigs = await listProjectAIPrompts(this.projectId)
        const matched = promptConfigs.find((c) => c.action === this.action && c.enabled)
        if (matched?.providerId) {
          const pinned = this.providers.find((p) => p.id === matched.providerId && p.enabled)
          if (pinned) {
            providerId = pinned.id
          }
        }
        if (matched && matched.defaultModel) {
          this.form.model = matched.defaultModel
        }
      } catch (_) {
        // 加载失败不阻断主流程
      }

      this.form.providerId = providerId
      if (this.needsIntentCapture) {
        this.$emit('generation-settled')
        return
      }
      await this.run()
    },
    onClose() {
      this.result = null
    },
    async run() {
      if (!this.form.providerId) {
        this.$message.warning('未找到可用的 AI 提供商')
        this.$emit('generation-settled')
        return
      }
      let prompt = ''
      if (this.needsIntentCapture) {
        prompt = String(this.form.intent || '').trim()
        if (!prompt) {
          this.$message.warning('请填写测试意图')
          this.$emit('generation-settled')
          return
        }
      }
      this.running = true
      try {
        const res = await aiChat(this.projectId, {
          providerId: this.form.providerId,
          action: this.action,
          prompt,
          model: (this.form.model || '').trim(),
          context: this.context ?? null
        })
        this.result = res
      } finally {
        this.running = false
        this.$emit('generation-settled')
      }
    },
    apply() {
      if (!this.result) return
      const payload = {
        action: this.action,
        text: this.result.text || '',
        parsed: this.result.parsed || null,
        model: this.result.model
      }
      this.$emit('apply', payload)
      this.visible = false
    },
    async copyText() {
      const text = this.resultDisplay
      if (!text) return
      try {
        await navigator.clipboard.writeText(text)
        this.$message.success('已复制到剪贴板')
      } catch (_) {
        this.$message.warning('当前环境不支持剪贴板，请手动选择复制')
      }
    }
  }
}
</script>

<style scoped>
.ai-intent-block {
  margin-top: 4px;
  margin-bottom: 8px;
}

.ai-intent-label {
  font-size: var(--app-font-size-base);
  font-weight: 500;
  color: var(--el-text-color-primary, #303133);
  margin-bottom: 6px;
}

.ai-intent-required {
  color: var(--el-color-danger, #f56c6c);
}

.ai-intent-hint {
  margin: 0 0 8px;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text, #909399);
  line-height: 1.5;
}

.ai-no-provider {
  margin-bottom: 12px;
}

.ai-link {
  color: var(--el-color-primary);
  text-decoration: none;
}

.ai-link:hover {
  text-decoration: underline;
}

.ai-generating {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 32px 0;
  justify-content: center;
  color: var(--app-secondary-text, #909399);
  font-size: var(--app-font-size-base);
}

.ai-spin {
  font-size: calc(var(--app-font-size-base) + 8px);
  animation: ai-rotating 1s linear infinite;
}

@keyframes ai-rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.ai-result-text :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.55;
}

.ai-result {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ai-result-meta {
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text, #909399);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.ai-result-meta span {
  color: var(--el-text-color-primary, #303133);
}

.ai-warning {
  margin-left: 4px;
}
</style>
