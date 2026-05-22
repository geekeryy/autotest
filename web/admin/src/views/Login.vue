<template>
  <div class="login-page">
    <el-card class="login-card">
      <div class="login-brand">
        <BrandLogo class="login-logo" />
        <h1>Autotest 管理后台</h1>
      </div>
      <p>接口自动化测试平台</p>
      <el-form ref="form" :model="form" :rules="rules" @keyup.enter="submit">
        <el-form-item prop="username">
          <el-input v-model="form.username" prefix-icon="User" placeholder="用户名" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" prefix-icon="Lock" placeholder="密码" show-password />
        </el-form-item>
        <el-button type="primary" :loading="loading" class="submit" @click="submit">登录</el-button>
      </el-form>
      <template v-if="githubOAuthConfigured">
        <div class="oauth-divider">或</div>
        <el-button class="github-btn" :loading="githubLoading" @click="loginWithGitHub">
          使用 GitHub 登录
        </el-button>
      </template>
    </el-card>
  </div>
</template>

<script>
import { getGitHubOAuthStatus, login } from '../api'
import { setToken } from '../utils/storage'
import { loadCurrentUser, authState } from '../auth'
import BrandLogo from '../components/BrandLogo.vue'

const OAUTH_ERROR_MESSAGES = {
  oauth_disabled: 'GitHub 登录尚未配置',
  invalid_state: '登录已过期或无效，请重新尝试',
  missing_code: '未收到授权码，请重试',
  exchange_failed: 'GitHub 授权失败，请稍后重试',
  profile_failed: '无法获取 GitHub 用户信息，请重试',
  oauth_failed: 'GitHub 登录失败，请重试',
}

export default {
  name: 'Login',
  components: {
    BrandLogo
  },
  data() {
    return {
      loading: false,
      githubLoading: false,
      githubOAuthConfigured: false,
      form: {
        username: '',
        password: ''
      },
      rules: {
        username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
        password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
      }
    }
  },
  async created() {
    const error = this.$route.query.error
    if (error) {
      const key = String(error)
      this.$message.error(OAUTH_ERROR_MESSAGES[key] || OAUTH_ERROR_MESSAGES.oauth_failed)
    }
    try {
      const status = await getGitHubOAuthStatus()
      this.githubOAuthConfigured = Boolean(status?.configured)
    } catch {
      this.githubOAuthConfigured = false
    }
  },
  methods: {
    loginWithGitHub() {
      this.githubLoading = true
      window.location.href = `${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/auth/github/login`
    },
    submit() {
      this.$refs.form.validate(async (valid) => {
        if (!valid) return
        this.loading = true
        try {
          const response = await login(this.form)
          setToken(response.token)
          await loadCurrentUser()
          if (!authState.user?.active) {
            this.$router.replace('/pending-approval')
            return
          }
          const redirect = this.$route.query.redirect || '/dashboard'
          this.$router.replace(redirect)
        } catch {
          // 错误已在全局请求拦截器中提示
        } finally {
          this.loading = false
        }
      })
    }
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--app-sidebar-bg), var(--app-primary-color));
}

.login-card {
  width: 380px;
}

.login-brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.login-logo {
  --brand-logo-size: 56px;
}

h1 {
  margin: 0;
  text-align: center;
}

p {
  margin: 0 0 24px;
  color: var(--app-secondary-text);
  text-align: center;
}

.submit {
  width: 100%;
}

.oauth-divider {
  margin: 16px 0;
  text-align: center;
  color: var(--app-secondary-text);
}

.github-btn {
  width: 100%;
}
</style>
