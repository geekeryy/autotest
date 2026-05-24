<template>
  <div :class="embedded ? 'tab-panel' : 'page-card'">
    <div v-if="embedded" class="tab-panel-toolbar">
      <p class="tab-panel-hint">
        配置 GitHub、GitLab、Google 等 OAuth 登录方式；本地用户名密码登录始终可用。
      </p>
      <el-button v-if="canWrite" type="primary" @click="openCreate">新增登录方式</el-button>
    </div>
    <div v-else class="page-header auth-provider-header">
      <div>
        <h2 class="page-title">登录方式</h2>
        <p class="page-subtitle">
          配置 GitHub、GitLab、Google 等 OAuth 登录方式；本地用户名密码登录始终可用。
        </p>
      </div>
      <el-button v-if="canWrite" type="primary" @click="openCreate">新增登录方式</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="loadList" />

    <el-table :data="providers" border row-key="id" v-loading="loading">
      <el-table-column label="类型" width="120">
        <template #default="{ row }">
          <el-tag size="small">{{ typeLabel(row.providerType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="slug" label="Slug" width="140" show-overflow-tooltip />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.enabled" type="success" size="small">启用</el-tag>
          <el-tag v-else type="info" size="small">停用</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Client Secret" width="180">
        <template #default="{ row }">
          <span v-if="row.clientSecretMasked" class="auth-secret">{{ row.clientSecretMasked }}</span>
          <el-tag v-else type="info" size="small">未设置</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="回调地址" min-width="280" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ liveCallbackUrl(row) }}</span>
          <el-tag v-if="row.callbackUrl && row.callbackUrl !== liveCallbackUrl(row)" type="warning" size="small"
            class="callback-stale-tag">
            未保存
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="可信前端域名" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.trustedFrontendOrigins?.length">{{ row.trustedFrontendOrigins.join(', ') }}</span>
          <el-tag v-else type="info" size="small">未配置</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="178">
        <template #default="{ row }">{{ formatDateTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <template v-if="canWrite">
            <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
            <el-button type="primary" link :loading="testingId === row.id" @click="test(row)">测试</el-button>
            <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
          </template>
          <span v-else class="text-secondary">只读</span>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !loadError && !providers.length" description="暂无 OAuth 登录方式，点击右上角「新增登录方式」创建" />

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="640px" align-center destroy-on-close>
      <el-form :model="form" label-width="130px">
        <el-form-item label="类型" required>
          <el-select v-model="form.providerType" placeholder="选择登录方式类型" style="width: 100%" :disabled="!!editingId"
            @change="onTypeChange">
            <el-option v-for="t in providerTypes" :key="t.type" :label="t.displayName || t.type" :value="t.type" />
          </el-select>
        </el-form-item>
        <el-form-item label="Slug" required>
          <el-input v-model="form.slug" placeholder="例如：github、company-github" />
          <p class="meta-notes">公开标识，用于 OAuth 登录/回调 URL；推荐 OAuth App Name</p>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如：公司 GitHub" />
        </el-form-item>
        <el-form-item label="Client ID" required>
          <el-input v-model="form.clientId" placeholder="OAuth Client ID" />
        </el-form-item>
        <el-form-item label="Client Secret" :required="!editingId">
          <el-input v-model="form.clientSecret" type="password" show-password
            :placeholder="editingId ? '留空则保持原 Secret 不变' : '请输入 Client Secret'" autocomplete="new-password" />
        </el-form-item>
        <el-form-item v-if="form.providerType === 'gitlab'" label="GitLab 地址" required>
          <el-input v-model="form.baseUrl" placeholder="https://gitlab.com" />
        </el-form-item>
        <el-form-item v-if="previewCallbackUrl" label="回调地址">
          <div class="callback-row">
            <el-input :model-value="previewCallbackUrl" readonly />
            <el-button @click="copyCallbackUrl">复制</el-button>
          </div>
          <p class="meta-notes">请在 IdP 控制台注册此回调 URL；保存时将写入数据库</p>
        </el-form-item>
        <el-form-item label="自动注册">
          <el-switch v-model="form.autoRegister" />
          <span class="meta-hint">首次 OAuth 登录时自动创建平台用户</span>
        </el-form-item>
        <el-form-item label="默认启用">
          <el-switch v-model="form.defaultActive" />
          <span class="meta-hint">新用户默认是否启用（关闭则待管理员审核）</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
          <span class="meta-hint">是否在登录页展示此 OAuth 按钮</span>
        </el-form-item>
        <el-form-item label="可信前端域名">
          <el-select v-model="form.trustedFrontendOrigins" multiple filterable allow-create default-first-option
            placeholder="输入后回车添加，例如 https://app.example.com" style="width: 100%" />
          <p class="meta-notes">
            仅用于 OAuth 登录成功后的前端回跳地址（frontendUrl）校验；完整 scheme+host+port，无路径。
            浏览器跨域访问 API 请在服务端配置 <code>CORS_ALLOWED_ORIGINS</code> 环境变量。
            示例：<code>https://app.example.com</code>、<code>http://localhost:8080</code>
          </p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createAuthProvider,
  deleteAuthProvider,
  listAuthProviderTypes,
  listAuthProviders,
  testAuthProvider,
  updateAuthProvider
} from '../../api'
import { hasPermission } from '../../auth'
import ListLoadError from '../../components/ListLoadError.vue'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'
import { oauthCallbackUrl } from '../../utils/apiBase'

function pad2(n) {
  return String(n).padStart(2, '0')
}

const blankForm = () => ({
  providerType: '',
  slug: '',
  name: '',
  clientId: '',
  clientSecret: '',
  baseUrl: 'https://gitlab.com',
  autoRegister: true,
  defaultActive: false,
  enabled: true,
  trustedFrontendOrigins: []
})

export default {
  name: 'AuthProviderList',
  components: { ListLoadError },
  props: {
    embedded: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      loading: false,
      loadError: '',
      providers: [],
      providerTypes: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      testingId: null,
      deletingId: null,
      form: blankForm()
    }
  },
  computed: {
    canWrite() {
      return hasPermission('users:manage')
    },
    dialogTitle() {
      return this.editingId ? '编辑登录方式' : '新增登录方式'
    },
    previewCallbackUrl() {
      return oauthCallbackUrl(this.form.slug)
    }
  },
  async created() {
    try {
      this.providerTypes = await listAuthProviderTypes()
    } catch (_) {
      this.providerTypes = []
    }
    await this.loadList()
  },
  methods: {
    typeLabel(type) {
      const meta = this.providerTypes.find((t) => t.type === type)
      return meta?.displayName || type
    },
    liveCallbackUrl(row) {
      const live = oauthCallbackUrl(row?.slug)
      return live || row?.callbackUrl || ''
    },
    formatDateTime(iso) {
      if (!iso) return ''
      const d = new Date(iso)
      if (Number.isNaN(d.getTime())) return String(iso)
      return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
    },
    async loadList() {
      this.loading = true
      this.loadError = ''
      try {
        this.providers = await listAuthProviders()
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err)
      } finally {
        this.loading = false
      }
    },
    onTypeChange(value) {
      const meta = this.providerTypes.find((t) => t.type === value)
      if (!this.editingId && value && !this.form.slug) {
        this.form.slug = value
      }
      if (value === 'gitlab' && meta?.extraConfigFields) {
        const baseField = meta.extraConfigFields.find((f) => f.key === 'baseUrl')
        if (baseField?.defaultValue && !this.form.baseUrl) {
          this.form.baseUrl = baseField.defaultValue
        }
      }
    },
    openCreate() {
      this.editingId = null
      this.form = blankForm()
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editingId = row.id
      const extra = row.extraConfig || {}
      this.form = {
        providerType: row.providerType,
        slug: row.slug || '',
        name: row.name,
        clientId: row.clientId || '',
        clientSecret: '',
        baseUrl: extra.baseUrl || 'https://gitlab.com',
        autoRegister: row.autoRegister !== false,
        defaultActive: !!row.defaultActive,
        enabled: row.enabled !== false,
        trustedFrontendOrigins: Array.isArray(row.trustedFrontendOrigins)
          ? [...row.trustedFrontendOrigins]
          : []
      }
      this.dialogVisible = true
    },
    buildExtraConfig() {
      if (this.form.providerType === 'gitlab') {
        return { baseUrl: (this.form.baseUrl || '').trim() || 'https://gitlab.com' }
      }
      return {}
    },
    async copyCallbackUrl() {
      const url = (this.previewCallbackUrl || '').trim()
      if (!url) return
      try {
        await navigator.clipboard.writeText(url)
        this.$message.success('已复制回调地址')
      } catch {
        this.$message.error('复制失败，请手动选择复制')
      }
    },
    async submit() {
      const name = (this.form.name || '').trim()
      const slug = (this.form.slug || '').trim().toLowerCase()
      if (!this.form.providerType) {
        this.$message.warning('请选择登录方式类型')
        return
      }
      if (!slug) {
        this.$message.warning('请填写 Slug')
        return
      }
      if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(slug)) {
        this.$message.warning('Slug 仅允许小写字母、数字与连字符，且不能以连字符开头或结尾')
        return
      }
      if (!name) {
        this.$message.warning('请填写名称')
        return
      }
      const clientId = (this.form.clientId || '').trim()
      if (!clientId) {
        this.$message.warning('请填写 Client ID')
        return
      }
      if (!this.editingId && !(this.form.clientSecret || '').trim()) {
        this.$message.warning('请填写 Client Secret')
        return
      }
      if (this.form.providerType === 'gitlab' && !(this.form.baseUrl || '').trim()) {
        this.$message.warning('请填写 GitLab 地址')
        return
      }

      const payload = {
        slug,
        name,
        providerType: this.form.providerType,
        clientId,
        callbackUrl: oauthCallbackUrl(slug),
        extraConfig: this.buildExtraConfig(),
        autoRegister: !!this.form.autoRegister,
        defaultActive: !!this.form.defaultActive,
        enabled: !!this.form.enabled,
        trustedFrontendOrigins: (this.form.trustedFrontendOrigins || [])
          .map((item) => String(item || '').trim())
          .filter(Boolean)
      }

      this.saving = true
      try {
        if (this.editingId) {
          const secret = (this.form.clientSecret || '').trim()
          if (secret) {
            payload.clientSecret = secret
          }
          await updateAuthProvider(this.editingId, payload)
          this.$message.success('已保存')
        } else {
          payload.clientSecret = (this.form.clientSecret || '').trim()
          await createAuthProvider(payload)
          this.$message.success('已创建')
        }
        this.dialogVisible = false
        await this.loadList()
      } finally {
        this.saving = false
      }
    },
    async test(row) {
      this.testingId = row.id
      try {
        const res = await testAuthProvider(row.id)
        const msg = res?.message || res?.authUrl || '配置校验通过'
        this.$message.success(typeof msg === 'string' ? msg : '连接测试通过')
      } finally {
        this.testingId = null
      }
    },
    async remove(row) {
      try {
        await this.$confirm(`确定删除登录方式「${row.name}」？`, '删除确认', { type: 'warning' })
      } catch {
        return
      }
      this.deletingId = row.id
      try {
        await deleteAuthProvider(row.id)
        this.$message.success('已删除')
        await this.loadList()
      } finally {
        this.deletingId = null
      }
    }
  }
}
</script>

<style scoped>
.auth-provider-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.auth-secret {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: var(--app-font-size-small);
  letter-spacing: 0.4px;
}

.callback-stale-tag {
  margin-left: 6px;
  vertical-align: middle;
}

.meta-notes {
  margin-top: 4px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.4;
}

.meta-hint {
  margin-left: 8px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.callback-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.callback-row .el-input {
  flex: 1;
  min-width: 0;
}

.tab-panel-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.tab-panel-hint {
  margin: 0;
  flex: 1;
  min-width: 240px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}
</style>
