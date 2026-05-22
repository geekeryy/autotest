<template>
  <div class="page-card test-data-page">
    <div class="page-header test-data-header">
      <div>
        <h2 class="page-title">测试数据表</h2>
        <p class="page-subtitle">
          项目级，跨服务可引用；列支持「手动输入 / 数据模拟函数 / AI 自动生成」三种生成方式，运行前以 <code class="inline-code">{{ inlineExampleHint }}</code> 占位符在请求模板中按行/按列取值。
        </p>
      </div>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理测试数据表。"
    />

    <div v-else class="test-data-layout">
      <section class="table-list-panel">
        <div class="panel-title-row">
          <div>
            <h3 class="panel-title">数据表列表</h3>
            <p class="panel-subtitle">点击切换右侧详情。</p>
          </div>
          <el-button type="primary" :disabled="!projectId" @click="openCreateTable">新增数据表</el-button>
        </div>

        <el-input
          v-model="searchText"
          class="table-search"
          placeholder="搜索名称或 Key"
          clearable
        />

        <el-table
          :data="filteredTables"
          border
          row-key="id"
          highlight-current-row
          v-loading="loadingTables"
          :row-class-name="tableRowClassName"
          empty-text="暂无数据表，新增后即可配置列与数据行"
          @row-click="selectTable"
        >
          <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
          <el-table-column prop="key" label="Key" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button type="danger" link :loading="deletingTableId === row.id" @click.stop="removeTable(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>

      <section class="table-detail-panel" v-loading="loadingDetail">
        <template v-if="selectedTable">
          <div class="detail-header">
            <el-form :model="tableForm" inline label-width="60px" class="detail-header-form">
              <el-form-item label="表名" class="header-form-item">
                <el-input v-model="tableForm.name" placeholder="例如：测试用户" />
              </el-form-item>
              <el-form-item label="Key" class="header-form-item">
                <el-input v-model="tableForm.key" placeholder="testUsers" />
              </el-form-item>
              <el-form-item label="说明" class="header-form-item header-form-item--wide">
                <el-input v-model="tableForm.description" placeholder="可选，便于其他成员理解用途" />
              </el-form-item>
            </el-form>
            <div class="detail-header-actions">
              <el-button :disabled="!canCopyExample" @click="copyInlineExample">复制内联示例</el-button>
              <el-button type="primary" :loading="savingTable" @click="saveTable">保存表配置</el-button>
            </div>
          </div>
          <div class="detail-key-tip">
            <span>字母/数字/下划线/中划线，项目内唯一。</span>
            <span>请求模板中可使用 <code class="inline-code">{{ inlineExample }}</code>（默认首行）或 <code class="inline-code">{{ inlineFilteredExample }}</code>（按等值过滤取首行）。</span>
          </div>

          <div class="section-title-row">
            <h3 class="section-title">列定义</h3>
            <span class="section-subtitle">行内编辑列定义；保存后同步到服务端。</span>
          </div>
          <el-table
            :data="tableForm.columns"
            border
            row-key="_uiId"
            class="column-table"
            empty-text="暂无列，点击下方「+ 新增列」开始定义"
          >
            <el-table-column label="Key" min-width="140">
              <template #default="{ row }">
                <el-input v-model="row.key" size="small" placeholder="userId" @change="onColumnKeyChange(row)" />
              </template>
            </el-table-column>
            <el-table-column label="名称" min-width="140">
              <template #default="{ row }">
                <el-input v-model="row.name" size="small" placeholder="可选，列展示名" />
              </template>
            </el-table-column>
            <el-table-column label="类型" width="120">
              <template #default="{ row }">
                <el-select v-model="row.type" size="small">
                  <el-option v-for="t in columnTypes" :key="t" :label="t" :value="t" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="必填" width="64" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.required" />
              </template>
            </el-table-column>
            <el-table-column label="描述" min-width="160">
              <template #default="{ row }">
                <el-input v-model="row.description" size="small" placeholder="可选，便于 AI 生成时理解列含义" />
              </template>
            </el-table-column>
            <el-table-column label="生成方式" min-width="320">
              <template #default="{ row }">
                <div class="generator-cell">
                  <el-radio-group v-model="row.generator.kind" size="small">
                    <el-radio-button value="manual">手动</el-radio-button>
                    <el-radio-button value="mock">Mock</el-radio-button>
                    <el-radio-button value="ai">AI</el-radio-button>
                  </el-radio-group>
                  <template v-if="row.generator.kind === 'mock'">
                    <el-select
                      v-model="row.generator.mock.helper"
                      size="small"
                      filterable
                      placeholder="选择 helper"
                      class="generator-input"
                      :loading="mockHelpersLoading"
                    >
                      <el-option v-for="h in mockHelpers" :key="h.name" :label="h.name" :value="h.name">
                        <span class="ds-helper-name">{{ h.name }}</span>
                        <span class="ds-helper-desc">{{ h.description }}</span>
                      </el-option>
                    </el-select>
                    <el-input
                      v-model="row.generator.mock.argsRaw"
                      size="small"
                      placeholder="args 用逗号分隔，例如 1,100"
                      class="generator-input"
                    />
                    <div v-if="helperDescription(row.generator.mock.helper)" class="generator-tip">
                      {{ helperDescription(row.generator.mock.helper) }}
                    </div>
                  </template>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="76" fixed="right">
              <template #default="{ $index }">
                <el-button type="danger" link @click="removeColumn($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="add-column-row">
            <el-button @click="addColumn">+ 新增列</el-button>
          </div>

          <div class="section-title-row row-section-title">
            <h3 class="section-title">数据行</h3>
            <span v-if="rowsDirty" class="section-subtitle section-subtitle--dirty">存在未保存的修改</span>
            <span v-else class="section-subtitle">单元格双击可编辑；修改后点击「保存当前行」整体提交。</span>
          </div>

          <el-alert
            v-if="aiNeeded && !aiProviders.length"
            class="row-alert"
            type="warning"
            show-icon
            :closable="false"
            title="当前项目尚未配置 AI 提供商"
            description="存在使用 AI 生成方式的列。请先在「项目管理 / AI 提供商」配置至少一个启用的提供商，并可在「项目管理 / Prompt 管理」中按 generate_case_data 维护 SystemPrompt。"
          />

          <div class="row-toolbar">
            <el-button :disabled="!hasColumns" @click="openGenerateDialog">批量生成 N 行</el-button>
            <el-button :disabled="!hasColumns" @click="addRow">新增空行</el-button>
            <el-button :disabled="!workingRows.length" @click="clearAllRows">清空所有行</el-button>
            <el-button type="primary" :loading="savingRows" :disabled="!hasColumns" @click="saveRows">保存当前行</el-button>
            <el-button :disabled="!workingRows.length" @click="exportRowsAsJSON">导出 JSON</el-button>
          </div>

          <el-table
            :data="workingRows"
            border
            class="row-table"
            empty-text="暂无数据行，可手动新增或批量生成"
          >
            <el-table-column type="index" label="#" width="60" />
            <el-table-column
              v-for="col in tableForm.columns"
              :key="col._uiId"
              :label="columnHeader(col)"
              min-width="160"
            >
              <template #default="{ row, $index }">
                <template v-if="isEditingCell($index, col.key)">
                  <el-input
                    ref="editingInput"
                    v-model="row.values[col.key]"
                    size="small"
                    type="textarea"
                    :autosize="{ minRows: 1, maxRows: 4 }"
                    @blur="commitCellEdit"
                  />
                </template>
                <span v-else class="cell-display" @dblclick="startCellEdit($index, col.key)">{{ rowCellText(row, col.key) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row, $index }">
                <el-button type="primary" link :disabled="!row.id" @click.stop="openRegenerateDialog(row, $index)">按列重新生成</el-button>
                <el-button type="danger" link @click.stop="removeRow($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </template>
        <el-empty v-else class="detail-empty" description="请选择左侧数据表，或点击「新增数据表」创建一个" />
      </section>
    </div>

    <el-dialog v-model="createDialogVisible" title="新增数据表" width="520px" align-center>
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="表名" required>
          <el-input v-model="createForm.name" placeholder="例如：测试用户" />
        </el-form-item>
        <el-form-item label="Key" required>
          <el-input v-model="createForm.key" placeholder="testUsers" />
          <div class="field-tip">字母/数字/下划线/中划线，项目内唯一</div>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="createForm.description" type="textarea" :rows="2" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingTable" @click="submitCreateTable">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="generateDialogVisible" title="批量生成测试数据" width="520px" align-center>
      <el-form :model="generateForm" label-width="120px">
        <el-form-item label="行数">
          <el-input-number v-model="generateForm.rowCount" :min="1" :max="200" />
        </el-form-item>
        <el-form-item label="保留已手填">
          <el-checkbox v-model="generateForm.preserveManual">保留已存在的非空单元格</el-checkbox>
        </el-form-item>
        <el-alert
          v-if="aiNeeded"
          class="dialog-tip"
          type="info"
          show-icon
          :closable="false"
          title="AI 列将使用项目 Prompt（generate_case_data）配置；未配置时回退到项目默认 AI 提供商。"
        />
      </el-form>
      <template #footer>
        <el-button @click="generateDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="generatingRows" @click="submitGenerateRows">开始生成</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="regenerateDialogVisible" title="按列重新生成" width="480px" align-center>
      <p class="regenerate-tip">选择需要重新生成的列（仅 mock / ai 列支持）：</p>
      <el-checkbox-group v-model="regenerateForm.columnKeys" class="regenerate-columns">
        <el-checkbox v-for="col in regenerableColumns" :key="col.key" :value="col.key">
          {{ col.name || col.key }}
          <span class="regenerate-kind">（{{ col.generator.kind }}）</span>
        </el-checkbox>
      </el-checkbox-group>
      <el-alert
        v-if="regenerateNeedsAI"
        class="dialog-tip"
        type="info"
        show-icon
        :closable="false"
        title="AI 列将使用项目 Prompt（generate_case_data）配置；未配置时回退到项目默认 AI 提供商。"
      />
      <template #footer>
        <el-button @click="regenerateDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="regenerateLoading"
          :disabled="!regenerateForm.columnKeys.length"
          @click="submitRegenerate"
        >重新生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createTestDataTable,
  deleteTestDataTable,
  generateTestDataRows,
  getTestDataTable,
  listAIProviders,
  listMockHelpers,
  listTestDataRows,
  listTestDataTables,
  regenerateTestDataCells,
  replaceTestDataRows,
  updateTestDataTable
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

const COLUMN_TYPES = ['string', 'integer', 'number', 'boolean', 'json']
const KEY_PATTERN = /^[A-Za-z0-9_-]+$/

let uiIdSeq = 0
function nextUIId() {
  uiIdSeq += 1
  return `c${uiIdSeq}`
}

function newColumn(initial = {}) {
  const generator = initial.generator || {}
  const mock = generator.mock || {}
  const args = Array.isArray(mock.args) ? mock.args.map((v) => String(v ?? '')) : []
  return {
    _uiId: nextUIId(),
    _originalKey: initial.key || '',
    key: initial.key || '',
    name: initial.name || '',
    type: COLUMN_TYPES.includes(initial.type) ? initial.type : 'string',
    required: !!initial.required,
    description: initial.description || '',
    generator: {
      kind: generator.kind || 'manual',
      mock: {
        helper: mock.helper || '',
        args,
        argsRaw: args.join(', ')
      }
    }
  }
}

function buildColumnPayload(col) {
  const args = String(col.generator.mock.argsRaw || '')
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
  const payload = {
    key: col.key,
    name: col.name || '',
    type: col.type,
    required: !!col.required,
    description: col.description || '',
    generator: {
      kind: col.generator.kind
    }
  }
  if (col.generator.kind === 'mock') {
    payload.generator.mock = {
      helper: col.generator.mock.helper || '',
      args
    }
  } else if (col.generator.kind === 'ai') {
    payload.generator.ai = {}
  }
  return payload
}

function normalizeRow(row, columns) {
  const source = row?.values || {}
  const values = {}
  for (const col of columns) {
    const v = source[col.key]
    values[col.key] = v == null ? '' : String(v)
  }
  for (const [key, value] of Object.entries(source)) {
    if (!(key in values)) values[key] = value == null ? '' : String(value)
  }
  return {
    id: row?.id || '',
    rowIndex: row?.rowIndex ?? 0,
    values,
    createdAt: row?.createdAt || '',
    updatedAt: row?.updatedAt || ''
  }
}

function emptyRowFor(columns) {
  const values = {}
  for (const col of columns) values[col.key] = ''
  return { id: '', rowIndex: 0, values, createdAt: '', updatedAt: '' }
}

export default {
  name: 'TestDataTableList',
  data() {
    return {
      tables: [],
      selectedTableId: '',
      searchText: '',
      loadingTables: false,
      loadingDetail: false,
      savingTable: false,
      savingRows: false,
      deletingTableId: '',

      tableForm: this.emptyTableForm(),
      workingRows: [],
      rowsBaseline: '[]',

      editingCell: null,

      mockHelpers: [],
      mockHelpersLoading: false,
      mockHelpersLoaded: false,
      aiProviders: [],

      createDialogVisible: false,
      createForm: { name: '', key: '', description: '' },
      creatingTable: false,

      generateDialogVisible: false,
      generateForm: { rowCount: 5, preserveManual: true },
      generatingRows: false,

      regenerateDialogVisible: false,
      regenerateForm: { columnKeys: [] },
      regenerateRow: null,
      regenerateRowIndex: -1,
      regenerateLoading: false,

      columnTypes: COLUMN_TYPES
    }
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    selectedTable() {
      return this.tables.find((t) => t.id === this.selectedTableId) || null
    },
    filteredTables() {
      const text = this.searchText.trim().toLowerCase()
      if (!text) return this.tables
      return this.tables.filter((t) => {
        return (t.name || '').toLowerCase().includes(text) || (t.key || '').toLowerCase().includes(text)
      })
    },
    columnKeys() {
      return this.tableForm.columns.map((c) => String(c.key || '').trim()).filter(Boolean)
    },
    hasColumns() {
      return this.columnKeys.length > 0
    },
    aiNeeded() {
      return this.tableForm.columns.some((c) => c.generator.kind === 'ai')
    },
    canCopyExample() {
      return !!this.tableForm.key && this.columnKeys.length > 0
    },
    inlineExampleHint() {
      return '{{$ds.<tableKey>.<colKey>}}'
    },
    inlineExample() {
      const tableKey = this.tableForm.key || '<tableKey>'
      const colKey = this.columnKeys[0] || '<colKey>'
      return `{{$ds.${tableKey}.${colKey}}}`
    },
    inlineFilteredExample() {
      const tableKey = this.tableForm.key || '<tableKey>'
      const colKey = this.columnKeys[0] || '<colKey>'
      const filterCol = this.columnKeys[1] || this.columnKeys[0] || '<filterCol>'
      return `{{$ds.${tableKey}[${filterCol}=<value>].${colKey}}}`
    },
    rowsDirty() {
      return JSON.stringify(this.workingRows.map((r) => r.values)) !== this.rowsBaseline
    },
    regenerableColumns() {
      return this.tableForm.columns.filter((c) => c.generator.kind !== 'manual' && c.key)
    },
    regenerateNeedsAI() {
      return this.regenerableColumns.some(
        (c) => c.generator.kind === 'ai' && this.regenerateForm.columnKeys.includes(c.key)
      )
    }
  },
  watch: {
    projectId() {
      this.selectedTableId = ''
      this.resetDetailState()
      this.loadInitial()
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.loadInitial()
  },
  methods: {
    emptyTableForm() {
      return { name: '', key: '', description: '', columns: [] }
    },
    resetDetailState() {
      this.tableForm = this.emptyTableForm()
      this.workingRows = []
      this.rowsBaseline = '[]'
      this.editingCell = null
    },
    async loadInitial() {
      if (!this.projectId) {
        this.tables = []
        this.aiProviders = []
        return
      }
      await Promise.all([this.loadTables(), this.ensureMockHelpers(), this.loadAIProviders()])
    },
    async loadTables() {
      this.loadingTables = true
      try {
        this.tables = await listTestDataTables(this.projectId)
        if (this.selectedTableId && !this.tables.some((t) => t.id === this.selectedTableId)) {
          this.selectedTableId = ''
          this.resetDetailState()
        }
      } finally {
        this.loadingTables = false
      }
    },
    async loadAIProviders() {
      try {
        this.aiProviders = await listAIProviders()
      } catch {
        this.aiProviders = []
      }
    },
    async ensureMockHelpers() {
      if (this.mockHelpersLoaded || this.mockHelpersLoading) return
      this.mockHelpersLoading = true
      try {
        this.mockHelpers = await listMockHelpers()
        this.mockHelpersLoaded = true
      } catch {
        this.mockHelpers = []
      } finally {
        this.mockHelpersLoading = false
      }
    },
    helperDescription(helper) {
      const found = this.mockHelpers.find((h) => h.name === helper)
      return found ? found.description : ''
    },
    columnHeader(col) {
      return col.name || col.key || '(未命名)'
    },
    tableRowClassName({ row }) {
      return row.id === this.selectedTableId ? 'is-selected-table' : ''
    },
    async selectTable(row) {
      if (!row || row.id === this.selectedTableId) return
      this.selectedTableId = row.id
      await this.loadTableDetail(row.id)
    },
    async loadTableDetail(tableId) {
      this.loadingDetail = true
      this.editingCell = null
      try {
        const [table, rows] = await Promise.all([
          getTestDataTable(this.projectId, tableId),
          listTestDataRows(this.projectId, tableId)
        ])
        const cols = (table?.columns || []).map((c) => newColumn(c))
        this.tableForm = {
          name: table?.name || '',
          key: table?.key || '',
          description: table?.description || '',
          columns: cols
        }
        this.workingRows = rows.map((r) => normalizeRow(r, cols))
        this.rowsBaseline = JSON.stringify(this.workingRows.map((r) => r.values))
      } finally {
        this.loadingDetail = false
      }
    },
    openCreateTable() {
      if (!this.projectId) {
        this.$message.warning('请先选择项目')
        return
      }
      this.createForm = { name: '', key: '', description: '' }
      this.createDialogVisible = true
    },
    validateKey(key, label = 'Key') {
      const value = String(key || '').trim()
      if (!value) {
        this.$message.error(`${label} 不能为空`)
        return null
      }
      if (!KEY_PATTERN.test(value)) {
        this.$message.error(`${label} 仅支持字母/数字/下划线/中划线`)
        return null
      }
      return value
    },
    async submitCreateTable() {
      const name = String(this.createForm.name || '').trim()
      if (!name) {
        this.$message.error('请输入表名')
        return
      }
      const key = this.validateKey(this.createForm.key)
      if (!key) return
      this.creatingTable = true
      try {
        const created = await createTestDataTable(this.projectId, {
          name,
          key,
          description: this.createForm.description || '',
          columns: []
        })
        this.$message.success('数据表已创建')
        this.createDialogVisible = false
        await this.loadTables()
        const id = created?.id
        if (id) {
          this.selectedTableId = id
          await this.loadTableDetail(id)
        }
      } finally {
        this.creatingTable = false
      }
    },
    async removeTable(row) {
      await this.$confirm(
        `确认删除数据表「${row.name || row.key}」？所有数据行也将被删除，引用该表的接口请求将无法继续解析。`,
        '提示'
      )
      this.deletingTableId = row.id
      try {
        await deleteTestDataTable(this.projectId, row.id)
        this.$message.success('数据表已删除')
        if (this.selectedTableId === row.id) {
          this.selectedTableId = ''
          this.resetDetailState()
        }
        await this.loadTables()
      } finally {
        this.deletingTableId = ''
      }
    },
    addColumn() {
      const col = newColumn({})
      this.tableForm.columns.push(col)
    },
    removeColumn(index) {
      const col = this.tableForm.columns[index]
      if (!col) return
      const removedKey = col.key
      this.tableForm.columns.splice(index, 1)
      if (removedKey) {
        for (const row of this.workingRows) {
          delete row.values[removedKey]
        }
      }
    },
    onColumnKeyChange(col) {
      const oldKey = col._originalKey
      const newKey = String(col.key || '').trim()
      if (oldKey === newKey) return
      if (!newKey) return
      for (const row of this.workingRows) {
        if (oldKey && Object.prototype.hasOwnProperty.call(row.values, oldKey)) {
          row.values[newKey] = row.values[oldKey]
          delete row.values[oldKey]
        } else if (row.values[newKey] == null) {
          row.values[newKey] = ''
        }
      }
      col._originalKey = newKey
    },
    buildColumnsPayload() {
      const seen = new Set()
      const cols = []
      for (const col of this.tableForm.columns) {
        const key = this.validateKey(col.key, '列 Key')
        if (!key) return null
        if (seen.has(key)) {
          this.$message.error(`列 Key 重复：${key}`)
          return null
        }
        seen.add(key)
        if (!COLUMN_TYPES.includes(col.type)) {
          this.$message.error(`列 ${key} 类型不合法`)
          return null
        }
        if (col.generator.kind === 'mock') {
          if (!col.generator.mock.helper) {
            this.$message.error(`列 ${key} 缺少 Mock helper`)
            return null
          }
        }
        cols.push(buildColumnPayload({ ...col, key }))
      }
      return cols
    },
    async saveTable() {
      if (!this.selectedTable) return
      const name = String(this.tableForm.name || '').trim()
      if (!name) {
        this.$message.error('请输入表名')
        return
      }
      const key = this.validateKey(this.tableForm.key)
      if (!key) return
      const cols = this.buildColumnsPayload()
      if (cols == null) return
      this.savingTable = true
      try {
        await updateTestDataTable(this.projectId, this.selectedTableId, {
          name,
          key,
          description: this.tableForm.description || '',
          columns: cols
        })
        this.$message.success('表配置已保存')
        await this.loadTables()
        await this.loadTableDetail(this.selectedTableId)
      } finally {
        this.savingTable = false
      }
    },
    isEditingCell(rowIndex, colKey) {
      return this.editingCell && this.editingCell.rowIndex === rowIndex && this.editingCell.colKey === colKey
    },
    startCellEdit(rowIndex, colKey) {
      this.editingCell = { rowIndex, colKey }
      this.$nextTick(() => {
        const ref = this.$refs.editingInput
        const target = Array.isArray(ref) ? ref[0] : ref
        target?.focus?.()
      })
    },
    commitCellEdit() {
      this.editingCell = null
    },
    rowCellText(row, colKey) {
      const v = row?.values?.[colKey]
      if (v == null || v === '') return '-'
      return String(v)
    },
    addRow() {
      this.workingRows.push(emptyRowFor(this.tableForm.columns))
    },
    removeRow(index) {
      this.workingRows.splice(index, 1)
    },
    async clearAllRows() {
      if (!this.workingRows.length) return
      await this.$confirm('确认清空当前所有数据行？此操作仅作用于本地未保存状态，点击「保存当前行」后才会同步到服务端。', '提示')
      this.workingRows = []
    },
    async saveRows() {
      if (!this.selectedTableId) return
      if (!this.tableForm.columns.length) {
        this.$message.error('请先定义列')
        return
      }
      const payload = this.workingRows.map((row) => {
        const values = {}
        for (const col of this.tableForm.columns) {
          const v = row.values?.[col.key]
          values[col.key] = v == null ? '' : String(v)
        }
        return { values }
      })
      this.savingRows = true
      try {
        await replaceTestDataRows(this.projectId, this.selectedTableId, payload)
        this.$message.success('数据行已保存')
        await this.loadTableDetail(this.selectedTableId)
      } finally {
        this.savingRows = false
      }
    },
    exportRowsAsJSON() {
      const rows = this.workingRows.map((row) => ({ values: { ...row.values } }))
      const text = JSON.stringify({ rows }, null, 2)
      try {
        const blob = new Blob([text], { type: 'application/json;charset=utf-8' })
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${this.tableForm.key || 'test-data'}-rows.json`
        document.body.appendChild(link)
        link.click()
        link.remove()
        URL.revokeObjectURL(url)
      } catch (error) {
        this.$message.error(`导出失败：${error.message || error}`)
      }
    },
    async copyInlineExample() {
      if (!this.canCopyExample) {
        this.$message.warning('请先填写表 Key 并添加至少一列')
        return
      }
      const text = this.inlineExample
      try {
        await navigator.clipboard.writeText(text)
        this.$message.success(`已复制 ${text}`)
      } catch {
        this.$message.info(text)
      }
    },
    openGenerateDialog() {
      if (!this.hasColumns) {
        this.$message.warning('请先添加列后再批量生成')
        return
      }
      if (this.aiNeeded && !this.aiProviders.length) {
        this.$message.warning('当前项目尚未配置 AI 提供商，无法生成 AI 列')
        return
      }
      this.generateForm = { rowCount: 5, preserveManual: true }
      this.generateDialogVisible = true
    },
    async submitGenerateRows() {
      if (!this.selectedTableId) return
      const body = {
        rowCount: Number(this.generateForm.rowCount) || 1,
        preserveManual: !!this.generateForm.preserveManual
      }
      this.generatingRows = true
      try {
        const result = await generateTestDataRows(this.projectId, this.selectedTableId, body)
        const rows = Array.isArray(result?.rows) ? result.rows : []
        this.workingRows = rows.map((r) => normalizeRow(r, this.tableForm.columns))
        this.rowsBaseline = JSON.stringify(this.workingRows.map((r) => r.values))
        const warnings = Array.isArray(result?.warnings) ? result.warnings : []
        if (warnings.length) {
          this.$message.warning(`生成完成（${rows.length} 行），存在告警：${warnings.join('；')}`)
        } else {
          this.$message.success(`已生成 ${rows.length} 行（已写入服务端）`)
        }
        this.generateDialogVisible = false
      } finally {
        this.generatingRows = false
      }
    },
    openRegenerateDialog(row, rowIndex) {
      if (!row.id) {
        this.$message.warning('该行尚未保存，请先点击「保存当前行」后再重新生成')
        return
      }
      const cols = this.regenerableColumns
      if (!cols.length) {
        this.$message.warning('当前表没有可自动生成的列（仅 mock / ai 列支持重新生成）')
        return
      }
      this.regenerateRow = row
      this.regenerateRowIndex = rowIndex
      this.regenerateForm = { columnKeys: cols.map((c) => c.key) }
      this.regenerateDialogVisible = true
    },
    async submitRegenerate() {
      if (!this.regenerateRow || !this.regenerateRow.id) return
      const body = { columnKeys: this.regenerateForm.columnKeys.slice() }
      this.regenerateLoading = true
      try {
        const updated = await regenerateTestDataCells(
          this.projectId,
          this.selectedTableId,
          this.regenerateRow.id,
          body
        )
        const normalized = normalizeRow(updated, this.tableForm.columns)
        if (this.regenerateRowIndex >= 0 && this.regenerateRowIndex < this.workingRows.length) {
          this.workingRows.splice(this.regenerateRowIndex, 1, normalized)
        }
        this.rowsBaseline = JSON.stringify(this.workingRows.map((r) => r.values))
        this.$message.success('该行指定列已重新生成')
        this.regenerateDialogVisible = false
      } finally {
        this.regenerateLoading = false
      }
    }
  }
}
</script>

<style scoped>
.test-data-header {
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
}

.page-subtitle,
.panel-subtitle {
  margin: 6px 0 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.project-hint {
  margin-bottom: 16px;
}

.test-data-layout {
  display: grid;
  grid-template-columns: minmax(320px, 0.65fr) minmax(0, 1.35fr);
  gap: 16px;
  align-items: start;
}

.table-list-panel,
.table-detail-panel {
  min-width: 0;
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  padding: 14px;
  background: var(--app-card-bg);
}

.panel-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.panel-title {
  margin: 0;
  font-size: var(--app-font-size-title);
  line-height: var(--app-line-height-base);
  font-weight: 600;
}

.table-search {
  margin-bottom: 12px;
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.detail-header-form {
  flex: 1 1 auto;
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
}

.header-form-item {
  margin-bottom: 8px;
  flex: 1 1 220px;
  min-width: 200px;
}

.header-form-item--wide {
  flex: 2 1 320px;
}

/*
  inline 模式下 el-form-item 默认 inline-block，label 通过 line-height 与 input 居中；
  项目支持自定义字号会让 input 高度变化，导致 label 文字不再垂直居中。
  改为 flex + align-items: center 让 label 与 input 始终垂直居中对齐。
*/
.detail-header-form :deep(.el-form-item) {
  display: flex;
  align-items: center;
  margin: 0 0 8px 0;
}
.detail-header-form :deep(.el-form-item__label) {
  display: inline-flex;
  align-items: center;
  height: auto;
  line-height: 1.5;
  padding: 0 8px 0 0;
}
.detail-header-form :deep(.el-form-item__content) {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  line-height: normal;
  margin-left: 0 !important;
}

.detail-header-actions {
  display: inline-flex;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-key-tip {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin: 0 0 16px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--app-code-bg);
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.section-title-row {
  display: flex;
  align-items: baseline;
  justify-content: flex-start;
  gap: 12px;
  margin: 8px 0 10px;
  flex-wrap: wrap;
}

.section-title {
  margin: 0;
  font-size: calc(var(--app-font-size-base) + 1px);
  line-height: var(--app-line-height-base);
  font-weight: 600;
}

.section-subtitle {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.section-subtitle--dirty {
  color: var(--app-accent-color);
}

.row-section-title {
  margin-top: 18px;
}

.column-table {
  margin-bottom: 8px;
}

.add-column-row {
  margin: 0 0 16px;
}

.row-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 8px 0 12px;
}

.row-alert {
  margin-bottom: 12px;
}

.row-table {
  margin-bottom: 8px;
}

.generator-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.generator-input {
  width: 100%;
}

.generator-tip,
.generator-label {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.4;
}

.generator-link {
  margin-left: 6px;
  color: var(--app-primary-color);
  text-decoration: none;
}

.generator-link:hover {
  text-decoration: underline;
}

.dialog-tip {
  margin-top: 8px;
}

.generator-required {
  color: var(--el-color-danger, #f56c6c);
}

.ds-helper-name {
  font-weight: 600;
  margin-right: 8px;
}

.ds-helper-desc {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.cell-display {
  display: inline-block;
  width: 100%;
  min-height: 24px;
  padding: 2px 4px;
  border-radius: 4px;
  cursor: text;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--app-text-color);
}

.cell-display:hover {
  background: var(--app-code-bg);
}

.field-tip {
  margin-top: 6px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.5;
}

.full-width {
  width: 100%;
}

.inline-code {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--app-code-bg);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.92em;
}

.regenerate-tip {
  margin: 0 0 10px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.regenerate-columns {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.regenerate-kind {
  margin-left: 4px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.regenerate-form {
  margin-top: 8px;
}

.detail-empty {
  padding: 36px 0;
}

:deep(.is-selected-table > td) {
  background: color-mix(in srgb, var(--app-primary-color) 8%, transparent);
}

@media (max-width: 1180px) {
  .test-data-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .detail-header {
    flex-direction: column;
    align-items: stretch;
  }

  .detail-header-actions {
    justify-content: flex-end;
  }
}
</style>
