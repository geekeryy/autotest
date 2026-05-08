<template>
  <div :class="embedded ? 'prompt-root' : 'page-card'">
    <div v-if="!embedded" class="page-header prompt-header">
      <div>
        <h2 class="page-title">Prompt 管理</h2>
        <p class="page-subtitle">
          按项目维护 AI 生成动作的 Prompt 配置，可覆盖内置默认提示词并指定默认模型。
        </p>
      </div>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增 Prompt</el-button>
    </div>
    <div v-else class="prompt-embedded-toolbar">
      <p class="page-subtitle">
        按当前全局项目维护 AI 生成动作的 Prompt 配置，可覆盖内置默认提示词并指定默认模型。
      </p>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增 Prompt</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在左侧列表或顶部选择项目后再管理 Prompt 配置。"
    />

    <el-table v-if="projectId" :data="prompts" border row-key="id" v-loading="loading">
      <el-table-column label="动作" width="180">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ actionLabel(row.action) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="名称" min-width="160" show-overflow-tooltip />
      <el-table-column prop="defaultModel" label="默认模型" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.defaultModel">{{ row.defaultModel }}</span>
          <span v-else class="text-secondary">（使用提供商默认）</span>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="90">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled"
            :loading="togglingId === row.id"
            @change="(val) => toggleEnabled(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty
      v-if="projectId && !loading && !prompts.length"
      description="暂无 Prompt 配置，点击右上角「新增 Prompt」创建"
    />

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="680px"
      align-center
      destroy-on-close
    >
      <el-form :model="form" label-width="120px">
        <el-form-item label="动作" required>
          <el-select
            v-model="form.action"
            placeholder="选择生成动作"
            style="width: 100%"
            :disabled="!!editingId"
            @change="onActionChange"
          >
            <el-option
              v-for="a in ACTION_OPTIONS"
              :key="a.value"
              :label="a.label"
              :value="a.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如：生成断言（严格模式）" />
        </el-form-item>
        <el-form-item label="System Prompt" required>
          <el-input
            v-model="form.systemPrompt"
            type="textarea"
            :autosize="{ minRows: 8, maxRows: 20 }"
            placeholder="输入 System Prompt，选择动作后将自动预填内置默认提示词"
            class="prompt-textarea"
          />
        </el-form-item>
        <el-form-item label="默认模型">
          <el-input
            v-model="form.defaultModel"
            placeholder="留空则不覆盖提供商默认模型"
          />
        </el-form-item>
        <el-form-item label="AI 提供商">
          <el-select
            v-model="form.providerId"
            clearable
            filterable
            placeholder="跟随项目默认"
            style="width: 100%"
          >
            <el-option label="跟随项目默认" value="" />
            <el-option
              v-for="p in enabledProviders"
              :key="p.id"
              :label="providerOptionLabel(p)"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
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
  listProjectAIPrompts,
  createProjectAIPrompt,
  updateProjectAIPrompt,
  deleteProjectAIPrompt,
  listAIProviders,
  listAIProviderTypes
} from '../../api'
import { projectState } from '../../utils/currentProject'

const ACTION_OPTIONS = [
  { value: 'generate_params', label: '生成请求参数' },
  { value: 'generate_assertion', label: '生成断言脚本' },
  { value: 'generate_case_data', label: '生成测试用例数据' },
  { value: 'raw', label: '通用对话' }
]

const DEFAULT_SYSTEM_PROMPTS = {
  generate_params: `你是接口自动化测试平台的「请求参数生成器」。
- 输入会包含一段 OpenAPI/Swagger 描述（包括 method、path、parameters、requestBody schema 等）以及当前用户已有的参数。
- 输出**必须是单个 JSON 对象**，结构如下：
{
  "path": { "<参数名>": "<取值>" },
  "query": { "<参数名>": "<取值>" },
  "headers": { "<参数名>": "<取值>" },
  "body": <按 schema 生成的合法请求体>
}
- 严禁输出任何解释文字或 Markdown，只输出 JSON。
- 优先使用 schema 中的 example、default、enum；其次按 format/字段名启发式生成真实仿真值。
- 参数名严格保持与输入一致（区分大小写）。
- 不需要的分区可省略，但顶层至少包含一个非空字段。`,
  generate_assertion: `你是接口自动化测试平台的「断言脚本生成器」。
- 用户在弹窗中必填「测试意图」（中文）；空意图不会发起生成。
- 输入包含响应快照（status、headers、body）以及上述意图描述。
- 输出 Postman 风格 JavaScript 断言代码，可直接粘贴到平台脚本断言编辑器中。
- 可用 API：pm.test('名称', () => { ... })、pm.response.code、pm.response.json()、pm.expect(...).to.equal(...) 等。
- 至少包含一个 pm.test，覆盖响应状态码与关键业务字段。
- 不要输出 Markdown 围栏（不要使用三个反引号）；直接输出 JS 源代码，不要任何额外解释。`,
  generate_case_data: `你是接口自动化测试平台的「测试用例数据生成器」。
- 输入包含接口的请求 schema、字段约束、以及希望生成的用例数量与场景描述。
- 输出**必须是单个 JSON 对象**，结构如下：
{
  "cases": [
    { "name": "<用例名称，简短中文>", "body": <符合 schema 的请求体>, "notes": "<可选说明>" }
  ]
}
- 至少覆盖正向用例与典型边界/异常用例（如必填缺失、枚举值边界、字符串极限长度）。
- 严禁输出 JSON 之外的任何文字。`,
  raw: `你是接口自动化测试平台的通用 AI 助手，用中文给出简洁、可执行的回答。`
}

const blankForm = () => ({
  action: '',
  name: '',
  systemPrompt: '',
  defaultModel: '',
  providerId: '',
  enabled: true
})

export default {
  name: 'ProjectPromptList',
  props: {
    embedded: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      projectState,
      ACTION_OPTIONS,
      loading: false,
      prompts: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      togglingId: null,
      deletingId: null,
      form: blankForm(),
      aiProviders: [],
      providerTypes: [],
      loadingProviders: false
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId || ''
    },
    dialogTitle() {
      return this.editingId ? '编辑 Prompt 配置' : '新增 Prompt 配置'
    },
    enabledProviders() {
      return (this.aiProviders || []).filter((p) => p.enabled)
    }
  },
  watch: {
    projectId: {
      immediate: true,
      async handler(val) {
        if (val) {
          this.loadList()
          this.loadAiProvidersIfNeeded(false)
        } else {
          this.prompts = []
          this.aiProviders = []
        }
      }
    }
  },
  methods: {
    actionLabel(action) {
      const opt = ACTION_OPTIONS.find((a) => a.value === action)
      return opt ? opt.label : action
    },
    providerOptionLabel(p) {
      const meta = this.providerTypes.find((t) => t.type === p.providerType)
      const typeLabel = meta?.label || p.providerType
      const dm = (p.defaultModel || '').trim() ? p.defaultModel : '—'
      return `${p.name} · ${typeLabel} · ${dm}`
    },
    async loadList() {
      if (!this.projectId) return
      this.loading = true
      try {
        this.prompts = await listProjectAIPrompts(this.projectId)
      } finally {
        this.loading = false
      }
    },
    async loadAiProvidersIfNeeded(force) {
      if (!this.projectId) return
      if (!force && this.aiProviders.length) return
      this.loadingProviders = true
      try {
        if (!this.providerTypes.length) {
          try {
            this.providerTypes = await listAIProviderTypes()
          } catch (_) {
            this.providerTypes = []
          }
        }
        this.aiProviders = await listAIProviders(this.projectId)
      } catch (_) {
        this.aiProviders = []
      } finally {
        this.loadingProviders = false
      }
    },
    onActionChange(value) {
      if (DEFAULT_SYSTEM_PROMPTS[value] && !this.form.systemPrompt) {
        this.form.systemPrompt = DEFAULT_SYSTEM_PROMPTS[value]
      }
    },
    async openCreate() {
      this.editingId = null
      this.form = blankForm()
      await this.loadAiProvidersIfNeeded(true)
      this.dialogVisible = true
    },
    async openEdit(row) {
      this.editingId = row.id
      await this.loadAiProvidersIfNeeded(true)
      this.form = {
        action: row.action,
        name: row.name,
        systemPrompt: row.systemPrompt || '',
        defaultModel: row.defaultModel || '',
        providerId: row.providerId ? String(row.providerId) : '',
        enabled: !!row.enabled
      }
      this.dialogVisible = true
    },
    async toggleEnabled(row, val) {
      this.togglingId = row.id
      try {
        await updateProjectAIPrompt(this.projectId, row.id, {
          name: row.name,
          systemPrompt: row.systemPrompt,
          defaultModel: row.defaultModel || '',
          providerId: row.providerId || null,
          enabled: val
        })
        row.enabled = val
      } finally {
        this.togglingId = null
      }
    },
    async submit() {
      const name = (this.form.name || '').trim()
      if (!this.form.action) {
        this.$message.warning('请选择动作')
        return
      }
      if (!name) {
        this.$message.warning('请填写名称')
        return
      }
      if (!(this.form.systemPrompt || '').trim()) {
        this.$message.warning('请填写 System Prompt')
        return
      }

      const payload = {
        action: this.form.action,
        name,
        systemPrompt: (this.form.systemPrompt || '').trim(),
        defaultModel: (this.form.defaultModel || '').trim(),
        providerId: this.form.providerId || null,
        enabled: !!this.form.enabled
      }

      this.saving = true
      try {
        if (this.editingId) {
          await updateProjectAIPrompt(this.projectId, this.editingId, payload)
          this.$message.success('已保存')
        } else {
          await createProjectAIPrompt(this.projectId, payload)
          this.$message.success('已创建')
        }
        this.dialogVisible = false
        await this.loadList()
      } finally {
        this.saving = false
      }
    },
    async remove(row) {
      try {
        await this.$confirm(`确定删除 Prompt「${row.name}」？`, '删除确认', { type: 'warning' })
      } catch {
        return
      }
      this.deletingId = row.id
      try {
        await deleteProjectAIPrompt(this.projectId, row.id)
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
.prompt-embedded-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.prompt-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.project-hint {
  margin-bottom: 12px;
}

.text-secondary {
  color: var(--app-secondary-text, #909399);
  font-size: 12px;
}

.prompt-textarea :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
}
</style>
