<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">项目管理</h2>
      <el-button type="primary" @click="dialogVisible = true">
        <el-icon><Plus /></el-icon>
        <span>新建项目</span>
      </el-button>
    </div>

    <el-table v-loading="loading" :data="projects" border>
      <el-table-column prop="name" label="项目名称" min-width="180" />
      <el-table-column prop="description" label="描述" min-width="240" />
      <el-table-column prop="createdAt" label="创建时间" width="180" :formatter="formatDateTimeColumn" />
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-popconfirm
            title="删除后项目及关联数据将被软删除，确认删除？"
            confirm-button-text="删除"
            cancel-button-text="取消"
            confirm-button-type="danger"
            @confirm="remove(row)"
          >
            <template #reference>
              <el-button link type="danger" :loading="deletingId === row.id">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新建项目" width="420px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { createProject, deleteProject, listProjects } from '../../api'
import { refreshGlobalProjects, setCurrentProjectId } from '../../utils/currentProject'
import { formatDateTime } from '../../utils/datetime'

export default {
  name: 'ProjectList',
  data() {
    return {
      loading: false,
      deletingId: '',
      dialogVisible: false,
      projects: [],
      form: {
        name: '',
        description: ''
      }
    }
  },
  created() {
    this.load()
  },
  methods: {
    formatDateTimeColumn(row, column, value) {
      return formatDateTime(value)
    },
    async load() {
      this.loading = true
      try {
        this.projects = await listProjects()
      } finally {
        this.loading = false
      }
    },
    async submit() {
      const project = await createProject(this.form)
      this.$message.success('项目已创建')
      this.dialogVisible = false
      this.form = { name: '', description: '' }
      await this.load()
      await refreshGlobalProjects()
      if (project?.id) setCurrentProjectId(project.id)
    },
    async remove(row) {
      this.deletingId = row.id
      try {
        await deleteProject(row.id)
        this.$message.success('项目已删除')
        await this.load()
        await refreshGlobalProjects()
      } finally {
        this.deletingId = ''
      }
    }
  }
}
</script>
