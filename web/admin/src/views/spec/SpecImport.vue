<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">OpenAPI/Swagger 导入</h2>
    </div>

    <div class="toolbar spec-toolbar">
      <el-select
        v-model="serviceId"
        class="spec-filter-select"
        :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
        :title="projectId ? '选择当前项目下的服务，用于导入 OpenAPI/Swagger' : '请先在顶部选择项目后再选择服务'"
        :disabled="!projectId"
        :no-data-text="projectId ? '当前项目暂无服务' : '请先在顶部选择项目'"
        filterable
        @change="loadHistory"
      >
        <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
      </el-select>
      <div class="spec-file-input">
        <el-button @click="$refs.fileInput.click()">
          <el-icon><Upload /></el-icon>
          <span>选择文件</span>
        </el-button>
        <span class="spec-file-name">{{ fileName || '支持 .json、.yaml、.yml 格式' }}</span>
        <input ref="fileInput" type="file" hidden accept=".json,.yaml,.yml" @change="readFile" />
      </div>
    </div>

    <el-input v-model="content" class="json-editor" type="textarea" :rows="16" placeholder="粘贴 OpenAPI/Swagger JSON 或 YAML 内容" />
    <div class="actions">
      <el-button type="primary" :disabled="!projectId || !serviceId || !content" :loading="loading" @click="submit">导入并生成接口</el-button>
    </div>

    <el-alert v-if="summary" type="success" show-icon :closable="false" class="summary">
      <template #title>
        导入成功：文档中共 {{ summary.apiCount }} 个接口，端点新增 {{ summary.createdEndpoints }}、更新
        {{ summary.updatedEndpoints }}，写入请求模板 {{ summary.generatedCases }} 条（spec v{{ summary.specVersion }}）
      </template>
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
import { Upload } from '@element-plus/icons-vue'
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
      loading: false,
      fileName: ''
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
      this.fileName = file.name
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
  display: inline-flex;
  align-items: center;
  gap: 10px;
  align-self: center;
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
  .spec-filter-select,
  .spec-file-input {
    max-width: none;
    min-width: 0;
    width: 100%;
  }
}
</style>
