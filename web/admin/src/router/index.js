import { createRouter, createWebHistory } from 'vue-router'

import { bindPage } from '../stores/aiAssistant'
import AdminLayout from '../layout/AdminLayout.vue'

export const menuRoutes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '概览' } },
  {
    path: '/ai-assistant',
    component: () => import('../views/AIAssistantPage.vue'),
    meta: { title: 'AI 助理', hidden: true }
  },
  { path: '/projects', component: () => import('../views/projects/ProjectManagement.vue'), meta: { title: '项目管理', permission: 'projects:read' } },
  {
    path: '/test-data',
    redirect: '/sql-parameter-sources',
    meta: { title: '测试数据', permission: 'projects:read' },
    children: [
      { path: '/mock-servers', component: () => import('../views/data/MockServerList.vue'), meta: { title: 'Mock Server', permission: 'projects:read' } },
      { path: '/sql-parameter-sources', component: () => import('../views/data/SQLParameterSourceList.vue'), meta: { title: 'SQL 参数源', permission: 'projects:read' } },
      {
        path: '/mock-servers/:serverId/access-logs',
        component: () => import('../views/data/MockServerAccessLogs.vue'),
        meta: { title: 'Mock 访问日志', permission: 'projects:read', hidden: true, activeMenu: '/mock-servers' }
      },
      { path: '/test-data-tables', component: () => import('../views/data/TestDataTableList.vue'), meta: { title: '测试数据表', permission: 'projects:read' } }
    ]
  },
  {
    path: '/services',
    redirect: () => ({ path: '/projects', query: { tab: 'services' } })
  },
  {
    path: '/data-sources',
    redirect: '/platform/data-sources'
  },
  {
    path: '/case-mgmt',
    redirect: '/cases'
  },
  { path: '/cases', component: () => import('../views/cases/ApiManagement.vue'), meta: { title: 'API管理', permission: 'cases:read' } },
  { path: '/spec-import', redirect: '/cases' },
  { path: '/run-console', component: () => import('../views/cases/CaseRunWorkspace.vue'), meta: { title: '运行控制台', permission: 'cases:read' } },
  {
    path: '/run-console/:caseID',
    component: () => import('../views/cases/CaseRunWorkspace.vue'),
    meta: { title: '运行控制台', permission: 'cases:read', hidden: true, activeMenu: '/run-console' }
  },
  { path: '/cases/run', redirect: '/run-console' },
  { path: '/cases/:caseID/run', redirect: (to) => ({ path: `/run-console/${to.params.caseID}` }) },

  { path: '/scenarios', component: () => import('../views/scenarios/ScenarioList.vue'), meta: { title: '场景编排', permission: 'cases:read' } },
  {
    path: '/scenarios/:scenarioID',
    component: () => import('../views/scenarios/ScenarioList.vue'),
    meta: { title: '场景编排', permission: 'cases:read', hidden: true, activeMenu: '/scenarios' }
  },
  {
    path: '/runs/:runId',
    component: () => import('../views/scenarios/ScenarioRunReport.vue'),
    meta: { title: '场景运行报告', permission: 'cases:read', hidden: true, activeMenu: '/reports/runs' }
  },
  {
    path: '/reports',
    redirect: '/reports/runs',
    meta: { title: '测试报告', permission: 'cases:read' },
    children: [
      { path: '/reports/runs', component: () => import('../views/reports/ProjectRunHistory.vue'), meta: { title: '运行记录', permission: 'cases:read' } }
    ]
  },
  {
    path: '/platform',
    redirect: '/script-library',
    meta: { title: '平台资源' },
    children: [
      { path: '/script-library', component: () => import('../views/scriptlib/ScriptLibraryList.vue'), meta: { title: '脚本库', permission: 'cases:read' } },
      { path: '/platform/data-sources', component: () => import('../views/data/DataSourceList.vue'), meta: { title: '业务数据源', permission: 'projects:read' } },
      { path: '/platform/ai-providers', component: () => import('../views/platform/AIProviderList.vue'), meta: { title: 'AI 提供商', permission: 'projects:read' } },
      { path: '/platform/ai-prompts', component: () => import('../views/projects/ProjectPromptList.vue'), meta: { title: 'Prompt 管理', permission: 'projects:read' } },
      { path: '/mock-value-sets', component: () => import('../views/platform/MockValueSetList.vue'), meta: { title: '命名值集合', permission: 'projects:read' } },
      { path: '/template-reference', component: () => import('../views/platform/TemplateReference.vue'), meta: { title: '模板与变量参考' } }
    ]
  },
  {
    path: '/ai-providers',
    redirect: '/platform/ai-providers'
  },
  {
    path: '/system',
    redirect: '/users',
    meta: { title: '系统管理' },
    children: [
      {
        path: '/users',
        component: () => import('../views/rbac/UserManagement.vue'),
        meta: {
          title: '用户管理',
          permissions: ['users:manage', 'roles:manage', 'permissions:manage'],
        },
      },
      {
        path: '/auth-providers',
        redirect: (to) => ({ path: '/users', query: { ...to.query, tab: 'auth-providers' } }),
      },
      {
        path: '/roles',
        redirect: (to) => ({ path: '/users', query: { ...to.query, tab: 'roles' } }),
      },
      {
        path: '/permissions',
        redirect: (to) => ({ path: '/users', query: { ...to.query, tab: 'permissions' } }),
      },
      { path: '/api-keys', component: () => import('../views/rbac/ApiKeyList.vue'), meta: { title: 'API Key', permission: 'apikeys:manage' } },
      { path: '/audit-logs', component: () => import('../views/rbac/AuditLogList.vue'), meta: { title: '审计日志', permission: 'audit:read' } },
    ],
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue'), meta: { public: true, title: '登录' } },
    { path: '/login/oauth/callback', component: () => import('../views/OAuthCallback.vue'), meta: { public: true, title: 'OAuth 登录' } },
    {
      path: '/login/github/callback',
      redirect: (to) => ({ path: '/login/oauth/callback', hash: to.hash, query: to.query })
    },
    { path: '/pending-approval', component: () => import('../views/PendingApproval.vue'), meta: { title: '待审核' } },
    { path: '/change-password', component: () => import('../views/ChangePassword.vue'), meta: { title: '修改密码' } },
    {
      path: '/',
      component: AdminLayout,
      children: menuRoutes
    }
  ]
})

// Push a baseline page-context snapshot on every navigation so the AI
// assistant always knows at least the current route. Individual pages
// can call `enrichPage` from setup/mount to add resolved fields like
// scenarioName / caseName once their data has loaded.
router.afterEach((to) => {
  if (to.meta?.public) {
    bindPage(null)
    return
  }
  const ctx = {
    path: to.path,
    routeTitle: to.meta?.title || '',
  }
  if (to.params?.scenarioID) ctx.scenarioId = String(to.params.scenarioID)
  if (to.params?.caseID) ctx.caseId = String(to.params.caseID)
  bindPage(ctx)
})

export default router
