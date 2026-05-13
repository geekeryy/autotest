# 需求索引

本文档只记录已确认、长期有效的业务需求索引。具体产品设计、交互细节和业务规则放在 `docs/design/*.md`；工程规范、agent 行为和协作约束放在 `.cursor/rules/*.mdc`。

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

### Mock、模板变量与测试数据

- [Mock Server](design/mock-template-and-test-data.md)：涉及 Mock 服务启动/停止、端口、规则匹配、动态响应模板、CORS、规则测试弹窗或 Mock 权限的请求，先判断变更是管理 API、运行时匹配还是浏览器测试体验；运行中请求应读取最新数据库规则。
- [Mock Value Sets](design/mock-template-and-test-data.md)：涉及命名值集合、权重、索引取值、顺序遍历、项目隔离或内置业务 helper 的请求，先判断取值是随机、固定索引还是 run/request 维度顺序游标；`set` 不支持过滤语法。
- [运行时模拟标签](design/mock-template-and-test-data.md)：涉及 `{{$mock.*}}` helper、参数解析、实时生成、类型保持或未知 helper 错误的请求，先判断标签是否应在运行时渲染而不是写入快照；同一请求中多次出现应独立生成。
- [模板变量规范](design/mock-template-and-test-data.md)：涉及 `$mock`、`$steps`、`$ds`、`$sql`、`$req`、普通变量、deprecated 旧语法或渲染顺序的请求，优先检查是否应统一走 `internal/templating`，避免在调用方新增正则。
- [SQL 参数源](design/mock-template-and-test-data.md)：涉及业务数据源、SQL 参数源、`{{$sql.*}}` 内联引用、过滤表达式、预览或执行快照的请求，先判断数据源是否按项目维护、SQL 是否只读、找不到来源/行/列时是否返回明确错误；当前 SQL 参数源 HTTP 层主要依赖登录态和 `projectId` 参数，未像 Mock/TestData 一样统一挂项目角色中间件。
- [测试数据表](design/mock-template-and-test-data.md)：涉及项目级测试数据表、列生成方式、`{{$ds.*}}` 引用、AI 生成测试数据或权限的请求，先判断数据应来自表行快照还是运行时解析；权限与 SQL 参数源、Mock Server 保持 viewer/developer 分层。

### AI 能力

- [AI 提供商](design/ai-capabilities.md)：涉及 provider 类型、Base URL、API Key 脱敏、默认模型、extraConfig、启用状态或默认提供商的请求，先判断配置是项目级还是全局；API 不应返回明文 Key。
- [项目级 Prompt](design/ai-capabilities.md)：涉及 action 级 System Prompt、默认模型、provider 绑定或回退逻辑的请求，先判断 providerId 为空是否应跟随项目默认提供商；绑定 provider 必须属于同项目且未删除。
- [AI 生成请求参数](design/ai-capabilities.md)：涉及 `generate_params`、上下文构造、pathVarNames、currentRequest 保留、Mock Value Sets 注入或模型输出格式的请求，先区分非 LLM 的 `GET /cases/{id}/generate-params` 采样接口与 LLM `/ai/chat` action，并注意下方覆盖范围冲突仍待决策。
- [AI 生成断言与测试数据](design/ai-capabilities.md)：涉及 `generate_assertion` 或 `generate_case_data` 的请求，先判断是否需要非空测试意图、项目 Prompt/provider 配置和明确中文错误；生成结果应由用户预览或追加，不应静默覆盖已有内容。
- [AI 智能分析](design/ai-capabilities.md)：涉及失败原因分析或 spec 变更影响分析的请求，先判断输入应来自本次运行快照、断言失败明细或 spec diff 摘要；分析结果当前不写库。

### 管理后台与访问控制

- [登录与后台基础](design/admin-and-access.md)：涉及登录、路由守卫、默认管理员账号密码对齐或后台技术栈的请求，先判断是认证流程、权限导航还是启动初始化；生产环境凭据应通过环境变量覆盖。
- [全局项目上下文](design/admin-and-access.md)：涉及顶部项目选择、页面间项目复用、上次项目/环境恢复或未选择项目提示的请求，先判断是否应依赖全局项目，避免在页面内重复选择项目；项目管理页实际承载服务环境、业务数据源、AI 提供商和 Prompt 管理。
- [服务与环境管理](design/admin-and-access.md)：涉及服务/环境树、环境变量 JSON、认证 JSON、编辑弹窗、提示图标或失焦格式化的请求，先判断变更是否影响运行控制台的环境编辑复用。
- [菜单、布局与视觉品牌](design/admin-and-access.md)：涉及侧边栏菜单、收起、滚动区域、字体、配色、头像、退出登录或 logo 的请求，先判断是全局布局约束还是单页样式调整；左侧导航不应单独滚动。
- [脚本库](design/admin-and-access.md)：涉及断言编辑器、场景脚本步骤、内置模板、项目自定义模板或 AI 生成脚本入口的请求，先判断模板作用域是全平台共享还是当前项目。
- [用户权限与 API Key](design/admin-and-access.md)：涉及 RBAC 菜单、项目权限、API Key 列表/创建/重置/禁用/过期/审计或 CI/CD 调用的请求，先区分前端路由权限、全局后端权限和项目角色中间件；API Key 当前仅允许 `specs:import`，管理操作只作用于当前登录用户的 Key。

## 待用户决策

- `generate_params` / 「一键生成参数」的覆盖范围存在历史冲突：一种说法要求仅覆盖 path、query、body，且不返回或覆盖 headers/security；另一种说法要求覆盖 query、headers、path、JSON Body。相关实现变更必须先由用户明确决策后再继续。
