<template>
  <el-container class="admin-layout">
    <el-aside :width="sidebarWidth" class="sidebar" :class="{ 'sidebar--collapsed': isSidebarCollapsed }">
      <div class="brand">
        <div class="brand-identity">
          <BrandLogo class="brand-logo-mark" />
          <span v-if="!isSidebarCollapsed" class="brand-title">Autotest</span>
        </div>
        <el-button
          class="collapse-button"
          text
          circle
          :title="isSidebarCollapsed ? '展开菜单' : '收起菜单'"
          :aria-label="isSidebarCollapsed ? '展开菜单' : '收起菜单'"
          @click="toggleSidebar"
        >
          <el-icon><component :is="isSidebarCollapsed ? 'Expand' : 'Fold'" /></el-icon>
        </el-button>
      </div>
      <el-menu
        class="sidebar-menu"
        :default-active="activeMenu"
        :default-openeds="defaultOpeneds"
        :collapse="isSidebarCollapsed"
        :collapse-transition="false"
        router
        :background-color="resolvedPalette.sidebarBg"
        :text-color="resolvedPalette.sidebarText"
        :active-text-color="resolvedPalette.sidebarActive"
      >
        <template v-for="item in visibleMenus" :key="item.path">
          <el-sub-menu v-if="item.children && item.children.length" :index="item.path">
            <template #title>
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.meta.title }}</span>
            </template>
            <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path">
              <el-icon><component :is="child.icon" /></el-icon>
              <span>{{ child.meta.title }}</span>
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item v-else :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.meta.title }}</span>
          </el-menu-item>
        </template>
      </el-menu>
      <div class="sidebar-footer">
        <div class="sidebar-user" :class="{ 'sidebar-user--collapsed': isSidebarCollapsed }">
          <el-dropdown trigger="click" placement="top-start" @command="onUserMenuCommand">
            <div
              class="sidebar-user-avatar-trigger"
              :class="{ 'sidebar-user-avatar-trigger--collapsed': isSidebarCollapsed }"
              role="button"
              tabindex="0"
            >
              <el-avatar :size="isSidebarCollapsed ? 36 : 40" :src="avatarSrc" :style="avatarBgStyle">
                {{ userInitials }}
              </el-avatar>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>{{ userName }}</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <div v-if="!isSidebarCollapsed" class="sidebar-user-main">
            <div class="sidebar-user-name" :title="userName">{{ userName }}</div>
          </div>
        </div>
      </div>
    </el-aside>

    <el-container>
      <el-header class="topbar">
        <div class="breadcrumb-box">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item>管理后台</el-breadcrumb-item>
            <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">{{ item.meta.title }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="user-box">
          <div class="global-project">
            <span class="global-project-label">项目</span>
            <el-select
              v-model="projectId"
              class="global-project-select"
              placeholder="请选择项目"
              no-data-text="暂无项目"
              filterable
              :loading="projectState.loading"
            >
              <el-option v-for="project in projectState.projects" :key="project.id" :label="project.name" :value="project.id" />
            </el-select>
          </div>
          <el-popover placement="bottom-end" :width="420" trigger="click">
            <template #reference>
              <el-button>
                <el-icon><Brush /></el-icon>
                <span>外观</span>
              </el-button>
            </template>
            <div class="appearance-panel">
              <div class="appearance-section">
                <div class="appearance-label">字体大小</div>
                <el-radio-group v-model="appearance.fontSize" class="font-size-group" size="small" @change="commitAppearance">
                  <el-radio-button v-for="item in fontSizeOptions" :key="item.value" :value="item.value">
                    {{ item.label }}
                  </el-radio-button>
                </el-radio-group>
              </div>
              <div class="appearance-section">
                <div class="appearance-label">配色方案</div>
                <el-radio-group v-model="appearance.palette" class="palette-group" size="small" @change="handlePaletteChange">
                  <el-radio-button v-for="item in paletteOptions" :key="item.value" :value="item.value">
                    <span class="palette-option">
                      <span class="palette-dots">
                        <span class="palette-dot" :style="{ background: item.primary }"></span>
                        <span class="palette-dot" :style="{ background: item.secondary }"></span>
                        <span class="palette-dot" :style="{ background: item.accent }"></span>
                      </span>
                      {{ item.label }}
                    </span>
                  </el-radio-button>
                </el-radio-group>
              </div>
              <div class="appearance-section custom-color-row">
                <span class="appearance-label">自定义主色</span>
                <el-color-picker v-model="appearance.primaryColor" :predefine="predefinedColors" @change="handlePrimaryColorChange" />
              </div>
              <div class="appearance-actions">
                <el-button link type="primary" @click="restoreDefaultAppearance">恢复默认</el-button>
              </div>
            </div>
          </el-popover>
        </div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script>
import { logout } from '../api'
import { authState, hasPermission, resetCurrentUser } from '../auth'
import { clearToken } from '../utils/storage'
import { menuRoutes } from '../router'
import { loadGlobalProjects, projectState, setCurrentProjectId } from '../utils/currentProject'
import BrandLogo from '../components/BrandLogo.vue'
import {
  FONT_SIZE_OPTIONS,
  PALETTE_OPTIONS,
  applyAppearance,
  getResolvedPalette,
  getStoredAppearance,
  resetAppearance,
  saveAppearance
} from '../utils/appearance'

const icons = {
  '/dashboard': 'DataAnalysis',
  '/projects': 'OfficeBuilding',
  '/test-data': 'TrendCharts',
  '/sql-parameter-sources': 'Tickets',
  '/mock-servers': 'Connection',
  '/test-data-tables': 'Grid',
  '/case-mgmt': 'Document',
  '/cases': 'Document',
  '/spec-import': 'Upload',
  '/run-console': 'Monitor',
  '/platform': 'Files',
  '/script-library': 'Notebook',
  '/template-reference': 'Reading',
  '/system': 'Setting',
  '/users': 'User',
  '/roles': 'UserFilled',
  '/permissions': 'Key'
}

function buildVisibleMenus(routes) {
  return routes.reduce((menus, item) => {
    if (item.meta?.hidden) return menus

    const children = item.children ? buildVisibleMenus(item.children) : []
    const canAccess = hasPermission(item.meta?.permission)

    if (children.length) {
      if (canAccess) {
        menus.push({ ...item, children, icon: icons[item.path] || 'Menu' })
      }
      return menus
    }

    if (item.component && canAccess) {
      menus.push({ ...item, icon: icons[item.path] || 'Menu' })
    }
    return menus
  }, [])
}

function findOpenMenus(items, currentPath) {
  return items.reduce((openeds, item) => {
    if (!item.children?.length) return openeds

    const childPaths = item.children.map((child) => child.path)
    if (childPaths.includes(currentPath)) {
      openeds.push(item.path)
    }
    openeds.push(...findOpenMenus(item.children, currentPath))
    return openeds
  }, [])
}

export default {
  name: 'AdminLayout',
  components: {
    BrandLogo
  },
  data() {
    return {
      isSidebarCollapsed: false,
      appearance: getStoredAppearance(),
      fontSizeOptions: FONT_SIZE_OPTIONS,
      paletteOptions: PALETTE_OPTIONS,
      predefinedColors: [...new Set(PALETTE_OPTIONS.flatMap((item) => [item.primary, item.secondary, item.accent]))]
    }
  },
  computed: {
    projectState() {
      return projectState
    },
    projectId: {
      get() {
        return projectState.currentProjectId
      },
      set(value) {
        setCurrentProjectId(value)
      }
    },
    resolvedPalette() {
      return getResolvedPalette(this.appearance)
    },
    sidebarWidth() {
      return this.isSidebarCollapsed ? '64px' : '220px'
    },
    visibleMenus() {
      return buildVisibleMenus(menuRoutes)
    },
    activeMenu() {
      return this.$route.meta?.activeMenu || this.$route.path
    },
    defaultOpeneds() {
      return findOpenMenus(this.visibleMenus, this.activeMenu)
    },
    breadcrumbs() {
      return this.$route.matched.filter((item) => item.meta?.title && !item.meta?.public)
    },
    userName() {
      return authState.user?.displayName || authState.user?.username || '管理员'
    },
    avatarSrc() {
      const u = authState.user
      return u?.avatarUrl ? String(u.avatarUrl) : ''
    },
    avatarHueBackground() {
      const raw = String(authState.user?.id || authState.user?.username || 'x')
      let h = 0
      for (let i = 0; i < raw.length; i++) {
        h = raw.charCodeAt(i) + ((h << 5) - h)
      }
      const hue = Math.abs(h) % 360
      return `hsl(${hue}, 42%, 44%)`
    },
    avatarBgStyle() {
      if (this.avatarSrc) return {}
      return { backgroundColor: this.avatarHueBackground }
    },
    userInitials() {
      const u = authState.user
      const display = (u?.displayName || '').trim()
      const username = (u?.username || '').trim()
      const name = display || username || '管'
      if (/[\u4e00-\u9fff]/.test(name)) {
        return name.length >= 2 ? name.slice(0, 2) : name.slice(0, 1)
      }
      const parts = name.split(/[\s._-]+/).filter(Boolean)
      if (parts.length >= 2) {
        return (parts[0][0] + parts[1][0]).toUpperCase()
      }
      return name.slice(0, 2).toUpperCase()
    }
  },
  created() {
    loadGlobalProjects()
  },
  methods: {
    toggleSidebar() {
      this.isSidebarCollapsed = !this.isSidebarCollapsed
    },
    commitAppearance() {
      this.appearance = saveAppearance(applyAppearance(this.appearance))
    },
    handlePaletteChange(value) {
      const selected = this.paletteOptions.find((item) => item.value === value)
      if (selected) {
        this.appearance.primaryColor = selected.primary
      }
      this.commitAppearance()
    },
    handlePrimaryColorChange(value) {
      if (!value) return
      this.appearance.palette = 'custom'
      this.appearance.primaryColor = value
      this.commitAppearance()
    },
    restoreDefaultAppearance() {
      this.appearance = resetAppearance()
    },
    async handleLogout() {
      try {
        await logout()
      } finally {
        clearToken()
        resetCurrentUser()
        this.$router.replace('/login')
      }
    },
    onUserMenuCommand(cmd) {
      if (cmd === 'logout') this.handleLogout()
    }
  }
}
</script>

<style scoped>
.admin-layout {
  height: 100vh;
  max-height: 100vh;
  overflow: hidden;
}

/* 右侧栏：顶栏 + 主区纵向排列，主区吸收剩余高度并单独滚动，避免整页增高导致左侧导航被带出视口 */
.admin-layout > .el-container {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.admin-layout :deep(.el-main) {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
}

.sidebar {
  display: flex;
  flex-direction: column;
  align-self: stretch;
  height: 100%;
  max-height: 100%;
  background: var(--app-sidebar-bg);
  transition: width 0.2s ease;
  overflow-x: hidden;
  overflow-y: hidden;
}

.brand {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 22px;
  color: var(--app-sidebar-active);
  font-size: var(--app-font-size-title);
  font-weight: 700;
  letter-spacing: 0.5px;
}

.brand-identity {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.brand-logo-mark {
  --brand-logo-size: 34px;
}

.brand-title {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.collapse-button {
  flex: 0 0 auto;
  color: var(--app-sidebar-text);
}

.collapse-button:hover,
.collapse-button:focus {
  color: var(--app-sidebar-active);
  background: rgba(255, 255, 255, 0.08);
}

.sidebar-menu {
  border-right: 0;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.sidebar-footer {
  flex-shrink: 0;
  padding: 12px 14px 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar-user {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.sidebar-user--collapsed {
  justify-content: center;
  padding: 2px 0;
}

.sidebar-user-main {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
}

.sidebar-user-name {
  width: 100%;
  font-size: var(--app-font-size-small);
  font-weight: 600;
  color: var(--app-sidebar-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.3;
}

.sidebar-user-avatar-trigger {
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  outline: none;
  flex-shrink: 0;
}

.sidebar-user-avatar-trigger:focus-visible {
  border-radius: 50%;
  box-shadow: 0 0 0 2px var(--app-sidebar-active);
}

.sidebar--collapsed .brand {
  justify-content: center;
  flex-direction: column;
  gap: 4px;
  height: auto;
  min-height: 60px;
  padding: 8px 6px 10px;
  line-height: 1;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.sidebar--collapsed .brand-identity {
  justify-content: center;
}

.sidebar--collapsed .brand-logo-mark {
  --brand-logo-size: 30px;
}

.sidebar--collapsed .collapse-button {
  width: 28px;
  min-height: 28px;
  height: 28px;
  padding: 0;
}

.topbar {
  background: var(--app-card-bg);
  border-bottom: 1px solid var(--app-border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.breadcrumb-box {
  min-width: 0;
  flex: 1;
}

.user-box {
  display: flex;
  align-items: center;
  gap: 16px;
  flex: 0 0 auto;
}

.global-project {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.global-project-label {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  white-space: nowrap;
}

.global-project-select {
  width: 220px;
}

.appearance-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.appearance-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.appearance-label {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.font-size-group {
  display: flex;
  flex-wrap: wrap;
}

.palette-group {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.palette-group :deep(.el-radio-button__inner) {
  width: 100%;
  border-left: var(--el-border);
  border-radius: 4px;
}

.palette-group :deep(.el-radio-button:first-child .el-radio-button__inner),
.palette-group :deep(.el-radio-button:last-child .el-radio-button__inner) {
  border-radius: 4px;
}

.palette-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  justify-content: flex-start;
}

.palette-dots {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.palette-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 0 0 1px rgba(15, 23, 42, 0.12);
}

.custom-color-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.appearance-actions {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 900px) {
  .topbar {
    height: auto;
    align-items: stretch;
    flex-direction: column;
    padding: 8px 12px;
  }

  .user-box {
    width: 100%;
    flex-wrap: wrap;
  }

  .global-project {
    flex: 1 1 220px;
  }

  .global-project-select {
    width: 100%;
  }
}
</style>
