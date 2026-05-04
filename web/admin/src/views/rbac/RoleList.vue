<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">角色管理</h2>
      <el-button type="primary" @click="openCreate">新增角色</el-button>
    </div>

    <el-table :data="roles" border>
      <el-table-column prop="code" label="编码" width="160" />
      <el-table-column prop="name" label="名称" width="160" />
      <el-table-column prop="description" label="描述" />
      <el-table-column label="权限" min-width="260">
        <template #default="{ row }">
          <el-tag v-for="permission in row.permissions" :key="permission.id" size="small">{{ permission.name }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button type="text" @click="openEdit(row)">编辑</el-button>
          <el-button type="text" class="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑角色' : '新增角色'" width="620px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="编码"><el-input v-model="form.code" :disabled="editing" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" /></el-form-item>
        <el-form-item label="权限">
          <el-checkbox-group v-model="form.permissionIds">
            <el-checkbox v-for="permission in permissions" :key="permission.id" :label="permission.id">{{ permission.name }}</el-checkbox>
          </el-checkbox-group>
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
import { createRole, deleteRole, listPermissions, listRoles, updateRole } from '../../api'

export default {
  name: 'RoleList',
  data() {
    return {
      roles: [],
      permissions: [],
      dialogVisible: false,
      editing: null,
      form: this.emptyForm()
    }
  },
  created() {
    this.load()
  },
  methods: {
    emptyForm() {
      return { code: '', name: '', description: '', permissionIds: [] }
    },
    async load() {
      const [roles, permissions] = await Promise.all([listRoles(), listPermissions()])
      this.roles = roles
      this.permissions = permissions
    },
    openCreate() {
      this.editing = null
      this.form = this.emptyForm()
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editing = row
      this.form = {
        code: row.code,
        name: row.name,
        description: row.description,
        permissionIds: (row.permissions || []).map((permission) => permission.id)
      }
      this.dialogVisible = true
    },
    async submit() {
      if (this.editing) {
        const payload = { ...this.form }
        delete payload.code
        await updateRole(this.editing.id, payload)
      } else {
        await createRole(this.form)
      }
      this.$message.success('角色已保存')
      this.dialogVisible = false
      this.load()
    },
    async remove(row) {
      await this.$confirm(`确认删除角色 ${row.name}？`, '提示')
      await deleteRole(row.id)
      this.$message.success('角色已删除')
      this.load()
    }
  }
}
</script>

<style scoped>
.danger {
  color: #f56c6c;
}

.el-tag {
  margin-right: 4px;
  margin-bottom: 4px;
}
</style>
