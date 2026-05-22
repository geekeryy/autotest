<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">概览</h2>
      <el-button @click="load">
        <el-icon><Refresh /></el-icon>
        <span>刷新</span>
      </el-button>
    </div>

    <p v-if="!currentProjectId" class="context-hint">在顶部选择项目后，接口、场景与数据源数量将按当前项目统计。</p>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-row :gutter="16" v-loading="loading" class="metric-row">
      <el-col :xs="12" :sm="12" :md="8" :lg="6" v-for="card in cards" :key="card.title">
        <el-card class="metric" :style="{ borderTopColor: card.color }">
          <div class="metric-icon" :style="{ color: card.color }">
            <el-icon :size="28"><component :is="card.icon" /></el-icon>
          </div>
          <div class="metric-value" :style="{ color: card.color }">{{ card.value }}</div>
          <div class="metric-title">{{ card.title }}</div>
          <div v-if="card.hint" class="metric-hint">{{ card.hint }}</div>
        </el-card>
      </el-col>
    </el-row>

    <section class="ai-spotlight" aria-label="AI 助理引导">
      <div class="ai-spotlight__aurora" aria-hidden="true" />
      <div class="ai-spotlight__inner">
        <div class="ai-spotlight__copy">
          <div class="ai-spotlight__eyebrow">
            <span class="ai-spotlight__pulse" />
            <span>全新 · AI 测试助理</span>
          </div>
          <h3 class="ai-spotlight__title">用对话完成查询、编排与断言调整</h3>
          <p class="ai-spotlight__desc">
            结合当前项目上下文理解你在看的场景与用例；所有写操作会先展示预览并等待你确认，再写入平台。
          </p>
          <ul class="ai-spotlight__features">
            <li v-for="item in aiFeatureHighlights" :key="item">{{ item }}</li>
          </ul>
        </div>
        <div class="ai-spotlight__cta">
          <router-link to="/ai-assistant" class="ai-spotlight__link">
            <el-button type="primary" size="large" class="ai-spotlight__btn">
              <el-icon><ChatLineRound /></el-icon>
              <span>进入 AI 工作台</span>
            </el-button>
          </router-link>
          <p class="ai-spotlight__cta-hint">顶栏「AI 助理」可随时打开右侧对话侧栏</p>
        </div>
      </div>
    </section>

    <div v-if="tokenUsageSection" class="page-card token-usage-panel">
      <h3 class="section-title">AI Token 消耗</h3>
      <p v-if="tokenUsageSection.hint && !tokenUsageSection.hasData" class="token-usage-panel__hint">
        {{ tokenUsageSection.hint }}
      </p>
      <el-row v-else :gutter="12" class="token-usage-panel__grid">
        <el-col :xs="12" :sm="8" :md="6" v-for="item in tokenUsageSection.items" :key="item.label">
          <div class="token-usage-panel__item" :class="{ 'token-usage-panel__item--input': item.inputCache }">
            <span class="token-usage-panel__value">{{ item.value }}</span>
            <span class="token-usage-panel__label">{{ item.label }}</span>
            <div
              v-if="item.inputCache?.showBreakdown"
              class="token-usage-panel__input-bar"
              role="img"
              :aria-label="`输入 Token：缓存命中 ${item.inputCache.hitPercentLabel}，非缓存 ${item.inputCache.nonHitPercentLabel}`"
            >
              <span
                class="token-usage-panel__input-bar-hit"
                :style="{ width: item.inputCache.hitPercent + '%' }"
              />
            </div>
            <p v-if="item.inputCache?.showBreakdown" class="token-usage-panel__input-legend">
              <span class="token-usage-panel__input-legend-hit">
                缓存命中 {{ item.inputCache.hitPercentLabel }}
                <span class="token-usage-panel__input-legend-count">{{ item.inputCache.hitFormatted }}</span>
              </span>
              <span class="token-usage-panel__input-legend-non-hit">
                非缓存 {{ item.inputCache.nonHitPercentLabel }}
                <span class="token-usage-panel__input-legend-count">{{ item.inputCache.nonHitFormatted }}</span>
              </span>
            </p>
          </div>
        </el-col>
      </el-row>
      <p class="token-usage-panel__scope">{{ tokenUsageSection.scopeLabel }}</p>
    </div>

    <div class="page-card shortcuts">
      <h3 class="section-title">快捷入口</h3>
      <p class="shortcuts-hint">根据本机记录的点击次数，优先展示使用最多的 4 个入口；次数仅在当前浏览器保存。</p>
      <el-row :gutter="12">
        <el-col v-for="item in topShortcuts" :key="item.key" :xs="12" :sm="8" :md="6" class="shortcut-col">
          <router-link :to="item.to" class="shortcut-link" @click="onShortcutClick(item.key)">
            <el-card shadow="hover" class="shortcut-card">
              <el-icon class="shortcut-icon" :size="22"><component :is="item.icon" /></el-icon>
              <div class="shortcut-title">{{ item.title }}</div>
              <div class="shortcut-desc">{{ item.desc }}</div>
            </el-card>
          </router-link>
        </el-col>
      </el-row>
    </div>


  </div>
</template>

<script>
import {
  ChatLineRound,
  Connection,
  Document,
  Folder,
  Monitor,
  Refresh,
  Setting,
  Tickets,
  DataAnalysis
} from '@element-plus/icons-vue'
import { getAIMyTokenUsage, getAIProjectTokenUsage, listCases, listDataSources, listProjects, listScenarios } from '../api'
import { buildCacheStatItems, inputTokenCacheBreakdown } from '../utils/aiCacheUsage'
import { hasPermission } from '../auth'
import ListLoadError from '../components/ListLoadError.vue'
import { incrementShortcutClick, rankShortcutsByClicks } from '../utils/dashboardShortcutClicks'
import { getListLoadErrorMessage } from '../utils/listPageLoad'
import { projectState } from '../utils/currentProject'

const SHORTCUTS = [
  { key: 'projects', title: '项目管理', desc: '项目、成员与资源', to: '/projects', permission: 'projects:read', icon: Folder },
  {
    key: 'services',
    title: '服务与环境',
    desc: 'Base URL 与认证',
    to: { path: '/projects', query: { tab: 'services' } },
    permission: 'projects:read',
    icon: Setting
  },
  { key: 'cases', title: 'API管理', desc: '导入文档与接口模板', to: '/cases', permission: 'cases:read', icon: Document },
  { key: 'run', title: '运行控制台', desc: '调试与保存用例', to: '/run-console', permission: 'cases:read', icon: Monitor },
  { key: 'scenarios', title: '场景编排', desc: '多步骤联调与回归', to: '/scenarios', permission: 'cases:read', icon: Connection },
  { key: 'sql', title: 'SQL 参数源', desc: '动态参数与预览', to: '/sql-parameter-sources', permission: 'projects:read', icon: Tickets }
]

const SHORTCUT_ORDER_KEYS = SHORTCUTS.map((s) => s.key)

const AI_FEATURE_HIGHLIGHTS = [
  '自然语言查询接口与场景',
  '一键生成多步骤编排',
  '写操作人工确认后生效',
]

export default {
  name: 'Dashboard',
  components: {
    ListLoadError,
    Refresh,
    Folder,
    Document,
    Connection,
    Tickets,
    ChatLineRound,
    DataAnalysis,
  },
  data() {
    return {
      aiFeatureHighlights: AI_FEATURE_HIGHLIGHTS,
      loading: false,
      loadError: '',
      counts: {
        projects: 0,
        cases: 0,
        scenarios: 0,
        dataSources: 0
      },
      tokenUsage: null,
      /** 用于点击后触发「按次数排序」的 computed 刷新 */
      shortcutClickRevision: 0
    }
  },
  computed: {
    currentProjectId() {
      return projectState.currentProjectId
    },
    visibleShortcuts() {
      return SHORTCUTS.filter((s) => hasPermission(s.permission))
    },
    topShortcuts() {
      this.shortcutClickRevision
      return rankShortcutsByClicks(this.visibleShortcuts, SHORTCUT_ORDER_KEYS).slice(0, 4)
    },
    cards() {
      const scoped = !!this.currentProjectId
      return [
        { title: '项目数', value: this.counts.projects, hint: '全部项目', icon: 'Folder', color: '#2563eb' },
        { title: '接口数', value: this.counts.cases, hint: scoped ? '当前项目' : '全部可见', icon: 'Document', color: '#14b8a6' },
        { title: '场景数', value: this.counts.scenarios, hint: scoped ? '当前项目' : '全部可见', icon: 'Connection', color: '#f97316' },
        // {
        //   title: '业务数据源',
        //   value: scoped ? this.counts.dataSources : '—',
        //   hint: scoped ? '当前项目' : '请先选择项目',
        //   icon: 'Tickets',
        //   color: '#8b5cf6'
        // },
        {
          title: 'AI Token 合计',
          value: this.tokenUsageCardValue,
          hint: this.tokenUsageCardHint,
          icon: 'DataAnalysis',
          color: '#0ea5e9'
        }
      ]
    },
    tokenUsageSection() {
      const t = this.tokenUsage
      if (!t) return null
      const fmt = this.formatTokenCount
      const scopeLabel = this.currentProjectId
        ? '统计范围：当前项目 · 我的 AI 会话'
        : '统计范围：全部项目 · 我的 AI 会话'
      if (!t.hasUsageData) {
        return { hasData: false, hint: t.hint, scopeLabel, items: [] }
      }
      const inputCache = this.buildInputCacheDisplay(t, fmt)
      const items = [
        {
          label: '输入 Token',
          value: fmt(t.promptTokens),
          inputCache: inputCache.showBreakdown ? inputCache : null,
        },
        { label: '输出 Token', value: fmt(t.completionTokens) },
        { label: '合计 Token', value: fmt(t.totalTokens) },
      ]
      items.push(...buildCacheStatItems(t, fmt, { omitHit: true, omitMiss: inputCache.showBreakdown }))
      if (t.reasoningTokens) items.push({ label: '推理 Token', value: fmt(t.reasoningTokens) })
      return { hasData: true, hint: '', scopeLabel, items }
    },
    tokenUsageCardValue() {
      const t = this.tokenUsage
      if (!t) return '—'
      if (!t.hasUsageData) return '—'
      return this.formatTokenCount(t.totalTokens)
    },
    tokenUsageCardHint() {
      const t = this.tokenUsage
      if (!t?.hasUsageData) return this.currentProjectId ? '当前项目 · 暂无用量' : '全部项目 · 暂无用量'
      const in_ = this.formatTokenCount(t.promptTokens)
      const out = this.formatTokenCount(t.completionTokens)
      return `输入 ${in_} · 输出 ${out}`
    }
  },
  watch: {
    currentProjectId() {
      this.load()
    }
  },
  created() {
    this.load()
  },
  methods: {
    onShortcutClick(key) {
      incrementShortcutClick(key)
      this.shortcutClickRevision++
    },
    formatTokenCount(n) {
      const v = Number(n)
      if (!Number.isFinite(v)) return '—'
      return v.toLocaleString('zh-CN')
    },
    formatPercent(ratio) {
      if (!Number.isFinite(ratio) || ratio <= 0) return '0%'
      if (ratio >= 99.95) return '100%'
      if (ratio < 0.05) return '<0.1%'
      return `${ratio.toFixed(1)}%`
    },
    buildInputCacheDisplay(usage, fmt) {
      const { total, hit, nonHit, hitPercent, showBreakdown } = inputTokenCacheBreakdown(usage)
      const nonHitPercent = showBreakdown ? Math.max(0, 100 - hitPercent) : 0
      return {
        showBreakdown,
        hitPercent,
        hitPercentLabel: this.formatPercent(hitPercent),
        nonHitPercentLabel: this.formatPercent(nonHitPercent),
        hitFormatted: fmt(hit),
        nonHitFormatted: fmt(nonHit),
        total,
      }
    },
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        const projects = await listProjects()
        this.counts.projects = projects.length

        const pid = this.currentProjectId
        const projectParams = pid ? { projectId: pid } : {}

        const [cases, scenarios, dataSources] = await Promise.all([
          listCases(projectParams),
          listScenarios(projectParams),
          listDataSources()
        ])

        this.counts.cases = Array.isArray(cases) ? cases.length : 0
        this.counts.scenarios = Array.isArray(scenarios) ? scenarios.length : 0
        this.counts.dataSources = Array.isArray(dataSources) ? dataSources.length : 0

        try {
          this.tokenUsage = pid
            ? await getAIProjectTokenUsage(pid)
            : await getAIMyTokenUsage()
        } catch {
          this.tokenUsage = null
        }
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err)
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.context-hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
}

.metric-row {
  margin-bottom: 8px;
}

.metric {
  margin-bottom: 16px;
  border-top: 3px solid transparent;
  transition: border-top-color 0.2s ease;
}

.metric-icon {
  margin-bottom: 10px;
}

.metric-value {
  font-size: var(--app-font-size-metric);
  font-weight: 700;
}

.metric-title {
  color: var(--app-secondary-text);
  margin-top: 8px;
}

.metric-hint {
  margin-top: 4px;
  font-size: var(--el-font-size-extra-small);
  color: var(--el-text-color-placeholder);
}

.token-usage-panel {
  margin-bottom: 16px;
}

.token-usage-panel__hint {
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-color-warning-dark-2, #b88230);
  background: var(--el-color-warning-light-9, #fdf6ec);
}

.token-usage-panel__grid {
  margin-top: 4px;
}

.token-usage-panel__item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  margin-bottom: 12px;
  border-radius: 10px;
  background: var(--app-surface-subtle);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.token-usage-panel__value {
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--app-text-color);
}

.token-usage-panel__label {
  font-size: 12px;
  color: var(--app-text-muted);
}

.token-usage-panel__item--input {
  gap: 6px;
}

.token-usage-panel__input-bar {
  display: flex;
  width: 100%;
  height: 6px;
  margin-top: 2px;
  border-radius: 999px;
  overflow: hidden;
  background: color-mix(in srgb, var(--el-color-info-light-5, #c6e2ff) 55%, var(--el-border-color-lighter, #ebeef5));
}

.token-usage-panel__input-bar-hit {
  display: block;
  min-width: 0;
  height: 100%;
  border-radius: 999px 0 0 999px;
  background: linear-gradient(
    90deg,
    var(--el-color-success, #67c23a),
    color-mix(in srgb, var(--el-color-success, #67c23a) 75%, var(--el-color-primary, #409eff))
  );
  transition: width 0.35s ease;
}

.token-usage-panel__input-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin: 0;
  font-size: 11px;
  line-height: 1.4;
  color: var(--app-text-muted);
}

.token-usage-panel__input-legend-hit::before,
.token-usage-panel__input-legend-non-hit::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 4px;
  border-radius: 50%;
  vertical-align: 0.05em;
}

.token-usage-panel__input-legend-hit::before {
  background: var(--el-color-success, #67c23a);
}

.token-usage-panel__input-legend-non-hit::before {
  background: color-mix(in srgb, var(--el-color-info-light-5, #c6e2ff) 70%, var(--el-border-color, #dcdfe6));
}

.token-usage-panel__input-legend-count {
  margin-left: 2px;
  font-variant-numeric: tabular-nums;
  color: var(--el-text-color-secondary, #909399);
}

.token-usage-panel__scope {
  margin: 0;
  font-size: 12px;
  color: var(--app-text-muted);
}

.ai-spotlight {
  position: relative;
  overflow: hidden;
  margin: 20px 0 8px;
  padding: 22px 24px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 30%, transparent);
  background:
    radial-gradient(ellipse 80% 60% at 100% 0%, color-mix(in srgb, #8b5cf6 14%, transparent), transparent),
    radial-gradient(ellipse 70% 50% at 0% 100%, color-mix(in srgb, var(--el-color-primary) 12%, transparent), transparent),
    linear-gradient(135deg, var(--app-card-bg, #fff), color-mix(in srgb, var(--el-color-primary-light-9, #ecf5ff) 55%, var(--app-card-bg, #fff)));
}

.ai-spotlight__aurora {
  position: absolute;
  inset: -50% -20%;
  background: conic-gradient(
    from 200deg at 50% 50%,
    transparent,
    color-mix(in srgb, var(--el-color-primary) 18%, transparent),
    transparent,
    color-mix(in srgb, #8b5cf6 14%, transparent),
    transparent
  );
  animation: spotlight-spin 14s linear infinite;
  opacity: 0.65;
  pointer-events: none;
}

.ai-spotlight__inner {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  flex-wrap: wrap;
}

.ai-spotlight__copy {
  flex: 1 1 320px;
  min-width: 0;
}

.ai-spotlight__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--el-color-primary);
}

.ai-spotlight__pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-primary);
  animation: spotlight-pulse 2s ease-in-out infinite;
}

.ai-spotlight__title {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.3;
  letter-spacing: -0.02em;
}

.ai-spotlight__desc {
  margin: 0 0 12px;
  max-width: 520px;
  font-size: 14px;
  line-height: 1.65;
  color: var(--app-secondary-text);
}

.ai-spotlight__features {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-spotlight__features li {
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 12px;
  background: color-mix(in srgb, var(--app-card-bg, #fff) 70%, var(--el-color-primary-light-9, #ecf5ff));
  border: 1px solid var(--el-border-color-lighter);
  color: var(--el-text-color-regular);
}

.ai-spotlight__cta {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.ai-spotlight__link {
  text-decoration: none;
}

.ai-spotlight__btn {
  border-radius: 12px;
  padding: 12px 22px;
  font-weight: 600;
  box-shadow: 0 8px 24px -8px color-mix(in srgb, var(--el-color-primary) 55%, transparent);
}

.ai-spotlight__cta-hint {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

@keyframes spotlight-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .ai-spotlight__aurora {
    animation: none;
    opacity: 0.35;
  }

  .ai-spotlight__pulse {
    animation: none;
  }
}

@keyframes spotlight-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--el-color-primary) 50%, transparent);
  }
  50% {
    box-shadow: 0 0 0 6px transparent;
  }
}

@media (max-width: 768px) {
  .ai-spotlight__inner {
    flex-direction: column;
    align-items: stretch;
  }

  .ai-spotlight__cta {
    align-items: stretch;
  }

  .ai-spotlight__btn {
    width: 100%;
  }
}

.shortcuts {
  margin-top: 8px;
}

.section-title {
  margin: 0 0 8px;
  font-size: var(--el-font-size-medium);
  font-weight: 600;
}

.shortcuts-hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
}

.shortcut-col {
  margin-bottom: 12px;
}

.shortcut-link {
  display: block;
  text-decoration: none;
  color: inherit;
}

.shortcut-card {
  height: 100%;
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.shortcut-card:hover {
  border-color: var(--app-primary-color);
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.shortcut-icon {
  color: var(--app-primary-color);
  margin-bottom: 8px;
}

.shortcut-title {
  font-weight: 600;
  margin-bottom: 4px;
}

.shortcut-desc {
  font-size: var(--el-font-size-extra-small);
  color: var(--app-secondary-text);
  line-height: 1.4;
}
</style>
