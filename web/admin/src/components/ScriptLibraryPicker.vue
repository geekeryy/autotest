<template>
  <div class="script-library-picker">
    <el-popover
      v-model:visible="popoverOpen"
      placement="bottom-start"
      :width="420"
      trigger="click"
      @update:visible="onPopoverVisible"
    >
      <template #reference>
        <el-button size="small" type="primary" plain>
          脚本库
        </el-button>
      </template>
      <div class="script-library-panel">
        <el-input
          v-model="query"
          size="small"
          clearable
          placeholder="搜索模板名称或描述"
          class="script-library-search"
        />
        <div v-if="libraryLoading" class="script-library-loading">加载脚本模板…</div>
        <el-scrollbar max-height="320px" class="script-library-scroll">
          <template v-if="filteredGroups.length">
            <div v-for="[category, items] in filteredGroups" :key="category" class="script-library-section">
              <div class="script-library-cat">{{ category }}</div>
              <ul class="script-library-list">
                <li v-for="item in items" :key="item.id">
                  <button type="button" class="script-library-item" @click="onPick(item)">
                    <span class="script-library-item-name">{{ item.name }}</span>
                    <span v-if="item.description" class="script-library-item-desc">{{ item.description }}</span>
                  </button>
                </li>
              </ul>
            </div>
          </template>
          <el-empty v-else description="无匹配模板" :image-size="48" />
        </el-scrollbar>
        <div class="script-library-footer">
          <router-link v-if="projectId" to="/script-library" class="script-library-manage-link" @click="popoverOpen = false">
            管理脚本模板
          </router-link>
          <template v-if="projectId">
            <span class="script-library-footer-sep">·</span>
          </template>
          <span>点击条目将追加到编辑器末尾；可再修改占位参数。</span>
        </div>
      </div>
    </el-popover>
  </div>
</template>

<script>
import { listScriptLibraryTemplates } from '../api'
import { projectState } from '../utils/currentProject'
import { groupTemplatesByCategory, listTemplatesForVariant } from '../utils/scriptTemplates'

export default {
  name: 'ScriptLibraryPicker',
  props: {
    /** assertion：接口断言脚本；scenario：场景脚本步骤 */
    variant: {
      type: String,
      default: 'assertion',
      validator: (v) => ['assertion', 'scenario'].includes(v)
    }
  },
  emits: ['append'],
  data() {
    return {
      query: '',
      popoverOpen: false,
      libraryTemplates: [],
      libraryLoading: false,
      projectState
    }
  },
  computed: {
    projectId() {
      return this.projectState.currentProjectId || ''
    },
    baseList() {
      return listTemplatesForVariant(this.variant, this.libraryTemplates)
    },
    filteredGroups() {
      const q = (this.query || '').trim().toLowerCase()
      let list = this.baseList
      if (q) {
        list = list.filter(
          (t) =>
            (t.name && t.name.toLowerCase().includes(q)) ||
            (t.description && t.description.toLowerCase().includes(q)) ||
            (t.category && t.category.toLowerCase().includes(q))
        )
      }
      return groupTemplatesByCategory(list)
    }
  },
  methods: {
    onPopoverVisible(visible) {
      if (visible) this.fetchLibraryTemplates()
    },
    async fetchLibraryTemplates() {
      const pid = this.projectId
      if (!pid) {
        this.libraryTemplates = []
        return
      }
      this.libraryLoading = true
      try {
        this.libraryTemplates = await listScriptLibraryTemplates(pid)
      } catch {
        this.libraryTemplates = []
      } finally {
        this.libraryLoading = false
      }
    },
    onPick(item) {
      const code = item.code || ''
      if (!code) return
      this.$emit('append', code)
    }
  }
}
</script>

<style scoped>
.script-library-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.script-library-search {
  width: 100%;
}

.script-library-section {
  margin-bottom: 10px;
}

.script-library-section:last-child {
  margin-bottom: 0;
}

.script-library-cat {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-secondary-text, #909399);
  margin-bottom: 6px;
  padding-left: 2px;
}

.script-library-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.script-library-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  width: 100%;
  text-align: left;
  padding: 8px 10px;
  margin-bottom: 4px;
  border: 1px solid var(--app-border-color, #ebeef5);
  border-radius: 6px;
  background: var(--app-card-bg, #fafafa);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.script-library-item:hover {
  border-color: var(--el-color-primary-light-5, #a0cfff);
  background: rgba(64, 158, 255, 0.06);
}

.script-library-item-name {
  font-size: 13px;
  color: var(--el-text-color-primary, #303133);
  font-weight: 500;
}

.script-library-item-desc {
  font-size: 12px;
  color: var(--app-secondary-text, #909399);
  margin-top: 4px;
  line-height: 1.45;
}

.script-library-loading {
  font-size: 12px;
  color: var(--app-secondary-text, #909399);
  margin-bottom: 4px;
}

.script-library-footer {
  font-size: 11px;
  color: var(--app-secondary-text, #909399);
  line-height: 1.4;
  padding-top: 4px;
  border-top: 1px dashed var(--app-border-color, #ebeef5);
}

.script-library-manage-link {
  color: var(--el-color-primary);
  text-decoration: none;
}

.script-library-manage-link:hover {
  text-decoration: underline;
}

.script-library-footer-sep {
  margin: 0 6px;
  opacity: 0.7;
}
</style>
