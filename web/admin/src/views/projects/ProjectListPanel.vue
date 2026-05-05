<template>
  <div class="project-list-panel">
    <div class="sidebar-toolbar">
      <el-button type="primary" size="small" class="sidebar-new-btn" @click="dialogVisible = true">
        <el-icon><Plus /></el-icon>
        <span>新建项目</span>
      </el-button>
    </div>
    <p class="sidebar-hint">点击项目即设为当前全局项目；可用快捷入口打开右侧「服务与环境」或「业务数据源」。</p>

    <el-scrollbar class="sidebar-scroll">
      <div v-loading="loading" class="sidebar-list">
        <el-empty v-if="!loading && !projects.length" description="暂无项目，请先新建" :image-size="72" />
        <div
          v-for="row in projects"
          :key="row.id"
          class="sidebar-item"
          :class="{ 'is-active': row.id === currentProjectId }"
          role="button"
          tabindex="0"
          @click="selectProject(row)"
          @keydown.enter.prevent="selectProject(row)"
        >
          <div class="sidebar-item-head">
            <span class="sidebar-item-name" :title="row.name">{{ row.name }}</span>
            <el-tag v-if="row.myRole" :type="roleTagType(row.myRole)" size="small" class="sidebar-role-tag">
              {{ roleLabel(row.myRole) }}
            </el-tag>
            <span v-else class="role-admin-hint">管理员</span>
          </div>
          <p v-if="row.description" class="sidebar-item-desc" :title="row.description">{{ row.description }}</p>
          <div class="sidebar-item-meta">{{ formatDateTime(row.createdAt) }}</div>
          <div class="sidebar-item-actions" @click.stop>
            <el-button link type="primary" size="small" @click="openServices(row)">环境</el-button>
            <el-button link type="primary" size="small" @click="openDataSources(row)">数据源</el-button>
            <el-button link type="primary" size="small" @click="openMembers(row)">成员</el-button>
            <el-popconfirm
              title="删除后项目及关联数据将被软删除，确认删除？"
              confirm-button-text="删除"
              cancel-button-text="取消"
              confirm-button-type="danger"
              @confirm="remove(row)"
            >
              <template #reference>
                <el-button link type="danger" size="small" :loading="deletingId === row.id">删除</el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </el-scrollbar>

    <!-- 新建项目弹窗 -->
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

    <!-- 成员管理弹窗 -->
    <el-dialog
      v-model="membersDialogVisible"
      :title="`成员管理 · ${currentProject?.name || ''}`"
      width="640px"
      @open="loadMembers"
    >
      <div class="members-toolbar">
        <el-button type="primary" size="small" @click="addMemberDialogVisible = true">
          <el-icon><Plus /></el-icon>添加成员
        </el-button>
      </div>
      <el-table v-loading="membersLoading" :data="members" border size="small">
        <el-table-column prop="displayName" label="姓名" min-width="100" />
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column label="角色" width="140">
          <template #default="{ row }">
            <el-select
              v-model="row.role"
              size="small"
              style="width: 110px"
              @change="(val) => changeRole(row, val)"
            >
              <el-option label="所有者" value="owner" />
              <el-option label="开发者" value="developer" />
              <el-option label="观察者" value="viewer" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="removeMember(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 添加成员子弹窗 -->
      <el-dialog
        v-model="addMemberDialogVisible"
        title="添加成员"
        width="380px"
        append-to-body
      >
        <el-form :model="addMemberForm" label-width="70px">
          <el-form-item label="用户">
            <el-select
              v-model="addMemberForm.userId"
              filterable
              placeholder="选择用户"
              style="width: 100%"
              :loading="usersLoading"
            >
              <el-option
                v-for="u in availableUsers"
                :key="u.id"
                :label="`${u.displayName || u.username} (${u.username})`"
                :value="u.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="角色">
            <el-select v-model="addMemberForm.role" style="width: 100%">
              <el-option label="所有者" value="owner" />
              <el-option label="开发者" value="developer" />
              <el-option label="观察者" value="viewer" />
            </el-select>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="addMemberDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="addingMember" @click="submitAddMember">添加</el-button>
        </template>
      </el-dialog>
    </el-dialog>
  </div>
</template>

<script>
import {
  createProject,
  deleteProject,
  listProjects,
  listProjectMembers,
  addProjectMember,
  updateProjectMember,
  removeProjectMember,
  listUsers,
} from '../../api'
import { refreshGlobalProjects, setCurrentProjectId, projectState } from '../../utils/currentProject'
import { formatDateTime } from '../../utils/datetime'

const ROLE_LABELS = { owner: '所有者', developer: '开发者', viewer: '观察者' }
const ROLE_TAG_TYPES = { owner: 'danger', developer: 'primary', viewer: 'info' }

export default {
  name: 'ProjectListPanel',
  data() {
    return {
      loading: false,
      deletingId: '',
      dialogVisible: false,
      projects: [],
      form: { name: '', description: '' },

      membersDialogVisible: false,
      membersLoading: false,
      currentProject: null,
      members: [],

      addMemberDialogVisible: false,
      addingMember: false,
      addMemberForm: { userId: '', role: 'developer' },
      allUsers: [],
      usersLoading: false,
    }
  },
  computed: {
    currentProjectId() {
      return projectState.currentProjectId
    },
    availableUsers() {
      const memberIds = new Set(this.members.map((m) => m.userId))
      return this.allUsers.filter((u) => !memberIds.has(u.id))
    },
  },
  created() {
    this.load()
  },
  methods: {
    formatDateTime,
    roleLabel(role) {
      return ROLE_LABELS[role] || role
    },
    roleTagType(role) {
      return ROLE_TAG_TYPES[role] || ''
    },
    selectProject(row) {
      setCurrentProjectId(row.id)
    },
    async load() {
      this.loading = true
      try {
        this.projects = await listProjects()
      } finally {
        this.loading = false
      }
    },
    openServices(row) {
      setCurrentProjectId(row.id)
      this.$router.replace({ path: '/projects', query: { tab: 'services' } })
    },
    openDataSources(row) {
      setCurrentProjectId(row.id)
      this.$router.replace({ path: '/projects', query: { tab: 'dataSources' } })
    },
    async openMembers(row) {
      this.currentProject = row
      this.membersDialogVisible = true
      this.addMemberForm = { userId: '', role: 'developer' }
      this.usersLoading = true
      try {
        this.allUsers = (await listUsers()) || []
      } finally {
        this.usersLoading = false
      }
    },
    async loadMembers() {
      if (!this.currentProject) return
      this.membersLoading = true
      try {
        this.members = await listProjectMembers(this.currentProject.id)
      } finally {
        this.membersLoading = false
      }
    },
    async changeRole(member, newRole) {
      try {
        await updateProjectMember(this.currentProject.id, member.userId, { role: newRole })
        this.$message.success('角色已更新')
      } catch {
        this.$message.error('更新角色失败')
        await this.loadMembers()
      }
    },
    async removeMember(member) {
      try {
        await removeProjectMember(this.currentProject.id, member.userId)
        this.$message.success('成员已移除')
        await this.loadMembers()
      } catch {
        this.$message.error('移除成员失败')
      }
    },
    async submitAddMember() {
      if (!this.addMemberForm.userId) {
        this.$message.warning('请选择用户')
        return
      }
      this.addingMember = true
      try {
        await addProjectMember(this.currentProject.id, {
          userId: this.addMemberForm.userId,
          role: this.addMemberForm.role,
        })
        this.$message.success('成员已添加')
        this.addMemberDialogVisible = false
        this.addMemberForm = { userId: '', role: 'developer' }
        await this.loadMembers()
      } catch (e) {
        this.$message.error(e?.response?.data?.error || '添加成员失败')
      } finally {
        this.addingMember = false
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
    },
  },
}
</script>

<style scoped>
.project-list-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 360px;
  padding: 12px;
  box-sizing: border-box;
}

.sidebar-toolbar {
  flex-shrink: 0;
  margin-bottom: 8px;
}

.sidebar-new-btn {
  width: 100%;
}

.sidebar-hint {
  flex-shrink: 0;
  margin: 0 0 10px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

.sidebar-scroll {
  flex: 1;
  min-height: 0;
}

.sidebar-scroll :deep(.el-scrollbar__wrap) {
  overflow-x: hidden;
}

.sidebar-list {
  padding-right: 4px;
}

.sidebar-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  padding: 10px 10px 8px;
  margin-bottom: 8px;
  cursor: pointer;
  background: var(--el-bg-color);
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.sidebar-item:hover {
  border-color: var(--el-color-primary-light-5);
}

.sidebar-item.is-active {
  border-color: var(--el-color-primary);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
}

.sidebar-item-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.sidebar-item-name {
  font-weight: 600;
  font-size: var(--app-font-size-base);
  color: var(--el-text-color-primary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-role-tag {
  flex-shrink: 0;
}

.sidebar-item-desc {
  margin: 0 0 4px;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.sidebar-item-meta {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-bottom: 6px;
}

.sidebar-item-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px 4px;
  padding-top: 4px;
  border-top: 1px dashed var(--el-border-color-lighter);
}

.role-admin-hint {
  font-size: 12px;
  color: var(--app-secondary-text);
}

.members-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
</style>
