# 需求与文档索引

本目录存放 autotest 的业务需求与设计说明。快速上手见根目录 [README.md](../README.md)。

本文档记录已确认、长期有效的业务需求索引，并作为 `docs/` 目录的导航入口。具体产品设计、交互细节和业务规则放在 `docs/design/*.md`；系统架构、配置与部署见 [architecture.md](design/architecture.md)；工程规范、agent 行为和协作约束放在 `.cursor/rules/*.mdc`。

## 阅读顺序

| 文档 | 用途 |
|------|------|
| 本文档 | **需求索引**：已确认的长期业务需求，按主题指向设计文档 |
| [design/api-automation-platform.md](design/api-automation-platform.md) | **设计总览**：各专题设计文档的导航 |
| [design/architecture.md](design/architecture.md) | **架构与运维**：系统架构、环境变量、部署（Firebase 分离 / Docker，All-in-One 可选） |

## 专题设计（`design/`）

| 文档 | 主题 |
|------|------|
| [platform-core.md](design/platform-core.md) | 术语、项目/服务/环境、Runner、断言、报告 |
| [api-management-and-runner.md](design/api-management-and-runner.md) | OpenAPI 导入、API 管理、运行控制台 |
| [scenario-orchestration.md](design/scenario-orchestration.md) | 场景编排、控制流、运行报告 |
| [mock-template-and-test-data.md](design/mock-template-and-test-data.md) | Mock Server、模板变量、测试数据 |
| [ai-capabilities.md](design/ai-capabilities.md) | AI 提供商、生成、分析、助理浮窗 |
| [admin-and-access.md](design/admin-and-access.md) | 登录、RBAC、菜单、API Key、通知 |

## 维护约定

- **业务需求**变更：更新本文档及对应 `design/*.md`。
- **架构、配置、部署**变更：更新 [architecture.md](design/architecture.md) 与根目录 [README.md](../README.md) 中的 Quick Start / Configuration 章节。
- **工程规范与 agent 行为**：写在 `.cursor/rules/*.mdc`，不写入需求文档。

## 收集原则

- 新增需求、需求变更或范围澄清，只有在影响产品行为、用户流程、权限、数据持久化、兼容性或对外/对内 API 契约时，才进入需求归档。
- Bug 修复、代码整理、测试补充、实现细节确认和临时排查不自动进入需求归档，应按具体影响判断是否沉淀。
- 已删除或已修改的旧需求必须同步移除或改写，避免继续干扰后续实现。
- 发现需求冲突或语义不清时，必须先交由用户决策，再继续相关实现。

## 当前业务需求

### 平台基础与服务模型

- [术语与核心对象](design/platform-core.md)：凡涉及「接口」「测试用例」「测试集」「请求模板」命名、数据归属或 `/cases` 历史路径兼容的请求，先按本文档判断概念边界；若要恢复测试集或改名影响界面/接口，需先确认是否改变既有产品模型。
- [项目、服务与环境](design/platform-core.md)：涉及项目软删除、服务与环境层级、baseURL 拼接、环境变量或认证 JSON 归属的请求，先判断是否影响运行时寻址和数据隔离；只改 UI 展示时仍需避免破坏项目/服务/环境边界。
- [Runner、断言与报告](design/platform-core.md)：涉及单接口运行、场景运行、断言执行、运行快照或报告输出的请求，先判断是否改变 Runner 输入输出契约；若会影响历史运行记录或报告字段，应同步检查持久化与兼容性。
- [示例服务与迁移](design/platform-core.md)：涉及 `tests` 示例 API、JWT 示例鉴权、admin 示例接口或 migrations 的请求，先判断是示例能力、开发辅助还是正式平台行为；数据库结构变化必须通过 migrations 落地。

### API 导入与运行控制台

- [OpenAPI/Swagger 导入](design/api-management-and-runner.md)：涉及 spec 上传、导入响应、幂等刷新、端点写入、模板自动生成或 schema 约束保留的请求，优先检查导入是否会产生重复数据或返回过大字段；当前导入响应为统计摘要，但 spec 列表仍可能返回 `normalizedSnapshot`。
- [API 管理](design/api-management-and-runner.md)：涉及导入历史、接口请求模板列表、手工模板、新增/编辑入口或页面权限的请求，先判断目标是 `api_endpoints` 还是可运行模板 `test_cases`；`cases:read`、`cases:write`、`specs:import` 当前主要用于前端路由/按钮权限，后端 cases/spec 扁平路由未全部强制这些权限。
- [运行控制台请求编辑](design/api-management-and-runner.md)：涉及接口 Tab、Path/Query/Header/Body/断言分区、默认参数、保存用例或请求快照的请求，先判断是否改变用户可编辑请求模型；当前无独立「变量」Tab，路径模板和路径参数值必须保持分离。
- [运行结果展示](design/api-management-and-runner.md)：涉及响应 Body、响应头、实际请求 curl、状态码、错误标签、耗时颜色或运行后默认 Tab 的请求，优先判断是展示调整还是运行快照结构变化；底层网络错误才额外展示错误原因标签。
- [环境认证继承](design/api-management-and-runner.md)：涉及 OpenAPI security、auth profile、defaultProfile、legacy 认证、Headers/Query 继承行或用户覆盖的请求，先判断继承值是否应写入请求覆盖；前端运行控制台/场景编辑与后端 Runner 各有实现，需保持语义一致。
- [运行工作台状态持久化](design/api-management-and-runner.md)：涉及打开接口 Tab、激活状态、请求草稿、本地恢复、关闭 Tab 或环境编辑入口的请求，先判断状态是否只应保存在浏览器本地；当前 Tab/激活状态按项目持久化，草稿按 `caseId`，关闭 Tab 表示丢弃该接口草稿。

### 场景编排

- [场景步骤模型](design/scenario-orchestration.md)：涉及 API、数据库、脚本、For 循环、条件分支等步骤新增/配置/执行的请求，先判断是否改变步骤类型契约、`config` 结构或运行器执行顺序；API 步骤引用的是接口请求模板。
- [变量提取与脚本步骤](design/scenario-orchestration.md)：涉及 `{{$steps[N].*}}`、响应/请求字段引用、变量提取、goja 脚本、Postman 风格 `pm.*`、console 输出或变量写回的请求，先判断渲染顺序和变量作用域是否受影响。
- [步骤编辑与列表交互](design/scenario-orchestration.md)：涉及步骤级 Tab、未保存草稿、拖拽排序、启停、删除按钮、列表宽度或本地恢复的请求，先区分内存草稿、本地持久化和服务端数据；刷新后表单应重新装载服务端最新数据。
- [步骤删除与历史槽位](design/scenario-orchestration.md)：涉及删除、恢复、排序、保存丢失或刷新后查不到步骤的请求，优先检查软删除与 `(scenario_id, step_order)` 槽位释放；同槽位新保存必须创建全新记录。
- [控制流](design/scenario-orchestration.md)：涉及 For 循环、条件分支、子步骤展示、`step_seq` 引用、嵌套、跳过分支或失败传播的请求，先判断是否会破坏控制步骤到子步骤的稳定引用和最大嵌套限制。
- [步骤克隆与运行结果](design/scenario-orchestration.md)：涉及跨场景克隆、深拷贝、控制流子树复制、运行结果面板、详情/断言/步骤输出展示的请求，先判断克隆是否需要重映射 `step_seq`，以及结果展示是否依赖运行快照。
- [测试报告与运行历史](design/scenario-orchestration.md)：涉及运行历史列表（`/reports/runs`）、报告详情页（`/runs/{runId}`）、HTML/JSON 导出、统计口径或编辑器内本次运行面板的请求，先区分 `test_runs` 持久化与内存中的当前运行结果；历史报告仅展示已落库步骤（不展示 skipped）；权限与场景读取一致（项目 viewer + `cases:read` 路由）。

### Mock、模板变量与测试数据

- [Mock Server](design/mock-template-and-test-data.md)：涉及 Mock 服务启动/停止、端口、规则匹配、动态响应模板、HTTP 重定向（SSO 模拟）、CORS、访问日志、规则测试弹窗或 Mock 权限的请求，先判断变更是管理 API、运行时匹配还是浏览器测试体验；运行中请求应读取最新数据库规则。
- [Mock Value Sets](design/mock-template-and-test-data.md)：涉及命名值集合、权重、索引取值、顺序遍历、项目隔离或内置业务 helper 的请求，先判断取值是随机、固定索引还是 run/request 维度顺序游标；`set` 不支持过滤语法。
- [运行时模拟标签](design/mock-template-and-test-data.md)：涉及 `{{$mock.*}}` helper、参数解析、实时生成、类型保持或未知 helper 错误的请求，先判断标签是否应在运行时渲染而不是写入快照；同一请求中多次出现应独立生成。
- [模板变量规范](design/mock-template-and-test-data.md)：涉及 `$mock`、`$steps`、`$ds`、`$sql`、`$req`、普通变量、deprecated 旧语法或渲染顺序的请求，优先检查是否应统一走 `internal/templating`，避免在调用方新增正则。
- [SQL 参数源](design/mock-template-and-test-data.md)：涉及业务数据源、SQL 参数源、`{{$sql.*}}` 内联引用、过滤表达式、预览或执行快照的请求，先判断**业务数据源为全局平台资源**（HTTP API 使用 `projects:read` / `projects:write`）、SQL 参数源仍按项目+服务维护、SQL 是否只读、找不到来源/行/列时是否返回明确错误；SQL 参数源 HTTP 层主要仍依赖登录态和 `projectId` 参数，未像 Mock/TestData 一样统一挂项目角色中间件。
- [测试数据表](design/mock-template-and-test-data.md)：涉及项目级测试数据表、列生成方式、`{{$ds.*}}` 引用、AI 生成测试数据或权限的请求，先判断数据应来自表行快照还是运行时解析；权限与 SQL 参数源、Mock Server 保持 viewer/developer 分层。

### AI 能力

- [AI 提供商](design/ai-capabilities.md)：涉及 provider 类型、Base URL、API Key 脱敏、文本/多模态默认模型（`modalityModels`）、模型能力标签、extraConfig、thinking/reasoning、启用状态或默认提供商的请求，先判断配置为**全局平台资源**（`/ai-providers`，不按项目隔离）；API 不应返回明文 Key。模型列表应通过上游 list-models 动态获取；`capabilities` 仅来自上游元数据（无则空），多模态路由依赖用户配置的 `modalityModels`，不按模型 id 推断能力。`/ai-provider-types` 仅保留类型元数据与离线 fallback。
- [平台 Prompt](design/ai-capabilities.md)：涉及 action 级 System Prompt、默认模型、provider 绑定或回退逻辑的请求，先判断 providerId 为空是否应跟随**平台**默认提供商；绑定 provider 须为未删除的全局提供商；每个 action 全局至多一条有效配置。
- [AI 生成请求参数](design/ai-capabilities.md)：涉及 `generate_params`、上下文构造、pathVarNames、currentRequest 保留、Mock Value Sets 注入或模型输出格式的请求，先区分非 LLM 的 `GET /cases/{id}/generate-params` 采样接口与 LLM `/ai/chat` action，并注意下方覆盖范围冲突仍待决策。
- [AI 生成断言与测试数据](design/ai-capabilities.md)：涉及 `generate_assertion` 或 `generate_case_data` 的请求，先判断是否需要非空测试意图、平台 Prompt/provider 配置和明确中文错误；生成结果应由用户预览或追加，不应静默覆盖已有内容。
- [AI 智能分析](design/ai-capabilities.md)：涉及失败原因分析或 spec 变更影响分析的请求，先判断输入应来自本次运行快照、断言失败明细或 spec diff 摘要；分析结果当前不写库，且分析输出统一为中文 Markdown 由前端 `MarkdownView` 渲染。
- [AI Tool Calling 框架](design/ai-capabilities.md)：涉及让 AI 通过内置工具拉系统状态、扩展工具集、改变工具权限边界或 mutating 工具确认机制的请求，先判断目标是否落在 `internal/aitools` Registry；分析类 action 仅允许只读工具，mutating 工具（如 `update_case_assertions`、场景编排相关写工具）只能通过浮窗对话流程并经过用户 confirm 才会执行。
- [全局 AI 助理浮窗](design/ai-capabilities.md)：涉及登录后管理后台 AI 助理浮窗、provider/model 选择、深度思考/联网搜索开关、Token 用量持久化与汇总（仪表盘/会话详情）、Debug 开关下浮窗每轮 token/缓存详情展示、会话列表查看会话详情（对话统计与 Token 汇总）、Xiaomi 图片上传与多模态消息、SSE 对话流、会话/消息持久化、空会话时「新对话」静默复用（不重复创建）、跨用户隔离或 mutating 工具人在回路确认的请求，先判断需求是否会改变 `ai_sessions`/`ai_messages` 表结构、SSE 事件 schema 或会话隔离边界；浮窗会话按 `(project_id, user_id)` 严格隔离，分析类 action 不暴露写工具。
- [AI 助理工作区分屏](design/ai-capabilities.md)：涉及工作区最多 6 屏、横向并排、每屏独立 provider/model、工作区主区域 +/各屏 X 增删分屏、同会话不可多屏重复打开或 pane 级设置持久化的请求，先确认是否仅影响前端 `aiAssistant` store 与 `/ai-assistant` 页，不改变 SSE 或表结构。
- [AI 场景生成与编排](design/ai-capabilities.md)：涉及让 AI 帮用户生成测试场景、追加/修改/删除/重排步骤、生成接口请求模板或一键运行的请求，先判断是工具集扩展还是运行触发；AI 通过 `list_* / get_*` 探查后调用 `create_scenario_with_steps`（API/Script/For/Condition 四种步骤）一次性创建完整场景，控制流子步骤引用使用 `stepOrder`，由平台转换为内部 `step_seq`；运行触发始终由用户在场景页手动点击，AI 不暴露任何"运行"工具。服务和环境的 create/update 不通过 AI 工具暴露——平台基础结构由用户手工维护，AI 只读相关元信息。
- [AI 会话权限与项目隔离](design/ai-capabilities.md)：涉及 AI 工具调用是否能跨项目、是否能越用户访问数据的请求，先确认每个 SSE 会话由 `(projectId, userId)` 绑定；任何接受 `projectId` 的工具均通过 `aitools.ResolveProjectID` 校验，按 `caseId/scenarioId/stepId` 操作的工具在查到对象后反向校验其 `projectId` 必须等于会话项目，违反时直接拒绝执行。
- [AI 助理页面上下文](design/ai-capabilities.md)：涉及 AI 浮窗感知用户当前页面、把 `scenarioId/caseId/serviceId` 等当作默认对象的请求，先确认前端按路由切换调 `bindPage` 写入 `assistantState.pageContext`，每次 `POST /ai/chat/stream` 与 `/tool-calls/{id}/confirm` 都附带这份快照；后端把它作为额外 system 消息注入 LLM 上下文，但工具仍以参数为权威，页面上下文不参与权限判定。

### 管理后台与访问控制

- [登录与后台基础](design/admin-and-access.md)：涉及登录、路由守卫、默认管理员账号密码对齐、首次改密或后台技术栈的请求，先判断是认证流程、权限导航还是启动初始化；默认管理员为 `admin`/`admin`（代码内 bootstrap），首次本地密码登录须改密后重新登录；`APP_ENV=production` 启动时校验 JWT 与数据库密码强度。支持本地用户名密码登录（bootstrap，不可关闭）与可配置 OAuth（GitHub/GitLab/Google，**用户管理 → 登录方式** Tab CRUD，`users:manage`）并存；OAuth 统一回调页 `/login/oauth/callback`（旧 `/login/github/callback` 重定向）；回跳前端地址由登录请求 `frontendUrl`（signed state）与**各登录方式**配置的 `trustedFrontendOrigins` 校验（**仅 OAuth 回跳，不用于 CORS**）。前后端分离部署时 CORS 白名单由环境变量 `CORS_ALLOWED_ORIGINS` 配置（逗号分隔）。`GET /auth/login-providers` 动态展示已启用且 `callbackUrl` 域名与当前 API 请求域名一致的 IdP。
- [全局项目上下文](design/admin-and-access.md)：涉及顶部项目选择、页面间项目复用、上次项目/环境恢复或未选择项目提示的请求，先判断是否应依赖全局项目，避免在页面内重复选择项目；项目管理页承载服务与环境管理；业务数据源、AI 提供商、Prompt 管理已归入「平台资源」菜单。
- [服务与环境管理](design/admin-and-access.md)：涉及服务/环境树、环境变量 JSON、认证 JSON、编辑弹窗、提示图标或失焦格式化的请求，先判断变更是否影响运行控制台的环境编辑复用。
- [菜单、布局与视觉品牌](design/admin-and-access.md)：涉及侧边栏菜单、收起、滚动区域、字体、配色、头像、退出登录或 logo 的请求，先判断是全局布局约束还是单页样式调整；左侧导航不应单独滚动。
- [脚本库](design/admin-and-access.md)：涉及断言编辑器、场景脚本步骤、内置模板、项目自定义模板或 AI 生成脚本入口的请求，先判断模板作用域是全平台共享还是当前项目。
- [用户权限与 API Key](design/admin-and-access.md)：涉及 RBAC 菜单、项目权限、API Key 列表/创建/重置/禁用/过期/审计或 CI/CD 调用的请求，先区分前端路由权限、全局后端权限和项目角色中间件；API Key 当前仅允许 `specs:import`，管理操作只作用于当前登录用户的 Key。**用户管理**页（`/users`）以 Tab 聚合用户、角色、权限、登录方式；历史路径 `/roles`、`/permissions`、`/auth-providers` 重定向到对应 Tab。用户管理支持上传头像：浏览器端先压成 JPEG，服务端再缩放编码后存入 `users.avatar_jpeg`，列表与 `/auth/me` 等接口在 JSON 中返回 `avatarUrl`（`data:image/jpeg;base64,...`），便于 Bearer 鉴权下直接用于 `<img>` / `el-avatar`。OAuth 首次登录创建的用户默认 `active=false`、无角色，管理员在用户管理中启用并分配角色后生效。
- [审计日志](design/admin-and-access.md)：涉及登录成功/失败记录、审计日志列表查询或菜单权限的请求，先确认当前仅持久化 `auth.login` 事件（本地密码与 OAuth）；列表接口 `GET /audit-logs` 需 `audit:read` 权限，默认授予 admin 角色；前端「系统管理 → 审计日志」（`/audit-logs`）展示时间、用户、方式、结果、失败原因、IP 与 User-Agent。
- [站内通知](design/admin-and-access.md)：涉及管理后台顶栏通知铃铛、未读角标、通知列表、已读状态、一键清空或 API Key 导入触发的通知写入的请求，先判断是否仅按登录用户隔离（不新增 RBAC 权限）；当前 API Key 成功导入 Swagger 时写入 `spec_import` 通知，GitHub 首次待审核用户注册时向拥有 `users:manage` 权限的用户写入 `user_pending` 通知；JWT 页面内手动导入不触发 spec 通知；正文与统计展示**本次实际变动**的接口数（`createdEndpoints + updatedEndpoints`），不含文档内未改动的端点；前端登录后通过 `GET /notifications/stream`（JWT、RejectAPIKey）SSE 实时推送，断线自动重连，页面重新可见时仅做一次 REST 同步；`POST /notifications/clear-all` 硬删除当前用户全部通知并通过 SSE 推送空 `snapshot`。

## 待用户决策

- `generate_params` / 「一键生成参数」的覆盖范围存在历史冲突：一种说法要求仅覆盖 path、query、body，且不返回或覆盖 headers/security；另一种说法要求覆盖 query、headers、path、JSON Body。相关实现变更必须先由用户明确决策后再继续。
