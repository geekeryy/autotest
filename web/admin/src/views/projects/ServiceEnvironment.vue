<template>
  <div :class="embedded ? 'service-env-root' : 'page-card'">
    <div v-if="!embedded" class="page-header service-env-header">
      <h2 class="page-title">服务与环境管理</h2>
      <div class="service-env-actions">
        <el-button @click="$router.push({ path: '/projects', query: { tab: 'dataSources' } })">业务数据源</el-button>
        <el-button @click="$router.push('/sql-parameter-sources')">SQL 参数源</el-button>
      </div>
    </div>
    <div v-else class="service-env-embedded-toolbar">
      <p class="page-subtitle">
        按当前全局项目维护服务及其环境（Base URL、变量与认证配置）；用于接口调试与场景运行。
      </p>
      <el-button type="primary" :disabled="!projectId" @click="openCreateService">新增服务</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在左侧列表或顶部选择项目后再管理服务与环境。"
    />

    <div v-else class="service-env-tree-wrap">
      <div v-if="!embedded" class="section-title">
        <span></span>
        <el-button type="primary" size="small" @click="openCreateService">新增服务</el-button>
      </div>

      <el-empty v-if="!services.length" :description="emptyServiceDescription" />

      <el-tree
        v-else
        ref="serviceEnvTree"
        :key="treeRenderKey"
        class="service-env-tree"
        node-key="treeKey"
        lazy
        :load="loadTreeNode"
        :props="treeProps"
        highlight-current
        :expand-on-click-node="false"
        @node-click="onTreeNodeClick"
      >
        <template #default="{ data }">
          <div class="tree-node-row">
            <div class="tree-node-main">
              <span class="tree-node-title">{{ data.name }}</span>
              <span v-if="data.kind === 'environment'" class="tree-node-meta" :title="data.baseUrl">{{ data.baseUrl }}</span>
            </div>
            <div class="tree-node-actions" @click.stop>
              <template v-if="data.kind === 'service'">
                <el-button type="primary" link size="small" @click="openCreateEnvironmentUnder(data)">新增环境</el-button>
                <el-button type="primary" link size="small" @click="openEditService(data)">编辑</el-button>
                <el-button type="danger" link size="small" :loading="deletingServiceId === data.id" @click="removeService(data)">删除</el-button>
              </template>
              <template v-else>
                <el-button type="primary" link size="small" @click="openEditEnvironment(data)">编辑</el-button>
                <el-button type="danger" link size="small" :loading="deletingEnvironmentId === data.id" @click="removeEnvironment(data)">删除</el-button>
              </template>
            </div>
          </div>
        </template>
      </el-tree>
    </div>

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
            </div>
            <el-input
              v-model="envForm.variables"
              class="json-editor"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"token":"env-token","page":1}'
              @blur="formatEnvironmentVariablesJson"
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
            </div>
            <el-input
              v-model="envForm.auth"
              class="json-editor"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"defaultProfile":"user","profiles":{"user":{"type":"bearer","token":"{{userToken}}"},"admin":{"type":"api_key","in":"query","name":"admin_token","value":"{{adminToken}}"}},"securitySchemes":{"UserAuth":"user","AdminAuth":"admin"}}'
              @blur="formatEnvironmentAuthJson"
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
  props: {
    embedded: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      services: [],
      selectedService: null,
      treeRenderKey: 0,
      treeProps: { label: 'name', isLeaf: 'isLeaf', children: 'children' },
      serviceDialog: false,
      envDialog: false,
      editingService: null,
      editingEnvironment: null,
      deletingServiceId: '',
      deletingEnvironmentId: '',
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
    emptyServiceDescription() {
      return this.projectId ? '暂无服务，请先新增服务' : '请先在顶部选择项目后再新增服务'
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
    async loadTreeNode(node, resolve) {
      if (!this.projectId) {
        resolve([])
        return
      }
      if (node.level === 0) {
        const roots = (this.services || []).map((s) => ({
          ...s,
          kind: 'service',
          treeKey: `service:${s.id}`,
          isLeaf: false
        }))
        resolve(roots)
        this.$nextTick(() => this.expandSelectedServiceInTree())
        return
      }
      const d = node.data
      if (d.kind === 'service') {
        try {
          const envs = await listServiceEnvironments(this.projectId, d.id)
          resolve(
            envs.map((e) => ({
              ...e,
              kind: 'environment',
              treeKey: `environment:${e.id}`,
              isLeaf: true,
              serviceId: e.serviceId || d.id
            }))
          )
        } catch {
          resolve([])
        }
        return
      }
      resolve([])
    },
    expandSelectedServiceInTree() {
      const tree = this.$refs.serviceEnvTree
      if (!tree || !this.selectedService) return
      const key = `service:${this.selectedService.id}`
      this.$nextTick(() => {
        const n = tree.getNode(key)
        if (n && !n.expanded) {
          n.expand()
        }
        tree.setCurrentKey(key)
      })
    },
    onTreeNodeClick(data) {
      const tree = this.$refs.serviceEnvTree
      if (data.kind === 'service') {
        this.applySelectedService(data)
        this.$nextTick(() => tree?.setCurrentKey(data.treeKey))
      } else if (data.kind === 'environment') {
        const svc = this.services.find((s) => s.id === data.serviceId)
        if (svc) {
          this.selectedService = svc
          persistServiceId(this.projectId, svc.id)
        }
        this.$nextTick(() => tree?.setCurrentKey(data.treeKey))
      }
    },
    applySelectedService(row) {
      if (!row) return
      this.selectedService = { id: row.id, name: row.name, description: row.description || '' }
      persistServiceId(this.projectId, row.id)
    },
    async loadServices() {
      if (!this.projectId) {
        this.services = []
        this.selectedService = null
        this.treeRenderKey += 1
        return
      }
      this.services = await listServices(this.projectId)
      const storedId = readStoredServiceId(this.projectId)
      const preferred =
        this.services.find((service) => service.id === storedId) ||
        this.services.find((service) => service.id === this.selectedService?.id) ||
        this.services[0] ||
        null
      if (preferred) {
        this.selectedService = { id: preferred.id, name: preferred.name, description: preferred.description || '' }
        persistServiceId(this.projectId, preferred.id)
      } else {
        this.selectedService = null
        persistServiceId(this.projectId, '')
      }
      this.treeRenderKey += 1
      await this.$nextTick()
      this.expandSelectedServiceInTree()
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
      this.editingService = { id: row.id, name: row.name, description: row.description || '' }
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
      await this.$confirm(`确认删除服务 ${row.name}？服务关联的环境、接口库、场景编排和运行记录将被软删除。`, '提示')
      this.deletingServiceId = row.id
      try {
        await deleteService(this.projectId, row.id)
        this.$message.success('服务已删除')
        await this.loadServices()
      } finally {
        this.deletingServiceId = ''
      }
    },
    openCreateEnvironmentUnder(serviceRow) {
      this.applySelectedService(serviceRow)
      this.openCreateEnvironment()
    },
    openCreateEnvironment() {
      if (!this.selectedService) {
        this.$message.warning('请先选择服务')
        return
      }
      this.editingEnvironment = null
      this.envForm = this.emptyEnvironmentForm()
      this.envDialog = true
    },
    openEditEnvironment(row) {
      const svc = this.services.find((s) => s.id === row.serviceId)
      if (svc) {
        this.applySelectedService(svc)
      }
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
        await updateServiceEnvironment(
          this.projectId,
          this.selectedService.id,
          this.editingEnvironment.id,
          payload
        )
      } else {
        await createServiceEnvironment(this.projectId, this.selectedService.id, payload)
      }
      this.$message.success('环境已保存')
      this.envDialog = false
      this.editingEnvironment = null
      this.envForm = this.emptyEnvironmentForm()
      await this.loadServices()
    },
    async removeEnvironment(row) {
      const sid = row.serviceId
      const svc = this.services.find((s) => s.id === sid)
      if (!svc) {
        this.$message.error('未找到所属服务')
        return
      }
      await this.$confirm(`确认删除环境 ${row.name}？`, '提示')
      this.deletingEnvironmentId = row.id
      try {
        await deleteServiceEnvironment(this.projectId, sid, row.id)
        this.$message.success('环境已删除')
        await this.loadServices()
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

.service-env-root {
  min-height: 0;
}

.service-env-embedded-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.service-env-embedded-toolbar .page-subtitle {
  margin: 0;
  flex: 1;
  min-width: 200px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.service-env-header {
  gap: 12px;
  flex-wrap: wrap;
}

.service-env-tree-wrap {
  min-height: 280px;
}

.service-env-tree {
  width: 100%;
  max-width: 100%;
  padding: 8px 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  background: var(--el-fill-color-blank);
}

.service-env-tree :deep(.el-tree-node__content) {
  height: auto;
  min-height: 36px;
  align-items: stretch;
  padding-top: 4px;
  padding-bottom: 4px;
}

.tree-node-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex: 1;
  min-width: 0;
  padding-right: 8px;
}

.tree-node-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.tree-node-title {
  font-weight: 500;
  word-break: break-all;
}

.tree-node-meta {
  font-size: var(--app-font-size-small);
  color: var(--el-text-color-secondary);
  word-break: break-all;
  line-height: 1.35;
}

.tree-node-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.service-env-header .page-title {
  flex: 0 1 auto;
  min-width: 0;
}

.service-env-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
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
