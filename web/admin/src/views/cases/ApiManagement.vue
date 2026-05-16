<template>
  <div class="api-management">
    <div class="page-header api-page-header">
      <div>
        <h2 class="page-title">API管理</h2>
        <p class="page-subtitle">统一导入 OpenAPI/Swagger 文档并维护当前服务下的接口请求模板。</p>
      </div>
      <el-button type="primary" :disabled="!canCreateCase" @click="openCreateDialog">新增接口</el-button>
    </div>

    <div class="page-card api-service-card">
      <div class="toolbar api-toolbar">
        <el-select
          v-model="serviceId"
          class="api-service-select"
          :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
          :title="projectId ? '选择当前项目下的服务' : '请先在顶部选择项目后再选择服务'"
          :disabled="!projectId"
          :no-data-text="projectId ? '当前项目暂无服务' : '请先在顶部选择项目'"
          filterable
          @change="handleServiceChange"
        >
          <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
        </el-select>
        <span class="api-service-hint">{{ serviceHint }}</span>
      </div>

      <el-alert
        v-if="!projectId"
        type="info"
        show-icon
        :closable="false"
        title="请先在顶部选择项目后再管理 API。"
      />
      <el-alert
        v-else-if="!services.length"
        type="info"
        show-icon
        :closable="false"
        title="当前项目暂无服务，请先到项目管理中新增服务。"
      />
    </div>

    <div class="api-layout">
      <div class="page-card import-card">
        <div class="section-header">
          <div>
            <h3 class="section-title">OpenAPI/Swagger 导入</h3>
            <p class="section-subtitle">选择或粘贴文档内容，导入后自动刷新接口列表。</p>
          </div>
          <el-tag v-if="!canImportSpec" type="warning" effect="plain">无导入权限</el-tag>
        </div>

        <div class="toolbar import-toolbar">
          <div class="spec-file-input">
            <el-button :disabled="!canImportSpec" @click="$refs.fileInput.click()">
              <el-icon><Upload /></el-icon>
              <span>选择文件</span>
            </el-button>
            <span class="spec-file-name">{{ fileName || '支持 .json、.yaml、.yml 格式' }}</span>
            <input ref="fileInput" type="file" hidden accept=".json,.yaml,.yml" @change="readFile" />
          </div>
        </div>

        <el-input
          v-model="content"
          class="json-editor"
          type="textarea"
          :rows="12"
          :disabled="!canImportSpec"
          placeholder="粘贴 OpenAPI/Swagger JSON 或 YAML 内容"
        />
        <div class="actions">
          <el-button type="primary" :disabled="!canSubmitImport" :loading="importing" @click="submitImport">
            导入并生成接口
          </el-button>
        </div>

        <el-alert v-if="summary" type="success" show-icon :closable="false" class="summary">
          <template #title>
            导入成功：文档中共 {{ summary.apiCount }} 个接口，端点新增 {{ summary.createdEndpoints }}、更新
            {{ summary.updatedEndpoints }}，写入请求模板 {{ summary.generatedCases }} 条（spec v{{ summary.specVersion }}）
          </template>
        </el-alert>

        <el-table :data="specs" border size="small" empty-text="暂无导入记录">
          <el-table-column prop="version" label="版本" width="80" />
          <el-table-column prop="contentHash" label="内容 Hash" min-width="220" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column prop="createdAt" label="导入时间" width="170" :formatter="formatDateTimeColumn" />
          <el-table-column label="操作" width="140">
            <template #default="{ row }">
              <el-tooltip
                content="基于本次 spec 与上一版本的结构化 diff 调用 AI 分析变更影响"
                placement="top"
              >
                <el-button
                  size="small"
                  type="warning"
                  plain
                  :loading="aiAnalysisLoading && analyzingSpecId === row.id"
                  :disabled="aiAnalysisLoading"
                  @click="openSpecChangesAnalysis(row)"
                >
                  <AiSparkleIcon class="ai-spec-changes-icon" />
                  <span>AI 分析</span>
                </el-button>
              </el-tooltip>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="page-card case-card">
        <div class="section-header">
          <div>
            <h3 class="section-title">接口列表</h3>
            <p class="section-subtitle">展示导入自动生成和手工新增的可运行请求模板。</p>
          </div>
          <el-button type="primary" plain :disabled="!canCreateCase" @click="openCreateDialog">新增接口</el-button>
        </div>

        <el-table :data="cases" border row-key="id" empty-text="暂无接口">
          <el-table-column label="接口名称" min-width="180">
            <template #default="{ row }">{{ displayCaseName(row) }}</template>
          </el-table-column>
          <el-table-column label="方法" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="methodType(row.method)">{{ row.method }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="path" label="路径" min-width="220" show-overflow-tooltip />
          <el-table-column prop="source" label="来源" width="100" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column label="操作" width="110">
            <template #default="{ row }">
              <el-button type="primary" link @click="openRun(row)">运行</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" title="新增手工接口" width="640px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="方法">
          <el-select v-model="form.method">
            <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="路径"><el-input v-model="form.path" placeholder="/api/users" /></el-form-item>
        <el-form-item label="请求 JSON"><el-input v-model="form.request" class="json-editor" type="textarea" :rows="5" /></el-form-item>
        <el-form-item label="断言 JSON"><el-input v-model="form.assertions" class="json-editor" type="textarea" :rows="5" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCase">保存</el-button>
      </template>
    </el-dialog>

    <AIAnalysisDialog
      v-model="aiAnalysisDialogVisible"
      title="AI 分析 Spec 变更影响"
      :loading="aiAnalysisLoading"
      :text="aiAnalysisText"
      :model="aiAnalysisModel"
      :elapsed-millis="aiAnalysisElapsed"
      :error="aiAnalysisError"
      :info="aiAnalysisInfo"
      @retry="rerunSpecChangesAnalysis"
    />
  </div>
</template>

<script>
import { Upload } from '@element-plus/icons-vue'
import {
  analyzeSpecChanges,
  createCase,
  importSpec,
  listCases,
  listEndpoints,
  listServices,
  listSpecs
} from '../../api'
import { hasPermission } from '../../auth'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { formatDateTime } from '../../utils/datetime'
import { persistServiceId, readStoredServiceId } from '../../utils/serviceSelection'
import { enrichPage } from '../../stores/aiAssistant'
import AIAnalysisDialog from '../../components/AIAnalysisDialog.vue'
import AiSparkleIcon from '../../components/icons/AiSparkleIcon.vue'

export default {
  name: 'ApiManagement',
  components: { Upload, AIAnalysisDialog, AiSparkleIcon },
  data() {
    return {
      services: [],
      serviceId: '',
      specs: [],
      cases: [],
      endpoints: [],
      content: '',
      summary: null,
      importing: false,
      fileName: '',
      dialogVisible: false,
      methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
      form: this.emptyCaseForm(),
      aiAnalysisDialogVisible: false,
      aiAnalysisLoading: false,
      aiAnalysisText: '',
      aiAnalysisModel: '',
      aiAnalysisElapsed: 0,
      aiAnalysisError: '',
      aiAnalysisInfo: '',
      analyzingSpecId: ''
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
    canImportSpec() {
      return hasPermission('specs:import')
    },
    canCreateCase() {
      return !!this.projectId && !!this.serviceId && hasPermission('cases:write')
    },
    canSubmitImport() {
      return !!this.projectId && !!this.serviceId && !!this.content && this.canImportSpec
    },
    serviceHint() {
      if (!this.projectId) return 'API 管理复用顶部全局项目'
      if (!this.services.length) return '当前项目暂无服务'
      return '导入、历史记录和接口列表均按当前服务过滤'
    }
  },
  watch: {
    projectId() {
      this.loadServices()
    },
    serviceId(id) {
      enrichPage({ serviceId: id || undefined })
    }
  },
  methods: {
    emptyCaseForm() {
      return {
        name: '',
        method: 'GET',
        path: '',
        request: '{}',
        assertions: '[{"type":"status_code","expected":200}]'
      }
    },
    formatDateTimeColumn(row, column, value) {
      return formatDateTime(value)
    },
    async loadServices() {
      this.serviceId = ''
      this.specs = []
      this.cases = []
      this.endpoints = []
      this.summary = null
      this.services = this.projectId ? await listServices(this.projectId) : []
      const storedId = readStoredServiceId(this.projectId)
      const preferred = this.services.find((service) => service.id === storedId) || this.services[0]
      if (preferred) {
        this.serviceId = preferred.id
        persistServiceId(this.projectId, this.serviceId)
        await this.loadServiceData()
      }
    },
    async handleServiceChange() {
      persistServiceId(this.projectId, this.serviceId)
      this.summary = null
      await this.loadServiceData()
    },
    async loadServiceData() {
      if (!this.projectId || !this.serviceId) {
        this.specs = []
        this.cases = []
        this.endpoints = []
        return
      }
      const [specs, cases, endpoints] = await Promise.all([
        listSpecs(this.projectId, this.serviceId),
        listCases({ projectId: this.projectId, serviceId: this.serviceId }),
        listEndpoints(this.projectId, this.serviceId)
      ])
      this.specs = Array.isArray(specs) ? specs : []
      this.cases = cases
      this.endpoints = Array.isArray(endpoints) ? endpoints : []
    },
    readFile(event) {
      const file = event.target.files?.[0]
      if (!file) return
      this.fileName = file.name
      const reader = new FileReader()
      reader.onload = () => {
        this.content = reader.result
      }
      reader.readAsText(file)
      event.target.value = ''
    },
    async submitImport() {
      this.importing = true
      try {
        this.summary = await importSpec(this.projectId, this.serviceId, this.content)
        this.$message.success('导入完成')
        await this.loadServiceData()
      } finally {
        this.importing = false
      }
    },
    endpointForCase(row) {
      return (
        this.endpoints.find((endpoint) => endpoint.id === row.endpointId) ||
        this.endpoints.find((endpoint) => endpoint.method === row.method && endpoint.path === row.path) ||
        null
      )
    },
    displayCaseName(row) {
      return this.endpointForCase(row)?.summary || row.name || row.path
    },
    openCreateDialog() {
      if (!hasPermission('cases:write')) {
        this.$message.warning('当前账号没有管理接口权限')
        return
      }
      if (!this.projectId || !this.serviceId) {
        this.$message.warning('请先选择项目和服务')
        return
      }
      this.dialogVisible = true
    },
    async submitCase() {
      let request
      let assertions
      try {
        request = JSON.parse(this.form.request || '{}')
        assertions = JSON.parse(this.form.assertions || '[]')
      } catch (error) {
        this.$message.error(`JSON 不合法：${error.message}`)
        return
      }
      await createCase({
        projectId: this.projectId,
        serviceId: this.serviceId,
        name: this.form.name,
        method: this.form.method,
        path: this.form.path,
        request,
        assertions
      })
      this.$message.success('接口已创建')
      this.dialogVisible = false
      this.form = this.emptyCaseForm()
      await this.loadServiceData()
    },
    openRun(row) {
      this.$router.push(`/run-console/${row.id}`)
    },
    methodType(method) {
      const value = String(method || '').toUpperCase()
      if (value === 'GET') return 'success'
      if (value === 'POST') return 'primary'
      if (value === 'DELETE') return 'danger'
      if (value === 'PUT' || value === 'PATCH') return 'warning'
      return 'info'
    },
    /**
     * 打开 Spec 变更 AI 分析弹窗：先打开弹窗以保证用户立即看到 loading 反馈，
     * 再异步调用 analyzeSpecChanges。analyzingSpecId 用于行级 loading 标识，
     * 避免多行同时显示 loading。
     */
    async openSpecChangesAnalysis(row) {
      if (!row?.id || !this.projectId || !this.serviceId) {
        this.$message.warning('请先选择项目与服务')
        return
      }
      this.analyzingSpecId = row.id
      this.aiAnalysisDialogVisible = true
      await this.runSpecChangesAnalysis(row.id)
    },
    async runSpecChangesAnalysis(specId) {
      if (!specId || !this.projectId || !this.serviceId) return
      this.aiAnalysisLoading = true
      this.aiAnalysisText = ''
      this.aiAnalysisModel = ''
      this.aiAnalysisElapsed = 0
      this.aiAnalysisError = ''
      this.aiAnalysisInfo = ''
      try {
        const resp = await analyzeSpecChanges(this.projectId, this.serviceId, specId)
        // 后端在没有上一版本可对比时返回 { message } 而非 AI 文本，前端把这种
        // 情况显示为温和的 info 提示，与真正的失败错误区分开。
        if (resp && resp.message && !resp.text) {
          this.aiAnalysisInfo = resp.message
        } else {
          this.aiAnalysisText = resp?.text || ''
          this.aiAnalysisModel = resp?.model || ''
          this.aiAnalysisElapsed = Number(resp?.elapsedMillis) || 0
          if (!this.aiAnalysisText) {
            this.aiAnalysisError = 'AI 未返回任何分析内容'
          }
        }
      } catch (error) {
        this.aiAnalysisError = error?.response?.data?.error || error?.message || 'AI 分析失败，请稍后重试'
      } finally {
        this.aiAnalysisLoading = false
      }
    },
    rerunSpecChangesAnalysis() {
      if (!this.analyzingSpecId) return
      this.runSpecChangesAnalysis(this.analyzingSpecId)
    }
  }
}
</script>

<style scoped>
.api-management {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.api-page-header {
  align-items: flex-start;
  margin-bottom: 0;
  gap: 16px;
  flex-wrap: wrap;
}

.page-subtitle,
.section-subtitle {
  margin: 6px 0 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.api-service-card {
  padding-bottom: 2px;
}

.api-toolbar {
  align-items: stretch;
}

.api-service-select {
  flex: 1 1 260px;
  max-width: 420px;
  min-width: 200px;
}

.api-service-hint {
  align-self: center;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.ai-spec-changes-icon {
  font-size: 14px;
  margin-right: 4px;
  vertical-align: -2px;
}

.api-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.section-title {
  margin: 0;
  font-size: var(--el-font-size-medium);
  font-weight: 600;
}

.import-toolbar {
  align-items: stretch;
}

.spec-file-input {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex: 1 1 240px;
  min-width: 220px;
}

.spec-file-name {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.actions {
  margin: 16px 0;
}

.summary {
  margin-bottom: 16px;
}

@media (max-width: 640px) {
  .api-service-select,
  .spec-file-input {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
