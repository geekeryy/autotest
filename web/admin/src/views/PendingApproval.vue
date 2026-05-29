<template>
  <div class="pending-page">
    <el-card class="pending-card">
      <div class="pending-brand">
        <BrandLogo class="pending-logo" />
        <h1>账号待审核</h1>
      </div>
      <p class="pending-desc">
        您的 GitHub 账号已成功登录，但尚未被管理员启用。请等待管理员在「用户管理」中启用账号并分配角色。
      </p>
      <div v-if="authState.user" class="pending-user">
        <el-avatar :size="48" :src="authState.user.avatarUrl || undefined">
          {{ initials }}
        </el-avatar>
        <div>
          <div class="pending-name">{{ authState.user.displayName || authState.user.username }}</div>
          <div class="pending-meta">{{ authState.user.email || authState.user.username }}</div>
        </div>
      </div>
      <div class="pending-actions">
        <el-button :loading="refreshing" @click="refreshStatus">刷新审核状态</el-button>
        <el-button type="danger" plain @click="logout">退出登录</el-button>
      </div>
    </el-card>
  </div>
</template>

<script>
import BrandLogo from '../components/BrandLogo.vue'
import { authState, loadCurrentUser, resetCurrentUser } from '../auth'
import { clearToken } from '../utils/storage'
import { getInitials } from '../utils/useInitialsColor'

export default {
  name: 'PendingApproval',
  components: { BrandLogo },
  data() {
    return {
      authState,
      refreshing: false,
    }
  },
  computed: {
    initials() {
      return getInitials(this.authState.user?.displayName || this.authState.user?.username, '用')
    },
  },
  methods: {
    async refreshStatus() {
      this.refreshing = true
      try {
        const user = await loadCurrentUser()
        if (user?.active) {
          this.$router.replace('/')
        } else {
          this.$message.info('账号仍处于待审核状态')
        }
      } catch {
        clearToken()
        resetCurrentUser()
        this.$router.replace('/login')
      } finally {
        this.refreshing = false
      }
    },
    logout() {
      clearToken()
      resetCurrentUser()
      this.$router.replace('/login')
    },
  },
}
</script>

<style scoped>
.pending-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--app-sidebar-bg), var(--app-primary-color));
}

.pending-card {
  width: 420px;
}

.pending-brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.pending-logo {
  --brand-logo-size: 56px;
}

h1 {
  margin: 0;
  text-align: center;
}

.pending-desc {
  margin: 0 0 24px;
  color: var(--app-secondary-text);
  line-height: 1.6;
}

.pending-user {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  padding: 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
}

.pending-name {
  font-weight: 600;
}

.pending-meta {
  margin-top: 4px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.pending-actions {
  display: flex;
  gap: 12px;
}

.pending-actions .el-button {
  flex: 1;
}
</style>
