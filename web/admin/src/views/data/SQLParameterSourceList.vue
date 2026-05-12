<template>
  <div class="page-card">
    <div class="page-header sql-source-header">
      <div>
        <h2 class="page-title">SQL 参数源</h2>
        <p class="page-subtitle">配置一次，多接口复用。可在 Path/Query/Header/Body 中直接使用 {{ inlineExample }}。</p>
      </div>
      <el-button type="primary" :disabled="!canCreate" @click="openCreate">新增参数源</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理 SQL 参数源。"
    />



    <div class="toolbar sql-source-toolbar">
      <el-select
        v-model="serviceId"
        class="filter-select"
        :placeholder="projectId ? '选择服务' : '请先选择项目'"
        :disabled="!projectId"
        :no-data-text="projectId ? '当前项目暂无服务' : '请先选择项目'"
        filterable
        @change="onServiceChange"
      >
        <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
      </el-select>
      <el-button :disabled="!serviceId" :loading="loading" @click="loadPageData">刷新</el-button>
    </div>

    <el-table :data="sources" border row-key="id" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="170" />
      <el-table-column label="Key" min-width="150" show-overflow-tooltip>
        <template #default="{ row }">{{ sourceKey(row) }}</template>
      </el-table-column>
      <el-table-column label="数据源" min-width="180">
        <template #default="{ row }">{{ dataSourceLabel(row.dataSourceId) }}</template>
      </el-table-column>
      <el-table-column label="SQL" min-width="280" show-overflow-tooltip>
        <template #default="{ row }">{{ compactSQL(row.sql) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="230" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link :loading="previewingId === row.id" @click="openPreview(row)">预览</el-button>
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !sources.length" description="暂无 SQL 参数源，新增后可在请求模板中内联引用" />

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑 SQL 参数源' : '新增 SQL 参数源'" width="920px" align-center>
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称"><el-input v-model="form.name" placeholder="例如：查询可用用户 ID" /></el-form-item>
        <el-form-item label="Key">
          <el-input v-model="form.key" :placeholder="'例如：userSeed，用于 {{$sql.userSeed.id}}'">
            <template #append>{{ inlineExampleForForm }}</template>
          </el-input>
          <div class="field-tip">仅支持字母、数字、下划线和中划线；同一项目服务下唯一。为空时后端会按名称生成，中文名称建议手动填写。</div>
        </el-form-item>
        <el-form-item label="业务数据源">
          <el-select v-model="form.dataSourceId" class="full-width" filterable placeholder="选择业务数据源">
            <el-option v-for="item in dataSources" :key="item.id" :label="dataSourceOptionLabel(item)" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="SQL">
          <SqlEditor
            v-model="form.sql"
            :schema-tables="schemaTables"
            :min-rows="8"
            :max-rows="22"
            placeholder="select id, token from users where status = $1 limit 1"
          />
        </el-form-item>
        <el-form-item label="入参 JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span
                >声明 SQL 位置参数（与 <code class="inline-code">$1</code>… 顺序一致）。未写 <code class="inline-code">value</code> 时从合并变量按 <code class="inline-code">name</code> 取值；<code class="inline-code">value</code> 中可使用
                <code v-pre class="inline-code">{{变量名}}</code> 动态拼接。</span
              >
            </div>
            <el-input v-model="form.inputParams" class="json-editor" type="textarea" :autosize="{ minRows: 5, maxRows: 14 }" @blur="formatInputParams" />
          </div>
        </el-form-item>
        <el-form-item label="测试 SQL">
          <div class="json-field">
            <div class="json-toolbar">
              <span>使用当前表单连接所选数据源执行只读 SQL，无需先保存。下方为本次预览传入的变量（对应入参 JSON 中的占位符）。</span>
            </div>
            <el-input
              v-model="draftPreviewVariables"
              class="json-editor"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 10 }"
              placeholder='例如：{ "status": "active" }'
              @blur="formatDraftPreviewVars"
            />
            <div class="draft-preview-actions">
              <el-button type="primary" plain :loading="previewDraftLoading" :disabled="!form.dataSourceId" @click="runDraftPreview">
                执行测试
              </el-button>
            </div>
            <pre class="preview-result draft-preview-result">{{ draftPreviewResultText }}</pre>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewDialog" title="预览 SQL 参数源" width="860px" align-center>
      <el-form label-width="100px">
        <el-form-item label="参数源">
          <el-input :model-value="previewSource?.name || ''" disabled />
        </el-form-item>
        <el-form-item label="入参 JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>填写本次预览可用变量，不会修改参数源配置。</span>
            </div>
            <el-input v-model="previewInput" class="json-editor" type="textarea" :autosize="{ minRows: 5, maxRows: 14 }" @blur="formatPreviewInput" />
          </div>
        </el-form-item>
        <el-form-item label="预览结果">
          <pre class="preview-result">{{ previewResultText }}</pre>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="previewDialog = false">关闭</el-button>
        <el-button type="primary" :loading="previewingId === previewSource?.id" @click="runPreview">执行预览</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createSQLParameterSource,
  deleteSQLParameterSource,
  getDataSourceSchema,
  listDataSources,
  listServices,
  listSQLParameterSources,
  previewSQLParameterSource,
  previewSQLParameterSourceDraft,
  updateSQLParameterSource
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { persistServiceId, readStoredServiceId } from '../../utils/serviceSelection'
import SqlEditor from '../../components/SqlEditor.vue'

export default {
  name: 'SQLParameterSourceList',
  components: { SqlEditor },
  data() {
    return {
      services: [],
      dataSources: [],
      sources: [],
      serviceId: '',
      loading: false,
      saving: false,
      deletingId: '',
      previewingId: '',
      dialogVisible: false,
      editing: null,
      form: this.emptyForm(),
      previewDialog: false,
      previewSource: null,
      previewInput: '{}',
      previewResult: null,
      draftPreviewVariables: '{}',
      draftPreviewResult: null,
      previewDraftLoading: false,
      schemaTables: [],
      schemaLoading: false
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.loadServices()
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    inlineExample() {
      return '{{$sql.userSeed[status=1].id}}'
    },
    inlineExampleForForm() {
      const key = this.form.key || 'userSeed'
      return `{{$sql.${key}.id}}`
    },
    canCreate() {
      return !!this.projectId && !!this.serviceId && !!this.dataSources.length
    },
    previewResultText() {
      return this.previewResult == null ? '尚未执行预览' : JSON.stringify(this.previewResult, null, 2)
    },
    draftPreviewResultText() {
      return this.draftPreviewResult == null ? '尚未执行测试' : JSON.stringify(this.draftPreviewResult, null, 2)
    }
  },
  watch: {
    projectId() {
      this.loadServices()
    },
    'form.dataSourceId'(id) {
      this.loadSchema(id)
    }
  },
  methods: {
    async loadSchema(dataSourceId) {
      this.schemaTables = []
      if (!dataSourceId) return
      this.schemaLoading = true
      try {
        this.schemaTables = await getDataSourceSchema(dataSourceId)
      } catch {
        // schema 加载失败不影响使用，只是没有表名/列名提示
      } finally {
        this.schemaLoading = false
      }
    },
    emptyForm() {
      return {
        key: '',
        name: '',
        dataSourceId: '',
        sql: '',
        inputParams: '[\n  {\n    "name": "status",\n    "required": false\n  }\n]'
      }
    },
    formatJSON(value) {
      if (!value) return '{}'
      if (typeof value === 'string') {
        try {
          return JSON.stringify(JSON.parse(value), null, 2)
        } catch {
          return value
        }
      }
      return JSON.stringify(value, null, 2)
    },
    compactSQL(sql) {
      return String(sql || '').replace(/\s+/g, ' ').trim()
    },
    sourceKey(row) {
      return row?.key || row?.id || '-'
    },
    dataSourceOptionLabel(item) {
      return `${item.name} / ${item.driver || 'postgres'}`
    },
    dataSourceLabel(id) {
      const item = this.dataSources.find((source) => source.id === id)
      return item ? this.dataSourceOptionLabel(item) : '-'
    },
    async loadServices() {
      this.serviceId = ''
      this.services = []
      this.dataSources = []
      this.sources = []
      if (!this.projectId) return

      this.services = await listServices(this.projectId)
      const storedId = readStoredServiceId(this.projectId)
      this.serviceId = this.services.some((item) => item.id === storedId) ? storedId : this.services[0]?.id || ''
      if (this.serviceId) persistServiceId(this.projectId, this.serviceId)
      await this.loadPageData()
    },
    async onServiceChange() {
      if (this.serviceId) persistServiceId(this.projectId, this.serviceId)
      await this.loadPageData()
    },
    async loadPageData() {
      this.dataSources = []
      this.sources = []
      if (!this.projectId || !this.serviceId) return

      this.loading = true
      try {
        const [dataSources, sources] = await Promise.all([
          listDataSources({ projectId: this.projectId }),
          listSQLParameterSources({ projectId: this.projectId, serviceId: this.serviceId })
        ])
        this.dataSources = dataSources
        this.sources = sources
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      if (!this.canCreate) {
        this.$message.warning('请先选择项目与服务，并在「业务数据源」中为当前项目配置至少一个数据源')
        return
      }
      this.editing = null
      this.form = { ...this.emptyForm(), dataSourceId: this.dataSources[0]?.id || '' }
      this.resetDraftPreview()
      this.loadSchema(this.form.dataSourceId)
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editing = row
      this.form = {
        key: row.key || '',
        name: row.name || '',
        dataSourceId: row.dataSourceId || '',
        sql: row.sql || '',
        inputParams: this.formatJSON(row.inputParams)
      }
      this.resetDraftPreview()
      this.loadSchema(this.form.dataSourceId)
      this.dialogVisible = true
    },
    resetDraftPreview() {
      this.draftPreviewVariables = '{}'
      this.draftPreviewResult = null
      this.previewDraftLoading = false
    },
    parseFormJSON() {
      try {
        return {
          inputParams: JSON.parse(this.form.inputParams || '[]')
        }
      } catch (error) {
        this.$message.error(`JSON 不合法：${error.message}`)
        return null
      }
    },
    formatInputParams() {
      try {
        this.form.inputParams = JSON.stringify(JSON.parse(this.form.inputParams || '[]'), null, 2)
      } catch (error) {
        this.$message.error(`入参 JSON 不合法：${error.message}`)
      }
    },
    formatDraftPreviewVars() {
      try {
        this.draftPreviewVariables = JSON.stringify(JSON.parse(this.draftPreviewVariables || '{}'), null, 2)
      } catch (error) {
        this.$message.error(`预览变量 JSON 不合法：${error.message}`)
      }
    },
    async runDraftPreview() {
      if (!this.projectId || !this.serviceId) {
        this.$message.warning('缺少项目或服务上下文')
        return
      }
      if (!this.form.dataSourceId) {
        this.$message.warning('请先选择业务数据源')
        return
      }
      const parsed = this.parseFormJSON()
      if (!parsed) return
      let variables
      try {
        variables = JSON.parse(this.draftPreviewVariables || '{}')
      } catch (error) {
        this.$message.error(`预览变量 JSON 不合法：${error.message}`)
        return
      }
      if (variables == null || typeof variables !== 'object' || Array.isArray(variables)) {
        this.$message.error('预览变量须为 JSON 对象（键值对），不能为数组')
        return
      }
      this.previewDraftLoading = true
      try {
        this.draftPreviewResult = await previewSQLParameterSourceDraft({
          projectId: this.projectId,
          serviceId: this.serviceId,
          dataSourceId: this.form.dataSourceId,
          key: this.form.key,
          name: this.form.name,
          sql: this.form.sql,
          inputParams: parsed.inputParams,
          variables
        })
      } finally {
        this.previewDraftLoading = false
      }
    },
    async submit() {
      const parsed = this.parseFormJSON()
      if (!parsed) return
      const payload = {
        projectId: this.projectId,
        serviceId: this.serviceId,
        dataSourceId: this.form.dataSourceId,
        key: this.form.key,
        name: this.form.name,
        sql: this.form.sql,
        inputParams: parsed.inputParams
      }
      this.saving = true
      try {
        if (this.editing) {
          await updateSQLParameterSource(this.editing.id, payload)
        } else {
          await createSQLParameterSource(payload)
        }
        this.$message.success('SQL 参数源已保存')
        this.dialogVisible = false
        this.editing = null
        await this.loadPageData()
      } finally {
        this.saving = false
      }
    },
    openPreview(row) {
      this.previewSource = row
      this.previewInput = '{}'
      this.previewResult = null
      this.previewDialog = true
    },
    formatPreviewInput() {
      try {
        this.previewInput = JSON.stringify(JSON.parse(this.previewInput || '{}'), null, 2)
      } catch (error) {
        this.$message.error(`入参 JSON 不合法：${error.message}`)
      }
    },
    async runPreview() {
      if (!this.previewSource) return
      let inputParams
      try {
        inputParams = JSON.parse(this.previewInput || '{}')
      } catch (error) {
        this.$message.error(`入参 JSON 不合法：${error.message}`)
        return
      }
      this.previewingId = this.previewSource.id
      try {
        this.previewResult = await previewSQLParameterSource(this.previewSource.id, { variables: inputParams })
      } finally {
        this.previewingId = ''
      }
    },
    async remove(row) {
      await this.$confirm(`确认删除 SQL 参数源「${row.name}」？使用其内联模板的接口请求将无法继续解析。`, '提示')
      this.deletingId = row.id
      try {
        await deleteSQLParameterSource(row.id)
        this.$message.success('SQL 参数源已删除')
        await this.loadPageData()
      } finally {
        this.deletingId = ''
      }
    }
  }
}
</script>

<style scoped>
.sql-source-header {
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
}

.page-subtitle {
  margin: 6px 0 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.project-hint {
  margin-bottom: 16px;
}

.sql-usage-alert {
  margin-bottom: 16px;
}

.sql-usage-body {
  margin: 0;
  line-height: 1.55;
  font-size: var(--app-font-size-small);
  color: var(--app-text-color);
}

.sql-usage-lead {
  margin: 0 0 8px;
}

.sql-usage-lead:last-of-type {
  margin-bottom: 6px;
}

.sql-usage-label {
  font-weight: 600;
}

.sql-usage-list {
  margin: 0 0 10px;
  padding-left: 1.25em;
}

.sql-usage-list li {
  margin-bottom: 6px;
}

.sql-usage-foot {
  margin: 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.sql-usage-code,
.inline-code {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--app-code-bg);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.92em;
}

.sql-source-toolbar {
  align-items: stretch;
}

.filter-select {
  flex: 1 1 220px;
  max-width: 320px;
  min-width: 180px;
}

.full-width,
.json-field {
  width: 100%;
}

.json-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.field-tip {
  margin-top: 6px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.json-editor :deep(textarea),
.sql-editor :deep(textarea),
.preview-result {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

.preview-result {
  width: 100%;
  min-height: 160px;
  max-height: 360px;
  overflow: auto;
  margin: 0;
  padding: 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 6px;
  background: var(--app-code-bg);
  white-space: pre-wrap;
}

.draft-preview-actions {
  margin: 10px 0;
}

.draft-preview-result {
  min-height: 120px;
  max-height: 280px;
}

@media (max-width: 760px) {
  .filter-select {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
