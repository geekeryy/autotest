<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;">
      <el-input v-model="search" placeholder="搜索技能名称..." clearable style="width:260px;" @input="load" />
      <el-button type="primary" @click="showCreate = true">新建 Skill</el-button>
    </div>

    <el-table :data="skills" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="180" />
      <el-table-column prop="label" label="标签" width="150" />
      <el-table-column prop="description" label="描述" show-overflow-tooltip />
      <el-table-column label="工具域" width="200">
        <template #default="{ row }">
          <el-tag v-for="d in (row.toolDomains || [])" :key="d" size="small" style="margin:2px;">{{ d }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" align="center">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="edit(row)">编辑</el-button>
          <el-popconfirm title="确认删除?" @confirm="remove(row.id)">
            <template #reference>
              <el-button link type="danger" size="small">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create / Edit Dialog -->
    <el-dialog v-model="showCreate" :title="editingId ? '编辑 Skill' : '新建 Skill'" width="600px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如 api_coverage_gen" />
        </el-form-item>
        <el-form-item label="标签" required>
          <el-input v-model="form.label" placeholder="如 API 覆盖生成" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="Prompt 模板" required>
          <el-input v-model="form.promptTemplate" type="textarea" :rows="6" placeholder="Go template 语法" />
        </el-form-item>
        <el-form-item label="工具域">
          <el-select v-model="form.toolDomains" multiple style="width:100%;">
            <el-option v-for="d in domains" :key="d" :label="d" :value="d" />
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
import { listAISkills, createAISkill, updateAISkill, deleteAISkill } from '../../../api'

const DOMAINS = ['meta','spec','cases','scenarios','mock','mockset','testdata','paramsource','scripts','runs']

export default {
  name: 'SkillsPanel',
  data() {
    return {
      skills: [], loading: false, search: '',
      showCreate: false, saving: false, editingId: null,
      domains: DOMAINS,
      form: { name: '', label: '', description: '', promptTemplate: '', toolDomains: [] }
    }
  },
  mounted() { this.load() },
  methods: {
    async load() {
      this.loading = true
      try {
        this.skills = await listAISkills()
        if (this.search) {
          const q = this.search.toLowerCase()
          this.skills = this.skills.filter(s => s.name.includes(q) || s.label.includes(q))
        }
      } finally { this.loading = false }
    },
    edit(row) {
      this.editingId = row.id
      this.form = { name: row.name, label: row.label, description: row.description, promptTemplate: row.promptTemplate, toolDomains: row.toolDomains || [] }
      this.showCreate = true
    },
    async submit() {
      this.saving = true
      try {
        if (this.editingId) {
          await updateAISkill(this.editingId, { label: this.form.label, description: this.form.description, promptTemplate: this.form.promptTemplate, toolDomains: this.form.toolDomains })
        } else {
          await createAISkill(this.form)
        }
        this.showCreate = false
        this.editingId = null
        this.form = { name: '', label: '', description: '', promptTemplate: '', toolDomains: [] }
        await this.load()
      } finally { this.saving = false }
    },
    async remove(id) {
      await deleteAISkill(id)
      await this.load()
    }
  }
}
</script>
