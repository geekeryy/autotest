<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">用户管理</h2>
      <el-button type="primary" @click="openCreate">新增用户</el-button>
    </div>

    <el-table :data="users" border>
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="displayName" label="显示名" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column label="角色" min-width="180">
        <template #default="{ row }">
          <el-tag v-for="role in row.roles" :key="role.id" size="small">{{ role.name }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="active" label="启用" width="80">
        <template #default="{ row }">{{ row.active ? '是' : '否' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button type="text" @click="openEdit(row)">编辑</el-button>
          <el-button type="text" class="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新增用户'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名"><el-input v-model="form.username" :disabled="editing" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" show-password :placeholder="editing ? '留空则不修改' : ''" /></el-form-item>
        <el-form-item label="显示名"><el-input v-model="form.displayName" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.active" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.roleIds" multiple filterable>
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
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
import { createUser, deleteUser, listRoles, listUsers, updateUser } from '../../api'

export default {
  name: 'UserList',
  data() {
    return {
      users: [],
      roles: [],
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
      return { username: '', password: '', displayName: '', email: '', active: true, roleIds: [] }
    },
    async load() {
      const [users, roles] = await Promise.all([listUsers(), listRoles()])
      this.users = users
      this.roles = roles
    },
    openCreate() {
      this.editing = null
      this.form = this.emptyForm()
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editing = row
      this.form = {
        username: row.username,
        password: '',
        displayName: row.displayName,
        email: row.email,
        active: row.active,
        roleIds: (row.roles || []).map((role) => role.id)
      }
      this.dialogVisible = true
    },
    async submit() {
      if (this.editing) {
        const payload = { ...this.form }
        delete payload.username
        await updateUser(this.editing.id, payload)
      } else {
        await createUser(this.form)
      }
      this.$message.success('用户已保存')
      this.dialogVisible = false
      this.load()
    },
    async remove(row) {
      await this.$confirm(`确认删除用户 ${row.username}？`, '提示')
      await deleteUser(row.id)
      this.$message.success('用户已删除')
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
}
</style>
