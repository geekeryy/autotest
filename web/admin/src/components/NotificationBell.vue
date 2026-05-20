<template>
  <el-popover
    v-model:visible="popoverVisible"
    placement="bottom-end"
    :width="380"
    trigger="click"
    popper-class="notification-popover"
    @show="handlePopoverShow"
  >
    <template #reference>
      <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notification-bell-badge">
        <el-button class="notification-bell-btn" :loading="loading && !items.length">
          <el-icon><Bell /></el-icon>
        </el-button>
      </el-badge>
    </template>

    <div class="notification-panel">
      <div class="notification-panel__header">
        <span class="notification-panel__title">通知</span>
        <div v-if="items.length" class="notification-panel__actions">
          <el-button
            v-if="unreadCount > 0"
            link
            type="primary"
            size="small"
            :loading="markAllLoading"
            @click="handleMarkAllRead"
          >
            全部标为已读
          </el-button>
          <el-button
            link
            type="primary"
            size="small"
            :loading="clearAllLoading"
            @click="handleClearAll"
          >
            一键清空
          </el-button>
        </div>
      </div>

      <div v-if="loading && !items.length" class="notification-panel__empty">加载中…</div>
      <div v-else-if="!items.length" class="notification-panel__empty">暂无通知</div>
      <ul v-else class="notification-list">
        <li
          v-for="item in items"
          :key="item.id"
          class="notification-item"
          :class="{ 'notification-item--unread': !item.readAt }"
          @click="handleItemClick(item)"
        >
          <div class="notification-item__title">{{ item.title }}</div>
          <div class="notification-item__body">{{ item.body }}</div>
          <div class="notification-item__time">{{ formatDateTime(item.createdAt) }}</div>
        </li>
      </ul>
    </div>
  </el-popover>
</template>

<script>
import { Bell } from '@element-plus/icons-vue'
import { clearAllNotifications, listNotifications, markAllNotificationsRead, markNotificationRead } from '../api'
import { setCurrentProjectId } from '../utils/currentProject'
import { formatDateTime } from '../utils/datetime'
import { subscribeNotificationStream } from '../utils/notificationStream'

export default {
  name: 'NotificationBell',
  components: { Bell },
  data() {
    return {
      items: [],
      unreadCount: 0,
      loading: false,
      markAllLoading: false,
      clearAllLoading: false,
      popoverVisible: false,
      stopStream: null,
    }
  },
  mounted() {
    this.refresh()
    this.stopStream = subscribeNotificationStream((event) => this.handleStreamEvent(event))
    document.addEventListener('visibilitychange', this.handleVisibilityChange)
  },
  beforeUnmount() {
    this.stopStream?.()
    this.stopStream = null
    document.removeEventListener('visibilitychange', this.handleVisibilityChange)
  },
  methods: {
    formatDateTime,
    handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        this.refresh({ silent: true })
      }
    },
    handleStreamEvent(event) {
      if (!event?.kind) return
      if (event.kind === 'snapshot') {
        if (Array.isArray(event.items)) {
          this.items = event.items
        }
        if (typeof event.unreadCount === 'number') {
          this.unreadCount = event.unreadCount
        }
        return
      }
      if (event.kind === 'notification') {
        if (typeof event.unreadCount === 'number') {
          this.unreadCount = event.unreadCount
        }
        const n = event.notification
        if (!n?.id) return
        const idx = this.items.findIndex((item) => item.id === n.id)
        if (idx >= 0) {
          this.items.splice(idx, 1, n)
        } else {
          this.items = [n, ...this.items].slice(0, 20)
        }
      }
    },
    handlePopoverShow() {
      this.refresh()
    },
    async refresh(options = {}) {
      if (!options.silent) {
        this.loading = true
      }
      try {
        const data = await listNotifications({ limit: 20 })
        this.items = Array.isArray(data?.items) ? data.items : []
        this.unreadCount = Number(data?.unreadCount) || 0
      } catch {
        if (!options.silent) {
          this.items = []
          this.unreadCount = 0
        }
      } finally {
        if (!options.silent) {
          this.loading = false
        }
      }
    },
    async handleMarkAllRead() {
      this.markAllLoading = true
      try {
        await markAllNotificationsRead()
        await this.refresh({ silent: true })
      } finally {
        this.markAllLoading = false
      }
    },
    async handleClearAll() {
      this.clearAllLoading = true
      try {
        await clearAllNotifications()
        this.items = []
        this.unreadCount = 0
      } finally {
        this.clearAllLoading = false
      }
    },
    async handleItemClick(item) {
      if (!item?.readAt) {
        try {
          await markNotificationRead(item.id)
          item.readAt = new Date().toISOString()
          this.unreadCount = Math.max(0, this.unreadCount - 1)
        } catch {
          // 跳转仍继续；切回页面时 refresh / SSE snapshot 会同步
        }
      }

      const payload = this.parsePayload(item?.payload)
      if (payload?.projectId) {
        setCurrentProjectId(payload.projectId)
      }

      this.popoverVisible = false
      if (this.$route.path !== '/cases') {
        await this.$router.push('/cases')
      }
    },
    parsePayload(raw) {
      if (!raw) return null
      if (typeof raw === 'object') return raw
      try {
        return JSON.parse(raw)
      } catch {
        return null
      }
    },
  },
}
</script>

<style scoped>
.notification-bell-badge :deep(.el-badge__content) {
  border: none;
}

.notification-bell-btn {
  padding: 8px 10px;
}

.notification-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.notification-panel__title {
  font-weight: 600;
  font-size: 14px;
}

.notification-panel__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.notification-panel__empty {
  padding: 24px 0;
  text-align: center;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.notification-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 360px;
  overflow-y: auto;
}

.notification-item {
  padding: 10px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.notification-item + .notification-item {
  border-top: 1px solid var(--el-border-color-lighter);
}

.notification-item:hover {
  background: var(--el-fill-color-light);
}

.notification-item--unread {
  background: var(--el-color-primary-light-9);
}

.notification-item--unread:hover {
  background: var(--el-color-primary-light-8);
}

.notification-item__title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
}

.notification-item__body {
  font-size: 12px;
  color: var(--el-text-color-regular);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.notification-item__time {
  margin-top: 6px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
</style>
