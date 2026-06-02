<template>
  <div class="page-card ai-management">
    <div class="page-header">
      <div>
        <h2 class="page-title">AI 能力管理</h2>
        <p class="page-subtitle">管理 AI Skills、记忆、Prompt 版本和速率限制。</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" @tab-change="onTabChange">
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
import SkillsPanel from './ai/SkillsPanel.vue'
import MemoryPanel from './ai/MemoryPanel.vue'
import PromptLayersPanel from './ai/PromptLayersPanel.vue'
import RateLimitsPanel from './ai/RateLimitsPanel.vue'

export default {
  name: 'AIManagement',
  components: { SkillsPanel, MemoryPanel, PromptLayersPanel, RateLimitsPanel },
  data() {
    return { activeTab: 'skills' }
  },
  methods: {
    onTabChange() {}
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
