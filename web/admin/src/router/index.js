import { createRouter, createWebHistory } from 'vue-router'

import { bindPage } from '../stores/aiAssistant'
import AdminLayout from '../layout/AdminLayout.vue'
import Login from '../views/Login.vue'
import GitHubCallback from '../views/GitHubCallback.vue'
import PendingApproval from '../views/PendingApproval.vue'
import Dashboard from '../views/Dashboard.vue'
import AIAssistantPage from '../views/AIAssistantPage.vue'
import ProjectManagement from '../views/projects/ProjectManagement.vue'
import SQLParameterSourceList from '../views/data/SQLParameterSourceList.vue'
import MockServerList from '../views/data/MockServerList.vue'
import MockServerAccessLogs from '../views/data/MockServerAccessLogs.vue'
import TestDataTableList from '../views/data/TestDataTableList.vue'
import ApiManagement from '../views/cases/ApiManagement.vue'
import CaseRunWorkspace from '../views/cases/CaseRunWorkspace.vue'
import ScenarioList from '../views/scenarios/ScenarioList.vue'
import ScenarioRunReport from '../views/scenarios/ScenarioRunReport.vue'
import ProjectRunHistory from '../views/reports/ProjectRunHistory.vue'
import ScriptLibraryList from '../views/scriptlib/ScriptLibraryList.vue'
import MockValueSetList from '../views/platform/MockValueSetList.vue'
import TemplateReference from '../views/platform/TemplateReference.vue'
import DataSourceList from '../views/data/DataSourceList.vue'
import AIProviderList from '../views/platform/AIProviderList.vue'
import ProjectPromptList from '../views/projects/ProjectPromptList.vue'
import UserList from '../views/rbac/UserList.vue'
import RoleList from '../views/rbac/RoleList.vue'
import PermissionList from '../views/rbac/PermissionList.vue'
import ApiKeyList from '../views/rbac/ApiKeyList.vue'

export const menuRoutes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: Dashboard, meta: { title: '概览' } },
  {
    path: '/ai-assistant',
    component: AIAssistantPage,
    meta: { title: 'AI 助理', hidden: true }
  },
  { path: '/projects', component: ProjectManagement, meta: { title: '项目管理', permission: 'projects:read' } },
  {
    path: '/test-data',
    redirect: '/sql-parameter-sources',
    meta: { title: '测试数据', permission: 'projects:read' },
    children: [
      { path: '/mock-servers', component: MockServerList, meta: { title: 'Mock Server', permission: 'projects:read' } },
      { path: '/sql-parameter-sources', component: SQLParameterSourceList, meta: { title: 'SQL 参数源', permission: 'projects:read' } },
      {
        path: '/mock-servers/:serverId/access-logs',
        component: MockServerAccessLogs,
        meta: { title: 'Mock 访问日志', permission: 'projects:read', hidden: true, activeMenu: '/mock-servers' }
      },
      { path: '/test-data-tables', component: TestDataTableList, meta: { title: '测试数据表', permission: 'projects:read' } }
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
  { path: '/cases', component: ApiManagement, meta: { title: 'API管理', permission: 'cases:read' } },
  { path: '/spec-import', redirect: '/cases' },
  { path: '/run-console', component: CaseRunWorkspace, meta: { title: '运行控制台', permission: 'cases:read' } },
  {
    path: '/run-console/:caseID',
    component: CaseRunWorkspace,
    meta: { title: '运行控制台', permission: 'cases:read', hidden: true, activeMenu: '/run-console' }
  },
  { path: '/cases/run', redirect: '/run-console' },
  { path: '/cases/:caseID/run', redirect: (to) => ({ path: `/run-console/${to.params.caseID}` }) },

  { path: '/scenarios', component: ScenarioList, meta: { title: '场景编排', permission: 'cases:read' } },
  {
    path: '/scenarios/:scenarioID',
    component: ScenarioList,
    meta: { title: '场景编排', permission: 'cases:read', hidden: true, activeMenu: '/scenarios' }
  },
  {
    path: '/runs/:runId',
    component: ScenarioRunReport,
    meta: { title: '场景运行报告', permission: 'cases:read', hidden: true, activeMenu: '/reports/runs' }
  },
  {
    path: '/reports',
    redirect: '/reports/runs',
    meta: { title: '测试报告', permission: 'cases:read' },
    children: [
      { path: '/reports/runs', component: ProjectRunHistory, meta: { title: '运行记录', permission: 'cases:read' } }
    ]
  },
  {
    path: '/platform',
    redirect: '/script-library',
    meta: { title: '平台资源' },
    children: [
      { path: '/script-library', component: ScriptLibraryList, meta: { title: '脚本库', permission: 'cases:read' } },
      { path: '/platform/data-sources', component: DataSourceList, meta: { title: '业务数据源', permission: 'projects:read' } },
      { path: '/platform/ai-providers', component: AIProviderList, meta: { title: 'AI 提供商', permission: 'projects:read' } },
      { path: '/platform/ai-prompts', component: ProjectPromptList, meta: { title: 'Prompt 管理', permission: 'projects:read' } },
      { path: '/mock-value-sets', component: MockValueSetList, meta: { title: '命名值集合', permission: 'projects:read' } },
      { path: '/template-reference', component: TemplateReference, meta: { title: '模板与变量参考' } }
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
      { path: '/users', component: UserList, meta: { title: '用户管理', permission: 'users:manage' } },
      { path: '/roles', component: RoleList, meta: { title: '角色管理', permission: 'roles:manage' } },
      { path: '/permissions', component: PermissionList, meta: { title: '权限菜单', permission: 'permissions:manage' } },
      { path: '/api-keys', component: ApiKeyList, meta: { title: 'API Key', permission: 'apikeys:manage' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    { path: '/login/github/callback', component: GitHubCallback, meta: { public: true, title: 'GitHub 登录' } },
    { path: '/pending-approval', component: PendingApproval, meta: { title: '待审核' } },
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
