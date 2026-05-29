<!--
  AIScenarioGenConfirm
  ====================
  Rich confirm UI for generate_and_verify_scenarios / generate_coverage_scenarios pending tool calls.
  Lets the user review target environment (verify only), auto-repair toggle, login body fields,
  and max rounds before approving execution.
-->
<template>
  <div class="gen-confirm" :class="{ 'gen-confirm--inline': inline }">
    <div class="gen-confirm__head">
      <el-icon class="gen-confirm__warn"><Warning /></el-icon>
      <div>
        <div class="gen-confirm__title">{{ confirmTitle }}</div>
        <p class="gen-confirm__desc">
          <template v-if="isVerifyMode">
            将对<strong>真实环境</strong>执行测试并可能产生或删除数据。请确认目标环境与闭环配置后再允许执行。
          </template>
          <template v-else>
            将生成覆盖当前服务的测试场景。请确认目标服务，并在下方填写登录接口所需参数后再允许执行。
          </template>
        </p>
      </div>
    </div>

    <el-form label-width="96px" label-position="left" class="gen-confirm__form" @submit.prevent>
      <el-form-item label="目标服务" required>
        <el-select
          v-model="form.serviceId"
          filterable
          placeholder="选择服务"
          :loading="loadingServices"
          :disabled="disabled || busy"
          class="gen-confirm__field"
          @change="onServiceChange"
        >
          <el-option v-for="svc in services" :key="svc.id" :label="svc.name" :value="svc.id" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="isVerifyMode" label="目标环境" required>
        <el-select
          v-model="form.environmentId"
          filterable
          placeholder="选择环境"
          :loading="loadingEnvironments"
          :disabled="disabled || busy || !form.serviceId"
          class="gen-confirm__field"
        >
          <el-option v-for="env in environments" :key="env.id" :label="envLabel(env)" :value="env.id" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="isVerifyMode" label="自动修复">
        <div class="gen-confirm__switch-row">
          <el-switch v-model="form.autoRepair" :disabled="disabled || busy" />
          <span class="gen-confirm__switch-hint">开启后将在真实环境执行失败分析并自动修复场景</span>
        </div>
      </el-form-item>
      <el-form-item v-if="isVerifyMode" label="最大轮次">
        <el-input-number
          v-model="form.maxRounds"
          :min="1"
          :max="10"
          :disabled="disabled || busy || !form.autoRepair"
        />
        <span class="gen-confirm__rounds-hint">执行→修复闭环的最大迭代次数</span>
      </el-form-item>
      <AIScenarioLoginCredentials
        v-model="form.loginCredentials"
        :project-id="projectId"
        :service-id="form.serviceId"
        :environment-id="form.environmentId"
        :disabled="disabled || busy"
      />
    </el-form>

    <details v-if="prettyArgs" class="gen-confirm__args">
      <summary>模型提议的参数</summary>
      <pre>{{ prettyArgs }}</pre>
    </details>

    <div v-if="validationError" class="gen-confirm__error">{{ validationError }}</div>

    <div class="gen-confirm__actions">
      <el-button v-if="!rejectExpanded" :disabled="disabled || busy" size="small" @click="rejectExpanded = true">
        拒绝
      </el-button>
      <template v-else>
        <el-input
          v-model="reason"
          type="textarea"
          :rows="2"
          placeholder="说明拒绝原因（可选）"
          class="gen-confirm__reason"
        />
        <div class="gen-confirm__reject-actions">
          <el-button size="small" @click="onCancelReject">取消</el-button>
          <el-button size="small" type="danger" :loading="busy" :disabled="disabled" @click="onReject">
            确认拒绝
          </el-button>
        </div>
      </template>
      <el-button
        size="small"
        type="primary"
        :loading="busy"
        :disabled="disabled || !canApprove"
        @click="onApprove"
      >
        允许执行
      </el-button>
    </div>
  </div>
</template>

<script>
import { Warning } from '@element-plus/icons-vue'
import AIScenarioLoginCredentials from './AIScenarioLoginCredentials.vue'
import { listServiceEnvironments, listServices } from '../api'
import { projectState } from '../utils/currentProject'

function parseArgs(raw) {
  if (raw == null) return {}
  try {
    return typeof raw === 'string' ? JSON.parse(raw) : { ...raw }
  } catch {
    return {}
  }
}

export default {
  name: 'AIScenarioGenConfirm',
  components: { Warning, AIScenarioLoginCredentials },
  props: {
    call: { type: Object, required: true },
    disabled: { type: Boolean, default: false },
    inline: { type: Boolean, default: false },
  },
  emits: ['decide'],
  data() {
    return {
      services: [],
      environments: [],
      loadingServices: false,
      loadingEnvironments: false,
      rejectExpanded: false,
      reason: '',
      busy: false,
      form: {
        serviceId: '',
        environmentId: '',
        autoRepair: false,
        maxRounds: 3,
        loginCredentials: {},
      },
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    parsedArgs() {
      return parseArgs(this.call?.arguments)
    },
    toolName() {
      return String(this.call?.name || '')
    },
    isVerifyMode() {
      return this.toolName === 'generate_and_verify_scenarios'
    },
    confirmTitle() {
      return this.isVerifyMode ? '生成并验证测试场景' : '生成覆盖测试场景'
    },
    prettyArgs() {
      const args = this.parsedArgs
      if (!Object.keys(args).length) return ''
      return JSON.stringify(args, null, 2)
    },
    validationError() {
      if (!this.form.serviceId) return '请选择目标服务'
      if (this.isVerifyMode && !this.form.environmentId) return '请选择目标环境'
      return ''
    },
    canApprove() {
      return !this.validationError
    },
  },
  watch: {
    call: {
      immediate: true,
      handler() {
        this.syncFromCall()
      },
    },
    projectId: {
      immediate: true,
      handler(id) {
        if (id) this.loadServices()
      },
    },
  },
  methods: {
    syncFromCall() {
      const args = this.parsedArgs
      this.form.serviceId = String(args.serviceId || args.service_id || '')
      this.form.environmentId = String(args.environmentId || args.environment_id || '')
      this.form.autoRepair = args.autoRepair ?? args.auto_repair ?? false
      const rounds = Number(args.maxRounds ?? args.max_rounds)
      this.form.maxRounds = Number.isFinite(rounds) && rounds > 0 ? rounds : 3
      this.form.loginCredentials = args.loginCredentials || {}
      if (this.form.serviceId) this.loadEnvironments(this.form.serviceId)
    },
    envLabel(env) {
      const base = env.name || env.id
      const url = env.baseUrl || env.base_url
      return url ? `${base} (${url})` : base
    },
    async loadServices() {
      if (!this.projectId) return
      this.loadingServices = true
      try {
        this.services = await listServices(this.projectId)
      } catch {
        this.services = []
      } finally {
        this.loadingServices = false
      }
    },
    async loadEnvironments(serviceId) {
      if (!this.projectId || !serviceId) {
        this.environments = []
        return
      }
      this.loadingEnvironments = true
      try {
        this.environments = await listServiceEnvironments(this.projectId, serviceId)
      } catch {
        this.environments = []
      } finally {
        this.loadingEnvironments = false
      }
    },
    onServiceChange(serviceId) {
      this.form.environmentId = ''
      this.loadEnvironments(serviceId)
    },
    onCancelReject() {
      this.rejectExpanded = false
      this.reason = ''
    },
    buildMergedArgs() {
      const base = {
        ...this.parsedArgs,
        serviceId: this.form.serviceId,
        loginCredentials: this.form.loginCredentials,
      }
      if (this.isVerifyMode) {
        return {
          ...base,
          environmentId: this.form.environmentId,
          autoRepair: !!this.form.autoRepair,
          maxRounds: this.form.autoRepair ? this.form.maxRounds : 0,
        }
      }
      return base
    },
    async onApprove() {
      if (!this.canApprove) return
      this.busy = true
      try {
        this.$emit('decide', {
          callId: this.call.id,
          approve: true,
          reason: '',
          argumentOverrides: this.buildMergedArgs(),
        })
      } finally {
        this.busy = false
      }
    },
    async onReject() {
      this.busy = true
      try {
        this.$emit('decide', {
          callId: this.call.id,
          approve: false,
          reason: this.reason,
        })
      } finally {
        this.busy = false
      }
    },
  },
}
</script>

<style scoped>
.gen-confirm {
  background: var(--el-fill-color-blank, #fff);
  border: 1px solid color-mix(in srgb, var(--el-color-warning) 55%, var(--app-border-color));
  border-radius: 12px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.gen-confirm--inline {
  border-color: var(--app-border-color);
  box-shadow: 0 1px 0 rgba(15, 23, 42, 0.04);
  background: var(--el-fill-color-blank, #fff);
}

.gen-confirm__head {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.gen-confirm__warn {
  flex-shrink: 0;
  font-size: 20px;
  color: var(--el-color-warning);
  margin-top: 2px;
}

.gen-confirm__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-color);
}

.gen-confirm__desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--app-secondary-text);
}

.gen-confirm__form {
  margin: 0;
}

.gen-confirm__field {
  width: 100%;
}

.gen-confirm__switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.gen-confirm__switch-hint,
.gen-confirm__rounds-hint {
  font-size: 12px;
  color: var(--app-text-muted);
  margin-left: 8px;
}

.gen-confirm__args {
  font-size: 12px;
  color: var(--app-secondary-text);
}

.gen-confirm__args pre {
  margin: 6px 0 0;
  padding: 8px;
  border-radius: 6px;
  background: var(--el-fill-color-light);
  overflow: auto;
  max-height: 160px;
}

.gen-confirm__error {
  font-size: 13px;
  color: var(--el-color-danger);
}

.gen-confirm__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: flex-end;
  gap: 8px;
}

.gen-confirm__reason {
  width: 100%;
}

.gen-confirm__reject-actions {
  display: flex;
  gap: 6px;
}
</style>
