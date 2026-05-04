import { createRouter, createWebHistory } from 'vue-router'

import AdminLayout from '../layout/AdminLayout.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import ProjectList from '../views/projects/ProjectList.vue'
import ServiceEnvironment from '../views/projects/ServiceEnvironment.vue'
import SpecImport from '../views/spec/SpecImport.vue'
import CaseList from '../views/cases/CaseList.vue'
import CaseRunWorkspace from '../views/cases/CaseRunWorkspace.vue'
import SuiteList from '../views/suites/SuiteList.vue'
import UserList from '../views/rbac/UserList.vue'
import RoleList from '../views/rbac/RoleList.vue'
import PermissionList from '../views/rbac/PermissionList.vue'

export const menuRoutes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: Dashboard, meta: { title: '概览' } },
  { path: '/projects', component: ProjectList, meta: { title: '项目管理', permission: 'projects:read' } },
  { path: '/services', component: ServiceEnvironment, meta: { title: '服务与环境', permission: 'projects:read' } },
  { path: '/spec-import', component: SpecImport, meta: { title: 'OpenAPI 导入', permission: 'specs:import' } },
  { path: '/cases', component: CaseList, meta: { title: '测试用例', permission: 'cases:read' } },
  { path: '/run-console', component: CaseRunWorkspace, meta: { title: '运行控制台', permission: 'cases:read' } },
  {
    path: '/run-console/:caseID',
    component: CaseRunWorkspace,
    meta: { title: '运行控制台', permission: 'cases:read', hidden: true, activeMenu: '/run-console' }
  },
  { path: '/cases/run', redirect: '/run-console' },
  { path: '/cases/:caseID/run', redirect: (to) => ({ path: `/run-console/${to.params.caseID}` }) },
  { path: '/suites', component: SuiteList, meta: { title: '测试集', permission: 'suites:read' } },
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
