<template>
  <div class="workspace-layout" :class="layoutClasses">
    <button
      v-if="showDrawerTrigger"
      type="button"
      class="workspace-layout__drawer-trigger"
      @click="sidebarDrawerOpen = true"
    >
      <el-icon><Menu /></el-icon>
      <span>{{ drawerTriggerLabel }}</span>
    </button>

    <aside
      v-show="!useDrawerSidebar"
      class="workspace-layout__sidebar"
      :style="sidebarStyle"
    >
      <slot name="sidebar" />
    </aside>

    <el-drawer
      v-if="useDrawerSidebar"
      v-model="sidebarDrawerOpen"
      direction="ltr"
      :size="sidebarWidth"
      :with-header="false"
      append-to-body
      class="workspace-layout__drawer"
    >
      <slot name="sidebar" />
    </el-drawer>

    <main class="workspace-layout__main">
      <slot />
    </main>
  </div>
</template>

<script>
import { Menu } from '@element-plus/icons-vue'

export default {
  name: 'WorkspaceLayout',
  components: { Menu },
  props: {
    sidebarWidth: { type: String, default: '260px' },
    /** px width at which layout becomes narrow */
    breakpoint: { type: Number, default: 1100 },
    /** stack: sidebar above main; drawer: sidebar in drawer when narrow */
    narrowMode: {
      type: String,
      default: 'stack',
      validator: (v) => ['stack', 'drawer'].includes(v),
    },
    drawerTriggerLabel: { type: String, default: '会话列表' },
  },
  data() {
    return {
      viewportWidth: typeof window !== 'undefined' ? window.innerWidth : 1200,
      sidebarDrawerOpen: false,
    }
  },
  computed: {
    isNarrow() {
      return this.viewportWidth < this.breakpoint
    },
    useDrawerSidebar() {
      return this.isNarrow && this.narrowMode === 'drawer'
    },
    showDrawerTrigger() {
      return this.useDrawerSidebar
    },
    layoutClasses() {
      return {
        'workspace-layout--stacked': this.isNarrow && this.narrowMode === 'stack',
        'workspace-layout--drawer': this.useDrawerSidebar,
      }
    },
    sidebarStyle() {
      if (this.isNarrow && this.narrowMode === 'stack') {
        return { width: '100%', maxWidth: 'none', flex: '0 0 auto' }
      }
      return { width: this.sidebarWidth, flex: `0 0 ${this.sidebarWidth}` }
    },
  },
  mounted() {
    this.onResize = () => {
      this.viewportWidth = window.innerWidth
      if (!this.useDrawerSidebar) this.sidebarDrawerOpen = false
    }
    window.addEventListener('resize', this.onResize)
    this.onResize()
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.onResize)
  },
  methods: {
    closeSidebarDrawer() {
      this.sidebarDrawerOpen = false
    },
  },
}
</script>

<style scoped>
.workspace-layout {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

.workspace-layout--stacked {
  flex-direction: column;
}

.workspace-layout__drawer-trigger {
  display: none;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  background: var(--app-card-bg);
  color: var(--app-text-color);
  font-size: var(--app-font-size-small);
  cursor: pointer;
}

.workspace-layout--drawer .workspace-layout__drawer-trigger {
  display: inline-flex;
}

.workspace-layout__sidebar {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: var(--app-card-bg);
  border-right: 1px solid var(--app-border-color);
}

.workspace-layout--stacked .workspace-layout__sidebar {
  border-right: none;
  border-bottom: 1px solid var(--app-border-color);
  max-height: 40vh;
}

.workspace-layout__main {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.workspace-layout--stacked .workspace-layout__main {
  flex: 1 1 auto;
  min-height: 0;
}
</style>
