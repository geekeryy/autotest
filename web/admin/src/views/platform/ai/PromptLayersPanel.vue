<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;">
      <el-input v-model="search" placeholder="搜索名称或域名..." clearable style="width:260px;" @input="load" />
      <el-button type="primary" @click="showCreate = true">新建 Prompt 层</el-button>
    </div>

    <el-table :data="layers" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="200" />
      <el-table-column prop="domain" label="域名" width="120" />
      <el-table-column prop="version" label="版本" width="80" align="center">
        <template #default="{ row }">
          <el-tag size="small">v{{ row.version }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="content" label="内容" show-overflow-tooltip />
      <el-table-column label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" align="center">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="enable(row)" :disabled="row.enabled">启用</el-button>
          <el-button link type="primary" size="small" @click="edit(row)">编辑</el-button>
          <el-popconfirm title="确认删除?" @confirm="remove(row.id)">
            <template #reference>
              <el-button link type="danger" size="small">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showCreate" :title="editingName ? '编辑 Prompt 层' : '新建 Prompt 层'" width="600px" destroy-on-close>
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" :disabled="!!editingName" placeholder="如 assistant_base" />
        </el-form-item>
        <el-form-item label="域名">
          <el-input v-model="form.domain" placeholder="如 cases, scenarios" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="form.content" type="textarea" :rows="8" />
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
import { listPromptLayers, createPromptLayer, enablePromptLayerVersion, deletePromptLayer, updatePromptLayerByName } from '../../../api'

export default {
  name: 'PromptLayersPanel',
  data() {
    return {
      layers: [], loading: false, search: '',
      showCreate: false, saving: false, editingName: null,
      form: { name: '', domain: '', content: '' }
    }
  },
  mounted() { this.load() },
  methods: {
    async load() {
      this.loading = true
      try {
        const params = {}
        if (this.search) params.domain = this.search
        this.layers = await listPromptLayers(params)
      } finally { this.loading = false }
    },
    edit(row) {
      this.editingName = row.name
      this.form = { name: row.name, domain: row.domain, content: row.content }
      this.showCreate = true
    },
    async enable(row) {
      await enablePromptLayerVersion(row.id)
      await this.load()
    },
    async submit() {
      this.saving = true
      try {
        if (this.editingName) {
          await updatePromptLayerByName(this.editingName, { content: this.form.content })
        } else {
          await createPromptLayer(this.form)
        }
        this.showCreate = false
        this.editingName = null
        this.form = { name: '', domain: '', content: '' }
        await this.load()
      } finally { this.saving = false }
    },
    async remove(id) {
      await deletePromptLayer(id)
      await this.load()
    }
  }
}
</script>
