<template>
  <div class="callback-page">
    <el-card class="callback-card">
      <p>{{ message }}</p>
    </el-card>
  </div>
</template>

<script>
import { setToken } from '../utils/storage'
import { loadCurrentUser, authState } from '../auth'

export default {
  name: 'OAuthCallback',
  data() {
    return {
      message: '正在完成 OAuth 登录…',
    }
  },
  async mounted() {
    const hash = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : window.location.hash
    const params = new URLSearchParams(hash)
    const token = params.get('token')
    if (!token) {
      this.message = '未收到登录令牌，请返回登录页重试'
      return
    }

    try {
      setToken(decodeURIComponent(token))
      await loadCurrentUser()
      if (authState.user?.mustChangePassword) {
        await this.$router.replace('/change-password')
        return
      }
      if (authState.user?.active) {
        const redirect = this.$route.query.redirect || '/'
        await this.$router.replace(redirect)
      } else {
        await this.$router.replace('/pending-approval')
      }
    } catch {
      this.message = '登录失败，请返回登录页重试'
    }
  },
}
</script>

<style scoped>
.callback-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
}

.callback-card {
  width: 360px;
  text-align: center;
}
</style>
