<!--
  AIScenarioGenLauncher
  =====================
  Inline config panel for proactively starting scenario generation from AI home.
-->
<template>
  <div class="gen-launcher">
    <div class="gen-launcher__head">
      <span class="gen-launcher__title">生成并验证测试场景</span>
      <button type="button" class="gen-launcher__close" aria-label="关闭" @click="$emit('close')">
        <el-icon><Close /></el-icon>
      </button>
    </div>
    <el-alert
      type="warning"
      show-icon
      :closable="false"
      title="将在真实环境执行测试，可能产生或删除数据"
      class="gen-launcher__alert"
    />
    <el-form label-width="96px" label-position="left" class="gen-launcher__form" @submit.prevent="onSubmit">
      <el-form-item label="目标服务" required>
        <el-select
          v-model="form.serviceId"
          filterable
          placeholder="选择服务"
          :loading="loadingServices"
          class="gen-launcher__field"
          @change="onServiceChange"
        >
          <el-option v-for="svc in services" :key="svc.id" :label="svc.name" :value="svc.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="目标环境" required>
        <el-select
          v-model="form.environmentId"
          filterable
          placeholder="选择环境"
          :loading="loadingEnvironments"
          :disabled="!form.serviceId"
          class="gen-launcher__field"
        >
          <el-option v-for="env in environments" :key="env.id" :label="envLabel(env)" :value="env.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="自动修复">
        <div class="gen-launcher__switch-row">
          <el-switch v-model="form.autoRepair" />
          <span class="gen-launcher__hint">失败时自动分析并修复场景后重跑</span>
        </div>
      </el-form-item>
      <el-form-item label="最大轮次">
        <el-input-number v-model="form.maxRounds" :min="1" :max="10" :disabled="!form.autoRepair" />
      </el-form-item>
      <AIScenarioLoginCredentials
        v-model="form.loginCredentials"
        :project-id="projectId"
        :service-id="form.serviceId"
        :environment-id="form.environmentId"
      />
      <div class="gen-launcher__actions">
        <el-button @click="$emit('close')">取消</el-button>
        <el-button type="primary" :disabled="!canSubmit" @click="onSubmit">开始生成</el-button>
      </div>
    </el-form>
  </div>
</template>

<script>
import { Close } from '@element-plus/icons-vue'
import AIScenarioLoginCredentials from './AIScenarioLoginCredentials.vue'
import { listServiceEnvironments, listServices } from '../api'
import { projectState } from '../utils/currentProject'

export default {
  name: 'AIScenarioGenLauncher',
  components: { Close, AIScenarioLoginCredentials },
  emits: ['close', 'submit'],
  data() {
    return {
      services: [],
      environments: [],
      loadingServices: false,
      loadingEnvironments: false,
      form: {
        serviceId: '',
        environmentId: '',
        autoRepair: true,
        maxRounds: 3,
        loginCredentials: {},
      },
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    canSubmit() {
      return !!(this.form.serviceId && this.form.environmentId)
    },
  },
  watch: {
    projectId: {
      immediate: true,
      handler(id) {
        if (id) this.loadServices()
      },
    },
  },
  methods: {
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
    onSubmit() {
      if (!this.canSubmit) return
      const svc = this.services.find((s) => s.id === this.form.serviceId)
      const env = this.environments.find((e) => e.id === this.form.environmentId)
      this.$emit('submit', {
        serviceId: this.form.serviceId,
        environmentId: this.form.environmentId,
        autoRepair: !!this.form.autoRepair,
        maxRounds: this.form.autoRepair ? this.form.maxRounds : 0,
        loginCredentials: this.form.loginCredentials,
        serviceName: svc?.name || '',
        environmentName: env?.name || '',
      })
    },
  },
}
</script>

<style scoped>
.gen-launcher {
  width: min(520px, 100%);
  margin: 0 auto 16px;
  padding: 16px 18px;
  border-radius: 14px;
  border: 1px solid var(--app-border-color);
  background: var(--app-card-bg);
  box-shadow: 0 8px 28px rgba(15, 23, 42, 0.08);
}

.gen-launcher__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.gen-launcher__title {
  font-size: 15px;
  font-weight: 600;
}

.gen-launcher__close {
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--app-text-muted);
  padding: 4px;
  border-radius: 6px;
}

.gen-launcher__close:hover {
  background: var(--el-fill-color-light);
}

.gen-launcher__alert {
  margin-bottom: 12px;
}

.gen-launcher__field {
  width: 100%;
}

.gen-launcher__switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.gen-launcher__hint {
  font-size: 12px;
  color: var(--app-text-muted);
}

.gen-launcher__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}
</style>
