<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">Swagger/OpenAPI 导入</h2>
    </div>

    <div class="toolbar spec-toolbar">
      <el-select
        v-model="serviceId"
        class="spec-filter-select"
        :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
        :title="projectId ? '选择当前项目下的服务，用于导入 Swagger/OpenAPI' : '请先在顶部选择项目后再选择服务'"
        :disabled="!projectId"
        :no-data-text="projectId ? '当前项目暂无服务' : '请先在顶部选择项目'"
        filterable
        @change="loadHistory"
      >
        <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
      </el-select>
      <input class="spec-file-input" type="file" accept=".json,.yaml,.yml" title="选择 .json、.yaml 或 .yml 格式的 OpenAPI/Swagger 文件" @change="readFile" />
    </div>

    <el-input v-model="content" class="json-editor" type="textarea" :rows="16" placeholder="粘贴 OpenAPI/Swagger JSON 或 YAML 内容" />
    <div class="actions">
      <el-button type="primary" :disabled="!projectId || !serviceId || !content" :loading="loading" @click="submit">导入并生成接口</el-button>
    </div>

    <el-alert v-if="summary" type="success" show-icon :closable="false" class="summary">
      <template #title>导入成功：接口定义 {{ summary.endpoints.length }} 个，生成可运行接口 {{ summary.generatedCases }} 条</template>
    </el-alert>

    <el-table :data="specs" border>
      <el-table-column prop="version" label="版本" width="80" />
      <el-table-column prop="contentHash" label="内容 Hash" min-width="260" />
      <el-table-column prop="status" label="状态" width="120" />
      <el-table-column prop="createdAt" label="导入时间" width="190" :formatter="formatDateTimeColumn" />
    </el-table>
  </div>
</template>

<script>
import { importSpec, listServices, listSpecs } from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { formatDateTime } from '../../utils/datetime'

export default {
  name: 'SpecImport',
  data() {
    return {
      services: [],
      specs: [],
      serviceId: '',
      content: '',
      summary: null,
      loading: false
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
    formatDateTimeColumn(row, column, value) {
      return formatDateTime(value)
    },
    async loadServices() {
      this.serviceId = ''
      this.specs = []
      this.services = this.projectId ? await listServices(this.projectId) : []
      if (this.services[0]) {
        this.serviceId = this.services[0].id
        this.loadHistory()
      }
    },
    async loadHistory() {
      if (!this.projectId || !this.serviceId) return
      this.specs = await listSpecs(this.projectId, this.serviceId)
    },
    readFile(event) {
      const file = event.target.files?.[0]
      if (!file) return
      const reader = new FileReader()
      reader.onload = () => {
        this.content = reader.result
      }
      reader.readAsText(file)
    },
    async submit() {
      this.loading = true
      try {
        this.summary = await importSpec(this.projectId, this.serviceId, this.content)
        this.$message.success('导入完成')
        this.loadHistory()
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.spec-toolbar {
  align-items: stretch;
}

.spec-filter-select {
  flex: 1 1 220px;
  max-width: 360px;
  min-width: 180px;
}

.spec-file-input {
  align-self: center;
  flex: 1 1 240px;
  min-width: 220px;
}

.actions {
  margin: 16px 0;
}

.summary {
  margin-bottom: 16px;
}

@media (max-width: 640px) {
  .spec-filter-select,
  .spec-file-input {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
