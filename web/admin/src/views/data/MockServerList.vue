<template>
  <div class="page-card mock-server-page">
    <div class="page-header mock-server-header">
      <div>
        <h2 class="page-title">Mock Server</h2>
        <p class="page-subtitle">按当前全局项目维护 Mock 服务与匹配规则，启动后通过本机端口直接访问。</p>
      </div>
      <div class="header-actions">
        <el-button :disabled="!projectId" :loading="loadingServers" @click="loadServers">刷新</el-button>
        <el-button type="primary" :disabled="!projectId" @click="openCreateServer">新增 Server</el-button>
      </div>
    </div>

    <el-alert
      v-if="!projectId"
      class="project-hint"
      type="info"
      show-icon
      :closable="false"
      title="请先在顶部选择项目后再管理 Mock Server。"
    />

    <div v-else class="mock-server-layout">
      <section class="server-panel">
        <div class="panel-title-row">
          <div>
            <h3 class="panel-title">Server 列表</h3>
            <p class="panel-subtitle">配置端口并控制启动/停止。</p>
          </div>
        </div>
        <el-table
          :data="servers"
          border
          row-key="id"
          highlight-current-row
          v-loading="loadingServers"
          :row-class-name="serverRowClassName"
          @row-click="selectServer"
        >
          <el-table-column prop="name" label="名称" min-width="150" show-overflow-tooltip />
          <el-table-column label="端口" width="90">
            <template #default="{ row }">{{ serverPort(row) || '-' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="isRunning(row) ? 'success' : 'info'" size="small">{{ serverStatusLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="访问地址" min-width="190" show-overflow-tooltip>
            <template #default="{ row }">{{ accessAddress(row) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="250" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="!isRunning(row)"
                type="success"
                link
                :loading="togglingServerId === row.id"
                @click.stop="startServer(row)"
              >
                启动
              </el-button>
              <el-button v-else type="warning" link :loading="togglingServerId === row.id" @click.stop="stopServer(row)">停止</el-button>
              <el-button type="primary" link @click.stop="openEditServer(row)">编辑</el-button>
              <el-button type="danger" link :loading="deletingServerId === row.id" @click.stop="removeServer(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loadingServers && !servers.length" description="暂无 Mock Server，新增后即可配置规则" />
      </section>

      <section class="route-panel">
        <template v-if="selectedServer">
          <div class="panel-title-row route-title-row">
            <div>
              <h3 class="panel-title">规则维护</h3>
              <p class="panel-subtitle">{{ selectedServer.name || '未命名 Server' }} 的请求匹配与响应配置。</p>
            </div>
            <el-button type="primary" @click="openCreateRoute">新增规则</el-button>
          </div>

          <div class="access-card">
            <span class="access-label">访问地址</span>
            <el-input :model-value="selectedAccessAddress" readonly>
              <template #append>
                <el-button @click="copySelectedAddress">复制</el-button>
              </template>
            </el-input>
          </div>

          <el-table :data="routes" border row-key="id" v-loading="loadingRoutes">
            <el-table-column label="方法" width="92">
              <template #default="{ row }">
                <el-tag :type="methodTagType(routeMethod(row))" size="small">{{ routeMethod(row) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="URL" min-width="210" show-overflow-tooltip>
              <template #default="{ row }">{{ routePath(row) }}</template>
            </el-table-column>
            <el-table-column label="优先级" width="90">
              <template #default="{ row }">{{ routePriority(row) }}</template>
            </el-table-column>
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-tag :type="routeEnabled(row) ? 'success' : 'info'" size="small">{{ routeEnabled(row) ? '启用' : '停用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="响应" width="110">
              <template #default="{ row }">{{ routeResponseStatus(row) }}</template>
            </el-table-column>
            <el-table-column label="类型" width="110">
              <template #default="{ row }">{{ routeResponseBodyType(row) }}</template>
            </el-table-column>
            <el-table-column label="延迟" width="100">
              <template #default="{ row }">{{ routeDelay(row) }} ms</template>
            </el-table-column>
            <el-table-column label="操作" width="280" fixed="right">
              <template #default="{ row }">
                <el-button type="success" link title="带入测试区" @click.stop="openTestRoute(row)">
                  <el-icon><Promotion /></el-icon>
                  测试
                </el-button>
                <el-button type="primary" link title="复制为新规则" @click.stop="openCopyRoute(row)">
                  <el-icon><CopyDocument /></el-icon>
                  复制
                </el-button>
                <el-button type="primary" link @click.stop="openEditRoute(row)">编辑</el-button>
                <el-button type="danger" link :loading="deletingRouteId === row.id" @click.stop="removeRoute(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loadingRoutes && !routes.length" description="暂无规则，新增后即可按条件返回 Mock 响应" />
        </template>
        <el-empty v-else description="请先选择一个 Mock Server" />
      </section>
    </div>

    <el-dialog v-model="serverDialogVisible" :title="serverEditing ? '编辑 Mock Server' : '新增 Mock Server'" width="620px" align-center>
      <el-form :model="serverForm" label-width="90px">
        <el-form-item label="名称"><el-input v-model="serverForm.name" placeholder="例如：订单服务 Mock" /></el-form-item>
        <el-form-item label="端口"><el-input v-model="serverForm.port" placeholder="例如：18080" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="serverForm.description" type="textarea" :rows="3" placeholder="可选，说明用途" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="serverDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingServer" @click="submitServer">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="routeDialogVisible" :title="routeDialogTitle" width="980px" align-center>
      <el-form :model="routeForm" label-width="120px">
        <div class="route-form-grid">
          <el-form-item label="方法">
            <el-select v-model="routeForm.method" class="full-width">
              <el-option v-for="method in httpMethods" :key="method" :label="method" :value="method" />
            </el-select>
          </el-form-item>
          <el-form-item label="URL">
            <el-input v-model="routeForm.path" placeholder="例如：/api/users/{id}" />
          </el-form-item>
          <el-form-item label="优先级">
            <el-input-number v-model="routeForm.priority" class="full-width" :min="0" :max="100000" />
          </el-form-item>
          <el-form-item label="启用">
            <el-switch v-model="routeForm.enabled" />
          </el-form-item>
        </div>

        <div class="form-section-header">
          <span>请求匹配</span>
        </div>
        <el-form-item label="Query JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>按 query 参数等值匹配，需填写 JSON 对象。</span>
            </div>
            <el-input v-model="routeForm.queryJSON" class="json-editor" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }" @blur="formatJSONField('queryJSON', 'Query JSON')" />
          </div>
        </el-form-item>
        <el-form-item label="Header JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>按请求头等值匹配，需填写 JSON 对象。</span>
            </div>
            <el-input v-model="routeForm.headerJSON" class="json-editor" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }" @blur="formatJSONField('headerJSON', 'Header JSON')" />
          </div>
        </el-form-item>
        <el-form-item label="Body 包含">
          <el-input v-model="routeForm.bodyContains" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" placeholder="可选，要求请求 Body 包含该文本" />
        </el-form-item>
        <el-form-item label="Body JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>可选，按 JSON 字段做简单等值匹配。</span>
            </div>
            <el-input v-model="routeForm.bodyJSON" class="json-editor" type="textarea" :autosize="{ minRows: 4, maxRows: 12 }" @blur="formatJSONField('bodyJSON', 'Body JSON', { optional: true })" />
          </div>
        </el-form-item>

        <div class="form-section-header">
          <span>响应配置</span>
        </div>
        <div class="route-form-grid">
          <el-form-item label="状态码">
            <el-input-number v-model="routeForm.responseStatus" class="full-width" :min="100" :max="599" />
          </el-form-item>
          <el-form-item label="响应类型">
            <el-select v-model="routeForm.responseBodyType" class="full-width">
              <el-option label="JSON" value="json" />
              <el-option label="Text" value="text" />
              <el-option label="Raw" value="raw" />
            </el-select>
          </el-form-item>
          <el-form-item label="延迟毫秒">
            <el-input-number v-model="routeForm.delayMillis" class="full-width" :min="0" :max="600000" />
          </el-form-item>
        </div>
        <el-form-item label="响应头 JSON">
          <div class="json-field">
            <div class="json-toolbar">
              <span>返回响应头，需填写 JSON 对象。</span>
            </div>
            <el-input v-model="routeForm.responseHeadersJSON" class="json-editor" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }" @blur="formatJSONField('responseHeadersJSON', '响应头 JSON')" />
          </div>
        </el-form-item>
        <el-form-item label="响应体">
          <div class="json-field">
            <div class="json-toolbar json-toolbar-stack">
              <span>可引用请求参数，例如 &#123;&#123;request.pathvar.id&#125;&#125;、&#123;&#123;request.query.name&#125;&#125;、&#123;&#123;request.body.user.id&#125;&#125;。</span>
              <span>支持运行时模拟数据标签 &#123;&#123;$mock.uuid&#125;&#125;、&#123;&#123;$mock.now&#125;&#125;、&#123;&#123;$mock.email&#125;&#125;、&#123;&#123;$mock.int(1,100)&#125;&#125;、&#123;&#123;$mock.pick(a,b,c)&#125;&#125; 等，每次请求实时生成新值。</span>
            </div>
            <el-input v-model="routeForm.responseBody" class="body-editor" type="textarea" :autosize="{ minRows: 8, maxRows: 18 }" @blur="formatRouteResponseBody" />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="routeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingRoute" @click="submitRoute">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="testDialogVisible" title="测试 Mock 规则" width="960px" align-center>
      <div class="test-panel">
        <div class="test-header">
          <div>
            <h4 class="test-title">请求测试</h4>
            <p class="panel-subtitle">从浏览器请求运行中的 Mock 地址，并与期望响应做简单对照。</p>
          </div>
          <el-button type="primary" :loading="testingMock" :disabled="!canRunTest" @click="runMockTest">发送测试</el-button>
        </div>
        <div class="test-grid">
          <el-select v-model="testForm.method" class="test-method">
            <el-option v-for="method in httpMethodsForTest" :key="method" :label="method" :value="method" />
          </el-select>
          <el-input v-model="testForm.url" placeholder="可输入 /path 或完整 URL" />
        </div>
        <div class="test-grid test-grid--stacked">
          <div class="json-field">
            <div class="json-toolbar">
              <span>Query JSON</span>
            </div>
            <el-input v-model="testForm.queryJSON" class="json-editor" type="textarea" :autosize="{ minRows: 3, maxRows: 8 }" @blur="formatTestQuery" />
          </div>
          <div class="json-field">
            <div class="json-toolbar">
              <span>请求头 JSON</span>
            </div>
            <el-input v-model="testForm.headersJSON" class="json-editor" type="textarea" :autosize="{ minRows: 3, maxRows: 8 }" @blur="formatTestHeaders" />
          </div>
          <div class="json-field test-body-field">
            <div class="json-toolbar">
              <span>请求体</span>
            </div>
            <el-input v-model="testForm.body" class="body-editor" type="textarea" :autosize="{ minRows: 3, maxRows: 10 }" @blur="formatTestBody" />
          </div>
        </div>
        <div class="test-grid test-grid--expected">
          <el-input-number v-model="testForm.expectedStatus" class="full-width" :min="100" :max="599" placeholder="期望状态码" />
          <el-input v-model="testForm.expectedBodyContains" placeholder="期望响应体包含片段（可选）" @input="testForm.expectedBodyContainsAuto = false" />
          <el-input v-model="testForm.expectedBodyJSON" placeholder="期望响应 JSON 片段（可选）" @input="testForm.expectedBodyJSONAuto = false" />
        </div>
        <el-alert
          v-if="testError"
          class="test-alert"
          type="warning"
          show-icon
          :closable="false"
          :title="testError"
          description="如果浏览器提示 Failed to fetch 且 Mock 已启动，通常需要后端 runtime 对跨端口请求提供 CORS/OPTIONS 支持。"
        />
        <div v-if="testResult" class="test-result">
          <div class="test-result-summary">
            <el-tag :type="testResult.ok ? 'success' : 'warning'">HTTP {{ testResult.status }}</el-tag>
            <span>{{ testResult.duration }} ms</span>
            <el-tag v-for="item in testAssertions" :key="item.label" :type="item.pass ? 'success' : 'danger'" size="small">
              {{ item.label }}：{{ item.pass ? '通过' : '未通过' }}
            </el-tag>
          </div>
          <el-tabs>
            <el-tab-pane label="响应体">
              <pre class="test-pre">{{ testResult.body || '(空响应)' }}</pre>
            </el-tab-pane>
            <el-tab-pane label="响应头">
              <pre class="test-pre">{{ formatJSONValue(testResult.headers) }}</pre>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
      <template #footer>
        <el-button @click="testDialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="testingMock" :disabled="!canRunTest" @click="runMockTest">发送测试</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  createMockRoute,
  createMockServer,
  deleteMockRoute,
  deleteMockServer,
  listMockRoutes,
  listMockServers,
  startMockServer,
  stopMockServer,
  updateMockRoute,
  updateMockServer
} from '../../api'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'

export default {
  name: 'MockServerList',
  data() {
    return {
      servers: [],
      routes: [],
      selectedServerId: '',
      loadingServers: false,
      loadingRoutes: false,
      togglingServerId: '',
      deletingServerId: '',
      deletingRouteId: '',
      savingServer: false,
      savingRoute: false,
      serverDialogVisible: false,
      routeDialogVisible: false,
      testDialogVisible: false,
      serverEditing: null,
      routeEditing: null,
      routeCopying: false,
      serverForm: this.emptyServerForm(),
      routeForm: this.emptyRouteForm(),
      testForm: this.emptyTestForm(),
      testingMock: false,
      testResult: null,
      testError: '',
      httpMethods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'ANY'],
      httpMethodsForTest: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
    }
  },
  async created() {
    await loadGlobalProjects()
    await this.loadServers()
  },
  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    selectedServer() {
      return this.servers.find((server) => server.id === this.selectedServerId) || null
    },
    selectedAccessAddress() {
      return this.selectedServer ? this.accessAddress(this.selectedServer) : ''
    },
    routeDialogTitle() {
      if (this.routeEditing) return '编辑规则'
      return this.routeCopying ? '复制规则' : '新增规则'
    },
    canRunTest() {
      return !!this.selectedServer && !!this.testForm.url && !this.testingMock
    },
    testAssertions() {
      if (!this.testResult) return []
      const assertions = []
      const expectedStatus = Number(this.testForm.expectedStatus)
      if (this.testForm.expectedStatus != null && this.testForm.expectedStatus !== '' && Number.isInteger(expectedStatus)) {
        assertions.push({
          label: '状态码',
          pass: this.testResult.status === expectedStatus
        })
      }
      const expectedText = this.testForm.expectedBodyContains.trim()
      if (expectedText) {
        assertions.push({
          label: 'Body 片段',
          pass: this.testResult.body.includes(expectedText)
        })
      }
      const expectedJSONText = this.testForm.expectedBodyJSON.trim()
      if (expectedJSONText) {
        const expectedJSON = this.parseJSONSilently(expectedJSONText)
        const actualJSON = this.parseJSONSilently(this.testResult.body)
        assertions.push({
          label: 'Body JSON',
          pass: expectedJSON.ok && actualJSON.ok && this.jsonContains(actualJSON.value, expectedJSON.value)
        })
      }
      return assertions
    }
  },
  watch: {
    projectId() {
      this.loadServers()
    }
  },
  methods: {
    emptyServerForm() {
      return {
        name: '',
        description: '',
        port: ''
      }
    },
    emptyRouteForm() {
      return {
        method: 'GET',
        path: '/',
        priority: 100,
        enabled: true,
        queryJSON: '{}',
        headerJSON: '{}',
        bodyContains: '',
        bodyJSON: '',
        responseStatus: 200,
        responseHeadersJSON: '{\n  "Content-Type": "application/json"\n}',
        responseBody: '{\n  "ok": true\n}',
        responseBodyType: 'json',
        delayMillis: 0
      }
    },
    emptyTestForm() {
      return {
        method: 'GET',
        url: '',
        queryJSON: '{}',
        headersJSON: '{}',
        body: '',
        expectedStatus: 200,
        expectedBodyContains: '',
        expectedBodyJSON: '',
        expectedBodyContainsAuto: false,
        expectedBodyJSONAuto: false,
        responseBodyTemplate: '',
        routePathTemplate: ''
      }
    },
    serverPort(row) {
      return row?.port ?? row?.listenPort ?? row?.listen_port ?? ''
    },
    isRunning(row) {
      if (typeof row?.status?.running === 'boolean') return row.status.running
      if (typeof row?.running === 'boolean') return row.running
      if (typeof row?.isRunning === 'boolean') return row.isRunning
      const status = String(row?.status || row?.runtimeStatus || row?.runtime_status || '').toLowerCase()
      return ['running', 'started', 'active'].includes(status)
    },
    serverStatusLabel(row) {
      return this.isRunning(row) ? '运行中' : '已停止'
    },
    accessAddress(row) {
      if (row?.status?.url) return row.status.url
      const port = this.serverPort(row)
      return port ? `http://localhost:${port}` : '-'
    },
    serverRowClassName({ row }) {
      return row.id === this.selectedServerId ? 'is-selected-server' : ''
    },
    routeMethod(row) {
      return String(row?.method || 'GET').toUpperCase()
    },
    routePath(row) {
      return row?.path || row?.url || '/'
    },
    routePriority(row) {
      return row?.priority ?? 0
    },
    routeEnabled(row) {
      return row?.enabled !== false
    },
    routeResponseStatus(row) {
      return row?.responseStatus ?? row?.response_status ?? 200
    },
    routeResponseBodyType(row) {
      return row?.responseBodyType || row?.response_body_type || 'json'
    },
    routeDelay(row) {
      return row?.delayMillis ?? row?.delay_millis ?? 0
    },
    routeRequestMatch(row) {
      return row?.requestMatch || row?.request_match || {}
    },
    routeResponseBody(row) {
      return row?.responseBody ?? row?.response_body ?? ''
    },
    routeResponseHeaders(row) {
      return row?.responseHeaders || row?.response_headers || {}
    },
    methodTagType(method) {
      const types = {
        GET: 'success',
        POST: 'primary',
        PUT: 'warning',
        PATCH: 'warning',
        DELETE: 'danger'
      }
      return types[method] || 'info'
    },
    formatJSONValue(value, fallback = '{}') {
      if (value == null || value === '') return fallback
      if (typeof value === 'string') {
        try {
          return JSON.stringify(JSON.parse(value), null, 2)
        } catch {
          return value
        }
      }
      return JSON.stringify(value, null, 2)
    },
    parseJSONSilently(value) {
      try {
        return { ok: true, value: JSON.parse(value) }
      } catch {
        return { ok: false, value: null }
      }
    },
    jsonContains(actual, expected) {
      if (expected == null || typeof expected !== 'object') return actual === expected
      if (Array.isArray(expected)) {
        if (!Array.isArray(actual) || actual.length < expected.length) return false
        return expected.every((item, index) => this.jsonContains(actual[index], item))
      }
      if (actual == null || typeof actual !== 'object' || Array.isArray(actual)) return false
      return Object.entries(expected).every(([key, value]) => this.jsonContains(actual[key], value))
    },
    renderMockResponseTemplateForTest(input, finalURL, headers) {
      if (!input || !input.includes('{{')) return input
      return input.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, (placeholder, rawExpr) => {
        const expr = rawExpr.trim()
        if (!expr.startsWith('request.')) return placeholder
        const resolved = this.resolveMockRequestTemplateValue(expr, finalURL, headers)
        return resolved == null ? placeholder : this.mockTemplateValueString(resolved)
      })
    },
    resolveMockRequestTemplateValue(expr, finalURL, headers) {
      const url = this.parseTestURL(finalURL)
      const requestPath = url?.pathname || ''
      const requestURL = url ? `${url.pathname}${url.search}` : finalURL
      if (expr === 'request.method') return this.testForm.method
      if (expr === 'request.path') return requestPath
      if (expr === 'request.url') return requestURL
      if (expr === 'request.body' || expr === 'request.bodyRaw') return this.testForm.body
      if (expr.startsWith('request.query.')) {
        return url?.searchParams.get(expr.slice('request.query.'.length))
      }
      if (expr.startsWith('request.header.')) {
        return this.lookupHeader(headers, expr.slice('request.header.'.length))
      }
      if (expr.startsWith('request.pathvar.')) {
        return this.lookupPathVar(expr.slice('request.pathvar.'.length), requestPath)
      }
      if (expr.startsWith('request.path.')) {
        return this.lookupPathVar(expr.slice('request.path.'.length), requestPath)
      }
      if (expr.startsWith('request.body.')) {
        const parsedBody = this.parseJSONSilently(this.testForm.body)
        if (!parsedBody.ok) return null
        return this.lookupJSONValue(parsedBody.value, expr.slice('request.body.'.length))
      }
      return null
    },
    parseTestURL(value) {
      try {
        return new URL(value, this.selectedAccessAddress && this.selectedAccessAddress !== '-' ? this.selectedAccessAddress : window.location.origin)
      } catch {
        return null
      }
    },
    lookupHeader(headers, name) {
      const entry = Object.entries(headers || {}).find(([key]) => key.toLowerCase() === name.toLowerCase())
      return entry ? entry[1] : null
    },
    lookupPathVar(name, actualPath) {
      const patternParts = String(this.testForm.routePathTemplate || '').replace(/^\/+|\/+$/g, '').split('/').filter(Boolean)
      const actualParts = String(actualPath || '').replace(/^\/+|\/+$/g, '').split('/').filter(Boolean)
      if (patternParts.length !== actualParts.length) return null
      for (let index = 0; index < patternParts.length; index += 1) {
        const part = patternParts[index]
        if (part === `{${name}}`) return actualParts[index]
      }
      return null
    },
    lookupJSONValue(value, path) {
      const directValue = value?.[path]
      if (directValue !== undefined) return directValue
      return path.split('.').reduce((current, key) => {
        if (current == null || typeof current !== 'object') return undefined
        return current[key]
      }, value)
    },
    mockTemplateValueString(value) {
      if (value == null) return ''
      if (typeof value === 'string') return value
      if (typeof value === 'number' || typeof value === 'boolean') return String(value)
      return JSON.stringify(value)
    },
    refreshAutoExpectedResponse(finalURL, headers) {
      if (!this.testForm.responseBodyTemplate) return
      if (!this.testForm.expectedBodyContainsAuto && !this.testForm.expectedBodyJSONAuto) return
      const rendered = this.renderMockResponseTemplateForTest(this.testForm.responseBodyTemplate, finalURL, headers)
      if (this.testForm.expectedBodyContainsAuto) {
        this.testForm.expectedBodyContains = rendered
      }
      if (this.testForm.expectedBodyJSONAuto) {
        this.testForm.expectedBodyJSON = this.formatJSONValue(rendered)
      }
    },
    mergeQueryParams(url, query) {
      const pairs = Object.entries(query || {}).filter(([, value]) => value !== undefined && value !== null)
      if (!pairs.length) return url
      const hashIndex = url.indexOf('#')
      const withoutHash = hashIndex === -1 ? url : url.slice(0, hashIndex)
      const hash = hashIndex === -1 ? '' : url.slice(hashIndex)
      const queryIndex = withoutHash.indexOf('?')
      const base = queryIndex === -1 ? withoutHash : withoutHash.slice(0, queryIndex)
      const params = new URLSearchParams(queryIndex === -1 ? '' : withoutHash.slice(queryIndex + 1))
      for (const [key, value] of pairs) {
        params.delete(key)
        const values = Array.isArray(value) ? value : [value]
        for (const item of values) {
          params.append(key, typeof item === 'object' ? JSON.stringify(item) : String(item))
        }
      }
      const queryString = params.toString()
      return `${base}${queryString ? `?${queryString}` : ''}${hash}`
    },
    buildRouteTestURL(row) {
      const path = this.routePath(row)
      if (!this.selectedAccessAddress || this.selectedAccessAddress === '-') {
        return path
      }
      return /^https?:\/\//i.test(path)
        ? path
        : `${this.selectedAccessAddress.replace(/\/+$/, '')}${path.startsWith('/') ? path : `/${path}`}`
    },
    async loadServers() {
      this.routes = []
      if (!this.projectId) {
        this.servers = []
        this.selectedServerId = ''
        return
      }

      const previousSelectedId = this.selectedServerId
      this.loadingServers = true
      try {
        this.servers = await listMockServers(this.projectId)
        const hasPrevious = this.servers.some((server) => server.id === previousSelectedId)
        this.selectedServerId = hasPrevious ? previousSelectedId : this.servers[0]?.id || ''
      } finally {
        this.loadingServers = false
      }
      await this.loadRoutes()
      this.resetTestForm()
    },
    async loadRoutes() {
      this.routes = []
      if (!this.projectId || !this.selectedServerId) return

      this.loadingRoutes = true
      try {
        this.routes = await listMockRoutes(this.projectId, this.selectedServerId)
      } finally {
        this.loadingRoutes = false
      }
    },
    selectServer(row) {
      if (!row || row.id === this.selectedServerId) return
      this.selectedServerId = row.id
      this.resetTestForm()
      this.loadRoutes()
    },
    openCreateServer() {
      if (!this.projectId) {
        this.$message.warning('请先选择项目')
        return
      }
      this.serverEditing = null
      this.serverForm = this.emptyServerForm()
      this.serverDialogVisible = true
    },
    openEditServer(row) {
      this.serverEditing = row
      this.serverForm = {
        name: row.name || '',
        description: row.description || '',
        port: String(this.serverPort(row) || '')
      }
      this.serverDialogVisible = true
    },
    parsePort(value) {
      const port = Number(String(value ?? '').trim())
      if (!Number.isInteger(port) || port < 1 || port > 65535) {
        this.$message.error('端口必须是 1 到 65535 之间的整数')
        return null
      }
      return port
    },
    buildServerPayload() {
      const name = this.serverForm.name.trim()
      if (!name) {
        this.$message.error('请填写 Server 名称')
        return null
      }
      const port = this.parsePort(this.serverForm.port)
      if (port == null) return null
      return {
        name,
        description: this.serverForm.description,
        port
      }
    },
    async submitServer() {
      const payload = this.buildServerPayload()
      if (!payload) return

      this.savingServer = true
      try {
        const saved = this.serverEditing
          ? await updateMockServer(this.projectId, this.serverEditing.id, payload)
          : await createMockServer(this.projectId, payload)
        this.$message.success('Mock Server 已保存')
        this.serverDialogVisible = false
        this.serverEditing = null
        if (saved?.id) this.selectedServerId = saved.id
        await this.loadServers()
      } finally {
        this.savingServer = false
      }
    },
    async removeServer(row) {
      await this.$confirm(`确认删除 Mock Server「${row.name}」？关联规则也将不可用。`, '提示')
      this.deletingServerId = row.id
      try {
        await deleteMockServer(this.projectId, row.id)
        this.$message.success('Mock Server 已删除')
        if (this.selectedServerId === row.id) this.selectedServerId = ''
        await this.loadServers()
      } finally {
        this.deletingServerId = ''
      }
    },
    async startServer(row) {
      this.togglingServerId = row.id
      try {
        await startMockServer(this.projectId, row.id)
        this.$message.success('Mock Server 已启动')
        await this.loadServers()
      } finally {
        this.togglingServerId = ''
      }
    },
    async stopServer(row) {
      this.togglingServerId = row.id
      try {
        await stopMockServer(this.projectId, row.id)
        this.$message.success('Mock Server 已停止')
        await this.loadServers()
      } finally {
        this.togglingServerId = ''
      }
    },
    async copySelectedAddress() {
      if (!this.selectedAccessAddress || this.selectedAccessAddress === '-') return
      try {
        await navigator.clipboard.writeText(this.selectedAccessAddress)
        this.$message.success('访问地址已复制')
      } catch {
        this.$message.info(this.selectedAccessAddress)
      }
    },
    resetTestForm() {
      this.testForm = {
        ...this.emptyTestForm(),
        url: this.selectedAccessAddress && this.selectedAccessAddress !== '-' ? `${this.selectedAccessAddress}/` : ''
      }
      this.testResult = null
      this.testError = ''
    },
    resolveTestURL() {
      const rawURL = this.testForm.url.trim()
      if (/^https?:\/\//i.test(rawURL)) return rawURL
      if (!this.selectedAccessAddress || this.selectedAccessAddress === '-') return ''
      const base = this.selectedAccessAddress.replace(/\/+$/, '')
      const path = rawURL.startsWith('/') ? rawURL : `/${rawURL}`
      return `${base}${path}`
    },
    formatTestHeaders() {
      this.formatJSONField('testForm.headersJSON', '请求头 JSON')
    },
    formatTestQuery() {
      this.formatJSONField('testForm.queryJSON', 'Query JSON')
    },
    formatTestBody() {
      const body = this.testForm.body.trim()
      if (!body) return
      try {
        this.testForm.body = JSON.stringify(JSON.parse(body), null, 2)
      } catch (error) {
        this.$message.error(`请求体不是合法 JSON：${error.message}`)
      }
    },
    async runMockTest() {
      const url = this.resolveTestURL()
      if (!url) {
        this.$message.warning('请填写测试 URL')
        return
      }
      const query = this.parseJSONObject(this.testForm.queryJSON, 'Query JSON')
      if (query == null) return
      const headers = this.parseJSONObject(this.testForm.headersJSON, '请求头 JSON')
      if (headers == null) return

      const method = this.testForm.method
      const options = { method, headers }
      if (!['GET', 'HEAD'].includes(method) && this.testForm.body !== '') {
        options.body = this.testForm.body
      }
      const finalURL = this.mergeQueryParams(url, query)
      this.refreshAutoExpectedResponse(finalURL, headers)

      this.testingMock = true
      this.testError = ''
      this.testResult = null
      const startedAt = performance.now()
      try {
        const response = await fetch(finalURL, options)
        const body = await response.text()
        this.testResult = {
          ok: response.ok,
          status: response.status,
          statusText: response.statusText,
          headers: Object.fromEntries(response.headers.entries()),
          body,
          duration: Math.round(performance.now() - startedAt)
        }
      } catch (error) {
        this.testError = `测试请求失败：${error.message || 'Failed to fetch'}`
      } finally {
        this.testingMock = false
      }
    },
    openTestRoute(row) {
      const match = this.routeRequestMatch(row)
      const query = match.query || match.queries || row.query || {}
      const headers = match.headers || match.header || row.headers || {}
      const bodyJson = match.bodyJson ?? match.body_json ?? row.bodyJson ?? row.body_json
      const bodyContains = match.bodyContains || match.body_contains || row.bodyContains || row.body_contains || ''
      const responseBody = this.routeResponseBody(row)
      const responseBodyType = String(this.routeResponseBodyType(row) || '').toLowerCase()
      const parsedResponseBody = this.parseJSONSilently(responseBody)
      const shouldExpectJSON = responseBody && (responseBodyType === 'json' || parsedResponseBody.ok)

      this.testForm = {
        ...this.emptyTestForm(),
        method: this.routeMethod(row) === 'ANY' ? 'GET' : this.routeMethod(row),
        url: this.buildRouteTestURL(row),
        queryJSON: this.formatJSONValue(query),
        headersJSON: this.formatJSONValue(headers),
        body: bodyJson == null ? String(bodyContains || '') : this.formatJSONValue(bodyJson),
        expectedStatus: this.routeResponseStatus(row),
        expectedBodyContains: shouldExpectJSON ? '' : String(responseBody || ''),
        expectedBodyJSON: shouldExpectJSON ? this.formatJSONValue(responseBody) : '',
        expectedBodyContainsAuto: !shouldExpectJSON && !!responseBody,
        expectedBodyJSONAuto: !!shouldExpectJSON,
        responseBodyTemplate: String(responseBody || ''),
        routePathTemplate: this.routePath(row)
      }
      this.refreshAutoExpectedResponse(this.mergeQueryParams(this.resolveTestURL(), query), headers)
      this.testResult = null
      this.testError = ''
      this.testDialogVisible = true
      this.$message.success('已将规则带入测试弹窗，可修改后发送')
    },
    openCreateRoute() {
      if (!this.selectedServer) {
        this.$message.warning('请先选择 Mock Server')
        return
      }
      this.routeEditing = null
      this.routeCopying = false
      this.routeForm = this.emptyRouteForm()
      this.routeDialogVisible = true
    },
    buildRouteFormFromRow(row) {
      const match = row.requestMatch || row.request_match || {}
      const bodyJson = match.bodyJson ?? match.body_json ?? row.bodyJson ?? row.body_json
      return {
        method: this.routeMethod(row),
        path: this.routePath(row),
        priority: this.routePriority(row),
        enabled: this.routeEnabled(row),
        queryJSON: this.formatJSONValue(match.query || match.queries || row.query),
        headerJSON: this.formatJSONValue(match.headers || match.header || row.headers),
        bodyContains: match.bodyContains || match.body_contains || row.bodyContains || row.body_contains || '',
        bodyJSON: bodyJson == null ? '' : this.formatJSONValue(bodyJson),
        responseStatus: this.routeResponseStatus(row),
        responseHeadersJSON: this.formatJSONValue(row.responseHeaders || row.response_headers),
        responseBody: row.responseBody ?? row.response_body ?? '',
        responseBodyType: this.routeResponseBodyType(row),
        delayMillis: this.routeDelay(row)
      }
    },
    openEditRoute(row) {
      this.routeEditing = row
      this.routeCopying = false
      this.routeForm = this.buildRouteFormFromRow(row)
      this.routeDialogVisible = true
    },
    openCopyRoute(row) {
      this.routeEditing = null
      this.routeCopying = true
      this.routeForm = this.buildRouteFormFromRow(row)
      this.routeDialogVisible = true
      this.$message.info('已复制规则内容，调整后保存会创建新规则')
    },
    formatJSONField(field, label, options = {}) {
      const target = field.includes('.') ? field.split('.').reduce((obj, key) => obj?.[key], this) : this.routeForm[field]
      if (options.optional && !String(target || '').trim()) return true
      try {
        const formatted = JSON.stringify(JSON.parse(target || '{}'), null, 2)
        if (field.includes('.')) {
          const parts = field.split('.')
          const last = parts.pop()
          const owner = parts.reduce((obj, key) => obj[key], this)
          owner[last] = formatted
        } else {
          this.routeForm[field] = formatted
        }
        return true
      } catch (error) {
        this.$message.error(`${label} 不合法：${error.message}`)
        return false
      }
    },
    formatRouteResponseBody() {
      const body = this.routeForm.responseBody.trim()
      const shouldFormatBody = body && (this.routeForm.responseBodyType === 'json' || /^[\[{]/.test(body))
      if (shouldFormatBody) {
        try {
          this.routeForm.responseBody = JSON.stringify(JSON.parse(body), null, 2)
        } catch (error) {
          this.$message.error(`响应体不是合法 JSON：${error.message}`)
        }
      }
    },
    parseJSONObject(value, label) {
      try {
        const parsed = JSON.parse(value || '{}')
        if (parsed == null || typeof parsed !== 'object' || Array.isArray(parsed)) {
          this.$message.error(`${label} 必须是 JSON 对象`)
          return null
        }
        return parsed
      } catch (error) {
        this.$message.error(`${label} 不合法：${error.message}`)
        return null
      }
    },
    parseOptionalJSON(value, label) {
      const text = String(value || '').trim()
      if (!text) return undefined
      try {
        return JSON.parse(text)
      } catch (error) {
        this.$message.error(`${label} 不合法：${error.message}`)
        return null
      }
    },
    buildRoutePayload() {
      const path = this.routeForm.path.trim()
      if (!path || !path.startsWith('/')) {
        this.$message.error('URL 必须以 / 开头')
        return null
      }
      const query = this.parseJSONObject(this.routeForm.queryJSON, 'Query JSON')
      if (query == null) return null
      const headers = this.parseJSONObject(this.routeForm.headerJSON, 'Header JSON')
      if (headers == null) return null
      const responseHeaders = this.parseJSONObject(this.routeForm.responseHeadersJSON, '响应头 JSON')
      if (responseHeaders == null) return null
      const bodyJson = this.parseOptionalJSON(this.routeForm.bodyJSON, 'Body JSON')
      if (bodyJson === null) return null

      const requestMatch = { query, headers }
      if (this.routeForm.bodyContains.trim()) requestMatch.bodyContains = this.routeForm.bodyContains
      if (bodyJson !== undefined) requestMatch.bodyJson = bodyJson

      return {
        method: this.routeForm.method,
        path,
        priority: Number(this.routeForm.priority) || 0,
        enabled: !!this.routeForm.enabled,
        requestMatch,
        responseStatus: Number(this.routeForm.responseStatus) || 200,
        responseHeaders,
        responseBody: this.routeForm.responseBody,
        responseBodyType: this.routeForm.responseBodyType,
        delayMillis: Number(this.routeForm.delayMillis) || 0
      }
    },
    async submitRoute() {
      const payload = this.buildRoutePayload()
      if (!payload) return

      this.savingRoute = true
      try {
        if (this.routeEditing) {
          await updateMockRoute(this.projectId, this.selectedServerId, this.routeEditing.id, payload)
        } else {
          await createMockRoute(this.projectId, this.selectedServerId, payload)
        }
        this.$message.success('Mock 规则已保存')
        this.routeDialogVisible = false
        this.routeEditing = null
        this.routeCopying = false
        await this.loadRoutes()
      } finally {
        this.savingRoute = false
      }
    },
    async removeRoute(row) {
      await this.$confirm(`确认删除规则「${this.routeMethod(row)} ${this.routePath(row)}」？`, '提示')
      this.deletingRouteId = row.id
      try {
        await deleteMockRoute(this.projectId, this.selectedServerId, row.id)
        this.$message.success('Mock 规则已删除')
        await this.loadRoutes()
      } finally {
        this.deletingRouteId = ''
      }
    }
  }
}
</script>

<style scoped>
.mock-server-header {
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

.header-actions {
  display: inline-flex;
  gap: 8px;
  flex-wrap: wrap;
}

.project-hint {
  margin-bottom: 16px;
}

.mock-server-layout {
  display: grid;
  grid-template-columns: minmax(420px, 0.9fr) minmax(0, 1.1fr);
  gap: 16px;
  align-items: start;
}

.server-panel,
.route-panel {
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
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.panel-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.access-card {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  margin-bottom: 14px;
  border-radius: 6px;
  background: var(--app-code-bg);
}

.access-label {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.test-panel {
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  background: var(--app-page-bg);
}

.test-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.test-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.test-grid {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 10px;
  margin-bottom: 10px;
}

.test-grid--stacked,
.test-grid--expected {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.test-grid--expected {
  grid-template-columns: 150px minmax(0, 1fr) minmax(0, 1fr);
}

.test-method {
  width: 100%;
}

.test-body-field {
  grid-column: 1 / -1;
}

.test-alert {
  margin: 10px 0;
}

.test-result {
  margin-top: 12px;
}

.test-result-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.test-pre {
  max-height: 260px;
  overflow: auto;
  margin: 0;
  padding: 10px;
  border: 1px solid var(--app-border-color);
  border-radius: 6px;
  background: var(--app-code-bg);
  white-space: pre-wrap;
  word-break: break-word;
}

.full-width,
.json-field {
  width: 100%;
}

.form-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 20px 0 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--app-border-color);
  color: var(--app-text-color);
  font-weight: 600;
}

.route-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.json-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
}

.json-toolbar-stack {
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.json-editor :deep(textarea),
.body-editor :deep(textarea),
.test-pre {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.45;
}

:deep(.is-selected-server > td) {
  background: color-mix(in srgb, var(--app-primary-color) 8%, transparent);
}

@media (max-width: 1180px) {
  .mock-server-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .route-form-grid,
  .access-card,
  .test-grid,
  .test-grid--stacked,
  .test-grid--expected {
    grid-template-columns: 1fr;
  }
}
</style>
