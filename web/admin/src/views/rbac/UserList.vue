<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">用户管理</h2>
      <el-button type="primary" @click="openCreate">新增用户</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-table v-loading="loading" :data="pagedUsers" border>
      <el-table-column label="头像" width="72" align="center">
        <template #default="{ row }">
          <el-avatar :size="36" :src="row.avatarUrl || undefined" :style="avatarStyleForRow(row)">
            {{ initialsForRow(row) }}
          </el-avatar>
        </template>
      </el-table-column>
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="displayName" label="显示名" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column label="登录方式" width="110">
        <template #default="{ row }">
          {{ row.authProvider === 'github' ? 'GitHub' : '本地' }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag v-if="isPendingGithubUser(row)" type="warning" size="small">待审核</el-tag>
          <span v-else>{{ row.active ? '已启用' : '已停用' }}</span>
        </template>
      </el-table-column>
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
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" link @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-empty v-if="!loading && !loadError && !users.length" description="暂无用户">
      <el-button type="primary" @click="openCreate">新增用户</el-button>
    </el-empty>

    <el-pagination
      v-if="!loadError && users.length > pageSize"
      v-model:current-page="page"
      v-model:page-size="pageSize"
      class="list-pagination"
      :total="users.length"
      :page-sizes="[10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      background
    />

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新增用户'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名"><el-input v-model="form.username" :disabled="editing" /></el-form-item>
        <el-form-item v-if="!isGithubEditing" label="密码">
          <el-input v-model="form.password" show-password :placeholder="editing ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item label="头像">
          <div class="avatar-row">
            <el-avatar :size="56" :src="form.avatarPreview || undefined">{{ avatarPlaceholder }}</el-avatar>
            <div class="avatar-actions">
              <el-upload accept="image/*" :show-file-list="false" :auto-upload="false" @change="onAvatarFile">
                <el-button type="primary" plain>上传并压缩</el-button>
              </el-upload>
              <el-button v-if="form.avatarPreview || editingHasAvatar" type="danger" link @click="clearAvatarLocal">清除头像</el-button>
            </div>
          </div>
          <div class="avatar-hint">本地会先压成 JPEG（最长边约 384px），服务端再次缩放至 256px 内。</div>
        </el-form-item>
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
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { createUser, deleteUser, listRoles, listUsers, updateUser } from '../../api'
import { authState, loadCurrentUser } from '../../auth'
import ListLoadError from '../../components/ListLoadError.vue'
import { compressAvatarToJpegBase64 } from '../../utils/avatarImage'
import { getAvatarBgStyle, getInitials } from '../../utils/useInitialsColor'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'

export default {
  name: 'UserList',
  components: { ListLoadError },
  data() {
    return {
      loading: false,
      loadError: '',
      users: [],
      roles: [],
      page: 1,
      pageSize: 20,
      dialogVisible: false,
      editing: null,
      submitting: false,
      form: this.emptyForm(),
    }
  },
  computed: {
    pagedUsers() {
      const start = (this.page - 1) * this.pageSize
      return this.users.slice(start, start + this.pageSize)
    },
    editingHasAvatar() {
      return !!(this.editing && this.editing.avatarUrl)
    },
    avatarPlaceholder() {
      return getInitials(this.form.displayName || this.form.username, '用')
    },
    isGithubEditing() {
      return !!(this.editing && this.editing.authProvider === 'github')
    },
  },
  created() {
    this.load()
  },
  methods: {
    isPendingGithubUser(row) {
      return row.authProvider === 'github' && !row.active
    },
    initialsForRow(row) {
      return getInitials(row.displayName || row.username, '用')
    },
    avatarStyleForRow(row) {
      return getAvatarBgStyle(row.id || row.username, !!row.avatarUrl)
    },
    emptyForm() {
      return {
        username: '',
        password: '',
        displayName: '',
        email: '',
        active: true,
        roleIds: [],
        avatarBase64: '',
        avatarPreview: '',
        clearAvatar: false,
      }
    },
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        const [users, roles] = await Promise.all([listUsers(), listRoles()])
        this.users = users
        this.roles = roles
        const maxPage = Math.max(1, Math.ceil(this.users.length / this.pageSize))
        if (this.page > maxPage) this.page = maxPage
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err)
      } finally {
        this.loading = false
      }
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
        roleIds: (row.roles || []).map((role) => role.id),
        avatarBase64: '',
        avatarPreview: row.avatarUrl || '',
        clearAvatar: false,
      }
      this.dialogVisible = true
    },
    clearAvatarLocal() {
      this.form.clearAvatar = true
      this.form.avatarBase64 = ''
      this.form.avatarPreview = ''
    },
    async onAvatarFile(uploadFile) {
      const raw = uploadFile?.raw
      if (!raw) return
      try {
        const b64 = await compressAvatarToJpegBase64(raw)
        this.form.avatarBase64 = b64
        this.form.avatarPreview = `data:image/jpeg;base64,${b64}`
        this.form.clearAvatar = false
      } catch (e) {
        this.$message.error(e?.message || '图片处理失败')
      }
    },
    async submit() {
      this.submitting = true
      try {
        if (this.editing) {
          const payload = {
            password: this.form.password,
            displayName: this.form.displayName,
            email: this.form.email,
            active: this.form.active,
            roleIds: this.form.roleIds,
          }
          if (this.form.clearAvatar) {
            payload.clearAvatar = true
          } else if (this.form.avatarBase64) {
            payload.avatarBase64 = this.form.avatarBase64
          }
          await updateUser(this.editing.id, payload)
          const editedId = this.editing.id
          if (authState.user && editedId === authState.user.id) {
            await loadCurrentUser()
          }
        } else {
          const payload = {
            username: this.form.username,
            password: this.form.password,
            displayName: this.form.displayName,
            email: this.form.email,
            active: this.form.active,
            roleIds: this.form.roleIds,
          }
          if (this.form.avatarBase64) {
            payload.avatarBase64 = this.form.avatarBase64
          }
          await createUser(payload)
        }
        this.$message.success('用户已保存')
        this.dialogVisible = false
        await this.load()
      } finally {
        this.submitting = false
      }
    },
    async remove(row) {
      await this.$confirm(`确认删除用户 ${row.username}？`, '提示')
      await deleteUser(row.id)
      this.$message.success('用户已删除')
      this.load()
    },
  },
}
</script>

<style scoped>
.el-tag {
  margin-right: 4px;
}

.avatar-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.avatar-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.avatar-hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.list-pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
