<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">API管理</h2>
      <el-button type="primary" :disabled="!projectId || !serviceId" @click="dialogVisible = true">新增接口</el-button>
    </div>

    <div class="toolbar case-toolbar">
      <el-select
        v-model="serviceId"
        class="case-filter-select"
        :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
        :disabled="!projectId"
        :no-data-text="projectId ? '当前项目暂无服务' : '请先在顶部选择项目'"
        filterable
        @change="loadCases"
      >
        <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
      </el-select>
    </div>

    <el-table :data="cases" border row-key="id">
      <el-table-column label="接口名称" min-width="180">
        <template #default="{ row }">{{ displayCaseName(row) }}</template>
      </el-table-column>
      <el-table-column prop="method" label="方法" width="90" />
      <el-table-column prop="path" label="路径" min-width="220" />
      <el-table-column prop="source" label="来源" width="100" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column label="操作" width="110">
        <template #default="{ row }">
          <el-button type="primary" link @click="openRun(row)">运行</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新增手工接口" width="640px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="方法"><el-select v-model="form.method"><el-option v-for="m in methods" :key="m" :label="m" :value="m" /></el-select></el-form-item>
        <el-form-item label="路径"><el-input v-model="form.path" placeholder="/api/users" /></el-form-item>
        <el-form-item label="请求 JSON"><el-input v-model="form.request" class="json-editor" type="textarea" :rows="5" /></el-form-item>
        <el-form-item label="断言 JSON"><el-input v-model="form.assertions" class="json-editor" type="textarea" :rows="5" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { createCase, listCases, listEndpoints, listServices } from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

export default {
  name: 'CaseList',
  data() {
    return {
      services: [],
      cases: [],
      endpoints: [],
      serviceId: '',
      dialogVisible: false,
      methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
      form: {
        name: '',
        method: 'GET',
        path: '',
        request: '{}',
        assertions: '[{"type":"status_code","expected":200}]'
      }
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.loadServices()
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    }
  },
  watch: {
    projectId() {
      this.loadServices()
    }
  },
  methods: {
    async loadServices() {
      this.serviceId = ''
      this.cases = []
      this.endpoints = []
      this.services = this.projectId ? await listServices(this.projectId) : []
      if (this.services[0]) {
        this.serviceId = this.services[0].id
        this.loadCases()
      }
    },
    async loadCases() {
      if (!this.projectId || !this.serviceId) {
        this.cases = []
        this.endpoints = []
        return
      }
      const [cases, endpoints] = await Promise.all([
        listCases({ projectId: this.projectId, serviceId: this.serviceId }),
        listEndpoints(this.projectId, this.serviceId)
      ])
      this.cases = cases
      this.endpoints = endpoints
    },
    endpointForCase(row) {
      return this.endpoints.find((endpoint) => endpoint.id === row.endpointId) ||
        this.endpoints.find((endpoint) => endpoint.method === row.method && endpoint.path === row.path) ||
        null
    },
    displayCaseName(row) {
      return this.endpointForCase(row)?.summary || row.name || row.path
    },
    async submit() {
      await createCase({
        projectId: this.projectId,
        serviceId: this.serviceId,
        name: this.form.name,
        method: this.form.method,
        path: this.form.path,
        request: JSON.parse(this.form.request || '{}'),
        assertions: JSON.parse(this.form.assertions || '[]')
      })
      this.$message.success('接口已创建')
      this.dialogVisible = false
      this.form = { name: '', method: 'GET', path: '', request: '{}', assertions: '[{"type":"status_code","expected":200}]' }
      this.loadCases()
    },
    openRun(row) {
      this.$router.push(`/run-console/${row.id}`)
    }
  }
}
</script>

<style scoped>
.case-toolbar {
  align-items: stretch;
}

.case-filter-select {
  flex: 1 1 220px;
  max-width: 360px;
  min-width: 180px;
}

@media (max-width: 640px) {
  .case-filter-select {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
