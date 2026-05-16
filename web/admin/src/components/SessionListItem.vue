<template>
  <component
    :is="rootTag"
    type="button"
    class="session-list-item"
    :class="itemClasses"
    @click="onSelect"
  >
    <span class="session-list-item__title-row">
      <span class="session-list-item__title">{{ session.title || '新对话' }}</span>
      <span v-if="tags.length" class="session-list-item__tags">
        <span v-for="tag in tags" :key="tag" class="session-list-item__tag">{{ tag }}</span>
      </span>
    </span>
    <span class="session-list-item__meta">{{ timeLabel }}</span>
    <button
      type="button"
      class="session-list-item__delete"
      title="删除会话"
      @click.stop="onDelete"
    >
      <el-icon><Delete /></el-icon>
    </button>
  </component>
</template>

<script>
import { Delete } from '@element-plus/icons-vue'
import { formatRelativeTime, formatSessionTime } from '../utils/useRelativeTime'

export default {
  name: 'SessionListItem',
  components: { Delete },
  props: {
    session: { type: Object, required: true },
    active: { type: Boolean, default: false },
    focused: { type: Boolean, default: false },
    tags: { type: Array, default: () => [] },
    /** sidebar | history */
    variant: { type: String, default: 'sidebar' },
    rootTag: { type: String, default: 'button' },
  },
  emits: ['select', 'delete'],
  computed: {
    itemClasses() {
      return {
        active: this.active,
        'session-list-item--focused': this.focused,
        'session-list-item--history': this.variant === 'history',
      }
    },
    timeLabel() {
      const fn = this.variant === 'history' ? formatSessionTime : formatRelativeTime
      return fn(this.session.updatedAt)
    },
  },
  methods: {
    onSelect() {
      this.$emit('select', this.session)
    },
    onDelete() {
      this.$emit('delete', this.session)
    },
  },
}
</script>

<style scoped>
.session-list-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  margin-bottom: 2px;
  padding: 10px 36px 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s ease;
  font: inherit;
  color: inherit;
}

.session-list-item:hover {
  background: var(--app-surface-subtle);
}

.session-list-item.active {
  background: var(--app-focus-ring);
}

.session-list-item--history {
  padding: 8px 32px 8px 10px;
  border-radius: 8px;
  margin-bottom: 0;
}

.session-list-item__title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.session-list-item__title {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--app-text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-list-item--history .session-list-item__title {
  font-size: 13px;
  font-weight: 500;
}

.session-list-item__meta {
  font-size: 12px;
  color: var(--app-text-muted);
}

.session-list-item__tags {
  display: inline-flex;
  flex-shrink: 0;
  gap: 4px;
}

.session-list-item__tag {
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--el-color-primary);
  background: var(--app-focus-ring);
}

.session-list-item__delete {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-muted);
  cursor: pointer;
  opacity: 0.55;
  transition: opacity 0.15s ease, background 0.15s ease, color 0.15s ease;
}

.session-list-item:hover .session-list-item__delete,
.session-list-item.active .session-list-item__delete {
  opacity: 0.85;
}

.session-list-item__delete:hover {
  background: color-mix(in srgb, var(--el-color-danger) 12%, transparent);
  color: var(--el-color-danger);
  opacity: 1;
}

.session-list-item--focused.active {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--el-color-primary) 35%, transparent);
}
</style>
