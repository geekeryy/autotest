<template>
  <div class="page-card template-reference">
    <div class="page-header">
      <div>
        <h2 class="page-title">模板与变量参考</h2>
        <p class="page-subtitle">
          梳理平台所有可在请求模板、场景步骤和 Mock 响应中使用的占位符、引用与函数。可直接复制示例到对应输入框。
        </p>
      </div>
    </div>

    <el-alert class="pipeline-alert" type="info" show-icon :closable="false">
      <template #title>渲染顺序（Runner 在每次发请求前依次执行）</template>
      <ol class="pipeline-list">
        <li v-for="step in renderingPipeline" :key="step.stage">
          <strong>{{ step.stage }}.</strong>
          <code class="pipeline-label">{{ step.label }}</code>
          <span class="pipeline-detail">{{ step.detail }}</span>
        </li>
      </ol>
      <p class="pipeline-note">
        以上顺序保证：模拟标签先生成具体值，再被场景引用 / SQL 引用 / 普通变量替换。互不干扰。
      </p>
    </el-alert>

    <el-tabs v-model="activeTab" type="border-card" class="reference-tabs">
      <!-- 1. 普通变量 -->
      <el-tab-pane name="variables" label="环境与运行变量">
        <section class="ref-section">
          <h3 class="ref-section-title"><code v-pre>{{varName}}</code></h3>
          <p class="ref-section-desc">
            最常用的占位符。运行时在以下来源中按优先级合并查找同名变量：<strong>请求覆盖变量 &gt; 模板默认变量 &gt; 场景变量 &gt; 环境变量 JSON</strong>。
            找不到的占位符会原样保留（前端渲染或 Runner 报错）。
          </p>
          <div class="ref-grid">
            <div class="ref-card">
              <div class="ref-card-title">生效范围</div>
              <ul class="ref-bullet-list">
                <li>请求模板：URL / Path / Query / Header / Body / 认证 token</li>
                <li>环境变量、场景变量、运行覆盖变量本身的值（支持嵌套，最多 5 层展开）</li>
                <li>SQL 参数源入参 JSON 的 <code>value</code> 字段</li>
                <li>断言脚本与场景脚本步骤中可通过 <code>pm.variables.get</code> 读取</li>
              </ul>
            </div>
            <div class="ref-card">
              <div class="ref-card-title">示例</div>
              <pre v-pre class="ref-code">{
  "Authorization": "Bearer {{token}}",
  "user": {
    "id": "{{userId}}",
    "tenant": "tenant-{{tenantId}}"
  }
}</pre>
            </div>
          </div>
        </section>
      </el-tab-pane>

      <!-- 2. 场景步骤引用 -->
      <el-tab-pane name="steps" label="场景步骤引用">
        <section class="ref-section">
          <h3 class="ref-section-title"><code v-pre>{{$steps[N].xxx}}</code></h3>
          <p class="ref-section-desc">
            仅在<strong>场景编排</strong>中生效。N 为左侧 <strong>#序号</strong> 标签上展示的稳定 step_seq，重排步骤后引用关系不会失效。
            子路径采用类 JSONPath 形式，支持 <code>a.b[0].c</code>、<code>arr[2]</code>。
          </p>
          <el-table :data="stepRefFields" border class="ref-table" :show-header="true">
            <el-table-column label="字段" min-width="240">
              <template #default="{ row }"><code class="ref-token">{{ row.name }}</code></template>
            </el-table-column>
            <el-table-column label="示例" min-width="320">
              <template #default="{ row }"><code class="ref-token">{{ row.example }}</code></template>
            </el-table-column>
            <el-table-column label="说明" min-width="280" prop="description" />
          </el-table>
          <p class="ref-tip">
            <strong>提示：</strong>API 步骤的输出同时保存了响应（status / headers / body）与本步骤实际发出的请求快照（request.query / request.pathvar / request.body），便于把请求侧字段直接传给后续步骤。
          </p>
        </section>
      </el-tab-pane>

      <!-- 3. SQL 内联引用 -->
      <el-tab-pane name="sql" label="SQL 内联引用">
        <section class="ref-section">
          <h3 class="ref-section-title"><code v-pre>{{sql.&lt;sourceKey&gt;.&lt;column&gt;}}</code></h3>
          <p class="ref-section-desc">
            从「测试数据 / SQL 参数源」管理的查询里取值。Runner 在发请求前会自动扫描请求并执行用到的参数源，无需绑定式配置。
            <code>sourceKey</code> 在同项目同服务下唯一；按 ID 或历史名称引用作为兜底。
          </p>
          <el-table :data="sqlInlineFields" border class="ref-table" :show-header="true">
            <el-table-column label="语法" min-width="320">
              <template #default="{ row }"><code class="ref-token">{{ row.name }}</code></template>
            </el-table-column>
            <el-table-column label="示例" min-width="280">
              <template #default="{ row }"><code class="ref-token">{{ row.example }}</code></template>
            </el-table-column>
            <el-table-column label="说明" min-width="320" prop="description" />
          </el-table>
          <div class="ref-card ref-card-block">
            <div class="ref-card-title">生效范围</div>
            <ul class="ref-bullet-list">
              <li>请求模板的 Path / Query / Header / Body / 变量值</li>
              <li>场景 API 步骤的请求覆盖参数</li>
              <li>SQL 参数源入参 JSON（在另一个参数源中再引用同源数据时不能形成循环）</li>
            </ul>
          </div>
          <pre v-pre class="ref-code">// 默认取首行
GET /api/v1/users/{{sql.userSeed.id}}

// 等值过滤：从结果里挑出 status=active 的第一行
GET /api/v1/users/{{sql.userSeed[status=active].id}}</pre>
        </section>
      </el-tab-pane>

      <!-- 4. 路径变量（OpenAPI 风格） -->
      <el-tab-pane name="pathvar" label="路径变量">
        <section class="ref-section">
          <h3 class="ref-section-title"><code v-pre>{name}</code></h3>
          <p class="ref-section-desc">
            <strong>OpenAPI 单大括号风格</strong>，仅在请求路径中识别，例如 <code>/api/v1/users/{id}</code>。
            运行时由「路径变量」表（<code>Path Vars</code>）中同名行的「参数值」填充。未填值且无同名运行变量时保持占位符不变。
          </p>
          <div class="ref-card">
            <div class="ref-card-title">和 <code v-pre>{{varName}}</code> 的区别</div>
            <ul class="ref-bullet-list">
              <li><code>{id}</code> 仅在 URL Path 中识别；不会被 Query/Header/Body 渲染管线处理。</li>
              <li>如果在 Path 中希望使用其他来源（如 SQL/步骤引用/模拟标签），仍可写双大括号形式，例如 <code v-pre>/api/v1/users/{{$mock.uuid}}</code>。</li>
              <li>Path Vars 表中的同名条目优先级最高；未启用的行会被忽略。</li>
            </ul>
          </div>
        </section>
      </el-tab-pane>

      <!-- 5. 运行时模拟数据标签 -->
      <el-tab-pane name="mock" label="运行时模拟标签">
        <section class="ref-section">
          <h3 class="ref-section-title">
            <code v-pre>{{$mock.helper}}</code>
            <span class="ref-section-title-aux">/</span>
            <code v-pre>{{$mock.helper(args)}}</code>
          </h3>
          <p class="ref-section-desc">
            每次请求渲染时实时生成一个值，<strong>多次出现各自独立</strong>。helper 名大小写不敏感；参数支持单/双引号包裹（用于含逗号或空白的字符串）。
            未识别的 helper 保留原始字面量（避免与未来变量冲突）。
          </p>
          <div class="ref-card ref-card-block">
            <div class="ref-card-title">生效范围</div>
            <ul class="ref-bullet-list">
              <li>请求模板的 URL / Path / Query / Header / Body / 变量值</li>
              <li>场景编排所有 API 步骤的请求覆盖</li>
              <li>Mock Server 规则的响应体</li>
            </ul>
          </div>
          <el-input
            v-model="mockKeyword"
            placeholder="按 helper 名称或描述搜索"
            clearable
            class="ref-search"
          />
          <el-table :data="filteredMockHelpers" border class="ref-table" stripe>
            <el-table-column label="helper" width="160">
              <template #default="{ row }"><code class="ref-token">{{ row.name }}</code></template>
            </el-table-column>
            <el-table-column label="示例" min-width="320">
              <template #default="{ row }">
                <div class="ref-token-wrap">
                  <code class="ref-token">{{ row.example }}</code>
                  <el-button type="primary" link size="small" @click="copy(row.example)">复制</el-button>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="说明" min-width="280" prop="description" />
          </el-table>
          <pre v-pre class="ref-code">// 综合示例：每次发请求都会生成一组新的值
{
  "id": "{{$mock.uuid}}",
  "email": "{{$mock.email}}",
  "age": {{$mock.int(18,80)}},
  "createdAt": "{{$mock.now}}",
  "role": "{{$mock.pick(admin,tester,viewer)}}"
}</pre>
        </section>
      </el-tab-pane>

      <!-- 6. Mock 响应模板 -->
      <el-tab-pane name="mockresp" label="Mock 响应模板">
        <section class="ref-section">
          <h3 class="ref-section-title"><code v-pre>{{request.xxx}}</code></h3>
          <p class="ref-section-desc">
            仅在 <strong>Mock Server 规则的响应体</strong>中生效，用于把当前 mock 请求的方法、路径变量、查询参数、请求头与 JSON 请求体回显到响应中。
            如果引用的字段不存在或请求体不是合法 JSON，Mock 会返回 500 并提示出错的占位符。
          </p>
          <el-table :data="mockResponseFields" border class="ref-table">
            <el-table-column label="字段" min-width="220">
              <template #default="{ row }"><code class="ref-token">{{ row.name }}</code></template>
            </el-table-column>
            <el-table-column label="示例" min-width="280">
              <template #default="{ row }"><code class="ref-token">{{ row.example }}</code></template>
            </el-table-column>
            <el-table-column label="说明" min-width="320" prop="description" />
          </el-table>
          <p class="ref-tip">
            <strong>组合用法：</strong>响应体内可以同时使用 <code v-pre>{{request.*}}</code> 与 <code v-pre>{{$mock.*}}</code>，例如返回 <code v-pre>{"requestId":"{{$mock.uuid}}","echoId":"{{request.pathvar.id}}"}</code>，前者每次请求新生成、后者来自当前请求。
          </p>
        </section>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
import {
  mockHelperList,
  stepRefFields,
  sqlInlineFields,
  mockResponseFields,
  renderingPipeline
} from '../../utils/templateTokens'

export default {
  name: 'TemplateReference',
  data() {
    return {
      activeTab: 'variables',
      mockKeyword: '',
      stepRefFields,
      sqlInlineFields,
      mockResponseFields,
      renderingPipeline
    }
  },
  computed: {
    filteredMockHelpers() {
      const keyword = this.mockKeyword.trim().toLowerCase()
      if (!keyword) return mockHelperList
      return mockHelperList.filter((helper) =>
        helper.name.toLowerCase().includes(keyword) ||
        helper.description.toLowerCase().includes(keyword) ||
        helper.example.toLowerCase().includes(keyword)
      )
    }
  },
  methods: {
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
.template-reference {
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

.pipeline-alert {
  align-items: flex-start;
}

.pipeline-list {
  margin: 4px 0 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pipeline-list li {
  list-style: none;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
}

.pipeline-label {
  background: var(--app-fill-color-light, #f1f5f9);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
}

.pipeline-detail {
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  flex: 1 1 240px;
}

.pipeline-note {
  margin: 8px 0 0;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text);
}

.reference-tabs {
  border-radius: 8px;
}

.reference-tabs :deep(.el-tabs__content) {
  padding: 16px;
}

.ref-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ref-section-title {
  margin: 0;
  font-size: var(--el-font-size-medium);
  font-weight: 600;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: baseline;
}

.ref-section-title code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--el-font-size-medium);
  background: var(--app-fill-color-light, #f1f5f9);
  padding: 1px 6px;
  border-radius: 4px;
}

.ref-section-title-aux {
  color: var(--app-secondary-text);
}

.ref-section-desc {
  margin: 0;
  color: var(--app-secondary-text);
  font-size: var(--app-font-size-small);
  line-height: 1.6;
}

.ref-grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(280px, 1fr);
  gap: 12px;
}

.ref-card {
  background: var(--app-fill-color-light, #f8fafc);
  border-radius: 8px;
  padding: 12px 14px;
}

.ref-card-block {
  width: 100%;
}

.ref-card-title {
  font-weight: 600;
  margin-bottom: 6px;
  font-size: var(--app-font-size-small);
}

.ref-bullet-list {
  margin: 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text);
  line-height: 1.55;
}

.ref-bullet-list li code {
  background: var(--el-fill-color, #eef2f7);
  padding: 0 4px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.ref-table :deep(.el-table__cell) {
  vertical-align: top;
}

.ref-token {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  color: var(--app-primary-text);
  word-break: break-all;
}

.ref-token-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ref-search {
  max-width: 320px;
}

.ref-code {
  margin: 0;
  background: var(--app-fill-color-light, #0f172a08);
  color: var(--app-primary-text);
  border-radius: 6px;
  padding: 12px 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--app-font-size-small);
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  border: 1px solid var(--app-border-color);
}

.ref-tip {
  margin: 0;
  font-size: var(--app-font-size-small);
  color: var(--app-secondary-text);
  line-height: 1.55;
}

.ref-tip code {
  background: var(--el-fill-color, #eef2f7);
  padding: 0 4px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

@media (max-width: 960px) {
  .ref-grid {
    grid-template-columns: 1fr;
  }
}
</style>
