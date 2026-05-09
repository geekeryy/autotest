import { createRouter, createWebHistory } from 'vue-router'

import AdminLayout from '../layout/AdminLayout.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import ProjectManagement from '../views/projects/ProjectManagement.vue'
import SQLParameterSourceList from '../views/data/SQLParameterSourceList.vue'
import MockServerList from '../views/data/MockServerList.vue'
import TestDataTableList from '../views/data/TestDataTableList.vue'
import SpecImport from '../views/spec/SpecImport.vue'
import CaseList from '../views/cases/CaseList.vue'
import CaseRunWorkspace from '../views/cases/CaseRunWorkspace.vue'
import ScenarioList from '../views/scenarios/ScenarioList.vue'
import ScriptLibraryList from '../views/scriptlib/ScriptLibraryList.vue'
import TemplateReference from '../views/platform/TemplateReference.vue'
import UserList from '../views/rbac/UserList.vue'
import RoleList from '../views/rbac/RoleList.vue'
import PermissionList from '../views/rbac/PermissionList.vue'

export const menuRoutes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: Dashboard, meta: { title: '概览' } },
  { path: '/projects', component: ProjectManagement, meta: { title: '项目管理', permission: 'projects:read' } },
  {
    path: '/test-data',
    redirect: '/sql-parameter-sources',
    meta: { title: '测试数据', permission: 'projects:read' },
    children: [
      { path: '/sql-parameter-sources', component: SQLParameterSourceList, meta: { title: 'SQL 参数源', permission: 'projects:read' } },
      { path: '/mock-servers', component: MockServerList, meta: { title: 'Mock Server', permission: 'projects:read' } },
      { path: '/test-data-tables', component: TestDataTableList, meta: { title: '测试数据表', permission: 'projects:read' } }
    ]
  },
  {
    path: '/services',
    redirect: () => ({ path: '/projects', query: { tab: 'services' } })
  },
  {
    path: '/data-sources',
    redirect: () => ({ path: '/projects', query: { tab: 'dataSources' } })
  },
  {
    path: '/case-mgmt',
    redirect: '/cases',
    meta: { title: 'API管理' },
    children: [
      { path: '/cases', component: CaseList, meta: { title: '接口列表', permission: 'cases:read' } },
      { path: '/spec-import', component: SpecImport, meta: { title: 'OpenAPI 导入', permission: 'specs:import' } }
    ]
  },
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
    path: '/platform',
    redirect: '/script-library',
    meta: { title: '平台资源' },
    children: [
      { path: '/script-library', component: ScriptLibraryList, meta: { title: '脚本库', permission: 'cases:read' } },
      { path: '/template-reference', component: TemplateReference, meta: { title: '模板与变量参考' } }
    ]
  },
  {
    path: '/ai-providers',
    redirect: () => ({ path: '/projects', query: { tab: 'aiProviders' } })
  },
  {
    path: '/system',
    redirect: '/users',
    meta: { title: '系统管理' },
    children: [
      { path: '/users', component: UserList, meta: { title: '用户管理', permission: 'users:manage' } },
      { path: '/roles', component: RoleList, meta: { title: '角色管理', permission: 'roles:manage' } },
      { path: '/permissions', component: PermissionList, meta: { title: '权限菜单', permission: 'permissions:manage' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    {
      path: '/',
      component: AdminLayout,
      children: menuRoutes
    }
  ]
})

export default router
