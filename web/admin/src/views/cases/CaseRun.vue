<template>
  <div class="debug-page" :class="{ 'is-embedded': embedded }">
    <div v-if="!embedded || savedRequests.length" class="debug-header">
      <div>
        <el-button v-if="!embedded" link type="primary" @click="$router.push('/cases')">返回API管理</el-button>
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
          <el-button :loading="aiParamsLoading" :disabled="aiParamsLoading" @click="openAIParamsDialog">AI 生成</el-button>
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
              <el-table-column label="字段注释" min-width="220">
                <template #default="{ row }">
                  <span class="param-comment">{{ parameterComment(row, 'path') }}</span>
                </template>
              </el-table-column>
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
              <el-table-column label="参数值" min-width="260">
                <template #default="{ row }">
                  <div class="query-value-cell">
                    <el-input v-model="row.value" placeholder="参数值" />
                    <span v-if="row.required" class="required-star" aria-label="必填">*</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="字段注释" min-width="220">
                <template #default="{ row }">
                  <span class="param-comment">{{ parameterComment(row, 'query') }}</span>
                </template>
              </el-table-column>
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
            <el-table-column label="字段注释" min-width="220">
              <template #default="{ row }">
                <span class="param-comment">{{ parameterComment(row, 'header') }}</span>
              </template>
            </el-table-column>
            <el-table-column width="90" label="操作"><template #default="{ $index }"><el-button link type="danger"
                  @click="removeRow(headerRows, $index)">删除</el-button></template></el-table-column>
          </el-table>
          <el-button class="add-row" @click="addRow(headerRows)">新增一行</el-button>
        </el-tab-pane>
        <el-tab-pane label="Body" name="body">
          <div class="body-toolbar">
            <span>JSON Body</span>
            <div class="body-view-toggle">
              <span :class="{ active: requestBodyViewMode === 'edit' }" @click="requestBodyViewMode = 'edit'">编辑</span>
              <span :class="{ active: requestBodyViewMode === 'preview' }" @click="requestBodyViewMode = 'preview'">预览</span>
            </div>
          </div>
          <div class="body-schema-layout" :style="bodySchemaLayoutStyle">
            <div v-if="requestBodyViewMode === 'preview' && requestBodyEditorJsonLines !== null" class="json-viewer code-view">
              <div v-for="line in requestBodyEditorJsonLines" :key="line.id" class="json-line" :style="{ paddingLeft: `${line.depth * 16}px` }">
                <span v-if="line.type === 'open'" class="json-toggle" @click="toggleSchemaCollapse(requestBodyJsonCollapsed, line.path)" />
                <span v-else-if="line.type === 'collapsed'" class="json-toggle is-collapsed" @click="toggleSchemaCollapse(requestBodyJsonCollapsed, line.path)" />
                <span v-else class="json-toggle-ph" />
                <span v-if="line.key !== null && line.key !== undefined" class="json-key">"{{ line.key }}"<span class="json-colon">: </span></span>
                <template v-if="line.type === 'primitive'">
                  <span class="json-value" :class="`json-${line.valueType}`">{{ line.displayValue }}</span>
                </template>
                <template v-else-if="line.type === 'open'">
                  <span class="json-bracket">{{ line.bracket }}</span>
                </template>
                <template v-else-if="line.type === 'close'">
                  <span class="json-bracket">{{ line.bracket }}</span>
                </template>
                <template v-else-if="line.type === 'collapsed'">
                  <span class="json-bracket">{{ line.open }}</span><span class="json-collapsed-preview"> {{ line.count }} {{ line.open === '[' ? 'items' : 'keys' }} </span><span class="json-bracket">{{ line.close }}</span>
                </template>
                <template v-else-if="line.type === 'empty-object'">
                  <span class="json-bracket">{}</span>
                </template>
                <template v-else-if="line.type === 'empty-array'">
                  <span class="json-bracket">[]</span>
                </template>
                <span v-if="line.hasComma" class="json-comma">,</span>
              </div>
            </div>
            <pre v-else-if="requestBodyViewMode === 'preview'" class="code-view">{{ bodyText }}</pre>
            <div v-show="requestBodyViewMode === 'edit'" ref="bodyEditor" class="code-editor" contenteditable spellcheck="false"
              @input="bodyText = $event.target.innerText" @blur="formatBody"></div>
            <div class="body-schema-resizer" title="拖拽调整注释区宽度" @pointerdown="startBodySchemaResize"></div>
            <div class="schema-panel">
              <div class="schema-panel-title-row">
                <div class="schema-panel-title">请求 Body</div>
                <span v-if="requestBodyAllRows.length" class="schema-field-count">{{ requestBodyAllRows.length }} 字段</span>
              </div>
              <div v-if="requestBodyAllRows.length" class="schema-field-table">
                <div class="schema-field-table-head">
                  <span>字段</span>
                  <span>类型</span>
                  <span>说明</span>
                </div>
                <div v-for="row in requestBodyDocRows" :key="row.path" class="schema-field-row">
                  <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                    <span v-if="row.hasChildren" class="schema-toggle" :class="{ 'is-collapsed': requestBodyCollapsed[row.path] }" @click="toggleSchemaCollapse(requestBodyCollapsed, row.path)" />
                    <span v-else class="schema-toggle-placeholder" />
                    <code :title="row.path">{{ row.name }}</code>
                    <span v-if="row.required" class="schema-required">*</span>
                  </div>
                  <span class="schema-type-chip">{{ row.type }}</span>
                  <span class="schema-field-meaning" :title="row.meaning">{{ row.meaning || '-' }}</span>
                </div>
              </div>
              <el-empty v-else description="暂无字段说明" :image-size="42" />
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane label="断言" name="assertions">
          <AssertionEditor v-model="editableAssertions" :ai-response-snapshot="responseSnapshot" />
          <div class="assertion-actions">
            <el-button size="small" :loading="savingAssertions" @click="saveAssertions">保存断言</el-button>
            <span v-if="assertionsSaved" class="save-ok-hint">已保存</span>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-card class="result-card">
      <div v-if="payload" class="result-title">
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
                <el-table-column label="字段注释" min-width="220">
                  <template #default="{ row }">
                    <span class="param-comment">{{ parameterComment(row, 'query') }}</span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="section-block">
              <h3>Headers</h3>
              <el-table :data="headersToRows(requestSnapshot.headers)" border>
                <el-table-column prop="key" label="名称" />
                <el-table-column prop="value" label="值" />
                <el-table-column label="字段注释" min-width="220">
                  <template #default="{ row }">
                    <span class="param-comment">{{ parameterComment(row, 'header') }}</span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="section-block full">
              <h3>Body</h3>
              <div class="body-schema-layout" :style="bodySchemaLayoutStyle">
                <div v-if="requestSnapshotBodyJsonLines !== null" class="json-viewer code-view">
                  <div v-for="line in requestSnapshotBodyJsonLines" :key="line.id" class="json-line" :style="{ paddingLeft: `${line.depth * 16}px` }">
                    <span v-if="line.type === 'open'" class="json-toggle" @click="toggleSchemaCollapse(requestSnapshotJsonCollapsed, line.path)" />
                    <span v-else-if="line.type === 'collapsed'" class="json-toggle is-collapsed" @click="toggleSchemaCollapse(requestSnapshotJsonCollapsed, line.path)" />
                    <span v-else class="json-toggle-ph" />
                    <span v-if="line.key !== null && line.key !== undefined" class="json-key">"{{ line.key }}"<span class="json-colon">: </span></span>
                    <template v-if="line.type === 'primitive'">
                      <span class="json-value" :class="`json-${line.valueType}`">{{ line.displayValue }}</span>
                    </template>
                    <template v-else-if="line.type === 'open'">
                      <span class="json-bracket">{{ line.bracket }}</span>
                    </template>
                    <template v-else-if="line.type === 'close'">
                      <span class="json-bracket">{{ line.bracket }}</span>
                    </template>
                    <template v-else-if="line.type === 'collapsed'">
                      <span class="json-bracket">{{ line.open }}</span><span class="json-collapsed-preview"> {{ line.count }} {{ line.open === '[' ? 'items' : 'keys' }} </span><span class="json-bracket">{{ line.close }}</span>
                    </template>
                    <template v-else-if="line.type === 'empty-object'">
                      <span class="json-bracket">{}</span>
                    </template>
                    <template v-else-if="line.type === 'empty-array'">
                      <span class="json-bracket">[]</span>
                    </template>
                    <span v-if="line.hasComma" class="json-comma">,</span>
                  </div>
                </div>
                <pre v-else class="code-view">{{ formatSnapshotBody(requestSnapshot.body) }}</pre>
                <div class="body-schema-resizer" title="拖拽调整注释区宽度" @pointerdown="startBodySchemaResize"></div>
                <div class="schema-panel">
                  <div class="schema-panel-title-row">
                    <div class="schema-panel-title">请求 Body</div>
                    <span v-if="requestBodyAllRows.length" class="schema-field-count">{{ requestBodyAllRows.length }} 字段</span>
                  </div>
                  <div v-if="requestBodyAllRows.length" class="schema-field-table">
                    <div class="schema-field-table-head">
                      <span>字段</span>
                      <span>类型</span>
                      <span>说明</span>
                    </div>
                    <div v-for="row in requestBodyDocRows" :key="row.path" class="schema-field-row">
                      <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                        <span v-if="row.hasChildren" class="schema-toggle" :class="{ 'is-collapsed': requestBodyCollapsed[row.path] }" @click="toggleSchemaCollapse(requestBodyCollapsed, row.path)" />
                        <span v-else class="schema-toggle-placeholder" />
                        <code :title="row.path">{{ row.name }}</code>
                        <span v-if="row.required" class="schema-required">*</span>
                      </div>
                      <span class="schema-type-chip">{{ row.type }}</span>
                      <span class="schema-field-meaning" :title="row.meaning">{{ row.meaning || '-' }}</span>
                    </div>
                  </div>
                  <el-empty v-else description="暂无字段说明" :image-size="42" />
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="响应" name="response">
          <el-tabs v-model="responseDetailTab" class="response-detail-tabs response-detail-tabs--subtle">
            <el-tab-pane label="Body" name="body">
              <div class="body-schema-layout" :style="bodySchemaLayoutStyle">
                <div v-if="responseBodyJsonLines !== null" class="json-viewer code-view">
                  <div v-for="line in responseBodyJsonLines" :key="line.id" class="json-line" :style="{ paddingLeft: `${line.depth * 16}px` }">
                    <span v-if="line.type === 'open'" class="json-toggle" @click="toggleSchemaCollapse(responseJsonCollapsed, line.path)" />
                    <span v-else-if="line.type === 'collapsed'" class="json-toggle is-collapsed" @click="toggleSchemaCollapse(responseJsonCollapsed, line.path)" />
                    <span v-else class="json-toggle-ph" />
                    <span v-if="line.key !== null && line.key !== undefined" class="json-key">"{{ line.key }}"<span class="json-colon">: </span></span>
                    <template v-if="line.type === 'primitive'">
                      <span class="json-value" :class="`json-${line.valueType}`">{{ line.displayValue }}</span>
                    </template>
                    <template v-else-if="line.type === 'open'">
                      <span class="json-bracket">{{ line.bracket }}</span>
                    </template>
                    <template v-else-if="line.type === 'close'">
                      <span class="json-bracket">{{ line.bracket }}</span>
                    </template>
                    <template v-else-if="line.type === 'collapsed'">
                      <span class="json-bracket">{{ line.open }}</span><span class="json-collapsed-preview"> {{ line.count }} {{ line.open === '[' ? 'items' : 'keys' }} </span><span class="json-bracket">{{ line.close }}</span>
                    </template>
                    <template v-else-if="line.type === 'empty-object'">
                      <span class="json-bracket">{}</span>
                    </template>
                    <template v-else-if="line.type === 'empty-array'">
                      <span class="json-bracket">[]</span>
                    </template>
                    <span v-if="line.hasComma" class="json-comma">,</span>
                  </div>
                </div>
                <pre v-else class="code-view">{{ formatSnapshotBody(responseSnapshot.body) }}</pre>
                <div class="body-schema-resizer" title="拖拽调整注释区宽度" @pointerdown="startBodySchemaResize"></div>
                <div class="schema-panel">
                  <div class="schema-panel-title-row">
                    <div class="schema-panel-title">响应 Body</div>
                    <span v-if="responseBodyAllRows.length" class="schema-field-count">{{ responseBodyAllRows.length }} 字段</span>
                  </div>
                  <div v-if="responseBodyAllRows.length" class="schema-field-table">
                    <div class="schema-field-table-head">
                      <span>字段</span>
                      <span>类型</span>
                      <span>说明</span>
                    </div>
                    <div v-for="row in responseBodyDocRows" :key="row.path" class="schema-field-row">
                      <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                        <span v-if="row.hasChildren" class="schema-toggle" :class="{ 'is-collapsed': responseBodyCollapsed[row.path] }" @click="toggleSchemaCollapse(responseBodyCollapsed, row.path)" />
                        <span v-else class="schema-toggle-placeholder" />
                        <code :title="row.path">{{ row.name }}</code>
                      </div>
                      <span class="schema-type-chip">{{ row.type }}</span>
                      <span class="schema-field-meaning" :title="row.meaning">{{ row.meaning || '-' }}</span>
                    </div>
                  </div>
                  <el-empty v-else description="暂无字段说明" :image-size="42" />
                </div>
              </div>
            </el-tab-pane>
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
            </div>
            <el-input v-model="envForm.variables" class="json-editor" type="textarea"
              :autosize="{ minRows: 6, maxRows: 28 }" placeholder='{"token":"env-token","page":1}' @blur="formatEnvironmentVariablesJson" />
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
            </div>
            <el-input v-model="envForm.auth" class="json-editor" type="textarea" :autosize="{ minRows: 6, maxRows: 28 }"
              placeholder='{"defaultProfile":"user","profiles":{"user":{"type":"bearer","token":"{{userToken}}"},"admin":{"type":"api_key","in":"query","name":"admin_token","value":"{{adminToken}}"}},"securitySchemes":{"UserAuth":"user","AdminAuth":"admin"}}' @blur="formatEnvironmentAuthJson" />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="envDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingEnvironment" @click="submitEnvironment">保存</el-button>
      </template>
    </el-dialog>

    <AIGenerateDialog
      v-model="aiParamsDialogVisible"
      action="generate_params"
      :context="aiParamsContext"
      @apply="onAIParamsApply"
      @generation-settled="aiParamsLoading = false"
    />

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
import AIGenerateDialog from '../../components/AIGenerateDialog.vue'

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
const BODY_SCHEMA_WIDTH_STORAGE_KEY = 'autotest.runConsole.bodySchemaPanelWidth'
const BODY_SCHEMA_PANEL_MIN_WIDTH = 260
const BODY_SCHEMA_PANEL_MAX_WIDTH = 760
const BODY_SCHEMA_EDITOR_MIN_WIDTH = 320

function clampNumber(value, min, max) {
  return Math.min(Math.max(value, min), max)
}

function readStoredBodySchemaPanelWidth() {
  try {
    const value = Number(localStorage.getItem(BODY_SCHEMA_WIDTH_STORAGE_KEY))
    if (Number.isFinite(value) && value > 0) {
      return clampNumber(value, BODY_SCHEMA_PANEL_MIN_WIDTH, BODY_SCHEMA_PANEL_MAX_WIDTH)
    }
  } catch {
    // Use the CSS default when localStorage is unavailable.
  }
  return null
}

function storeBodySchemaPanelWidth(width) {
  try {
    localStorage.setItem(BODY_SCHEMA_WIDTH_STORAGE_KEY, String(Math.round(width)))
  } catch {
    // Ignore storage failures; the width remains effective for this session.
  }
}

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
  components: { AssertionEditor, AIGenerateDialog },
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
      responseDetailTab: 'body',
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
      assertionsSaved: false,
      bodySchemaPanelWidth: readStoredBodySchemaPanelWidth(),
      bodySchemaResizeState: null,
      requestBodyCollapsed: {},
      requestBodyViewMode: 'edit',
      requestBodyJsonCollapsed: {},
      aiParamsDialogVisible: false,
      aiParamsLoading: false,
      responseBodyCollapsed: {},
      responseJsonCollapsed: {},
      requestSnapshotJsonCollapsed: {}
    }
  },
  computed: {
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
    },
    aiParamsContext() {
      const current = this.normalizeRequest(this.request)
      let bodyDraft = null
      if (this.bodyText && this.bodyText.trim()) {
        try {
          bodyDraft = JSON.parse(this.bodyText)
        } catch (_) {
          bodyDraft = this.bodyText
        }
      }
      return {
        method: current.method,
        path: current.path,
        endpoint: this.endpoint
          ? {
              summary: this.endpoint.summary,
              description: this.endpoint.description,
              parameters: this.endpoint.parameters,
              requestSchema: this.endpoint.requestSchema,
              responses: this.endpoint.responses
            }
          : null,
        currentRequest: {
          path: current.path,
          query: current.query,
          headers: current.headers,
          variables: current.variables,
          body: bodyDraft
        }
      }
    },
    requestSchema() {
      return this.normalizeRequest(this.endpoint?.requestSchema)
    },
    responseSchema() {
      return this.normalizeRequest(this.endpoint?.responseSchema)
    },
    requestBodySchema() {
      return this.requestSchema?.body || null
    },
    responseBodySchema() {
      return this.responseSchema?.body || null
    },
    requestBodyAllRows() {
      return this.schemaDocRows(this.requestBodySchema)
    },
    responseBodyAllRows() {
      return this.schemaDocRows(this.responseBodySchema)
    },
    requestBodyEditorJsonLines() {
      if (this.requestBodyViewMode !== 'preview') return null
      try {
        const parsed = JSON.parse(this.bodyText)
        const lines = []
        this.collectJsonLines(parsed, null, '$', 0, true, this.requestBodyJsonCollapsed, lines)
        return lines
      } catch { return null }
    },
    responseBodyParsed() {
      const body = this.responseSnapshot.body
      if (body == null || body === '') return undefined
      try { return typeof body === 'string' ? JSON.parse(body) : body } catch { return undefined }
    },
    requestSnapshotBodyParsed() {
      const body = this.requestSnapshot.body
      if (body == null || body === '') return undefined
      try { return typeof body === 'string' ? JSON.parse(body) : body } catch { return undefined }
    },
    responseBodyJsonLines() {
      if (this.responseBodyParsed === undefined) return null
      const lines = []
      this.collectJsonLines(this.responseBodyParsed, null, '$', 0, true, this.responseJsonCollapsed, lines)
      return lines
    },
    requestSnapshotBodyJsonLines() {
      if (this.requestSnapshotBodyParsed === undefined) return null
      const lines = []
      this.collectJsonLines(this.requestSnapshotBodyParsed, null, '$', 0, true, this.requestSnapshotJsonCollapsed, lines)
      return lines
    },
    requestBodyDocRows() {
      return this.filterSchemaRows(this.requestBodyAllRows, this.requestBodyCollapsed)
    },
    responseBodyDocRows() {
      return this.filterSchemaRows(this.responseBodyAllRows, this.responseBodyCollapsed)
    },
    bodySchemaLayoutStyle() {
      return Number.isFinite(this.bodySchemaPanelWidth)
        ? { '--body-schema-panel-width': `${this.bodySchemaPanelWidth}px` }
        : {}
    }
  },
  async created() {
    await this.loadData()
  },
  beforeUnmount() {
    this.stopBodySchemaResize()
  },
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
    },
    aiParamsDialogVisible(val) {
      if (!val) this.aiParamsLoading = false
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
    startBodySchemaResize(event) {
      if (event.button !== undefined && event.button !== 0) return
      const layout = event.currentTarget?.closest?.('.body-schema-layout')
      if (!layout) return
      const rect = layout.getBoundingClientRect()
      this.bodySchemaResizeState = {
        right: rect.right,
        maxWidth: Math.max(
          BODY_SCHEMA_PANEL_MIN_WIDTH,
          Math.min(BODY_SCHEMA_PANEL_MAX_WIDTH, rect.width - BODY_SCHEMA_EDITOR_MIN_WIDTH)
        )
      }
      document.addEventListener('pointermove', this.onBodySchemaResize)
      document.addEventListener('pointerup', this.stopBodySchemaResize)
      document.addEventListener('pointercancel', this.stopBodySchemaResize)
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      event.currentTarget.setPointerCapture?.(event.pointerId)
      event.preventDefault()
      this.onBodySchemaResize(event)
    },
    onBodySchemaResize(event) {
      if (!this.bodySchemaResizeState) return
      const nextWidth = this.bodySchemaResizeState.right - event.clientX
      this.bodySchemaPanelWidth = clampNumber(
        nextWidth,
        BODY_SCHEMA_PANEL_MIN_WIDTH,
        this.bodySchemaResizeState.maxWidth
      )
    },
    stopBodySchemaResize() {
      if (!this.bodySchemaResizeState) return
      document.removeEventListener('pointermove', this.onBodySchemaResize)
      document.removeEventListener('pointerup', this.stopBodySchemaResize)
      document.removeEventListener('pointercancel', this.stopBodySchemaResize)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      storeBodySchemaPanelWidth(this.bodySchemaPanelWidth)
      this.bodySchemaResizeState = null
    },
    toggleSchemaCollapse(collapsedObj, path) {
      if (collapsedObj[path]) {
        delete collapsedObj[path]
      } else {
        collapsedObj[path] = true
      }
    },
    collectJsonLines(value, keyName, path, depth, isLast, collapsed, lines) {
      if (value === null || typeof value !== 'object') {
        lines.push({
          id: path,
          type: 'primitive',
          depth,
          key: keyName,
          displayValue: this.formatJsonPrimitive(value),
          valueType: value === null ? 'null' : typeof value,
          hasComma: !isLast
        })
        return
      }
      const isArray = Array.isArray(value)
      const keys = isArray ? null : Object.keys(value)
      const count = isArray ? value.length : keys.length
      if (count === 0) {
        lines.push({ id: path, type: isArray ? 'empty-array' : 'empty-object', depth, key: keyName, hasComma: !isLast })
        return
      }
      if (collapsed[path]) {
        lines.push({ id: path, type: 'collapsed', depth, key: keyName, open: isArray ? '[' : '{', close: isArray ? ']' : '}', count, path, hasComma: !isLast })
        return
      }
      lines.push({ id: `${path}:o`, type: 'open', depth, key: keyName, bracket: isArray ? '[' : '{', path })
      if (isArray) {
        value.forEach((item, i) => {
          this.collectJsonLines(item, null, `${path}[${i}]`, depth + 1, i === value.length - 1, collapsed, lines)
        })
      } else {
        keys.forEach((k, i) => {
          this.collectJsonLines(value[k], k, `${path}.${k}`, depth + 1, i === keys.length - 1, collapsed, lines)
        })
      }
      lines.push({ id: `${path}:c`, type: 'close', depth, bracket: isArray ? ']' : '}', hasComma: !isLast })
    },
    formatJsonPrimitive(value) {
      if (value === null) return 'null'
      if (typeof value === 'string') return JSON.stringify(value)
      return String(value)
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
        this.emitTabTitle()
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
      const savedQuery = this.normalizeObject(saved.query)
      const savedQueryEnabled = this.normalizeObject(saved.queryEnabled)
      const savedQueryForMerge = {}
      for (const key of Object.keys(savedQuery)) {
        if (row.source !== 'auto' || generated.queryRequired.has(key) || savedQueryEnabled[key] === true) {
          savedQueryForMerge[key] = savedQuery[key]
        }
      }
      const savedQueryKeys = new Set(Object.keys(savedQueryForMerge))
      const mergedQuery = { ...generated.query, ...savedQueryForMerge }
      const queryEnabled = {}
      const queryRequired = { ...generated.queryRequiredMap }
      for (const key of Object.keys(mergedQuery)) {
        queryEnabled[key] = generated.queryRequired.has(key) || savedQueryKeys.has(key)
        if (!Object.prototype.hasOwnProperty.call(queryRequired, key)) queryRequired[key] = false
      }
      const request = {
        method: saved.method || row.method,
        path: this.normalizePathVariables(saved.path || row.path),
        headers: { ...generated.headers, ...this.normalizeObject(saved.headers) },
        query: mergedQuery,
        queryEnabled,
        queryRequired,
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
      this.queryRows = this.objectToRows(raw.query, raw.queryEnabled, raw.queryRequired)
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
        const generatedQueryState = this.mergeGeneratedQueryParams(schemaGenerated, generated.query)
        const request = {
          method: current.method || this.testCase.method,
          path: this.normalizePathVariables(current.path || this.testCase.path),
          headers: this.normalizeObject(current.headers),
          query: generatedQueryState.query,
          queryEnabled: generatedQueryState.queryEnabled,
          queryRequired: generatedQueryState.queryRequired,
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
    openAIParamsDialog() {
      this.aiParamsLoading = true
      this.aiParamsDialogVisible = true
    },
    onAIParamsApply(payload) {
      const generated = payload?.parsed
      if (!generated || typeof generated !== 'object' || Array.isArray(generated)) {
        this.$message.warning('AI 未返回结构化参数，请查看原文')
        return
      }
      const aiHeaders = (generated.headers && typeof generated.headers === 'object' && !Array.isArray(generated.headers))
        ? generated.headers : {}
      const aiQuery = (generated.query && typeof generated.query === 'object' && !Array.isArray(generated.query))
        ? generated.query : {}
      const aiPath = (generated.path && typeof generated.path === 'object' && !Array.isArray(generated.path))
        ? generated.path : (generated.pathvar || generated.pathVars || {})
      const aiBody = Object.prototype.hasOwnProperty.call(generated, 'body') ? generated.body : null

      const currentQuery = this.rowsToValueMap(this.queryRows)
      const currentQueryEnabled = this.rowsToEnabledMap(this.queryRows)
      const currentQueryRequired = this.rowsToRequiredMap(this.queryRows)
      const currentHeaders = this.rowsToValueMap(this.headerRows)
      const currentPath = this.rowsToValueMap(this.pathRows)
      const currentVars = this.rowsToValueMap(this.variableRows)
      const current = this.normalizeRequest(this.request)

      const mergedHeaders = { ...currentHeaders }
      for (const [k, v] of Object.entries(aiHeaders)) {
        mergedHeaders[k] = typeof v === 'string' ? v : JSON.stringify(v)
      }

      const mergedQuery = { ...currentQuery }
      const mergedQueryEnabled = { ...currentQueryEnabled }
      const mergedQueryRequired = { ...currentQueryRequired }
      for (const [k, v] of Object.entries(aiQuery)) {
        mergedQuery[k] = typeof v === 'string' ? v : JSON.stringify(v)
        mergedQueryEnabled[k] = true
        if (!Object.prototype.hasOwnProperty.call(mergedQueryRequired, k)) mergedQueryRequired[k] = false
      }

      const mergedVariables = { ...currentVars, ...currentPath }
      for (const [k, v] of Object.entries(aiPath)) {
        mergedVariables[k] = typeof v === 'string' ? v : JSON.stringify(v)
      }

      let bodyDraft = current.body
      if (this.bodyText && this.bodyText.trim()) {
        try {
          bodyDraft = JSON.parse(this.bodyText)
        } catch (_) {
          bodyDraft = this.bodyText
        }
      }
      const nextBody = aiBody !== null && aiBody !== undefined ? aiBody : bodyDraft
      const request = {
        method: current.method,
        path: this.normalizePathVariables(current.path),
        headers: mergedHeaders,
        query: mergedQuery,
        queryEnabled: mergedQueryEnabled,
        queryRequired: mergedQueryRequired,
        variables: mergedVariables,
        body: nextBody
      }
      if (current.security !== undefined) {
        request.security = this.cloneSample(current.security)
      }
      if (request.body != null && !request.headers['Content-Type']) {
        request.headers['Content-Type'] = 'application/json'
      }
      this.applyRequest(request)
      this.$message.success('已应用 AI 生成的参数')
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
      rows.push({ enabled: true, key: '', value: '', required: false })
    },
    removeRow(rows, index) {
      rows.splice(index, 1)
    },
    rowsToValueMap(rows) {
      return rows.reduce((out, row) => {
        if (row.key) out[row.key] = row.value
        return out
      }, {})
    },
    rowsToEnabledMap(rows) {
      return rows.reduce((out, row) => {
        if (row.key) out[row.key] = row.enabled !== false
        return out
      }, {})
    },
    rowsToRequiredMap(rows) {
      return rows.reduce((out, row) => {
        if (row.key) out[row.key] = row.required === true
        return out
      }, {})
    },
    objectToRows(value, enabledMap, requiredMap) {
      return Object.entries(value || {}).map(([key, item]) => ({
        enabled: enabledMap ? (enabledMap[key] !== false) : true,
        key,
        value: String(item),
        required: requiredMap ? requiredMap[key] === true : false
      }))
    },
    rowsToObject(rows) {
      return rows.reduce((out, row) => {
        if (row.enabled && row.key) out[row.key] = row.value
        return out
      }, {})
    },
    mergeGeneratedQueryParams(schemaGenerated, generatedQuery) {
      const schemaValues = this.normalizeObject(schemaGenerated.query)
      const generatedValues = this.normalizeObject(generatedQuery)
      const currentValues = this.rowsToValueMap(this.queryRows)
      const currentEnabled = this.rowsToEnabledMap(this.queryRows)
      const currentRequired = this.rowsToRequiredMap(this.queryRows)
      const query = {}
      const queryEnabled = {}
      const queryRequired = { ...schemaGenerated.queryRequiredMap }
      const keys = new Set([
        ...Object.keys(schemaValues),
        ...Object.keys(generatedValues),
        ...Object.keys(currentValues)
      ])

      for (const key of keys) {
        const hasCurrentValue = Object.prototype.hasOwnProperty.call(currentValues, key)
        const hasCurrentEnabled = Object.prototype.hasOwnProperty.call(currentEnabled, key)
        const hasCurrentRequired = Object.prototype.hasOwnProperty.call(currentRequired, key)
        const hasGeneratedValue = Object.prototype.hasOwnProperty.call(generatedValues, key)
        const hasSchemaValue = Object.prototype.hasOwnProperty.call(schemaValues, key)
        const schemaRequired = schemaGenerated.queryRequired.has(key)
        const required = hasCurrentRequired ? currentRequired[key] === true : schemaRequired
        const enabled = hasCurrentEnabled
          ? currentEnabled[key]
          : schemaRequired || (!hasSchemaValue && hasGeneratedValue)

        queryRequired[key] = required
        queryEnabled[key] = enabled
        if (enabled && hasGeneratedValue) {
          query[key] = generatedValues[key]
        } else if (enabled && hasSchemaValue) {
          query[key] = schemaValues[key]
        } else if (hasCurrentValue) {
          query[key] = currentValues[key]
        } else if (hasSchemaValue) {
          query[key] = schemaValues[key]
        } else {
          query[key] = generatedValues[key]
        }
      }

      return { query, queryEnabled, queryRequired }
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
        queryRequiredMap: {},
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
          const required = param.required === true
          request.query[param.name] = required ? value : ''
          request.queryRequiredMap[param.name] = required
          if (required) request.queryRequired.add(param.name)
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
    parameterComment(row, location) {
      const name = row?.key
      if (!name) return '-'
      const parameters = Array.isArray(this.requestSchema?.parameters) ? this.requestSchema.parameters : []
      const param = parameters.find((item) => item?.name === name && item?.in === location)
      if (!param) return '-'
      const description = typeof param.description === 'string' ? param.description.trim() : ''
      if (description) return description
      return this.schemaFieldMeaning(param.schema)
    },
    schemaDocRows(schema) {
      if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return []
      const root = this.normalizeSchemaNode(schema)
      if (!Object.keys(root).length) return []
      const rows = []
      this.collectSchemaDocRows(root, '$', '$', null, 0, false, rows)
      if (rows.length <= 1) return rows
      return rows
        .filter((row) => row.path !== '$')
        .map((row) => ({ ...row, parentPath: row.parentPath === '$' ? null : row.parentPath }))
    },
    filterSchemaRows(rows, collapsed) {
      if (!Object.keys(collapsed).length) return rows
      const byPath = {}
      rows.forEach((r) => { byPath[r.path] = r })
      return rows.filter((row) => {
        let p = row.parentPath
        while (p !== null && p !== undefined) {
          if (collapsed[p]) return false
          p = byPath[p]?.parentPath ?? null
        }
        return true
      })
    },
    collectSchemaDocRows(schema, path, name, parentPath, depth, required, rows) {
      const node = this.normalizeSchemaNode(schema)
      const type = this.schemaType(node)

      let hasChildren = false
      if (type === 'object') {
        const props = this.normalizeObject(node.properties)
        if (Object.keys(props).length > 0 || (node.additionalProperties && typeof node.additionalProperties === 'object')) {
          hasChildren = true
        }
      } else if (type === 'array' && node.items && typeof node.items === 'object') {
        const itemsNode = this.normalizeSchemaNode(node.items)
        const itemsType = this.schemaType(itemsNode)
        if (itemsType === 'object' && Object.keys(this.normalizeObject(itemsNode.properties)).length > 0) {
          hasChildren = true
        }
      }

      rows.push({
        path,
        name,
        parentPath,
        depth,
        required,
        hasChildren,
        type: this.schemaTypeLabel(node),
        meaning: this.schemaFieldMeaning(node)
      })

      if (type === 'object') {
        const requiredFields = new Set(Array.isArray(node.required) ? node.required : [])
        for (const [key, property] of Object.entries(this.normalizeObject(node.properties))) {
          this.collectSchemaDocRows(
            property,
            path === '$' ? key : `${path}.${key}`,
            key,
            path,
            depth + 1,
            requiredFields.has(key),
            rows
          )
        }
        if (node.additionalProperties && typeof node.additionalProperties === 'object') {
          this.collectSchemaDocRows(node.additionalProperties, `${path}.{key}`, '{key}', path, depth + 1, false, rows)
        }
      } else if (type === 'array' && node.items && typeof node.items === 'object') {
        const itemsNode = this.normalizeSchemaNode(node.items)
        const itemsType = this.schemaType(itemsNode)
        if (itemsType === 'object') {
          const requiredFields = new Set(Array.isArray(itemsNode.required) ? itemsNode.required : [])
          for (const [key, property] of Object.entries(this.normalizeObject(itemsNode.properties))) {
            this.collectSchemaDocRows(
              property,
              `${path}[].${key}`,
              key,
              path,
              depth + 1,
              requiredFields.has(key),
              rows
            )
          }
          if (itemsNode.additionalProperties && typeof itemsNode.additionalProperties === 'object') {
            this.collectSchemaDocRows(itemsNode.additionalProperties, `${path}[].{key}`, '{key}', path, depth + 1, false, rows)
          }
        }
      }
    },
    normalizeSchemaNode(schema) {
      if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return {}
      let out = { ...schema }

      if (Array.isArray(schema.allOf) && schema.allOf.length > 0) {
        const composed = schema.allOf.reduce((merged, item) => {
          return this.mergeSchemaNodes(merged, this.normalizeSchemaNode(item))
        }, {})
        out = this.mergeSchemaNodes(composed, { ...schema, allOf: undefined })
      }

      const alternatives = Array.isArray(out.oneOf) && out.oneOf.length > 0
        ? out.oneOf
        : (Array.isArray(out.anyOf) && out.anyOf.length > 0 ? out.anyOf : null)
      if (alternatives && !out.properties && !out.items) {
        out = this.mergeSchemaNodes(this.normalizeSchemaNode(alternatives[0]), out)
      }

      return out
    },
    mergeSchemaNodes(base, override) {
      const merged = { ...base, ...override }
      if (base.properties || override.properties) {
        merged.properties = {
          ...this.normalizeObject(base.properties),
          ...this.normalizeObject(override.properties)
        }
      }
      const required = [
        ...(Array.isArray(base.required) ? base.required : []),
        ...(Array.isArray(override.required) ? override.required : [])
      ]
      if (required.length > 0) merged.required = Array.from(new Set(required))
      return merged
    },
    schemaFieldMeaning(schema) {
      const node = this.normalizeSchemaNode(schema)
      const description = typeof node.description === 'string' ? node.description.trim() : ''
      if (description) return description
      const title = typeof node.title === 'string' ? node.title.trim() : ''
      if (title) return title
      return ''
    },
    schemaTypeLabel(schema) {
      const node = this.normalizeSchemaNode(schema)
      const type = this.schemaType(node)
      const format = typeof node.format === 'string' ? node.format.trim() : ''
      if (!type && Array.isArray(node.enum) && node.enum.length > 0) return 'enum'
      if (type === 'array' && node.items && typeof node.items === 'object') {
        const itemsNode = this.normalizeSchemaNode(node.items)
        const itemsType = this.schemaType(itemsNode)
        if (itemsType) return `array[${itemsType}]`
      }
      return format ? `${type || 'value'}:${format}` : (type || 'value')
    },
    schemaType(schema) {
      if (!schema || typeof schema !== 'object') return ''
      if (Array.isArray(schema.type) && schema.type.length > 0) return schema.type.join('|')
      if (typeof schema.type === 'string' && schema.type) return schema.type
      if (schema.properties) return 'object'
      if (schema.items) return 'array'
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return 'enum'
      return ''
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
    emitTabTitle() {
      const name = (this.endpoint?.summary || '').trim() || this.testCase?.path || this.testCase?.name || '接口'
      const caseId = this.testCase?.id || this.caseId || this.$route.params.caseID
      if (caseId && name) this.$emit('tab-title-change', { caseId, name })
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
      const schemaGenerated = this.requestFromSchema(this.endpoint?.requestSchema)
      const queryRequired = {
        ...schemaGenerated.queryRequiredMap,
        ...this.normalizeObject(request.queryRequired)
      }
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
        queryRows: this.objectToRows(request.query, request.queryEnabled, queryRequired),
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
      const queryRequired = this.queryRows.reduce((out, row) => {
        if (row.key) out[row.key] = row.required === true
        return out
      }, {})
      return {
        method: this.request.method,
        path: this.pathForRun(this.request.path),
        headers: this.rowsToObject(this.headerRows),
        query: this.rowsToObject(this.queryRows),
        queryEnabled,
        queryRequired,
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

.debug-actions,
.request-line,
.body-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.body-view-toggle {
  display: flex;
  gap: 0;
  border: 1px solid var(--app-border-color);
  border-radius: 6px;
  overflow: hidden;
  margin-left: auto;
}

.body-view-toggle span {
  padding: 2px 10px;
  font-size: 12px;
  color: var(--app-secondary-text);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s, color 0.15s;
}

.body-view-toggle span:first-child {
  border-right: 1px solid var(--app-border-color);
}

.body-view-toggle span.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
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

.query-value-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}

.query-value-cell :deep(.el-input) {
  flex: 1 1 auto;
}

.required-star {
  flex: 0 0 auto;
  color: var(--el-color-danger);
  font-size: 16px;
  font-weight: 700;
  line-height: 1;
}

.param-comment {
  color: var(--app-secondary-text);
  line-height: 1.45;
  word-break: break-word;
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

.body-schema-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 8px minmax(260px, var(--body-schema-panel-width, 1fr));
  gap: 8px;
  align-items: stretch;
}

.body-schema-layout > .code-editor,
.body-schema-layout > .code-view {
  min-width: 0;
}

.body-schema-resizer {
  position: relative;
  width: 8px;
  min-height: 100%;
  border-radius: 999px;
  cursor: col-resize;
  touch-action: none;
}

.body-schema-resizer::before {
  content: '';
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 3px;
  width: 2px;
  border-radius: 999px;
  background: var(--app-border-color);
  transition: background 0.12s ease, width 0.12s ease, left 0.12s ease;
}

.body-schema-resizer:hover::before {
  left: 2px;
  width: 4px;
  background: var(--el-color-primary-light-5, #a0cfff);
}

.schema-panel {
  min-height: 260px;
  max-height: 520px;
  overflow: auto;
  padding: 10px 12px;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-card-bg);
}

.schema-panel-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.schema-panel-title {
  color: var(--el-text-color-primary);
  font-size: var(--app-font-size-small);
  font-weight: 700;
}

.schema-field-count {
  flex: 0 0 auto;
  color: var(--app-secondary-text);
  font-size: 12px;
}

.schema-field-table {
  border: 1px solid var(--app-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.schema-field-table-head,
.schema-field-row {
  display: grid;
  grid-template-columns: minmax(112px, 1.25fr) minmax(72px, 0.65fr) minmax(120px, 1.2fr);
  align-items: center;
  column-gap: 8px;
}

.schema-field-table-head {
  min-height: 32px;
  padding: 0 10px;
  background: #f8fafc;
  color: var(--app-secondary-text);
  font-size: 12px;
  font-weight: 600;
}

.schema-field-row {
  min-height: 34px;
  padding: 6px 10px;
  border-top: 1px solid var(--app-border-color);
}

.schema-field-name {
  display: flex;
  align-items: center;
  gap: 3px;
  min-width: 0;
}

.json-viewer {
  overflow: auto;
  padding: 10px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.7;
}

.json-line {
  display: flex;
  align-items: baseline;
  gap: 0;
  white-space: pre;
  padding-right: 10px;
}

.json-toggle,
.json-toggle-ph {
  flex: 0 0 14px;
  width: 14px;
  height: 14px;
  position: relative;
  top: 1px;
}

.json-toggle {
  cursor: pointer;
  border-radius: 2px;
  flex-shrink: 0;
}

.json-toggle:hover {
  background: var(--el-fill-color-light);
}

.json-toggle::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  border: 3.5px solid transparent;
  border-top: 4.5px solid #94a3b8;
  border-bottom: 0;
  transform: translate(-50%, -20%);
  transition: transform 0.12s;
}

.json-toggle.is-collapsed::before {
  transform: translate(-50%, -50%) rotate(-90deg);
}

.json-key {
  color: #1e40af;
}

.json-colon {
  color: var(--el-text-color-regular);
}

.json-bracket {
  color: var(--el-text-color-regular);
}

.json-comma {
  color: var(--el-text-color-regular);
}

.json-string { color: #16a34a; }
.json-number { color: #c2410c; }
.json-boolean { color: #7c3aed; }
.json-null { color: #94a3b8; }

.json-collapsed-preview {
  color: #94a3b8;
  font-style: italic;
}

.schema-toggle,
.schema-toggle-placeholder {
  flex: 0 0 14px;
  width: 14px;
  height: 14px;
}

.schema-toggle {
  position: relative;
  cursor: pointer;
  border-radius: 3px;
  transition: background 0.15s;
}

.schema-toggle:hover {
  background: var(--el-fill-color-light);
}

.schema-toggle::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  border: 4px solid transparent;
  border-top: 5px solid var(--app-secondary-text);
  border-bottom: 0;
  transform: translate(-50%, -30%);
  transition: transform 0.15s;
}

.schema-toggle.is-collapsed::before {
  transform: translate(-50%, -50%) rotate(-90deg);
}

.schema-field-name code {
  overflow: hidden;
  color: var(--el-text-color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-required {
  flex: 0 0 auto;
  color: var(--el-color-danger);
  font-size: 13px;
  font-weight: 700;
}

.schema-type-chip {
  justify-self: start;
  max-width: 100%;
  overflow: hidden;
  padding: 1px 7px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-field-meaning {
  overflow: hidden;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.response-detail-tabs--subtle :deep(.el-tabs__header) {
  margin-bottom: 8px;
}

.response-detail-tabs--subtle :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--app-border-color);
}

.response-detail-tabs--subtle :deep(.el-tabs__item) {
  height: 30px;
  padding: 0 10px;
  color: var(--app-secondary-text);
  font-size: 12px;
  line-height: 30px;
}

.response-detail-tabs--subtle :deep(.el-tabs__item.is-active) {
  color: var(--el-color-primary);
}

.response-detail-tabs--subtle :deep(.el-tabs__active-bar) {
  height: 2px;
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

  .body-schema-layout {
    grid-template-columns: 1fr;
  }

  .body-schema-resizer {
    display: none;
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
