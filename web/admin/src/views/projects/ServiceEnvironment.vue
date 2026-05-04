<template>
  <div class="page-card">
    <div class="page-header service-env-header">
      <h2 class="page-title">服务与环境管理</h2>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理服务与环境。"
    />

    <el-row :gutter="16" class="service-env-layout">
      <el-col :span="10">
        <div class="section-title">
          <span>服务</span>
          <el-tooltip content="请先选择项目" :disabled="!!projectId" placement="top">
            <span class="tooltip-btn-wrap">
              <el-button type="primary" size="small" :disabled="!projectId" @click="openCreateService">新增服务</el-button>
            </span>
          </el-tooltip>
        </div>
        <el-empty v-if="!services.length" :description="emptyServiceDescription">
          <el-tooltip content="请先选择项目" :disabled="!!projectId" placement="top">
            <span class="tooltip-btn-wrap">
              <el-button type="primary" :disabled="!projectId" @click="openCreateService">新增服务</el-button>
            </span>
          </el-tooltip>
        </el-empty>
        <el-table
          v-else
          ref="serviceTable"
          :data="services"
          border
          highlight-current-row
          class="service-table"
          @current-change="onServiceCurrentChange"
        >
          <el-table-column prop="name" label="名称" min-width="100" />
          <el-table-column prop="description" label="描述" min-width="80" show-overflow-tooltip />
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button type="text" @click.stop="openEditService(row)">编辑</el-button>
              <el-button type="text" class="danger" :loading="deletingServiceId === row.id" @click.stop="removeService(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-col>

      <el-col :span="14">
        <div class="section-title env-panel-title">
          <div class="env-panel-heading">
            <span class="env-panel-label">环境配置</span>
            <el-select
              v-model="selectedServiceId"
              class="service-switcher"
              filterable
              clearable
              placeholder="选择服务"
              :disabled="!services.length"
            >
              <el-option
                v-for="s in services"
                :key="s.id"
                :label="serviceOptionLabel(s)"
                :value="s.id"
              />
            </el-select>
          </div>
          <el-button type="primary" size="small" :disabled="!selectedService" @click="openCreateEnvironment">新增环境</el-button>
        </div>
        <div v-if="!selectedService" class="env-panel-placeholder">
          <el-empty :description="envPlaceholderDescription" />
        </div>
        <template v-else>
          <el-table :data="environments" border v-loading="loadingEnvironments" class="env-table">
            <el-table-column prop="name" label="名称" width="140" />
            <el-table-column prop="baseUrl" label="Base URL" min-width="220" show-overflow-tooltip />
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <el-button type="text" @click="openEditEnvironment(row)">编辑</el-button>
                <el-button type="text" class="danger" :loading="deletingEnvironmentId === row.id" @click="removeEnvironment(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!environments.length && !loadingEnvironments" description="当前服务暂无环境，点击「新增环境」添加" />
        </template>
      </el-col>
    </el-row>

    <el-dialog v-model="serviceDialog" :title="editingService ? '编辑服务' : '新增服务'" width="420px">
      <el-form :model="serviceForm" label-width="90px">
        <el-form-item label="名称"><el-input v-model="serviceForm.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="serviceForm.description" type="textarea" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="serviceDialog = false">取消</el-button>
        <el-button type="primary" @click="submitService">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="envDialog" class="env-config-dialog" :title="editingEnvironment ? '编辑环境' : '新增环境'" width="860px" align-center>
      <el-form :model="envForm" label-width="90px">
        <el-form-item label="名称"><el-input v-model="envForm.name" placeholder="例如：测试环境、预发环境" /></el-form-item>
        <el-form-item label="Base URL"><el-input v-model="envForm.baseUrl" placeholder="http://127.0.0.1:8081" /></el-form-item>
        <el-form-item class="json-form-item" label-width="0">
          <div class="json-field">
            <div class="json-field-label-row">
              <div class="json-field-label">
                <span>变量 JSON</span>
                <el-tooltip placement="top-start" effect="dark" popper-class="json-help-tooltip">
                  <template #content>
                    <div v-pre class="json-help-content">
                      <p class="json-help-lead">填写当前服务在该环境下的键值变量。</p>
                      <p class="json-help-caption">示例</p>
                      <pre class="json-help-code">{
  "token": "env-token",
  "page": 1
}</pre>
                      <p class="json-help-hint">可在请求 URL、路径、请求头和查询参数中用 {{变量名}} 引用。</p>
                      <p class="json-help-hint">无需变量时填 <code>{}</code>。</p>
                    </div>
                  </template>
                  <span class="field-help-icon" aria-label="变量 JSON 填写说明">?</span>
                </el-tooltip>
              </div>
              <el-button size="small" @click="formatEnvironmentVariablesJson">格式化</el-button>
            </div>
            <el-input
              v-model="envForm.variables"
              class="json-editor"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"token":"env-token","page":1}'
            />
          </div>
        </el-form-item>
        <el-form-item class="json-form-item" label-width="0">
          <div class="json-field">
            <div class="json-field-label-row">
              <div class="json-field-label">
                <span>认证 JSON</span>
                <el-tooltip placement="top-start" effect="dark" popper-class="json-help-tooltip">
                  <template #content>
                    <div v-pre class="json-help-content">
                      <p class="json-help-lead">填写当前服务在该环境下的认证配置，可按 OpenAPI/Swagger security 选择不同参数或 token。</p>
                      <p class="json-help-hint">无需认证时填 <code>{}</code>。单一认证示例：</p>
                      <pre class="json-help-code">{
  "type": "bearer",
  "token": "{{token}}"
}</pre>
                      <p class="json-help-caption">多 security 示例</p>
                      <pre class="json-help-code json-help-code--scroll">{
  "defaultProfile": "user",
  "profiles": {
    "user": {
      "type": "bearer",
      "token": "{{userToken}}"
    },
    "admin": {
      "type": "api_key",
      "in": "query",
      "name": "admin_token",
      "value": "{{adminToken}}"
    }
  },
  "securitySchemes": {
    "UserAuth": "user",
    "AdminAuth": "admin"
  }
}</pre>
                    </div>
                  </template>
                  <span class="field-help-icon" aria-label="认证 JSON 填写说明">?</span>
                </el-tooltip>
              </div>
              <el-button size="small" @click="formatEnvironmentAuthJson">格式化</el-button>
            </div>
            <el-input
              v-model="envForm.auth"
              class="json-editor"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"defaultProfile":"user","profiles":{"user":{"type":"bearer","token":"{{userToken}}"},"admin":{"type":"api_key","in":"query","name":"admin_token","value":"{{adminToken}}"}},"securitySchemes":{"UserAuth":"user","AdminAuth":"admin"}}'
            />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="envDialog = false">取消</el-button>
        <el-button type="primary" @click="submitEnvironment">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createService,
  createServiceEnvironment,
  deleteService,
  deleteServiceEnvironment,
  listServiceEnvironments,
  listServices,
  updateService,
  updateServiceEnvironment
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { persistServiceId, readStoredServiceId } from '../../utils/serviceSelection'

export default {
  name: 'ServiceEnvironment',
  data() {
    return {
      services: [],
      environments: [],
      selectedService: null,
      serviceDialog: false,
      envDialog: false,
      editingService: null,
      editingEnvironment: null,
      deletingServiceId: '',
      deletingEnvironmentId: '',
      loadingEnvironments: false,
      serviceForm: this.emptyServiceForm(),
      envForm: this.emptyEnvironmentForm()
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
    selectedServiceId: {
      get() {
        return this.selectedService?.id ?? ''
      },
      set(id) {
        if (!id) {
          this.selectedService = null
          this.environments = []
          persistServiceId(this.projectId, '')
          this.$nextTick(() => this.syncServiceTableCurrentRow())
          return
        }
        const row = this.services.find((s) => s.id === id)
        if (row) {
          this.applySelectedService(row)
        }
      }
    },
    emptyServiceDescription() {
      return this.projectId ? '暂无服务，请先新增服务' : '请先在顶部选择项目后再新增服务'
    },
    envPlaceholderDescription() {
      if (!this.projectId) return '请先在顶部选择项目'
      return '暂无可用服务，请在左侧新增服务'
    }
  },
  watch: {
    projectId() {
      this.loadServices()
    }
  },
  methods: {
    emptyServiceForm() {
      return { name: '', description: '' }
    },
    emptyEnvironmentForm() {
      return { name: '', baseUrl: '', variables: '{}', auth: '{}' }
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
    serviceOptionLabel(s) {
      return s.name
    },
    syncServiceTableCurrentRow() {
      const table = this.$refs.serviceTable
      if (!table) return
      if (this.selectedService) {
        const row = this.services.find((svc) => svc.id === this.selectedService.id)
        if (row) {
          table.setCurrentRow(row)
          return
        }
      }
      table.setCurrentRow(null)
    },
    applySelectedService(row) {
      if (!row) return
      const changed = this.selectedService?.id !== row.id
      this.selectedService = row
      persistServiceId(this.projectId, row.id)
      if (changed) {
        this.loadEnvironments()
      }
      this.$nextTick(() => this.syncServiceTableCurrentRow())
    },
    onServiceCurrentChange(row) {
      if (!row) return
      if (this.selectedService?.id === row.id) return
      this.applySelectedService(row)
    },
    async loadServices() {
      if (!this.projectId) {
        this.services = []
        this.environments = []
        this.selectedService = null
        return
      }
      this.services = await listServices(this.projectId)
      const storedId = readStoredServiceId(this.projectId)
      const preferred =
        this.services.find((service) => service.id === storedId) ||
        this.services.find((service) => service.id === this.selectedService?.id) ||
        this.services[0] ||
        null
      this.selectedService = preferred
      if (this.selectedService) {
        persistServiceId(this.projectId, this.selectedService.id)
      } else {
        persistServiceId(this.projectId, '')
      }
      await this.loadEnvironments()
      await this.$nextTick()
      this.syncServiceTableCurrentRow()
    },
    async loadEnvironments() {
      if (!this.projectId || !this.selectedService) {
        this.environments = []
        return
      }
      this.loadingEnvironments = true
      try {
        this.environments = await listServiceEnvironments(this.projectId, this.selectedService.id)
      } finally {
        this.loadingEnvironments = false
      }
    },
    openCreateService() {
      if (!this.projectId) {
        this.$message.warning('请先选择项目')
        return
      }
      this.editingService = null
      this.serviceForm = this.emptyServiceForm()
      this.serviceDialog = true
    },
    openEditService(row) {
      this.editingService = row
      this.serviceForm = {
        name: row.name,
        description: row.description || ''
      }
      this.serviceDialog = true
    },
    async submitService() {
      if (this.editingService) {
        await updateService(this.projectId, this.editingService.id, this.serviceForm)
      } else {
        await createService(this.projectId, this.serviceForm)
      }
      this.$message.success('服务已保存')
      this.serviceDialog = false
      this.editingService = null
      this.serviceForm = this.emptyServiceForm()
      await this.loadServices()
    },
    async removeService(row) {
      await this.$confirm(`确认删除服务 ${row.name}？服务关联的环境、接口、用例、套件和运行记录将被软删除。`, '提示')
      this.deletingServiceId = row.id
      try {
        await deleteService(this.projectId, row.id)
        this.$message.success('服务已删除')
        await this.loadServices()
      } finally {
        this.deletingServiceId = ''
      }
    },
    openCreateEnvironment() {
      this.editingEnvironment = null
      this.envForm = this.emptyEnvironmentForm()
      this.envDialog = true
    },
    openEditEnvironment(row) {
      this.editingEnvironment = row
      this.envForm = {
        name: row.name,
        baseUrl: row.baseUrl,
        variables: this.formatJSON(row.variables),
        auth: this.formatJSON(row.auth)
      }
      this.envDialog = true
    },
    formatEnvironmentVariablesJson() {
      try {
        const parsed = JSON.parse(this.envForm.variables || '{}')
        this.envForm.variables = JSON.stringify(parsed, null, 2)
      } catch (error) {
        this.$message.error(`变量 JSON 不是合法 JSON：${error.message}`)
      }
    },
    formatEnvironmentAuthJson() {
      try {
        const parsed = JSON.parse(this.envForm.auth || '{}')
        this.envForm.auth = JSON.stringify(parsed, null, 2)
      } catch (error) {
        this.$message.error(`认证 JSON 不是合法 JSON：${error.message}`)
      }
    },
    async submitEnvironment() {
      if (!this.selectedService) return
      let variables
      let auth
      try {
        variables = JSON.parse(this.envForm.variables || '{}')
        auth = JSON.parse(this.envForm.auth || '{}')
      } catch (error) {
        this.$message.error(`环境 JSON 不合法：${error.message}`)
        return
      }
      const payload = {
        name: this.envForm.name,
        baseUrl: this.envForm.baseUrl,
        variables,
        auth
      }
      if (this.editingEnvironment) {
        await updateServiceEnvironment(this.projectId, this.selectedService.id, this.editingEnvironment.id, payload)
      } else {
        await createServiceEnvironment(this.projectId, this.selectedService.id, payload)
      }
      this.$message.success('环境已保存')
      this.envDialog = false
      this.editingEnvironment = null
      this.envForm = this.emptyEnvironmentForm()
      await this.loadEnvironments()
    },
    async removeEnvironment(row) {
      await this.$confirm(`确认删除环境 ${row.name}？`, '提示')
      this.deletingEnvironmentId = row.id
      try {
        await deleteServiceEnvironment(this.projectId, this.selectedService.id, row.id)
        this.$message.success('环境已删除')
        await this.loadEnvironments()
      } finally {
        this.deletingEnvironmentId = ''
      }
    }
  }
}
</script>

<style scoped>
.project-hint {
  margin-bottom: 16px;
}

.tooltip-btn-wrap {
  display: inline-block;
}

.service-env-layout {
  align-items: flex-start;
}

.service-env-header {
  gap: 12px;
  flex-wrap: wrap;
}

.env-panel-title {
  flex-wrap: wrap;
  gap: 8px;
}

.env-panel-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.env-panel-label {
  flex-shrink: 0;
}

.service-switcher {
  min-width: 160px;
  max-width: 100%;
  flex: 1;
}

.service-table {
  width: 100%;
}

.env-table {
  width: 100%;
}

.env-panel-placeholder {
  min-height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.service-env-header .page-title {
  flex: 0 1 auto;
  min-width: 0;
}

.section-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: 600;
}

.danger {
  color: #f56c6c;
}

.json-form-item {
  margin-bottom: 18px;
}

.json-form-item :deep(.el-form-item__content) {
  margin-left: 0 !important;
}

.env-config-dialog :deep(.el-dialog__body) {
  max-height: min(78vh, 920px);
  overflow-y: auto;
  padding-top: 8px;
}

.json-field {
  width: 100%;
}

.json-field-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.json-field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 0;
  color: var(--el-text-color-regular);
  font-weight: 500;
  line-height: 1;
}

.json-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

.field-help-icon {
  display: inline-flex;
  width: 16px;
  height: 16px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--el-border-color);
  border-radius: 50%;
  color: var(--el-text-color-secondary);
  cursor: help;
  font-size: var(--app-font-size-small);
  line-height: 1;
}

:global(.json-help-tooltip) {
  max-width: min(520px, 92vw);
}

:global(.json-help-content) {
  line-height: 1.55;
  font-size: 12px;
}

:global(.json-help-content p) {
  margin: 0;
}

:global(.json-help-lead) {
  margin-bottom: 8px !important;
  color: rgba(255, 255, 255, 0.95);
}

:global(.json-help-caption) {
  margin: 10px 0 6px !important;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.92);
}

:global(.json-help-hint) {
  margin-top: 8px !important;
  color: rgba(255, 255, 255, 0.88);
}

:global(.json-help-hint code) {
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.14);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
}

:global(.json-help-code) {
  margin: 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.28);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.45;
  white-space: pre;
  overflow-x: auto;
  max-width: 100%;
  color: #e8f0ff;
}

:global(.json-help-code--scroll) {
  max-height: 220px;
  overflow-y: auto;
}
</style>
