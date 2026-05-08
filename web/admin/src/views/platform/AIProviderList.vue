<template>
  <div :class="embedded ? 'ai-provider-root' : 'page-card'">
    <div v-if="!embedded" class="page-header ai-provider-header">
      <div>
        <h2 class="page-title">AI 提供商</h2>
        <p class="page-subtitle">
          按项目维护 AI 大模型提供商配置，供运行控制台、断言编辑器与场景脚本的「AI 生成」入口使用。
          支持 DeepSeek、Xiaomi、OpenAI、Anthropic、Kimi、Ollama；developer+ 可写、viewer 可读。
        </p>
      </div>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增提供商</el-button>
    </div>
    <div v-else class="ai-provider-embedded-toolbar">
      <p class="page-subtitle">
        按当前全局项目维护 AI 大模型提供商配置，供运行控制台、断言编辑器与场景脚本的「AI 生成」入口使用。
        支持 DeepSeek、Xiaomi、OpenAI、Anthropic、Kimi、Ollama；developer+ 可写、viewer 可读。
      </p>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增提供商</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在左侧列表或顶部选择项目后再管理 AI 提供商。"
    />

    <el-table v-if="projectId" :data="providers" border row-key="id" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="类型" width="140">
        <template #default="{ row }">
          <el-tag size="small">{{ typeLabel(row.providerType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="defaultModel" label="默认模型" min-width="160" show-overflow-tooltip />
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
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="primary" link :loading="testingId === row.id" @click="test(row)">测试</el-button>
          <el-button v-if="!row.isDefault" type="primary" link @click="setDefault(row)">设为默认</el-button>
          <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="projectId && !loading && !providers.length" description="暂无 AI 提供商，点击右上角「新增提供商」创建" />

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
        <el-form-item label="默认模型">
          <el-autocomplete
            v-model="form.defaultModel"
            :fetch-suggestions="modelSuggestions"
            placeholder="如 deepseek-chat、gpt-4o-mini，调用时仍可覆盖"
            style="width: 100%"
            clearable
          />
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
          <span class="meta-hint">同一项目最多一个默认提供商；运行控制台等入口默认选中。</span>
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
  listAIProviderTypes,
  listAIProviders,
  testAIProvider,
  updateAIProvider
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

function pad2(n) {
  return String(n).padStart(2, '0')
}

const blankForm = () => ({
  name: '',
  providerType: '',
  baseUrl: '',
  apiKey: '',
  defaultModel: '',
  extraConfig: '',
  enabled: true,
  isDefault: false
})

export default {
  name: 'AIProviderList',
  props: {
    embedded: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      projectState,
      loading: false,
      providers: [],
      providerTypes: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      testingId: null,
      deletingId: null,
      form: blankForm()
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId || ''
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
    }
  },
  watch: {
    projectId: {
      immediate: true,
      handler(val) {
        if (val) this.loadList()
        else this.providers = []
      }
    }
  },
  async created() {
    if (!this.embedded) loadGlobalProjects()
    try {
      this.providerTypes = await listAIProviderTypes()
    } catch (_) {
      this.providerTypes = []
    }
  },
  methods: {
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
    modelSuggestions(query, cb) {
      const meta = this.currentTypeMeta
      const list = (meta?.models || []).map((m) => ({ value: m }))
      if (!query) return cb(list)
      const lower = String(query).toLowerCase()
      cb(list.filter((item) => item.value.toLowerCase().includes(lower)))
    },
    async loadList() {
      if (!this.projectId) return
      this.loading = true
      try {
        this.providers = await listAIProviders(this.projectId)
      } finally {
        this.loading = false
      }
    },
    onTypeChange(value) {
      const meta = this.providerTypes.find((t) => t.type === value)
      if (!meta) return
      if (!this.form.baseUrl && meta.defaultBaseUrl) this.form.baseUrl = meta.defaultBaseUrl
      if (!this.form.defaultModel && meta.defaultModel) this.form.defaultModel = meta.defaultModel
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
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editingId = row.id
      const extra = row.extraConfig ? JSON.stringify(row.extraConfig, null, 2) : ''
      this.form = {
        name: row.name,
        providerType: row.providerType,
        baseUrl: row.baseUrl,
        apiKey: '',
        defaultModel: row.defaultModel || '',
        extraConfig: extra,
        enabled: !!row.enabled,
        isDefault: !!row.isDefault
      }
      this.dialogVisible = true
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

      const payload = {
        name,
        providerType: this.form.providerType,
        baseUrl: (this.form.baseUrl || '').trim(),
        defaultModel: (this.form.defaultModel || '').trim(),
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
          await updateAIProvider(this.projectId, this.editingId, payload)
          this.$message.success('已保存')
        } else {
          payload.apiKey = this.form.apiKey || ''
          await createAIProvider(this.projectId, payload)
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
        const res = await testAIProvider(this.projectId, row.id)
        const snippet = (res?.text || '').slice(0, 200)
        this.$message.success(`连接成功（${res?.elapsedMillis || 0}ms）：${snippet}`)
      } finally {
        this.testingId = null
      }
    },
    async setDefault(row) {
      try {
        await updateAIProvider(this.projectId, row.id, {
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
        await deleteAIProvider(this.projectId, row.id)
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
  font-size: 12px;
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
  font-size: 12px;
}
</style>
