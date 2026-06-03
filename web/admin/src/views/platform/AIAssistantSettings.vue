<template>
  <div :class="embedded ? 'ai-assistant-settings-embedded' : 'page-card ai-assistant-settings'">
    <div v-if="!embedded" class="page-header">
      <div>
        <h2 class="page-title">AI 助理配置</h2>
        <p class="page-subtitle">
          平台级 AI 助理运行时参数：工具调用轮次、按需路由、场景生成闭环与向量检索等。
          修改后立即生效，无需重启服务。
        </p>
      </div>
      <el-button v-if="canWrite" type="primary" :loading="saving" @click="submit">保存配置</el-button>
    </div>
    <div v-else class="assistant-embedded-toolbar">
      <p class="page-subtitle">
        平台级 AI 助理运行时参数：工具调用轮次、按需路由、场景生成闭环与向量检索等。
        修改后立即生效，无需重启服务。
      </p>
      <el-button v-if="canWrite" type="primary" :loading="saving" @click="submit">保存配置</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-form v-loading="loading" :model="form" label-width="180px" class="settings-form">
      <template v-for="field in fields" :key="field.key">
        <el-form-item :label="field.label">
          <template v-if="field.type === 'bool'">
            <el-switch v-model="form[field.key]" :disabled="!canWrite" />
          </template>
          <template v-else-if="field.type === 'enum'">
            <el-select v-model="form[field.key]" :disabled="!canWrite" style="width: 100%">
              <el-option
                v-for="opt in field.options"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
          </template>
          <template v-else-if="field.type === 'int'">
            <el-input-number
              v-model="form[field.key]"
              :min="field.min"
              :max="field.max"
              :disabled="!canWrite"
            />
          </template>
          <template v-else-if="field.type === 'float'">
            <el-input-number
              v-model="form[field.key]"
              :min="field.min"
              :max="field.max"
              :step="0.05"
              :precision="2"
              :disabled="!canWrite"
            />
          </template>
          <template v-else-if="field.type === 'password'">
            <el-input
              v-model="form[field.key]"
              type="password"
              show-password
              :placeholder="passwordPlaceholder(field.key)"
              :disabled="!canWrite"
              autocomplete="new-password"
            />
          </template>
          <template v-else>
            <el-input v-model="form[field.key]" :disabled="!canWrite" clearable />
          </template>
          <p class="field-desc">{{ field.description }}</p>
          <p v-if="field.defaultValue !== undefined && field.defaultValue !== ''" class="field-default">
            默认值：{{ formatDefault(field) }}
          </p>
        </el-form-item>
      </template>
    </el-form>

    <p v-if="updatedAt" class="updated-at">上次保存：{{ updatedAt }}</p>
  </div>
</template>

<script>
import { getAIAssistantSettings, updateAIAssistantSettings } from '../../api'
import { hasPermission } from '../../auth'
import ListLoadError from '../../components/ListLoadError.vue'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'

const SECRET_KEYS = new Set(['toolEmbedApiKey'])

const PRESENT_KEYS = {
  toolEmbedApiKey: 'toolEmbedApiKeyPresent',
}

function blankForm() {
  return {
    maxToolHops: 6,
    toolRoutingMode: 'shadow',
    scenarioAutorunEnabled: false,
    scenarioGenDefaultMaxRounds: 3,
    routerConfidenceThreshold: 0.7,
    toolEmbedBaseUrl: '',
    toolEmbedApiKey: '',
    toolEmbedModel: 'text-embedding-3-small',
  }
}

export default {
  name: 'AIAssistantSettings',
  components: { ListLoadError },
  props: {
    embedded: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      loading: false,
      saving: false,
      loadError: '',
      fields: [],
      form: blankForm(),
      secretPresent: {},
      updatedAt: ''
    }
  },
  computed: {
    canWrite() {
      return hasPermission('projects:write')
    }
  },
  created() {
    this.load()
  },
  methods: {
    passwordPlaceholder(key) {
      if (this.secretPresent[PRESENT_KEYS[key]]) {
        return '留空则保持原值不变'
      }
      return '可选'
    },
    formatDefault(field) {
      if (field.type === 'enum' && field.options?.length) {
        const opt = field.options.find((o) => o.value === field.defaultValue)
        return opt ? opt.label : field.defaultValue
      }
      if (field.type === 'bool') {
        return field.defaultValue ? '开启' : '关闭'
      }
      return field.defaultValue
    },
    applySettings(settings) {
      const form = blankForm()
      for (const key of Object.keys(form)) {
        if (settings[key] !== undefined && settings[key] !== null && !SECRET_KEYS.has(key)) {
          form[key] = settings[key]
        }
      }
      this.form = form
      this.secretPresent = {
        toolEmbedApiKeyPresent: settings.toolEmbedApiKeyPresent,
      }
    },
    buildPayload() {
      const payload = { ...this.form }
      for (const key of SECRET_KEYS) {
        const v = String(payload[key] ?? '').trim()
        if (v === '') {
          delete payload[key]
        }
      }
      return payload
    },
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        const res = await getAIAssistantSettings()
        this.fields = res.fields || []
        this.applySettings(res.settings || {})
        this.updatedAt = res.updatedAt ? new Date(res.updatedAt).toLocaleString() : ''
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err, '加载 AI 助理配置失败')
      } finally {
        this.loading = false
      }
    },
    async submit() {
      this.saving = true
      try {
        const res = await updateAIAssistantSettings(this.buildPayload())
        this.applySettings(res.settings || {})
        this.updatedAt = res.updatedAt ? new Date(res.updatedAt).toLocaleString() : ''
        this.form.toolEmbedApiKey = ''
        this.$message.success('AI 助理配置已保存')
      } catch (err) {
        this.$message.error(err?.response?.data?.error || err.message || '保存失败')
      } finally {
        this.saving = false
      }
    }
  }
}
</script>

<style scoped>
.ai-assistant-settings .settings-form {
  max-width: 820px;
}
.field-desc {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}
.field-default {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
.updated-at {
  margin-top: 24px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.assistant-embedded-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
</style>
