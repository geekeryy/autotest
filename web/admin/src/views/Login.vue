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
    </el-card>
  </div>
</template>

<script>
import { login } from '../api'
import { setToken } from '../utils/storage'
import { loadCurrentUser } from '../auth'
import BrandLogo from '../components/BrandLogo.vue'

export default {
  name: 'Login',
  components: {
    BrandLogo
  },
  data() {
    return {
      loading: false,
      form: {
        username: 'admin',
        password: 'admin123'
      },
      rules: {
        username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
        password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
      }
    }
  },
  methods: {
    submit() {
      this.$refs.form.validate(async (valid) => {
        if (!valid) return
        this.loading = true
        try {
          const response = await login(this.form)
          setToken(response.token)
          await loadCurrentUser()
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
</style>
