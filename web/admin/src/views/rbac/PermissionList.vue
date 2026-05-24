<template>
  <div :class="embedded ? 'tab-panel' : 'page-card'">
    <div v-if="embedded" class="tab-panel-toolbar">
      <el-button type="primary" @click="dialogVisible = true">新增权限点</el-button>
    </div>
    <div v-else class="page-header">
      <h2 class="page-title">权限菜单</h2>
      <el-button type="primary" @click="dialogVisible = true">新增权限点</el-button>
    </div>

    <ListLoadError v-if="loadError" :message="loadError" @retry="load" />

    <el-table v-loading="loading" :data="permissions" border>
      <el-table-column prop="code" label="权限编码" width="220" />
      <el-table-column prop="name" label="名称" width="180" />
      <el-table-column prop="description" label="描述" />
    </el-table>

    <el-empty v-if="!loading && !loadError && !permissions.length" description="暂无权限点">
      <el-button type="primary" @click="dialogVisible = true">新增权限点</el-button>
    </el-empty>

    <el-dialog v-model="dialogVisible" title="新增权限点" width="460px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="编码"><el-input v-model="form.code" placeholder="resource:action" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { createPermission, listPermissions } from '../../api'
import ListLoadError from '../../components/ListLoadError.vue'
import { getListLoadErrorMessage } from '../../utils/listPageLoad'

export default {
  name: 'PermissionList',
  components: { ListLoadError },
  props: {
    embedded: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      loading: false,
      loadError: '',
      permissions: [],
      dialogVisible: false,
      form: { code: '', name: '', description: '' }
    }
  },
  created() {
    this.load()
  },
  methods: {
    async load() {
      this.loading = true
      this.loadError = ''
      try {
        this.permissions = await listPermissions()
      } catch (err) {
        this.loadError = getListLoadErrorMessage(err)
      } finally {
        this.loading = false
      }
    },
    async submit() {
      await createPermission(this.form)
      this.$message.success('权限点已创建')
      this.dialogVisible = false
      this.form = { code: '', name: '', description: '' }
      this.load()
    }
  }
}
</script>

<style scoped>
.tab-panel-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
</style>
