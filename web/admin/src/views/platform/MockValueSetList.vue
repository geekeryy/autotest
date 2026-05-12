<template>
  <div class="page-card mock-value-sets-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">命名值集合</h2>
        <p class="page-subtitle">
          维护项目级离散值池，供模板里的 <code v-pre>{{$mock.set.&lt;key&gt;}}</code> 系列引用。Runner 在每次请求时按权重随机抽样、按索引取值或按运行游标顺序遍历。
        </p>
      </div>
      <el-button type="primary" :disabled="!projectId" @click="openCreate">新增集合</el-button>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理命名值集合。"
    />

    <el-card v-if="projectId" class="syntax-card" shadow="never">
      <template #header>
        <div class="syntax-header">
          <span>模板语法</span>
          <span class="syntax-tip">点击「复制」可快速粘贴到请求模板、场景步骤或 Mock 响应体中。</span>
        </div>
      </template>
      <ul class="syntax-list">
        <li v-for="item in mockSetSyntax" :key="item.name">
          <div class="syntax-row">
            <code class="syntax-token">{{ item.name }}</code>
            <span class="syntax-arrow">→</span>
            <code class="syntax-example">{{ item.example }}</code>
            <el-button type="primary" link size="small" @click="copy(item.example)">复制</el-button>
          </div>
          <div class="syntax-desc">{{ item.description }}</div>
        </li>
      </ul>
    </el-card>

    <el-table v-if="projectId" :data="sets" border row-key="id" v-loading="loading" class="sets-table">
      <el-table-column prop="key" label="key" width="180" show-overflow-tooltip />
      <el-table-column prop="name" label="名称" min-width="160" show-overflow-tooltip />
      <el-table-column label="值预览" min-width="240">
        <template #default="{ row }">{{ valuesPreview(row) }}</template>
      </el-table-column>
      <el-table-column label="数量" width="80">
        <template #default="{ row }">{{ (row.values || []).length }}</template>
      </el-table-column>
      <el-table-column label="权重" width="80">
        <template #default="{ row }">
          <el-tag v-if="hasWeights(row)" size="small" type="success">已配置</el-tag>
          <el-tag v-else size="small" type="info">均匀</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
      <el-table-column label="更新时间" width="178">
        <template #default="{ row }">{{ formatDateTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" link :loading="deletingId === row.id" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="projectId && !loading && !sets.length" description="暂无命名值集合，新增后即可在模板里引用" />

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑命名值集合' : '新增命名值集合'"
      width="720px"
      align-center
      destroy-on-close
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="key" required>
          <el-input
            v-model="form.key"
            :disabled="!!editingId"
            placeholder="项目内唯一，仅允许字母 / 数字 / 下划线 / 中划线"
          />
          <div class="form-hint">key 一旦保存不可修改；如需重命名请新建集合并迁移引用。</div>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="便于识别的中文名" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选；说明用途或业务含义" />
        </el-form-item>
        <el-form-item label="候选值" required>
          <el-input
            v-model="form.valuesText"
            type="textarea"
            :autosize="{ minRows: 6, maxRows: 16 }"
            placeholder="每行一项；失焦时自动 trim 空行并去重"
            @blur="normalizeValuesText"
          />
          <div class="form-hint">当前 {{ valuesCount }} 项；模板里 <code v-pre>{{$mock.set.&lt;key&gt;[N]}}</code> 的 N 取值范围 0 ～ {{ Math.max(0, valuesCount - 1) }}。</div>
        </el-form-item>
        <el-form-item label="权重">
          <el-input
            v-model="form.weightsText"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 12 }"
            placeholder="可选；每行一个非负数字。留空则等同于均匀随机"
            @blur="normalizeWeightsText"
          />
          <div class="form-hint">如填写，行数必须等于候选值的行数。</div>
        </el-form-item>
        <el-form-item label="测试随机">
          <div class="distribution-preview">
            <el-button size="small" :disabled="!previewReady" @click="runSampleDistribution">本地随机抽样 100 次</el-button>
            <span v-if="distributionRows.length === 0" class="dist-empty">点击按钮即可在不保存的情况下，本地按当前 values + weights 验证分布。</span>
            <ul v-else class="dist-list">
              <li v-for="row in distributionRows" :key="row.value">
                <code class="dist-value">{{ row.value }}</code>
                <span class="dist-bar" :style="{ width: row.percent + '%' }"></span>
                <span class="dist-count">{{ row.count }} ({{ row.percent }}%)</span>
              </li>
            </ul>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createMockValueSet,
  deleteMockValueSet,
  listMockValueSets,
  updateMockValueSet
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import { mockSetSyntax } from '../../utils/templateTokens'

const KEY_PATTERN = /^[A-Za-z0-9_-]+$/

function pad2(n) {
  return String(n).padStart(2, '0')
}

function formatDateTime(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())} ` +
    `${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`
}

function uniqueLines(text) {
  const seen = new Set()
  const out = []
  for (const raw of String(text || '').split(/\r?\n/)) {
    const trimmed = raw.trim()
    if (!trimmed) continue
    if (seen.has(trimmed)) continue
    seen.add(trimmed)
    out.push(trimmed)
  }
  return out
}

function parseWeights(text) {
  return String(text || '')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .map((line) => Number(line))
}

export default {
  name: 'MockValueSetList',
  data() {
    return {
      projectState,
      loading: false,
      sets: [],
      dialogVisible: false,
      editingId: null,
      saving: false,
      deletingId: null,
      mockSetSyntax,
      form: {
        key: '',
        name: '',
        description: '',
        valuesText: '',
        weightsText: ''
      },
      distributionRows: []
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId || ''
    },
    valuesCount() {
      return uniqueLines(this.form.valuesText).length
    },
    previewReady() {
      const values = uniqueLines(this.form.valuesText)
      if (values.length === 0) return false
      const weightsRaw = String(this.form.weightsText || '').trim()
      if (!weightsRaw) return true
      const weights = parseWeights(this.form.weightsText)
      if (weights.length !== values.length) return false
      return weights.every((w) => Number.isFinite(w) && w >= 0)
    }
  },
  watch: {
    projectId: {
      immediate: true,
      handler(val) {
        if (val) this.loadList()
        else this.sets = []
      }
    }
  },
  created() {
    loadGlobalProjects()
  },
  methods: {
    formatDateTime,
    valuesPreview(row) {
      const values = Array.isArray(row?.values) ? row.values : []
      if (values.length === 0) return ''
      const head = values.slice(0, 5).join(', ')
      return values.length > 5 ? `${head}, …` : head
    },
    hasWeights(row) {
      return Array.isArray(row?.weights) && row.weights.length > 0
    },
    async loadList() {
      if (!this.projectId) return
      this.loading = true
      try {
        this.sets = await listMockValueSets(this.projectId)
      } catch (error) {
        this.$message.error(`加载命名值集合失败：${error?.message || error}`)
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.editingId = null
      this.form = { key: '', name: '', description: '', valuesText: '', weightsText: '' }
      this.distributionRows = []
      this.dialogVisible = true
    },
    openEdit(row) {
      this.editingId = row.id
      this.form = {
        key: row.key || '',
        name: row.name || '',
        description: row.description || '',
        valuesText: (row.values || []).join('\n'),
        weightsText: (row.weights || []).join('\n')
      }
      this.distributionRows = []
      this.dialogVisible = true
    },
    normalizeValuesText() {
      this.form.valuesText = uniqueLines(this.form.valuesText).join('\n')
    },
    normalizeWeightsText() {
      const cleaned = String(this.form.weightsText || '')
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter((line) => line.length > 0)
        .join('\n')
      this.form.weightsText = cleaned
    },
    runSampleDistribution() {
      const values = uniqueLines(this.form.valuesText)
      if (values.length === 0) return
      const weights = parseWeights(this.form.weightsText)
      const useWeights = weights.length === values.length && weights.every((w) => Number.isFinite(w) && w >= 0)
      const totalWeight = useWeights ? weights.reduce((acc, w) => acc + Math.max(0, w), 0) : 0
      const counts = new Map(values.map((v) => [v, 0]))
      const SAMPLES = 100
      for (let i = 0; i < SAMPLES; i++) {
        let pick = values[0]
        if (useWeights && totalWeight > 0) {
          const target = Math.random() * totalWeight
          let acc = 0
          for (let j = 0; j < values.length; j++) {
            const w = Math.max(0, weights[j])
            if (w <= 0) continue
            acc += w
            if (target < acc) {
              pick = values[j]
              break
            }
          }
        } else {
          pick = values[Math.floor(Math.random() * values.length)]
        }
        counts.set(pick, (counts.get(pick) || 0) + 1)
      }
      this.distributionRows = values.map((value) => {
        const count = counts.get(value) || 0
        const percent = Math.round((count / SAMPLES) * 100)
        return { value, count, percent }
      })
    },
    async submit() {
      if (!this.editingId && !KEY_PATTERN.test(this.form.key.trim())) {
        this.$message.error('key 必须仅包含字母、数字、下划线或中划线')
        return
      }
      const values = uniqueLines(this.form.valuesText)
      if (values.length === 0) {
        this.$message.error('请至少填写 1 个候选值')
        return
      }
      const weightsText = String(this.form.weightsText || '').trim()
      let weights = []
      if (weightsText) {
        weights = parseWeights(this.form.weightsText)
        if (weights.length !== values.length) {
          this.$message.error('权重行数必须与候选值行数一致；如不需要权重请清空该字段')
          return
        }
        if (weights.some((w) => !Number.isFinite(w) || w < 0)) {
          this.$message.error('权重必须是非负数字')
          return
        }
      }
      this.saving = true
      try {
        if (this.editingId) {
          await updateMockValueSet(this.projectId, this.editingId, {
            name: this.form.name.trim(),
            description: this.form.description.trim(),
            values,
            weights
          })
          this.$message.success('已保存')
        } else {
          await createMockValueSet(this.projectId, {
            key: this.form.key.trim(),
            name: this.form.name.trim(),
            description: this.form.description.trim(),
            values,
            weights
          })
          this.$message.success('已创建')
        }
        this.dialogVisible = false
        await this.loadList()
      } catch (error) {
        const message = error?.response?.data?.error || error?.message || String(error)
        this.$message.error(`保存失败：${message}`)
      } finally {
        this.saving = false
      }
    },
    async remove(row) {
      try {
        await this.$confirm(`确定删除命名值集合「${row.name || row.key}」？引用该集合的模板将报错。`, '删除确认', {
          type: 'warning'
        })
      } catch {
        return
      }
      this.deletingId = row.id
      try {
        await deleteMockValueSet(this.projectId, row.id)
        this.$message.success('已删除')
        await this.loadList()
      } catch (error) {
        this.$message.error(`删除失败：${error?.message || error}`)
      } finally {
        this.deletingId = null
      }
    },
    async copy(text) {
      try {
        if (navigator?.clipboard?.writeText) {
          await navigator.clipboard.writeText(text)
        } else {
          const el = document.createElement('textarea')
          el.value = text
          document.body.appendChild(el)
          el.select()
          document.execCommand('copy')
          document.body.removeChild(el)
        }
        this.$message.success('已复制到剪贴板')
      } catch (error) {
        this.$message.error(`复制失败：${error?.message || error}`)
      }
    }
  }
}
</script>

<style scoped>
.mock-value-sets-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.page-subtitle {
  margin: 4px 0 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.page-subtitle code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  background: var(--app-fill-color-light, #f1f5f9);
  padding: 1px 4px;
  border-radius: 3px;
}

.project-hint {
  margin-bottom: 8px;
}

.syntax-card {
  border-radius: 8px;
}

.syntax-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.syntax-tip {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.syntax-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.syntax-list li {
  border-bottom: 1px dashed var(--el-border-color-lighter);
  padding-bottom: 6px;
}

.syntax-list li:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.syntax-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.syntax-token,
.syntax-example {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  background: var(--app-fill-color-light, #f1f5f9);
  padding: 2px 6px;
  border-radius: 4px;
}

.syntax-arrow {
  color: var(--app-secondary-text);
}

.syntax-desc {
  margin-top: 4px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.sets-table {
  margin-top: 4px;
}

.form-hint {
  margin-top: 4px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.form-hint code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  background: var(--app-fill-color-light, #f1f5f9);
  padding: 1px 4px;
  border-radius: 3px;
}

.distribution-preview {
  width: 100%;
}

.dist-empty {
  margin-left: 12px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.dist-list {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dist-list li {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dist-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  width: 120px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dist-bar {
  display: inline-block;
  height: 10px;
  background: var(--el-color-primary-light-5, #95d5b2);
  border-radius: 4px;
  min-width: 4px;
}

.dist-count {
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text);
  min-width: 80px;
}
</style>
