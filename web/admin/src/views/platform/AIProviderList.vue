<template>
  <div :class="embedded ? 'ai-provider-root' : 'page-card'">
    <div v-if="!embedded" class="page-header ai-provider-header">
      <div>
        <h2 class="page-title">AI 提供商</h2>
        <p class="page-subtitle">
          平台级 AI 大模型提供商配置，供运行控制台、断言编辑器与场景脚本的「AI 生成」入口使用。
          支持 DeepSeek、Xiaomi、OpenAI、Anthropic、Kimi、Ollama。
        </p>
      </div>
      <el-button v-if="canWrite" type="primary" @click="openCreate">新增提供商</el-button>
    </div>
    <div v-else class="ai-provider-embedded-toolbar">
      <p class="page-subtitle">
        平台级 AI 大模型提供商配置，供运行控制台、断言编辑器与场景脚本的「AI 生成」入口使用。
        拥有「管理项目」权限的用户可新增与编辑。
      </p>
      <el-button v-if="canWrite" type="primary" @click="openCreate">新增提供商</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="loadList" />

    <el-table :data="providers" border row-key="id" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="类型" width="140">
        <template #default="{ row }">
          <el-tag size="small">{{ typeLabel(row.providerType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="defaultModel" label="文本模型" min-width="140" show-overflow-tooltip />
      <el-table-column label="图片模型" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">{{ row.modalityModels?.image || '—' }}</template>
      </el-table-column>
      <el-table-column prop="baseUrl" label="Base URL" min-width="220" show-overflow-tooltip />
      <el-table-column label="API Key" width="180">
        <template #default="{ row }">
          <span v-if="row.apiKeyMasked" class="ai-secret">{{ row.apiKeyMasked }}</span>
          <el-tag v-else type="info" size="small">未设置</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="160">
        <template #default="{ row }">
          <el-tag v-if="row.enabled" type="success" size="small">启用</el-tag>
          <el-tag v-else type="info" size="small">停用</el-tag>
          <el-tag v-if="row.isDefault" type="warning" size="small" style="margin-left: 6px">默认</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="178">
        <template #default="{ row }">{{ formatDateTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <template v-if="canWrite">
            <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
            <el-button type="primary" link :loading="testingId === row.id" @click="test(row)">测试</el-button>
            <el-button v-if="!row.isDefault" type="primary" link @click="setDefault(row)">设为默认</el-button>
            <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
          </template>
          <span v-else class="text-secondary">只读</span>
        </template>
      </el-table-column>
    </el-table>
    <el-empty
      v-if="!loading && !loadError && !providers.length"
      description="暂无 AI 提供商，点击右上角「新增提供商」创建"
    />

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="640px"
      align-center
      destroy-on-close
    >
      <el-form :model="form" label-width="120px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如：DeepSeek-生产" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="form.providerType" placeholder="选择提供商类型" style="width: 100%" @change="onTypeChange">
            <el-option v-for="t in providerTypes" :key="t.type" :label="t.label" :value="t.type" />
          </el-select>
          <div v-if="currentTypeMeta?.notes" class="meta-notes">{{ currentTypeMeta.notes }}</div>
        </el-form-item>
        <el-form-item label="Base URL" required>
          <el-input v-model="form.baseUrl" placeholder="https://api.example.com/v1" />
        </el-form-item>
        <el-form-item :label="apiKeyLabel" :required="apiKeyRequired">
          <el-input
            v-model="form.apiKey"
            type="password"
            show-password
            :placeholder="editingId ? '留空则保持原 API Key 不变' : '请输入 API Key'"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="文本模型">
          <div class="model-field-row">
            <el-select
              v-model="form.defaultModel"
              class="model-select"
              placeholder="纯文本对话默认模型"
              filterable
              allow-create
              default-first-option
              clearable
              :loading="modelsLoading"
              :disabled="!canFetchModels && !remoteModelEntries.length"
              no-data-text="请先填写 Base URL 与 API Key，再点「刷新列表」"
            >
              <el-option
                v-for="m in modelSelectOptions"
                :key="m.id"
                :label="modelOptionLabel(m)"
                :value="m.id"
              />
            </el-select>
            <el-button
              type="primary"
              link
              :loading="modelsLoading"
              :disabled="!canFetchModels"
              @click="refreshModels"
            >
              刷新列表
            </el-button>
          </div>
          <div v-if="modelsHint" class="meta-notes">{{ modelsHint }}</div>
          <div v-if="modelsWarning" class="meta-notes models-warning">{{ modelsWarning }}</div>
        </el-form-item>
        <el-form-item label="图片模型">
          <el-select
            v-model="form.modalityModels.image"
            class="model-select"
            placeholder="含图消息时自动切换"
            filterable
            allow-create
            default-first-option
            clearable
            :loading="modelsLoading"
          >
            <el-option
              v-for="m in modelSelectOptions"
              :key="'img-' + m.id"
              :label="modelOptionLabel(m)"
              :value="m.id"
            />
          </el-select>
          <p class="meta-notes">从上游列表自行选择用于含图消息的模型；AI 助理上传图片前须配置此项。</p>
        </el-form-item>
        <el-form-item label="音频模型">
          <el-select
            v-model="form.modalityModels.audio"
            class="model-select"
            placeholder="预留：音频输入时切换"
            filterable
            allow-create
            default-first-option
            clearable
            :loading="modelsLoading"
          >
            <el-option
              v-for="m in modelSelectOptions"
              :key="'aud-' + m.id"
              :label="modelOptionLabel(m)"
              :value="m.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="视频模型">
          <el-select
            v-model="form.modalityModels.video"
            class="model-select"
            placeholder="预留：视频输入时切换"
            filterable
            allow-create
            default-first-option
            clearable
            :loading="modelsLoading"
          >
            <el-option
              v-for="m in modelSelectOptions"
              :key="'vid-' + m.id"
              :label="modelOptionLabel(m)"
              :value="m.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="高级参数 JSON">
          <el-input
            v-model="form.extraConfig"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 8 }"
            placeholder='可选；例如 OpenAI 组织 ID 或自定义 headers，{"organization":"org_xxx"}'
            class="extra-input"
            @blur="formatExtra"
          />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="设为默认">
          <el-switch v-model="form.isDefault" />
          <span class="meta-hint">全平台最多一个默认提供商；运行控制台等入口默认选中。</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createAIProvider,
  deleteAIProvider,
  discoverAIProviderModels,
  listAIProviderModels,
  listAIProviderTypes,
  listAIProviders,
  testAIProvider,
  updateAIProvider
} from '../../api'
import { hasPermission } from '../../auth'
import ListLoadError from '../../components/ListLoadError.vue'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'
import { formatModelCapabilities } from '../../utils/modelCapabilities'

function pad2(n) {
  return String(n).padStart(2, '0')
}

const blankModalityModels = () => ({ image: '', audio: '', video: '' })

const blankForm = () => ({
  name: '',
  providerType: '',
  baseUrl: '',
  apiKey: '',
  defaultModel: '',
  modalityModels: blankModalityModels(),
  extraConfig: '',
  enabled: true,
  isDefault: false
})

export default {
  name: 'AIProviderList',
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
      loadError: '',
      providers: [],
      providerTypes: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      testingId: null,
      deletingId: null,
      form: blankForm(),
      remoteModels: [],
      remoteModelEntries: [],
      modelsLoading: false,
      modelsWarning: '',
      modelsHint: '',
      modelsSource: '',
      modelsFetchTimer: null
    }
  },
  computed: {
    canWrite() {
      return hasPermission('projects:write')
    },
    dialogTitle() {
      return this.editingId ? '编辑 AI 提供商' : '新增 AI 提供商'
    },
    currentTypeMeta() {
      return this.providerTypes.find((t) => t.type === this.form.providerType) || null
    },
    apiKeyRequired() {
      if (!this.currentTypeMeta) return true
      return this.currentTypeMeta.apiKeyRequired
    },
    apiKeyLabel() {
      return this.apiKeyRequired ? 'API Key' : 'API Key（可选）'
    },
    canFetchModels() {
      if (!this.form.providerType || !this.form.baseUrl) return false
      if (this.apiKeyRequired) {
        if (this.form.apiKey) return true
        return !!this.editingId
      }
      return true
    },
    modelSelectOptions() {
      const byId = new Map()
      for (const m of this.remoteModelEntries || []) {
        const id = String(m?.id || '').trim()
        if (id) byId.set(id, m)
      }
      const addId = (id) => {
        const v = String(id || '').trim()
        if (!v) return
        if (!byId.has(v)) byId.set(v, { id: v, capabilities: [] })
      }
      addId(this.form.defaultModel)
      addId(this.currentTypeMeta?.defaultModel)
      return Array.from(byId.values()).sort((a, b) => a.id.localeCompare(b.id))
    },
  },
  watch: {
    dialogVisible(val) {
      if (!val) {
        this.clearModelsFetchTimer()
        this.remoteModels = []
        this.remoteModelEntries = []
        this.modelsWarning = ''
        this.modelsHint = ''
        this.modelsSource = ''
      }
    },
    'form.providerType'() {
      this.scheduleModelsFetch()
    },
    'form.baseUrl'() {
      this.scheduleModelsFetch()
    },
    'form.apiKey'() {
      this.scheduleModelsFetch()
    }
  },
  async created() {
    try {
      this.providerTypes = await listAIProviderTypes()
    } catch (_) {
      this.providerTypes = []
    }
    await this.loadList()
  },
  methods: {
    modelOptionLabel(m) {
      const id = String(m?.id || '').trim()
      const caps = formatModelCapabilities(m?.capabilities)
      return caps ? `${id}（${caps}）` : id
    },
    typeLabel(type) {
      const meta = this.providerTypes.find((t) => t.type === type)
      return meta?.label || type
    },
    formatDateTime(iso) {
      if (!iso) return ''
      const d = new Date(iso)
      if (Number.isNaN(d.getTime())) return String(iso)
      return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
    },
    clearModelsFetchTimer() {
      if (this.modelsFetchTimer) {
        clearTimeout(this.modelsFetchTimer)
        this.modelsFetchTimer = null
      }
    },
    scheduleModelsFetch() {
      if (!this.dialogVisible) return
      this.clearModelsFetchTimer()
      this.modelsFetchTimer = setTimeout(() => {
        this.modelsFetchTimer = null
        void this.refreshModels()
      }, 500)
    },
    async refreshModels() {
      if (!this.canFetchModels) {
        this.remoteModels = []
        this.remoteModelEntries = []
        return
      }
      this.modelsLoading = true
      try {
        let extra = null
        try {
          extra = this.parseExtra()
        } catch {
          extra = null
        }
        const payload = {
          providerType: this.form.providerType,
          baseUrl: (this.form.baseUrl || '').trim(),
          apiKey: (this.form.apiKey || '').trim(),
          extraConfig: extra || {}
        }
        if (this.editingId) payload.providerId = this.editingId
        let res
        if (this.editingId && !payload.apiKey) {
          res = await listAIProviderModels(this.editingId)
        } else {
          res = await discoverAIProviderModels(payload)
        }
        const entries = Array.isArray(res?.models)
          ? res.models
              .map((m) => ({
                id: String(m?.id || '').trim(),
                capabilities: Array.isArray(m?.capabilities) ? m.capabilities : [],
              }))
              .filter((m) => m.id)
          : []
        this.remoteModelEntries = entries
        this.remoteModels = entries.map((m) => m.id)
        this.modelsSource = res?.source ? String(res.source) : ''
        this.modelsWarning = res?.warning ? String(res.warning) : ''
        const srcLabel = this.modelsSource === 'api' ? '上游 API' : '内置建议'
        if (entries.length) {
          this.modelsHint = `已加载 ${entries.length} 个模型（${srcLabel}），下拉项括号内为能力标签`
          this.$message.success(`已加载 ${entries.length} 个模型`)
        } else {
          this.modelsHint = '未获取到模型，请检查 Base URL 与 API Key'
        }
        if (!this.form.defaultModel && entries.length) {
          const meta = this.currentTypeMeta
          const preferred = meta?.defaultModel
          if (preferred && entries.some((m) => m.id === preferred)) {
            this.form.defaultModel = preferred
          } else if (entries.length === 1) {
            this.form.defaultModel = entries[0].id
          }
        }
      } catch (e) {
        this.remoteModels = []
        this.remoteModelEntries = []
        this.modelsHint = ''
        this.modelsWarning = e?.message ? String(e.message) : '获取模型列表失败'
      } finally {
        this.modelsLoading = false
      }
    },
    async loadList() {
      this.loading = true
      this.loadError = ''
      try {
        this.providers = await listAIProviders()
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err)
      } finally {
        this.loading = false
      }
    },
    onTypeChange(value) {
      const meta = this.providerTypes.find((t) => t.type === value)
      if (!meta) return
      if (!this.form.baseUrl && meta.defaultBaseUrl) this.form.baseUrl = meta.defaultBaseUrl
      if (!this.form.defaultModel && meta.defaultModel) this.form.defaultModel = meta.defaultModel
      this.scheduleModelsFetch()
    },
    formatExtra() {
      const raw = (this.form.extraConfig || '').trim()
      if (!raw) return
      try {
        const parsed = JSON.parse(raw)
        this.form.extraConfig = JSON.stringify(parsed, null, 2)
      } catch (_) {
        // 留给用户改正，不强制阻断
      }
    },
    openCreate() {
      this.editingId = null
      this.form = blankForm()
      this.remoteModels = []
      this.remoteModelEntries = []
      this.modelsWarning = ''
      this.modelsHint = ''
      this.modelsSource = ''
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editingId = row.id
      const extra = row.extraConfig ? JSON.stringify(row.extraConfig, null, 2) : ''
      const mm = row.modalityModels || {}
      this.form = {
        name: row.name,
        providerType: row.providerType,
        baseUrl: row.baseUrl,
        apiKey: '',
        defaultModel: row.defaultModel || '',
        modalityModels: {
          image: mm.image || '',
          audio: mm.audio || '',
          video: mm.video || '',
        },
        extraConfig: extra,
        enabled: !!row.enabled,
        isDefault: !!row.isDefault
      }
      this.dialogVisible = true
      this.scheduleModelsFetch()
    },
    parseExtra() {
      const raw = (this.form.extraConfig || '').trim()
      if (!raw) return null
      try {
        return JSON.parse(raw)
      } catch (e) {
        throw new Error('高级参数 JSON 格式不合法')
      }
    },
    async submit() {
      const name = (this.form.name || '').trim()
      if (!name) {
        this.$message.warning('请填写名称')
        return
      }
      if (!this.form.providerType) {
        this.$message.warning('请选择提供商类型')
        return
      }
      if (!this.form.baseUrl) {
        this.$message.warning('请填写 Base URL')
        return
      }
      if (!this.editingId && this.apiKeyRequired && !this.form.apiKey) {
        this.$message.warning('请填写 API Key')
        return
      }

      let extra = null
      try {
        extra = this.parseExtra()
      } catch (e) {
        this.$message.error(e.message)
        return
      }

      const modalityModels = {
        image: (this.form.modalityModels?.image || '').trim(),
        audio: (this.form.modalityModels?.audio || '').trim(),
        video: (this.form.modalityModels?.video || '').trim(),
      }

      const payload = {
        name,
        providerType: this.form.providerType,
        baseUrl: (this.form.baseUrl || '').trim(),
        defaultModel: (this.form.defaultModel || '').trim(),
        modalityModels,
        extraConfig: extra || {},
        enabled: !!this.form.enabled,
        isDefault: !!this.form.isDefault
      }

      this.saving = true
      try {
        if (this.editingId) {
          if (this.form.apiKey) {
            payload.apiKey = this.form.apiKey
          }
          await updateAIProvider(this.editingId, payload)
          this.$message.success('已保存')
        } else {
          payload.apiKey = this.form.apiKey || ''
          await createAIProvider(payload)
          this.$message.success('已创建')
        }
        this.dialogVisible = false
        await this.loadList()
      } finally {
        this.saving = false
      }
    },
    async test(row) {
      this.testingId = row.id
      try {
        const res = await testAIProvider(row.id)
        const snippet = (res?.text || '').slice(0, 200)
        this.$message.success(`连接成功（${res?.elapsedMillis || 0}ms）：${snippet}`)
      } finally {
        this.testingId = null
      }
    },
    async setDefault(row) {
      try {
        await updateAIProvider(row.id, {
          name: row.name,
          providerType: row.providerType,
          baseUrl: row.baseUrl,
          defaultModel: row.defaultModel || '',
          extraConfig: row.extraConfig || {},
          enabled: !!row.enabled,
          isDefault: true
        })
        this.$message.success('已设为默认')
        await this.loadList()
      } catch (_) {
        // 错误由全局拦截器提示
      }
    },
    async remove(row) {
      try {
        await this.$confirm(`确定删除提供商「${row.name}」？`, '删除确认', { type: 'warning' })
      } catch {
        return
      }
      this.deletingId = row.id
      try {
        await deleteAIProvider(row.id)
        this.$message.success('已删除')
        await this.loadList()
      } finally {
        this.deletingId = null
      }
    }
  }
}
</script>

<style scoped>
.ai-provider-embedded-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.ai-provider-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.project-hint {
  margin-bottom: 12px;
}

.ai-secret {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: var(--app-font-size-small);
  letter-spacing: 0.4px;
}

.meta-notes {
  margin-top: 4px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.4;
}

.meta-hint {
  margin-left: 8px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.extra-input :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: var(--app-font-size-small);
}

.model-field-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.model-field-row .model-select {
  flex: 1;
  min-width: 0;
}

.models-warning {
  color: var(--el-color-warning);
}
</style>
