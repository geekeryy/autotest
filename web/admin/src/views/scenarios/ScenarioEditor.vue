<template>
  <div class="scenario-editor">
    <div class="main-header">
      <div class="main-title">
        <span>{{ scenario.name }}</span>
        <el-tag v-if="scenario.description" type="info" size="small" class="scenario-desc-tag">
          {{ scenario.description }}
        </el-tag>
      </div>
      <div class="main-actions">
        <el-select
          v-model="runEnvId"
          placeholder="选择运行环境"
          class="run-env-select"
          filterable
        >
          <el-option v-for="env in environments" :key="env.id" :label="env.name" :value="env.id" />
        </el-select>
        <el-button type="success" :loading="running" :disabled="!runEnvId" @click="runScenario">
          运行场景
        </el-button>
      </div>
    </div>

    <div class="step-dialog-workspace inline-step-workspace">
      <aside class="step-dialog-sidebar" :style="stepSidebarStyle">
        <div class="step-sidebar-title">
          <span>已添加步骤</span>
          <button type="button" class="step-sidebar-add" @click="startNewStepDraft">
            <el-icon class="step-sidebar-add-icon"><Plus /></el-icon>
            <span>新增步骤</span>
          </button>
        </div>
        <div class="step-case-list">
          <div ref="stepSortableList" class="step-sortable-root">
            <scenario-step-tree-node
              v-for="block in stepTreeBlocks"
              :key="block.key"
              :node="block"
            />
          </div>
          <el-empty v-if="!panelSteps.length" description="暂无已添加步骤" :image-size="60" />
        </div>
      </aside>

      <div
        class="step-sidebar-resizer"
        role="separator"
        :aria-valuenow="stepSidebarWidthPx"
        aria-valuemin="180"
        aria-valuemax="560"
        aria-orientation="vertical"
        aria-label="调整步骤列表宽度"
        tabindex="0"
        @mousedown.prevent="startStepSidebarResize"
        @keydown.left.prevent="nudgeStepSidebar(-16)"
        @keydown.right.prevent="nudgeStepSidebar(16)"
      />

      <section class="step-dialog-main scenario-step-console">
        <el-tabs
          v-if="stepTabs.length"
          v-model="activeStepTabId"
          class="scenario-step-tabs"
          type="card"
          closable
          :before-leave="onStepTabBeforeLeave"
          @tab-change="onStepTabChange"
          @tab-remove="closeStepTab"
        >
          <el-tab-pane v-for="t in stepTabs" :key="t.id" :name="t.id">
            <template #label>
              <span class="scenario-step-tab-label" :title="t.label || ''">
                {{ t.label || (t.kind === 'draft' ? '新增步骤' : '步骤') }}
              </span>
            </template>
          </el-tab-pane>
        </el-tabs>

        <template v-if="isStepFormActive">
          <el-card class="request-card">
            <div class="scenario-step-title-row">
              <el-select
                v-model="stepForm.stepType"
                class="step-type-select"
                :disabled="Boolean(editingStep)"
                @change="onStepTypeChange"
              >
                <el-option v-for="item in stepTypes" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
              <template v-if="!editingStepName">
                <span
                  class="step-title-text"
                  :class="{ placeholder: !stepTitleDisplay }"
                  :title="stepTitleDisplay"
                  @dblclick="startEditStepName"
                >{{ stepTitleDisplay || '请配置步骤' }}</span>
                <span class="step-title-hint">双击编辑名称</span>
              </template>
              <el-input
                v-else
                ref="stepNameInputRef"
                v-model="stepForm.name"
                class="step-title-input"
                :placeholder="stepForm.stepType === 'api' ? '自定义步骤名称（留空则用接口名称）' : '自定义步骤名称'"
                @blur="finishEditStepName"
                @keydown.enter="finishEditStepName"
              />
            </div>

            <div
              v-show="stepForm.stepType === 'api' && !editingStep"
              class="scenario-api-pick-line"
            >
              <el-select
                v-model="stepForm.testCaseId"
                filterable
                clearable
                placeholder="搜索并选择接口"
                class="scenario-api-select-full"
                popper-class="scenario-case-select-dropdown"
                @change="onStepCaseChange"
                @clear="onStepApiClear"
              >
                <el-option
                  v-for="c in cases"
                  :key="c.id"
                  :label="`[${c.method}] ${c.path} — ${c.name}`"
                  :value="c.id"
                />
              </el-select>
            </div>

            <template v-if="stepForm.stepType === 'api' && stepForm.testCaseId">
              <div class="request-line">
                <el-select v-model="stepForm.requestMethod" class="method-select" @change="onRequestMethodChange">
                  <el-option v-for="m in httpMethods" :key="m" :label="m" :value="m" />
                </el-select>
                <el-input
                  v-model="stepForm.requestPath"
                  class="path-input"
                  placeholder="/api/users/{id}"
                  @change="() => syncPathParamRows()"
                />
                <div class="send-actions">
                  <el-button :loading="generatingStepParams" @click="generateStepParams">生成参数</el-button>
                  <el-button
                    :loading="aiStepParamsLoading"
                    :disabled="aiStepParamsLoading"
                    @click="openStepAIParamsDialog"
                  >AI 生成</el-button>
                  <el-tooltip
                    content="将当前请求参数保存到该场景步骤（数据库持久化），不会改写接口管理中的请求模板"
                    placement="top"
                  >
                    <el-button :loading="savingStep" @click="saveStepFromPanel">保存参数</el-button>
                  </el-tooltip>
                  <el-button
                    class="send-button"
                    type="primary"
                    :loading="runningStep"
                    :disabled="!runEnvId || !stepForm.testCaseId"
                    @click="runStepRequest"
                  >
                    发送
                  </el-button>
                </div>
              </div>
              <el-tabs v-model="stepForm.requestTab" class="request-tabs">
                  <el-tab-pane label="Params" name="params">
                    <div class="params-section">
                      <div class="params-section-title">路径参数（Path Params）</div>
                      <el-table :data="stepForm.pathParamRows" border class="kv-table">
                        <el-table-column width="70" label="启用">
                          <template #default="{ row }"><el-checkbox v-model="row.enabled" :disabled="row.authInherited" @change="markRowOverridden(row)" /></template>
                        </el-table-column>
                        <el-table-column label="参数名" min-width="180">
                          <template #default="{ row }"><el-input v-model="row.key" placeholder="路径参数名" /></template>
                        </el-table-column>
                        <el-table-column label="参数值" min-width="260">
                          <template #default="{ row }"><el-input v-model="row.value" placeholder="支持 {{变量名}}" /></template>
                        </el-table-column>
                        <el-table-column label="字段注释" min-width="220">
                          <template #default="{ row }">
                            <span class="param-comment">{{ parameterCommentForStep(row, 'path') }}</span>
                          </template>
                        </el-table-column>
                        <el-table-column width="90" label="操作">
                          <template #default="{ row, $index }">
                            <el-button link type="danger" @click="removeRow(stepForm.pathParamRows, $index)">删除</el-button>
                          </template>
                        </el-table-column>
                      </el-table>
                      <el-button class="add-row" @click="addRow(stepForm.pathParamRows)">新增一行</el-button>
                    </div>
                    <div class="params-section">
                      <div class="params-section-title">Query</div>
                      <el-table :data="stepForm.queryRows" border class="kv-table">
                        <el-table-column width="70" label="启用">
                          <template #default="{ row }"><el-checkbox v-model="row.enabled" /></template>
                        </el-table-column>
                        <el-table-column label="参数名" min-width="180">
                          <template #default="{ row }"><el-input v-model="row.key" placeholder="参数名" @input="markRowOverridden(row)" /></template>
                        </el-table-column>
                        <el-table-column label="参数值" min-width="260">
                          <template #default="{ row }">
                            <div class="query-value-cell">
                              <el-input v-model="row.value" placeholder="参数值 / {{变量}}" @input="markRowOverridden(row)" />
                              <span v-if="row.required" class="required-star" aria-label="必填">*</span>
                              <el-tag v-if="row.authInherited && !row.authOverridden" size="small" type="info">环境认证</el-tag>
                            </div>
                          </template>
                        </el-table-column>
                        <el-table-column label="字段注释" min-width="220">
                          <template #default="{ row }">
                            <span class="param-comment">{{ parameterCommentForStep(row, 'query') }}</span>
                          </template>
                        </el-table-column>
                        <el-table-column width="90" label="操作">
                          <template #default="{ $index }">
                            <el-button link type="danger" :disabled="row.authInherited && !row.authOverridden" @click="removeRow(stepForm.queryRows, $index)">删除</el-button>
                          </template>
                        </el-table-column>
                      </el-table>
                      <el-button class="add-row" @click="addRow(stepForm.queryRows)">新增一行</el-button>
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="Headers" name="headers">
                    <el-table :data="stepForm.headerRows" border class="kv-table">
                      <el-table-column width="70" label="启用">
                        <template #default="{ row }"><el-checkbox v-model="row.enabled" :disabled="row.authInherited" @change="markRowOverridden(row)" /></template>
                      </el-table-column>
                      <el-table-column label="Header" min-width="180">
                        <template #default="{ row }"><el-input v-model="row.key" placeholder="Header" @input="markRowOverridden(row)" /></template>
                      </el-table-column>
                      <el-table-column label="Value" min-width="260">
                        <template #default="{ row }">
                          <div class="auth-value-cell">
                            <el-input v-model="row.value" placeholder="支持 {{变量名}}" @input="markRowOverridden(row)" />
                            <el-tag v-if="row.authInherited && !row.authOverridden" size="small" type="info">环境认证</el-tag>
                          </div>
                        </template>
                      </el-table-column>
                      <el-table-column label="字段注释" min-width="220">
                        <template #default="{ row }">
                          <span class="param-comment">{{ parameterCommentForStep(row, 'header') }}</span>
                        </template>
                      </el-table-column>
                      <el-table-column width="90" label="操作">
                        <template #default="{ row, $index }">
                          <el-button link type="danger" :disabled="row.authInherited && !row.authOverridden" @click="removeRow(stepForm.headerRows, $index)">删除</el-button>
                        </template>
                      </el-table-column>
                    </el-table>
                    <el-button class="add-row" @click="addRow(stepForm.headerRows)">新增一行</el-button>
                  </el-tab-pane>
                  <el-tab-pane label="Body" name="body">
                    <div class="body-toolbar">
                      <span>JSON Body</span>
                      <div class="body-view-toggle">
                        <span
                          :class="{ active: stepRequestBodyViewMode === 'edit' }"
                          @click="stepRequestBodyViewMode = 'edit'"
                        >编辑</span>
                        <span
                          :class="{ active: stepRequestBodyViewMode === 'preview' }"
                          @click="stepRequestBodyViewMode = 'preview'"
                        >预览</span>
                      </div>
                    </div>
                    <div class="body-schema-layout" :style="scenarioStepBodySchemaLayoutStyle">
                      <div
                        v-if="stepRequestBodyViewMode === 'preview' && scenarioStepBodyEditorJsonLines !== null"
                        class="json-viewer code-view scenario-body-code-pane"
                      >
                        <div
                          v-for="line in scenarioStepBodyEditorJsonLines"
                          :key="line.id"
                          class="json-line"
                          :style="{ paddingLeft: `${line.depth * 16}px` }"
                        >
                          <span
                            v-if="line.type === 'open'"
                            class="json-toggle"
                            @click="toggleSchemaCollapse(stepRequestBodyJsonCollapsed, line.path)"
                          />
                          <span
                            v-else-if="line.type === 'collapsed'"
                            class="json-toggle is-collapsed"
                            @click="toggleSchemaCollapse(stepRequestBodyJsonCollapsed, line.path)"
                          />
                          <span v-else class="json-toggle-ph" />
                          <span
                            v-if="line.key !== null && line.key !== undefined"
                            class="json-key"
                          >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                      <pre
                        v-else-if="stepRequestBodyViewMode === 'preview'"
                        class="code-view scenario-body-code-pane"
                      >{{ stepForm.bodyText }}</pre>
                      <JsonBodyEditor
                        v-show="stepRequestBodyViewMode === 'edit'"
                        v-model="stepForm.bodyText"
                        class="scenario-body-code-pane"
                        @blur="formatStepBody"
                      />
                      <div
                        class="body-schema-resizer"
                        title="拖拽调整注释区宽度"
                        @pointerdown="startStepBodySchemaResize"
                      />
                      <div class="schema-panel">
                        <div class="schema-panel-title-row">
                          <div class="schema-panel-title">请求 Body</div>
                          <span v-if="scenarioStepRequestBodyAllRows.length" class="schema-field-count">{{ scenarioStepRequestBodyAllRows.length }} 字段</span>
                        </div>
                        <div v-if="scenarioStepRequestBodyAllRows.length" class="schema-field-table">
                          <div class="schema-field-table-head">
                            <span>字段</span>
                            <span>类型</span>
                            <span>限制</span>
                            <span>说明</span>
                          </div>
                          <div v-for="row in scenarioStepRequestBodyDocRows" :key="row.path" class="schema-field-row">
                            <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                              <span
                                v-if="row.hasChildren"
                                class="schema-toggle"
                                :class="{ 'is-collapsed': stepRequestBodyCollapsed[row.path] }"
                                @click="toggleSchemaCollapse(stepRequestBodyCollapsed, row.path)"
                              />
                              <span v-else class="schema-toggle-placeholder" />
                              <code :title="row.path">{{ row.name }}</code>
                              <span v-if="row.required" class="schema-required">*</span>
                            </div>
                            <span class="schema-type-chip">{{ row.type }}</span>
                            <span class="schema-field-limits" :title="row.constraints">{{ row.constraints || '-' }}</span>
                            <span class="schema-field-meaning" :title="row.meaning">{{ row.meaning || '-' }}</span>
                          </div>
                        </div>
                        <el-empty v-else description="暂无字段说明" :image-size="42" />
                      </div>
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="断言" name="assertions">
                    <div class="step-assertion-tip">
                      此处添加的断言将在场景运行时追加到接口本身的断言之后执行。
                    </div>
                    <AssertionEditor v-model="stepForm.stepAssertions" />
                  </el-tab-pane>
                </el-tabs>
                <div class="inline-sql-hint scenario-request-hint">
                  路径参数（Path Params）的值会直接嵌入路径；Query、Headers 和 Body 作为请求覆盖保存。
                  后续步骤可通过 <code v-pre>{{$steps[N].xxx}}</code> 引用本步骤输出，N 为左侧 <strong>#序号</strong>。
                  支持运行时模拟数据标签 <code v-pre>{{$mock.uuid}}</code>、<code v-pre>{{$mock.now}}</code>、<code v-pre>{{$mock.email}}</code>、<code v-pre>{{$mock.int(1,100)}}</code>、<code v-pre>{{$mock.pick(a,b,c)}}</code> 等，每次请求实时生成新值。
                </div>
            </template>

            <template v-else-if="stepForm.stepType === 'database'">
              <el-form :model="stepForm" class="scenario-step-extra-form" label-width="96px" size="small">
                <el-form-item label="数据源" required>
                  <el-select v-model="stepForm.dbDataSourceId" filterable placeholder="选择项目下的业务数据源" style="width:100%">
                    <el-option v-for="ds in dataSources" :key="ds.id" :label="ds.name" :value="ds.id" />
                  </el-select>
                  <div class="extraction-hint">业务数据源归属项目，与环境无关；SQL 支持 $1、$2 参数，参数值可引用 <code v-pre>{{变量名}}</code>。</div>
                </el-form-item>
                <el-form-item label="SQL" required>
                  <SqlEditor
                    v-model="stepForm.dbSQL"
                    :schema-tables="schemaTables"
                    :min-rows="6"
                    :max-rows="16"
                    placeholder="select id, token from users where email = $1"
                  />
                </el-form-item>
                <el-form-item label="入参 JSON">
                  <el-input
                    v-model="stepForm.dbInputParamsText"
                    class="body-editor"
                    type="textarea"
                    :autosize="{ minRows: 3, maxRows: 8 }"
                    placeholder='[{"name":"email","value":"{{email}}","required":true}]'
                  />
                </el-form-item>
                <el-form-item label="超时(ms)">
                  <el-input-number v-model="stepForm.dbTimeoutMillis" :min="100" :step="500" />
                </el-form-item>
                <div class="step-ref-hint">
                  保存后可通过
                  <code>{{ stepsRefSnippet(stepForm.stepSeq, '.firstRow.字段名') }}</code>
                  或 <code>{{ stepsRefSnippet(stepForm.stepSeq, '.rows[0].字段名') }}</code> 在后续步骤中引用本步骤输出。
                </div>
                <div class="scenario-save-row">
                  <el-button type="primary" class="send-button" :loading="savingStep" @click="saveStepFromPanel">保存步骤</el-button>
                </div>
              </el-form>
            </template>

            <template v-else-if="stepForm.stepType === 'script'">
              <el-form :model="stepForm" class="scenario-step-extra-form" label-width="96px" size="small">
                <el-form-item label="脚本" required>
                  <div class="scenario-script-field">
                    <div class="scenario-script-toolbar">
                      <ScriptLibraryPicker variant="scenario" @append="appendScenarioScriptTemplate" />
                      <el-button
                        size="small"
                        type="primary"
                        plain
                        :loading="aiScenarioScriptLoading"
                        :disabled="aiScenarioScriptLoading"
                        @click="openScenarioScriptAI"
                      >AI 生成</el-button>
                      <span class="scenario-script-toolbar-hint">从脚本库插入模板或调用 AI 生成；追加到编辑区后可改占位参数</span>
                    </div>
                    <el-input
                      v-model="stepForm.scriptSource"
                      class="body-editor"
                      type="textarea"
                      :autosize="{ minRows: 10, maxRows: 24 }"
                      placeholder="// 多行 JavaScript，支持 pm.variables / pm.environment / pm.test / console&#10;pm.variables.set('k', 'v');&#10;console.log(JSON.stringify({ ok: true }));"
                    />
                  </div>
                </el-form-item>
                <el-form-item label="超时(ms)">
                  <el-input-number v-model="stepForm.scriptTimeoutMillis" :min="100" :step="1000" />
                </el-form-item>
                <div class="step-ref-hint">
                  使用环境变量与 <code>&#123;&#123;变量名&#125;&#125;</code>、<code>&#123;&#123;$steps[N].path&#125;&#125;</code>
                  占位；<code>pm.variables.set</code> 会写入场景变量供后续步骤使用。console 输出写入
                  <code>{{ stepsRefSnippet(stepForm.stepSeq, '.stdout') }}</code> /
                  <code>{{ stepsRefSnippet(stepForm.stepSeq, '.stderr') }}</code>。
                </div>
                <div class="scenario-save-row">
                  <el-button type="primary" class="send-button" :loading="savingStep" @click="saveStepFromPanel">保存步骤</el-button>
                </div>
              </el-form>
            </template>

            <template v-else-if="stepForm.stepType === 'for'">
              <el-form :model="stepForm" class="scenario-step-extra-form" label-width="112px" size="small">
                <el-form-item label="循环模式">
                  <el-radio-group v-model="stepForm.forMode">
                    <el-radio-button value="count">按次数</el-radio-button>
                    <el-radio-button value="array">遍历数组</el-radio-button>
                  </el-radio-group>
                </el-form-item>
                <el-form-item v-if="stepForm.forMode === 'count'" label="循环次数" required>
                  <el-input
                    v-model="stepForm.forCountExpression"
                    placeholder="3 或 {{$steps[1].body.total}}"
                  />
                </el-form-item>
                <el-form-item v-else label="数组表达式" required>
                  <el-input
                    v-model="stepForm.forItemsExpression"
                    class="body-editor"
                    type="textarea"
                    :autosize="{ minRows: 3, maxRows: 8 }"
                    placeholder='["a","b"] 或 {{$steps[1].body.items}}'
                  />
                </el-form-item>
                <el-form-item label="循环变量">
                  <div class="control-inline-fields">
                    <el-input v-model="stepForm.forItemVar" placeholder="item" />
                    <el-input v-model="stepForm.forIndexVar" placeholder="index" />
                  </div>
                  <div class="extraction-hint">循环体内可使用 <code v-pre>{{item}}</code>、<code v-pre>{{index}}</code>；对象项会额外展开为 <code v-pre>{{item.id}}</code>。</div>
                </el-form-item>
                <el-form-item label="最大迭代">
                  <el-input-number v-model="stepForm.forMaxIterations" :min="1" :max="1000" :step="10" />
                </el-form-item>
                <div class="step-ref-hint">
                  保存 For 步骤后，在左侧步骤列表的循环体框底部点击「+」添加要执行的步骤；循环次数硬限制最高 1000 次。
                </div>
                <div class="scenario-save-row">
                  <el-button type="primary" class="send-button" :loading="savingStep" @click="saveStepFromPanel">保存步骤</el-button>
                </div>
              </el-form>
            </template>

            <template v-else-if="stepForm.stepType === 'condition'">
              <el-form :model="stepForm" class="scenario-step-extra-form" label-width="112px" size="small">
                <div
                  v-for="(br, cidx) in stepForm.conditionBranches"
                  :key="'cbranch-' + cidx"
                  class="condition-branch-editor"
                >
                  <div class="condition-branch-editor-head">
                    <span class="condition-branch-editor-title">分支 {{ cidx + 1 }}</span>
                    <el-button
                      v-if="stepForm.conditionBranches.length > 1"
                      link
                      type="danger"
                      size="small"
                      @click="removeConditionBranch(cidx)"
                    >
                      删除分支
                    </el-button>
                  </div>
                  <el-form-item label="左值" required>
                    <el-input
                      v-model="br.left"
                      placeholder="{{$steps[1].status}} 或 {{token}}"
                    />
                  </el-form-item>
                  <el-form-item label="操作符">
                    <el-select v-model="br.operator" style="width:100%">
                      <el-option
                        v-for="op in conditionOperators"
                        :key="op.value"
                        :label="op.label"
                        :value="op.value"
                      />
                    </el-select>
                  </el-form-item>
                  <el-form-item
                    v-if="!['exists', 'not_exists', 'truthy', 'falsy'].includes(br.operator)"
                    label="右值"
                  >
                    <el-input v-model="br.right" placeholder="200、success 或 {{expectedStatus}}" />
                  </el-form-item>
                </div>
                <el-form-item label-width="0">
                  <el-button size="small" @click="addConditionBranch">添加分支</el-button>
                </el-form-item>
                <div class="step-ref-hint">
                  自上而下依次判断各分支条件，首个成立的分支会被执行；若均不成立则走「否则」框内步骤。表达式会先渲染环境变量和
                  <code v-pre>{{$steps[N].xxx}}</code>；保存后在左侧各分支框底部点击「+」新增步骤，未进入的路径会在运行结果中标记为跳过。
                </div>
                <div class="scenario-save-row">
                  <el-button type="primary" class="send-button" :loading="savingStep" @click="saveStepFromPanel">保存步骤</el-button>
                </div>
              </el-form>
            </template>
          </el-card>

          <el-card
            v-if="stepRunOutput && stepForm.stepType === 'api'"
            class="result-card scenario-step-result-card"
          >
            <div v-if="stepRunOutput" class="result-title scenario-step-result-head">
              <div>
                <h3>运行结果</h3>
                <p>Run ID: {{ stepDebugRunRecord?.id || '-' }}</p>
              </div>
              <div class="result-metrics">
                <span class="metric-chip" :class="stepDebugMetricClass(stepDebugRunRecord?.status)">
                  <el-icon>
                    <component :is="stepDebugStatusIconName(stepDebugRunRecord?.status)" />
                  </el-icon>
                  <span>运行 {{ stepDebugStatusLabel(stepDebugRunRecord?.status) }}</span>
                </span>
                <span class="metric-chip" :class="stepDebugMetricClass(stepRunResult?.status)">
                  <el-icon>
                    <component :is="stepDebugStatusIconName(stepRunResult?.status)" />
                  </el-icon>
                  <span>结果 {{ stepDebugStatusLabel(stepRunResult?.status) }}</span>
                </span>
                <span class="metric-chip" :class="durationChipClass(stepRunResult?.durationMillis)">
                  <el-icon><Timer /></el-icon>
                  <span>{{ formatDuration(stepRunResult?.durationMillis) }}</span>
                </span>
                <div class="response-overview-tags" aria-label="响应概览">
                  <el-tag
                    :type="stepDebugResponseStatusTagType(stepDebugResponseSnapshot?.statusCode)"
                    effect="plain"
                    size="small"
                  >{{ stepDebugResponseSnapshot?.statusCode ?? '-' }}</el-tag>
                  <el-tag
                    v-if="stepRunResult?.error"
                    effect="plain"
                    type="danger"
                    size="small"
                    class="response-msg-tag"
                  >
                    <span class="response-msg-tag-text">{{ stepRunResult.error }}</span>
                  </el-tag>
                </div>
              </div>
            </div>
            <el-tabs v-model="stepRunResultTab" class="step-debug-result-tabs scenario-step-result-tabs">
              <el-tab-pane label="请求" name="request">
                <div class="section-grid">
                  <div class="section-block full">
                    <h3>URL</h3>
                    <div class="url-line">
                      <el-tag>{{ stepDebugRequestSnapshot.method || '-' }}</el-tag>
                      <span>{{ stepDebugRequestSnapshot.url || '-' }}</span>
                    </div>
                  </div>
                  <div class="section-block">
                    <h3>Params</h3>
                    <el-table :data="stepDebugRequestQueryRows" border>
                      <el-table-column prop="key" label="名称" />
                      <el-table-column prop="value" label="值" />
                      <el-table-column label="字段注释" min-width="220">
                        <template #default="{ row }">
                          <span class="param-comment">{{ parameterCommentForStep(row, 'query') }}</span>
                        </template>
                      </el-table-column>
                    </el-table>
                  </div>
                  <div class="section-block">
                    <h3>Headers</h3>
                    <el-table :data="headersToRows(stepDebugRequestSnapshot.headers)" border>
                      <el-table-column prop="key" label="名称" />
                      <el-table-column prop="value" label="值" />
                      <el-table-column label="字段注释" min-width="220">
                        <template #default="{ row }">
                          <span class="param-comment">{{ parameterCommentForStep(row, 'header') }}</span>
                        </template>
                      </el-table-column>
                    </el-table>
                  </div>
                  <div class="section-block full">
                    <h3>Body</h3>
                    <div class="body-schema-layout" :style="scenarioStepBodySchemaLayoutStyle">
                      <div
                        v-if="stepDebugRequestSnapshotBodyJsonLines !== null"
                        class="json-viewer code-view scenario-body-code-pane"
                      >
                        <div
                          v-for="line in stepDebugRequestSnapshotBodyJsonLines"
                          :key="line.id"
                          class="json-line"
                          :style="{ paddingLeft: `${line.depth * 16}px` }"
                        >
                          <span
                            v-if="line.type === 'open'"
                            class="json-toggle"
                            @click="toggleSchemaCollapse(stepDebugRequestSnapshotJsonCollapsed, line.path)"
                          />
                          <span
                            v-else-if="line.type === 'collapsed'"
                            class="json-toggle is-collapsed"
                            @click="toggleSchemaCollapse(stepDebugRequestSnapshotJsonCollapsed, line.path)"
                          />
                          <span v-else class="json-toggle-ph" />
                          <span
                            v-if="line.key !== null && line.key !== undefined"
                            class="json-key"
                          >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                      <pre
                        v-else
                        class="code-view scenario-body-code-pane scenario-body-code-pane--muted"
                      >{{ formatSnapshotBody(stepDebugRequestSnapshot.body) }}</pre>
                      <div class="schema-panel-placeholder" aria-hidden="true" />
                      <div class="schema-panel">
                        <div class="schema-panel-title-row">
                          <div class="schema-panel-title">请求 Body</div>
                          <span v-if="scenarioStepRequestBodyAllRows.length" class="schema-field-count">{{ scenarioStepRequestBodyAllRows.length }} 字段</span>
                        </div>
                        <div v-if="scenarioStepRequestBodyAllRows.length" class="schema-field-table">
                          <div class="schema-field-table-head">
                            <span>字段</span>
                            <span>类型</span>
                            <span>限制</span>
                            <span>说明</span>
                          </div>
                          <div v-for="row in scenarioStepRequestBodyDocRows" :key="'reqsnap-' + row.path" class="schema-field-row">
                            <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                              <span
                                v-if="row.hasChildren"
                                class="schema-toggle"
                                :class="{ 'is-collapsed': stepRequestBodyCollapsed[row.path] }"
                                @click="toggleSchemaCollapse(stepRequestBodyCollapsed, row.path)"
                              />
                              <span v-else class="schema-toggle-placeholder" />
                              <code :title="row.path">{{ row.name }}</code>
                              <span v-if="row.required" class="schema-required">*</span>
                            </div>
                            <span class="schema-type-chip">{{ row.type }}</span>
                            <span class="schema-field-limits" :title="row.constraints">{{ row.constraints || '-' }}</span>
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
                <el-tabs v-model="stepRunResponseDetailTab" class="response-detail-tabs response-detail-tabs--subtle">
                  <el-tab-pane label="Body" name="body">
                    <div class="body-schema-layout" :style="scenarioStepBodySchemaLayoutStyle">
                      <div
                        v-if="stepDebugResponseBodyJsonLines !== null"
                        class="json-viewer code-view scenario-body-code-pane"
                      >
                        <div
                          v-for="line in stepDebugResponseBodyJsonLines"
                          :key="line.id"
                          class="json-line"
                          :style="{ paddingLeft: `${line.depth * 16}px` }"
                        >
                          <span
                            v-if="line.type === 'open'"
                            class="json-toggle"
                            @click="toggleSchemaCollapse(stepDebugResponseJsonCollapsed, line.path)"
                          />
                          <span
                            v-else-if="line.type === 'collapsed'"
                            class="json-toggle is-collapsed"
                            @click="toggleSchemaCollapse(stepDebugResponseJsonCollapsed, line.path)"
                          />
                          <span v-else class="json-toggle-ph" />
                          <span
                            v-if="line.key !== null && line.key !== undefined"
                            class="json-key"
                          >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                      <pre
                        v-else
                        class="code-view scenario-body-code-pane scenario-body-code-pane--muted"
                      >{{ formatSnapshotBody(stepDebugResponseSnapshot.body) }}</pre>
                      <div class="schema-panel-placeholder" aria-hidden="true" />
                      <div class="schema-panel">
                        <div class="schema-panel-title-row">
                          <div class="schema-panel-title">响应 Body</div>
                          <span v-if="stepDebugResponseBodyAllRows.length" class="schema-field-count">{{ stepDebugResponseBodyAllRows.length }} 字段</span>
                        </div>
                        <div v-if="stepDebugResponseBodyAllRows.length" class="schema-field-table">
                          <div class="schema-field-table-head">
                            <span>字段</span>
                            <span>类型</span>
                            <span>限制</span>
                            <span>说明</span>
                          </div>
                          <div v-for="row in stepDebugResponseBodyDocRows" :key="'ressnap-' + row.path" class="schema-field-row">
                            <div class="schema-field-name" :style="{ paddingLeft: `${row.depth * 12}px` }">
                              <span
                                v-if="row.hasChildren"
                                class="schema-toggle"
                                :class="{ 'is-collapsed': stepDebugResponseBodyCollapsed[row.path] }"
                                @click="toggleSchemaCollapse(stepDebugResponseBodyCollapsed, row.path)"
                              />
                              <span v-else class="schema-toggle-placeholder" />
                              <code :title="row.path">{{ row.name }}</code>
                            </div>
                            <span class="schema-type-chip">{{ row.type }}</span>
                            <span class="schema-field-limits" :title="row.constraints">{{ row.constraints || '-' }}</span>
                            <span class="schema-field-meaning" :title="row.meaning">{{ row.meaning || '-' }}</span>
                          </div>
                        </div>
                        <el-empty v-else description="暂无字段说明" :image-size="42" />
                      </div>
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="响应头" name="headers">
                    <el-table :data="headersToRows(stepRunHttpResponse ? stepRunHttpResponse.headers : stepDebugResponseSnapshot.headers)" border>
                      <el-table-column prop="key" label="名称" />
                      <el-table-column prop="value" label="值" />
                    </el-table>
                  </el-tab-pane>
                  <el-tab-pane label="实际请求" name="curl">
                    <pre class="code-view code-view--curl scenario-step-curl">{{ stepDebugCurlCommand }}</pre>
                  </el-tab-pane>
                </el-tabs>
              </el-tab-pane>

              <el-tab-pane label="断言" name="assertions">
                <el-table :data="stepDebugAssertions" border>
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
                      <code v-else class="assertion-path">{{ row.path || row.headerName || '' }}</code>
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
                <el-empty v-if="!stepDebugAssertions.length" description="暂无断言结果" :image-size="48" />
              </el-tab-pane>
            </el-tabs>
          </el-card>

          <div v-else-if="stepRunOutput && stepForm.stepType !== 'api'" class="step-debug-result step-debug-result--legacy">
            <div class="result-header">
              <span>发送结果：</span>
              <el-tag :type="stepRunOutput.run?.status === 'passed' ? 'success' : 'danger'" size="small">
                {{ stepRunOutput.run?.status || '-' }}
              </el-tag>
              <span v-if="stepRunResult?.durationMillis" class="duration">{{ stepRunResult.durationMillis }}ms</span>
            </div>
            <div v-if="stepRunResult?.error" class="error-text">{{ stepRunResult.error }}</div>
            <pre class="snapshot-pre">{{ formatSnapshot(stepRunResult?.responseSnapshot || stepRunOutput) }}</pre>
          </div>
        </template>
        <el-empty v-else description="请点击左侧步骤或新增步骤" />
      </section>
    </div>

    <div
      v-if="lastRunOutput"
      class="run-result-area"
      :style="{ '--run-result-panel-height': `${runResultPanelHeightPx}px` }"
    >
      <div
        class="run-result-resizer"
        role="separator"
        aria-orientation="horizontal"
        tabindex="0"
        title="拖动调整运行结果面板高度"
        @mousedown="startRunResultPanelResize"
        @keydown.up.prevent="nudgeRunResultPanelHeight(24)"
        @keydown.down.prevent="nudgeRunResultPanelHeight(-24)"
      />
      <el-collapse v-model="scenarioRunResultActiveNames" class="scenario-run-outer-collapse">
        <el-collapse-item name="panel">
          <template #title>
            <div class="run-result-collapse-title">
              <span>运行结果：{{ lastRunOutput.run.name }}</span>
              <el-tag :type="lastRunOutput.run.status === 'passed' ? 'success' : 'danger'" size="small">
                {{ lastRunOutput.run.status === 'passed' ? '通过' : '失败' }}
              </el-tag>
            </div>
          </template>
          <div class="run-result-collapse-body" :style="runResultPanelBodyStyle">
            <el-collapse>
              <el-collapse-item v-for="(sr, si) in lastRunOutput.stepResults" :key="si" :name="si">
                <template #title>
                  <template v-for="m in [stepRunOutcomeMeta(sr)]" :key="m.key">
                    <div class="step-run-result-title">
                      <el-icon class="step-run-icon" :class="'step-run-icon--' + m.key">
                        <component :is="m.Icon" />
                      </el-icon>
                      <span class="step-run-result-title__seq"
                        >#{{ sr.step.stepSeq != null && sr.step.stepSeq !== '' ? sr.step.stepSeq : '?' }}</span
                      >
                      <span class="step-run-result-title__name">{{ stepDisplayName(sr.step) }}</span>
                      <span v-if="sr.result?.durationMillis != null" class="step-run-result-title__duration">
                        {{ formatDuration(sr.result.durationMillis) }}
                      </span>
                    </div>
                  </template>
                </template>
                <div class="step-result-detail">
                  <el-tabs
                    :model-value="runResultDetailTabByStep[si] ?? 'detail'"
                    class="step-result-info-tabs"
                    type="border-card"
                    @update:model-value="(v) => setRunResultDetailTab(si, v)"
                  >
                    <el-tab-pane label="详情" name="detail">
                      <div v-if="sr.result?.error || sr.stepErrors?.length" class="step-result-error-strip">
                        <div v-if="sr.result?.error" class="error-text">{{ sr.result.error }}</div>
                        <div v-if="sr.stepErrors?.length" class="error-text">
                          步骤错误：{{ sr.stepErrors.join('; ') }}
                        </div>
                      </div>
                      <div class="step-result-detail-grid">
                        <section class="step-result-snapshot-panel">
                          <div class="step-result-panel-head">
                            <span>请求</span>
                            <el-tag v-if="runResultRequestMethod(sr)" size="small" effect="plain">
                              {{ runResultRequestMethod(sr) }}
                            </el-tag>
                          </div>
                          <template v-for="lines in [runResultJsonLines(si, 'request', sr.result?.requestSnapshot)]" :key="'req-lines-' + si">
                            <div v-if="lines?.length" class="json-viewer snapshot-pre run-result-json-viewer">
                              <div
                                v-for="line in lines"
                                :key="'req-' + si + '-' + line.id"
                                class="json-line"
                                :style="{ paddingLeft: `${line.depth * 16}px` }"
                              >
                                <span
                                  v-if="line.type === 'open'"
                                  class="json-toggle"
                                  @click="toggleRunResultJsonCollapse(si, 'request', line.path)"
                                />
                                <span
                                  v-else-if="line.type === 'collapsed'"
                                  class="json-toggle is-collapsed"
                                  @click="toggleRunResultJsonCollapse(si, 'request', line.path)"
                                />
                                <span v-else class="json-toggle-ph" />
                                <span
                                  v-if="line.key !== null && line.key !== undefined"
                                  class="json-key"
                                >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                            <el-empty v-else description="暂无请求信息" :image-size="42" />
                          </template>
                        </section>

                        <section class="step-result-snapshot-panel">
                          <div class="step-result-panel-head">
                            <span>响应</span>
                            <el-tag
                              v-if="getStepHttpResponse(sr)"
                              :type="responseSnapshotStatusType(sr.result?.responseSnapshot)"
                              size="small"
                              effect="plain"
                            >
                              {{ getStepHttpResponse(sr).statusCode }}
                            </el-tag>
                          </div>
                          <template v-for="lines in [runResultJsonLines(si, 'response', sr.result?.responseSnapshot)]" :key="'res-lines-' + si">
                            <div v-if="lines?.length" class="json-viewer snapshot-pre run-result-json-viewer">
                              <div
                                v-for="line in lines"
                                :key="'res-' + si + '-' + line.id"
                                class="json-line"
                                :style="{ paddingLeft: `${line.depth * 16}px` }"
                              >
                                <span
                                  v-if="line.type === 'open'"
                                  class="json-toggle"
                                  @click="toggleRunResultJsonCollapse(si, 'response', line.path)"
                                />
                                <span
                                  v-else-if="line.type === 'collapsed'"
                                  class="json-toggle is-collapsed"
                                  @click="toggleRunResultJsonCollapse(si, 'response', line.path)"
                                />
                                <span v-else class="json-toggle-ph" />
                                <span
                                  v-if="line.key !== null && line.key !== undefined"
                                  class="json-key"
                                >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                            <el-empty v-else description="暂无响应信息" :image-size="42" />
                          </template>
                        </section>

                      </div>
                    </el-tab-pane>

                    <el-tab-pane label="断言" name="assertions">
                      <el-table
                        v-if="sr.result?.assertions?.length"
                        :data="sr.result.assertions"
                        border
                        size="small"
                        class="assertion-result-table"
                      >
                        <el-table-column label="类型" width="100">
                          <template #default="{ row }">
                            <el-tag size="small" effect="plain">{{ assertionTypeLabel(row.type) }}</el-tag>
                          </template>
                        </el-table-column>
                        <el-table-column label="名称 / 路径" min-width="140">
                          <template #default="{ row }">
                            <span v-if="row.name">{{ row.name }}</span>
                            <code v-else class="assertion-path">{{ row.path || row.headerName || '' }}</code>
                          </template>
                        </el-table-column>
                        <el-table-column label="状态" width="80">
                          <template #default="{ row }">
                            <el-tag :type="row.passed ? 'success' : 'danger'" size="small">
                              {{ row.passed ? '通过' : '失败' }}
                            </el-tag>
                          </template>
                        </el-table-column>
                        <el-table-column prop="message" label="详情" min-width="180" />
                      </el-table>
                      <el-empty v-else description="暂无断言结果" :image-size="42" />
                    </el-tab-pane>

                    <el-tab-pane
                      v-if="sr.output && Object.keys(sr.output).length"
                      label="步骤输出"
                      name="output"
                    >
                      <section class="step-result-snapshot-panel step-result-snapshot-panel--full">
                        <div class="step-result-panel-head">
                          <span>步骤输出</span>
                          <span class="step-result-panel-hint">
                            可通过 <code>{{ stepsRefSnippet(sr.step.stepSeq, '.xxx') }}</code> 在后续步骤中引用
                          </span>
                        </div>
                        <template v-for="lines in [runResultJsonLines(si, 'output', sr.output)]" :key="'out-lines-' + si">
                          <div v-if="lines?.length" class="json-viewer snapshot-pre run-result-json-viewer">
                            <div
                              v-for="line in lines"
                              :key="'out-' + si + '-' + line.id"
                              class="json-line"
                              :style="{ paddingLeft: `${line.depth * 16}px` }"
                            >
                              <span
                                v-if="line.type === 'open'"
                                class="json-toggle"
                                @click="toggleRunResultJsonCollapse(si, 'output', line.path)"
                              />
                              <span
                                v-else-if="line.type === 'collapsed'"
                                class="json-toggle is-collapsed"
                                @click="toggleRunResultJsonCollapse(si, 'output', line.path)"
                              />
                              <span v-else class="json-toggle-ph" />
                              <span
                                v-if="line.key !== null && line.key !== undefined"
                                class="json-key"
                              >"{{ line.key }}"<span class="json-colon">: </span></span>
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
                        </template>
                      </section>
                    </el-tab-pane>
                  </el-tabs>
                </div>
              </el-collapse-item>
            </el-collapse>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <AIGenerateDialog
      v-model="aiScenarioScriptDialogVisible"
      action="generate_assertion"
      title="AI 生成场景脚本"
      :context="aiScenarioScriptContext"
      @apply="onScenarioScriptAIApply"
      @generation-settled="aiScenarioScriptLoading = false"
    />

    <AIGenerateDialog
      v-model="aiStepParamsDialogVisible"
      action="generate_params"
      :context="scenarioStepAiParamsContext"
      @apply="onScenarioStepAIParamsApply"
      @generation-settled="aiStepParamsLoading = false"
    />
  </div>
</template>

<script>
import Sortable from 'sortablejs'
import {
  CircleCheck,
  CircleClose,
  Plus,
  QuestionFilled,
  Remove,
  WarningFilled
} from '@element-plus/icons-vue'
import {
  deleteScenarioStep,
  generateCaseParams,
  getDataSourceSchema,
  listScenarioSteps,
  reorderScenarioSteps,
  runCase,
  runScenario,
  upsertScenarioStep
} from '../../api'
import { buildCurlFromRequestSnapshot } from '../../utils/curl'
import SqlEditor from '../../components/SqlEditor.vue'
import AssertionEditor from '../../components/AssertionEditor.vue'
import ScriptLibraryPicker from '../../components/ScriptLibraryPicker.vue'
import AIGenerateDialog from '../../components/AIGenerateDialog.vue'
import JsonBodyEditor from '../../components/JsonBodyEditor.vue'
import ScenarioStepTreeNode from './ScenarioStepTreeNode.vue'
import {
  SCENARIO_STEP_WORKSPACE_KEY,
  buildScopeKey,
  readTabStateByScopeKey,
  writeTabStateByScopeKey
} from '../../utils/workspaceTabs.js'

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

const BODY_SCHEMA_WIDTH_SCENARIO_KEY = 'autotest.scenario.bodySchemaPanelWidth'
const BODY_SCHEMA_PANEL_MIN_WIDTH = 260
const BODY_SCHEMA_PANEL_MAX_WIDTH = 760
const BODY_SCHEMA_EDITOR_MIN_WIDTH = 320

function clampNumber(value, min, max) {
  return Math.min(Math.max(value, min), max)
}

function readStoredScenarioBodySchemaPanelWidth() {
  try {
    const value = Number(localStorage.getItem(BODY_SCHEMA_WIDTH_SCENARIO_KEY))
    if (Number.isFinite(value) && value > 0) {
      return clampNumber(value, BODY_SCHEMA_PANEL_MIN_WIDTH, BODY_SCHEMA_PANEL_MAX_WIDTH)
    }
  } catch {
    /* ignore */
  }
  return BODY_SCHEMA_PANEL_MIN_WIDTH
}

function storeScenarioBodySchemaPanelWidth(width) {
  try {
    localStorage.setItem(BODY_SCHEMA_WIDTH_SCENARIO_KEY, String(Math.round(width)))
  } catch {
    /* ignore */
  }
}

const STEP_SIDEBAR_WIDTH_STORAGE_KEY = 'autotest.scenarioStepSidebarWidth'
const STEP_SIDEBAR_MIN = 180
const STEP_SIDEBAR_MAX = 560
const STEP_SIDEBAR_DEFAULT = 260

const STEP_TYPES = [
  { label: 'API', value: 'api' },
  { label: '数据库', value: 'database' },
  { label: '脚本', value: 'script' },
  { label: 'For循环', value: 'for' },
  { label: '条件分支', value: 'condition' }
]

const CONDITION_OPERATORS = [
  { label: '等于', value: 'equals' },
  { label: '不等于', value: 'not_equals' },
  { label: '包含', value: 'contains' },
  { label: '不包含', value: 'not_contains' },
  { label: '大于', value: 'greater_than' },
  { label: '大于等于', value: 'greater_or_equal' },
  { label: '小于', value: 'less_than' },
  { label: '小于等于', value: 'less_or_equal' },
  { label: '存在', value: 'exists' },
  { label: '不存在', value: 'not_exists' },
  { label: '为真', value: 'truthy' },
  { label: '为假', value: 'falsy' }
]

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

export default {
  name: 'ScenarioEditor',
  components: {
    CircleCheck,
    CircleClose,
    Plus,
    QuestionFilled,
    Remove,
    WarningFilled,
    ScenarioStepTreeNode,
    SqlEditor,
    AssertionEditor,
    ScriptLibraryPicker,
    AIGenerateDialog,
    JsonBodyEditor
  },

  props: {
    scenario: { type: Object, required: true },
    active: { type: Boolean, default: true },
    cases: { type: Array, default: () => [] },
    serviceEndpoints: { type: Array, default: () => [] },
    environments: { type: Array, default: () => [] },
    dataSources: { type: Array, default: () => [] },
    projectId: { type: String, default: '' },
    serviceId: { type: String, default: '' }
  },

  data() {
    return {
      steps: [],
      schemaTables: [],
      runEnvId: '',
      running: false,
      runningStep: false,
      savingStep: false,
      generatingStepParams: false,
      aiStepParamsDialogVisible: false,
      aiStepParamsLoading: false,
      lastRunOutput: null,
      stepRunOutput: null,
      stepRunResultTab: 'response',
      stepRunResponseDetailTab: 'body',
      runResultDetailTabByStep: {},
      scenarioRunResultActiveNames: ['panel'],
      runResultPanelHeightPx: 360,
      runResultJsonCollapsed: {},
      _runResultPanelResize: null,

      stepSidebarWidthPx: STEP_SIDEBAR_DEFAULT,
      _stepSidebarResize: null,

      aiScenarioScriptDialogVisible: false,
      aiScenarioScriptLoading: false,
      stepRequestBodyViewMode: 'edit',
      stepBodySchemaPanelWidth: readStoredScenarioBodySchemaPanelWidth(),
      stepBodySchemaResizeState: null,
      stepRequestBodyCollapsed: {},
      stepRequestBodyJsonCollapsed: {},
      stepDebugRequestSnapshotJsonCollapsed: {},
      stepDebugResponseBodyCollapsed: {},
      stepDebugResponseJsonCollapsed: {},

      editingStep: null,
      editingStepName: false,
      controlFlowChildAttach: null,
      sortableInstance: null,

      stepTabs: [],
      activeStepTabId: '',

      stepTypes: STEP_TYPES,
      conditionOperators: CONDITION_OPERATORS,
      httpMethods: HTTP_METHODS,

      stepForm: {
        name: '',
        stepType: 'api',
        stepSeq: null,
        stepOrder: undefined,
        testCaseId: '',
        enabled: true,
        dbDataSourceId: '',
        dbSQL: '',
        dbInputParamsText: '[]',
        dbTimeoutMillis: 3000,
        scriptSource: '',
        scriptTimeoutMillis: 10000,
        requestMethod: 'GET',
        requestPath: '',
        requestTab: 'params',
        pathParamRows: [],
        queryRows: [],
        headerRows: [],
        bodyText: '',
        requestSecurity: undefined,
        forMode: 'count',
        forCountExpression: '1',
        forItemsExpression: '[]',
        forItemVar: 'item',
        forIndexVar: 'index',
        forBodyStepSeqs: [],
        forMaxIterations: 100,
        conditionBranches: [
          { left: '', operator: 'equals', right: '', stepSeqs: [] }
        ],
        conditionElseStepSeqs: [],
        stepAssertions: []
      }
    }
  },

  computed: {
    panelSteps() {
      return [...this.steps].sort((a, b) => a.stepOrder - b.stepOrder)
    },
    stepTreeBlocks() {
      return this.buildStepBlocks()
    },
    stepRunResult() {
      return this.stepRunOutput?.result || this.stepRunOutput?.results?.[0] || null
    },
    stepRunHttpResponse() {
      return this.httpResponseFromSnapshot(this.stepRunResult?.responseSnapshot)
    },
    isStepFormActive() {
      if (!this.activeStepTabId) return false
      const o = Number(this.stepForm.stepOrder)
      return Number.isFinite(o) && o > 0
    },
    activeStepTab() {
      return this.stepTabs.find((t) => t.id === this.activeStepTabId) || null
    },
    activeTabStepId() {
      const tab = this.activeStepTab
      return tab && tab.kind === 'step' ? tab.stepId : ''
    },
    stepWorkspaceScopeKey() {
      return buildScopeKey(this.projectId, this.serviceId, this.scenario?.id)
    },
    stepTitleDisplay() {
      const custom = (this.stepForm.name || '').trim()
      if (custom) return custom
      if (this.stepForm.stepType === 'api') {
        return this.stepForm.testCaseId ? this.findCaseName(this.stepForm.testCaseId) : ''
      }
      return this.defaultStepTypeName(this.stepForm.stepType)
    },
    stepSidebarStyle() {
      const w = this.stepSidebarWidthPx
      return {
        width: `${w}px`,
        flex: `0 0 ${w}px`
      }
    },
    runResultPanelBodyStyle() {
      return {
        height: `${this.runResultPanelHeightPx}px`
      }
    },
    scenarioStepResolvedEndpoint() {
      if (this.stepForm.stepType !== 'api' || !this.stepForm.testCaseId) return null
      const tc = this.findCase(this.stepForm.testCaseId)
      if (!tc) return null
      return this.findEndpoint(this.serviceEndpoints, tc)
    },
    scenarioStepRequestSchema() {
      return this.normalizeRequest(this.scenarioStepResolvedEndpoint?.requestSchema)
    },
    scenarioStepResponseSchema() {
      return this.normalizeRequest(this.scenarioStepResolvedEndpoint?.responseSchema)
    },
    scenarioStepRequestBodySchema() {
      return this.scenarioStepRequestSchema?.body || null
    },
    scenarioStepRequestBodyAllRows() {
      return this.schemaDocRows(this.scenarioStepRequestBodySchema)
    },
    scenarioStepRequestBodyDocRows() {
      return this.filterSchemaRows(this.scenarioStepRequestBodyAllRows, this.stepRequestBodyCollapsed)
    },
    scenarioStepBodySchemaLayoutStyle() {
      return Number.isFinite(this.stepBodySchemaPanelWidth) && this.stepBodySchemaPanelWidth > 0
        ? { '--body-schema-panel-width': `${this.stepBodySchemaPanelWidth}px` }
        : {}
    },
    scenarioStepBodyEditorJsonLines() {
      if (this.stepRequestBodyViewMode !== 'preview') return null
      const raw = (this.stepForm.bodyText || '').trim()
      if (!raw) return null
      try {
        const parsed = this.parseTemplateAwareJSON(raw)
        const lines = []
        this.collectJsonLines(parsed, null, '$', 0, true, this.stepRequestBodyJsonCollapsed, lines)
        return lines
      } catch {
        return null
      }
    },
    stepDebugRunRecord() {
      return this.stepRunOutput?.run || null
    },
    stepDebugRequestSnapshot() {
      return this.stepRunResult?.requestSnapshot || {}
    },
    stepDebugResponseSnapshot() {
      const o = this.parseSnapshotObject(this.stepRunResult?.responseSnapshot)
      return o && typeof o === 'object' && !Array.isArray(o) ? o : {}
    },
    stepDebugRequestQueryRows() {
      const url = this.stepDebugRequestSnapshot?.url
      if (!url) return []
      try {
        const parsed = new URL(url, window.location.origin)
        return Array.from(parsed.searchParams.entries()).map(([key, value]) => ({ key, value }))
      } catch {
        return []
      }
    },
    stepDebugAssertions() {
      return Array.isArray(this.stepRunResult?.assertions) ? this.stepRunResult.assertions : []
    },
    stepDebugCurlCommand() {
      return buildCurlFromRequestSnapshot(this.stepDebugRequestSnapshot)
    },
    stepDebugResponseBodyParsed() {
      const body = this.stepDebugResponseSnapshot?.body
      if (body == null || body === '') return undefined
      try {
        return typeof body === 'string' ? JSON.parse(body) : body
      } catch {
        return undefined
      }
    },
    stepDebugResponseBodyJsonLines() {
      if (this.stepDebugResponseBodyParsed === undefined) return null
      const lines = []
      this.collectJsonLines(this.stepDebugResponseBodyParsed, null, '$', 0, true, this.stepDebugResponseJsonCollapsed, lines)
      return lines
    },
    stepDebugResponseBodySchema() {
      return this.scenarioStepResponseSchema?.body || null
    },
    stepDebugResponseBodyAllRows() {
      return this.schemaDocRows(this.stepDebugResponseBodySchema)
    },
    stepDebugResponseBodyDocRows() {
      return this.filterSchemaRows(this.stepDebugResponseBodyAllRows, this.stepDebugResponseBodyCollapsed)
    },
    stepDebugRequestSnapshotBodyParsed() {
      const body = this.stepDebugRequestSnapshot?.body
      if (body == null || body === '') return undefined
      try {
        return typeof body === 'string' ? JSON.parse(body) : body
      } catch {
        return undefined
      }
    },
    stepDebugRequestSnapshotBodyJsonLines() {
      if (this.stepDebugRequestSnapshotBodyParsed === undefined) return null
      const lines = []
      this.collectJsonLines(
        this.stepDebugRequestSnapshotBodyParsed,
        null,
        '$',
        0,
        true,
        this.stepDebugRequestSnapshotJsonCollapsed,
        lines
      )
      return lines
    },
    aiScenarioScriptContext() {
      return {
        scenarioName: this.scenario?.name || '',
        scenarioDescription: this.scenario?.description || '',
        currentStepName: this.stepForm?.name || '',
        currentStepSeq: this.stepForm?.stepSeq || null,
        existingScript: this.stepForm?.scriptSource || '',
        relatedSteps: this.panelSteps.map((s) => ({
          stepSeq: s.stepSeq,
          stepOrder: s.stepOrder,
          stepType: s.stepType,
          name: s.name
        }))
      }
    },
    scenarioStepAiParamsContext() {
      const tc = this.findCase(this.stepForm.testCaseId)
      if (!tc || this.stepForm.stepType !== 'api') {
        return {
          method: '',
          path: '',
          endpoint: null,
          currentRequest: { path: '', query: {}, headers: {}, variables: {}, body: null }
        }
      }
      let bodyDraft = null
      if ((this.stepForm.bodyText || '').trim()) {
        try {
          bodyDraft = this.parseTemplateAwareJSON(this.stepForm.bodyText)
        } catch {
          bodyDraft = this.stepForm.bodyText
        }
      }
      const pathVars = this.rowsToObject(this.stepForm.pathParamRows)
      const ep = this.scenarioStepResolvedEndpoint
      return {
        method: this.stepForm.requestMethod || tc.method,
        path: this.stepForm.requestPath || tc.path,
        endpoint: ep
          ? {
              summary: ep.summary,
              description: ep.description,
              parameters: ep.parameters,
              requestSchema: ep.requestSchema,
              responses: ep.responses
            }
          : null,
        currentRequest: {
          path: this.stepForm.requestPath || tc.path,
          query: this.rowsToObject(this.stepForm.queryRows),
          headers: this.rowsToObject(this.stepForm.headerRows),
          variables: pathVars,
          body: bodyDraft
        }
      }
    }
  },

  watch: {
    'scenario.id'(newId, oldId) {
      if (!newId || newId === oldId) return
      if (oldId) {
        const oldScope = buildScopeKey(this.projectId, this.serviceId, oldId)
        this.flushStepTabsToStorage(oldScope)
      }
      this.lastRunOutput = null
      this.runResultDetailTabByStep = {}
      this.runResultJsonCollapsed = {}
      this._stepTabDrafts = {}
      this.stepTabs = []
      this.activeStepTabId = ''
      this.loadStepsForScenario()
    },
    'stepForm.dbDataSourceId'(id) {
      this.loadSchema(id)
    },
    panelSteps: {
      handler() {
        this.$nextTick(() => this.initStepSortable())
      },
      deep: true
    },
    lastRunOutput(val) {
      if (val) this.scenarioRunResultActiveNames = ['panel']
    },
    aiScenarioScriptDialogVisible(val) {
      if (!val) this.aiScenarioScriptLoading = false
    },
    aiStepParamsDialogVisible(val) {
      if (!val) this.aiStepParamsLoading = false
    },
    active(val) {
      if (val) {
        this.$nextTick(() => {
          this.initStepSortable()
        })
      } else if (this.sortableInstance) {
        this.sortableInstance.destroy()
        this.sortableInstance = null
      }
    },
    environments(envs) {
      if (!this.runEnvId && Array.isArray(envs) && envs.length) {
        this.runEnvId = envs[0].id
      }
      this.refreshEnvironmentAuthRows()
    },
    runEnvId() {
      this.refreshEnvironmentAuthRows()
    }
  },

  provide() {
    return {
      scenarioStepListApi: {
        selectPanelStep: (s) => this.selectPanelStep(s),
        isPanelStepActive: (s) => this.isPanelStepActive(s),
        stepDisplayName: (s) => this.stepDisplayName(s),
        stepTypeTagType: (t) => this.stepTypeTagType(t),
        stepTypeLabel: (t) => this.stepTypeLabel(t),
        onStepEnabledChange: (s, v) => this.onStepEnabledChange(s, v),
        confirmDeleteStep: (s) => this.confirmDeleteStep(s),
        startNewControlChildDraft: (p, b) => this.startNewControlChildDraft(p, b),
        leafRowClass: (depth) => this.leafRowClass(depth)
      }
    }
  },

  async created() {
    this._stepTabDrafts = {}
    this._stepTabsPersistTimer = null
    this._restoringStepTabs = false
    this.initStepSidebarWidth()
    if (Array.isArray(this.environments) && this.environments.length) {
      this.runEnvId = this.environments[0].id
    }
    this.refreshEnvironmentAuthRows()
    await this.loadStepsForScenario()
  },

  beforeUnmount() {
    this.detachStepSidebarResizeListeners()
    this.detachRunResultPanelResizeListeners()
    this.stopStepBodySchemaResize()
    if (this.sortableInstance) {
      this.sortableInstance.destroy()
      this.sortableInstance = null
    }
    if (this._stepTabsPersistTimer) {
      clearTimeout(this._stepTabsPersistTimer)
      this._stepTabsPersistTimer = null
    }
    this.flushStepTabsToStorage(this.stepWorkspaceScopeKey)
  },

  methods: {
    tabIdForStep(stepId) {
      return stepId ? `step:${stepId}` : ''
    },
    generateDraftTabId() {
      const rand = Math.random().toString(36).slice(2, 9)
      return `draft:${Date.now().toString(36)}-${rand}`
    },

    findStepTab(tabId) {
      if (!tabId) return null
      return this.stepTabs.find((t) => t.id === tabId) || null
    },

    stepTabLabelForStep(step) {
      if (!step) return '步骤'
      const customName = (step.name || '').trim()
      if (customName) return customName
      if (step.stepType === 'api' && step.testCaseId) {
        const caseName = this.findCaseName?.(step.testCaseId)
        if (caseName) return caseName
      }
      const seq = step.stepSeq != null && step.stepSeq !== '' ? `#${step.stepSeq}` : ''
      const typeLabel = this.stepTypeLabel?.(step.stepType) || step.stepType || '步骤'
      return seq ? `${seq} ${typeLabel}` : typeLabel
    },

    stepTabLabelForForm() {
      const display = (this.stepForm?.name || '').trim() || this.stepTitleDisplay
      if (display) return display
      const typeLabel = this.stepTypeLabel?.(this.stepForm?.stepType) || '步骤'
      return `新增${typeLabel}`
    },

    cloneStepFormForSnapshot() {
      try {
        return JSON.parse(JSON.stringify(this.stepForm))
      } catch {
        return { ...this.stepForm }
      }
    },

    snapshotActiveStepTabState() {
      const tabId = this.activeStepTabId
      if (!tabId) return
      if (!this._stepTabDrafts) this._stepTabDrafts = {}
      this._stepTabDrafts[tabId] = {
        stepForm: this.cloneStepFormForSnapshot(),
        editingStepId: this.editingStep?.id || '',
        editingStepName: this.editingStepName,
        controlFlowChildAttach: this.controlFlowChildAttach
          ? { ...this.controlFlowChildAttach }
          : null,
        stepRunOutput: this.stepRunOutput,
        stepRunResultTab: this.stepRunResultTab,
        stepRunResponseDetailTab: this.stepRunResponseDetailTab
      }
    },

    restoreStepTabStateFromDraft(tabId) {
      const draft = this._stepTabDrafts && this._stepTabDrafts[tabId]
      if (!draft) return false
      this.stepForm = draft.stepForm
      this.editingStep = draft.editingStepId
        ? this.steps.find((s) => s.id === draft.editingStepId) || null
        : null
      this.editingStepName = !!draft.editingStepName
      this.controlFlowChildAttach = draft.controlFlowChildAttach
        ? { ...draft.controlFlowChildAttach }
        : null
      this.stepRunOutput = draft.stepRunOutput || null
      this.stepRunResultTab = draft.stepRunResultTab || 'response'
      this.stepRunResponseDetailTab = draft.stepRunResponseDetailTab || 'body'
      this.stepRequestBodyViewMode = 'edit'
      this.stepRequestBodyCollapsed = {}
      this.stepRequestBodyJsonCollapsed = {}
      this.stepDebugRequestSnapshotJsonCollapsed = {}
      this.stepDebugResponseBodyCollapsed = {}
      this.stepDebugResponseJsonCollapsed = {}
      return true
    },

    discardStepTabDraft(tabId) {
      if (this._stepTabDrafts && tabId) {
        delete this._stepTabDrafts[tabId]
      }
    },

    openStepTab(step) {
      if (!step?.id) return
      this.snapshotActiveStepTabState()
      const tabId = this.tabIdForStep(step.id)
      let tab = this.findStepTab(tabId)
      if (!tab) {
        tab = {
          id: tabId,
          kind: 'step',
          stepId: step.id,
          label: this.stepTabLabelForStep(step)
        }
        this.stepTabs.push(tab)
      } else {
        const label = this.stepTabLabelForStep(step)
        if (tab.label !== label) tab.label = label
      }
      this.activeStepTabId = tabId
      if (!this.restoreStepTabStateFromDraft(tabId)) {
        this.applyStepToForm(step)
      }
      this.scheduleStepTabsPersist()
    },

    openDraftStepTab(attach = null) {
      this.snapshotActiveStepTabState()
      const tabId = this.generateDraftTabId()
      this.stepTabs.push({
        id: tabId,
        kind: 'draft',
        stepId: '',
        label: '新增步骤'
      })
      this.activeStepTabId = tabId
      this.editingStep = null
      this.editingStepName = false
      this.controlFlowChildAttach = attach || null
      this.stepRunOutput = null
      this.stepRunResultTab = 'response'
      this.stepRunResponseDetailTab = 'body'
      this.stepRequestBodyViewMode = 'edit'
      this.stepRequestBodyCollapsed = {}
      this.stepRequestBodyJsonCollapsed = {}
      this.stepDebugRequestSnapshotJsonCollapsed = {}
      this.stepDebugResponseBodyCollapsed = {}
      this.stepDebugResponseJsonCollapsed = {}
      this.stepForm = this.emptyStepForm(this.nextStepOrder())
      this.scheduleStepTabsPersist()
    },

    onStepTabBeforeLeave(_targetName, currentName) {
      if (currentName && currentName !== _targetName) {
        // 此时 activeStepTabId 仍指向旧 tab，stepForm 仍是旧 tab 的内容，先快照。
        this.snapshotActiveStepTabState()
      }
      return true
    },

    onStepTabChange(targetTabId) {
      if (!targetTabId) return
      this.applyStepTabContext(targetTabId)
      this.scheduleStepTabsPersist()
    },

    applyStepTabContext(tabId) {
      const tab = this.findStepTab(tabId)
      if (!tab) return
      if (this.restoreStepTabStateFromDraft(tabId)) return
      if (tab.kind === 'step') {
        const step = this.steps.find((s) => s.id === tab.stepId)
        if (step) {
          this.applyStepToForm(step)
        } else {
          this.removeStepTab(tabId)
        }
      } else {
        this.editingStep = null
        this.editingStepName = false
        this.controlFlowChildAttach = null
        this.stepRunOutput = null
        this.stepRunResultTab = 'response'
        this.stepRunResponseDetailTab = 'body'
        this.stepRequestBodyViewMode = 'edit'
        this.stepRequestBodyCollapsed = {}
        this.stepRequestBodyJsonCollapsed = {}
        this.stepDebugRequestSnapshotJsonCollapsed = {}
        this.stepDebugResponseBodyCollapsed = {}
        this.stepDebugResponseJsonCollapsed = {}
        this.stepForm = this.emptyStepForm(this.nextStepOrder())
      }
    },

    switchToStepTab(tabId) {
      const tab = this.findStepTab(tabId)
      if (!tab) return
      if (tabId === this.activeStepTabId) return
      this.snapshotActiveStepTabState()
      this.activeStepTabId = tabId
      this.applyStepTabContext(tabId)
      this.scheduleStepTabsPersist()
    },

    closeStepTab(tabId) {
      if (!tabId) return
      this.removeStepTab(tabId)
    },

    removeStepTab(tabId) {
      const idx = this.stepTabs.findIndex((t) => t.id === tabId)
      if (idx < 0) return
      const wasActive = this.activeStepTabId === tabId
      this.stepTabs.splice(idx, 1)
      this.discardStepTabDraft(tabId)
      if (wasActive) {
        const next = this.stepTabs[idx] || this.stepTabs[idx - 1] || null
        if (next) {
          this.activeStepTabId = next.id
          this.applyStepTabContext(next.id)
        } else {
          this.activeStepTabId = ''
          this.editingStep = null
          this.editingStepName = false
          this.controlFlowChildAttach = null
          this.stepRunOutput = null
          this.stepForm = this.emptyStepForm(undefined)
        }
      }
      this.scheduleStepTabsPersist()
    },

    refreshStepTabLabel(step) {
      if (!step?.id) return
      const tab = this.findStepTab(this.tabIdForStep(step.id))
      if (!tab) return
      const label = this.stepTabLabelForStep(step)
      if (tab.label !== label) {
        tab.label = label
        this.scheduleStepTabsPersist()
      }
    },

    promoteDraftTabToStep(savedStep) {
      if (!savedStep?.id) return
      const tabId = this.activeStepTabId
      const tab = this.findStepTab(tabId)
      if (!tab || tab.kind !== 'draft') return
      const newTabId = this.tabIdForStep(savedStep.id)
      const existingIdx = this.stepTabs.findIndex(
        (t) => t.id === newTabId && t.id !== tabId
      )
      if (existingIdx !== -1) {
        this.stepTabs.splice(existingIdx, 1)
        if (this._stepTabDrafts) delete this._stepTabDrafts[newTabId]
      }
      tab.id = newTabId
      tab.kind = 'step'
      tab.stepId = savedStep.id
      tab.label = this.stepTabLabelForStep(savedStep)
      if (this._stepTabDrafts && this._stepTabDrafts[tabId]) {
        this._stepTabDrafts[newTabId] = this._stepTabDrafts[tabId]
        delete this._stepTabDrafts[tabId]
      }
      this.activeStepTabId = newTabId
      this.scheduleStepTabsPersist()
    },

    scheduleStepTabsPersist() {
      if (this._restoringStepTabs) return
      if (!this.stepWorkspaceScopeKey) return
      if (this._stepTabsPersistTimer) clearTimeout(this._stepTabsPersistTimer)
      this._stepTabsPersistTimer = setTimeout(() => {
        this._stepTabsPersistTimer = null
        this.flushStepTabsToStorage(this.stepWorkspaceScopeKey)
      }, 150)
    },

    flushStepTabsToStorage(scopeKey) {
      if (!scopeKey) return
      const persistableTabs = this.stepTabs
        .filter((t) => t.kind === 'step' && t.stepId)
        .map((t) => ({
          id: t.id,
          kind: t.kind,
          stepId: t.stepId,
          label: t.label || ''
        }))
      const activeTabPersistable =
        this.stepTabs.find((t) => t.id === this.activeStepTabId)?.kind === 'step'
          ? this.activeStepTabId
          : ''
      writeTabStateByScopeKey(SCENARIO_STEP_WORKSPACE_KEY, scopeKey, {
        tabs: persistableTabs,
        activeTabId: activeTabPersistable
      })
    },

    restoreStepTabsFromStorage() {
      const scopeKey = this.stepWorkspaceScopeKey
      if (!scopeKey) return false
      const stored = readTabStateByScopeKey(SCENARIO_STEP_WORKSPACE_KEY, scopeKey)
      const stepIndex = new Map(this.steps.map((s) => [s.id, s]))
      const restored = []
      for (const t of stored.tabs) {
        if (t.kind !== 'step' || !t.stepId) continue
        const live = stepIndex.get(t.stepId)
        if (!live) continue
        restored.push({
          id: this.tabIdForStep(t.stepId),
          kind: 'step',
          stepId: t.stepId,
          label: this.stepTabLabelForStep(live)
        })
      }
      if (!restored.length) return false
      this._restoringStepTabs = true
      try {
        this.stepTabs = restored
        const wantActive = stored.activeTabId
        const matchedActive = restored.find((t) => t.id === wantActive)
        const finalActive = matchedActive || restored[0]
        this.activeStepTabId = finalActive.id
        const step = stepIndex.get(finalActive.stepId)
        if (step) this.applyStepToForm(step)
      } finally {
        this._restoringStepTabs = false
      }
      return true
    },

    initStepSidebarWidth() {
      try {
        const raw = localStorage.getItem(STEP_SIDEBAR_WIDTH_STORAGE_KEY)
        const w = raw != null ? Number.parseInt(raw, 10) : NaN
        if (!Number.isNaN(w)) {
          this.stepSidebarWidthPx = Math.min(STEP_SIDEBAR_MAX, Math.max(STEP_SIDEBAR_MIN, w))
        }
      } catch {
        /* ignore */
      }
    },
    persistStepSidebarWidth() {
      try {
        localStorage.setItem(STEP_SIDEBAR_WIDTH_STORAGE_KEY, String(this.stepSidebarWidthPx))
      } catch {
        /* ignore */
      }
    },
    clampStepSidebarWidth(value) {
      return Math.min(STEP_SIDEBAR_MAX, Math.max(STEP_SIDEBAR_MIN, value))
    },
    nudgeStepSidebar(delta) {
      this.stepSidebarWidthPx = this.clampStepSidebarWidth(this.stepSidebarWidthPx + delta)
      this.persistStepSidebarWidth()
    },
    startStepSidebarResize(downEvent) {
      const startX = downEvent.clientX
      const startW = this.stepSidebarWidthPx
      const onMove = (e) => {
        const dx = e.clientX - startX
        this.stepSidebarWidthPx = this.clampStepSidebarWidth(startW + dx)
      }
      const onUp = () => {
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        this.persistStepSidebarWidth()
      }
      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      this._stepSidebarResize = { onMove, onUp }
    },
    detachStepSidebarResizeListeners() {
      if (!this._stepSidebarResize) return
      document.removeEventListener('mousemove', this._stepSidebarResize.onMove)
      document.removeEventListener('mouseup', this._stepSidebarResize.onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      this._stepSidebarResize = null
    },

    setRunResultDetailTab(stepIndex, tab) {
      this.runResultDetailTabByStep = {
        ...this.runResultDetailTabByStep,
        [stepIndex]: tab
      }
    },
    clampRunResultPanelHeight(value) {
      const viewport = typeof window !== 'undefined' ? window.innerHeight : 800
      const max = Math.max(260, Math.floor(viewport * 0.72))
      return clampNumber(Number(value) || 360, 220, max)
    },
    nudgeRunResultPanelHeight(delta) {
      this.runResultPanelHeightPx = this.clampRunResultPanelHeight(this.runResultPanelHeightPx + delta)
    },
    startRunResultPanelResize(event) {
      if (event.button !== undefined && event.button !== 0) return
      const startY = event.clientY
      const startH = this.runResultPanelHeightPx
      const onMove = (e) => {
        this.runResultPanelHeightPx = this.clampRunResultPanelHeight(startH + startY - e.clientY)
      }
      const onUp = () => {
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        this._runResultPanelResize = null
      }
      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
      document.body.style.cursor = 'row-resize'
      document.body.style.userSelect = 'none'
      this._runResultPanelResize = { onMove, onUp }
    },
    detachRunResultPanelResizeListeners() {
      if (!this._runResultPanelResize) return
      document.removeEventListener('mousemove', this._runResultPanelResize.onMove)
      document.removeEventListener('mouseup', this._runResultPanelResize.onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      this._runResultPanelResize = null
    },
    runResultCollapseKey(stepIndex, kind) {
      return `${stepIndex}:${kind}`
    },
    toggleRunResultJsonCollapse(stepIndex, kind, path) {
      const key = this.runResultCollapseKey(stepIndex, kind)
      const current = { ...(this.runResultJsonCollapsed[key] || {}) }
      if (current[path]) {
        delete current[path]
      } else {
        current[path] = true
      }
      this.runResultJsonCollapsed = {
        ...this.runResultJsonCollapsed,
        [key]: current
      }
    },
    runResultJsonLines(stepIndex, kind, snapshot) {
      const value = this.normalizeSnapshotForViewer(snapshot)
      if (value == null || value === '') return []
      const lines = []
      const collapsed = this.runResultJsonCollapsed[this.runResultCollapseKey(stepIndex, kind)] || {}
      this.collectJsonLines(value, null, '$', 0, true, collapsed, lines)
      return lines
    },
    normalizeSnapshotForViewer(value) {
      if (value == null || value === '') return value
      if (typeof value === 'string') {
        const parsed = this.tryParseJSONText(value)
        return parsed === undefined ? value : this.normalizeSnapshotForViewer(parsed)
      }
      if (Array.isArray(value)) {
        return value.map((item) => this.normalizeSnapshotForViewer(item))
      }
      if (typeof value === 'object') {
        return Object.fromEntries(
          Object.entries(value).map(([key, item]) => [key, this.normalizeSnapshotForViewer(item)])
        )
      }
      return value
    },
    tryParseJSONText(value) {
      const text = String(value).trim()
      if (!text || !['{', '['].includes(text[0])) return undefined
      try {
        return JSON.parse(text)
      } catch {
        return undefined
      }
    },
    runResultRequestMethod(sr) {
      const req = this.parseSnapshotObject(sr?.result?.requestSnapshot)
      return req?.method || req?.type || ''
    },
    stepsRefSnippet(stepSeq, pathAfterBracket) {
      const n = stepSeq != null ? String(stepSeq) : '?'
      return `{{$steps[${n}]${pathAfterBracket}}}`
    },

    leafRowClass(depth) {
      return {
        'nested-step-row': depth > 0,
        'nested-step-row--block': depth >= 2
      }
    },

    buildStepBlocks() {
      const stepBySeq = new Map()
      for (const step of this.panelSteps) {
        const seq = Number(step.stepSeq)
        if (Number.isFinite(seq) && seq > 0) stepBySeq.set(seq, step)
      }

      const controlledSeqs = new Set()
      for (const step of this.panelSteps) {
        for (const seq of this.controlChildSeqs(step)) controlledSeqs.add(seq)
      }

      const blockForStep = (step, depth, path = new Set()) => {
        const seq = Number(step.stepSeq)
        if (!Number.isFinite(seq) || path.has(seq)) {
          return { type: 'leaf', key: `leaf-${step.id}-${depth}`, step, depth }
        }

        const nextPath = new Set(path)
        nextPath.add(seq)

        if (step.stepType === 'for') {
          const children = []
          for (const childSeq of this.controlChildSeqs(step, 'body')) {
            const child = stepBySeq.get(childSeq)
            if (child) children.push(blockForStep(child, depth + 2, nextPath))
          }
          return { type: 'for', key: `for-${step.id}`, step, depth, children }
        }

        if (step.stepType === 'condition') {
          const cfg = this.parseMaybeJSON(step.config)
          const norm = this.normalizeConditionConfig(cfg)
          const branchFrames = norm.branches.map((br, idx) => {
            const children = []
            for (const childSeq of this.normalizeStepSeqSelection(br.stepSeqs)) {
              const child = stepBySeq.get(childSeq)
              if (child) children.push(blockForStep(child, depth + 2, nextPath))
            }
            return {
              key: `cond-${step.id}-br-${idx}`,
              branchIndex: idx,
              children
            }
          })
          const elseChildren = []
          for (const childSeq of this.controlChildSeqs(step, 'else')) {
            const child = stepBySeq.get(childSeq)
            if (child) elseChildren.push(blockForStep(child, depth + 2, nextPath))
          }
          return {
            type: 'condition',
            key: `cond-${step.id}`,
            step,
            depth,
            branchFrames,
            elseChildren
          }
        }

        return { type: 'leaf', key: `leaf-${step.id}-${depth}`, step, depth }
      }

      const blocks = []
      let rootCount = 0
      for (const step of this.panelSteps) {
        const seq = Number(step.stepSeq)
        if (Number.isFinite(seq) && controlledSeqs.has(seq)) continue
        rootCount += 1
        blocks.push(blockForStep(step, 0, new Set()))
      }
      if (!rootCount && this.panelSteps.length) {
        for (const step of this.panelSteps) blocks.push(blockForStep(step, 0, new Set()))
      }
      return blocks
    },

    controlChildSeqs(step, branch) {
      const cfg = this.parseMaybeJSON(step?.config)
      if ((step?.stepType || 'api') === 'for') {
        return branch && branch !== 'body' ? [] : this.normalizeStepSeqSelection(cfg.bodyStepSeqs)
      }
      if ((step?.stepType || 'api') === 'condition') {
        const norm = this.normalizeConditionConfig(cfg)
        if (branch === 'else') return norm.elseStepSeqs
        if (typeof branch === 'number' && Number.isFinite(branch)) {
          const br = norm.branches[branch]
          return br ? this.normalizeStepSeqSelection(br.stepSeqs) : []
        }
        const all = []
        for (const b of norm.branches) {
          all.push(...this.normalizeStepSeqSelection(b.stepSeqs))
        }
        all.push(...norm.elseStepSeqs)
        return this.normalizeStepSeqSelection(all)
      }
      return []
    },

    normalizeConditionConfig(cfg = {}) {
      const legacyBranch = {
        left: cfg.left || '',
        operator: cfg.operator || 'equals',
        right: cfg.right || '',
        stepSeqs: this.normalizeStepSeqSelection(cfg.thenStepSeqs)
      }
      const branches = Array.isArray(cfg.branches) && cfg.branches.length
        ? cfg.branches.map((b) => ({
          left: b?.left || '',
          operator: b?.operator || 'equals',
          right: b?.right || '',
          stepSeqs: this.normalizeStepSeqSelection(b?.stepSeqs)
        }))
        : [legacyBranch]
      return {
        branches: branches.length ? branches : [legacyBranch],
        elseStepSeqs: this.normalizeStepSeqSelection(cfg.elseStepSeqs)
      }
    },

    addConditionBranch() {
      this.stepForm.conditionBranches.push({
        left: '',
        operator: 'equals',
        right: '',
        stepSeqs: []
      })
    },

    removeConditionBranch(index) {
      if (this.stepForm.conditionBranches.length <= 1) return
      this.stepForm.conditionBranches.splice(index, 1)
    },

    async loadSchema(dataSourceId) {
      this.schemaTables = []
      if (!dataSourceId) return
      try {
        this.schemaTables = await getDataSourceSchema(dataSourceId)
      } catch {
        /* ignore */
      }
    },

    initStepSortable() {
      if (!this.active) return
      if (this.sortableInstance) {
        this.sortableInstance.destroy()
        this.sortableInstance = null
      }
      const el = this.$refs.stepSortableList
      if (!el || !this.panelSteps.length || !this.scenario) return
      this.sortableInstance = Sortable.create(el, {
        animation: 150,
        filter:
          '.control-frame-border-add, .step-sidebar-add, .step-seq-badge, .step-delete-icon, .el-checkbox, .el-checkbox__input',
        preventOnFilter: false,
        onEnd: this.onStepSortEnd
      })
    },

    async onStepSortEnd(evt) {
      if (evt.oldIndex === evt.newIndex || !this.scenario) return
      const root = this.$refs.stepSortableList
      if (!root) return
      const ids = [...root.querySelectorAll('[data-step-id]')]
        .map((el) => el.dataset.stepId)
        .filter(Boolean)
      if (ids.length !== this.panelSteps.length) return
      try {
        await reorderScenarioSteps(this.scenario.id, { stepIds: ids })
        this.steps = await listScenarioSteps(this.scenario.id)
        for (const tab of this.stepTabs) {
          if (tab.kind !== 'step') continue
          const live = this.steps.find((s) => s.id === tab.stepId)
          if (live) {
            tab.label = this.stepTabLabelForStep(live)
            this.discardStepTabDraft(tab.id)
          }
        }
        const curId = this.editingStep?.id
        if (curId) {
          const u = this.steps.find((s) => s.id === curId)
          if (u) this.applyStepToForm(u)
        }
        this.scheduleStepTabsPersist()
      } catch {
        this.steps = await listScenarioSteps(this.scenario.id)
      }
    },

    parseMaybeJSON(val) {
      if (val == null || val === '') return {}
      if (typeof val === 'object' && !Array.isArray(val)) return val
      if (typeof val === 'string') {
        try {
          return JSON.parse(val)
        } catch {
          return {}
        }
      }
      return {}
    },

    async onStepEnabledChange(step, enabled) {
      if (!this.scenario) return
      const prev = step.enabled
      try {
        await upsertScenarioStep(this.scenario.id, step.stepOrder, {
          testCaseId: step.testCaseId,
          stepOrder: step.stepOrder,
          stepType: step.stepType || 'api',
          name: step.name || '',
          enabled,
          config: this.parseMaybeJSON(step.config),
          requestOverride: this.parseMaybeJSON(step.requestOverride)
        })
        step.enabled = enabled
        if (this.editingStep?.id === step.id) {
          this.stepForm.enabled = enabled
        }
        const tabId = this.tabIdForStep(step.id)
        const draft = this._stepTabDrafts && this._stepTabDrafts[tabId]
        if (draft?.stepForm) draft.stepForm.enabled = enabled
      } catch {
        step.enabled = prev
      }
    },

    async loadStepsForScenario() {
      if (!this.scenario?.id) {
        this.steps = []
        this.stepTabs = []
        this.activeStepTabId = ''
        this._stepTabDrafts = {}
        return
      }
      this.steps = await listScenarioSteps(this.scenario.id)
      this.stepTabs = []
      this.activeStepTabId = ''
      this._stepTabDrafts = {}
      const restored = this.restoreStepTabsFromStorage()
      if (restored) return
      if (this.steps.length) {
        this.openStepTab(this.steps[0])
      } else {
        this.openDraftStepTab()
      }
    },
    emptyStepForm(stepOrder) {
      return {
        name: '',
        stepType: 'api',
        stepSeq: null,
        testCaseId: '',
        enabled: true,
        stepOrder,
        dbDataSourceId: '',
        dbSQL: '',
        dbInputParamsText: '[]',
        dbTimeoutMillis: 3000,
        scriptSource: '',
        scriptTimeoutMillis: 10000,
        requestMethod: 'GET',
        requestPath: '',
        requestTab: 'params',
        pathParamRows: [],
        queryRows: [],
        headerRows: [],
        bodyText: '',
        requestSecurity: undefined,
        stepAssertions: [],
        forMode: 'count',
        forCountExpression: '1',
        forItemsExpression: '[]',
        forItemVar: 'item',
        forIndexVar: 'index',
        forBodyStepSeqs: [],
        forMaxIterations: 100,
        conditionBranches: [
          { left: '', operator: 'equals', right: '', stepSeqs: [] }
        ],
        conditionElseStepSeqs: []
      }
    },

    nextStepOrder() {
      if (!this.steps.length) return 1
      const nums = this.steps
        .map((s) => Number(s?.stepOrder))
        .filter((n) => Number.isFinite(n))
      if (!nums.length) return 1
      return Math.max(...nums) + 1
    },

    startNewStepDraft(attach = null) {
      this.openDraftStepTab(attach)
    },

    startNewControlChildDraft(parentStep, branch) {
      this.startNewStepDraft({
        parentId: parentStep.id,
        parentStepSeq: parentStep.stepSeq,
        branch
      })
      const branchName = branch === 'body'
        ? '循环体'
        : typeof branch === 'number'
          ? `条件分支 ${branch + 1}`
          : '「否则」'
      this.$message.info(`保存后会自动加入 #${parentStep.stepSeq} ${branchName}步骤组`)
    },

    applyStepToForm(step) {
      if (!step) return
      this.editingStep = step
      this.editingStepName = false
      if (step?.id) {
        this.controlFlowChildAttach = null
      }
      this.stepRunOutput = null
      this.stepRunResultTab = 'response'
      this.stepRunResponseDetailTab = 'body'
      this.stepRequestBodyViewMode = 'edit'
      this.stepRequestBodyCollapsed = {}
      this.stepRequestBodyJsonCollapsed = {}
      this.stepDebugRequestSnapshotJsonCollapsed = {}
      this.stepDebugResponseBodyCollapsed = {}
      this.stepDebugResponseJsonCollapsed = {}
      this.stepForm = this.emptyStepForm(step.stepOrder)
      this.stepForm.name = step.name || ''
      this.stepForm.stepType = step.stepType || 'api'
      this.stepForm.stepSeq = step.stepSeq != null && step.stepSeq !== '' ? step.stepSeq : null
      this.stepForm.testCaseId = step.testCaseId
      this.stepForm.enabled = step.enabled
      this.applyStepConfigEditor(step.config || {})
      if (this.stepForm.stepType === 'api') {
        this.applyStepRequestEditor(step.testCaseId, step.requestOverride || {})
      }
    },

    editStep(step) {
      if (!step?.id) return
      this.openStepTab(step)
    },

    applyStepConfigEditor(rawConfig) {
      const cfg = this.parseMaybeJSON(rawConfig)
      if (this.stepForm.stepType === 'api') {
        this.stepForm.stepAssertions = Array.isArray(cfg.assertions) ? cfg.assertions : []
      } else if (this.stepForm.stepType === 'database') {
        this.stepForm.dbDataSourceId = cfg.dataSourceId || ''
        this.stepForm.dbSQL = cfg.sql || ''
        this.stepForm.dbInputParamsText = this.formatJSON(cfg.inputParams || [])
        this.stepForm.dbTimeoutMillis = cfg.timeoutMillis || 3000
      } else if (this.stepForm.stepType === 'script') {
        this.stepForm.scriptSource = cfg.script || cfg.command || ''
        this.stepForm.scriptTimeoutMillis = cfg.timeoutMillis || 10000
      } else if (this.stepForm.stepType === 'for') {
        this.stepForm.forMode = cfg.mode || 'count'
        this.stepForm.forCountExpression = cfg.countExpression || String(cfg.count ?? 1)
        this.stepForm.forItemsExpression = cfg.itemsExpression || this.formatJSON(cfg.items || [])
        this.stepForm.forItemVar = cfg.itemVar || 'item'
        this.stepForm.forIndexVar = cfg.indexVar || 'index'
        this.stepForm.forBodyStepSeqs = Array.isArray(cfg.bodyStepSeqs) ? cfg.bodyStepSeqs : []
        this.stepForm.forMaxIterations = cfg.maxIterations || 100
      } else if (this.stepForm.stepType === 'condition') {
        const norm = this.normalizeConditionConfig(cfg)
        this.stepForm.conditionBranches = norm.branches.map((b) => ({
          left: b.left || '',
          operator: b.operator || 'equals',
          right: b.right || '',
          stepSeqs: Array.isArray(b.stepSeqs) ? [...b.stepSeqs] : []
        }))
        this.stepForm.conditionElseStepSeqs = [...norm.elseStepSeqs]
      }
    },

    onStepTypeChange() {
      const order = this.stepForm.stepOrder
      const name = this.stepForm.name
      const enabled = this.stepForm.enabled
      const stepType = this.stepForm.stepType
      this.stepForm = this.emptyStepForm(order)
      this.stepForm.stepType = stepType
      this.stepForm.name = name
      this.stepForm.enabled = enabled
      this.editingStepName = false
      this.stepRunOutput = null
      this.stepRunResponseDetailTab = 'body'
      this.stepRequestBodyViewMode = 'edit'
    },

    startEditStepName() {
      if (
        this.stepForm.stepType === 'api' &&
        this.stepForm.testCaseId &&
        !(this.stepForm.name || '').trim()
      ) {
        this.stepForm.name = this.findCaseName(this.stepForm.testCaseId) || ''
      }
      this.editingStepName = true
      this.$nextTick(() => {
        this.$refs.stepNameInputRef?.focus?.()
      })
    },

    finishEditStepName() {
      this.stepForm.name = (this.stepForm.name || '').trim()
      this.editingStepName = false
    },

    selectPanelStep(step) {
      this.openStepTab(step)
    },

    isPanelStepActive(step) {
      return Boolean(step?.id) && this.activeTabStepId === step.id
    },

    onRequestMethodChange() {
      this.stepForm.requestTab = this.defaultStepRequestTab()
    },

    onStepCaseChange() {
      this.editingStepName = false
      this.stepRunOutput = null
      this.stepRunResultTab = 'response'
      this.stepRunResponseDetailTab = 'body'
      this.stepRequestBodyViewMode = 'edit'
      this.stepForm.name = ''
      this.applyStepRequestEditor(this.stepForm.testCaseId, {})
    },

    onStepApiClear() {
      this.stepForm.testCaseId = ''
      this.onStepCaseChange()
    },

    applyStepRequestEditor(testCaseId, rawOverride) {
      const tc = this.findCase(testCaseId)
      if (!tc) {
        this.stepForm.requestMethod = 'GET'
        this.stepForm.requestPath = ''
        this.stepForm.pathParamRows = []
        this.stepForm.queryRows = []
        this.stepForm.headerRows = []
        this.stepForm.bodyText = ''
        this.stepForm.requestSecurity = undefined
        this.setStepBodyText('')
        return
      }

      const request = this.resolveStepRequest(tc, rawOverride)
      this.stepForm.requestMethod = String(request.method || tc.method || 'GET').toUpperCase()
      this.stepForm.requestPath = this.normalizePathVariables(request.path)
      this.stepForm.pathParamRows = this.buildStepPathRows(this.stepForm.requestPath, request.variables)
      this.stepForm.queryRows = this.buildStepQueryRows(tc, this.normalizeObject(request.query))
      this.stepForm.headerRows = this.buildStepHeaderRows(tc, this.normalizeObject(request.headers))
      this.setStepBodyText(request.body == null ? '' : this.formatStepBodyValue(request.body))
      this.stepForm.requestSecurity = request.security
      this.stepForm.requestTab = this.defaultStepRequestTab()
      this.refreshEnvironmentAuthRows()
    },
    resolveStepRequest(tc, rawOverride) {
      const base = this.normalizeRequest(tc.request)
      const override = this.normalizeRequest(rawOverride)
      const hasOverride = (key) => Object.prototype.hasOwnProperty.call(override, key)
      const request = {
        method: hasOverride('method') ? override.method : (base.method || tc.method),
        path: hasOverride('path') ? override.path : (base.path || tc.path),
        headers: hasOverride('headers') ? this.normalizeObject(override.headers) : this.normalizeObject(base.headers),
        query: hasOverride('query') ? this.normalizeObject(override.query) : this.normalizeObject(base.query),
        variables: hasOverride('variables') ? this.normalizeObject(override.variables) : this.normalizeObject(base.variables),
        body: hasOverride('body') ? override.body : base.body,
        security: hasOverride('security') ? override.security : base.security
      }
      const separated = this.separateRuntimePathValues(request.path, base.path || tc.path, request.variables)
      request.path = separated.path
      request.variables = separated.variables
      return request
    },

    syncPathParamRows() {
      const current = this.rowsToObject(this.stepForm.pathParamRows)
      this.stepForm.pathParamRows = this.buildStepPathRows(this.stepForm.requestPath, current)
      this.refreshEnvironmentAuthRows()
    },

    buildStepPathRows(path, values) {
      const current = this.normalizeObject(values)
      const defaults = this.pathVariables(path)
      const rows = []
      for (const key of Object.keys(defaults)) {
        rows.push({
          enabled: true,
          key,
          value: Object.prototype.hasOwnProperty.call(current, key) ? current[key] : defaults[key]
        })
      }
      return rows
    },

    buildStepRequestOverride() {
      let body = null
      if (this.stepForm.bodyText.trim()) {
        try {
          body = this.parseTemplateAwareJSON(this.stepForm.bodyText, true)
        } catch (error) {
          throw new Error(`Body 不是合法 JSON：${error.message}`)
        }
      }
      const path = this.normalizePathVariables(this.stepForm.requestPath || this.findCasePath(this.stepForm.testCaseId))
      const tc = this.findCase(this.stepForm.testCaseId)
      const baseRequest = this.normalizeRequest(tc?.request)
      const request = {
        method: this.stepForm.requestMethod || this.findCaseMethod(this.stepForm.testCaseId),
        path,
        headers: this.rowsToObject(this.stepForm.headerRows),
        query: this.rowsToObject(this.stepForm.queryRows),
        variables: {
          ...this.normalizeObject(baseRequest.variables),
          ...this.rowsToObject(this.stepForm.pathParamRows)
        },
        body
      }
      if (this.stepForm.requestSecurity !== undefined) {
        request.security = this.cloneSample(this.stepForm.requestSecurity)
      }
      return request
    },

    formatStepBody() {
      if (!this.stepForm.bodyText.trim()) return
      try {
        this.setStepBodyText(this.formatTemplateAwareJSON(this.stepForm.bodyText))
      } catch (error) {
        this.$message.error(`Body 不是合法 JSON：${error.message}`)
      }
    },

    addRow(rows) {
      rows.push({ enabled: true, key: '', value: '', required: false })
    },

    removeRow(rows, index) {
      const removed = rows.splice(index, 1)
      if (removed[0]?.authInherited) this.refreshEnvironmentAuthRows()
    },

    objectToRows(value, enabledMap, requiredMap) {
      return Object.entries(value || {}).map(([key, item]) => ({
        enabled: enabledMap ? enabledMap[key] !== false : true,
        key,
        value: String(item),
        required: requiredMap ? requiredMap[key] === true : false
      }))
    },

    rowsToObject(rows, options = {}) {
      const includeInherited = options.includeInherited === true
      return (rows || []).reduce((out, row) => {
        if (row.authInherited && !row.authOverridden && !includeInherited) return out
        if (row.enabled !== false && row.key && row.key.trim()) out[row.key.trim()] = row.value
        return out
      }, {})
    },

    markRowOverridden(row) {
      if (row && row.authInherited) {
        row.authOverridden = true
      }
    },

    normalizeObject(value) {
      return value && typeof value === 'object' && !Array.isArray(value) ? value : {}
    },

    normalizeRequest(request) {
      if (!request) return {}
      if (typeof request === 'string') {
        try {
          return JSON.parse(request)
        } catch {
          return {}
        }
      }
      try {
        return JSON.parse(JSON.stringify(request))
      } catch {
        return {}
      }
    },

    isPathVariableName(name) {
      return /^[A-Za-z_][A-Za-z0-9_-]*$/.test(String(name || '').trim())
    },

    normalizePathVariables(path) {
      return String(path || '')
        .replace(/\{+\s*(\$[^{}]+?)\s*\}+/g, (_, expr) => `{{${expr.trim()}}}`)
        .replace(/\{\{+([^{}]+)\}\}+/g, (match, name) => {
          const trimmed = String(name || '').trim()
          return this.isPathVariableName(trimmed) ? `{${trimmed}}` : match
        })
    },

    separateRuntimePathValues(path, templatePath, variables) {
      const normalizedPath = this.normalizePathVariables(path)
      const normalizedTemplate = this.normalizePathVariables(templatePath)
      const values = { ...this.normalizeObject(variables) }
      const pathParts = normalizedPath.split('/')
      const templateParts = normalizedTemplate.split('/')
      if (pathParts.length !== templateParts.length) {
        return { path: normalizedPath, variables: values }
      }
      let changed = false
      for (let idx = 0; idx < templateParts.length; idx += 1) {
        const match = templateParts[idx].match(/^\{([A-Za-z_][A-Za-z0-9_-]*)\}$/)
        if (!match || pathParts[idx] === templateParts[idx]) continue
        if (!Object.prototype.hasOwnProperty.call(values, match[1])) {
          values[match[1]] = pathParts[idx]
        }
        changed = true
      }
      return { path: changed ? normalizedTemplate : normalizedPath, variables: values }
    },

    pathForRun(path) {
      return this.normalizePathVariables(path).replace(/(^|[^{])\{([^{}]+)\}(?!\})/g, (match, prefix, name) => {
        const trimmed = String(name || '').trim()
        return this.isPathVariableName(trimmed) ? `${prefix}{{${trimmed}}}` : match
      })
    },

    pathVariables(path) {
      const variables = {}
      const source = this.normalizePathVariables(path)
      for (const match of source.matchAll(/(^|[^{])\{([^{}]+)\}(?!\})/g)) {
        const name = String(match[2] || '').trim()
        if (this.isPathVariableName(name)) variables[name] = '1'
      }
      return variables
    },

    defaultStepRequestTab() {
      const method = String(this.stepForm.requestMethod || '').toUpperCase()
      if (method === 'POST' || method === 'PUT' || method === 'PATCH') return 'body'
      return 'params'
    },

    formatJSON(value) {
      return JSON.stringify(value, null, 2)
    },

    formatStepBodyValue(value) {
      const prepared = this.prepareBareTemplateMarkersForFormatting(value)
      let formatted = this.formatJSON(prepared.value)
      for (const item of prepared.placeholders) {
        formatted = formatted.split(JSON.stringify(item.placeholder)).join(item.token)
      }
      return formatted
    },

    parseTemplateAwareJSON(text, markBareTemplates = false) {
      const source = String(text || '')
      try {
        return JSON.parse(source)
      } catch (nativeError) {
        const prepared = this.prepareBareTemplateJSON(source)
        if (!prepared.changed) throw nativeError
        return this.restoreBareTemplateJSONValue(JSON.parse(prepared.text), prepared.placeholders, markBareTemplates)
      }
    },

    formatTemplateAwareJSON(text) {
      const source = String(text || '')
      const prepared = this.prepareBareTemplateJSON(source)
      const parsed = JSON.parse(prepared.text)
      let formatted = this.formatJSON(parsed)
      for (const item of prepared.placeholders) {
        formatted = formatted.split(JSON.stringify(item.placeholder)).join(item.token)
      }
      return formatted
    },

    prepareBareTemplateJSON(text) {
      const placeholders = []
      let out = ''
      let i = 0
      let inString = false
      let escaping = false
      while (i < text.length) {
        const ch = text[i]
        if (inString) {
          out += ch
          if (escaping) {
            escaping = false
          } else if (ch === '\\') {
            escaping = true
          } else if (ch === '"') {
            inString = false
          }
          i += 1
          continue
        }
        if (ch === '"') {
          inString = true
          out += ch
          i += 1
          continue
        }
        if (ch === '{' && text[i + 1] === '{') {
          const end = text.indexOf('}}', i + 2)
          if (end !== -1) {
            const token = text.slice(i, end + 2)
            const placeholder = `__AUTOTEST_BARE_TEMPLATE_${placeholders.length}__`
            placeholders.push({ placeholder, token })
            out += JSON.stringify(placeholder)
            i = end + 2
            continue
          }
        }
        out += ch
        i += 1
      }
      return { text: out, placeholders, changed: placeholders.length > 0 }
    },

    restoreBareTemplateJSONValue(value, placeholders, markBareTemplates = false) {
      if (!placeholders.length || value == null) return value
      if (typeof value === 'string') {
        const hit = placeholders.find((item) => item.placeholder === value)
        if (!hit) return value
        return markBareTemplates ? { __autotestBareTemplate: hit.token } : hit.token
      }
      if (Array.isArray(value)) {
        return value.map((item) => this.restoreBareTemplateJSONValue(item, placeholders, markBareTemplates))
      }
      if (typeof value === 'object') {
        const out = {}
        for (const [key, item] of Object.entries(value)) {
          out[key] = this.restoreBareTemplateJSONValue(item, placeholders, markBareTemplates)
        }
        return out
      }
      return value
    },

    prepareBareTemplateMarkersForFormatting(value) {
      const placeholders = []
      const walk = (item) => {
        if (item == null || typeof item !== 'object') return item
        if (!Array.isArray(item) && Object.keys(item).length === 1 && typeof item.__autotestBareTemplate === 'string') {
          const placeholder = `__AUTOTEST_FORMAT_BARE_TEMPLATE_${placeholders.length}__`
          placeholders.push({ placeholder, token: item.__autotestBareTemplate })
          return placeholder
        }
        if (Array.isArray(item)) return item.map(walk)
        const out = {}
        for (const [key, child] of Object.entries(item)) {
          out[key] = walk(child)
        }
        return out
      }
      return { value: walk(value), placeholders }
    },

    cloneSample(value) {
      if (value == null || typeof value !== 'object') return value
      return JSON.parse(JSON.stringify(value))
    },

    findEndpoint(endpoints, row) {
      return (endpoints || []).find((endpoint) => endpoint.id === row.endpointId) ||
        (endpoints || []).find((endpoint) => endpoint.method === row.method && endpoint.path === row.path) ||
        null
    },

    buildStepQueryRows(tc, flatQueryOverride) {
      const ep = this.findEndpoint(this.serviceEndpoints, tc)
      const sg = this.requestFromSchema(ep?.requestSchema)
      const saved = this.normalizeRequest(tc.request || {})
      const savedQueryEnabled = this.normalizeObject(saved.queryEnabled || {})
      const savedQueryRequired = this.normalizeObject(saved.queryRequired || {})
      const mergedQuery = { ...this.normalizeObject(sg.query), ...this.normalizeObject(flatQueryOverride) }
      const queryRequired = {}
      const queryEnabled = {}
      for (const key of Object.keys(mergedQuery)) {
        if (Object.prototype.hasOwnProperty.call(savedQueryRequired, key)) {
          queryRequired[key] = savedQueryRequired[key] === true
        } else {
          queryRequired[key] = sg.queryRequiredMap[key] === true
        }
        queryEnabled[key] =
          sg.queryRequired.has(key) ||
          savedQueryEnabled[key] === true ||
          Object.prototype.hasOwnProperty.call(this.normalizeObject(flatQueryOverride), key)
      }
      return this.objectToRows(mergedQuery, queryEnabled, queryRequired)
    },

    buildStepHeaderRows(tc, hdrOverride) {
      const ep = this.findEndpoint(this.serviceEndpoints, tc)
      const sg = this.requestFromSchema(ep?.requestSchema)
      const merged = { ...this.normalizeObject(sg.headers), ...this.normalizeObject(hdrOverride) }
      return this.objectToRows(merged)
    },

    refreshEnvironmentAuthRows() {
      if (!this.stepForm || this.stepForm.stepType !== 'api') return
      this.stepForm.headerRows = this.mergeInheritedAuthRows(
        this.stepForm.headerRows,
        this.environmentAuthRequestRows().headers,
        true
      )
      this.stepForm.queryRows = this.mergeInheritedAuthRows(
        this.stepForm.queryRows,
        this.environmentAuthRequestRows().query,
        false
      )
    },

    mergeInheritedAuthRows(currentRows, inheritedRows, headerMode) {
      const manualRows = (currentRows || []).filter((row) => !row.authInherited || row.authOverridden)
      const exists = (key) => {
        const target = headerMode ? String(key).toLowerCase() : String(key)
        return manualRows.some((row) => {
          if (row.enabled === false) return false
          const rowKey = headerMode ? String(row.key || '').toLowerCase() : String(row.key || '')
          return rowKey === target
        })
      }
      const additions = (inheritedRows || [])
        .filter((row) => row.key && !exists(row.key))
        .map((row) => ({
          enabled: true,
          required: false,
          ...row,
          authInherited: true,
          authOverridden: false
        }))
      return [...manualRows, ...additions]
    },

    environmentAuthRequestRows() {
      const auth = this.selectEnvironmentAuthDefinition()
      if (!auth) return { headers: [], query: [] }

      const headers = []
      const query = []
      const headerKeys = new Set()
      const queryKeys = new Set()
      const addHeader = (key, value) => {
        const name = String(key || '').trim()
        const normalized = name.toLowerCase()
        if (name && !headerKeys.has(normalized)) {
          headerKeys.add(normalized)
          headers.push({ key: name, value: String(value ?? '') })
        }
      }
      const addQuery = (key, value) => {
        const name = String(key || '').trim()
        if (name && !queryKeys.has(name)) {
          queryKeys.add(name)
          query.push({ key: name, value: String(value ?? '') })
        }
      }

      for (const [key, value] of Object.entries(this.normalizeObject(auth.headers))) {
        addHeader(key, value)
      }
      for (const [key, value] of Object.entries(this.normalizeObject(auth.query))) {
        addQuery(key, value)
      }

      const type = String(auth.type || '').trim().toLowerCase()
      if (['bearer', 'jwt', 'oauth2'].includes(type)) {
        const tokenTemplate = auth.token || auth.value || ''
        const token = String(tokenTemplate).trim()
        if (token) addHeader('Authorization', token.toLowerCase().startsWith('bearer ') ? token : `Bearer ${token}`)
      } else if (type === 'basic') {
        const username = auth.username || '{{username}}'
        const password = auth.password || '{{password}}'
        addHeader('Authorization', `Basic ${username}:${password}`)
      } else if (['api_key', 'apikey', 'api-key'].includes(type)) {
        const name = auth.name || ''
        const value = auth.value || auth.token || ''
        if (String(auth.in || '').toLowerCase() === 'query') addQuery(name, value)
        else addHeader(name, value)
      } else if (type === 'header') {
        addHeader(auth.name, auth.value)
      } else if (type === 'query') {
        addQuery(auth.name, auth.value)
      }
      return { headers, query }
    },

    selectEnvironmentAuthDefinition() {
      const env = this.environments.find((item) => String(item.id) === String(this.runEnvId))
      const rawAuth = this.parseEnvironmentAuth(env?.auth)
      if (!rawAuth || !Object.keys(rawAuth).length) return null

      const profiles = this.normalizeObject(rawAuth.profiles)
      if (!Object.keys(profiles).length) {
        return this.hasStepSecurityRequirements() ? rawAuth : null
      }

      const bySecurity = this.profileNameForStepSecurity(rawAuth, profiles)
      if (bySecurity) return this.normalizeObject(profiles[bySecurity])

      const byPath = this.profileNameForEnvironmentPath(rawAuth, profiles)
      if (byPath) return this.normalizeObject(profiles[byPath])

      const fallback = String(rawAuth.defaultProfile || '').trim()
      return fallback && profiles[fallback] ? this.normalizeObject(profiles[fallback]) : null
    },

    parseEnvironmentAuth(raw) {
      if (!raw) return {}
      if (typeof raw === 'object' && !Array.isArray(raw)) return raw
      if (typeof raw === 'string') {
        try {
          const parsed = JSON.parse(raw)
          return this.normalizeObject(parsed)
        } catch {
          return {}
        }
      }
      return {}
    },

    hasStepSecurityRequirements() {
      const security = this.stepForm?.requestSecurity
      return Array.isArray(security) && security.some((requirement) => (
        requirement && typeof requirement === 'object' && Object.keys(requirement).length > 0
      ))
    },

    profileNameForStepSecurity(authConfig, profiles) {
      const security = Array.isArray(this.stepForm?.requestSecurity) ? this.stepForm.requestSecurity : []
      const securitySchemes = this.normalizeObject(authConfig.securitySchemes)
      for (const requirement of security) {
        if (!requirement || typeof requirement !== 'object') continue
        for (const schemeName of Object.keys(requirement)) {
          const scheme = String(schemeName || '').trim()
          if (!scheme) continue
          const mapped = String(securitySchemes[scheme] || '').trim()
          if (mapped && profiles[mapped]) return mapped
          if (profiles[scheme]) return scheme
        }
      }
      return ''
    },

    profileNameForEnvironmentPath(authConfig, profiles) {
      const rules = Array.isArray(authConfig.pathRules) ? authConfig.pathRules : []
      const paths = [
        this.normalizePathVariables(this.stepForm?.requestPath || ''),
        this.normalizePathVariables(this.findCasePath(this.stepForm?.testCaseId) || '')
      ]
      let best = ''
      let bestLen = -1
      for (const rule of rules) {
        const profile = String(rule?.profile || '').trim()
        if (!profile || !profiles[profile]) continue
        for (const rawPrefix of [rule.prefix, rule.path]) {
          const prefix = this.normalizeAuthPath(rawPrefix)
          if (!prefix) continue
          for (const path of paths) {
            if (this.pathMatchesAuthPrefix(this.normalizeAuthPath(path), prefix) && prefix.length > bestLen) {
              best = profile
              bestLen = prefix.length
            }
          }
        }
      }
      return best
    },

    normalizeAuthPath(path) {
      const trimmed = String(path || '').trim()
      if (!trimmed) return ''
      return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
    },

    pathMatchesAuthPrefix(path, prefix) {
      if (!path || !prefix) return false
      return path === prefix || path.startsWith(`${prefix.replace(/\/+$/, '')}/`)
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

    parameterCommentForStep(row, location) {
      if (row?.authInherited && !row?.authOverridden) {
        return '继承自当前环境认证；修改后会保存为步骤覆盖，支持模板变量。'
      }
      const name = row?.key
      if (!name) return '-'
      const parameters = Array.isArray(this.scenarioStepRequestSchema?.parameters)
        ? this.scenarioStepRequestSchema.parameters
        : []
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
      rows.forEach((r) => {
        byPath[r.path] = r
      })
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
        if (
          Object.keys(props).length > 0 ||
          (node.additionalProperties && typeof node.additionalProperties === 'object')
        ) {
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
        constraints: this.schemaFieldConstraints(node),
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
          this.collectSchemaDocRows(
            node.additionalProperties,
            `${path}.{key}`,
            '{key}',
            path,
            depth + 1,
            false,
            rows
          )
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
            this.collectSchemaDocRows(
              itemsNode.additionalProperties,
              `${path}[].{key}`,
              '{key}',
              path,
              depth + 1,
              false,
              rows
            )
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

      const alternatives =
        Array.isArray(out.oneOf) && out.oneOf.length > 0
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

    schemaFieldConstraints(schema) {
      const node = this.normalizeSchemaNode(schema)
      const constraints = []
      const sources = [
        node,
        node.validation,
        node.validations,
        node.constraints,
        node.rules,
        node['x-validation'],
        node['x-validations'],
        node['x-constraints'],
        node['x-rules']
      ].filter((item) => item && typeof item === 'object' && !Array.isArray(item))
      const seen = new Set()
      const findValue = (aliases) => {
        for (const source of sources) {
          for (const key of aliases) {
            if (Object.prototype.hasOwnProperty.call(source, key)) {
              return { exists: true, value: source[key] }
            }
          }
        }
        return { exists: false, value: undefined }
      }
      const valueText = (value) => {
        if (typeof value === 'string') return value
        if (Array.isArray(value)) return value.map(valueText).join('|')
        if (value === null) return 'null'
        return String(value)
      }
      const push = (text) => {
        if (!text || seen.has(text)) return
        seen.add(text)
        constraints.push(text)
      }
      const bound = (aliases, symbol) => {
        const found = findValue(aliases)
        if (found.exists) push(`${symbol}${valueText(found.value)}`)
      }
      const rangeBound = (minAliases, exclusiveMinAliases, maxAliases, exclusiveMaxAliases) => {
        const min = findValue(minAliases)
        const max = findValue(maxAliases)
        const exclusiveMin = findValue(exclusiveMinAliases)
        const exclusiveMax = findValue(exclusiveMaxAliases)

        if (typeof exclusiveMin.value === 'number') {
          push(`>${valueText(exclusiveMin.value)}`)
        } else if (exclusiveMin.value === true && min.exists) {
          push(`>${valueText(min.value)}`)
        } else if (min.exists) {
          push(`≥${valueText(min.value)}`)
        }

        if (typeof exclusiveMax.value === 'number') {
          push(`<${valueText(exclusiveMax.value)}`)
        } else if (exclusiveMax.value === true && max.exists) {
          push(`<${valueText(max.value)}`)
        } else if (max.exists) {
          push(`≤${valueText(max.value)}`)
        }
      }
      const genericMinMax = () => {
        const min = findValue(['min', 'x-min'])
        const max = findValue(['max', 'x-max'])
        if (!min.exists && !max.exists) return
        const type = this.schemaType(node).split('|')[0]
        if (type === 'string') {
          if (min.exists) push(`len≥${valueText(min.value)}`)
          if (max.exists) push(`len≤${valueText(max.value)}`)
        } else if (type === 'array') {
          if (min.exists) push(`items≥${valueText(min.value)}`)
          if (max.exists) push(`items≤${valueText(max.value)}`)
        } else if (type === 'object') {
          if (min.exists) push(`props≥${valueText(min.value)}`)
          if (max.exists) push(`props≤${valueText(max.value)}`)
        } else {
          if (min.exists) push(`≥${valueText(min.value)}`)
          if (max.exists) push(`≤${valueText(max.value)}`)
        }
      }

      if (Array.isArray(node.enum) && node.enum.length > 0) {
        push(`∈ ${node.enum.map(valueText).join('|')}`)
      }
      bound(['const', 'constant', 'x-const'], '=')

      const minLength = findValue(['minLength', 'minlength', 'minLen', 'min_len', 'x-minLength', 'x-min-length'])
      const maxLength = findValue(['maxLength', 'maxlength', 'maxLen', 'max_len', 'x-maxLength', 'x-max-length'])
      const exactLength = findValue(['length', 'len', 'exactLength', 'x-length'])
      if (exactLength.exists) {
        push(`len=${valueText(exactLength.value)}`)
      } else if (minLength.exists && maxLength.exists && minLength.value === maxLength.value) {
        push(`len=${valueText(minLength.value)}`)
      } else {
        if (minLength.exists) push(`len≥${valueText(minLength.value)}`)
        if (maxLength.exists) push(`len≤${valueText(maxLength.value)}`)
      }

      rangeBound(
        ['minimum', 'minValue', 'x-minimum'],
        ['exclusiveMinimum', 'exclusiveMin', 'x-exclusiveMinimum'],
        ['maximum', 'maxValue', 'x-maximum'],
        ['exclusiveMaximum', 'exclusiveMax', 'x-exclusiveMaximum']
      )
      genericMinMax()
      bound(['multipleOf', 'multiple', 'x-multipleOf'], '×')

      const minItems = findValue(['minItems', 'minitems', 'min_items', 'minSize', 'x-minItems'])
      const maxItems = findValue(['maxItems', 'maxitems', 'max_items', 'maxSize', 'x-maxItems'])
      if (minItems.exists && maxItems.exists && minItems.value === maxItems.value) {
        push(`items=${valueText(minItems.value)}`)
      } else {
        if (minItems.exists) push(`items≥${valueText(minItems.value)}`)
        if (maxItems.exists) push(`items≤${valueText(maxItems.value)}`)
      }
      if (findValue(['uniqueItems', 'unique', 'x-uniqueItems']).value === true) push('unique')
      bound(['minProperties', 'minProps', 'x-minProperties'], 'props≥')
      bound(['maxProperties', 'maxProps', 'x-maxProperties'], 'props≤')

      if (typeof node.pattern === 'string' && node.pattern.trim()) {
        push(`/${node.pattern.trim()}/`)
      }
      const pattern = findValue(['regex', 'regexp', 'matches', 'x-pattern'])
      if (typeof pattern.value === 'string' && pattern.value.trim()) push(`/${pattern.value.trim()}/`)
      if (findValue(['nullable', 'x-nullable']).value === true) push('null?')
      if (findValue(['allowEmptyValue', 'allowEmpty', 'x-allowEmptyValue']).value === true) push('empty?')
      if (findValue(['readOnly', 'readonly']).value === true) push('readOnly')
      if (findValue(['writeOnly', 'writeonly']).value === true) push('writeOnly')
      if (findValue(['deprecated', 'x-deprecated']).value === true) push('deprecated')

      return constraints.join(' · ')
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
        lines.push({
          id: path,
          type: isArray ? 'empty-array' : 'empty-object',
          depth,
          key: keyName,
          hasComma: !isLast
        })
        return
      }
      if (collapsed[path]) {
        lines.push({
          id: path,
          type: 'collapsed',
          depth,
          key: keyName,
          open: isArray ? '[' : '{',
          close: isArray ? ']' : '}',
          count,
          path,
          hasComma: !isLast
        })
        return
      }
      lines.push({
        id: `${path}:o`,
        type: 'open',
        depth,
        key: keyName,
        bracket: isArray ? '[' : '{',
        path
      })
      if (isArray) {
        value.forEach((item, i) => {
          this.collectJsonLines(
            item,
            null,
            `${path}[${i}]`,
            depth + 1,
            i === value.length - 1,
            collapsed,
            lines
          )
        })
      } else {
        keys.forEach((k, i) => {
          this.collectJsonLines(
            value[k],
            k,
            `${path}.${k}`,
            depth + 1,
            i === keys.length - 1,
            collapsed,
            lines
          )
        })
      }
      lines.push({
        id: `${path}:c`,
        type: 'close',
        depth,
        bracket: isArray ? ']' : '}',
        hasComma: !isLast
      })
    },

    formatJsonPrimitive(value) {
      if (value === null) return 'null'
      if (typeof value === 'string') return JSON.stringify(value)
      return String(value)
    },

    startStepBodySchemaResize(event) {
      if (event.button !== undefined && event.button !== 0) return
      const layout = event.currentTarget?.closest?.('.body-schema-layout')
      if (!layout) return
      const rect = layout.getBoundingClientRect()
      this.stepBodySchemaResizeState = {
        right: rect.right,
        maxWidth: Math.max(
          BODY_SCHEMA_PANEL_MIN_WIDTH,
          Math.min(BODY_SCHEMA_PANEL_MAX_WIDTH, rect.width - BODY_SCHEMA_EDITOR_MIN_WIDTH)
        )
      }
      document.addEventListener('pointermove', this.onStepBodySchemaResize)
      document.addEventListener('pointerup', this.stopStepBodySchemaResize)
      document.addEventListener('pointercancel', this.stopStepBodySchemaResize)
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      event.currentTarget.setPointerCapture?.(event.pointerId)
      event.preventDefault()
      this.onStepBodySchemaResize(event)
    },

    onStepBodySchemaResize(event) {
      if (!this.stepBodySchemaResizeState) return
      const nextWidth = this.stepBodySchemaResizeState.right - event.clientX
      this.stepBodySchemaPanelWidth = clampNumber(
        nextWidth,
        BODY_SCHEMA_PANEL_MIN_WIDTH,
        this.stepBodySchemaResizeState.maxWidth
      )
    },

    stopStepBodySchemaResize() {
      if (!this.stepBodySchemaResizeState) return
      document.removeEventListener('pointermove', this.onStepBodySchemaResize)
      document.removeEventListener('pointerup', this.stopStepBodySchemaResize)
      document.removeEventListener('pointercancel', this.stopStepBodySchemaResize)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      storeScenarioBodySchemaPanelWidth(this.stepBodySchemaPanelWidth)
      this.stepBodySchemaResizeState = null
    },

    setStepBodyText(text) {
      this.stepForm.bodyText = text
    },

    stepDebugMetricClass(status) {
      if (status === 'passed') return 'is-success'
      if (status === 'failed' || status === 'error') return 'is-danger'
      if (status) return 'is-warning'
      return 'is-idle'
    },

    stepDebugStatusIconName(status) {
      if (status === 'passed') return 'CircleCheck'
      if (status === 'failed' || status === 'error') return 'CircleClose'
      if (status) return 'Warning'
      return 'Clock'
    },

    stepDebugStatusLabel(status) {
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

    assertionTypeColor(type) {
      return ASSERTION_TYPE_COLORS[type] || ''
    },

    stepDebugResponseStatusTagType(code) {
      const c = Number(code)
      if (!Number.isFinite(c)) return 'info'
      if (c >= 200 && c < 300) return 'success'
      if (c >= 400) return 'danger'
      return 'warning'
    },

    parseJSONText(text, fallback, label) {
      const raw = String(text || '').trim()
      if (!raw) return fallback
      try {
        return JSON.parse(raw)
      } catch (error) {
        throw new Error(`${label} 不是合法 JSON：${error.message}`)
      }
    },

    appendScenarioScriptTemplate(code) {
      const cur = this.stepForm.scriptSource || ''
      this.stepForm.scriptSource = cur.trim() ? `${cur.trim()}\n\n${code}` : code
    },

    openScenarioScriptAI() {
      this.aiScenarioScriptLoading = true
      this.aiScenarioScriptDialogVisible = true
    },

    async generateStepParams() {
      if (!this.stepForm.testCaseId) {
        this.$message.warning('请先选择接口')
        return
      }
      const tc = this.findCase(this.stepForm.testCaseId)
      if (!tc) return
      this.generatingStepParams = true
      try {
        const generated = await generateCaseParams(this.stepForm.testCaseId)
        const pathTpl = this.normalizePathVariables(this.stepForm.requestPath || tc.path)
        this.stepForm.requestPath = pathTpl
        const curPath = this.rowsToObject(this.stepForm.pathParamRows)
        const genPath = this.normalizeObject(generated.path)
        const mergedPathVars = { ...curPath, ...genPath }
        const defaults = this.pathVariables(pathTpl)
        const pathRows = []
        for (const key of Object.keys(defaults)) {
          pathRows.push({
            enabled: true,
            key,
            value: Object.prototype.hasOwnProperty.call(mergedPathVars, key)
              ? String(mergedPathVars[key])
              : defaults[key]
          })
        }
        this.stepForm.pathParamRows = pathRows

        const curQuery = this.rowsToObject(this.stepForm.queryRows)
        const mergedQuery = { ...curQuery, ...this.normalizeObject(generated.query) }
        this.stepForm.queryRows = this.buildStepQueryRows(tc, mergedQuery)

        const curHeaders = this.rowsToObject(this.stepForm.headerRows)
        const mergedHeaders = { ...curHeaders, ...this.normalizeObject(generated.headers) }
        this.stepForm.headerRows = this.buildStepHeaderRows(tc, mergedHeaders)
        this.refreshEnvironmentAuthRows()

        if (generated.body != null) {
          this.setStepBodyText(this.formatStepBodyValue(generated.body))
        }
        this.$message.success('已重新生成参数')
      } catch {
        /* 错误由请求拦截器提示 */
      } finally {
        this.generatingStepParams = false
      }
    },

    openStepAIParamsDialog() {
      if (!this.stepForm.testCaseId) {
        this.$message.warning('请先选择接口')
        return
      }
      this.aiStepParamsLoading = true
      this.aiStepParamsDialogVisible = true
    },

    onScenarioStepAIParamsApply(payload) {
      const generated = payload?.parsed
      if (!generated || typeof generated !== 'object' || Array.isArray(generated)) {
        this.$message.warning('AI 未返回结构化参数，请查看原文')
        return
      }
      const tc = this.findCase(this.stepForm.testCaseId)
      if (!tc) return
      const aiHeaders =
        generated.headers && typeof generated.headers === 'object' && !Array.isArray(generated.headers)
          ? generated.headers
          : {}
      const aiQuery =
        generated.query && typeof generated.query === 'object' && !Array.isArray(generated.query)
          ? generated.query
          : {}
      const aiPath =
        generated.path && typeof generated.path === 'object' && !Array.isArray(generated.path)
          ? generated.path
          : generated.pathvar || generated.pathVars || {}
      const aiBody = Object.prototype.hasOwnProperty.call(generated, 'body') ? generated.body : null

      const currentQuery = this.rowsToObject(this.stepForm.queryRows)
      const currentHeaders = this.rowsToObject(this.stepForm.headerRows)
      const currentPathVars = this.rowsToObject(this.stepForm.pathParamRows)

      const mergedHeaders = { ...currentHeaders }
      for (const [k, v] of Object.entries(aiHeaders)) {
        mergedHeaders[k] = typeof v === 'string' ? v : JSON.stringify(v)
      }

      const mergedQuery = { ...currentQuery }
      for (const [k, v] of Object.entries(aiQuery)) {
        mergedQuery[k] = typeof v === 'string' ? v : JSON.stringify(v)
      }

      const mergedPathVars = { ...currentPathVars }
      for (const [k, v] of Object.entries(aiPath || {})) {
        mergedPathVars[k] = typeof v === 'string' ? v : JSON.stringify(v)
      }

      const pathTpl = this.normalizePathVariables(this.stepForm.requestPath || tc.path)
      this.stepForm.requestPath = pathTpl
      const defaults = this.pathVariables(pathTpl)
      const pathRows = []
      for (const key of Object.keys(defaults)) {
        pathRows.push({
          enabled: true,
          key,
          value: Object.prototype.hasOwnProperty.call(mergedPathVars, key)
            ? String(mergedPathVars[key])
            : defaults[key]
        })
      }
      this.stepForm.pathParamRows = pathRows

      this.stepForm.headerRows = this.buildStepHeaderRows(tc, mergedHeaders)
      this.stepForm.queryRows = this.buildStepQueryRows(tc, mergedQuery)
      this.refreshEnvironmentAuthRows()

      let bodyDraft = null
      if ((this.stepForm.bodyText || '').trim()) {
        try {
          bodyDraft = this.parseTemplateAwareJSON(this.stepForm.bodyText)
        } catch {
          bodyDraft = this.stepForm.bodyText
        }
      }
      const nextBody = aiBody !== null && aiBody !== undefined ? aiBody : bodyDraft
      if (nextBody == null || nextBody === '') {
        this.setStepBodyText('')
      } else if (typeof nextBody === 'string') {
        this.setStepBodyText(nextBody)
      } else {
        this.setStepBodyText(this.formatStepBodyValue(nextBody))
      }

      if (nextBody != null && nextBody !== '' && !mergedHeaders['Content-Type']) {
        mergedHeaders['Content-Type'] = 'application/json'
        this.stepForm.headerRows = this.buildStepHeaderRows(tc, mergedHeaders)
        this.refreshEnvironmentAuthRows()
      }

      this.$message.success('已应用 AI 生成的参数')
    },

    onScenarioScriptAIApply(payload) {
      const code = (payload?.text || '').trim()
      if (!code) return
      this.appendScenarioScriptTemplate(code)
    },

    buildAPIStepConfig() {
      const assertions = Array.isArray(this.stepForm.stepAssertions) ? this.stepForm.stepAssertions : []
      return assertions.length ? { assertions } : {}
    },

    buildDatabaseStepConfig() {
      if (!this.stepForm.dbDataSourceId) {
        throw new Error('请选择数据库步骤的数据源')
      }
      if (!this.stepForm.dbSQL.trim()) {
        throw new Error('请输入数据库步骤 SQL')
      }
      return {
        dataSourceId: this.stepForm.dbDataSourceId,
        sql: this.stepForm.dbSQL,
        inputParams: this.parseJSONText(this.stepForm.dbInputParamsText, [], '入参 JSON'),
        timeoutMillis: this.stepForm.dbTimeoutMillis || 3000
      }
    },

    buildScriptStepConfig() {
      if (!(this.stepForm.scriptSource || '').trim()) {
        throw new Error('请输入脚本内容')
      }
      return {
        script: this.stepForm.scriptSource,
        timeoutMillis: this.stepForm.scriptTimeoutMillis || 10000
      }
    },

    buildForStepConfig() {
      const formSeqs = this.normalizeStepSeqSelection(this.stepForm.forBodyStepSeqs)
      let mergedSeqs = formSeqs
      if (this.editingStep?.id) {
        const row = this.steps.find((s) => s.id === this.editingStep.id)
        if (row && row.stepType === 'for') {
          const persisted = this.normalizeStepSeqSelection(this.parseMaybeJSON(row.config).bodyStepSeqs)
          mergedSeqs = this.normalizeStepSeqSelection([...persisted, ...formSeqs])
        }
      }
      const bodyStepSeqs = mergedSeqs
      const cfg = {
        mode: this.stepForm.forMode || 'count',
        itemVar: (this.stepForm.forItemVar || 'item').trim(),
        indexVar: (this.stepForm.forIndexVar || 'index').trim(),
        bodyStepSeqs,
        maxIterations: this.stepForm.forMaxIterations || 100
      }
      if (cfg.mode === 'array') {
        if (!(this.stepForm.forItemsExpression || '').trim()) {
          throw new Error('请输入数组表达式')
        }
        cfg.itemsExpression = this.stepForm.forItemsExpression
      } else {
        if (!(this.stepForm.forCountExpression || '').trim()) {
          throw new Error('请输入循环次数')
        }
        cfg.countExpression = this.stepForm.forCountExpression
      }
      return cfg
    },

    buildConditionStepConfig() {
      let persistedNorm = null
      if (this.editingStep?.id) {
        const row = this.steps.find((s) => s.id === this.editingStep.id)
        if (row && row.stepType === 'condition') {
          persistedNorm = this.normalizeConditionConfig(this.parseMaybeJSON(row.config))
        }
      }
      const branches = (this.stepForm.conditionBranches || []).map((b, i) => {
        const formSeqs = this.normalizeStepSeqSelection(b.stepSeqs)
        const diskSeqs = persistedNorm?.branches?.[i]
          ? this.normalizeStepSeqSelection(persistedNorm.branches[i].stepSeqs)
          : []
        return {
          left: (b.left || '').trim(),
          operator: b.operator || 'equals',
          right: (b.right || '').trim(),
          stepSeqs: this.normalizeStepSeqSelection([...diskSeqs, ...formSeqs])
        }
      })
      if (!branches.length) {
        throw new Error('至少保留一个条件分支')
      }
      for (let i = 0; i < branches.length; i++) {
        if (!branches[i].left) {
          throw new Error(`分支 ${i + 1} 的条件左值不能为空`)
        }
      }
      const elseForm = this.normalizeStepSeqSelection(this.stepForm.conditionElseStepSeqs)
      const elseDisk = persistedNorm ? this.normalizeStepSeqSelection(persistedNorm.elseStepSeqs) : []
      return {
        branches,
        elseStepSeqs: this.normalizeStepSeqSelection([...elseDisk, ...elseForm])
      }
    },

    normalizeStepSeqSelection(values) {
      const seen = new Set()
      return (values || [])
        .map((v) => Number(v))
        .filter((n) => Number.isFinite(n) && n > 0 && !seen.has(n) && seen.add(n))
    },

    buildAttachUnderParentPayload() {
      if (!this.controlFlowChildAttach) return undefined
      if (this.editingStep?.id) return undefined
      const a = this.controlFlowChildAttach
      const out = { parentStepId: a.parentId }
      if (a.branch === 'body') {
        out.forLoopBody = true
      } else if (a.branch === 'else') {
        out.conditionElse = true
      } else if (typeof a.branch === 'number') {
        out.conditionBranchIndex = a.branch
      } else {
        return undefined
      }
      return out
    },

    buildStepPayload() {
      if (this.stepForm.stepType === 'api' && !this.stepForm.testCaseId) {
        throw new Error('请选择接口')
      }
      const payload = {
        testCaseId: this.stepForm.stepType === 'api' ? this.stepForm.testCaseId : undefined,
        stepOrder: this.stepForm.stepOrder,
        stepType: this.stepForm.stepType,
        name: this.stepForm.name,
        enabled: this.stepForm.enabled,
        config:
          this.stepForm.stepType === 'database'
            ? this.buildDatabaseStepConfig()
            : this.stepForm.stepType === 'script'
              ? this.buildScriptStepConfig()
              : this.stepForm.stepType === 'for'
                ? this.buildForStepConfig()
                : this.stepForm.stepType === 'condition'
                  ? this.buildConditionStepConfig()
                  : this.buildAPIStepConfig(),
        requestOverride: this.stepForm.stepType === 'api' ? this.buildStepRequestOverride() : {}
      }
      const attach = this.buildAttachUnderParentPayload()
      if (attach) payload.attachUnderParent = attach
      return payload
    },
    async submitStep() {
      let payload
      try {
        payload = this.buildStepPayload()
      } catch (error) {
        this.$message.error(error.message)
        return null
      }

      const pendingAttachMeta = this.controlFlowChildAttach
      const hadPayloadAttach = Boolean(payload.attachUnderParent)
      this.savingStep = true
      try {
        const saved = await upsertScenarioStep(this.scenario.id, payload.stepOrder, payload)

        if (pendingAttachMeta && !hadPayloadAttach) {
          try {
            await this.attachChildToControlParent(saved, pendingAttachMeta)
          } catch {
            this.$message.warning(
              '子步骤已保存，但未能写入父级 For/条件 配置（请刷新页面后重试，或检查接口版本是否已更新）'
            )
          }
        }

        this.controlFlowChildAttach = null

        if (hadPayloadAttach || pendingAttachMeta) {
          try {
            this.steps = await listScenarioSteps(this.scenario.id)
            const cur = this.steps.find((s) => s.id === saved.id)
            this.handleStepSaved(cur || saved)
          } catch {
            const idx = this.steps.findIndex((s) => s.stepOrder === saved.stepOrder)
            if (idx !== -1) this.steps.splice(idx, 1, saved)
            else {
              this.steps.push(saved)
              this.steps.sort((a, b) => a.stepOrder - b.stepOrder)
            }
            this.handleStepSaved(saved)
          }
        } else {
          const idx = this.steps.findIndex((s) => s.stepOrder === saved.stepOrder)
          if (idx !== -1) {
            this.steps.splice(idx, 1, saved)
          } else {
            this.steps.push(saved)
            this.steps.sort((a, b) => a.stepOrder - b.stepOrder)
          }
          this.handleStepSaved(saved)
        }
        this.$message.success('步骤参数已保存')
        return saved
      } finally {
        this.savingStep = false
      }
    },

    handleStepSaved(savedStep) {
      if (!savedStep?.id) return
      const tab = this.findStepTab(this.activeStepTabId)
      if (tab && tab.kind === 'draft') {
        this.promoteDraftTabToStep(savedStep)
      } else if (tab && tab.kind === 'step' && tab.stepId === savedStep.id) {
        this.refreshStepTabLabel(savedStep)
      } else {
        this.openStepTab(savedStep)
        return
      }
      this.applyStepToForm(savedStep)
    },

    saveStepFromPanel() {
      return this.submitStep()
    },

    async attachChildToControlParent(savedStep, attach) {
      if (!attach || savedStep == null) return
      const childSeq = Number(savedStep.stepSeq)
      if (!Number.isFinite(childSeq) || childSeq <= 0) return

      const parent = this.steps.find((s) => String(s.id) === String(attach.parentId))
      if (!parent) return

      const cfg = this.parseMaybeJSON(parent.config)
      if (parent.stepType === 'for' && attach.branch === 'body') {
        cfg.bodyStepSeqs = this.normalizeStepSeqSelection([
          ...(cfg.bodyStepSeqs || []),
          childSeq
        ])
      } else if (parent.stepType === 'condition') {
        const norm = this.normalizeConditionConfig(cfg)
        if (attach.branch === 'else') {
          norm.elseStepSeqs = this.normalizeStepSeqSelection([
            ...norm.elseStepSeqs,
            childSeq
          ])
        } else if (typeof attach.branch === 'number' && norm.branches[attach.branch]) {
          norm.branches[attach.branch].stepSeqs = this.normalizeStepSeqSelection([
            ...norm.branches[attach.branch].stepSeqs,
            childSeq
          ])
        } else {
          return
        }
        cfg.branches = norm.branches
        cfg.elseStepSeqs = norm.elseStepSeqs
        delete cfg.left
        delete cfg.operator
        delete cfg.right
        delete cfg.thenStepSeqs
      } else {
        return
      }

      const updatedParent = await upsertScenarioStep(this.scenario.id, parent.stepOrder, {
        testCaseId: parent.stepType === 'api' ? parent.testCaseId : undefined,
        stepOrder: parent.stepOrder,
        stepType: parent.stepType || 'api',
        name: parent.name || '',
        enabled: parent.enabled,
        config: cfg,
        requestOverride: parent.requestOverride || {}
      })
      const pidx = this.steps.findIndex((s) => s.id === updatedParent.id)
      if (pidx !== -1) this.steps.splice(pidx, 1, updatedParent)
    },

    async runStepRequest() {
      if (!this.runEnvId) {
        this.$message.warning('请先选择运行环境')
        return
      }
      let payload
      try {
        payload = this.buildStepPayload()
      } catch (error) {
        this.$message.error(error.message)
        return
      }
      this.runningStep = true
      try {
        const output = await runCase(this.stepForm.testCaseId, {
          environmentId: this.runEnvId,
          name: (this.stepForm.name || '').trim() || this.findCaseName(this.stepForm.testCaseId),
          request: payload.requestOverride
        })
        this.stepRunOutput = output
        this.stepRunResultTab = 'response'
        this.stepRunResponseDetailTab = 'body'
        this.$message.success('请求发送完成')
      } catch {
        /* 错误消息由请求拦截器统一展示 */
      } finally {
        this.runningStep = false
      }
    },

    async saveStep(step) {
      await upsertScenarioStep(this.scenario.id, step.stepOrder, {
        testCaseId: step.testCaseId,
        stepOrder: step.stepOrder,
        stepType: step.stepType || 'api',
        name: step.name,
        enabled: step.enabled,
        config: this.parseMaybeJSON(step.config),
        requestOverride: step.requestOverride || {}
      })
    },

    async confirmDeleteStep(step) {
      const label =
        (step.name || '').trim() ||
        this.findCaseName(step.testCaseId) ||
        `步骤 ${step.stepOrder}`
      await this.$confirm(`确认删除步骤「${label}」？`, '删除步骤', {
        type: 'warning',
        confirmButtonText: '删除',
        confirmButtonClass: 'el-button--danger'
      })
      await this.removeStepFromControlConfigs(step.stepSeq, step.id)
      await deleteScenarioStep(this.scenario.id, step.id)
      this.steps = this.steps.filter((s) => s.id !== step.id)
      const removedTabId = this.tabIdForStep(step.id)
      if (this.findStepTab(removedTabId)) {
        this.removeStepTab(removedTabId)
      }
      if (!this.stepTabs.length) {
        if (this.steps.length) {
          this.openStepTab(this.steps[0])
        } else {
          this.openDraftStepTab()
        }
      }
      this.$message.success('步骤已删除')
    },

    async removeStepFromControlConfigs(stepSeq, stepId) {
      const seq = Number(stepSeq)
      if (!Number.isFinite(seq) || seq <= 0) return
      for (const parent of [...this.steps]) {
        if (parent.id === stepId || !['for', 'condition'].includes(parent.stepType)) continue
        const cfg = this.parseMaybeJSON(parent.config)
        let changed = false
        if (parent.stepType === 'for') {
          const before = this.normalizeStepSeqSelection(cfg.bodyStepSeqs)
          const after = before.filter((n) => n !== seq)
          if (after.length !== before.length) {
            cfg.bodyStepSeqs = after
            changed = true
          }
        } else if (parent.stepType === 'condition') {
          const norm = this.normalizeConditionConfig(cfg)
          for (const br of norm.branches) {
            const before = this.normalizeStepSeqSelection(br.stepSeqs)
            const after = before.filter((n) => n !== seq)
            if (after.length !== before.length) {
              br.stepSeqs = after
              changed = true
            }
          }
          const beforeElse = norm.elseStepSeqs
          const afterElse = beforeElse.filter((n) => n !== seq)
          if (afterElse.length !== beforeElse.length) {
            norm.elseStepSeqs = afterElse
            changed = true
          }
          if (changed) {
            cfg.branches = norm.branches
            cfg.elseStepSeqs = norm.elseStepSeqs
            delete cfg.left
            delete cfg.operator
            delete cfg.right
            delete cfg.thenStepSeqs
          }
        }
        if (!changed) continue

        const updated = await upsertScenarioStep(this.scenario.id, parent.stepOrder, {
          testCaseId: parent.stepType === 'api' ? parent.testCaseId : undefined,
          stepOrder: parent.stepOrder,
          stepType: parent.stepType || 'api',
          name: parent.name || '',
          enabled: parent.enabled,
          config: cfg,
          requestOverride: parent.requestOverride || {}
        })
        const idx = this.steps.findIndex((s) => s.id === updated.id)
        if (idx !== -1) this.steps.splice(idx, 1, updated)
      }
    },

    async runScenario() {
      if (!this.runEnvId) {
        this.$message.warning('请先选择运行环境')
        return
      }
      this.running = true
      this.lastRunOutput = null
      this.runResultDetailTabByStep = {}
      this.runResultJsonCollapsed = {}
      try {
        const output = await runScenario(this.scenario.id, {
          environmentId: this.runEnvId,
          name: this.scenario.name
        })
        this.lastRunOutput = output
        const passed = output.run.status === 'passed'
        this.$message[passed ? 'success' : 'error'](
          passed ? '场景运行通过' : '场景运行失败，请查看结果详情'
        )
      } finally {
        this.running = false
      }
    },

    findCase(caseId) {
      if (caseId == null || caseId === '') return undefined
      const sid = String(caseId)
      return this.cases.find((c) => String(c.id) === sid)
    },
    findCaseName(caseId) {
      return this.findCase(caseId)?.name || caseId
    },
    defaultStepTypeName(stepType) {
      if (stepType === 'database') return '数据库操作'
      if (stepType === 'script') return '脚本'
      if (stepType === 'for') return 'For循环'
      if (stepType === 'condition') return '条件分支'
      return 'API'
    },
    stepTypeLabel(stepType) {
      return this.stepTypes.find((item) => item.value === (stepType || 'api'))?.label || 'API'
    },
    stepTypeTagType(stepType) {
      if (stepType === 'database') return 'warning'
      if (stepType === 'script') return 'info'
      if (stepType === 'for') return 'primary'
      if (stepType === 'condition') return 'warning'
      return 'success'
    },
    stepDisplayName(step) {
      const custom = (step?.name || '').trim()
      if (custom) return custom
      const stepType = step?.stepType || 'api'
      if (stepType === 'api') return this.findCaseName(step.testCaseId)
      return this.defaultStepTypeName(stepType)
    },
    stepRunOutcomeMeta(sr) {
      let key = 'unknown'
      if (sr.output && sr.output.skipped === true) {
        key = 'skipped'
      } else {
        const st = sr.result?.status
        if (st === 'passed') key = 'passed'
        else if (st === 'failed') key = 'failed'
        else if (st === 'error') key = 'error'
      }
      const icons = {
        passed: CircleCheck,
        failed: CircleClose,
        error: WarningFilled,
        skipped: Remove,
        unknown: QuestionFilled
      }
      return { key, Icon: icons[key] || QuestionFilled }
    },
    findCaseMethod(caseId) {
      return this.findCase(caseId)?.method || ''
    },
    findCasePath(caseId) {
      return this.findCase(caseId)?.path || ''
    },

    parseSnapshotObject(snapshot) {
      if (snapshot == null || snapshot === '') return null
      try {
        return typeof snapshot === 'string' ? JSON.parse(snapshot) : snapshot
      } catch {
        return null
      }
    },
    isHttpSnapshotObject(o) {
      return (
        !!o &&
        typeof o === 'object' &&
        !Array.isArray(o) &&
        typeof o.statusCode === 'number' &&
        Object.prototype.hasOwnProperty.call(o, 'body')
      )
    },
    httpResponseFromSnapshot(snapshot) {
      const o = this.parseSnapshotObject(snapshot)
      return this.isHttpSnapshotObject(o) ? o : null
    },
    getStepHttpResponse(sr) {
      return this.httpResponseFromSnapshot(sr?.result?.responseSnapshot)
    },
    responseSnapshotStatusType(snapshot) {
      const o = this.parseSnapshotObject(snapshot)
      const code = Number(o?.statusCode)
      if (!Number.isFinite(code)) return 'info'
      if (code >= 200 && code < 300) return 'success'
      if (code >= 400) return 'danger'
      return 'warning'
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
      } catch {
        return String(value)
      }
    },

    formatSnapshot(snapshot) {
      if (!snapshot) return ''
      try {
        const obj = typeof snapshot === 'string' ? JSON.parse(snapshot) : snapshot
        return JSON.stringify(obj, null, 2)
      } catch {
        return String(snapshot)
      }
    },

    assertionTypeLabel(type) {
      return ASSERTION_TYPE_LABELS[type] || type || '-'
    }
  }
}
</script>

<style scoped>
.scenario-editor {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.main-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
}

.main-title {
  font-size: 15px;
  font-weight: 600;
}

.main-actions {
  display: flex;
  align-items: center;
}

.scenario-desc-tag {
  margin-left: 8px;
}

.run-env-select {
  width: 200px;
  margin-right: 10px;
}

/* Run result */
.run-result-area {
  border-top: 1px solid var(--el-border-color);
  padding: 0;
  flex-shrink: 0;
  position: relative;
}

.run-result-resizer {
  height: 10px;
  margin-top: -5px;
  cursor: row-resize;
  background: transparent;
  touch-action: none;
}

.run-result-resizer:hover,
.run-result-resizer:focus-visible {
  background: color-mix(in srgb, var(--el-color-primary) 16%, transparent);
  outline: none;
}

.run-result-resizer:focus-visible {
  box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--el-color-primary) 42%, transparent);
}

.scenario-run-outer-collapse {
  border: none;
  --el-collapse-border-color: transparent;
}

.scenario-run-outer-collapse :deep(.el-collapse-item__header) {
  padding: 12px 20px;
  font-weight: 600;
  background: var(--el-bg-color);
}

.scenario-run-outer-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.run-result-collapse-title {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  width: 100%;
  padding-right: 8px;
}

.run-result-collapse-body {
  padding: 0 20px 16px;
  overflow-y: auto;
}

.step-run-result-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
  padding-right: 8px;
}

.step-run-result-title__seq {
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--el-text-color-secondary);
}

.step-run-result-title__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
  min-width: 0;
}

.step-run-result-title__duration {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  font-weight: 500;
}

.step-run-icon {
  flex-shrink: 0;
  font-size: 18px;
}

.step-run-icon--passed {
  color: var(--el-color-success);
}

.step-run-icon--failed {
  color: var(--el-color-danger);
}

.step-run-icon--error {
  color: var(--el-color-warning);
}

.step-run-icon--skipped {
  color: var(--el-color-info);
}

.step-run-icon--unknown {
  color: var(--el-text-color-placeholder);
}

.response-http-panel {
  margin-top: 4px;
}

.response-http-panel--nested {
  margin-top: 8px;
}

.response-http-meta {
  margin-bottom: 6px;
}

.response-body-headers-tabs {
  width: 100%;
}

.response-body-headers-tabs :deep(.el-tabs__header) {
  margin-bottom: 8px;
}

.snapshot-pre--response-body {
  max-height: min(360px, 40vh);
}

.result-header {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 10px;
}

.step-result-detail {
  padding: 4px 0;
}

.step-result-info-tabs {
  --el-border-color-light: var(--el-border-color-lighter);
}

.step-result-info-tabs :deep(.el-tabs__content) {
  padding: 12px;
}

.step-result-error-strip {
  margin-bottom: 10px;
}

.step-result-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.step-result-snapshot-panel {
  min-width: 0;
}

.step-result-snapshot-panel--full {
  grid-column: 1 / -1;
}

.step-result-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 28px;
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.step-result-panel-hint {
  min-width: 0;
  overflow: hidden;
  color: var(--el-text-color-secondary);
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.run-result-json-viewer {
  max-height: none;
  min-height: 96px;
  margin: 0;
  padding: 10px 0;
  white-space: nowrap;
  word-break: normal;
}

.step-result-snapshot-panel .snapshot-pre {
  max-height: none;
}

.result-status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.duration {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.error-text {
  color: var(--el-color-danger);
  font-size: 12px;
  margin: 4px 0;
}

.section-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 8px 0 4px;
}

.snapshot-pre {
  background: var(--el-fill-color-light);
  border-radius: 4px;
  padding: 8px;
  font-size: 11px;
  overflow: auto;
  max-height: 180px;
  white-space: pre-wrap;
  word-break: break-all;
}

.step-result-snapshot-panel .run-result-json-viewer {
  white-space: nowrap;
  word-break: normal;
}

@media (max-width: 1100px) {
  .step-result-detail-grid {
    grid-template-columns: 1fr;
  }
}

.kv-table,
.extraction-table {
  width: 100%;
}

.step-dialog-workspace {
  display: flex;
  height: 68vh;
  min-height: 560px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.inline-step-workspace {
  flex: 1;
  height: auto;
  min-height: 0;
  margin: 16px 20px;
}

.step-dialog-sidebar {
  min-width: 0;
  display: flex;
  flex-direction: column;
  border-right: none;
  background: var(--el-fill-color-extra-light);
}

.step-sidebar-resizer {
  flex: 0 0 10px;
  margin: 0 2px;
  align-self: stretch;
  border-radius: 6px;
  cursor: col-resize;
  background: transparent;
  touch-action: none;
}

.step-sidebar-resizer:hover,
.step-sidebar-resizer:focus-visible {
  background: color-mix(in srgb, var(--el-color-primary) 18%, transparent);
  outline: none;
}

.step-sidebar-resizer:focus-visible {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--el-color-primary) 45%, transparent);
}

.step-sidebar-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 12px 12px 8px;
  font-size: 13px;
  font-weight: 600;
}

.step-sidebar-add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0;
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px dashed rgba(148, 163, 184, 0.65);
  background: rgba(255, 255, 255, 0.55);
  color: var(--app-secondary-text, #64748b);
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    border-style 0.15s ease,
    background 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.step-sidebar-add:hover {
  border-style: solid;
  border-color: var(--app-primary-color, #2563eb);
  background: rgba(37, 99, 235, 0.06);
  color: var(--app-primary-color, #2563eb);
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.12);
}

.step-sidebar-add:active {
  transform: translateY(0.5px);
}

.step-sidebar-add-icon {
  font-size: 14px;
  opacity: 0.9;
}

.method-tag {
  flex-shrink: 0;
}

.step-dialog-main {
  flex: 1;
  overflow-y: auto;
  padding: 14px 16px;
  background: var(--el-bg-color);
}

.step-dialog-main.scenario-step-console {
  padding: 12px 14px 16px;
  background: #f3f6fb;
}

.scenario-step-tabs {
  flex: 0 0 auto;
  margin: 0 0 10px;
}

.scenario-step-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
  border-bottom: 1px solid var(--el-border-color-light);
}

.scenario-step-tabs :deep(.el-tabs__nav) {
  border: none;
}

.scenario-step-tabs :deep(.el-tabs__item) {
  height: 30px;
  line-height: 30px;
  padding: 0 12px !important;
  font-size: 12.5px;
  color: var(--el-text-color-regular);
  border: 1px solid transparent;
  border-bottom: none;
  border-radius: 6px 6px 0 0;
}

.scenario-step-tabs :deep(.el-tabs__item:hover) {
  color: var(--el-color-primary);
}

.scenario-step-tabs :deep(.el-tabs__item.is-active) {
  background: #ffffff;
  border-color: var(--el-border-color-light);
  color: var(--el-color-primary);
}

.scenario-step-tabs :deep(.el-tabs__content) {
  display: none;
}

.scenario-step-tab-label {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  pointer-events: none;
}

.request-card {
  overflow: hidden;
  border-radius: 12px;
  background: #ffffff;
}

.request-card :deep(.el-card__body) {
  padding: 0;
}

.scenario-step-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #f8fafc;
}

.step-title-text {
  flex: 1;
  min-width: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: text;
  user-select: none;
}

.step-title-text.placeholder {
  color: var(--el-text-color-secondary);
  font-weight: 500;
}

.step-title-hint {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.step-title-placeholder {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.step-title-input {
  flex: 1;
  min-width: 0;
}

.step-type-select {
  width: 120px;
  flex: 0 0 auto;
}

.scenario-api-pick-line {
  display: flex;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #ffffff;
}

.scenario-api-select-full {
  width: 100%;
}

.scenario-request-hint {
  padding: 8px 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #f8fafc;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  font-size: var(--app-font-size-small, 12px);
}

.scenario-step-extra-form {
  padding: 12px 16px 16px;
  border-top: 1px solid var(--app-border-color, var(--el-border-color));
  background: #ffffff;
}

.scenario-script-field {
  width: 100%;
}

.scenario-script-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.scenario-script-toolbar-hint {
  font-size: 12px;
  color: var(--app-secondary-text, #909399);
  line-height: 1.4;
}

.control-inline-fields {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 8px;
  width: 100%;
}

.condition-branch-editor {
  margin-bottom: 12px;
  padding: 12px 12px 2px;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 8px;
  background: #f8fafc;
}

.condition-branch-editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.condition-branch-editor-title {
  font-size: var(--app-font-size-small, 13px);
  font-weight: 600;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
}

.request-meta,
.request-line,
.body-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.request-meta {
  padding: 14px 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #f8fafc;
  flex-wrap: wrap;
}

.body-view-toggle {
  display: flex;
  gap: 0;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 6px;
  overflow: hidden;
  margin-left: auto;
}

.body-view-toggle span {
  padding: 2px 10px;
  font-size: 12px;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  cursor: pointer;
  user-select: none;
  transition: background 0.15s, color 0.15s;
}

.body-view-toggle span:first-child {
  border-right: 1px solid var(--app-border-color, var(--el-border-color));
}

.body-view-toggle span.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
}

.code-view {
  min-height: 260px;
  max-height: 520px;
  overflow: auto;
  padding: 14px;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 10px;
  background: var(--app-code-bg, var(--el-fill-color-light));
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small, 12px);
  line-height: 1.6;
  white-space: pre-wrap;
  margin: 0;
}

.code-editor,
.scenario-body-code-pane {
  min-height: 260px;
  max-height: 520px;
}

.code-editor {
  overflow: auto;
  padding: 14px;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 10px;
  background: var(--app-code-bg, var(--el-fill-color-light));
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small, 12px);
  line-height: 1.6;
  white-space: pre-wrap;
  outline: none;
}

.scenario-body-code-pane--muted {
  opacity: 0.96;
}

.body-schema-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 8px minmax(260px, var(--body-schema-panel-width, 1fr));
  gap: 8px;
  align-items: stretch;
}

.body-schema-layout > .scenario-body-code-pane,
.body-schema-layout > .code-editor,
.body-schema-layout > .json-viewer.code-view {
  min-width: 0;
}

.schema-panel-placeholder {
  width: 8px;
  min-height: 260px;
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
  background: var(--app-border-color, var(--el-border-color));
  transition:
    background 0.12s ease,
    width 0.12s ease,
    left 0.12s ease;
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
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 10px;
  background: var(--app-card-bg, #ffffff);
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
  font-size: var(--app-font-size-small, 13px);
  font-weight: 700;
}

.schema-field-count {
  flex: 0 0 auto;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  font-size: 12px;
}

.schema-field-table {
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.schema-field-table-head,
.schema-field-row {
  display: grid;
  grid-template-columns: minmax(104px, 1.15fr) minmax(72px, 0.62fr) minmax(96px, 0.9fr) minmax(120px, 1.2fr);
  align-items: center;
  column-gap: 8px;
}

.schema-field-table-head {
  min-height: 32px;
  padding: 0 10px;
  background: #f8fafc;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  font-size: 12px;
  font-weight: 600;
}

.schema-field-row {
  min-height: 34px;
  padding: 6px 10px;
  border-top: 1px solid var(--app-border-color, var(--el-border-color));
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

.json-colon,
.json-bracket,
.json-comma {
  color: var(--el-text-color-regular);
}

.json-string {
  color: #16a34a;
}

.json-number {
  color: #c2410c;
}

.json-boolean {
  color: #7c3aed;
}

.json-null {
  color: #94a3b8;
}

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
  border-top: 5px solid var(--app-secondary-text, var(--el-text-color-secondary));
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

.schema-field-limits {
  overflow: hidden;
  color: #475569;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-field-meaning {
  overflow: hidden;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  font-size: var(--app-font-size-small, 13px);
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.auth-value-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}

.auth-value-cell :deep(.el-input) {
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
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  line-height: 1.45;
  word-break: break-word;
}

.result-card.scenario-step-result-card {
  margin-top: 12px;
  overflow: hidden;
  border-radius: 12px;
  background: #ffffff;
}

.scenario-step-result-card :deep(.el-card__body) {
  padding: 16px;
}

.scenario-step-result-head.result-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.scenario-step-result-head h3 {
  margin: 0 0 6px;
  font-size: var(--app-font-size-title, 16px);
}

.scenario-step-result-head p {
  margin: 0;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
}

.result-metrics {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.metric-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 26px;
  padding: 3px 9px;
  border-radius: 999px;
  font-size: var(--app-font-size-small, 13px);
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
  font-size: var(--app-font-size-base, 14px);
}

.url-line {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 10px 12px;
  border: 1px solid var(--app-border-color, var(--el-border-color));
  border-radius: 10px;
  background: var(--app-card-bg, #fafafa);
  word-break: break-all;
}

.scenario-step-result-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.response-detail-tabs {
  width: 100%;
}

.response-detail-tabs--subtle :deep(.el-tabs__header) {
  margin-bottom: 8px;
}

.response-detail-tabs--subtle :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--app-border-color, var(--el-border-color));
}

.response-detail-tabs--subtle :deep(.el-tabs__item) {
  height: 30px;
  padding: 0 10px;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  font-size: 12px;
  line-height: 30px;
}

.response-detail-tabs--subtle :deep(.el-tabs__item.is-active) {
  color: var(--el-color-primary);
}

.response-detail-tabs--subtle :deep(.el-tabs__active-bar) {
  height: 2px;
}

.code-view--curl,
.scenario-step-curl {
  min-height: 160px;
}

.assertion-name {
  font-size: var(--app-font-size-small, 13px);
  font-weight: 500;
}

.method-select {
  width: 118px;
  flex: 0 0 auto;
}

.request-line {
  padding: 14px 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #ffffff;
}

.request-line .send-actions {
  margin-left: auto;
}

.request-line :deep(.el-select__wrapper),
.request-line :deep(.el-input__wrapper) {
  min-height: 42px;
  box-shadow: 0 0 0 1px var(--app-border-color, var(--el-border-color)) inset;
}

.path-input {
  flex: 1 1 0;
  min-width: 0;
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

.scenario-save-row {
  display: flex;
  justify-content: flex-end;
}

.request-tabs {
  margin-top: 0;
  padding: 0 16px 16px;
}

.request-tabs :deep(.el-tabs__header) {
  margin: 0 -16px 16px;
  padding: 0 16px;
  border-bottom: 1px solid var(--app-border-color, var(--el-border-color));
  background: #fbfdff;
}

.request-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.request-tabs :deep(.el-tabs__item) {
  height: 44px;
  font-weight: 600;
}

.kv-row,
.extraction-row {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 6px;
}

.extraction-header {
  display: flex;
  gap: 6px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
  padding: 0 2px;
}

.extraction-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 6px;
}

.params-section {
  margin-bottom: 20px;
}

.params-section:last-child {
  margin-bottom: 0;
}

.params-section-title {
  margin: 0 0 10px;
  font-size: var(--app-font-size-small, 12px);
  font-weight: 700;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
  letter-spacing: 0.02em;
}

.add-row {
  margin-top: 12px;
}

.body-toolbar {
  justify-content: space-between;
  margin-bottom: 8px;
  color: var(--app-secondary-text, var(--el-text-color-secondary));
}

.body-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.step-debug-result {
  margin-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 12px;
}

.step-debug-tabs {
  margin-top: 8px;
}

.extraction-hint code {
  background: var(--el-fill-color);
  padding: 1px 4px;
  border-radius: 3px;
}

.step-ref-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 0 0 12px;
  line-height: 1.6;
}

.step-ref-hint code {
  background: var(--el-fill-color);
  padding: 1px 4px;
  border-radius: 3px;
  font-family: ui-monospace, monospace;
}

.scenario-request-hint code {
  background: var(--el-fill-color);
  padding: 1px 4px;
  border-radius: 3px;
  font-family: ui-monospace, monospace;
}

.step-assertion-tip {
  margin-bottom: 10px;
  font-size: var(--app-font-size-small, 13px);
  color: var(--app-secondary-text, #909399);
}

.assertion-result-table {
  margin-top: 8px;
}

.assertion-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: var(--app-secondary-text);
}
</style>

<style>
/* 步骤侧栏由 ScenarioStepRow / ScenarioStepTreeNode 渲染，需脱离 scoped */

.scenario-editor .step-dialog-sidebar .step-case-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px 32px;
}

.scenario-editor .step-dialog-sidebar .step-sortable-root {
  min-height: 0;
}

.scenario-editor .step-dialog-sidebar .step-case-row {
  display: flex;
  align-items: stretch;
  gap: 4px;
  margin-bottom: 6px;
  padding-left: calc(var(--step-depth, 0) * 12px);
  border-radius: 6px;
  cursor: grab;
}

.scenario-editor .step-dialog-sidebar .step-case-row:active {
  cursor: grabbing;
}

.scenario-editor .step-dialog-sidebar .nested-step-row--block {
  cursor: grab;
  margin-bottom: 4px;
  padding-top: 2px;
  padding-bottom: 2px;
  padding-left: calc(var(--step-depth, 0) * 12px + 4px);
  border-radius: 0 8px 8px 0;
  box-shadow: inset 3px 0 0 rgba(148, 163, 184, 0.35);
  background: linear-gradient(90deg, rgba(248, 250, 252, 0.95) 0%, rgba(255, 255, 255, 0) 52%);
}

.scenario-editor .step-dialog-sidebar .control-flow-frame .nested-step-row--block {
  box-shadow: inset 2px 0 0 rgba(148, 163, 184, 0.2);
  background: transparent;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .nested-step-row--block,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .nested-step-row--block,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .nested-step-row--block {
  box-shadow: none;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .step-case-row,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .step-case-row,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .step-case-row {
  gap: 2px;
  padding-left: calc(max(0, var(--step-depth, 0) - 2) * 6px + 2px);
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .step-case-main,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .step-case-main,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .step-case-main {
  gap: 4px;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .step-seq-badge,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .step-seq-badge,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .step-seq-badge {
  padding: 1px 4px;
  font-size: 10px;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .step-case-item,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .step-case-item,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .step-case-item {
  padding: 6px 4px;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop .step-type-tag.el-tag.el-tag--small,
.scenario-editor .step-dialog-sidebar .control-flow-frame--then .step-type-tag.el-tag.el-tag--small,
.scenario-editor .step-dialog-sidebar .control-flow-frame--else .step-type-tag.el-tag.el-tag--small {
  height: 18px;
  min-height: 18px;
  padding: 0 5px;
  font-size: 10px;
}

.scenario-editor .step-dialog-sidebar .nested-step-row .step-case-item {
  border-left: 1px solid transparent;
}

.scenario-editor .step-dialog-sidebar .control-flow-wrap {
  margin-bottom: 4px;
}

.scenario-editor .step-dialog-sidebar .control-flow-frame {
  margin-left: calc(var(--control-parent-depth, 0) * 12px + 4px);
  padding: 10px 10px 8px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(255, 255, 255, 0.55);
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--loop {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(234, 88, 12, 0.32);
  background: linear-gradient(180deg, rgba(255, 251, 235, 0.65) 0%, rgba(255, 255, 255, 0.35) 100%);
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--then {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(5, 150, 105, 0.26);
  background: linear-gradient(180deg, rgba(236, 253, 245, 0.55) 0%, rgba(255, 255, 255, 0.3) 100%);
}

.scenario-editor .step-dialog-sidebar .control-flow-frame--else {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(100, 116, 139, 0.28);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.9) 0%, rgba(255, 255, 255, 0.35) 100%);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add-anchor {
  position: absolute;
  left: 50%;
  bottom: 0;
  z-index: 1;
  transform: translate(-50%, 50%);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  margin: 0;
  padding: 0;
  border-radius: 999px;
  border: 1px dashed transparent;
  background: rgba(255, 255, 255, 0.95);
  cursor: pointer;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.08);
  transition:
    border-color 0.15s ease,
    border-style 0.15s ease,
    background 0.15s ease,
    box-shadow 0.15s ease,
    color 0.15s ease;
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--loop {
  border-color: rgba(234, 88, 12, 0.38);
  color: #ea580c;
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--loop:hover {
  border-style: solid;
  border-color: #ea580c;
  background: rgba(255, 247, 237, 0.98);
  box-shadow: 0 2px 6px rgba(234, 88, 12, 0.18);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--then {
  border-color: rgba(5, 150, 105, 0.42);
  color: #059669;
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--then:hover {
  border-style: solid;
  border-color: #059669;
  background: rgba(236, 253, 245, 0.98);
  box-shadow: 0 2px 6px rgba(5, 150, 105, 0.2);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--else {
  border-color: rgba(100, 116, 139, 0.42);
  color: #64748b;
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add--else:hover {
  border-style: solid;
  border-color: #64748b;
  background: rgba(248, 250, 252, 0.98);
  box-shadow: 0 2px 6px rgba(100, 116, 139, 0.18);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add:active {
  transform: translateY(0.5px);
}

.scenario-editor .step-dialog-sidebar .control-frame-border-add-icon {
  font-size: 16px;
}

.scenario-editor .step-dialog-sidebar .control-flow-branch-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-left: calc(var(--control-parent-depth, 0) * 12px + 4px);
}

.scenario-editor .step-dialog-sidebar .control-flow-wrap:hover .control-flow-parent .step-case-item {
  background: var(--el-fill-color-light);
}

.scenario-editor .step-dialog-sidebar .step-case-item {
  flex: 1;
  min-width: 0;
  display: block;
  text-align: left;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  padding: 8px;
  cursor: pointer;
  color: var(--el-text-color-primary);
}

.scenario-editor .step-dialog-sidebar .step-case-item:hover {
  background: var(--el-fill-color-light);
}

.scenario-editor .step-dialog-sidebar .step-case-item.active {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.scenario-editor .step-dialog-sidebar .step-delete-icon {
  visibility: hidden;
  flex-shrink: 0;
  align-self: center;
  cursor: pointer;
  color: var(--el-text-color-secondary);
  font-size: 16px;
}

.scenario-editor .step-dialog-sidebar .step-case-row:hover .step-delete-icon {
  visibility: visible;
}

.scenario-editor .step-dialog-sidebar .step-delete-icon:hover {
  color: var(--el-color-danger);
}

.scenario-editor .step-dialog-sidebar .step-case-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.scenario-editor .step-dialog-sidebar .step-seq-badge {
  flex-shrink: 0;
  align-self: center;
  margin: 0;
  border: 1px solid transparent;
  font-size: 11px;
  font-weight: 700;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 4px;
  padding: 2px 6px;
  font-family: ui-monospace, monospace;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  line-height: 1.2;
  transition:
    color 0.15s ease,
    background 0.15s ease,
    border-color 0.15s ease,
    opacity 0.15s ease;
}

.scenario-editor .step-dialog-sidebar .step-seq-badge:hover {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-8);
}

.scenario-editor .step-dialog-sidebar .step-seq-badge:focus-visible {
  outline: 2px solid var(--el-color-primary-light-3);
  outline-offset: 1px;
}

.scenario-editor .step-dialog-sidebar .step-seq-badge--disabled {
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  opacity: 0.85;
}

.scenario-editor .step-dialog-sidebar .step-seq-badge--disabled:hover {
  border-color: var(--el-border-color);
  background: var(--el-fill-color);
}

.scenario-editor .step-dialog-sidebar .step-type-tag.el-tag.el-tag--small {
  flex-shrink: 0;
  --el-tag-font-size: 11px;
  font-size: 11px;
  line-height: 1;
  height: 20px;
  min-height: 20px;
  padding: 0 6px;
  box-sizing: border-box;
}

.scenario-editor .step-dialog-sidebar .step-case-title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}
</style>
