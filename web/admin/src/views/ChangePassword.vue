<template>
  <div class="change-page">
    <el-card class="change-card">
      <div class="change-brand">
        <BrandLogo class="change-logo" />
        <h1>首次登录须修改密码</h1>
      </div>
      <p class="change-desc">
        为保障账号安全，请设置新密码。修改成功后需使用新密码重新登录。
      </p>
      <el-form ref="form" :model="form" :rules="rules" @keyup.enter="submit">
        <el-form-item prop="currentPassword">
          <el-input
            v-model="form.currentPassword"
            prefix-icon="Lock"
            placeholder="当前密码"
            show-password
          />
        </el-form-item>
        <el-form-item prop="newPassword">
          <el-input
            v-model="form.newPassword"
            prefix-icon="Key"
            placeholder="新密码（至少 6 位）"
            show-password
          />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            prefix-icon="Key"
            placeholder="确认新密码"
            show-password
          />
        </el-form-item>
        <el-button type="primary" :loading="loading" class="submit" @click="submit">
          修改密码并重新登录
        </el-button>
      </el-form>
      <div class="change-actions">
        <el-button type="danger" plain @click="logout">退出登录</el-button>
      </div>
    </el-card>
  </div>
</template>

<script>
import BrandLogo from '../components/BrandLogo.vue'
import { changePassword } from '../api'
import { clearToken } from '../utils/storage'
import { resetCurrentUser } from '../auth'

export default {
  name: 'ChangePassword',
  components: { BrandLogo },
  data() {
    const validateConfirm = (_rule, value, callback) => {
      if (value !== this.form.newPassword) {
        callback(new Error('两次输入的新密码不一致'))
        return
      }
      callback()
    }
    return {
      loading: false,
      form: {
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
      },
      rules: {
        currentPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
        newPassword: [
          { required: true, message: '请输入新密码', trigger: 'blur' },
          { min: 6, message: '新密码至少 6 位', trigger: 'blur' },
        ],
        confirmPassword: [
          { required: true, message: '请确认新密码', trigger: 'blur' },
          { validator: validateConfirm, trigger: 'blur' },
        ],
      },
    }
  },
  methods: {
    submit() {
      this.$refs.form.validate(async (valid) => {
        if (!valid) return
        this.loading = true
        try {
          await changePassword({
            currentPassword: this.form.currentPassword,
            newPassword: this.form.newPassword,
          })
          clearToken()
          resetCurrentUser()
          this.$message.success('密码已修改，请使用新密码重新登录')
          this.$router.replace('/login')
        } catch {
          // 错误已在全局请求拦截器中提示
        } finally {
          this.loading = false
        }
      })
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
.change-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--app-sidebar-bg), var(--app-primary-color));
}

.change-card {
  width: 420px;
}

.change-brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.change-logo {
  --brand-logo-size: 56px;
}

h1 {
  margin: 0;
  text-align: center;
  font-size: 20px;
}

.change-desc {
  margin: 0 0 24px;
  color: var(--app-secondary-text);
  line-height: 1.6;
}

.submit {
  width: 100%;
}

.change-actions {
  margin-top: 16px;
}

.change-actions .el-button {
  width: 100%;
}
</style>
