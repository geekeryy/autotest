# AI 能力设计

本文档记录 AI 提供商、项目级 Prompt、AI 生成与 AI 分析能力的业务设计。

## AI 提供商

- 平台支持**全局** AI 提供商配置（不按项目隔离），在「平台资源 → AI 提供商」维护。
- 提供商类型包括 deepseek、xiaomi、openai、anthropic、kimi、ollama。
- 全平台可维护多份 AI 提供商配置。
- 配置包含名称、Base URL、API Key 脱敏、**文本默认模型**、按能力配置的默认模型（`modalityModels.image` / `audio` / `video`，存于 `extra_config`）、extraConfig、启用状态与是否默认。
- `GET .../models` 与 discover 返回的每条模型可含 `capabilities` 标签（`text` / `image` / `audio` / `video`），**仅**解析上游响应中的元数据字段，不按模型 id 推断；多模态路由完全依赖用户在 `modalityModels` 中配置的型号。
- 全平台最多一个默认提供商。
- DeepSeek 与 Xiaomi 走 OpenAI 兼容协议时默认启用 thinking/reasoning：DeepSeek 请求携带 `thinking.type=enabled` 与 `reasoning_effort`，Xiaomi 请求携带 `reasoning.effort`。默认 effort 为 `high`；可通过 `extraConfig.thinking=false` 或 `extraConfig.thinking.enabled=false` 关闭，也可通过 `extraConfig.reasoning_effort` / `extraConfig.reasoningEffort` 或 `extraConfig.thinking.effort` 覆盖强度。
- 模型下拉不再依赖 `/ai-provider-types` 的静态列表：已保存配置走 `GET /ai-providers/{providerId}/models`；创建/编辑表单在填写 Base URL 与 API Key 后走 `POST /ai-providers/models/discover`（编辑时 apiKey 留空则复用库内密钥）。OpenAI 兼容类型调用上游 `GET {baseUrl}/models`，Anthropic 调用 `GET {baseUrl}/models`，Ollama 在 `/v1/models` 不可用时回退 `GET /api/tags`；上游失败时返回内置 fallback 列表与 `warning`。
- 管理 API 使用全局 RBAC：`projects:read` 可读，`projects:write` 可写（与项目管理权限一致）。

## AI 助理平台配置

- 在「平台资源 → AI 助理配置」（`/platform/ai-assistant-settings`）维护 AI 助理运行时参数，持久化于 `ai_assistant_settings` 表（singleton 行，`config` jsonb）。
- 管理 API：`GET/PUT /ai-assistant-settings`；`GET` 返回当前值 + 各字段 `label` / `description` / `type` / 默认值元数据，供表单渲染；`PUT` 保存后立即刷新内存配置，**无需重启**。
- 可配置项（实现于 `internal/aiconfig`）：
  - **工具调用最大轮次**（`maxToolHops`，默认 6）：单次对话/分析的工具循环上限。
  - **按需工具路由模式**（`toolRoutingMode`，默认 `shadow`）：见下文「灰度开关」。
  - **路由置信度阈值**（`routerConfidenceThreshold`，默认 0.7）：`dynamic_fallback` 模式下低于此值回退全量工具。
  - **场景生成真实环境验证**（`scenarioAutorunEnabled`，默认 off）：控制 `generate_and_verify_scenarios` 是否允许在真实环境执行。
  - **场景闭环默认最大轮次**（`scenarioGenDefaultMaxRounds`，默认 3）：genagent 未指定 `maxRounds` 时的默认值。
  - **工具向量检索**（`toolEmbedBaseUrl` / `toolEmbedApiKey` / `toolEmbedModel`）：`find_tools` 查询向量；Key 类字段 API 脱敏返回。
- 场景生成**登录凭据不在此配置**：须通过 `list_scenario_login_hints` 从已保存用例/环境 auth 发现候选，由用户在 `generate_and_verify_scenarios` 确认面板填写 `loginCredentials`，或生成后用 `update_scenario_step` 补全占位符。
- 优先级：数据库已保存值 > 环境变量（启动默认）> 代码默认值。环境变量主要用于首次部署或未配置后台前的 bootstrap。
- 权限同 AI 提供商：`projects:read` 可读，`projects:write` 可写。

## 平台 Prompt

- 平台 Prompt（`project_ai_prompts` 表，历史表名保留）按 **action** 全局唯一，维护 System Prompt / 默认模型。
- 可选绑定 `providerId`；留空时跟随平台默认 AI 提供商。
- 管理 API：`GET/POST /ai-prompts`、`PUT/DELETE /ai-prompts/{promptID}`，权限同 AI 提供商。

## 统一 AI 入口

- `/projects/{projectID}/ai/chat` 统一 AI 入口支持 `generate_params`、`generate_assertion`、`generate_case_data`、`analyze_failure`、`analyze_spec_changes` 等 action。
- 运行控制台的「一键生成参数」还存在非 LLM 采样接口 `GET /cases/{caseID}/generate-params`；它与 LLM `generate_params` 是两条路径，文档和实现变更时需要区分。
- `generate_assertion` 必须在弹窗中填写非空测试意图，后端拒绝空意图。
- `generate_case_data` 使用平台 Prompt 配置。
- `generate_case_data` 未配置 provider 或 prompt 时返回明确中文错误。

## 生成请求参数

- `generate_params` 的 context 必须基于 `api_endpoints` 真实存在字段构造。
- 禁止引用 endpoint 上未定义属性。
- context 至少包含 method、path、pathVarNames、endpoint summary/operationId/tags/requestSchema/responseSchema、currentRequest。
- Mock Value Sets 摘要可注入 `generate_params` 上下文，供模型优先选择 `{{$mock.set.<key>}}`。
- 系统提示需声明字段含义。
- 系统提示需要求模型保留 currentRequest 中已存在且非空的值，仅补全缺失字段。
- 系统提示需禁止 markdown 围栏。

## AI 分析

- 测试失败分析基于本次 run 请求/响应快照、断言失败明细、失败步骤摘要调用 AI。
- 测试失败分析返回原因推断与修复建议。
- spec 变更影响分析基于本次导入 spec 与同一 service 上一条 spec 对比。
- spec 变更影响分析结合当前服务下接口模板与场景步骤摘要输出影响评估与建议动作。
- 当前既有 `/ai/chat` action，也有专用 HTTP 分析入口（如运行失败分析、spec 变更分析）;分析结果当前不写库。
- 分析侧 AI 可基于内置只读工具按需补充上下文（见下方「AI Tool Calling 框架」与「智能分析的工具增强」）。
- 分析输出统一为中文 Markdown 结构；前端使用 `MarkdownView`（marked + DOMPurify）渲染，禁止注入未经净化的 HTML。

## AI Tool Calling 框架

- 平台为分析类与对话型 AI 入口提供统一的 Tool Calling 框架，工具实现位于 `internal/aitools`。
- 工具描述使用 JSON Schema（draft-07 子集），由 `internal/aiprovider/client` 在调用 LLM 时按 OpenAI `tools/tool_calls` 与 Anthropic `tool_use` 双协议自动适配,调用方无需关心 provider 差异。
- 工具元数据携带 `Mutating` 与 `RequiresConfirm`：只读工具直接执行；**create/update 类写工具**（`Mutating=true` 且 `RequiresConfirm=false`）在对话流中自动执行；**delete_* 等删除类写工具**（`RequiresConfirm=true`）必须经过下文「写工具确认」的人在回路流程。任何分析类 action 都只允许暴露只读工具。
- 工具循环最多 `maxToolHops` 跳（默认 6）；达到上限仍未给出结论时返回中文提示并停止，不允许无限工具调用。该值可在 **平台资源 → AI 助理配置** 调整，持久化于 `ai_assistant_settings` 表。
- 工具运行错误以 `{"error":"..."}` 形式注入到对话中,模型据此自行决策是否换工具或停止;错误本身不算 LLM 调用失败,不应让整个 action 中断。
- 工具调用沿用调用方 `context.Context` 上的 `*auth.Principal` 与 `ProjectRoleFromContext` 进行权限判定。在此之上还有一层"会话项目隔离"——SSE handler 与分析类入口在调用工具循环前都会通过 `aitools.WithCaller(ctx, CallerContext{UserID, ProjectID})` 把当前用户与会话项目 ID 写入 ctx；工具实现统一调 `aitools.ResolveProjectID(ctx, "")`（或拿到对象 projectId 后 `aitools.RequireProjectAccess(ctx, projectID)`）做强校验，确保 AI 既无法跨项目操作、也不需要也无法手动指定项目。
- 所有内置工具的 JSON Schema 都 **不暴露** `projectId` 字段（`additionalProperties: false`），模型既看不到这个参数也无法绕过；项目归属只能从会话 `CallerContext` 推断。这样可以彻底避免模型出于"礼貌"反复追问用户项目 ID，也避免任何精心构造的 projectId 进入工具调用链路。仅按 `caseId/scenarioId/stepId` 操作的写工具在查到对象后仍会反向比较其 `projectId` 与会话项目，不一致直接拒绝。

## 智能分析的工具增强

- `analyze_failure` 与 `analyze_spec_changes` 在请求阶段挂载 `builtin.ReadOnly(deps)` 提供的只读工具集（包含 `get_case` / `get_scenario` / `get_endpoint`，以及 `list_services` / `list_endpoints` / `list_cases` / `list_scenarios` / `list_environments` 等通用 discovery 工具）。
- 服务端继续负责一次性聚合基础上下文(spec diff、affectedTemplates、affectedScenarioSteps 等),保证分析结果的可重复性;AI 仅按需调用工具补全个别字段细节。
- spec 变更分析的上下文新增顶层 `projectId` 与 `serviceId`,使 AI 在调用 `get_endpoint` 时无需从嵌套结构里推断 ID。
- 失败分析中若 `stepResults` 引用了 `testCaseId` 或 `scenarioId`,AI 可调用 `get_case` / `get_scenario` 拉补充上下文;工具结果与原始证据等价,但应在产出中显式注明。
- 分析类 action 永远只挂载只读工具，不暴露任何 mutating 工具，避免分析过程产生意外写入。

## 内置工具集（`internal/aitools/builtin`）

按域拆分为多个源文件（`builtin_meta` / `builtin_cases` / `builtin_scenarios` / `builtin_projects` / `builtin_spec` / `builtin_mockserver` / `builtin_mockset` / `builtin_testdata` / `builtin_paramsource` / `builtin_scripts` / `builtin_runs` 等），入口为 `builtin.go` 的 `ReadOnly` / `Mutating` / `All`。

工具分类：

- **只读**（discovery / 查询，分析类与浮窗均可用）：服务/接口/环境/Spec、接口模板、场景、Mock、测试数据、SQL 参数源、脚本库、运行历史等 `list_*` / `get_*` / `preview_*`。
- **自动写**（`Mutating` 且 `RequiresConfirm=false`）：create/update/reorder/import 等，在 SSE 工具循环中立即执行，无需用户点确认。
- **删除写**（`RequiresConfirm=true`，名称通常为 `delete_*`）：删除服务/环境、场景、步骤、Mock、测试数据表、数据源、脚本模板等，浮窗展示确认卡片，用户批准后才执行。
- 平台 **不暴露**「运行」类工具（单接口/场景执行仍由用户手动触发）。
- **例外（gated）**：当 **AI 助理配置** 中开启「场景生成真实环境验证」时，对话工具集注册 `generate_and_verify_scenarios`（运行时仍校验开关）；可在用户指定 `environmentId` 的真实环境上异步执行「生成→验证→（可选）自动修复」闭环；该工具不对分析类 action 暴露，且工具描述含真实环境数据变更告警。
- 平台 **不暴露** 用户管理相关写能力：用户/角色/权限/登录方式/API Key 等系统管理接口不在工具集中。

完整工具名以 `builtin.All()` 注册顺序为准（约 60+ 个）；新增工具须遵循 `additionalProperties: false`、不暴露 `projectId`、删除类使用 `deleteTool` helper。

## AI 场景生成与编排

- 入口：用户在浮窗里描述测试目标（如"帮我生成一个用户登录后下单的场景"），AI 沿以下工作流推进：
  1. 调用 `list_services` / `list_endpoints` 找到目标接口；禁止凭空捏造 path/method。
  2. 调用 `list_cases` 判定哪些接口已有可运行的请求模板；对缺失接口调 `create_case_from_endpoint` 创建模板（可附基础断言，例如 `status==200`）。
  3. 调用 `create_scenario_with_steps` 一次性创建场景 + 全部步骤，用户在 UI 上 confirm 一次即生效。
  4. 用户提出调整时，再用 `add_scenario_step` / `update_scenario_step` / `delete_scenario_step` / `reorder_scenario_steps` 做细粒度 refine；每一次写都仍走 confirm。
- 步骤类型与 `config` 形态：
  - `api`：必填 `testCaseId`，`config` 一般为 `{}`，可选 `requestOverride`。
  - `script`：`config` 形如 `{"script": "pm.test(...)", "timeoutMillis": 5000}`；脚本为 Postman 风格 JavaScript（goja 沙箱）。
  - `for`：`config` 关键字段 `mode=count|items`、`count` / `countExpression` / `items` / `itemsExpression`、`itemVar` / `indexVar`、`bodyStepOrders`、`maxIterations`。
  - `condition`：`config` 关键字段 `branches=[{left, operator, right, stepOrders}, ...]` 或旧版 `left/operator/right/thenStepOrders`、`elseStepOrders`。
- 控制流子步骤引用使用 `stepOrder`：AI 在调用工具时只需知道本次场景里其它步骤的 `stepOrder`，工具内部会按"创建后回填 `step_seq`"两阶段写入：
  1. 第一阶段按 `stepOrder` 顺序把所有步骤写入 DB，拿到 `stepOrder → stepSeq` 映射；
  2. 第二阶段把控制流步骤里的 `bodyStepOrders` / `thenStepOrders` / `elseStepOrders` / `branches[].stepOrders` 改写为 `bodyStepSeqs` / `thenStepSeqs` / `elseStepSeqs` / `branches[].stepSeqs`，再 UpsertStep 一次。
  - 如果模型直接使用 `*Seqs` 字段，平台不会再做任何改写。
- `update_scenario_step` 采用"读旧值-合并-Upsert"流程：未传字段保留现值；传 `null` 与未传同义；如改变 `stepType` 必须同时给出该类型必填的字段（如 API 的 `testCaseId`）。
- 失败处理：`create_scenario_with_steps` 在第 N 步写入失败时不会回滚已创建的场景与前 N-1 步，错误信息中会返回 `scenarioId` 与已写入的步骤数，便于用户在 UI 里继续编辑或要求 AI 用细粒度工具补救。

## 场景生成登录凭据（不平台写死）

- 登录用户名/密码**不得**写入 AI 助理平台配置或环境变量默认值；每个项目/环境的凭据来自运行时数据或用户确认。
- 发现候选：`list_scenario_login_hints`（AI 工具）与 `GET /projects/{id}/services/{sid}/scenario-login-hints?environmentId=`（管理 UI）扫描：
  - 已保存的登录接口模板 request body；
  - 目标环境的 auth profiles（basic 类型 username/password）；
  - 环境 variables 中含 USER/LOGIN/PASS 的键（密码脱敏）。
- 生成策略（`scenariogen.GenerateCoverage`）：登录步优先复用 saved case body → 用户经确认面板 `loginCredentials` 传入的值（含 `body` 扩展字段，如 code/union_id/phone_code）→ 标准占位符 `__FILL_<field>__`（仅后端写入，AI 禁止自行编造）。
- 真实环境闭环：`generate_and_verify_scenarios` 与 `generate_coverage_scenarios` 均为 **RequiresConfirm**，在**对话流内嵌确认卡片**（`AIScenarioGenConfirm`，Cursor 式交互）；用户于卡片内选择服务/环境、点击 enum 选项或输入登录 body 字段，点「允许执行」后通过 `/tool-calls/{id}/confirm` 的 `toolArguments` 传入再执行。
- AI 助理 prompt 要求：**禁止**在对话中逐条追问登录/OAuth/业务 ID；应直接挂起 `generate_coverage_scenarios` / `generate_and_verify_scenarios`，在确认卡片（`AIScenarioGenConfirm`）中收集 `loginCredentials`。可选只读 `list_scenario_login_hints` 供卡片展示字段；路径/业务 ID 由依赖图注入引用，勿在聊天里问用户是否手工加前置步骤。

## 依赖图驱动覆盖生成（阶段 1）

- `internal/scenariogen/depgraph` 从 `api_endpoints` 的 `requestSchema` / `responseSchema` 推断 producer→consumer 依赖（路径参数 `{id}`、body 外键、响应 `id`/`*Id` 字段），并按 OpenAPI tags 分组、CRUD/鉴权优先级拓扑排序。
- `generate_coverage_scenarios` / `scenariogen.Generator.GenerateCoverage` 使用该依赖图自动生成 `requestOverride`（Bearer 链、路径 ID 引用），替代原先 E2E 硬编码 path 与账号；登录凭据优先复用**已保存登录用例** request body，否则写入 `__FILL_*` 占位符，并应通过 `list_scenario_login_hints` + 用户确认或 `update_scenario_step` 补全。

## 执行-修复闭环 Agent（阶段 3）

- `internal/genagent` 提供异步任务：阶段 1 生成场景 → `runner.RunScenario` 在真实 `environmentId` 上执行 → LLM 分类失败（测试数据 / 生成 bug / 真实缺陷）→ 对前两类自动修复步骤 → 重跑，直到全绿或达 `maxRounds`（默认 3）。
- 运行权限双重 gate：用户 `autoRepair` 开关 + 平台 **AI 助理配置** 中「场景生成真实环境验证」（默认 off）；常规对话工具集仍不含其它「运行」工具；闭环在 Agent 内部 bypass 工具轮次上限与写工具 confirm。
- 持久化：`scenario_gen_jobs`（migration 017），按 `project_id` 隔离；SSE 新增 `gen_job_progress` 事件（轮次、通过率、修复摘要），不改变既有事件语义。
- gated 工具 `generate_and_verify_scenarios`：入参 `serviceId` / `environmentId` / `autoRepair` / `maxRounds`；运行时校验平台开关，未开启则拒绝执行。

## AI 优先入口页

- 登录后默认进入 `/`（`AIHome.vue`）：类 DeepSeek Chat 居中对话 + 会话侧栏 + 推荐 prompt；复用 `aiAssistant` store 与 `GlobalAIAssistant`。
- 顶栏提供「进入控制台」跳转 `/dashboard`（现有 AdminLayout 手工管理页不变）。
- 生成工作流：`AIScenarioGenLauncher` 选择服务/环境、自动修复开关与最大轮次；`AIScenarioGenProgress` 展示 SSE `gen_job_progress` 流式进度卡片。

## 页面上下文（Page Context）

- 浮窗对话支持"AI 感知用户当前页面"。前端通过 `web/admin/src/stores/aiAssistant.js` 暴露 `bindPage(snapshot)` 与 `enrichPage(patch)` 两个 action：
  - `router/index.js` 的 `afterEach` 钩子在每次路由切换时调一次 `bindPage`，写入基础字段（`path`、`routeTitle`，以及路由参数中的 `scenarioId` / `caseId`）；
  - 各业务页面（场景列表、API 管理、运行控制台等）在数据加载完成后通过 `enrichPage` 追加更具语义的字段（如 `scenarioName`、`caseName`、`serviceId`），让 AI 拿到"用户当前看的对象的可读标签"。
- 每次 `POST /ai/chat/stream` 与 `/ai/tool-calls/{id}/confirm` 都会把 `assistantState.pageContext` 序列化进请求体的 `pageContext` 字段；后端 `AssistantStreamRequest.PageContext` 是不持久化的 `json.RawMessage`，每轮请求都使用最新快照。
- 后端 `renderPageContextSystem` 把 `pageContext` 渲染为一条"`## 用户当前页面上下文`"开头的 system 消息，插入到主 system prompt 之后，供 LLM 在用户使用代词或省略对象 ID 时把页面快照里的 `scenarioId/caseId/serviceId` 作为默认对象。
- **页面上下文只是提示，不是授权依据**：系统 prompt 明确告知 AI 该字段只用于消歧，工具参数仍以模型显式传入为权威，最终的项目归属和权限校验由 `aitools.ResolveProjectID` / `aitools.RequireProjectAccess` 把关。
- 浮窗 UI 在头部下方显示一行"当前上下文：…"提示，方便用户直观看到 AI 正在感知什么；`pageContext` 为空或只剩占位字段时该提示自动隐藏。

## AI 助理工作区（全页分屏）

- 管理后台提供独立 **AI 助理工作区页**（`/ai-assistant`），与右侧浮窗共用 `assistantState` 与 SSE 对话能力，但工作区 pane（`w0`…`w5`）与浮窗 pane（`panel`）会话、模型设置相互隔离。
- **分屏**：默认单屏（`w0`）；末屏标题栏 **+** 可增至最多 **6** 屏，**X** 关闭该屏（至少保留 1 屏）。分屏由 `workspacePaneIds.length > 1` 隐式判定，无侧栏开关。窄屏（&lt;900px）时用 Tab「对话 1…6」（按槽位 `w0`…`w5` 固定编号，不随增删重排）切换，宽屏横向并排展示。
- **每屏独立模型**：各工作区 pane 在分屏标题栏 **设置** 图标打开的 `ModelSettingsPopover` 中选择 **提供商 + 模型** 与 Debug；深度思考、联网搜索、图片附件仍在本屏 composer 工具栏中，读写各自 pane 的 `thinkingEnabled` / `webSearchEnabled` / `debugEnabled`。（浮窗 pane 的模型与 Debug 在顶栏同一 Popover 中。）切换提供商/模型时 Popover 保持打开，仅点击外部区域或再次点击设置按钮时关闭。
- **会话**：同一会话不可在多个工作区分屏同时打开；会话列表 tag 显示所在分屏序号（1…6）。pane 级设置与会话 activeId 按 `w0`…`w5` 持久化；自旧版 `left`/`right` 迁移为 `w0`/`w1`。**新对话**：当前 pane 已有活跃会话且无任何 user/assistant/tool 轮次（与 UI 可见消息口径一致，不含 system）时，触发「新对话」不再 POST 创建会话，静默复用当前空会话；浮窗 pane（`panel`）同理。

## 全局 AI 助理浮窗（对话型入口）

- 登录后管理后台提供一个全局 AI 助理浮窗，对话上下文按「当前项目 + 当前登录用户」隔离，跨用户/跨项目互不可见。
- 浮窗对话挂载在统一的 `assistant_chat` action 上，使用独立 system prompt（`assistantChatSystem`），引导模型以 Markdown 回应并主动调用工具补充上下文。
- 浮窗输入区提供本次对话的 AI 设置：用户可选择当前项目下已启用的 AI 提供商，并在该提供商的推荐模型列表中选择或手工输入模型名；请求会携带 `providerId` 与 `model`，未选择时回退项目默认提供商和 provider 默认模型。
- 平台在支持的提供商上**始终**收集上游 `usage` 元数据并写入 `ai_messages.usage_details`（OpenAI 兼容流式请求启用 `stream_options.include_usage`；Anthropic 从 `message_delta.usage` 解析），供首页仪表盘、会话详情 Token 汇总等统计使用。
- 模型设置中提供 **Debug** 开关（按项目本地持久化）。开启后请求携带 `debugEnabled=true`，后端额外通过 SSE `usage` 事件推送每轮明细；前端在浮窗每条 assistant 回复下展示输入/输出/合计 Token、**缓存命中**（归并 `cached_tokens`、`prompt_cache_hit_tokens`、`cache_read_input_tokens` 等）、**缓存未命中**、**缓存写入**、推理 Token 与耗时，并可展开查看上游原始 JSON。关闭 Debug 不影响用量落库与汇总统计。
- 当所选提供商为 DeepSeek 或 Xiaomi 时，浮窗参考 DeepSeek Chat 暴露「深度思考」开关。该开关按请求发送 `thinkingEnabled` 与 `reasoningEffort=high`，关闭时本轮不发送 thinking/reasoning 参数；其它 provider 不展示该开关。
- 当所选提供商为 Xiaomi 时，浮窗额外暴露「联网搜索」开关。开启后请求在 tools 中附带小米内置 `web_search` 工具（由平台执行并注入结果，assistant 循环不本地执行该工具）；关闭时不附带。联网搜索按次计费，与 Token 计费独立。
- 输入区支持上传图片（JPEG/PNG/GIF/WebP，单张 ≤5MB，每条消息最多 4 张），**前提是当前提供商已在设置中配置 `modalityModels.image`**。前端以 base64 data URL 随 `POST /ai/chat/stream` 的 `images` 提交；后端写入 `ai_messages.attachments`，并以 OpenAI 多模态 `content`（`text` + `image_url`）调用模型。含图时服务端按提供商配置的**图片默认模型**自动切换（与上游 `/models` 列表按 id 规范化匹配），纯文本轮次仍用用户所选或文本默认模型；不要求用户手动改下拉框。历史会话通过 `attachments` 回显缩略图。音频/视频路由字段已预留，待附件类型扩展后生效。
- 深度思考开启后，前端在 assistant 占位消息中展示「正在深度思考」状态与耗时；后端仅通过 SSE `thinking` 事件同步 active/inactive 状态，不向前端暴露 `reasoning_content` 文本。
- 浮窗会话由 `ai_sessions` 与 `ai_messages` 两张表持久化（见 `migrations/001_schema.sql`）：
  - `ai_sessions` 记录会话归属与标题；标题由后端在**第 1、2 条用户消息**时自动生成（第 1 条为主、第 2 条为辅精炼），第 3 条及之后不再更新；采用 few-shot「只输出标题」提示词，且不使用 reasoning 链作为标题；若模型仍输出分析口吻（如「首先，用户的指令是…」）则后处理剥离或回退为首问截断。
  - `ai_messages` 记录所有 LLM 对话轮（system/user/assistant/tool）与 `tool_calls` 元数据，并通过 `status` 字段区分 `final` / `pending_confirm` / `rejected`。
- 浮窗的 API 入口：
  - `GET/POST /projects/{projectID}/ai/sessions`、`GET/PATCH/DELETE /projects/{projectID}/ai/sessions/{sessionID}`：会话 CRUD。
  - `GET /projects/{projectID}/ai/sessions/{sessionID}/stats`：会话详情统计（对话条数、工具调用、待确认、模型列表、Token 汇总）；Token 汇总来自 `usage_details`（有上游用量数据的 assistant 轮次）。
  - `GET /projects/{projectID}/ai/sessions/token-usage`：当前用户在项目内的 AI Token 消耗汇总（跨会话聚合）；`GET /me/ai/token-usage` 为全部项目汇总。供首页仪表盘展示，数据口径同上。
  - `POST /projects/{projectID}/ai/chat/stream`：发起对话并以 SSE 返回模型增量输出（含工具调用事件）。
  - `POST /projects/{projectID}/ai/tool-calls/{callID}/confirm`：用户对挂起的写工具调用做出 approve/reject 决策，并以同样的 SSE schema 恢复对话。

## SSE 事件 schema

- 对话流采用 Server-Sent Events，事件字段 `event:` 与 JSON 负载里的 `kind` 一致，统一在 `aiprovider.AssistantStreamEvent` 中定义：
  - `message`：刚持久化进 `ai_messages` 的一条记录（user / assistant / tool）。前端据此更新本地会话。
  - `text`：assistant 文本增量。
  - `thinking`：支持 thinking 的 provider 正在输出或结束输出 `reasoning_content`；负载只包含 `active` 与可选 `elapsedMillis`，不包含思维链原文。
  - `tool_call`：只读工具被立即执行前的通知（含 `id`/`name`/`arguments`/`mutating=false`）。
  - `tool_result`：单次工具执行的结果（成功或失败均以 JSON 字符串放在 `content` 中）。
  - `pending_confirm`：写工具调用挂起，等待用户在 UI 上确认。负载里的 `toolCall.mutating=true`，调用真正执行前不会改动平台数据。
  - `usage`：单轮 LLM 调用的 token / 缓存明细（仅 `debugEnabled` 时推送，用于浮窗实时展示；负载为 `AssistantUsageDetail`）。用量仍会写入 `ai_messages.usage_details`。
  - `done`：本次流结束，`finish` 字段在 `stop` / `tool_calls` / `pending_confirm` / `hop_budget_exhausted` / `error` 之中。
  - `gen_job_progress`：场景生成/验证异步任务进度（`jobId` / `round` / `maxRounds` / `phase` / `passRate` / `repairs`）；由 `generate_and_verify_scenarios` 触发，不改变其它事件语义。
  - `session`：会话元数据更新（如自动生成的标题）；前端据此刷新侧栏会话名。
  - `error`：致命错误；前端应在收到后展示错误并允许重试。
- 全局 chi `Timeout(60s)` 通过 `timeoutExceptStream` 中间件主动放行 `chat/stream` 与 `tool-calls/.../confirm` 路径,防止把长连接掐断。

## 写工具确认（Human-in-the-loop）

- 浮窗暴露的工具集包含只读 + 受控写（mutating）两类。当前的写工具集合见上方「内置工具集」与「AI 场景生成与编排」章节，覆盖接口模板创建/断言更新与场景编排全套（增删改/重排）。后续新增写工具时须遵循同样的契约。
- 写工具不会被分析类 action 看到；只在 `assistant_chat` 行为下可见。
- 当一轮 LLM 输出包含任意 mutating tool 调用时，流程会:
  1. 把当前 assistant 回复持久化为 `status=pending_confirm`,`tool_calls` 字段保存所有调用元数据;
  2. 通过 SSE 发出 `pending_confirm` 事件并结束本次流;
  3. 等待前端调用 `POST /ai/tool-calls/{callID}/confirm`,携带 `approve` 与可选 `reason`;
  4. 后端在 `ContinueAfterConfirm` 中执行（或跳过）该工具,把 tool 消息持久化为 `final`,然后继续 SSE 直到模型给出最终回复。
- 若用户 reject，平台不会执行工具;tool 消息内容为 `{"error":"...","rejected":"true"}` 风格的 JSON，模型据此调整后续输出（如改用只读工具或建议人工处理）。
- 一轮里同时出现只读 + mutating 调用时，整轮挂起;只读调用不会被偷偷执行，以保证 UI 上一次只确认一组相关写操作的同时,不让模型「绕过」用户期望的顺序。

## 会话与消息持久化

- `ai_sessions` 隔离维度为 `(project_id, user_id)`;查询/写入都通过 `aisession.Service` 强校验 `user_id` 与请求登录态一致。删除采用软删（`deleted_at`），便于后续审计。
- `ai_messages` 通过自增 `seq`（在 `session_id` 范围内单调递增）保证前端增量渲染稳定。`tool_calls`（assistant 端）与 `tool_call_id`（tool 端）共同维护 OpenAI / Anthropic 都能复现的对话历史。
- 写入会话历史的同时 `ai_sessions.updated_at` 会刷新,以便用户在"最近会话"列表里看到最新对话排在前面。
- 会话恢复时,`pending_confirm` 消息会被过滤掉再喂给 LLM,避免重复请求确认；已决定的写工具调用会保留对应 assistant `tool_calls` 与后续 tool 结果，保证 OpenAI 兼容协议的历史结构完整。
- 对支持 thinking 的 OpenAI 兼容提供商，平台会把上游返回的 `reasoning_content` 存入 `ai_messages.reasoning_content`，用于工具调用后的上下文续传；该内容不作为普通 assistant 文本通过 SSE 展示。

## 路由观测日志（Routing Logs）

- 平台引入 `ai_routing_logs` 表，记录每条用户消息触发的路由观测数据：planner 输出、router 输出、per-hop 的 offered/called/missing 工具与 token、以及最终 outcome。
- 用途仅限 **Shadow 对比与离线评测**：用于在不影响线上对话的前提下回放/对比不同路由策略的表现，**不参与** SSE 事件 schema、**不改变**对话流的任何对外行为。
- 该表按 `session_id` 关联 `ai_sessions`，并随会话级联删除（`on delete cascade`），不引入新的隔离维度，会话项目/用户隔离边界保持不变。
- `message_seq` 关联触发本次路由的用户消息 `seq`（可空）；`planner_output` / `router_output` / `per_hop` 以 `jsonb` 承载，`outcome` 记录最终结果。
- `outcome` 取值固定为 `success` / `pending_confirm` / `hop_exhausted` / `error` 四种，与主循环的终止分支一一对应。
- `per_hop` 是每跳的数组，每项含 `hop` / `offered`（本跳实际发给 LLM 的工具名）/ `called`（模型本跳实际调用的工具名）/ `missing`（模型试图调用却不在 offered 内的工具，正常应为空）/ `inputTokens`（本跳 prompt token）/ `narrowed`（本跳是否启用了动态收窄）。

## 按需工具表面（Planner / Router / Meta 工具 / 动态挂载）

为把每轮 LLM 请求的工具从全量 60+ 降到按需 8-12 个，平台在浮窗对话主循环前引入「Planner → Router」预路由，并以两个常驻 meta 工具支持按需扩展。该能力对 **SSE 事件 schema、confirm 挂起流、会话/项目隔离均无任何改变**，只影响「发给 LLM 的工具定义子集」。

- **Catalog（`internal/aitools/catalog.go`）**：对全部领域工具的只读索引。`Index()` 输出紧凑列表（name/domain/summary/mutating/requiresConfirm，无 Schema）；`Lookup(names)` 输出完整含 JSON Schema 的 wire 定义并保序、未知名安全跳过；`ByDomain(domain)` 按域取工具名。工具元数据 `Domain` / `Summary` / `Prerequisites` / `RelatedTools` / `AntiPatterns` **不进** `ToDefinition()`，不会泄漏给 LLM。
- **Meta 工具（`internal/aitools/meta_tools.go`，常驻在线、只读）**：
  - `find_tools(query, domain?, limit?)`：对 Catalog 做 **关键词 + 内存向量 hybrid 检索**（RRF 融合）。工具向量在构建期预计算并随二进制嵌入 `internal/aitools/data/tool_embeddings.json`（`make gen-tool-embeddings`）；查询向量在运行时经 OpenAI 兼容 `/embeddings` 生成（默认复用对话 Provider，或通过 `AI_TOOL_EMBED_*` 独立配置）。无 embedding 或模型不一致时自动降级为纯关键词。返回 `[{name, domain, summary}]`，不含 Schema。
  - `describe_tools(names[])`：返回 `Catalog.Lookup` 的完整描述 + JSON Schema；并通过 ctx 上的 `DescribeCollector` 把已 describe 的工具名回传给主循环，下一跳自动纳入挂载集。
  - 两者 schema 均 `additionalProperties:false`、不暴露 `projectId`，遵循现有内置工具约定。
- **Planner（`internal/aiprovider/planner.go`）**：用与主对话相同的 provider client、小 `MaxTokens`、关闭 thinking、`JSONOnly` 调一次 LLM，产出 `{intent, domains[], workflow[], needsWrite, ambiguities[]}`。输入只带紧凑域目录（无 JSON Schema）。**强容错**：解析失败/超时/空返回一律降级为安全输出（空 domains、needsWrite=false），绝不让主对话失败。
- **Router（`internal/aiprovider/router.go`，纯函数无 IO）**：产出 `{activeTools[], confidence, fallbackDomains[], reason}`。规则：CoreDiscovery（`list_services`/`get_endpoint`/`get_case`/`get_scenario`）恒并入；按 `plan.domains` 经 `Catalog.ByDomain` 取域内工具；结合 pageContext 路由/对象 ID 增补对应域；置信度启发式（有 domains 且无歧义→高分 0.9，有歧义→0.6，空 domains→0.3）；低于阈值 0.7 时把全部域纳入 `fallbackDomains` 兜底。`find_tools`/`describe_tools` 不在 Router 输出里——由主循环恒定挂载。
- **动态挂载落点（`runStreamLoop`）**：每个 hop 开头重算 `activeNames = union(CoreDiscovery, Router.ActiveTools, DescribedTools, {find_tools, describe_tools})`，据此仅收窄「发给 LLM 的 `opts.Tools`」；`cfg.Tools`（执行用全量 map）**始终保持全量**，确保任何被调用工具都能执行。`describe_tools` 执行后其目标工具在下一 hop 自动并入。`ContinueAfterConfirm` 续流时**不重跑 Planner**，改为按 pageContext 经 Router 恢复 active 集，保证 confirm 后工具仍可用。

### 灰度开关（`toolRoutingMode`，可在 AI 助理配置页调整）

控制是否真正收窄发给 LLM 的工具；**无论开关取值，Planner/Router 都会被计算并写 `ai_routing_logs`**（影子记录），便于离线对比。默认 `shadow`，零风险上线。

- `shadow`（默认）：始终发全量工具，但并行计算 Planner/Router 并写日志。
- `dynamic_fallback`（Phase 1）：confidence ≥ 0.7 时发收窄包，否则回退全量。
- `dynamic`（Phase 2）：默认发收窄包；当 Router 未选出任何领域工具时回退全量兜底。
- `meta_only`（Phase 3）：仅发 CoreDiscovery + meta + 已 describe 的工具，其余完全靠 `find_tools`/`describe_tools` 发现。

### 路由观测指标口径

均基于 `ai_routing_logs` 的 `per_hop` / `outcome` 离线计算（后端字段已齐备，前端可视化为后续跟进项）：

- **Tool Recall@Hop1**：首跳 `offered` 是否覆盖模型在该对话中实际用到的关键工具（对照 Golden Set 期望集）。
- **Mis-route Rate**：`per_hop[*].missing` 非空（模型想调用却未挂载）的占比；理想为 0。
- **describe 扩展率**：触发了 `describe_tools` 扩展（后续 hop 出现新挂载工具）的会话占比，反映首轮选包不足的频率。
- **Confirm Reject Rate**：`outcome=pending_confirm` 后用户 reject 的占比（结合 `ai_messages.status=rejected`）。
- **Hop to Success**：`outcome=success` 时 `per_hop` 的长度分布（完成任务所需跳数）。
- **Token per Task**：`sum(per_hop[*].inputTokens)`（结合 `ai_messages.usage_details`）按任务聚合，用于对比收窄前后的 token 下降幅度。

### 离线评测（Golden Set）

- `testdata/ai_assistant_eval/*.yaml` 定义用例（`user` / `pageContext` / `planner` mock 输出 / `expectToolsFirstHop` / `forbidTools` / `expectDomains` / `expectFinish`）。
- `internal/aiprovider/eval` 为评测 harness，**默认不依赖真实 LLM**：用例直接提供 planner 输出，跑真实 Router 规则 + 首轮工具表面计算，比对召回/禁用/域覆盖；真实 LLM 跑 Planner 的模式可后续以 build tag / env 接入。
- `make ai-eval`（或 `go run ./cmd/ai-eval`）运行评测并在有回归时以非零退出，作为 Router/Planner prompt 变更的回归门槛。

### P3 后续工作

- **prompt / 规则迭代**：基于 eval failure case 持续打磨 Planner/Router prompt 与 `AntiPatterns` 库（字段已可被 Router/prompt 使用）。
- **历史压缩**：与本架构正交的进一步降 token 手段（旧 tool result 摘要化）。
- **前端指标仪表盘**：上述指标的可视化展示属较大且非阻塞项，后续在现有 Debug/usage 通道之上跟进。

#### find_tools 向量检索（已实现，无独立向量库）

- 工具向量：启动时从嵌入的 `tool_embeddings.json` 加载到内存；发版前运行 `make gen-tool-embeddings`（需 `AI_TOOL_EMBED_BASE_URL` / `AI_TOOL_EMBED_API_KEY`，可选 `AI_TOOL_EMBED_MODEL`，默认 `text-embedding-3-small`）。
- 查询向量：助理对话工具循环经 **AI 助理配置** 中的工具向量字段，或 OpenAI 兼容 chat Provider 的 `/embeddings` 端点；Anthropic 对话 Provider 需单独配置向量 Base URL。
- 检索：`searchCatalogHybrid` = 关键词打分 + 余弦相似度，RRF 融合；wire 契约不变。

## 与其他能力的关系

- Mock Value Sets 参与 `generate_params` 的上下文注入,详见 `docs/design/mock-template-and-test-data.md`。
- 运行控制台中的 AI 生成参数入口与请求编辑能力关系见 `docs/design/api-management-and-runner.md`。
