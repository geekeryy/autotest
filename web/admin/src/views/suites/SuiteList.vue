<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">测试集管理</h2>
      <el-button type="primary" :disabled="!projectId || !serviceId" @click="dialogVisible = true">新建测试集</el-button>
    </div>

    <div class="toolbar suite-toolbar">
      <el-select
        v-model="serviceId"
        class="filter-select"
        :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
        :disabled="!projectId"
        :no-data-text="projectId ? '当前项目暂无服务' : '请先在顶部选择项目'"
        filterable
        @change="loadData"
      >
        <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
      </el-select>
    </div>

    <el-table :data="suites" border @current-change="selectedSuite = $event" highlight-current-row>
      <el-table-column prop="name" label="测试集" min-width="180" />
      <el-table-column prop="kind" label="类型" width="100" />
      <el-table-column prop="description" label="描述" min-width="220" />
    </el-table>

    <div class="add-case">
      <el-select v-model="selectedCaseId" placeholder="选择用例添加到当前测试集" filterable>
        <el-option v-for="item in cases" :key="item.id" :label="`${item.method} ${item.path} - ${item.name}`" :value="item.id" />
      </el-select>
      <el-button type="primary" :disabled="!selectedSuite || !selectedCaseId" @click="addCase">添加用例</el-button>
    </div>

    <el-dialog v-model="dialogVisible" title="新建测试集" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.kind">
            <el-option label="手工" value="manual" />
            <el-option label="自动" value="auto" />
            <el-option label="混合" value="mixed" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { addSuiteItem, createSuite, listCases, listServices, listSuites } from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

export default {
  name: 'SuiteList',
  data() {
    return {
      services: [],
      suites: [],
      cases: [],
      serviceId: '',
      selectedSuite: null,
      selectedCaseId: '',
      dialogVisible: false,
      form: { name: '', kind: 'manual', description: '' }
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
      this.suites = []
      this.cases = []
      this.services = this.projectId ? await listServices(this.projectId) : []
      if (this.services[0]) {
        this.serviceId = this.services[0].id
        this.loadData()
      }
    },
    async loadData() {
      if (!this.projectId || !this.serviceId) {
        this.suites = []
        this.cases = []
        this.selectedSuite = null
        return
      }
      const params = { projectId: this.projectId, serviceId: this.serviceId }
      const [suites, cases] = await Promise.all([listSuites(params), listCases(params)])
      this.suites = suites
      this.cases = cases
      this.selectedSuite = null
    },
    async submit() {
      await createSuite({
        ...this.form,
        projectId: this.projectId,
        serviceId: this.serviceId
      })
      this.$message.success('测试集已创建')
      this.dialogVisible = false
      this.form = { name: '', kind: 'manual', description: '' }
      this.loadData()
    },
    async addCase() {
      await addSuiteItem(this.selectedSuite.id, {
        testCaseId: this.selectedCaseId,
        itemOrder: 1,
        source: 'manual_added',
        enabled: true
      })
      this.$message.success('用例已添加')
      this.selectedCaseId = ''
    }
  }
}
</script>

<style scoped>
.suite-toolbar {
  align-items: stretch;
}

.filter-select {
  flex: 1 1 220px;
  max-width: 360px;
  min-width: 180px;
}

.add-case {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.add-case .el-select {
  width: 420px;
}

@media (max-width: 640px) {
  .filter-select {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
