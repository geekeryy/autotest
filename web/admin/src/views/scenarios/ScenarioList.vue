<template>
  <div class="scenario-workspace">
    <!-- 左侧：场景列表 -->
    <div class="scenario-sidebar">
      <div class="sidebar-header">
        <el-select
          v-model="serviceId"
          class="service-select"
          :placeholder="projectId ? '选择服务' : '请先在顶部选择项目'"
          :disabled="!projectId"
          filterable
          @change="onServiceChange"
        >
          <el-option v-for="svc in services" :key="svc.id" :label="svc.name" :value="svc.id" />
        </el-select>
        <el-button
          type="primary"
          size="small"
          :disabled="!serviceId"
          @click="openCreateDialog"
        >新建</el-button>
      </div>

      <div class="scenario-list">
        <div
          v-for="sc in scenarios"
          :key="sc.id"
          class="scenario-item"
          :class="{ active: selectedScenario?.id === sc.id }"
          @click="selectScenario(sc)"
        >
          <span class="scenario-name">{{ sc.name }}</span>
          <el-dropdown trigger="click" @command="(cmd) => handleScenarioCommand(cmd, sc)" @click.stop>
            <el-icon class="scenario-action"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">编辑信息</el-dropdown-item>
                <el-dropdown-item command="delete" class="danger-item">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div v-if="!scenarios.length && serviceId" class="empty-tip">暂无场景，点击新建</div>
        <div v-if="!serviceId" class="empty-tip">请先选择服务</div>
      </div>
    </div>

    <!-- 右侧：步骤编辑 + 运行 -->
    <div class="scenario-main">
      <template v-if="selectedScenario">
        <div class="main-header">
          <div class="main-title">
            <span>{{ selectedScenario.name }}</span>
            <el-tag v-if="selectedScenario.description" type="info" size="small" class="scenario-desc-tag">
              {{ selectedScenario.description }}
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

        <!-- 步骤编辑工作区 -->
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
                        content="将当前请求参数保存到该场景步骤（数据库持久化），不会改写接口管理中原始用例"
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
                          <div class="params-section-title">Path Vars</div>
                          <el-table :data="stepForm.pathParamRows" border class="kv-table">
                            <el-table-column width="70" label="启用">
                              <template #default="{ row }"><el-checkbox v-model="row.enabled" /></template>
                            </el-table-column>
                            <el-table-column label="参数名" min-width="180">
                              <template #default="{ row }"><el-input v-model="row.key" placeholder="路径变量名" /></template>
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
                              <template #default="{ $index }">
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
                              <template #default="{ row }"><el-input v-model="row.key" placeholder="参数名" /></template>
                            </el-table-column>
                            <el-table-column label="参数值" min-width="260">
                              <template #default="{ row }">
                                <div class="query-value-cell">
                                  <el-input v-model="row.value" placeholder="参数值 / {{变量}}" />
                                  <span v-if="row.required" class="required-star" aria-label="必填">*</span>
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
                                <el-button link type="danger" @click="removeRow(stepForm.queryRows, $index)">删除</el-button>
                              </template>
                            </el-table-column>
                          </el-table>
                          <el-button class="add-row" @click="addRow(stepForm.queryRows)">新增一行</el-button>
                        </div>
                      </el-tab-pane>
                      <el-tab-pane label="Headers" name="headers">
                        <el-table :data="stepForm.headerRows" border class="kv-table">
                          <el-table-column width="70" label="启用">
                            <template #default="{ row }"><el-checkbox v-model="row.enabled" /></template>
                          </el-table-column>
                          <el-table-column label="Header" min-width="180">
                            <template #default="{ row }"><el-input v-model="row.key" placeholder="Header" /></template>
                          </el-table-column>
                          <el-table-column label="Value" min-width="260">
                            <template #default="{ row }"><el-input v-model="row.value" placeholder="支持 {{变量名}}" /></template>
                          </el-table-column>
                          <el-table-column label="字段注释" min-width="220">
                            <template #default="{ row }">
                              <span class="param-comment">{{ parameterCommentForStep(row, 'header') }}</span>
                            </template>
                          </el-table-column>
                          <el-table-column width="90" label="操作">
                            <template #default="{ $index }">
                              <el-button link type="danger" @click="removeRow(stepForm.headerRows, $index)">删除</el-button>
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
                          <div
                            v-show="stepRequestBodyViewMode === 'edit'"
                            ref="stepBodyEditor"
                            class="code-editor scenario-body-code-pane"
                            contenteditable="true"
                            spellcheck="false"
                            @input="stepForm.bodyText = $event.target.innerText"
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
                      Path Vars 的值会直接嵌入路径；Query、Headers 和 Body 作为请求覆盖保存。
                      后续步骤可通过 <code v-pre>{{$steps[N].xxx}}</code> 引用本步骤输出，N 为左侧 <strong>#序号</strong>。
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

        <!-- 运行结果（整块可收起） -->
        <div v-if="lastRunOutput" class="run-result-area">
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
              <div class="run-result-collapse-body">
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
                        </div>
                      </template>
                    </template>
                    <div class="step-result-detail">
                      <div class="result-status-row">
                        <el-tag
                          :type="sr.result?.status === 'passed' ? 'success' : sr.result?.status === 'error' ? 'warning' : 'danger'"
                          size="small"
                        >{{ sr.result?.status || '未运行' }}</el-tag>
                        <span v-if="sr.result?.durationMillis" class="duration">
                          {{ sr.result.durationMillis }}ms
                        </span>
                      </div>
                      <div v-if="sr.result?.error" class="error-text">{{ sr.result.error }}</div>
                      <div v-if="sr.stepErrors?.length" class="error-text">
                        步骤错误：{{ sr.stepErrors.join('; ') }}
                      </div>

                      <!-- 步骤输出 -->
                      <template v-if="sr.output && Object.keys(sr.output).length">
                        <div class="section-label">
                          步骤输出（可通过 <code>{{ stepsRefSnippet(sr.step.stepSeq, '.xxx') }}</code> 在后续步骤中引用）：
                        </div>
                        <pre class="snapshot-pre">{{ JSON.stringify(sr.output, null, 2) }}</pre>
                      </template>

                      <!-- 断言结果 -->
                      <template v-if="sr.result?.assertions?.length">
                        <div class="section-label">断言结果：</div>
                        <el-table :data="sr.result.assertions" border size="small" class="assertion-result-table">
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
                      </template>

                      <!-- 响应（HTTP：Body 单独展示） -->
                      <template v-for="hr in [getStepHttpResponse(sr)]" :key="'srhr' + si">
                        <div v-if="hr" class="response-http-panel response-http-panel--nested">
                          <div class="response-http-meta">
                            <el-tag :type="responseSnapshotStatusType(sr.result?.responseSnapshot)" size="small">
                              {{ hr.statusCode }}
                            </el-tag>
                          </div>
                          <el-tabs
                            :model-value="runResultResponseDetailTabByStep[si] ?? 'body'"
                            class="response-body-headers-tabs"
                            type="card"
                            @update:model-value="(v) => setRunResultResponseDetailTab(si, v)"
                          >
                            <el-tab-pane label="Body" name="body">
                              <pre class="snapshot-pre snapshot-pre--response-body">{{ formatSnapshotBody(hr.body) }}</pre>
                            </el-tab-pane>
                            <el-tab-pane label="响应头" name="headers">
                              <el-table :data="headersToRows(hr.headers)" border size="small">
                                <el-table-column prop="key" label="名称" min-width="160" />
                                <el-table-column prop="value" label="值" min-width="220" />
                              </el-table>
                            </el-tab-pane>
                          </el-tabs>
                        </div>
                        <template v-else-if="sr.result?.responseSnapshot">
                          <div class="section-label">响应快照：</div>
                          <pre class="snapshot-pre">{{ formatSnapshot(sr.result.responseSnapshot) }}</pre>
                        </template>
                      </template>
                    </div>
                  </el-collapse-item>
                </el-collapse>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>

      <div v-else class="placeholder">
        <el-empty description="请从左侧选择场景" />
      </div>
    </div>

    <!-- 新建/编辑场景对话框 -->
    <el-dialog v-model="scenarioDialogVisible" :title="editingScenario ? '编辑场景' : '新建场景'" width="480px">
      <el-form :model="scenarioForm" label-width="70px">
        <el-form-item label="名称" required>
          <el-input v-model="scenarioForm.name" placeholder="场景名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="scenarioForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="scenarioDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitScenario">保存</el-button>
      </template>
    </el-dialog>

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
  Delete,
  MoreFilled,
  Plus,
  QuestionFilled,
  Remove,
  WarningFilled
} from '@element-plus/icons-vue'
import {
  createScenario,
  deleteScenario,
  deleteScenarioStep,
  generateCaseParams,
  getDataSourceSchema,
  listDataSources,
  listCases,
  listEndpoints,
  listScenarioSteps,
  listScenarios,
  listServiceEnvironments,
  listServices,
  reorderScenarioSteps,
  runCase,
  runScenario,
  updateScenario,
  upsertScenarioStep
} from '../../api'
import { buildCurlFromRequestSnapshot } from '../../utils/curl'
import { loadGlobalProjects, projectState } from '../../utils/currentProject'
import SqlEditor from '../../components/SqlEditor.vue'
import AssertionEditor from '../../components/AssertionEditor.vue'
import ScriptLibraryPicker from '../../components/ScriptLibraryPicker.vue'
import AIGenerateDialog from '../../components/AIGenerateDialog.vue'
import ScenarioStepTreeNode from './ScenarioStepTreeNode.vue'

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

export default {
  name: 'ScenarioList',
  components: {
    CircleCheck,
    CircleClose,
    MoreFilled,
    Delete,
    Plus,
    QuestionFilled,
    Remove,
    WarningFilled,
    ScenarioStepTreeNode,
    SqlEditor,
    AssertionEditor,
    ScriptLibraryPicker,
    AIGenerateDialog
  },

  data() {
    return {
      services: [],
      serviceId: '',
      /** 当前服务接口定义（字段注释 / Body Schema） */
      serviceEndpoints: [],
      scenarios: [],
      selectedScenario: null,
      steps: [],
      cases: [],
      environments: [],
      dataSources: [],
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
      /** 单步发送结果：响应区 Body / 响应头 */
      stepRunResponseDetailTab: 'body',
      /** 整场景运行结果：各步骤响应区 Body / 响应头 */
      runResultResponseDetailTabByStep: {},
      /** 场景运行结果外层折叠（整块收起） */
      scenarioRunResultActiveNames: ['panel'],

      stepSidebarWidthPx: STEP_SIDEBAR_DEFAULT,
      _stepSidebarResize: null,

      aiScenarioScriptDialogVisible: false,
      aiScenarioScriptLoading: false,
      /** Body 页与运行控制台一致的 编辑/预览 切换 */
      stepRequestBodyViewMode: 'edit',
      stepBodySchemaPanelWidth: readStoredScenarioBodySchemaPanelWidth(),
      stepBodySchemaResizeState: null,
      stepRequestBodyCollapsed: {},
      stepRequestBodyJsonCollapsed: {},
      /** 单步调试结果：折叠树 */
      stepDebugRequestSnapshotJsonCollapsed: {},
      stepDebugResponseBodyCollapsed: {},
      stepDebugResponseJsonCollapsed: {},

      // Scenario dialog
      scenarioDialogVisible: false,
      editingScenario: null,
      scenarioForm: { name: '', description: '' },

      editingStep: null,
      editingStepName: false,
      /** 左侧控制流「+」新增子步骤时，随首次保存 POST 体一并提交 attachUnderParent；有 id 的已保存步骤在编辑时会被清空 */
      controlFlowChildAttach: null,
      sortableInstance: null,
      stepTypes: [
        { label: 'API', value: 'api' },
        { label: '数据库', value: 'database' },
        { label: '脚本', value: 'script' },
        { label: 'For循环', value: 'for' },
        { label: '条件分支', value: 'condition' }
      ],
      conditionOperators: [
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
      ],
      httpMethods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],

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
        conditionElseStepSeqs: []
      }
    }
  },

  computed: {
    projectId() {
      return projectState.currentProjectId
    },
    panelSteps() {
      return [...this.steps].sort((a, b) => a.stepOrder - b.stepOrder)
    },
    stepTreeBlocks() {
      return this.buildStepBlocks()
    },
    stepRunResult() {
      return this.stepRunOutput?.result || this.stepRunOutput?.results?.[0] || null
    },
    /** 单步调试发送结果：是否为 HTTP 响应快照（含 statusCode + body） */
    stepRunHttpResponse() {
      return this.httpResponseFromSnapshot(this.stepRunResult?.responseSnapshot)
    },
    /** 右侧步骤面板是否展示（stepOrder 无效时点了「新增」会像无反应） */
    isStepFormActive() {
      const o = Number(this.stepForm.stepOrder)
      return Number.isFinite(o) && o > 0
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
        const parsed = JSON.parse(raw)
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
        scenarioName: this.selectedScenario?.name || '',
        scenarioDescription: this.selectedScenario?.description || '',
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
          bodyDraft = JSON.parse(this.stepForm.bodyText)
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
    projectId() {
      this.loadServices()
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
    stepRequestBodyViewMode(mode) {
      if (mode === 'edit') this.$nextTick(() => this.syncStepBodyEditorInner())
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
    await loadGlobalProjects()
    await this.loadServices()
    this.initStepSidebarWidth()
  },

  beforeUnmount() {
    this.detachStepSidebarResizeListeners()
    this.stopStepBodySchemaResize()
    if (this.sortableInstance) {
      this.sortableInstance.destroy()
      this.sortableInstance = null
    }
  },

  methods: {
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

    setRunResultResponseDetailTab(stepIndex, tab) {
      this.runResultResponseDetailTabByStep = {
        ...this.runResultResponseDetailTabByStep,
        [stepIndex]: tab
      }
    },
    /** 文档占位：生成 {{$steps[n].path}} 示例文案；不能在模板字符串里直接写 `}}`，会被 Vue 当成插值结束符 */
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
        // schema 加载失败不影响使用
      }
    },
    initStepSortable() {
      if (this.sortableInstance) {
        this.sortableInstance.destroy()
        this.sortableInstance = null
      }
      const el = this.$refs.stepSortableList
      if (!el || !this.panelSteps.length || !this.selectedScenario) return
      this.sortableInstance = Sortable.create(el, {
        animation: 150,
        filter:
          '.control-frame-border-add, .step-sidebar-add, .step-seq-badge, .step-delete-icon, .el-checkbox, .el-checkbox__input',
        // true 会在 filter 匹配元素上 preventDefault，导致循环体/分支框底「+」无法触发 click
        preventOnFilter: false,
        onEnd: this.onStepSortEnd
      })
    },

    async onStepSortEnd(evt) {
      if (evt.oldIndex === evt.newIndex || !this.selectedScenario) return
      const root = this.$refs.stepSortableList
      if (!root) return
      const ids = [...root.querySelectorAll('[data-step-id]')]
        .map((el) => el.dataset.stepId)
        .filter(Boolean)
      if (ids.length !== this.panelSteps.length) return
      try {
        await reorderScenarioSteps(this.selectedScenario.id, { stepIds: ids })
        this.steps = await listScenarioSteps(this.selectedScenario.id)
        const curId = this.editingStep?.id
        if (curId) {
          const u = this.steps.find((s) => s.id === curId)
          if (u) this.editStep(u)
        }
      } catch {
        this.steps = await listScenarioSteps(this.selectedScenario.id)
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
      if (!this.selectedScenario) return
      const prev = step.enabled
      try {
        await upsertScenarioStep(this.selectedScenario.id, step.stepOrder, {
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
      } catch {
        step.enabled = prev
      }
    },

    async loadServices() {
      this.serviceId = ''
      this.scenarios = []
      this.selectedScenario = null
      this.steps = []
      this.dataSources = []
      this.services = this.projectId ? await listServices(this.projectId) : []
      if (this.services[0]) {
        this.serviceId = this.services[0].id
        await this.onServiceChange()
      }
    },

    async onServiceChange() {
      this.scenarios = []
      this.selectedScenario = null
      this.steps = []
      this.dataSources = []
      this.serviceEndpoints = []
      if (!this.projectId || !this.serviceId) return
      const params = { projectId: this.projectId, serviceId: this.serviceId }
      const [scenarios, cases, envs, endpointsRaw] = await Promise.all([
        listScenarios(params),
        listCases(params),
        listServiceEnvironments(this.projectId, this.serviceId),
        listEndpoints(this.projectId, this.serviceId)
      ])
      this.scenarios = scenarios
      this.cases = cases
      this.serviceEndpoints = Array.isArray(endpointsRaw) ? endpointsRaw : []
      this.environments = envs
      if (envs.length) this.runEnvId = envs[0].id
      await this.loadDataSources()
    },

    async loadDataSources() {
      if (!this.projectId) {
        this.dataSources = []
        return
      }
      this.dataSources = await listDataSources({
        projectId: this.projectId
      })
    },

    async selectScenario(sc) {
      this.selectedScenario = sc
      this.lastRunOutput = null
      this.runResultResponseDetailTabByStep = {}
      this.steps = await listScenarioSteps(sc.id)
      if (this.steps.length) {
        this.editStep(this.steps[0])
      } else {
        this.startNewStepDraft()
      }
    },

    openCreateDialog() {
      this.editingScenario = null
      this.scenarioForm = { name: '', description: '' }
      this.scenarioDialogVisible = true
    },

    handleScenarioCommand(cmd, sc) {
      if (cmd === 'edit') {
        this.editingScenario = sc
        this.scenarioForm = { name: sc.name, description: sc.description || '' }
        this.scenarioDialogVisible = true
      } else if (cmd === 'delete') {
        this.$confirm(`确认删除场景「${sc.name}」？此操作不可恢复。`, '删除场景', {
          type: 'warning',
          confirmButtonText: '删除',
          confirmButtonClass: 'el-button--danger'
        }).then(async () => {
          await deleteScenario(sc.id)
          this.$message.success('场景已删除')
          if (this.selectedScenario?.id === sc.id) {
            this.selectedScenario = null
            this.steps = []
          }
          this.scenarios = this.scenarios.filter((s) => s.id !== sc.id)
        }).catch(() => {})
      }
    },

    async submitScenario() {
      if (!this.scenarioForm.name.trim()) {
        this.$message.warning('请输入场景名称')
        return
      }
      if (this.editingScenario) {
        const updated = await updateScenario(this.editingScenario.id, this.scenarioForm)
        const idx = this.scenarios.findIndex((s) => s.id === updated.id)
        if (idx !== -1) this.scenarios.splice(idx, 1, updated)
        if (this.selectedScenario?.id === updated.id) this.selectedScenario = updated
      } else {
        const sc = await createScenario({
          ...this.scenarioForm,
          projectId: this.projectId,
          serviceId: this.serviceId
        })
        this.scenarios.push(sc)
        await this.selectScenario(sc)
      }
      this.$message.success('保存成功')
      this.scenarioDialogVisible = false
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

    editStep(step) {
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
      this.editStep(step)
    },

    isPanelStepActive(step) {
      return this.editingStep?.id === step.id
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
      this.stepForm.queryRows = this.buildStepQueryRows(tc, this.normalizeObject(request.query))
      this.stepForm.headerRows = this.buildStepHeaderRows(tc, this.normalizeObject(request.headers))
      this.setStepBodyText(request.body == null ? '' : this.formatJSON(request.body))
      this.stepForm.requestSecurity = request.security
      this.syncPathParamRows()
      this.stepForm.requestTab = this.defaultStepRequestTab()
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
        body: hasOverride('body') ? override.body : base.body,
        security: hasOverride('security') ? override.security : base.security
      }
      return request
    },

    syncPathParamRows() {
      // Extract path variable names from the path template and pre-populate the table.
      // The user fills in values; on save they are directly embedded into the path.
      const current = this.rowsToObject(this.stepForm.pathParamRows)
      const defaults = this.pathVariables(this.stepForm.requestPath)
      const rows = []
      for (const key of Object.keys(defaults)) {
        rows.push({
          enabled: true,
          key,
          value: Object.prototype.hasOwnProperty.call(current, key) ? current[key] : defaults[key]
        })
      }
      this.stepForm.pathParamRows = rows
    },

    buildStepRequestOverride() {
      let body = null
      if (this.stepForm.bodyText.trim()) {
        try {
          body = JSON.parse(this.stepForm.bodyText)
        } catch (error) {
          throw new Error(`Body 不是合法 JSON：${error.message}`)
        }
      }
      // Embed path param values directly into the path template.
      // This allows values like {{$steps[1].body.id}} or literal strings.
      let path = this.pathForRun(this.stepForm.requestPath || this.findCasePath(this.stepForm.testCaseId))
      for (const row of (this.stepForm.pathParamRows || [])) {
        if (row.enabled !== false && row.key && row.key.trim()) {
          const escaped = row.key.trim().replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
          path = path.replace(new RegExp(`\\{\\{+${escaped}\\}\\}+`, 'g'), row.value || '')
        }
      }
      const request = {
        method: this.stepForm.requestMethod || this.findCaseMethod(this.stepForm.testCaseId),
        path,
        headers: this.rowsToObject(this.stepForm.headerRows),
        query: this.rowsToObject(this.stepForm.queryRows),
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
        this.setStepBodyText(this.formatJSON(JSON.parse(this.stepForm.bodyText)))
      } catch (error) {
        this.$message.error(`Body 不是合法 JSON：${error.message}`)
      }
    },

    addRow(rows) {
      rows.push({ enabled: true, key: '', value: '', required: false })
    },

    removeRow(rows, index) {
      rows.splice(index, 1)
    },

    objectToRows(value, enabledMap, requiredMap) {
      return Object.entries(value || {}).map(([key, item]) => ({
        enabled: enabledMap ? enabledMap[key] !== false : true,
        key,
        value: String(item),
        required: requiredMap ? requiredMap[key] === true : false
      }))
    },

    rowsToObject(rows) {
      return (rows || []).reduce((out, row) => {
        if (row.enabled !== false && row.key && row.key.trim()) out[row.key.trim()] = row.value
        return out
      }, {})
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

    normalizePathVariables(path) {
      return String(path || '').replace(/\{\{+([^{}]+)\}\}+/g, '{$1}')
    },

    pathForRun(path) {
      return String(path || '').replace(/(^|[^{])\{([^{}]+)\}(?!\})/g, '$1{{$2}}')
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

    defaultStepRequestTab() {
      const method = String(this.stepForm.requestMethod || '').toUpperCase()
      if (method === 'POST' || method === 'PUT' || method === 'PATCH') return 'body'
      return 'params'
    },

    formatJSON(value) {
      return JSON.stringify(value, null, 2)
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
      this.$nextTick(() => {
        const el = this.$refs.stepBodyEditor
        if (el && el.innerText !== text) el.innerText = text || ''
      })
    },

    syncStepBodyEditorInner() {
      const el = this.$refs.stepBodyEditor
      if (!el) return
      const t = this.stepForm.bodyText ?? ''
      if (el.innerText !== t) el.innerText = t
    },

    /** 与 CaseRun「运行控制台」中单步结果的指标 / 断言展示一致 */
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

        if (generated.body != null) {
          this.setStepBodyText(this.formatJSON(generated.body))
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

      let bodyDraft = null
      if ((this.stepForm.bodyText || '').trim()) {
        try {
          bodyDraft = JSON.parse(this.stepForm.bodyText)
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
        this.setStepBodyText(this.formatJSON(nextBody))
      }

      if (nextBody != null && nextBody !== '' && !mergedHeaders['Content-Type']) {
        mergedHeaders['Content-Type'] = 'application/json'
        this.stepForm.headerRows = this.buildStepHeaderRows(tc, mergedHeaders)
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
      // 子步骤在左侧「+」保存后会写回父级 config，但右侧 For 表单可能仍是打开时的旧数据；合并已持久化的 bodyStepSeqs，避免保存父步骤时清空循环体引用。
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
      // 已落库的步骤再次保存时不要重复挂到父级；新草稿 editingStep 为 null
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
        const saved = await upsertScenarioStep(this.selectedScenario.id, payload.stepOrder, payload)

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
            this.steps = await listScenarioSteps(this.selectedScenario.id)
            const cur = this.steps.find((s) => s.id === saved.id)
            if (cur) this.editStep(cur)
            else this.editingStep = saved
          } catch {
            const idx = this.steps.findIndex((s) => s.stepOrder === saved.stepOrder)
            if (idx !== -1) this.steps.splice(idx, 1, saved)
            else {
              this.steps.push(saved)
              this.steps.sort((a, b) => a.stepOrder - b.stepOrder)
            }
            this.editingStep = saved
          }
        } else {
          const idx = this.steps.findIndex((s) => s.stepOrder === saved.stepOrder)
          if (idx !== -1) {
            this.steps.splice(idx, 1, saved)
          } else {
            this.steps.push(saved)
            this.steps.sort((a, b) => a.stepOrder - b.stepOrder)
          }
          this.editingStep = saved
        }
        this.$message.success('步骤参数已保存')
        return saved
      } finally {
        this.savingStep = false
      }
    },

    saveStepFromPanel() {
      return this.submitStep()
    },

    /**
     * 当 attachUnderParent 未随 PUT 一并提交时（例如旧版 API），第二次请求更新父级 config。
     */
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

      const updatedParent = await upsertScenarioStep(this.selectedScenario.id, parent.stepOrder, {
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
        // 错误消息由请求拦截器统一展示。
      } finally {
        this.runningStep = false
      }
    },

    async saveStep(step) {
      await upsertScenarioStep(this.selectedScenario.id, step.stepOrder, {
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
      await deleteScenarioStep(this.selectedScenario.id, step.id)
      this.steps = this.steps.filter((s) => s.id !== step.id)
      if (this.editingStep?.id === step.id) {
        if (this.steps.length) {
          this.editStep(this.steps[0])
        } else {
          this.startNewStepDraft()
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

        const updated = await upsertScenarioStep(this.selectedScenario.id, parent.stepOrder, {
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
      this.runResultResponseDetailTabByStep = {}
      try {
        const output = await runScenario(this.selectedScenario.id, {
          environmentId: this.runEnvId,
          name: this.selectedScenario.name
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
    /** 场景运行结果折叠标题图标：passed / failed / error / skipped / unknown */
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
.scenario-workspace {
  display: flex;
  height: 100%;
  gap: 0;
  overflow: hidden;
}

/* Left sidebar */
.scenario-sidebar {
  width: 260px;
  min-width: 180px;
  max-width: 320px;
  border-right: 1px solid var(--el-border-color);
  display: flex;
  flex-direction: column;
  background: var(--el-bg-color);
}

.sidebar-header {
  display: flex;
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid var(--el-border-color-light);
  align-items: center;
}

.service-select {
  flex: 1;
  min-width: 0;
}

.scenario-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.scenario-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  cursor: pointer;
  border-radius: 4px;
  margin: 0 6px 2px;
  transition: background 0.15s;
}

.scenario-item:hover {
  background: var(--el-fill-color-light);
}

.scenario-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.scenario-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.scenario-action {
  visibility: hidden;
  cursor: pointer;
  color: var(--el-text-color-secondary);
}

.scenario-item:hover .scenario-action {
  visibility: visible;
}

/* Main area */
.scenario-main {
  flex: 1;
  display: flex;
  flex-direction: column;
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
  max-height: 45vh;
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

.placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-tip {
  text-align: center;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  padding: 24px;
}

/* Dialogs */
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
  grid-template-columns: minmax(112px, 1.25fr) minmax(72px, 0.65fr) minmax(120px, 1.2fr);
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

:deep(.danger-item) {
  color: var(--el-color-danger);
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

.scenario-workspace .step-dialog-sidebar .step-case-list {
  flex: 1;
  overflow-y: auto;
  /* 控制流框底「+」一半伸出框外，需多留底边距以免被滚动区域裁切导致点不到 */
  padding: 0 8px 32px;
}

.scenario-workspace .step-dialog-sidebar .step-sortable-root {
  min-height: 0;
}

.scenario-workspace .step-dialog-sidebar .step-case-row {
  display: flex;
  align-items: stretch;
  gap: 4px;
  margin-bottom: 6px;
  padding-left: calc(var(--step-depth, 0) * 12px);
  border-radius: 6px;
  cursor: grab;
}

.scenario-workspace .step-dialog-sidebar .step-case-row:active {
  cursor: grabbing;
}

.scenario-workspace .step-dialog-sidebar .nested-step-row--block {
  cursor: grab;
  margin-bottom: 4px;
  padding-top: 2px;
  padding-bottom: 2px;
  padding-left: calc(var(--step-depth, 0) * 12px + 4px);
  border-radius: 0 8px 8px 0;
  box-shadow: inset 3px 0 0 rgba(148, 163, 184, 0.35);
  background: linear-gradient(90deg, rgba(248, 250, 252, 0.95) 0%, rgba(255, 255, 255, 0) 52%);
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame .nested-step-row--block {
  box-shadow: inset 2px 0 0 rgba(148, 163, 184, 0.2);
  background: transparent;
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .nested-step-row--block,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .nested-step-row--block,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .nested-step-row--block {
  box-shadow: none;
}

/* 控制流子帧（循环体 / Then / Else）：收紧留白与控件占位，深度缩进从子帧首层起算，便于展示更长用例名 */
.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .step-case-row,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .step-case-row,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .step-case-row {
  gap: 2px;
  padding-left: calc(max(0, var(--step-depth, 0) - 2) * 6px + 2px);
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .step-case-main,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .step-case-main,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .step-case-main {
  gap: 4px;
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .step-seq-badge,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .step-seq-badge,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .step-seq-badge {
  padding: 1px 4px;
  font-size: 10px;
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .step-case-item,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .step-case-item,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .step-case-item {
  padding: 6px 4px;
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop .step-type-tag.el-tag.el-tag--small,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--then .step-type-tag.el-tag.el-tag--small,
.scenario-workspace .step-dialog-sidebar .control-flow-frame--else .step-type-tag.el-tag.el-tag--small {
  height: 18px;
  min-height: 18px;
  padding: 0 5px;
  font-size: 10px;
}

.scenario-workspace .step-dialog-sidebar .nested-step-row .step-case-item {
  border-left: 1px solid transparent;
}

.scenario-workspace .step-dialog-sidebar .control-flow-wrap {
  margin-bottom: 4px;
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame {
  margin-left: calc(var(--control-parent-depth, 0) * 12px + 4px);
  padding: 10px 10px 8px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(255, 255, 255, 0.55);
}

/* 控制流子帧（循环体 / 条件分支）：统一底边加号与框内水平留白；子步骤紧凑布局见上一条 */
.scenario-workspace .step-dialog-sidebar .control-flow-frame--loop {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(234, 88, 12, 0.32);
  background: linear-gradient(180deg, rgba(255, 251, 235, 0.65) 0%, rgba(255, 255, 255, 0.35) 100%);
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--then {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(5, 150, 105, 0.26);
  background: linear-gradient(180deg, rgba(236, 253, 245, 0.55) 0%, rgba(255, 255, 255, 0.3) 100%);
}

.scenario-workspace .step-dialog-sidebar .control-flow-frame--else {
  position: relative;
  padding: 10px 2px 18px;
  border-color: rgba(100, 116, 139, 0.28);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.9) 0%, rgba(255, 255, 255, 0.35) 100%);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add-anchor {
  position: absolute;
  left: 50%;
  bottom: 0;
  z-index: 1;
  transform: translate(-50%, 50%);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add {
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

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--loop {
  border-color: rgba(234, 88, 12, 0.38);
  color: #ea580c;
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--loop:hover {
  border-style: solid;
  border-color: #ea580c;
  background: rgba(255, 247, 237, 0.98);
  box-shadow: 0 2px 6px rgba(234, 88, 12, 0.18);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--then {
  border-color: rgba(5, 150, 105, 0.42);
  color: #059669;
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--then:hover {
  border-style: solid;
  border-color: #059669;
  background: rgba(236, 253, 245, 0.98);
  box-shadow: 0 2px 6px rgba(5, 150, 105, 0.2);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--else {
  border-color: rgba(100, 116, 139, 0.42);
  color: #64748b;
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add--else:hover {
  border-style: solid;
  border-color: #64748b;
  background: rgba(248, 250, 252, 0.98);
  box-shadow: 0 2px 6px rgba(100, 116, 139, 0.18);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add:active {
  transform: translateY(0.5px);
}

.scenario-workspace .step-dialog-sidebar .control-frame-border-add-icon {
  font-size: 16px;
}

.scenario-workspace .step-dialog-sidebar .control-flow-branch-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-left: calc(var(--control-parent-depth, 0) * 12px + 4px);
}

.scenario-workspace .step-dialog-sidebar .control-flow-wrap:hover .control-flow-parent .step-case-item {
  background: var(--el-fill-color-light);
}

.scenario-workspace .step-dialog-sidebar .step-case-item {
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

.scenario-workspace .step-dialog-sidebar .step-case-item:hover {
  background: var(--el-fill-color-light);
}

.scenario-workspace .step-dialog-sidebar .step-case-item.active {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.scenario-workspace .step-dialog-sidebar .step-delete-icon {
  visibility: hidden;
  flex-shrink: 0;
  align-self: center;
  cursor: pointer;
  color: var(--el-text-color-secondary);
  font-size: 16px;
}

.scenario-workspace .step-dialog-sidebar .step-case-row:hover .step-delete-icon {
  visibility: visible;
}

.scenario-workspace .step-dialog-sidebar .step-delete-icon:hover {
  color: var(--el-color-danger);
}

.scenario-workspace .step-dialog-sidebar .step-case-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.scenario-workspace .step-dialog-sidebar .step-seq-badge {
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

.scenario-workspace .step-dialog-sidebar .step-seq-badge:hover {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-8);
}

.scenario-workspace .step-dialog-sidebar .step-seq-badge:focus-visible {
  outline: 2px solid var(--el-color-primary-light-3);
  outline-offset: 1px;
}

.scenario-workspace .step-dialog-sidebar .step-seq-badge--disabled {
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  opacity: 0.85;
}

.scenario-workspace .step-dialog-sidebar .step-seq-badge--disabled:hover {
  border-color: var(--el-border-color);
  background: var(--el-fill-color);
}

.scenario-workspace .step-dialog-sidebar .step-type-tag.el-tag.el-tag--small {
  flex-shrink: 0;
  --el-tag-font-size: 11px;
  font-size: 11px;
  line-height: 1;
  height: 20px;
  min-height: 20px;
  padding: 0 6px;
  box-sizing: border-box;
}

.scenario-workspace .step-dialog-sidebar .step-case-title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}
</style>
