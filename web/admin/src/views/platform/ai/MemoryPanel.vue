<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;">
      <div style="display:flex;gap:8px;">
        <el-select v-model="filterType" clearable placeholder="类型" style="width:120px;" @change="load">
          <el-option label="自动" value="auto" />
          <el-option label="项目" value="project" />
          <el-option label="全局" value="global" />
        </el-select>
        <el-select v-model="filterCategory" clearable placeholder="分类" style="width:120px;" @change="load">
          <el-option label="偏好" value="preference" />
          <el-option label="模式" value="pattern" />
          <el-option label="约定" value="convention" />
          <el-option label="断言" value="assertion" />
          <el-option label="环境" value="environment" />
          <el-option label="通用" value="general" />
        </el-select>
      </div>
      <el-button type="primary" @click="showCreate = true">新建记忆</el-button>
    </div>

    <el-table :data="memories" v-loading="loading" stripe>
      <el-table-column prop="key" label="Key" width="160" show-overflow-tooltip />
      <el-table-column prop="content" label="内容" show-overflow-tooltip />
      <el-table-column prop="type" label="类型" width="80">
        <template #default="{ row }">
          <el-tag size="small" :type="row.type === 'global' ? 'danger' : row.type === 'project' ? '' : 'info'">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="category" label="分类" width="90" />
      <el-table-column prop="confidence" label="置信度" width="80" align="center">
        <template #default="{ row }">{{ (row.confidence * 100).toFixed(0) }}%</template>
      </el-table-column>
      <el-table-column prop="refCount" label="引用" width="60" align="center" />
      <el-table-column label="操作" width="100" align="center">
        <template #default="{ row }">
          <el-popconfirm title="确认删除?" @confirm="remove(row.id)">
            <template #reference>
              <el-button link type="danger" size="small">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showCreate" title="新建记忆" width="500px" destroy-on-close>
      <el-form :model="form" label-width="80px">
        <el-form-item label="Key" required>
          <el-input v-model="form.key" placeholder="如 api_convention" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="form.content" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" style="width:100%;">
            <el-option label="通用" value="general" />
            <el-option label="偏好" value="preference" />
            <el-option label="模式" value="pattern" />
            <el-option label="约定" value="convention" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { listAIMemories, saveAIMemory, deleteAIMemory } from '../../../api'

export default {
  name: 'MemoryPanel',
  data() {
    return {
      memories: [], loading: false,
      filterType: '', filterCategory: '',
      showCreate: false, saving: false,
      form: { key: '', content: '', category: 'general' }
    }
  },
  mounted() { this.load() },
  methods: {
    async load() {
      this.loading = true
      try {
        const params = { limit: 100 }
        if (this.filterType) params.type = this.filterType
        if (this.filterCategory) params.category = this.filterCategory
        const res = await listAIMemories(params)
        this.memories = res.items || []
      } finally { this.loading = false }
    },
    async submit() {
      this.saving = true
      try {
        await saveAIMemory(this.form)
        this.showCreate = false
        this.form = { key: '', content: '', category: 'general' }
        await this.load()
      } finally { this.saving = false }
    },
    async remove(id) {
      await deleteAIMemory(id)
      await this.load()
    }
  }
}
</script>
