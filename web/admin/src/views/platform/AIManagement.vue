<template>
  <div class="page-card ai-management">
    <div class="page-header">
      <div>
        <h2 class="page-title">AI 能力管理</h2>
        <p class="page-subtitle">
          集中管理 AI 提供商、助理运行时配置、生成动作 Prompt、Skills、记忆、Prompt 分层版本与速率限制。
        </p>
      </div>
    </div>

    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="AI 提供商" name="providers">
        <AIProviderList embedded />
      </el-tab-pane>
      <el-tab-pane label="AI 助理配置" name="assistant">
        <AIAssistantSettings embedded />
      </el-tab-pane>
      <el-tab-pane label="动作 Prompt" name="action-prompts">
        <ProjectPromptList embedded />
      </el-tab-pane>
      <el-tab-pane label="Skills 技能" name="skills">
        <SkillsPanel />
      </el-tab-pane>
      <el-tab-pane label="记忆管理" name="memory">
        <MemoryPanel />
      </el-tab-pane>
      <el-tab-pane label="Prompt 版本" name="prompts">
        <PromptLayersPanel />
      </el-tab-pane>
      <el-tab-pane label="速率限制" name="ratelimits">
        <RateLimitsPanel />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
import AIProviderList from './AIProviderList.vue'
import AIAssistantSettings from './AIAssistantSettings.vue'
import ProjectPromptList from '../projects/ProjectPromptList.vue'
import SkillsPanel from './ai/SkillsPanel.vue'
import MemoryPanel from './ai/MemoryPanel.vue'
import PromptLayersPanel from './ai/PromptLayersPanel.vue'
import RateLimitsPanel from './ai/RateLimitsPanel.vue'

const VALID_TABS = new Set([
  'providers',
  'assistant',
  'action-prompts',
  'skills',
  'memory',
  'prompts',
  'ratelimits'
])

const DEFAULT_TAB = 'providers'

export default {
  name: 'AIManagement',
  components: {
    AIProviderList,
    AIAssistantSettings,
    ProjectPromptList,
    SkillsPanel,
    MemoryPanel,
    PromptLayersPanel,
    RateLimitsPanel
  },
  data() {
    return { activeTab: DEFAULT_TAB }
  },
  watch: {
    '$route.query.tab': {
      immediate: true,
      handler() {
        this.syncTabFromRoute()
      }
    }
  },
  methods: {
    syncTabFromRoute() {
      const tab = this.$route.query.tab
      if (typeof tab === 'string' && VALID_TABS.has(tab) && tab !== this.activeTab) {
        this.activeTab = tab
      }
    },
    onTabChange(name) {
      const tab = VALID_TABS.has(name) ? name : DEFAULT_TAB
      if (this.$route.query.tab === tab) return
      const query = { ...this.$route.query, tab }
      this.$router.replace({ path: this.$route.path, query })
    }
  }
}
</script>

<style scoped>
.ai-management {
  padding: 24px;
}
.page-header {
  margin-bottom: 16px;
}
.page-title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}
.page-subtitle {
  margin: 0;
  color: #909399;
  font-size: 13px;
}
</style>
