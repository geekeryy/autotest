<template>
  <div :class="embedded ? 'data-source-root' : 'page-card'">
    <div v-if="!embedded" class="page-header data-source-header">
      <div>
        <h2 class="page-title">业务数据源</h2>
        <p class="page-subtitle">按当前项目维护被测业务库连接（与环境、服务无关），配置后可被 SQL 参数源与场景数据库步骤复用。</p>
      </div>
      <el-button type="primary" :disabled="!canCreate" @click="openCreate">新增数据源</el-button>
    </div>
    <div v-else class="data-source-embedded-toolbar">
      <p class="page-subtitle">按当前全局项目维护被测业务库连接；与环境、服务无关，可被 SQL 参数源与场景数据库步骤复用。</p>
      <el-button type="primary" :disabled="!canCreate" @click="openCreate">新增数据源</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在左侧列表或顶部选择项目后再管理业务数据源。"
    />



    <el-table :data="dataSources" border row-key="id" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="driver" label="Driver" width="120" />
      <el-table-column label="连接信息" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">{{ connectionSummary(row) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link :loading="testingId === row.id" @click="testConnection(row)">测试</el-button>
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !dataSources.length" description="暂无业务数据源，点击「新增数据源」配置后可多处复用" />

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑业务数据源' : '新增业务数据源'" width="760px" align-center>
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称"><el-input v-model="form.name" placeholder="例如：订单库只读连接" /></el-form-item>
        <el-form-item label="Driver">
          <el-select v-model="form.driver" class="full-width">
            <el-option label="PostgreSQL" value="postgres" />
          </el-select>
        </el-form-item>
        <el-form-item label="Host"><el-input v-model="form.host" placeholder="127.0.0.1" /></el-form-item>
        <el-form-item label="Port"><el-input v-model="form.port" placeholder="5432" /></el-form-item>
        <el-form-item label="Database"><el-input v-model="form.database" placeholder="business_db" /></el-form-item>
        <el-form-item label="Username"><el-input v-model="form.username" placeholder="readonly_user" /></el-form-item>
        <el-form-item label="Password">
          <el-input v-model="form.password" type="password" show-password :placeholder="editing ? '留空表示不修改密码' : '数据库密码'" />
        </el-form-item>
        <el-form-item label="SSL Mode">
          <el-select v-model="form.sslMode" class="full-width">
            <el-option label="disable" value="disable" />
            <el-option label="prefer" value="prefer" />
            <el-option label="require" value="require" />
          </el-select>
        </el-form-item>
        <el-form-item label="DSN">
          <el-input v-model="form.dsn" type="textarea" :rows="3" placeholder="可选：完整连接串，填写后后端可优先使用" />
        </el-form-item>
        <el-form-item label="扩展配置 JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>用于后端扩展连接参数，保存时会与上方字段合并。</span>
              <el-button size="small" @click="formatExtraConfig">格式化</el-button>
            </div>
            <el-input v-model="form.extraConfig" class="json-editor" type="textarea" :autosize="{ minRows: 5, maxRows: 14 }" />
          </div>
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
import { createDataSource, deleteDataSource, listDataSources, testDataSource, updateDataSource } from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

export default {
  name: 'DataSourceList',
  props: {
    embedded: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      dataSources: [],
      loading: false,
      saving: false,
      testingId: '',
      deletingId: '',
      dialogVisible: false,
      editing: null,
      form: this.emptyForm()
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.loadDataSources()
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    canCreate() {
      return !!this.projectId
    }
  },
  watch: {
    projectId() {
      this.loadDataSources()
    }
  },
  methods: {
    emptyForm() {
      return {
        name: '',
        driver: 'postgres',
        host: '',
        port: '5432',
        database: '',
        username: '',
        password: '',
        sslMode: 'disable',
        dsn: '',
        extraConfig: '{}'
      }
    },
    configOf(row) {
      return row?.config && typeof row.config === 'object' ? row.config : {}
    },
    connectionSummary(row) {
      const config = this.configOf(row)
      if (row.dsn) return row.dsn
      const host = row.host || config.host || '-'
      const port = row.port || config.port || ''
      const database = row.database || config.database || ''
      const user = row.username || config.username || ''
      return `${host}${port ? `:${port}` : ''}${database ? `/${database}` : ''}${user ? ` · ${user}` : ''}`
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
    async loadDataSources() {
      if (!this.projectId) {
        this.dataSources = []
        return
      }
      this.loading = true
      try {
        this.dataSources = await listDataSources({ projectId: this.projectId })
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      if (!this.canCreate) {
        this.$message.warning('请先选择项目')
        return
      }
      this.editing = null
      this.form = this.emptyForm()
      this.dialogVisible = true
    },
    openEdit(row) {
      const config = this.configOf(row)
      const extraConfig = { ...config }
      for (const key of ['host', 'port', 'database', 'username', 'password', 'sslMode']) {
        delete extraConfig[key]
      }
      this.editing = row
      this.form = {
        name: row.name || '',
        driver: row.driver || 'postgres',
        host: row.host || config.host || '',
        port: String(row.port || config.port || '5432'),
        database: row.database || config.database || '',
        username: row.username || config.username || '',
        password: '',
        sslMode: row.sslMode || config.sslMode || 'disable',
        dsn: row.dsn || '',
        extraConfig: this.formatJSON(extraConfig)
      }
      this.dialogVisible = true
    },
    formatExtraConfig() {
      try {
        this.form.extraConfig = JSON.stringify(JSON.parse(this.form.extraConfig || '{}'), null, 2)
      } catch (error) {
        this.$message.error(`扩展配置 JSON 不合法：${error.message}`)
      }
    },
    parsePortInput(value) {
      const text = String(value ?? '').trim()
      if (!text) return 0
      const port = Number(text)
      if (!Number.isInteger(port) || port < 0) {
        this.$message.error('Port 必须是数字')
        return null
      }
      return port
    },
    buildPayload() {
      let extraConfig
      try {
        extraConfig = JSON.parse(this.form.extraConfig || '{}')
      } catch (error) {
        this.$message.error(`扩展配置 JSON 不合法：${error.message}`)
        return null
      }
      const port = this.parsePortInput(this.form.port)
      if (port == null) return null
      const config = {
        ...extraConfig,
        host: this.form.host,
        port,
        database: this.form.database,
        username: this.form.username,
        sslMode: this.form.sslMode
      }
      if (this.form.password) config.password = this.form.password
      const payload = {
        projectId: this.projectId,
        name: this.form.name,
        driver: this.form.driver,
        dsn: this.form.dsn,
        config,
        host: this.form.host,
        port,
        database: this.form.database,
        username: this.form.username,
        sslMode: this.form.sslMode
      }
      if (this.form.password) payload.password = this.form.password
      return payload
    },
    async submit() {
      const payload = this.buildPayload()
      if (!payload) return
      this.saving = true
      try {
        if (this.editing) {
          await updateDataSource(this.editing.id, payload)
        } else {
          await createDataSource(payload)
        }
        this.$message.success('业务数据源已保存')
        this.dialogVisible = false
        this.editing = null
        await this.loadDataSources()
      } finally {
        this.saving = false
      }
    },
    async testConnection(row) {
      this.testingId = row.id
      try {
        const result = await testDataSource(row.id)
        const message = result?.message || (result?.success === false ? '连接测试未通过' : '连接测试通过')
        if (result?.success === false) {
          this.$message.error(message)
        } else {
          this.$message.success(message)
        }
      } finally {
        this.testingId = ''
      }
    },
    async remove(row) {
      await this.$confirm(`确认删除业务数据源「${row.name}」？`, '提示')
      this.deletingId = row.id
      try {
        await deleteDataSource(row.id)
        this.$message.success('业务数据源已删除')
        await this.loadDataSources()
      } finally {
        this.deletingId = ''
      }
    }
  }
}
</script>

<style scoped>
.data-source-root {
  min-height: 0;
}

.data-source-embedded-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.data-source-embedded-toolbar .page-subtitle {
  margin: 0;
  flex: 1;
  min-width: 200px;
}

.data-source-header {
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

.data-source-toolbar {
  align-items: stretch;
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

.json-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}
</style>
