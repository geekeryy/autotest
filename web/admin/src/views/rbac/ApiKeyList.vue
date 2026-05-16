<template>
  <div class="page-card">
    <div class="page-header">
      <h2 class="page-title">API Key</h2>
      <el-button type="primary" @click="openCreate">新增 API Key</el-button>
    </div>

    <p class="page-hint">
      API Key 归属<strong>当前登录用户</strong>：列表与编辑、重置、删除仅针对您本人创建的 Key，调用时平台以该用户权限校验（scope =
      <code>specs:import</code>）。使用方式：<code>Authorization: Bearer at-...</code>；当前阶段仅允许 OpenAPI/Swagger 导入接口，其余接口拒绝。
    </p>

    <el-card shadow="never" class="example-card">
      <template #header>
        <span class="example-card-title">OpenAPI/Swagger 导入调用示例</span>
      </template>
      <p class="example-intro">
        下方命令中的 <code>YOUR_AT_TOKEN</code> 请替换为「新增」或「重置」后在弹窗里复制的完整 <code>at-...</code>
        令牌（与重置弹窗分离，便于先保存令牌再对照示例）。
      </p>
      <div class="example-selectors">
        <el-select
          v-model="exampleProjectId"
          placeholder="选择项目"
          filterable
          style="flex: 1"
          @change="onExampleProjectChange"
        >
          <el-option v-for="p in exampleProjects" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
        <el-select
          v-model="exampleServiceId"
          placeholder="选择服务"
          filterable
          style="flex: 1"
          :disabled="!exampleProjectId"
        >
          <el-option v-for="s in exampleServices" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
      </div>
      <div class="curl-header">
        <span class="token-tip" style="margin: 0">示例命令：</span>
        <el-radio-group v-model="exampleFormat" size="small">
          <el-radio-button value="json">JSON</el-radio-button>
          <el-radio-button value="yaml">YAML</el-radio-button>
        </el-radio-group>
        <el-button v-if="importCurlExample" type="primary" size="small" link @click="copyImportCurl">复制命令</el-button>
      </div>
      <pre class="token-curl"><code>{{ importCurlExample || '请先选择项目和服务' }}</code></pre>
    </el-card>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-table v-loading="loading" :data="apiKeys" border>
      <el-table-column prop="name" label="备注名" min-width="160" />
      <el-table-column label="Token" min-width="200">
        <template #default="{ row }">
          <code class="token-mask">{{ row.mask }}</code>
        </template>
      </el-table-column>
      <el-table-column label="Scope" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="scope in row.scopes || []" :key="scope" size="small" type="info">{{ scope }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled"
            :loading="row.__toggling"
            @change="(val) => toggleEnabled(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="过期时间" min-width="170">
        <template #default="{ row }">
          <span v-if="row.expiresAt">{{ formatDateTime(row.expiresAt) }}</span>
          <span v-else class="muted">永不过期</span>
        </template>
      </el-table-column>
      <el-table-column label="上次使用" min-width="170">
        <template #default="{ row }">
          <span v-if="row.lastUsedAt">{{ formatDateTime(row.lastUsedAt) }}</span>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" min-width="170">
        <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="warning" link @click="rotate(row)">重置</el-button>
          <el-button type="danger" link @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-empty v-if="!loading && !loadError && !apiKeys.length" description="暂无 API Key">
      <el-button type="primary" @click="openCreate">新增 API Key</el-button>
    </el-empty>

    <!-- 新增 / 编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editing ? '编辑 API Key' : '新增 API Key'"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-form :model="form" label-width="90px">
        <el-form-item label="备注名">
          <el-input v-model="form.name" placeholder="例如：CI 流水线 / 张三本地脚本" />
        </el-form-item>
        <el-form-item v-if="editing" label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expiresAt"
            type="datetime"
            placeholder="留空表示永不过期"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            format="YYYY-MM-DD HH:mm:ss"
            clearable
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 一次性明文 token 展示弹窗（创建与重置）：仅展示令牌，不含调用示例 -->
    <el-dialog
      v-model="tokenDialogVisible"
      :title="tokenDialogTitle"
      width="480px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      @close="onTokenDialogClose"
    >
      <p class="token-warning">⚠ 请立即复制并妥善保存以下令牌。关闭后无法再次查看，丢失只能重新生成。</p>
      <div class="token-box">
        <code class="token-plain">{{ createdToken }}</code>
        <el-button type="primary" size="small" @click="copyToken">复制</el-button>
      </div>
      <p class="token-tip muted">导入接口的 curl 示例见页面上方「OpenAPI/Swagger 导入调用示例」。</p>
      <template #footer>
        <el-button type="primary" @click="tokenDialogVisible = false">我已复制保存，关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { ElMessage, ElMessageBox } from 'element-plus'

import { createApiKey, deleteApiKey, listApiKeys, listProjects, listServices, rotateApiKey, updateApiKey } from '../../api'
import ListLoadError from '../../components/ListLoadError.vue'
import { formatDateTime } from '../../utils/datetime'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'

export default {
  name: 'ApiKeyList',
  components: { ListLoadError },
  data() {
    return {
      loading: false,
      loadError: '',
      submitting: false,
      apiKeys: [],
      dialogVisible: false,
      editing: null,
      form: this.emptyForm(),
      tokenDialogVisible: false,
      tokenDialogTitle: 'API Key 创建成功',
      createdToken: '',
      exampleProjects: [],
      exampleServices: [],
      exampleProjectId: '',
      exampleServiceId: '',
      exampleFormat: 'json'
    }
  },
  created() {
    this.load()
    this.loadExampleProjects()
  },
  computed: {
    importCurlExample() {
      if (!this.exampleProjectId || !this.exampleServiceId) return ''
      const origin = window.location.origin
      const isJson = this.exampleFormat === 'json'
      const contentType = isJson ? 'application/json' : 'application/yaml'
      const fileName = isJson ? 'openapi.json' : 'openapi.yaml'
      return [
        `curl -X POST "${origin}/api/v1/projects/${this.exampleProjectId}/services/${this.exampleServiceId}/specs/import" \\`,
        `  -H "Authorization: Bearer YOUR_AT_TOKEN" \\`,
        `  -H "Content-Type: ${contentType}" \\`,
        `  -d @${fileName}`
      ].join('\n')
    }
  },
  methods: {
    formatDateTime,
    emptyForm() {
      return { name: '', enabled: true, expiresAt: null }
    },
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        this.apiKeys = await listApiKeys()
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
        name: row.name,
        enabled: !!row.enabled,
        expiresAt: row.expiresAt || null
      }
      this.dialogVisible = true
    },
    async submit() {
      if (!this.form.name || !this.form.name.trim()) {
        ElMessage.warning('请填写备注名')
        return
      }
      this.submitting = true
      try {
        if (this.editing) {
          const payload = {
            name: this.form.name.trim(),
            enabled: this.form.enabled
          }
          if (this.form.expiresAt) {
            payload.expiresAt = this.form.expiresAt
          } else if (this.editing.expiresAt) {
            // 用户清空了原本的过期时间：必须显式 clearExpiresAt，
            // 后端 PATCH 把 expiresAt 缺省/null 当作"不变更"
            payload.clearExpiresAt = true
          }
          await updateApiKey(this.editing.id, payload)
          ElMessage.success('API Key 已更新')
          this.dialogVisible = false
          await this.load()
        } else {
          const created = await createApiKey({
            name: this.form.name.trim(),
            expiresAt: this.form.expiresAt || null
          })
          this.dialogVisible = false
          const plain = created?.token || ''
          if (plain) {
            this.tokenDialogTitle = 'API Key 创建成功'
            this.createdToken = plain
            this.tokenDialogVisible = true
          } else {
            // 兜底：后端未返回明文（理论上不会发生）也刷新列表
            ElMessage.success('API Key 已创建')
            await this.load()
          }
        }
      } finally {
        this.submitting = false
      }
    },
    async toggleEnabled(row, value) {
      row.__toggling = true
      try {
        await updateApiKey(row.id, { enabled: value })
        row.enabled = value
        ElMessage.success(value ? '已启用' : '已禁用')
      } catch {
        // request 拦截器已弹错误；这里回滚开关，避免界面与后端不一致
        await this.load()
      } finally {
        row.__toggling = false
      }
    },
    async rotate(row) {
      try {
        await ElMessageBox.confirm(
          '重置后原 token 将立即失效，使用该 API Key 的所有 CI/CD 调用都需要更新。是否继续？',
          `重置 API Key「${row.name}」`,
          {
            confirmButtonText: '重置',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      } catch {
        return
      }
      const result = await rotateApiKey(row.id)
      const plain = result?.token || ''
      if (plain) {
        this.tokenDialogTitle = 'API Key 已重置'
        this.createdToken = plain
        this.tokenDialogVisible = true
      } else {
        ElMessage.success('API Key 已重置')
        await this.load()
      }
    },
    async remove(row) {
      try {
        await ElMessageBox.confirm(`确认删除 API Key「${row.name}」？删除后使用该令牌的调用将失败。`, '删除确认', {
          confirmButtonText: '删除',
          cancelButtonText: '取消',
          type: 'warning'
        })
      } catch {
        return
      }
      await deleteApiKey(row.id)
      ElMessage.success('API Key 已删除')
      await this.load()
    },
    async copyToken() {
      const text = this.createdToken
      if (!text) {
        return
      }
      // 优先走 Clipboard API；非安全上下文（非 HTTPS 且非 localhost）或权限被拒时
      // 回退到临时 textarea + execCommand('copy')，最大限度兼容内网部署
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text)
          ElMessage.success('已复制到剪贴板')
          return
        }
      } catch {
        // 落入下面的兜底
      }
      if (this.legacyCopy(text)) {
        ElMessage.success('已复制到剪贴板')
      } else {
        ElMessage.error('复制失败，请手动选中复制')
      }
    },
    legacyCopy(text) {
      try {
        const ta = document.createElement('textarea')
        ta.value = text
        ta.setAttribute('readonly', '')
        ta.style.position = 'fixed'
        ta.style.top = '-1000px'
        ta.style.opacity = '0'
        document.body.appendChild(ta)
        ta.focus()
        ta.select()
        ta.setSelectionRange(0, text.length)
        const ok = document.execCommand('copy')
        document.body.removeChild(ta)
        return ok
      } catch {
        return false
      }
    },
    async loadExampleProjects() {
      try {
        this.exampleProjects = await listProjects()
        if (this.exampleProjects.length > 0) {
          this.exampleProjectId = this.exampleProjects[0].id
          await this.loadExampleServices(this.exampleProjectId)
        }
      } catch {
        // 静默失败，不影响主流程
      }
    },
    async loadExampleServices(projectId) {
      this.exampleServiceId = ''
      this.exampleServices = []
      if (!projectId) return
      try {
        this.exampleServices = await listServices(projectId)
        if (this.exampleServices.length > 0) {
          this.exampleServiceId = this.exampleServices[0].id
        }
      } catch {
        // 静默失败
      }
    },
    async onExampleProjectChange(projectId) {
      await this.loadExampleServices(projectId)
    },
    async copyImportCurl() {
      const text = this.importCurlExample
      if (!text) return
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text)
          ElMessage.success('已复制到剪贴板')
          return
        }
      } catch {
        // 落入兜底
      }
      if (this.legacyCopy(text)) {
        ElMessage.success('已复制到剪贴板')
      } else {
        ElMessage.error('复制失败，请手动选中复制')
      }
    },
    onTokenDialogClose() {
      this.createdToken = ''
      this.load()
    }
  }
}
</script>

<style scoped>
.page-hint {
  margin: 0 0 12px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.6;
}

.example-card {
  margin-bottom: 16px;
  border: 1px solid var(--app-border-color);
}

.example-card :deep(.el-card__header) {
  padding: 10px 16px;
}

.example-card-title {
  font-weight: 600;
  font-size: var(--app-font-size-base);
}

.example-intro {
  margin: 0 0 12px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.6;
}

.example-intro code {
  background: var(--el-fill-color-light);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
}

.page-hint code {
  background: var(--el-fill-color-light);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
}

.token-mask {
  font-family: var(--el-font-family-monospace, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.muted {
  color: var(--el-text-color-placeholder);
}

.el-tag {
  margin-right: 4px;
  margin-bottom: 2px;
}

.token-warning {
  color: #f56c6c;
  font-weight: 600;
  margin: 0 0 12px;
  line-height: 1.6;
}

.token-box {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--el-fill-color-light);
  border: 1px solid var(--app-border-color);
  border-radius: 4px;
  padding: 10px 12px;
  margin-bottom: 12px;
}

.token-plain {
  flex: 1;
  font-family: var(--el-font-family-monospace, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 14px;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.token-tip {
  margin: 12px 0 6px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.token-curl {
  margin: 0;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}

.example-selectors {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.curl-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
