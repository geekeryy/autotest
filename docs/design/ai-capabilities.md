# AI 能力设计

本文档记录 AI 提供商、项目级 Prompt、AI 生成与 AI 分析能力的业务设计。

## AI 提供商

- 平台支持项目级 AI 提供商配置。
- 提供商类型包括 deepseek、xiaomi、openai、anthropic、kimi、ollama。
- 每个项目可维护多份 AI 提供商配置。
- 配置包含名称、Base URL、API Key 脱敏、默认模型、extraConfig、启用状态与是否默认。
- 同一项目最多一个默认提供商。
- DeepSeek 与 Xiaomi 走 OpenAI 兼容协议时默认启用 thinking/reasoning：DeepSeek 请求携带 `thinking.type=enabled` 与 `reasoning_effort`，Xiaomi 请求携带 `reasoning.effort`。默认 effort 为 `high`；可通过 `extraConfig.thinking=false` 或 `extraConfig.thinking.enabled=false` 关闭，也可通过 `extraConfig.reasoning_effort` / `extraConfig.reasoningEffort` 或 `extraConfig.thinking.effort` 覆盖强度。
- 模型下拉不再依赖 `/ai-provider-types` 的静态列表：已保存配置走 `GET /projects/{projectId}/ai-providers/{providerId}/models`；创建/编辑表单在填写 Base URL 与 API Key 后走 `POST .../ai-providers/models/discover`（编辑时 apiKey 留空则复用库内密钥）。OpenAI 兼容类型调用上游 `GET {baseUrl}/models`，Anthropic 调用 `GET {baseUrl}/models`，Ollama 在 `/v1/models` 不可用时回退 `GET /api/tags`；上游失败时返回内置 fallback 列表与 `warning`。

## 项目级 Prompt

- 项目级 Prompt（`project_ai_prompts`）可按动作维护 System Prompt / 默认模型。
- 项目级 Prompt 可选绑定 providerId。
- providerId 留空时跟随项目默认 AI 提供商。

## 统一 AI 入口

- `/projects/{projectID}/ai/chat` 统一 AI 入口支持 `generate_params`、`generate_assertion`、`generate_case_data`、`analyze_failure`、`analyze_spec_changes` 等 action。
- 运行控制台的「一键生成参数」还存在非 LLM 采样接口 `GET /cases/{caseID}/generate-params`；它与 LLM `generate_params` 是两条路径，文档和实现变更时需要区分。
- `generate_assertion` 必须在弹窗中填写非空测试意图，后端拒绝空意图。
- `generate_case_data` 使用项目级 Prompt 配置。
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
- 工具元数据携带 `Mutating bool`,只读工具可直接执行;Mutating（写）工具必须经过下文「写工具确认」中的人在回路流程,任何分析类 action 都只允许暴露只读工具。
- 工具循环最多 `MaxToolHops` 跳(默认 6);达到上限仍未给出结论时返回中文提示并停止,不允许无限工具调用。
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

按域拆分为四个源文件：

- `builtin.go`：`Deps` / `ReadOnly` / `Mutating` / `All` 入口与 `rawSchema` helper。
- `deps.go`：消费侧接口（`CaseService` / `ScenarioService` / `SpecRepository` / `ProjectService`），每个依赖 nil 时对应工具运行期返回中文错误，而不是 panic。
- `builtin_meta.go`：服务/接口/环境的 discovery（`list_services` / `list_endpoints` / `list_environments` / `get_endpoint`）。`list_endpoints` 返回 method/path/operationId/summary/tags 的精简摘要，调用方需要 schema 时再走 `get_endpoint`。
- `builtin_cases.go`：接口请求模板相关（`get_case` / `list_cases` / `create_case_from_endpoint` / `update_case_assertions`）。
- `builtin_scenarios.go`：场景与步骤编排（`get_scenario` / `list_scenarios` / `create_scenario_with_steps` / `add_scenario_step` / `update_scenario_step` / `delete_scenario_step` / `reorder_scenario_steps`）。

工具分类：

- 只读（discovery / 查询）：`list_services` / `list_endpoints` / `list_environments` / `get_endpoint` / `list_cases` / `get_case` / `list_scenarios` / `get_scenario`。
- 受控写（mutating，必经人在回路确认）：`create_case_from_endpoint` / `update_case_assertions` / `create_scenario_with_steps` / `add_scenario_step` / `update_scenario_step` / `delete_scenario_step` / `reorder_scenario_steps`。
- 平台 **不暴露** "运行" 类工具。AI 完成场景生成后由用户在前端手动点击运行按钮，确保所有真实测试动作都经过人的明确触发。
- 平台 **不暴露** 服务（service）/环境（environment）的创建、更新工具。项目基础结构（服务、环境、AI 提供商、Prompt、OpenAPI spec 导入、Mock Server 启停等）属于平台管理面，仅由用户手工维护；AI 工具只读相关元信息以辅助生成场景，但永远不会写入。

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

## 页面上下文（Page Context）

- 浮窗对话支持"AI 感知用户当前页面"。前端通过 `web/admin/src/stores/aiAssistant.js` 暴露 `bindPage(snapshot)` 与 `enrichPage(patch)` 两个 action：
  - `router/index.js` 的 `afterEach` 钩子在每次路由切换时调一次 `bindPage`，写入基础字段（`path`、`routeTitle`，以及路由参数中的 `scenarioId` / `caseId`）；
  - 各业务页面（场景列表、API 管理、运行控制台等）在数据加载完成后通过 `enrichPage` 追加更具语义的字段（如 `scenarioName`、`caseName`、`serviceId`），让 AI 拿到"用户当前看的对象的可读标签"。
- 每次 `POST /ai/chat/stream` 与 `/ai/tool-calls/{id}/confirm` 都会把 `assistantState.pageContext` 序列化进请求体的 `pageContext` 字段；后端 `AssistantStreamRequest.PageContext` 是不持久化的 `json.RawMessage`，每轮请求都使用最新快照。
- 后端 `renderPageContextSystem` 把 `pageContext` 渲染为一条"`## 用户当前页面上下文`"开头的 system 消息，插入到主 system prompt 之后，供 LLM 在用户使用代词或省略对象 ID 时把页面快照里的 `scenarioId/caseId/serviceId` 作为默认对象。
- **页面上下文只是提示，不是授权依据**：系统 prompt 明确告知 AI 该字段只用于消歧，工具参数仍以模型显式传入为权威，最终的项目归属和权限校验由 `aitools.ResolveProjectID` / `aitools.RequireProjectAccess` 把关。
- 浮窗 UI 在头部下方显示一行"当前上下文：…"提示，方便用户直观看到 AI 正在感知什么；`pageContext` 为空或只剩占位字段时该提示自动隐藏。

## 全局 AI 助理浮窗（对话型入口）

- 登录后管理后台提供一个全局 AI 助理浮窗，对话上下文按「当前项目 + 当前登录用户」隔离，跨用户/跨项目互不可见。
- 浮窗对话挂载在统一的 `assistant_chat` action 上，使用独立 system prompt（`assistantChatSystem`），引导模型以 Markdown 回应并主动调用工具补充上下文。
- 浮窗输入区提供本次对话的 AI 设置：用户可选择当前项目下已启用的 AI 提供商，并在该提供商的推荐模型列表中选择或手工输入模型名；请求会携带 `providerId` 与 `model`，未选择时回退项目默认提供商和 provider 默认模型。
- 当所选提供商为 DeepSeek 或 Xiaomi 时，浮窗参考 DeepSeek Chat 暴露「深度思考」开关。该开关按请求发送 `thinkingEnabled` 与 `reasoningEffort=high`，关闭时本轮不发送 thinking/reasoning 参数；其它 provider 不展示该开关。
- 当所选提供商为 Xiaomi 时，浮窗额外暴露「联网搜索」开关。开启后请求在 tools 中附带小米内置 `web_search` 工具（由平台执行并注入结果，assistant 循环不本地执行该工具）；关闭时不附带。联网搜索按次计费，与 Token 计费独立。
- 当所选提供商为 Xiaomi 时，输入区支持上传图片（JPEG/PNG/GIF/WebP，单张 ≤5MB，每条消息最多 4 张）。前端以 base64 data URL 随 `POST /ai/chat/stream` 的 `images` 字段提交；后端校验后写入 `ai_messages.attachments`，并以 OpenAI 多模态 `content` 数组（`text` + `image_url`）调用模型。行业常见做法是**按能力自动路由**：用户可在设置里默认选 `mimo-v2-pro` 等文本模型，一旦会话历史含图片，服务端在本轮请求中自动将 upstream `model` 改为 `mimo-v2.5`（若网关列表仅有 `mimo-v2-5` / `mimo-v2-omni` 则按候选匹配），**不要求用户手动改下拉框**。纯文本轮次仍用用户所选模型。历史会话加载时通过 `attachments` 字段回显缩略图。
- 深度思考开启后，前端在 assistant 占位消息中展示「正在深度思考」状态与耗时；后端仅通过 SSE `thinking` 事件同步 active/inactive 状态，不向前端暴露 `reasoning_content` 文本。
- 浮窗会话由 `ai_sessions` 与 `ai_messages` 两张表持久化（migration 012）：
  - `ai_sessions` 记录会话归属与标题；标题由后端在**第 1、2 条用户消息**时自动生成（第 1 条为主、第 2 条为辅精炼），第 3 条及之后不再更新；采用 few-shot「只输出标题」提示词，且不使用 reasoning 链作为标题；若模型仍输出分析口吻（如「首先，用户的指令是…」）则后处理剥离或回退为首问截断。
  - `ai_messages` 记录所有 LLM 对话轮（system/user/assistant/tool）与 `tool_calls` 元数据，并通过 `status` 字段区分 `final` / `pending_confirm` / `rejected`。
- 浮窗的 API 入口：
  - `GET/POST /projects/{projectID}/ai/sessions`、`GET/PATCH/DELETE /projects/{projectID}/ai/sessions/{sessionID}`：会话 CRUD。
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
  - `done`：本次流结束，`finish` 字段在 `stop` / `tool_calls` / `pending_confirm` / `hop_budget_exhausted` / `error` 之中。
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

## 与其他能力的关系

- Mock Value Sets 参与 `generate_params` 的上下文注入,详见 `docs/design/mock-template-and-test-data.md`。
- 运行控制台中的 AI 生成参数入口与请求编辑能力关系见 `docs/design/api-management-and-runner.md`。
