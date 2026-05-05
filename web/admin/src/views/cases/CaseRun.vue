<template>
  <div class="debug-page" :class="{ 'is-embedded': embedded }">
    <div class="debug-header">
      <div>
        <el-button v-if="!embedded" link type="primary" @click="$router.push('/cases')">返回API管理</el-button>
        <div class="debug-title-wrap">
          <h2 class="debug-title">{{ displayTitle }}</h2>
        </div>
        <div v-if="savedRequests.length" class="saved-requests-tabs">
          <div v-for="(saved, index) in savedRequests" :key="saved.id"
            class="saved-request-tab"
            :class="{ 'is-active': activeSavedIndex === index }"
            @click.stop="applySavedRequest(saved, index)"
            @dblclick.stop="startSavedRename(index)">
            <input
              v-if="renamingIndex === index"
              :ref="(el) => setSavedRenameRef(index, el)"
              v-model="renamingName"
              class="saved-request-tab-rename"
              @blur="finishSavedRename"
              @keydown.enter.prevent="finishSavedRename"
              @keydown.esc.prevent="cancelSavedRename"
              @click.stop
              @dblclick.stop
            />
            <span v-else class="saved-request-tab-label" :title="saved.label">{{ saved.label }}</span>
            <button class="saved-request-tab-close" title="删除" @click.stop="deleteSavedRequest(index)">×</button>
          </div>
        </div>
      </div>
      <div v-if="!embedded" class="debug-actions">
        <div class="environment-picker">
          <div class="environment-select-control">
            <el-select v-model="environmentId" class="environment-select" popper-class="environment-select-dropdown"
              placeholder="选择运行环境" filterable @change="rememberCurrentEnvironment">
              <el-option v-for="env in environments" :key="env.id" :label="env.name" :value="env.id">
                <div class="env-option-row">
                  <span class="env-option-label">{{ env.name }}</span>
                  <el-tooltip content="编辑环境" placement="right">
                    <el-button class="env-option-edit-btn" type="primary" link @mousedown.stop
                      @click.stop="editEnvironmentFromList(env)">
                      <el-icon>
                        <EditPen />
                      </el-icon>
                    </el-button>
                  </el-tooltip>
                </div>
              </el-option>
            </el-select>
          </div>
        </div>
      </div>
    </div>

    <Teleport v-if="embedded && active" to="#run-console-environment-control">
      <div class="environment-picker">
        <div class="environment-select-control">
          <el-select v-model="environmentId" class="environment-select" popper-class="environment-select-dropdown"
            placeholder="选择运行环境" filterable @change="rememberCurrentEnvironment">
            <el-option v-for="env in environments" :key="env.id" :label="env.name" :value="env.id">
              <div class="env-option-row">
                <span class="env-option-label">{{ env.name }}</span>
                <el-tooltip content="编辑环境" placement="right">
                  <el-button class="env-option-edit-btn" type="primary" link @mousedown.stop
                    @click.stop="editEnvironmentFromList(env)">
                    <el-icon>
                      <EditPen />
                    </el-icon>
                  </el-button>
                </el-tooltip>
              </div>
            </el-option>
          </el-select>
        </div>
      </div>
    </Teleport>

    <el-card v-loading="loading" class="request-card">
      <div class="request-line">
        <el-select v-model="request.method" class="method-select">
          <el-option v-for="method in methods" :key="method" :label="method" :value="method" />
        </el-select>
        <el-input v-model="request.path" class="path-input" placeholder="/api/users/{id}" />
        <div class="send-actions">
          <el-button :loading="generating" @click="generateParams">生成参数</el-button>
          <el-tooltip content="将当前请求参数保存为一条测试用例（数据库持久化，可在路径右侧 Tab 切换并被场景/测试集引用）" placement="top">
            <el-button :loading="savingCase" @click="saveCurrentRequest">保存用例</el-button>
          </el-tooltip>
          <el-button class="send-button" type="primary" :loading="running" :disabled="!environmentId"
            @click="executeRun">
            发送
          </el-button>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="request-tabs">
        <el-tab-pane label="Params" name="params">
          <div class="params-section">
            <div class="params-section-title">Path Vars</div>
            <el-table :data="pathRows" border class="kv-table">
              <el-table-column width="70" label="启用"><template #default="{ row }"><el-checkbox
                    v-model="row.enabled" /></template></el-table-column>
              <el-table-column label="参数名" min-width="180"><template #default="{ row }"><el-input v-model="row.key"
                    placeholder="路径变量名" /></template></el-table-column>
              <el-table-column label="参数值" min-width="260"><template #default="{ row }"><el-input v-model="row.value"
                    placeholder="参数值" /></template></el-table-column>
              <el-table-column width="90" label="操作"><template #default="{ $index }"><el-button link type="danger"
                    @click="removeRow(pathRows, $index)">删除</el-button></template></el-table-column>
            </el-table>
          </div>
          <div class="params-section">
            <div class="params-section-title">Query</div>
            <el-table :data="queryRows" border class="kv-table">
              <el-table-column width="70" label="启用"><template #default="{ row }"><el-checkbox
                    v-model="row.enabled" /></template></el-table-column>
              <el-table-column label="参数名" min-width="180"><template #default="{ row }"><el-input v-model="row.key"
                    placeholder="参数名" /></template></el-table-column>
              <el-table-column label="参数值" min-width="260"><template #default="{ row }"><el-input v-model="row.value"
                    placeholder="参数值" /></template></el-table-column>
              <el-table-column width="90" label="操作"><template #default="{ $index }"><el-button link type="danger"
                    @click="removeRow(queryRows, $index)">删除</el-button></template></el-table-column>
            </el-table>
            <el-button class="add-row" @click="addRow(queryRows)">新增一行</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Headers" name="headers">
          <el-table :data="headerRows" border class="kv-table">
            <el-table-column width="70" label="启用"><template #default="{ row }"><el-checkbox
                  v-model="row.enabled" /></template></el-table-column>
            <el-table-column label="Header" min-width="180"><template #default="{ row }"><el-input v-model="row.key"
                  placeholder="Header" /></template></el-table-column>
            <el-table-column label="Value" min-width="260"><template #default="{ row }"><el-input v-model="row.value"
                  placeholder="Value" /></template></el-table-column>
            <el-table-column width="90" label="操作"><template #default="{ $index }"><el-button link type="danger"
                  @click="removeRow(headerRows, $index)">删除</el-button></template></el-table-column>
          </el-table>
          <el-button class="add-row" @click="addRow(headerRows)">新增一行</el-button>
        </el-tab-pane>
        <el-tab-pane label="Body" name="body">
          <div class="body-toolbar">
            <span>JSON Body</span>
            <el-button size="small" @click="formatBody">格式化</el-button>
          </div>
          <div ref="bodyEditor" class="code-editor" contenteditable spellcheck="false"
            @input="bodyText = $event.target.innerText"></div>
        </el-tab-pane>
        <el-tab-pane label="断言" name="assertions">
          <AssertionEditor v-model="editableAssertions" />
          <div class="assertion-actions">
            <el-button size="small" :loading="savingAssertions" @click="saveAssertions">保存断言</el-button>
            <span v-if="assertionsSaved" class="save-ok-hint">已保存</span>
          </div>
        </el-tab-pane>
        <el-tab-pane label="上次响应" name="lastResponse">
          <div v-if="hasLastResponse" class="response-preview">
            <pre>{{ formatJSON(testCase.lastResponseSnapshot) }}</pre>
          </div>
          <el-empty v-else description="暂无保存的响应" />
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-card v-if="payload" class="result-card">
      <div class="result-title">
        <div>
          <h3>运行结果</h3>
          <p>Run ID: {{ runRecord?.id || '-' }}</p>
        </div>
        <div class="result-metrics">
          <span class="metric-chip" :class="metricClass(runRecord?.status)">
            <el-icon>
              <component :is="statusIcon(runRecord?.status)" />
            </el-icon>
            <span>运行 {{ statusLabel(runRecord?.status) }}</span>
          </span>
          <span class="metric-chip" :class="metricClass(result?.status)">
            <el-icon>
              <component :is="statusIcon(result?.status)" />
            </el-icon>
            <span>结果 {{ statusLabel(result?.status) }}</span>
          </span>
          <span class="metric-chip" :class="durationChipClass(result?.durationMillis)">
            <el-icon>
              <Timer />
            </el-icon>
            <span>{{ formatDuration(result?.durationMillis) }}</span>
          </span>
          <div class="response-overview-tags" aria-label="响应概览">
            <el-tag :type="responseStatusType" effect="plain" size="small">{{ responseSnapshot.statusCode || '-'
            }}</el-tag>
            <el-tag v-if="result?.error" effect="plain" type="danger" size="small" class="response-msg-tag">
              <span class="response-msg-tag-text">{{ result.error }}</span>
            </el-tag>
          </div>
        </div>
      </div>

      <el-tabs v-model="resultTab">
        <el-tab-pane label="请求" name="request">
          <div class="section-grid">
            <div class="section-block full">
              <h3>URL</h3>
              <div class="url-line">
                <el-tag>{{ requestSnapshot.method || '-' }}</el-tag>
                <span>{{ requestSnapshot.url || '-' }}</span>
              </div>
            </div>
            <div class="section-block">
              <h3>Params</h3>
              <el-table :data="requestQueryRows" border>
                <el-table-column prop="key" label="名称" />
                <el-table-column prop="value" label="值" />
              </el-table>
            </div>
            <div class="section-block">
              <h3>Headers</h3>
              <el-table :data="headersToRows(requestSnapshot.headers)" border>
                <el-table-column prop="key" label="名称" />
                <el-table-column prop="value" label="值" />
              </el-table>
            </div>
            <div class="section-block full">
              <h3>Body</h3>
              <pre class="code-view">{{ formatSnapshotBody(requestSnapshot.body) }}</pre>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="响应" name="response">
          <div class="section-grid">
            <div class="section-block full">
              <h3>Body</h3>
              <pre class="code-view">{{ formatSnapshotBody(responseSnapshot.body) }}</pre>
            </div>
            <div class="section-block full">
              <el-tabs v-model="responseDetailTab" class="response-detail-tabs" type="card">
                <el-tab-pane label="响应头" name="headers">
                  <el-table :data="headersToRows(responseSnapshot.headers)" border>
                    <el-table-column prop="key" label="名称" />
                    <el-table-column prop="value" label="值" />
                  </el-table>
                </el-tab-pane>
                <el-tab-pane label="实际请求" name="curl">
                  <pre class="code-view code-view--curl">{{ requestCurlCommand }}</pre>
                </el-tab-pane>
              </el-tabs>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="断言" name="assertions">
          <el-table :data="assertions" border>
            <el-table-column label="类型" width="120">
              <template #default="{ row }">
                <el-tag size="small" :type="assertionTypeColor(row.type)" effect="plain">
                  {{ assertionTypeLabel(row.type) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="名称 / 路径" min-width="180">
              <template #default="{ row }">
                <span v-if="row.name" class="assertion-name">{{ row.name }}</span>
                <code v-else-if="row.type === 'jsonpath' || row.type === 'header'" class="assertion-path">
                  {{ assertionSummary(row) }}
                </code>
                <span v-else class="assertion-path">{{ assertionSummary(row) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.passed ? 'success' : 'danger'" size="small">
                  {{ row.passed ? '通过' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="详情" min-width="220" />
          </el-table>
          <el-empty v-if="!assertions.length" description="暂无断言结果" :image-size="48" />
        </el-tab-pane>
      </el-tabs>
    </el-card>
    <el-card v-else class="result-card response-empty-card">
      <div class="result-title">
        <div>
          <h3>响应结果</h3>
          <p>点击发送后展示本次请求、响应和断言结果。</p>
        </div>
        <div class="result-metrics">
          <span class="metric-chip is-idle">
            <el-icon>
              <Clock />
            </el-icon>
            <span>未运行</span>
          </span>
        </div>
      </div>
      <el-empty description="暂无响应结果" />
    </el-card>

    <el-dialog v-model="envDialog" class="env-config-dialog" title="编辑环境" width="860px" align-center>
      <el-form :model="envForm" label-width="90px">
        <el-form-item label="名称"><el-input v-model="envForm.name" /></el-form-item>
        <el-form-item label="Base URL"><el-input v-model="envForm.baseUrl" /></el-form-item>
        <el-form-item class="json-form-item" label-width="0">
          <div class="json-field">
            <div class="json-field-label-row">
              <div class="json-field-label">
                <span>变量 JSON</span>
                <el-tooltip placement="top-start" effect="dark" popper-class="json-help-tooltip">
                  <template #content>
                    <div v-pre class="json-help-content">
                      <p class="json-help-lead">填写环境级键值变量。</p>
                      <p class="json-help-caption">示例</p>
                      <pre class="json-help-code">{
  "token": "env-token",
  "page": 1
}</pre>
                      <p class="json-help-hint">可在请求 URL、路径、请求头和查询参数中用 {{变量名}} 引用。</p>
                      <p class="json-help-hint">无需变量时填 <code>{}</code>。</p>
                    </div>
                  </template>
                  <span class="field-help-icon" aria-label="变量 JSON 填写说明">?</span>
                </el-tooltip>
              </div>
              <el-button size="small" @click="formatEnvironmentVariablesJson">格式化</el-button>
            </div>
            <el-input v-model="envForm.variables" class="json-editor" type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }" placeholder='{"token":"env-token","page":1}' />
          </div>
        </el-form-item>
        <el-form-item class="json-form-item" label-width="0">
          <div class="json-field">
            <div class="json-field-label-row">
              <div class="json-field-label">
                <span>认证 JSON</span>
                <el-tooltip placement="top-start" effect="dark" popper-class="json-help-tooltip">
                  <template #content>
                    <div v-pre class="json-help-content">
                      <p class="json-help-lead">填写环境认证配置，可按 OpenAPI/Swagger security 选择不同参数或 token。</p>
                      <p class="json-help-hint">无需认证时填 <code>{}</code>。单一认证示例：</p>
                      <pre class="json-help-code">{
  "type": "bearer",
  "token": "{{token}}"
}</pre>
                      <p class="json-help-caption">多 security 示例</p>
                      <pre class="json-help-code json-help-code--scroll">{
  "defaultProfile": "user",
  "profiles": {
    "user": {
      "type": "bearer",
      "token": "{{userToken}}"
    },
    "admin": {
      "type": "api_key",
      "in": "query",
      "name": "admin_token",
      "value": "{{adminToken}}"
    }
  },
  "securitySchemes": {
    "UserAuth": "user",
    "AdminAuth": "admin"
  }
}</pre>
                    </div>
                  </template>
                  <span class="field-help-icon" aria-label="认证 JSON 填写说明">?</span>
                </el-tooltip>
              </div>
              <el-button size="small" @click="formatEnvironmentAuthJson">格式化</el-button>
            </div>
            <el-input v-model="envForm.auth" class="json-editor" type="textarea" :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"defaultProfile":"user","profiles":{"user":{"type":"bearer","token":"{{userToken}}"},"admin":{"type":"api_key","in":"query","name":"admin_token","value":"{{adminToken}}"}},"securitySchemes":{"UserAuth":"user","AdminAuth":"admin"}}' />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="envDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingEnvironment" @click="submitEnvironment">保存</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script>
import { ElMessageBox } from 'element-plus'
import {
  createSavedCase,
  deleteSavedCase,
  generateCaseParams,
  getCase,
  listEndpoints,
  listSavedCases,
  listServiceEnvironments,
  patchCase,
  runCase,
  updateServiceEnvironment
} from '../../api'
import { buildCurlFromRequestSnapshot } from '../../utils/curl'
import AssertionEditor from '../../components/AssertionEditor.vue'

const ASSERTION_TYPE_LABELS = {
  status_code: '状态码',
  jsonpath: 'JSONPath',
  header: '响应头',
  body_contains: 'Body',
  response_time: '响应时间',
  script: 'JS 脚本'
}

const ASSERTION_TYPE_COLORS = {
  status_code: 'primary',
  jsonpath: 'success',
  header: 'warning',
  body_contains: '',
  response_time: 'info',
  script: 'danger'
}

const ENVIRONMENT_STORAGE_KEY = 'autotest.currentEnvironmentIdByProject'

function readStoredEnvironmentIds() {
  try {
    const raw = localStorage.getItem(ENVIRONMENT_STORAGE_KEY)
    const stored = raw ? JSON.parse(raw) : {}
    return stored && typeof stored === 'object' && !Array.isArray(stored) ? stored : {}
  } catch {
    return {}
  }
}

function environmentStorageKey(projectId, serviceId) {
  return serviceId ? `${projectId}:${serviceId}` : projectId
}

function getStoredEnvironmentId(projectId, serviceId) {
  return readStoredEnvironmentIds()[environmentStorageKey(projectId, serviceId)] || ''
}

function storeEnvironmentId(projectId, serviceId, environmentId) {
  if (!projectId) return
  try {
    const stored = readStoredEnvironmentIds()
    const key = environmentStorageKey(projectId, serviceId)
    if (environmentId) {
      stored[key] = environmentId
    } else {
      delete stored[key]
    }
    if (Object.keys(stored).length) {
      localStorage.setItem(ENVIRONMENT_STORAGE_KEY, JSON.stringify(stored))
    } else {
      localStorage.removeItem(ENVIRONMENT_STORAGE_KEY)
    }
  } catch {
    // Ignore storage failures so environment switching still works in memory.
  }
}

export default {
  name: 'CaseRun',
  components: { AssertionEditor },
  props: {
    caseId: {
      type: String,
      default: ''
    },
    embedded: {
      type: Boolean,
      default: false
    },
    active: {
      type: Boolean,
      default: true
    }
  },
  emits: ['status-change', 'tab-title-change'],
  data() {
    return {
      loading: false,
      running: false,
      generating: false,
      savingCase: false,
      savingEnvironment: false,
      payload: null,
      testCase: null,
      endpoint: null,
      environments: [],
      environmentId: '',
      envDialog: false,
      envForm: this.emptyEnvironmentForm(),
      runName: '',
      activeTab: 'params',
      resultTab: 'response',
      responseDetailTab: 'headers',
      methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
      request: { method: 'GET', path: '', headers: {}, query: {}, body: null },
      pathRows: [],
      queryRows: [],
      headerRows: [],
      variableRows: [],
      bodyText: '{}',
      savedRequests: [],
      activeSavedIndex: -1,
      renamingIndex: -1,
      renamingName: '',
      editableAssertions: [],
      savingAssertions: false,
      assertionsSaved: false
    }
  },
  computed: {
    displayTitle() {
      const trimmed = (this.runName || '').trim()
      if (trimmed) return trimmed
      return this.testCase?.name || '接口调试'
    },
    hasLastResponse() {
      const value = this.testCase?.lastResponseSnapshot
      return value && typeof value === 'object' && Object.keys(value).length > 0
    },
    currentEnvironment() {
      return this.environments.find((env) => env.id === this.environmentId) || null
    },
    runRecord() {
      return this.payload?.run
    },
    result() {
      return this.payload?.result || this.payload?.results?.[0]
    },
    requestSnapshot() {
      return this.result?.requestSnapshot || {}
    },
    responseSnapshot() {
      return this.result?.responseSnapshot || {}
    },
    assertions() {
      return Array.isArray(this.result?.assertions) ? this.result.assertions : []
    },
    requestQueryRows() {
      const url = this.requestSnapshot.url
      if (!url) return []
      try {
        const parsed = new URL(url, window.location.origin)
        return Array.from(parsed.searchParams.entries()).map(([key, value]) => ({ key, value }))
      } catch (error) {
        return []
      }
    },
    responseStatusType() {
      const code = Number(this.responseSnapshot.statusCode)
      if (code >= 200 && code < 300) return 'success'
      if (code >= 400) return 'danger'
      return 'warning'
    },
    requestCurlCommand() {
      return buildCurlFromRequestSnapshot(this.requestSnapshot)
    }
  },
  async created() {
    await this.loadData()
  },
  beforeUnmount() {},
  watch: {
    caseId() {
      this.loadData()
    },
    'request.method'() {
      this.activeTab = this.defaultTab()
    },
    'request.path'() {
      this.activeTab = this.defaultTab()
      const merged = {
        ...this.rowsToObject(this.pathRows),
        ...this.rowsToObject(this.variableRows)
      }
      const { forPath, forRest } = this.splitVariablesByPathTemplate(this.request.path, merged)
      this.pathRows = this.objectToRows(forPath)
      this.variableRows = this.objectToRows(forRest)
      this.request.variables = { ...forPath, ...forRest }
    }
  },
  methods: {
    updateRunName(name) {
      if (name && name.trim()) this.runName = name.trim()
    },
    emptyEnvironmentForm() {
      return { name: '', baseUrl: '', variables: '{}', auth: '{}' }
    },
    resolveEnvironmentId(projectId, serviceId) {
      const storedEnvironmentId = getStoredEnvironmentId(projectId, serviceId)
      return this.environments.some((env) => env.id === storedEnvironmentId)
        ? storedEnvironmentId
        : this.environments[0]?.id || ''
    },
    rememberCurrentEnvironment() {
      storeEnvironmentId(this.testCase?.projectId, this.testCase?.serviceId, this.environmentId)
    },
    async loadData() {
      this.loading = true
      this.activeSavedIndex = -1
      this.savedRequests = []
      try {
        this.payload = null
        this.emitRunSummary({ running: false, runStatus: '', resultStatus: '', durationMillis: null })
        this.testCase = await getCase(this.caseId || this.$route.params.caseID)
        const [environments, endpoints] = await Promise.all([
          listServiceEnvironments(this.testCase.projectId, this.testCase.serviceId),
          listEndpoints(this.testCase.projectId, this.testCase.serviceId)
        ])
        this.environments = environments
        this.endpoint = this.findEndpoint(endpoints, this.testCase)
        this.environmentId = this.resolveEnvironmentId(this.testCase.projectId, this.testCase.serviceId)
        this.runName = this.testCase.name
        this.editableAssertions = this.parseAssertions(this.testCase.assertions)
        this.applyRequest(this.buildGeneratedRequest(this.testCase))
        await this.loadSavedRequests()
      } catch {
        // 错误已在全局请求拦截器中提示
        this.testCase = null
        this.environments = []
        this.savedRequests = []
      } finally {
        this.loading = false
      }
    },
    buildGeneratedRequest(row) {
      const saved = this.normalizeRequest(row.request)
      const generated = this.requestFromSchema(this.endpoint?.requestSchema)
      const mergedVariables = {
        ...this.normalizeObject(generated.variables),
        ...this.normalizeObject(saved.variables),
        ...this.normalizeObject(saved.pathVars)
      }
      const savedQueryKeys = new Set(Object.keys(this.normalizeObject(saved.query)))
      const mergedQuery = { ...generated.query, ...this.normalizeObject(saved.query) }
      const queryEnabled = {}
      for (const key of Object.keys(mergedQuery)) {
        queryEnabled[key] = generated.queryRequired.has(key) || savedQueryKeys.has(key)
      }
      const request = {
        method: saved.method || row.method,
        path: this.normalizePathVariables(saved.path || row.path),
        headers: { ...generated.headers, ...this.normalizeObject(saved.headers) },
        query: mergedQuery,
        queryEnabled,
        variables: mergedVariables,
        body: generated.hasBody ? generated.body : saved.body
      }
      const security = Object.prototype.hasOwnProperty.call(saved, 'security') ? saved.security : generated.security
      if (security !== undefined) {
        request.security = this.cloneSample(security)
      }
      if (request.body == null && ['POST', 'PUT', 'PATCH'].includes(String(request.method).toUpperCase())) {
        request.body = {}
      }
      if (request.body && !request.headers['Content-Type']) {
        request.headers['Content-Type'] = 'application/json'
      }
      return request
    },
    applyRequest(request) {
      const raw = this.normalizeRequest(request)
      const mergedVars = {
        ...this.normalizeObject(raw.pathVars),
        ...this.normalizeObject(raw.variables)
      }
      const { forPath, forRest } = this.splitVariablesByPathTemplate(raw.path, mergedVars)
      this.request = {
        ...raw,
        variables: { ...forPath, ...forRest }
      }
      this.queryRows = this.objectToRows(raw.query, raw.queryEnabled)
      this.headerRows = this.objectToRows(raw.headers)
      this.pathRows = this.objectToRows(forPath)
      this.variableRows = this.objectToRows(forRest)
      this.setBodyText(raw.body == null ? '' : this.formatJSON(raw.body))
      this.activeTab = this.defaultTab()
    },
    async generateParams() {
      if (!this.testCase) return
      this.generating = true
      try {
        const generated = await generateCaseParams(this.testCase.id)
        const current = this.normalizeRequest(this.request)
        const schemaGenerated = this.requestFromSchema(this.endpoint?.requestSchema)
        const generatedQuery = this.normalizeObject(generated.query)
        const queryEnabled = {}
        for (const key of Object.keys(generatedQuery)) {
          queryEnabled[key] = schemaGenerated.queryRequired.has(key)
        }
        const request = {
          method: current.method || this.testCase.method,
          path: this.normalizePathVariables(current.path || this.testCase.path),
          headers: this.normalizeObject(current.headers),
          query: generatedQuery,
          queryEnabled,
          variables: {
            ...this.normalizeObject(current.variables),
            ...this.normalizeObject(generated.path)
          },
          body: generated.body != null ? generated.body : current.body
        }
        if (current.security !== undefined) {
          request.security = this.cloneSample(current.security)
        }
        if (request.body == null && ['POST', 'PUT', 'PATCH'].includes(String(request.method).toUpperCase())) {
          request.body = {}
        }
        if (request.body != null && !request.headers['Content-Type']) {
          request.headers['Content-Type'] = 'application/json'
        }
        this.applyRequest(request)
        this.$message.success('已重新生成参数')
      } catch (_) {
        // 错误消息已由请求拦截器统一展示
      } finally {
        this.generating = false
      }
    },
    openEditEnvironment() {
      if (!this.currentEnvironment) return
      this.envForm = {
        name: this.currentEnvironment.name,
        baseUrl: this.currentEnvironment.baseUrl,
        variables: this.formatEditableJSON(this.currentEnvironment.variables),
        auth: this.formatEditableJSON(this.currentEnvironment.auth)
      }
      this.envDialog = true
    },
    editEnvironmentFromList(env) {
      if (!env) return
      this.environmentId = env.id
      this.rememberCurrentEnvironment()
      this.openEditEnvironment()
    },
    formatEnvironmentVariablesJson() {
      try {
        const parsed = JSON.parse(this.envForm.variables || '{}')
        this.envForm.variables = JSON.stringify(parsed, null, 2)
      } catch (error) {
        this.$message.error(`变量 JSON 不是合法 JSON：${error.message}`)
      }
    },
    formatEnvironmentAuthJson() {
      try {
        const parsed = JSON.parse(this.envForm.auth || '{}')
        this.envForm.auth = JSON.stringify(parsed, null, 2)
      } catch (error) {
        this.$message.error(`认证 JSON 不是合法 JSON：${error.message}`)
      }
    },
    async submitEnvironment() {
      if (!this.currentEnvironment || !this.testCase) return
      let variables
      let auth
      try {
        variables = JSON.parse(this.envForm.variables || '{}')
        auth = JSON.parse(this.envForm.auth || '{}')
      } catch (error) {
        this.$message.error(`环境 JSON 不合法：${error.message}`)
        return
      }
      this.savingEnvironment = true
      try {
        await updateServiceEnvironment(this.testCase.projectId, this.testCase.serviceId, this.currentEnvironment.id, {
          name: this.envForm.name,
          baseUrl: this.envForm.baseUrl,
          variables,
          auth
        })
        const selectedId = this.environmentId
        this.environments = await listServiceEnvironments(this.testCase.projectId, this.testCase.serviceId)
        this.environmentId = this.environments.some((env) => env.id === selectedId)
          ? selectedId
          : this.environments[0]?.id || ''
        this.envDialog = false
        this.envForm = this.emptyEnvironmentForm()
        this.$message.success('环境已保存')
      } finally {
        this.savingEnvironment = false
      }
    },
    async executeRun() {
      let body = null
      if (this.bodyText.trim()) {
        try {
          body = JSON.parse(this.bodyText)
        } catch (error) {
          this.$message.error(`Body 不是合法 JSON：${error.message}`)
          return
        }
      }
      this.running = true
      this.emitRunSummary({ running: true, runStatus: 'running', resultStatus: '', durationMillis: null })
      try {
        const activeSaved = this.activeSavedIndex >= 0 ? this.savedRequests[this.activeSavedIndex] : null
        const output = await runCase(activeSaved?.id || this.testCase.id, {
          name: this.runName,
          environmentId: this.environmentId,
          request: {
            method: this.request.method,
            path: this.pathForRun(this.request.path),
            headers: this.rowsToObject(this.headerRows),
            query: this.rowsToObject(this.queryRows),
            body,
            security: this.request.security
          },
          variables: {
            ...this.rowsToObject(this.pathRows),
            ...this.rowsToObject(this.variableRows)
          }
        })
        this.payload = output
        this.resultTab = 'response'
        this.emitRunSummary({
          running: false,
          runStatus: output.run?.status || '',
          resultStatus: output.result?.status || '',
          durationMillis: output.result?.durationMillis ?? null
        })
        if (this.activeSavedIndex >= 0 && this.savedRequests[this.activeSavedIndex]) {
          this.savedRequests[this.activeSavedIndex].payload = JSON.parse(JSON.stringify(output))
        }
        this.$message.success('运行完成')
      } catch {
        this.emitRunSummary({ running: false, runStatus: 'error', resultStatus: 'error', durationMillis: null })
        // 不再向上抛出，避免未处理的 Promise 拒绝（错误已在请求拦截器中提示）
      } finally {
        this.running = false
      }
    },
    formatBody() {
      if (!this.bodyText.trim()) return
      try {
        this.setBodyText(this.formatJSON(JSON.parse(this.bodyText)))
      } catch (error) {
        this.$message.error(`Body 不是合法 JSON：${error.message}`)
      }
    },
    setBodyText(text) {
      this.bodyText = text
      this.$nextTick(() => {
        if (this.$refs.bodyEditor && this.$refs.bodyEditor.innerText !== text) {
          this.$refs.bodyEditor.innerText = text
        }
      })
    },
    addRow(rows) {
      rows.push({ enabled: true, key: '', value: '' })
    },
    removeRow(rows, index) {
      rows.splice(index, 1)
    },
    objectToRows(value, enabledMap) {
      return Object.entries(value || {}).map(([key, item]) => ({
        enabled: enabledMap ? (enabledMap[key] !== false) : true,
        key,
        value: String(item)
      }))
    },
    rowsToObject(rows) {
      return rows.reduce((out, row) => {
        if (row.enabled && row.key) out[row.key] = row.value
        return out
      }, {})
    },
    splitVariablesByPathTemplate(path, variables) {
      const defaults = this.pathVariables(path)
      const vars = this.normalizeObject(variables)
      const forPath = {}
      const forRest = { ...vars }
      for (const key of Object.keys(defaults)) {
        forPath[key] = Object.prototype.hasOwnProperty.call(vars, key) ? String(vars[key]) : defaults[key]
        delete forRest[key]
      }
      return { forPath, forRest }
    },
    findEndpoint(endpoints, row) {
      return (endpoints || []).find((endpoint) => endpoint.id === row.endpointId) ||
        (endpoints || []).find((endpoint) => endpoint.method === row.method && endpoint.path === row.path) ||
        null
    },
    requestFromSchema(rawSchema) {
      const schema = this.normalizeRequest(rawSchema)
      const request = {
        headers: {},
        query: {},
        queryRequired: new Set(),
        variables: {},
        body: undefined,
        hasBody: false,
        security: undefined
      }
      const parameters = Array.isArray(schema.parameters) ? schema.parameters : []
      for (const param of parameters) {
        if (!param || !param.name) continue
        const value = this.stringifySample(this.sampleFromSchema(param.schema))
        if (param.in === 'query') {
          request.query[param.name] = value
          if (param.required) request.queryRequired.add(param.name)
        } else if (param.in === 'header') {
          request.headers[param.name] = value
        } else if (param.in === 'path') {
          request.variables[param.name] = value || '1'
        }
      }
      if (schema.body && typeof schema.body === 'object') {
        request.body = this.sampleFromSchema(schema.body)
        request.hasBody = true
        request.headers['Content-Type'] = 'application/json'
      }
      if (Object.prototype.hasOwnProperty.call(schema, 'security')) {
        request.security = this.cloneSample(schema.security)
      }
      return request
    },
    sampleFromSchema(schema) {
      if (!schema || typeof schema !== 'object') return {}
      if (Object.prototype.hasOwnProperty.call(schema, 'example')) return this.cloneSample(schema.example)
      if (Object.prototype.hasOwnProperty.call(schema, 'default')) return this.cloneSample(schema.default)
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return this.cloneSample(schema.enum[0])
      if (Array.isArray(schema.allOf) && schema.allOf.length > 0) {
        return schema.allOf.reduce((out, item) => {
          const sample = this.sampleFromSchema(item)
          return sample && typeof sample === 'object' && !Array.isArray(sample) ? { ...out, ...sample } : out
        }, {})
      }
      if (Array.isArray(schema.oneOf) && schema.oneOf.length > 0) return this.sampleFromSchema(schema.oneOf[0])
      if (Array.isArray(schema.anyOf) && schema.anyOf.length > 0) return this.sampleFromSchema(schema.anyOf[0])

      const schemaType = Array.isArray(schema.type) ? schema.type[0] : schema.type
      if (schemaType === 'object' || (!schemaType && schema.properties)) {
        return Object.entries(schema.properties || {}).reduce((out, [key, property]) => {
          out[key] = this.sampleFromSchema(property)
          return out
        }, {})
      }
      if (schemaType === 'array') return [this.sampleFromSchema(schema.items)]
      if (schemaType === 'integer') return 1
      if (schemaType === 'number') return 1.0
      if (schemaType === 'boolean') return true
      if (schemaType === 'string') {
        if (schema.format === 'date-time') return '2026-01-01T00:00:00Z'
        if (schema.format === 'date') return '2026-01-01'
        return 'string'
      }
      return {}
    },
    stringifySample(value) {
      if (value == null) return ''
      if (typeof value === 'string') return value
      if (typeof value === 'number' || typeof value === 'boolean') return String(value)
      return JSON.stringify(value)
    },
    cloneSample(value) {
      if (value == null || typeof value !== 'object') return value
      return JSON.parse(JSON.stringify(value))
    },
    normalizeObject(value) {
      return value && typeof value === 'object' && !Array.isArray(value) ? value : {}
    },
    normalizeRequest(request) {
      if (!request) return {}
      if (typeof request === 'string') {
        try {
          return JSON.parse(request)
        } catch (error) {
          return {}
        }
      }
      return JSON.parse(JSON.stringify(request))
    },
    normalizePathVariables(path) {
      return String(path || '')
        .replace(/\{\{+([^{}]+)\}\}+/g, '{$1}')
    },
    pathForRun(path) {
      return String(path || '').replace(/(^|[^{])\{([^{}]+)\}(?!\})/g, '$1{{$2}}')
    },
    defaultTab() {
      const m = String(this.request.method || '').toUpperCase()
      if (m === 'POST' || m === 'PUT') {
        return 'body'
      }
      if (Object.keys(this.pathVariables(this.request.path)).length > 0) {
        return 'params'
      }
      return this.activeTab
    },
    pathVariables(path) {
      const variables = {}
      const source = String(path || '')
      for (const match of source.matchAll(/\{\{+([^{}]+)\}\}+/g)) {
        variables[match[1]] = '1'
      }
      for (const match of source.matchAll(/(^|[^{])\{([^{}]+)\}(?!\})/g)) {
        variables[match[2]] = '1'
      }
      return variables
    },
    formatJSON(value) {
      return JSON.stringify(value, null, 2)
    },
    formatEditableJSON(value) {
      if (!value) return '{}'
      if (typeof value === 'string') {
        try {
          return JSON.stringify(JSON.parse(value), null, 2)
        } catch {
          return value
        }
      }
      return JSON.stringify(value, null, 2)
    },
    metricClass(status) {
      if (status === 'passed') return 'is-success'
      if (status === 'failed' || status === 'error') return 'is-danger'
      if (status) return 'is-warning'
      return 'is-idle'
    },
    statusIcon(status) {
      if (status === 'passed') return 'CircleCheck'
      if (status === 'failed' || status === 'error') return 'CircleClose'
      if (status) return 'Warning'
      return 'Clock'
    },
    statusLabel(status) {
      const labels = {
        running: '运行中',
        passed: '通过',
        failed: '失败',
        error: '错误',
        pending: '等待中'
      }
      return labels[status] || status || '-'
    },
    formatDuration(ms) {
      if (ms == null) return '-'
      const value = Number(ms || 0)
      if (value < 1000) return `${value} ms`
      return `${(value / 1000).toFixed(2)} s`
    },
    durationChipClass(ms) {
      if (ms == null) return 'is-duration'
      return Number(ms) < 100 ? 'is-duration-fast' : 'is-duration'
    },
    emitRunSummary(summary) {
      this.$emit('status-change', summary)
    },
    headersToRows(headers) {
      return Object.entries(headers || {}).map(([key, value]) => ({
        key,
        value: Array.isArray(value) ? value.join(', ') : String(value)
      }))
    },
    formatSnapshotBody(value) {
      if (value == null || value === '') return ''
      if (typeof value !== 'string') return JSON.stringify(value, null, 2)
      try {
        return JSON.stringify(JSON.parse(value), null, 2)
      } catch (error) {
        return value
      }
    },
    async loadSavedRequests() {
      if (!this.testCase?.id) {
        this.savedRequests = []
        return
      }
      try {
        const rows = await listSavedCases(this.testCase.id)
        this.savedRequests = rows.map((row) => this.savedCaseToTab(row))
      } catch {
        this.savedRequests = []
      }
    },
    savedCaseToTab(row, payload = null) {
      const request = this.normalizeRequest(row.request)
      const path = this.normalizePathVariables(request.path || row.path)
      const mergedVariables = {
        ...this.normalizeObject(request.pathVars),
        ...this.normalizeObject(request.variables)
      }
      const { forPath, forRest } = this.splitVariablesByPathTemplate(path, mergedVariables)
      return {
        id: row.id,
        label: row.name,
        createdAt: row.createdAt,
        method: request.method || row.method,
        path,
        security: request.security,
        pathRows: this.objectToRows(forPath),
        queryRows: this.objectToRows(request.query, request.queryEnabled),
        headerRows: this.objectToRows(request.headers),
        variableRows: this.objectToRows(forRest),
        bodyText: request.body == null ? '' : this.formatJSON(request.body),
        payload
      }
    },
    buildCurrentRequestDefinition(body) {
      const queryEnabled = this.queryRows.reduce((out, row) => {
        if (row.key) out[row.key] = row.enabled !== false
        return out
      }, {})
      return {
        method: this.request.method,
        path: this.pathForRun(this.request.path),
        headers: this.rowsToObject(this.headerRows),
        query: this.rowsToObject(this.queryRows),
        queryEnabled,
        variables: {
          ...this.rowsToObject(this.pathRows),
          ...this.rowsToObject(this.variableRows)
        },
        body,
        security: this.request.security
      }
    },
    async saveCurrentRequest() {
      if (!this.testCase) return
      let body = null
      if (this.bodyText.trim()) {
        try {
          body = JSON.parse(this.bodyText)
        } catch (error) {
          this.$message.error(`Body 不是合法 JSON：${error.message}`)
          return
        }
      }
      const label = this.runName.trim() || `参数 ${this.savedRequests.length + 1}`
      this.savingCase = true
      try {
        const created = await createSavedCase(this.testCase.id, {
          name: label,
          method: this.request.method,
          path: this.request.path,
          request: this.buildCurrentRequestDefinition(body),
          assertions: this.testCase.assertions || []
        })
        const saved = this.savedCaseToTab(
          created,
          this.payload ? JSON.parse(JSON.stringify(this.payload)) : null
        )
        const existedIndex = this.savedRequests.findIndex((item) => item.id === saved.id)
        if (existedIndex >= 0) {
          this.savedRequests.splice(existedIndex, 1, saved)
          this.activeSavedIndex = existedIndex
        } else {
          this.savedRequests.push(saved)
          this.activeSavedIndex = this.savedRequests.length - 1
        }
        this.$message.success(`测试用例「${label}」已保存到数据库`)
      } finally {
        this.savingCase = false
      }
    },
    applySavedRequest(saved, index) {
      if (this.activeSavedIndex === index) {
        this.activeSavedIndex = -1
        this.payload = null
        this.emitRunSummary({ running: false, runStatus: '', resultStatus: '', durationMillis: null })
        this.runName = this.testCase?.name || ''
        this.applyRequest(this.buildGeneratedRequest(this.testCase))
        return
      }
      this.request = {
        method: saved.method,
        path: saved.path,
        headers: this.rowsToObject(saved.headerRows || []),
        query: this.rowsToObject(saved.queryRows || []),
        body: null,
        security: saved.security
      }
      this.pathRows = (saved.pathRows || []).map((r) => ({ ...r }))
      this.queryRows = (saved.queryRows || []).map((r) => ({ ...r }))
      this.headerRows = (saved.headerRows || []).map((r) => ({ ...r }))
      this.variableRows = (saved.variableRows || []).map((r) => ({ ...r }))
      this.setBodyText(saved.bodyText || '')
      if (saved.label) this.runName = saved.label
      this.activeTab = this.defaultTab()
      this.activeSavedIndex = index
      this.payload = saved.payload ? JSON.parse(JSON.stringify(saved.payload)) : null
      if (this.payload) {
        this.resultTab = 'response'
        const result = this.payload?.result || this.payload?.results?.[0]
        this.emitRunSummary({
          running: false,
          runStatus: this.payload.run?.status || '',
          resultStatus: result?.status || '',
          durationMillis: result?.durationMillis ?? null
        })
      } else {
        this.emitRunSummary({ running: false, runStatus: '', resultStatus: '', durationMillis: null })
      }
      this.$message.success(`已加载「${saved.label}」`)
    },
    async deleteSavedRequest(index) {
      const label = this.savedRequests[index]?.label
      const id = this.savedRequests[index]?.id
      if (!id || !this.testCase?.id) return
      try {
        await ElMessageBox.confirm(`确定删除用例「${label}」吗？`, '删除确认', {
          confirmButtonText: '删除',
          cancelButtonText: '取消',
          type: 'warning',
          confirmButtonClass: 'el-button--danger'
        })
      } catch {
        return
      }
      try {
        await deleteSavedCase(this.testCase.id, id)
        this.savedRequests.splice(index, 1)
        if (this.activeSavedIndex === index) {
          this.activeSavedIndex = -1
        } else if (this.activeSavedIndex > index) {
          this.activeSavedIndex -= 1
        }
        if (label) this.$message.info(`已删除「${label}」`)
      } catch {
        // 错误消息已由请求拦截器统一展示
      }
    },
    startSavedRename(index) {
      const saved = this.savedRequests[index]
      if (!saved) return
      this.renamingIndex = index
      this.renamingName = saved.label
      this.$nextTick(() => {
        const input = this._savedRenameRefs?.[index]
        if (input) { input.focus(); input.select() }
      })
    },
    setSavedRenameRef(index, el) {
      if (!this._savedRenameRefs) this._savedRenameRefs = {}
      if (el) this._savedRenameRefs[index] = el
      else delete this._savedRenameRefs[index]
    },
    async finishSavedRename() {
      const index = this.renamingIndex
      if (index < 0) return
      const name = (this.renamingName || '').trim()
      this.renamingIndex = -1
      this.renamingName = ''
      const saved = this.savedRequests[index]
      if (!saved || !name || name === saved.label) return
      const prevLabel = saved.label
      saved.label = name
      try {
        await patchCase(saved.id, { name })
      } catch {
        saved.label = prevLabel
      }
    },
    cancelSavedRename() {
      this.renamingIndex = -1
      this.renamingName = ''
    },

    parseAssertions(raw) {
      if (!raw) return []
      if (Array.isArray(raw)) return raw
      if (typeof raw === 'string') {
        try { return JSON.parse(raw) } catch { return [] }
      }
      return []
    },

    async saveAssertions() {
      if (!this.testCase) return
      this.savingAssertions = true
      this.assertionsSaved = false
      try {
        await patchCase(this.testCase.id, { assertions: this.editableAssertions })
        this.testCase.assertions = this.editableAssertions
        this.assertionsSaved = true
        setTimeout(() => { this.assertionsSaved = false }, 2000)
      } finally {
        this.savingAssertions = false
      }
    },

    assertionTypeLabel(type) {
      return ASSERTION_TYPE_LABELS[type] || type || '-'
    },

    assertionTypeColor(type) {
      return ASSERTION_TYPE_COLORS[type] || ''
    },

    assertionSummary(row) {
      if (!row) return ''
      if (row.type === 'jsonpath') return row.path || ''
      if (row.type === 'header') return row.name || ''
      if (row.type === 'body_contains') return row.op || ''
      if (row.type === 'response_time') return `${row.op} ${row.expected}ms`
      if (row.type === 'status_code') return `= ${row.expected}`
      return ''
    }
  }
}
</script>

<style scoped>
.debug-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.debug-page.is-embedded {
  padding: 2px 0;
}

.debug-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 2px 2px 0;
}

.debug-title-wrap {
  min-width: 0;
}

.debug-title {
  margin: 4px 0;
  font-size: var(--app-font-size-title);
}

.debug-actions,
.request-line,
.body-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.request-card {
  overflow: hidden;
  border-radius: 12px;
  background: #ffffff;
}

.request-card :deep(.el-card__body) {
  padding: 0;
}

.result-card {
  overflow: hidden;
  border-radius: 12px;
  background: #ffffff;
}

.result-card :deep(.el-card__body) {
  padding: 16px;
}

.result-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.result-title h3 {
  margin: 0 0 6px;
  font-size: var(--app-font-size-title);
}

.result-title p {
  margin: 0;
  color: var(--app-secondary-text);
}

.result-metrics {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.response-overview-tags {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.response-msg-tag-text {
  display: inline-block;
  max-width: min(320px, 42vw);
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}

.metric-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 26px;
  padding: 3px 9px;
  border-radius: 999px;
  font-size: var(--app-font-size-small);
  font-weight: 600;
  white-space: nowrap;
}

.metric-chip.is-idle {
  background: #f1f5f9;
  color: #64748b;
}

.metric-chip.is-warning,
.metric-chip.is-duration {
  background: #fef3c7;
  color: #d97706;
}

.metric-chip.is-success,
.metric-chip.is-duration-fast {
  background: #dcfce7;
  color: #16a34a;
}

.metric-chip.is-danger {
  background: #fee2e2;
  color: #dc2626;
}

.environment-picker {
  display: flex;
  align-items: center;
  gap: 8px;
}

.environment-select-control {
  position: relative;
  width: 280px;
}

.environment-select {
  width: 100%;
}

.method-select {
  width: 118px;
  flex: 0 0 auto;
}

.request-line {
  padding: 14px 16px;
  border-bottom: 1px solid var(--app-border-color);
  background: #ffffff;
}

.request-line .send-actions {
  margin-left: auto;
}

.request-line :deep(.el-select__wrapper),
.request-line :deep(.el-input__wrapper) {
  min-height: 42px;
  box-shadow: 0 0 0 1px var(--app-border-color) inset;
}

.inline-sql-hint {
  padding: 8px 16px;
  border-bottom: 1px solid var(--app-border-color);
  background: #f8fafc;
  color: var(--app-secondary-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
}

.send-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

.send-button {
  min-width: 88px;
  min-height: 42px;
  font-weight: 700;
}

.saved-requests-tabs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}

.saved-request-tab {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: 20px;
  background: transparent;
  border: 1px solid transparent;
  color: var(--el-text-color-secondary, #909399);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s;
  user-select: none;
}

.saved-request-tab:hover {
  background: var(--el-fill-color-light, #f5f7fa);
  color: var(--el-text-color-regular, #606266);
}

.saved-request-tab.is-active {
  background: var(--el-color-primary-light-9, #ecf5ff);
  color: var(--el-color-primary, #409eff);
  font-weight: 500;
}

.saved-request-tab-label {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.saved-request-tab-rename {
  width: 90px;
  height: 18px;
  padding: 0 4px;
  border: 1px solid var(--el-color-primary);
  border-radius: 3px;
  outline: none;
  font-size: 12px;
  font-family: inherit;
  line-height: 18px;
  background: #fff;
  color: inherit;
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--el-color-primary) 20%, transparent);
}

.saved-request-tab-close {
  display: none;
  align-items: center;
  justify-content: center;
  width: 15px;
  height: 15px;
  border: none;
  background: none;
  padding: 0;
  cursor: pointer;
  color: var(--el-text-color-placeholder, #c0c4cc);
  font-size: 14px;
  line-height: 1;
  flex-shrink: 0;
  border-radius: 50%;
  transition: background 0.1s, color 0.1s;
}

.saved-request-tab.is-active:hover .saved-request-tab-close {
  display: inline-flex;
}

.saved-request-tab-close:hover {
  background: var(--el-color-danger-light-8, #fde2e2);
  color: var(--el-color-danger, #f56c6c);
}

.request-tabs {
  margin-top: 0;
  padding: 0 16px 16px;
}

.request-tabs :deep(.el-tabs__header) {
  margin: 0 -16px 16px;
  padding: 0 16px;
  border-bottom: 1px solid var(--app-border-color);
  background: #fbfdff;
}

.request-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.request-tabs :deep(.el-tabs__item) {
  height: 44px;
  font-weight: 600;
}

.add-row {
  margin-top: 12px;
}

.params-section {
  margin-bottom: 20px;
}

.params-section:last-child {
  margin-bottom: 0;
}

.params-section-title {
  margin: 0 0 10px;
  font-size: var(--app-font-size-small);
  font-weight: 700;
  color: var(--app-secondary-text);
  letter-spacing: 0.02em;
}

.path-input {
  flex: 1 1 0;
  min-width: 0;
}

.path-input :deep(.el-input__wrapper) {
  min-height: 42px;
  box-shadow: 0 0 0 1px var(--app-border-color) inset;
}

.body-toolbar {
  justify-content: space-between;
  margin-bottom: 8px;
  color: var(--app-secondary-text);
}

.code-editor,
.response-preview pre,
.code-view {
  min-height: 260px;
  max-height: 520px;
  overflow: auto;
  padding: 14px;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-code-bg);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.6;
  white-space: pre-wrap;
  outline: none;
}

.response-empty-card {
  min-height: 220px;
}

.code-view {
  min-height: 240px;
  margin: 0;
}

.response-detail-tabs {
  width: 100%;
}

.response-detail-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.code-view--curl {
  min-height: 160px;
}

.assertion-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
}

.save-ok-hint {
  font-size: var(--app-font-size-small);
  color: var(--el-color-success);
}

.assertion-name {
  font-size: var(--app-font-size-small);
  font-weight: 500;
}

.assertion-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: var(--app-secondary-text);
}

.section-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.section-block.full {
  grid-column: 1 / -1;
}

.section-block h3 {
  margin: 0 0 10px;
  font-size: var(--app-font-size-base);
}

.url-line {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 10px 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-card-bg);
  word-break: break-all;
}

.json-form-item {
  margin-bottom: 18px;
}

.json-form-item :deep(.el-form-item__content) {
  margin-left: 0 !important;
}

.env-config-dialog :deep(.el-dialog__body) {
  max-height: min(78vh, 920px);
  overflow-y: auto;
  padding-top: 8px;
}

.json-field {
  width: 100%;
}

.json-field-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.json-field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 0;
  color: var(--el-text-color-regular);
  font-weight: 500;
  line-height: 1;
}

.json-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

.field-help-icon {
  display: inline-flex;
  width: 16px;
  height: 16px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--el-border-color);
  border-radius: 50%;
  color: var(--el-text-color-secondary);
  cursor: help;
  font-size: var(--app-font-size-small);
  line-height: 1;
}

:global(.json-help-tooltip) {
  max-width: min(520px, 92vw);
}

:global(.json-help-content) {
  line-height: 1.55;
  font-size: 12px;
}

:global(.json-help-content p) {
  margin: 0;
}

:global(.json-help-lead) {
  margin-bottom: 8px !important;
  color: rgba(255, 255, 255, 0.95);
}

:global(.json-help-caption) {
  margin: 10px 0 6px !important;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.92);
}

:global(.json-help-hint) {
  margin-top: 8px !important;
  color: rgba(255, 255, 255, 0.88);
}

:global(.json-help-hint code) {
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.14);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
}

:global(.json-help-code) {
  margin: 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.28);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.45;
  white-space: pre;
  overflow-x: auto;
  max-width: 100%;
  color: #e8f0ff;
}

:global(.json-help-code--scroll) {
  max-height: 220px;
  overflow-y: auto;
}

@media (max-width: 900px) {

  .debug-header,
  .debug-actions,
  .environment-picker,
  .request-line,
  .send-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .section-grid {
    grid-template-columns: 1fr;
  }

  .method-select,
  .path-input,
  .environment-select-control,
  .environment-select {
    width: 100%;
  }


  .send-button {
    width: 100%;
  }
}
</style>

<!-- 下拉层 teleport 到 body，类名单独作用于 environment-select-dropdown -->
<style>
.environment-select-dropdown .el-select-dropdown__item {
  padding-right: 10px;
}

.environment-select-dropdown .env-option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.environment-select-dropdown .env-option-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.environment-select-dropdown .env-option-edit-btn {
  flex-shrink: 0;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.12s ease;
}

.environment-select-dropdown .env-option-edit-btn .el-icon {
  font-size: 16px;
}

.environment-select-dropdown .el-select-dropdown__item:hover .env-option-edit-btn {
  opacity: 1;
  pointer-events: auto;
}
</style>
