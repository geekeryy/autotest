<template>
  <div class="page-card user-management">
    <div class="page-header">
      <h2 class="page-title">用户管理</h2>
    </div>

    <el-tabs v-model="activeTab" class="user-mgmt-tabs" @tab-change="onTabChange">
      <el-tab-pane v-for="tab in visibleTabs" :key="tab.name" :label="tab.label" :name="tab.name" />
    </el-tabs>

    <UserList v-if="activeTab === 'users'" embedded />
    <AuthProviderList v-else-if="activeTab === 'auth-providers'" embedded />
    <RoleList v-else-if="activeTab === 'roles'" embedded />
    <PermissionList v-else-if="activeTab === 'permissions'" embedded />
  </div>
</template>

<script>
import { hasPermission } from '../../auth'
import UserList from './UserList.vue'
import AuthProviderList from './AuthProviderList.vue'
import RoleList from './RoleList.vue'
import PermissionList from './PermissionList.vue'

const TAB_DEFS = [
  { name: 'users', label: '用户', permission: 'users:manage' },
  { name: 'auth-providers', label: '登录方式', permission: 'users:manage' },
  { name: 'roles', label: '角色', permission: 'roles:manage' },
  { name: 'permissions', label: '权限菜单', permission: 'permissions:manage' },
]

export default {
  name: 'UserManagement',
  components: {
    UserList,
    AuthProviderList,
    RoleList,
    PermissionList,
  },
  data() {
    return {
      activeTab: '',
    }
  },
  computed: {
    visibleTabs() {
      return TAB_DEFS.filter((tab) => hasPermission(tab.permission))
    },
  },
  watch: {
    '$route.query.tab': {
      immediate: true,
      handler(tab) {
        this.activeTab = this.resolveTab(tab)
      },
    },
    visibleTabs: {
      immediate: true,
      handler() {
        this.activeTab = this.resolveTab(this.$route.query.tab)
      },
    },
  },
  methods: {
    resolveTab(tab) {
      const visible = this.visibleTabs
      if (!visible.length) return ''
      if (tab && visible.some((item) => item.name === tab)) return tab
      return visible[0].name
    },
    onTabChange(name) {
      if (this.$route.query.tab === name) return
      this.$router.replace({ path: '/users', query: { ...this.$route.query, tab: name } })
    },
  },
}
</script>

<style scoped>
.user-mgmt-tabs {
  margin-bottom: 16px;
}

.user-mgmt-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}
</style>
