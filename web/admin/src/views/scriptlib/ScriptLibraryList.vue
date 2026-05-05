<template>
  <div class="page-card">
    <div class="page-header script-lib-header">
      <div>
        <h2 class="page-title">脚本库</h2>
        <p class="page-subtitle">
          内置模板存于数据库、全平台共享，可编辑内容；项目自定义模板仅当前项目可见。在运行工作台断言、场景编排断言与脚本步骤中通过「脚本库」按钮插入。
        </p>
      </div>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增模板</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理脚本模板。"
    />

    <el-table v-if="projectId" :data="templates" border row-key="id" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="类型" width="96">
        <template #default="{ row }">
          <el-tag v-if="row.isBuiltin" type="info" size="small">内置</el-tag>
          <el-tag v-else type="success" size="small">自定义</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="category" label="分类" width="130" />
      <el-table-column label="适用范围" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="s in row.scopes || []" :key="s" size="small" style="margin-right: 6px">{{ scopeLabel(s) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
      <el-table-column label="更新时间" width="178">
        <template #default="{ row }">{{ formatDateTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button
            v-if="!row.isBuiltin"
            type="danger"
            link
            :loading="deletingId === row.id"
            @click="remove(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="projectId && !loading && !templates.length" description="暂无脚本模板；请先执行数据库迁移或稍后再试" />

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="720px"
      align-center
      destroy-on-close
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="简短名称，便于在脚本库列表中识别" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="默认「自定义」，可按团队习惯分组" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选；说明用途或需替换的占位参数" />
        </el-form-item>
        <el-form-item label="适用范围" required>
          <el-checkbox-group v-model="form.scopes">
            <el-checkbox label="assertion">接口断言（可用 pm.response）</el-checkbox>
            <el-checkbox label="scenario">场景脚本步骤（无 HTTP 响应）</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="脚本内容" required>
          <el-input
            v-model="form.code"
            type="textarea"
            :autosize="{ minRows: 12, maxRows: 28 }"
            placeholder="Postman 风格 pm.test / pm.variables / console …"
            class="script-code-input"
          />
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
  createScriptLibraryTemplate,
  deleteScriptLibraryTemplate,
  listScriptLibraryTemplates,
  updateScriptLibraryTemplate
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

function pad2(n) {
  return String(n).padStart(2, '0')
}

export default {
  name: 'ScriptLibraryList',
  data() {
    return {
      projectState,
      loading: false,
      templates: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      deletingId: null,
      editingBuiltin: false,
      form: {
        name: '',
        category: '',
        description: '',
        code: '',
        scopes: ['assertion', 'scenario']
      }
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId || ''
    },
    dialogTitle() {
      if (this.editingId && this.editingBuiltin) return '编辑内置脚本模板'
      if (this.editingId) return '编辑脚本模板'
      return '新增脚本模板'
    }
  },
  watch: {
    projectId: {
      immediate: true,
      handler(val) {
        if (val) this.loadList()
        else this.templates = []
      }
    }
  },
  created() {
    loadGlobalProjects()
  },
  methods: {
    scopeLabel(s) {
      if (s === 'assertion') return '接口断言'
      if (s === 'scenario') return '场景脚本'
      return s
    },
    formatDateTime(iso) {
      if (!iso) return ''
      const d = new Date(iso)
      if (Number.isNaN(d.getTime())) return String(iso)
      return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
    },
    async loadList() {
      if (!this.projectId) return
      this.loading = true
      try {
        this.templates = await listScriptLibraryTemplates(this.projectId)
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.editingId = null
      this.editingBuiltin = false
      this.form = {
        name: '',
        category: '',
        description: '',
        code: '',
        scopes: ['assertion', 'scenario']
      }
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editingId = row.id
      this.editingBuiltin = !!row.isBuiltin
      this.form = {
        name: row.name || '',
        category: row.category || '',
        description: row.description || '',
        code: row.code || '',
        scopes: Array.isArray(row.scopes) && row.scopes.length ? [...row.scopes] : ['assertion', 'scenario']
      }
      this.dialogVisible = true
    },
    async submit() {
      const name = (this.form.name || '').trim()
      const code = (this.form.code || '').trim()
      if (!name) {
        this.$message.warning('请填写名称')
        return
      }
      if (!code) {
        this.$message.warning('请填写脚本内容')
        return
      }
      if (!this.form.scopes || !this.form.scopes.length) {
        this.$message.warning('请至少选择一种适用范围')
        return
      }
      this.saving = true
      try {
        const payload = {
          name,
          category: (this.form.category || '').trim(),
          description: (this.form.description || '').trim(),
          code,
          scopes: this.form.scopes
        }
        if (this.editingId) {
          await updateScriptLibraryTemplate(this.editingId, payload)
          this.$message.success(this.editingBuiltin ? '内置模板已保存（全局生效）' : '已保存')
        } else {
          await createScriptLibraryTemplate({
            projectId: this.projectId,
            ...payload
          })
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
        await this.$confirm(`确定删除模板「${row.name}」？`, '删除确认', { type: 'warning' })
      } catch {
        return
      }
      this.deletingId = row.id
      try {
        await deleteScriptLibraryTemplate(row.id)
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
.script-lib-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.project-hint {
  margin-bottom: 12px;
}

.script-code-input :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.45;
}
</style>
