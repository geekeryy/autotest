<template>
  <div
    class="session-list-item"
    :class="itemClasses"
    role="button"
    tabindex="0"
    @click="onSelect"
    @keydown.enter.prevent="onSelect"
  >
    <div class="session-list-item__body">
      <span class="session-list-item__title-row">
        <span class="session-list-item__title">{{ session.title || '新对话' }}</span>
        <span v-if="tags.length" class="session-list-item__tags">
          <span v-for="tag in tags" :key="tag" class="session-list-item__tag">{{ tag }}</span>
        </span>
      </span>
    </div>
    <el-dropdown
      trigger="click"
      placement="bottom-end"
      popper-class="session-list-item-menu"
      @command="onMenuCommand"
      @click.stop
    >
      <el-icon class="session-list-item__action" title="更多操作" @click.stop>
        <MoreFilled />
      </el-icon>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="detail">
            <el-icon><InfoFilled /></el-icon>
            <span>会话详情</span>
          </el-dropdown-item>
          <el-dropdown-item command="rename">
            <el-icon><EditPen /></el-icon>
            <span>重命名</span>
          </el-dropdown-item>
          <el-dropdown-item command="delete" divided class="session-list-item-menu__danger">
            <el-icon><Delete /></el-icon>
            <span>删除</span>
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script>
import { Delete, EditPen, InfoFilled, MoreFilled } from '@element-plus/icons-vue'

export default {
  name: 'SessionListItem',
  components: { Delete, EditPen, InfoFilled, MoreFilled },
  props: {
    session: { type: Object, required: true },
    active: { type: Boolean, default: false },
    focused: { type: Boolean, default: false },
    tags: { type: Array, default: () => [] },
    /** sidebar | history */
    variant: { type: String, default: 'sidebar' },
  },
  emits: ['select', 'delete', 'detail', 'rename'],
  computed: {
    itemClasses() {
      return {
        active: this.active,
        'session-list-item--focused': this.focused,
        'session-list-item--history': this.variant === 'history',
      }
    },
  },
  methods: {
    onSelect() {
      this.$emit('select', this.session)
    },
    onMenuCommand(command) {
      if (command === 'detail') {
        this.$emit('detail', this.session)
        return
      }
      if (command === 'rename') {
        this.$emit('rename', this.session)
        return
      }
      if (command === 'delete') {
        this.$emit('delete', this.session)
      }
    },
  },
}
</script>

<style scoped>
.session-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  width: 100%;
  margin-bottom: 4px;
  padding: 10px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s ease;
  font: inherit;
  color: inherit;
  outline: none;
}

.session-list-item:hover,
.session-list-item:focus-visible {
  background: #e8f0fe;
}

.session-list-item.active {
  background: #e4edfd;
}

.session-list-item.active .session-list-item__title {
  color: #3964fe;
}


.session-list-item--history {
  padding: 8px 10px;
  border-radius: 8px;
  margin-bottom: 2px;
}

.session-list-item__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
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
  font-weight: 400;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-list-item--history .session-list-item__title {
  font-size: 13px;
  color: #333;
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

.session-list-item__action {
  flex-shrink: 0;
  width: 23px;
  height: 23px;
  font-size: 20px;
  padding: 4px;
  border-radius: 6px;
  color: var(--app-text-muted);
  opacity: 0;
  cursor: pointer;
  transition: opacity 0.15s ease, background 0.15s ease, color 0.15s ease;
}

.session-list-item:hover .session-list-item__action,
.session-list-item:focus-within .session-list-item__action,
.session-list-item.active .session-list-item__action {
  opacity: 0.45;
}

.session-list-item:hover .session-list-item__action:hover,
.session-list-item:focus-within .session-list-item__action:hover,
.session-list-item.active .session-list-item__action:hover {
  opacity: 1;
  background: color-mix(in srgb, var(--app-text-color) 8%, transparent);
  color: var(--app-text-color);
}

.session-list-item--focused.active {
  box-shadow: none;
}
</style>

<style>
.session-list-item-menu.el-popper {
  padding: 4px 0;
}

.session-list-item-menu .el-dropdown-menu {
  display: flex;
  flex-direction: column;
  min-width: 148px;
  padding: 4px 0;
}

.session-list-item-menu .el-dropdown-menu__item {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 14px;
  line-height: 1.4;
  white-space: nowrap;
}

.session-list-item-menu .el-dropdown-menu__item .el-icon {
  flex-shrink: 0;
  margin: 0;
  font-size: 18px;
}

.session-list-item-menu .el-dropdown-menu__item > span {
  flex: 1;
  font-size: 14px;
}

.session-list-item-menu__danger {
  color: var(--el-color-danger);
}

.session-list-item-menu__danger .el-icon {
  color: var(--el-color-danger);
}
</style>
