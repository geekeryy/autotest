<template>
  <div class="login-creds">
    <p class="login-creds__intro">
      登录凭据不会写入平台配置。可从已保存用例/环境配置中选择候选，或手动填写；留空则生成时使用占位符，需后续补全。
    </p>
    <el-alert v-if="loadError" type="warning" :closable="false" :title="loadError" class="login-creds__alert" />
    <div v-if="endpointHints.length" class="login-creds__hints">
      <div class="login-creds__hint-title">发现的登录接口</div>
      <div v-for="hint in endpointHints" :key="hintKey(hint)" class="login-creds__hint-row">
        <span class="login-creds__hint-path">{{ hint.method }} {{ hint.path }}</span>
        <span v-if="hint.username" class="login-creds__hint-meta">用户 {{ hint.username }}</span>
        <span v-if="hint.passwordHint" class="login-creds__hint-meta">密码 {{ hint.passwordHint }}</span>
        <el-button
          v-if="hint.username || (hint.bodyFields && hint.bodyFields.length)"
          link
          type="primary"
          size="small"
          @click="applyHint(hint)"
        >
          填入{{ hint.role === 'admin' ? ' Admin' : '' }}凭据
        </el-button>
      </div>
    </div>
    <el-form label-width="108px" label-position="left" class="login-creds__form">
      <template v-for="section in sections" :key="section.role">
        <div class="login-creds__section-title">{{ section.title }}</div>
        <template v-if="section.showClassic">
          <el-form-item label="用户名">
            <el-input v-model="local[section.role].username" placeholder="可选" clearable :disabled="disabled" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="local[section.role].password"
              type="password"
              show-password
              placeholder="可选"
              :disabled="disabled"
            />
          </el-form-item>
        </template>
        <el-form-item
          v-for="field in section.bodyFields"
          :key="`${section.role}-${field.name}`"
          :label="field.label || field.name"
          :required="field.required"
        >
          <div v-if="field.enum && field.enum.length" class="login-creds__field-stack">
            <div class="login-creds__chips" role="group" :aria-label="field.label || field.name">
              <button
                v-for="opt in field.enum"
                :key="String(opt)"
                type="button"
                class="login-creds__chip"
                :class="{ 'login-creds__chip--active': isChipActive(section.role, field.name, opt) }"
                :disabled="disabled"
                @click="selectChip(section.role, field.name, opt)"
              >
                {{ opt }}
              </button>
            </div>
            <el-select
              v-if="field.enum.length > 4"
              v-model="local[section.role].body[field.name]"
              :placeholder="field.needsInput ? '或从列表选择' : '可选'"
              clearable
              filterable
              :disabled="disabled"
              class="login-creds__field"
            >
              <el-option v-for="opt in field.enum" :key="String(opt)" :label="String(opt)" :value="normalizeEnumValue(opt)" />
            </el-select>
          </div>
          <el-input
            v-else-if="field.type === 'integer' || field.type === 'number'"
            v-model.number="local[section.role].body[field.name]"
            type="number"
            :placeholder="field.needsInput ? '必填' : '可选'"
            clearable
            :disabled="disabled"
          />
          <el-input
            v-else
            v-model="local[section.role].body[field.name]"
            :type="field.sensitive ? 'password' : 'text'"
            :show-password="field.sensitive"
            :placeholder="field.needsInput ? '必填' : '可选'"
            clearable
            :disabled="disabled"
          />
        </el-form-item>
      </template>
    </el-form>
  </div>
</template>

<script>
import { getScenarioLoginHints } from '../api'

const blankPair = () => ({ username: '', password: '', body: {} })

function fieldsForRole(hints, role) {
  const hint = (hints || []).find((h) => h.role === role && h.path)
  const fields = hint?.bodyFields || []
  const names = new Set(fields.map((f) => f.name))
  const showClassic = fields.length === 0 || names.has('username') || names.has('password') || names.has('user')
  return { fields, showClassic }
}

export default {
  name: 'AIScenarioLoginCredentials',
  props: {
    projectId: { type: String, default: '' },
    serviceId: { type: String, default: '' },
    environmentId: { type: String, default: '' },
    modelValue: { type: Object, default: () => ({ user: blankPair(), admin: blankPair() }) },
    disabled: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      local: {
        user: blankPair(),
        admin: blankPair(),
      },
      hints: [],
      loadError: '',
    }
  },
  computed: {
    endpointHints() {
      return (this.hints || []).filter((h) => h.path)
    },
    sections() {
      const userMeta = fieldsForRole(this.endpointHints, 'user')
      const adminMeta = fieldsForRole(this.endpointHints, 'admin')
      const out = [
        {
          role: 'user',
          title: '普通用户登录',
          bodyFields: userMeta.fields.filter((f) => !isClassicField(f.name)),
          showClassic: userMeta.showClassic,
        },
      ]
      if (this.endpointHints.some((h) => h.role === 'admin')) {
        out.push({
          role: 'admin',
          title: 'Admin 登录',
          bodyFields: adminMeta.fields.filter((f) => !isClassicField(f.name)),
          showClassic: adminMeta.showClassic,
        })
      }
      return out
    },
  },
  watch: {
    modelValue: {
      immediate: true,
      deep: true,
      handler(v) {
        this.local = {
          user: { ...blankPair(), ...(v?.user || {}), body: { ...(v?.user?.body || {}) } },
          admin: { ...blankPair(), ...(v?.admin || {}), body: { ...(v?.admin?.body || {}) } },
        }
      },
    },
    local: {
      deep: true,
      handler(v) {
        this.$emit('update:modelValue', this.buildPayload(v))
      },
    },
    serviceId: {
      immediate: true,
      handler() {
        this.loadHints()
      },
    },
    environmentId() {
      this.loadHints()
    },
  },
  methods: {
    hintKey(hint) {
      return `${hint.role || 'user'}-${hint.path || hint.profileName || hint.variableName}`
    },
    normalizeEnumValue(opt) {
      return typeof opt === 'number' ? opt : String(opt)
    },
    isChipActive(role, fieldName, opt) {
      const val = this.local[role]?.body?.[fieldName]
      return val === this.normalizeEnumValue(opt)
    },
    selectChip(role, fieldName, opt) {
      if (!this.local[role].body) this.local[role].body = {}
      this.local[role].body[fieldName] = this.normalizeEnumValue(opt)
    },
    buildPayload(v) {
      const out = {}
      for (const role of ['user', 'admin']) {
        const pair = v[role]
        if (!pair) continue
        const body = {}
        for (const [k, val] of Object.entries(pair.body || {})) {
          if (val !== '' && val != null) body[k] = val
        }
        const hasBody = Object.keys(body).length > 0
        const hasClassic = pair.username && pair.password
        if (hasClassic || hasBody) {
          out[role] = {
            ...(pair.username ? { username: pair.username } : {}),
            ...(pair.password ? { password: pair.password } : {}),
            ...(hasBody ? { body } : {}),
          }
        }
      }
      return out
    },
    applyHint(hint) {
      const target = hint.role === 'admin' ? this.local.admin : this.local.user
      if (hint.username) target.username = hint.username
      for (const field of hint.bodyFields || []) {
        if (field.currentValue != null && field.currentValue !== '' && !String(field.currentValue).startsWith('__FILL_')) {
          target.body[field.name] = field.enum?.length ? this.normalizeEnumValue(field.currentValue) : field.currentValue
        }
      }
    },
    async loadHints() {
      this.loadError = ''
      this.hints = []
      if (!this.projectId || !this.serviceId) return
      try {
        const res = await getScenarioLoginHints(this.projectId, this.serviceId, this.environmentId)
        this.hints = res.hints || []
      } catch (err) {
        this.loadError = err?.response?.data?.error || err.message || '加载登录凭据候选失败'
      }
    },
  },
}

function isClassicField(name) {
  const lower = String(name || '').toLowerCase()
  return ['username', 'user', 'email', 'login', 'password', 'pass', 'pwd'].includes(lower)
}
</script>

<style scoped>
.login-creds__intro {
  margin: 0 0 10px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--app-text-muted);
}
.login-creds__alert {
  margin-bottom: 10px;
}
.login-creds__hints {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
}
.login-creds__hint-title,
.login-creds__section-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}
.login-creds__hint-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  margin-bottom: 6px;
}
.login-creds__hint-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.login-creds__hint-meta {
  color: var(--app-text-muted);
}
.login-creds__section-title {
  margin-top: 4px;
}
.login-creds__field {
  width: 100%;
}
.login-creds__field-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.login-creds__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.login-creds__chip {
  border: 1px solid var(--app-border-color);
  background: var(--el-fill-color-blank, #fff);
  color: var(--app-text-color);
  border-radius: 999px;
  padding: 4px 12px;
  font-size: 13px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
}
.login-creds__chip:hover:not(:disabled) {
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
}
.login-creds__chip--active {
  border-color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 12%, transparent);
  color: var(--el-color-primary);
  font-weight: 600;
}
.login-creds__chip:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
