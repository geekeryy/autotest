# 需求归档

本文档是项目级需求归档的唯一事实来源。新增需求、需求变更或范围澄清必须先记录到这里，再继续实现工作。

## 接口自动化测试平台

### 术语（产品与界面）

- **接口**：OpenAPI/Swagger 导入写入 `api_endpoints`；「API管理」列表中的每一行对应库表 `test_cases` 中一条与该 HTTP 接口关联的**可运行请求模板**（含导入自动生成与手工新建）。Runner 与场景编排中的 API 步骤引用的是该模板。
- **测试用例**：在运行控制台通过「保存用例」固化的一组请求参数快照，需保存到数据库并关联当前接口模板；展示为路径栏右侧 Tab，用于多套参数切换与复跑，并可在场景编排中引用。
- **测试集**：已移除。原测试集功能（批量组织接口执行）由场景编排统一承载；历史数据库表 `test_suites` / `test_suite_items` 保留不删除，数据可迁移为场景。

对内 REST 路径仍为 `/cases`（历史兼容），文档与界面用「接口」指代上述请求模板。

MVP 阶段需求：

- 构建一个 API/接口自动化测试平台，后端主力语言为 Go，数据库使用 PostgreSQL。
- 支持导入 OpenAPI/Swagger，将 API 定义纳入平台管理。
- 重复导入同一服务下内容相同的 OpenAPI/Swagger 应保持幂等，不因内容哈希唯一约束报错，不产生重复接口定义或重复自动生成接口请求模板，并应继续刷新接口定义与模板。
- 支持基于导入的 API 定义自动生成接口请求模板（库表 `test_cases`）；自动生成模板名称应使用 API 名称（优先 OpenAPI/Swagger `summary`，缺省回退 `operationId`，再回退 `METHOD PATH`），不拼接生成规则名称。
- 支持在API管理中手动新建接口请求模板。
- 在同一套平台模型中统一管理手动模板与自动生成模板。
- 不单独提供测试集功能；批量执行多个接口通过场景编排实现。
- 支持像 Apifox 一样从运行控制台选择环境并独立运行接口请求模板，运行前可命名本次运行、手动修改请求参数、自动生成默认参数，并可将本次请求和响应作为参考；运行界面提供「保存用例」将当前请求参数固化为一组**测试用例**快照（数据库持久化，按接口模板 ID 隔离），以 Tab 显示在 path 输入框右侧，且可被场景编排引用；运行后展示本次请求、响应、断言和状态结果。
- 运行声明了 OpenAPI/Swagger `security` 的请求模板时，应基于所选环境的认证配置自动附带认证信息。
- 环境认证配置应支持在同一环境内维护多套 auth profile，不同 OpenAPI/Swagger `security` 可配置不同参数或 token；Runner 根据请求模板中的 `security` 与请求路径选择对应 profile，并兼容历史单一认证配置。
- 当请求体中未携带 `security`（例如手工模板、被旧版客户端或 saveSnapshot 流程清掉了 security 字段），且环境认证配置使用了多 profile 形式且配置了 `defaultProfile` 时，Runner 应使用 `defaultProfile` 作为兜底；legacy 单一认证配置在请求未声明 `security` 时仍保持不附带认证。
- 运行控制台的「一键生成参数」（`GET /cases/{id}/generate-params`）只覆盖 path、query、body 三类参数，不再返回或覆盖请求头与 `security`，避免运行时丢失原有 Content-Type 与认证信息；自动生成模板（generator/sampler）仍可在创建模板时附带 Content-Type 等默认请求头。
- 「一键生成参数」与自动生成模板的仿真值生成应在保持 `example` → `default` → `enum[0]` 优先级的同时，对 fallback 路径根据 schema `format` 与字段名（如 name、email、phone、id/uuid、createdAt 等）启发式产生**每次随机变化**的真实仿真值，而非固定常量。
- 运行控制台「一键生成参数」与「AI 生成请求参数」在 fallback 路径应优先输出 `{{$mock.<helper>}}` 字符串占位（仅限字符串字段，且仅当 schema 未提供 `example` / `default` / `enum`），由 Runner 在每次发请求时实时替换为新值；整数、浮点、布尔、对象、数组等非字符串字段仍输出具体仿真值，避免破坏 JSON 类型。AI 生成的系统提示需告知模型可用 `{{$mock.*}}` helper 列表与适配规则；自动生成模板（importer/generator）保持现有的具体仿真值，避免影响导入态稳定性。
- 运行控制台请求工具栏不单独提供「模拟标签」帮助按钮；可用 helper 列表统一在「平台资源 / 模板与变量参考」页面维护与查阅，运行控制台的 fallback 生成结果天然展示这些标签，无需额外入口。
- 项目可包含多个服务，每个服务仅维护名称与描述（不设服务级 Base Path；请求由环境 baseURL 与导入的接口路径组合）；每个服务独立维护自己的环境配置列表；服务环境包含环境名称、baseURL、变量 JSON 与认证 JSON，Runner 根据请求模板所属服务和选择的服务环境执行请求。
- 提供 Runner，用于执行接口请求模板与场景编排。
- 支持断言，用于校验 API 响应和测试结果。
- 为测试运行生成执行报告。
- 支持删除项目；删除项目时对项目及其关联业务数据执行软删除，默认业务查询不展示已软删除数据。
- 支持项目级别的权限控制：每个项目维护独立的成员列表，成员角色分为「所有者（owner）」、「开发者（developer）」、「观察者（viewer）」三级；拥有全局 `users:manage` 权限的超级管理员可访问所有项目并自动获得所有者权限；普通用户只能看到自己所属项目；创建项目时创建者自动成为该项目所有者；项目成员管理（增删改角色）需要所有者权限；服务与环境的写操作需要开发者或所有者权限；读操作需要任意项目成员身份；成员管理入口在项目列表的操作列。
- 支持项目级 Mock Server：每个项目可维护多个 Mock Server 配置，按端口在主 API 进程内动态启动/停止独立 HTTP 服务；每个 Mock Server 可配置多条规则，规则包含方法、路径（支持 `{id}` 占位段）、优先级、启用状态、query/header/bodyContains/bodyJson 匹配条件及响应状态码、响应头、响应体、响应类型和延迟；规则响应体应支持 Mustache 风格引用当前请求参数，例如 `{{request.pathvar.id}}`、`{{request.query.name}}`、`{{request.body.user.id}}`，便于回显路径参数、查询参数和 JSON 请求体字段；管理后台支持复制已有规则后另存为新规则，并在相关 JSON 输入框失去焦点后自动格式化请求/响应 JSON；规则列表每条规则应提供测试操作，点击后以弹窗或抽屉临时展示测试表单和结果，自动把该规则的方法、地址、query/header/body 匹配条件和期望响应带入，用户可再修改后执行，关闭后不占用规则维护主界面空间；运行中的 Mock 请求需实时从数据库读取规则以便编辑后立即生效；Mock 服务需提供基础 CORS 与 OPTIONS 预检支持，便于管理后台测试区从浏览器跨端口直接发起请求；管理 API 读操作需项目 viewer 权限，新增、编辑、删除、启动和停止需 developer 权限。
- Mock 规则测试弹窗需提供独立 Query JSON 编辑区，打开规则测试时自动带入规则 query 匹配条件，发送测试时与 URL 中已有查询参数合并；当规则响应体使用 `{{request.*}}` 动态模板时，测试弹窗的自动响应校验应基于本次测试请求渲染后的期望响应执行，避免拿未渲染模板与实际响应比较。
- 通过 migrations 维护数据库结构变更。
- 支持测试场景编排：场景由有序步骤组成，步骤类型支持 API、数据库操作和脚本；API 步骤引用**API管理**中的一条请求模板（`test_cases`），步骤间通过**变量提取**传递数据（从响应 body/header/status 提取值，存入变量供后续步骤引用）；`$steps[N]` 引用除支持响应字段（`status`、`headers.*`、`body.*`）外，还支持引用 API 步骤的请求参数：`request.query.*`（查询参数）、`request.pathvar.*`（路径参数，兼容历史 pathvar 语法）、`request.body.*`（请求体字段），语法为 `{{$steps[N].request.query.xxx}}`；数据库步骤复用当前项目下的业务数据源（与环境、服务无绑定），执行单条参数化 SQL 并记录行结果、影响行数和输出变量；脚本步骤在平台内以 **JavaScript**（goja 沙箱）执行，支持多行脚本与 **Postman 风格** 的 `pm.variables` / `pm.environment` / `pm.test` / `console` 等，不通过外部解释器；带超时、逻辑错误时失败、console 输出映射为 stdout/stderr 快照，且 `pm.variables.set` 会写回场景变量供后续步骤与模板使用；步骤可配置独立的变量覆盖和请求覆盖（JSON 合并）；管理后台步骤编辑区卡片顶部仅一行步骤展示名：API 步骤默认显示所引用接口的模板名称（`name`），数据库和脚本步骤默认显示步骤类型名称，双击进入编辑可保存自定义名称（留空则仍视为使用默认名称）；其下一行为步骤类型配置区，API 步骤保留 API 搜索选择及方法与路径工具栏、Params/Headers/Body 等能力，Body 编辑区预览需支持 JSON 对象折叠，并允许在 JSON 值位置直接使用未加引号的模板表达式（例如 `"duration": {{$mock.int(30,60)}}`）通过合法性检查和格式化；已加引号的模板字符串（例如 `"origin_price": "{{$mock.int(1,100)}}"`）在保存、重新打开和请求渲染后仍应保持字符串类型，不得被展示格式化或运行渲染为裸数字；未加引号的完整 mock 模板值在请求渲染时保持为对应 JSON 数字或布尔类型，未加引号的完整 `$steps[N]` 模板值应按渲染结果恢复 JSON 标量类型（数字、布尔、null），无法恢复时保持字符串；场景运行环境仅在场景级选择一次，步骤内不再重复选择环境；左侧步骤列表支持按住行拖拽排序（不与序号按钮、删除冲突）、点击行首序号启用/停用步骤、悬停显示删除图标；左侧步骤列表每行展示序号标签与步骤展示名；步骤列表区域宽度支持用户拖拽调节，调节结果在本地持久化以便下次进入恢复；自定义名称持久化保存；场景执行结果包含每个步骤的运行结果和提取/输出的变量；管理后台场景运行结果面板支持拖拽调整高度，单个步骤标题在步骤名后展示执行时间，步骤内部去掉概览与独立响应 Tab，保留「详情」「断言」「步骤输出」Tab，且「步骤输出」位于「断言」之后；「详情」中请求与响应左右排版，内容框随内容自适应高度，并支持 JSON 对象折叠和语法高亮；管理后台在侧边栏提供「场景编排」独立菜单。
- 场景编排选择运行环境后，API 步骤编辑区应基于当前环境认证配置、请求 `security` 与路径规则，在 Headers 或 Query 等对应位置动态展示将由环境继承的认证参数；继承行未修改时不写入步骤请求覆盖，用户修改后应作为步骤覆盖保存，并可使用 `{{变量名}}`、`{{$steps[N].*}}`、`{{$mock.*}}` 等模板变量覆盖环境认证值。
- 运行控制台选择环境后，应与场景编排相同：根据请求 `security` 与路径规则，从环境认证配置（含 `profiles` / `securitySchemes` / `pathRules` / `defaultProfile`）中选定 profile，并把继承的认证参数动态展示到 Headers / Query 表格相应位置（带「环境认证」标识）；继承行未被用户修改时不写入请求覆盖，被用户修改（编辑值、改名、切换启用、删除）后视为覆盖保存到当前请求并参与发送。当请求未声明 `security` 且环境只有 legacy 单一认证配置时，不附带认证；同 key 已被用户填写时优先用户值，不重复展示继承提示。场景编排与运行控制台共享同一份解析逻辑（`web/admin/src/utils/authInheritance.js`）。
- 场景编排左侧步骤列表：For 循环体与条件分支的 Then/Else 子帧采用同一套布局——不展示顶部的分支标题与「新增步骤」整行，仅在各色边框框体底部边线中央保留圆形「+」入口向对应分支新增步骤（含分支为空时）；上述子帧内的子步骤左侧不展示嵌套竖线装饰；循环体与 Then/Else 共用同一套子步骤布局（含框内约 2px 水平留白、自子帧首层起的深度缩进、序号与类型标签紧凑尺寸），以尽量展示用例名称。
- 场景编排步骤的删除采用软删除（保留 `deleted_at` 历史），但 (scenario_id, step_order) 槽位在删除后应被视为已释放：在同一 stepOrder 上再次保存（含 For/条件/API/数据库/脚本所有类型）必须落库为一条全新的步骤记录（新的 `id` 与新的 `step_seq`），不得复用已被软删除行的占位、也不得保留其 `deleted_at`，以避免保存成功但列表查不到、刷新后丢失的现象。
- 场景编排进入某个场景后，对每个步骤的编辑也要支持多 Tab 切换：右侧步骤编辑区顶部展示步骤级 Tab 条，点击左侧步骤行/「新增步骤」/控制流子帧底部「+」时若该步骤未打开则新开 Tab，否则切到已存在的 Tab；可通过 Tab 关闭按钮关闭单个步骤编辑，关闭后丢弃该 Tab 的内存草稿。同一会话内 Tab 之间的未保存编辑互不影响（在内存中按 Tab 隔离）；浏览器本地仅持久化打开的步骤 Tab 列表与当前激活 Tab，按「项目 + 服务 + 场景」三元组维度按 scope 隔离，刷新或重新打开后恢复仍存在的步骤 Tab，刷新后步骤表单重新基于服务端最新数据装载（草稿不持久化，避免与服务端状态冲突）；未保存的「新增步骤」草稿仅在当前会话保留，不参与本地持久化恢复。步骤被删除或场景被关闭时自动清理对应 Tab。
- 场景编排步骤支持控制流：新增 for 循环和条件分支步骤。循环步骤可按次数或 JSON 数组表达式迭代，提供当前项变量与索引变量，并限制最大迭代次数防止失控；当数组表达式返回对象且对象中包含常见数组字段（如 `items`、`data`、`list`、`rows`、`results`、`records`）时，Runner 可自动展开该数组，降低 API 包装响应的配置成本；条件步骤支持多个「条件—子步骤组」按书写顺序匹配（首个成立的组被执行），并可选配置「否则」默认分支；兼容仅配置一组条件与成立/否则二分支的旧版 JSON（`left`/`operator`/`right` + `thenStepSeqs`/`elseStepSeqs`）。控制流步骤通过稳定的 `step_seq` 引用子步骤，允许子步骤复用现有 API、数据库、脚本步骤能力；管理后台步骤列表需按 For 循环体、条件各分支及否则分支展示层级关系，控制步骤编辑表单不提供执行步骤选择器，只能在各子帧底部「+」入口新增执行步骤并自动绑定到控制步骤配置；运行器需记录控制步骤结果、跳过未进入分支、阻断循环自引用与过深嵌套，并在任一执行步骤失败时让场景失败。
- 在 `tests` 目录提供可运行的示例 API 服务，用于生成 OpenAPI/Swagger 文件并作为自动化测试平台导入与执行的被测服务；示例 API 需支持 JWT 登录，并让业务接口通过 Bearer Token 访问；`/api/v1/admin` 示例接口需使用独立于普通业务接口的鉴权密钥。
- 支持 API Key 认证用于 CI/CD 调用：管理员在「系统管理 → API Key」生成令牌（明文仅展示一次，库内仅存 SHA-256 哈希 + 前后缀掩码），令牌通过 `Authorization: Bearer at-...` 调用平台 API，当前阶段仅允许调用 OpenAPI/Swagger 导入接口（scope=`specs:import`），其余接口对 API Key 来源返回 403；支持禁用、过期与最近使用时间审计。仅拥有 `apikeys:manage` 权限的角色（默认超级管理员）可创建/编辑/删除 API Key。支持重置（rotate）：管理员可在列表里重置某个 API Key，后端生成新明文并使原 token 立即失效，新明文以与创建相同的一次性 Dialog 展示。
- 场景编排「新增步骤」支持「克隆」入口，与 API、数据库、脚本、For 循环、条件分支并列；可选源覆盖**同项目下任意场景**（含当前场景，按服务分组并展示「场景名 · 服务名」），跨服务克隆允许，已被软删的源场景与源步骤不出现在选择列表中。克隆为深拷贝：复制源步骤的名称、类型、`config`（深拷贝 JSON）、`requestOverride` 等字段；若源步骤是 For 循环或条件分支，其循环体和各分支下的子步骤递归一起被克隆（按现有最大嵌套深度限制，不放宽）；克隆产物全部作为全新记录写入目标场景（新 `id`、新 `step_seq`、新 `step_order`），不复用任何已存在的占位，控制流 `config` 中的 `bodyStepSeqs` / `branches[*].stepSeqs` / `elseStepSeqs` 重映射到克隆出来的新 `step_seq`。同项目跨场景克隆的 API/数据库/脚本步骤所引用的 `testCaseId`、`dataSourceId`、脚本模板等外部资源保持原引用不动（即使跨服务也不静默清空），由用户自行后续调整。克隆完成后新步骤按原顺序追加到当前场景列表末尾，复用现有 `upsertScenarioStep` 接口逐条保存，前端无需新增后端 API；保存成功后自动激活克隆出的根步骤 Tab 并 toast 提示。
- 平台支持「AI 智能分析」：
  1. 测试失败分析（`analyze_failure`）：运行控制台单接口运行与场景编排场景运行结果面板均提供「AI 分析失败原因」入口；点击后基于本次 run 的请求/响应快照（`requestSnapshot` / `responseSnapshot`）、断言失败明细、场景内每个失败步骤摘要调用 AI，返回失败原因推断与修复建议。
  2. spec 变更影响分析（`analyze_spec_changes`）：OpenAPI/Swagger 导入完成后，在导入结果提示与 spec 历史列表行提供「AI 分析变更」入口；以本次导入的 spec 与同一 service 上一条已存在 spec 为对比基线（`api_specs` 同 `service_id` 的上一 `version`），后端基于 `normalized_snapshot` 做结构化 diff 得到新增 / 删除 / 修改的 endpoint 与字段清单，再连同当前服务下的接口模板（`test_cases`）与场景步骤（`scenario_steps` 引用了哪些 endpoint）摘要交给 AI，输出变更清单 + 对现有模板与场景步骤的影响评估与建议动作（继续可用 / 需调整 / 应废弃）。
  3. 两类分析复用 `AIGenerateDialog`（或同等弹窗）展示，不写库；后端 action 名 `analyze_failure` / `analyze_spec_changes`，对应可在「项目管理 / Prompt 管理」覆盖 SystemPrompt / providerId / defaultModel；未配置时回落到项目默认 AI 提供商；提供商缺失或 prompt 为空时返回明确中文错误。
- 平台支持「命名值集合（Mock Value Sets）」用于自定义运行时模拟标签的取值池：
  - 项目级管理（与脚本库、AI 提供商同级，归「平台资源」分组）；CRUD 字段：`key`（项目内唯一，仅允许字母/数字/下划线/中划线）、`name`、`description`、`values: string[]`、`weights: number[]`（可选，与 values 等长，未配置则均匀随机）。
  - 模板语法：
    - `{{$mock.set.<key>}}` → 默认按 `weights` 抽样（无 `weights` 则均匀随机）。
    - `{{$mock.set.<key>[N]}}` → 按 0-based 索引取；越界返回明确中文错误。
    - `{{$mock.set.<key>[*]}}` → **同一次运行（run）会话内**按顺序遍历不重复（到末尾后回到 0）。runner 在 run 上下文里为每个 `<key>` 维护游标，单次 run 内的所有渲染共享游标；不同 run 互不影响；mockserver 的请求渲染按 request 维度独立计数（每次请求重置）。
    - `set` 仅一维数组语义，**不支持** `[?col=val]` 过滤。
  - `$ds` / `$sql` 的过滤语法保持 `[col=val]` 不变，本期不切换。
  - 持久化：`mock_value_sets` 表（迁移 `migrations/011_mock_value_sets.sql`），同 `project_id` 内 `key` 唯一约束（含软删 unique partial index）。
  - 后端：`internal/mockset/` 包提供 `Service.Lookup(projectID, key) (values []string, weights []float64, ok bool)`；通过新接口注入到 `internal/templating` 的 Mock resolver；`internal/runner` 在 run 开始时分配 set 游标存入 run-scoped context；`internal/mockserver` 在 request 开始时分配 request-scoped context。`internal/mockdata` 不直接依赖 mockset，仍保持纯函数；`set` 的解析在 templating 的 Mock hook 里完成。
  - AI 集成：`generate_params` 的 user context 自动带上当前项目所有 `mock_value_sets` 摘要（`key` + `name` + `values` 前 10 条 + 是否含 `weights`），系统提示要求模型对业务字段优先输出 `{{$mock.set.<key>}}`，避免编造。
  - 内置业务 helper（与 set 并存，无需用户配置）：`internal/mockdata` 注册 `idCard`（中国二代身份证 18 位含校验位）、`plateNumber`（中国车牌号，省份简称 + · + 字母 + 5 位字母数字）、`bankCard`（16-19 位 Luhn 合法卡号，默认 19）、`unifiedSocialCreditCode`（统一社会信用代码 18 位，含 GB 32100 校验位）、`sku`（默认 `[A-Z]{2}-\d{6}`，可指定长度）；同步更新 `web/admin/src/utils/templateTokens.js` 的 `mockHelperList` 与「模板与变量参考」页。
  - 权限：`mockset:read`（viewer+ 可读）/ `mockset:write`（developer+ 可写），通过项目级 `RequireProjectRole` 控制（与 SQL 参数源、Mock Server 一致）。
  - 不引入"用户脚本自定义 helper"概念。
- MVP 聚焦核心流程：导入 API 定义、创建/生成接口模板、组织场景、运行测试、断言结果、查看报告。

## 管理后台系统

管理端界面需求：

- 使用 Vue 3 和 Element Plus 构建管理后台。
- 提供登录流程；服务启动时按 `ADMIN_USERNAME` / `ADMIN_PASSWORD`（未设置时默认 `admin` / `admin123`）与库中默认管理员账户对齐密码，使文档化凭据可登录（生产环境应通过环境变量设置强密码）。
- 提供主应用布局；侧边栏将「运行控制台」作为与「API管理」并列的独立一级菜单，进入运行工作台时高亮该菜单（从API管理等入口进入运行页时亦同）。
- 增加路由守卫，用于登录态和权限导航控制。
- 提供接口自动化测试平台的功能页面。
- 脚本断言（运行工作台断言编辑器、场景编排内 API 步骤断言）与场景「脚本」步骤编辑区提供「脚本库」：内置常用 JavaScript 模板存于数据库（全平台共享），可在「脚本库」页编辑内容；选择后追加到编辑框，用户再按需修改占位参数；支持按**当前全局项目**维护自定义脚本模板（新增、编辑、删除），自定义模板与内置模板一起在脚本库面板中展示；管理入口为侧边栏「脚本库」页面。脚本库新增/编辑模板时，「脚本内容」编辑区上方提供 AI 生成入口（复用 `generate_assertion` 动作），生成结果以追加方式合并到当前脚本内容；context 同步当前模板的名称、描述、分类、`scopes` 与已有 `currentCode`，以辅助 AI 适配输出风格（`scopes` 仅含 `scenario` 时倾向 `pm.variables` / `console`，含 `assertion` 时倾向 `pm.test` + `pm.response`）。
- 包含用户与权限管理。
- 支持 RBAC 风格的访问控制，覆盖角色、权限和受保护功能。
- 用户管理、角色管理、权限菜单应归入同一个独立的二级菜单分组展示。
- Web 界面中的时间展示统一使用 `YYYY-MM-DD HH:mm:ss` 格式。
- 管理后台的项目选择属于全局上下文，应在主应用顶部统一选择一次；服务与环境、场景编排、API管理与运行控制台等页面应复用当前全局项目，不再要求各页面重复选择项目；侧边栏仅保留「项目管理」一项进入统一页面，该页采用**主从布局**：左侧为项目列表（选择项目并同步为当前全局项目），右侧以标签区分「服务与环境」「业务数据源」；项目行操作可快速打开右侧对应标签；历史路径 `/services`、`/data-sources` 重定向到该页对应标签；OpenAPI/Swagger 导入与接口列表应合并为单个「API管理」页面，页面内统一选择服务，上方承载导入和历史记录，下方展示接口请求模板列表；当无可选项目或未选中项目时，顶部项目选择框占位文案为「请选择项目」。
- 浏览器应默认记住上次选择的全局项目和运行环境，并在下次打开时优先恢复仍然有效的选择。
- 服务与环境管理页应基于当前全局项目展示服务与环境配置；未选择全局项目时，「新增服务」不可操作，并提示先选择项目。
- 服务与环境管理页应记住当前项目下上次选择的服务（本地持久化）；服务与环境以树形展示，环境作为服务的子级节点（子目录结构）；选中服务或某环境下的环境节点时与持久化的当前服务联动高亮。
- 服务与环境管理支持新增、编辑和删除服务与环境配置。
- 环境配置弹窗中的变量 JSON 与认证 JSON 需在字段旁提供提示图标，避免字段标签被挤压折行；鼠标悬停时分行展示填写方式和示例；示例 JSON 需格式化展示以便阅读。
- 环境配置弹窗（运行控制台内编辑环境与服务与环境管理页）应足够宽大以便编辑；变量 JSON 与认证 JSON 文本框应随内容在一定范围内自适应高度，并在输入框失去焦点后自动格式化 JSON，不再提供单独的 JSON 格式化按钮。
- OpenAPI/Swagger 导入页的服务选择框应随容器宽度自适应布局，并提供清晰的选择提示与空数据提示。
- API管理页的服务筛选选择框应随容器宽度自适应布局，并提供清晰的选择提示与空数据提示；页面入口权限使用 `cases:read`，OpenAPI/Swagger 导入操作使用 `specs:import`，手工新增接口请求模板操作使用 `cases:write`。
- 运行控制台配置和运行结果应通过运行子页面承载；界面参考 Apifox 工作台风格，提供左侧接口树、顶部接口 Tab（同一接口可开多 Tab）、请求方法与发送工具栏、环境选择、请求参数分区、响应结果分区等核心交互（路径模板编辑位于 Params 的 Path 区域）；HTTP 请求与响应需按 URL、Params、Headers、Body、变量、断言等部位分区编辑和展示，避免把完整请求或结果堆在单个纯文本框中；Params 分区内应单独划分 Path（路径模板与路径参数）与 Query，并展示 OpenAPI/Swagger 参数说明；Query 参数表格不使用参数类型、图标或「必填/非必填」视觉标签标识必填性，仅在必填参数的参数值后方显示红色 `*`；非必填参数默认不自动填充模拟数据；请求配置区不展示「上次响应」页签；Path 路径参数子区显隐按"是否存在 `{变量名}` 占位 + 模板来源"判定：路径里存在占位时一律展示；路径无占位时，仅手工新建模板（`source != 'auto'`）展示空表格便于临时补占位符，OpenAPI/Swagger 自动生成模板（`source == 'auto'`）直接隐藏 Path 子区，避免无意义的空表格干扰；该规则同时适用于运行控制台与场景编排 API 步骤编辑区；「一键生成参数」接口返回的路径参数映射字段名为 `path`；运行控制台一键生成 Query 参数时应以参数启用状态决定是否生成/带入值，并按参数名尽量保留已有启用状态；运行控制台在请求方法为 `POST` 或 `PUT` 时应默认选中 Body 分区（含存在路径参数占位时）；请求 Body 编辑器和响应 Body JSON 框右侧需单独以类似 Apifox 的紧凑表格/树形风格展示结构、类型、必填标记、字段限制和字段含义，字段限制需覆盖 OpenAPI/Swagger schema 中常见的 enum、长度、数值范围、数组数量、对象属性数量、pattern、multipleOf、nullable、readOnly/writeOnly、deprecated 等约束；导入 OpenAPI/Swagger 时必须将上述约束保留进 `requestSchema` / `responseSchema`，前端展示需兼容常见 `min/max`、`minLen/maxLen`、`validation/constraints/rules` 与 `x-*` 扩展别名，并以简洁明确的符号语言呈现；字段含义优先来自 OpenAPI/Swagger `description`，缺省回退 `title`，不展示冗长默认说明，且右侧注释区默认与 JSON 区各占一半宽度，支持拖拽调节并在本地记忆；单次运行完成后应在结果区默认定位到响应 tab，便于多次运行。
- 运行结果的「响应」区须先展示响应 Body，其下方以子 Tab 切换「响应头」与「实际请求」（基于本次 requestSnapshot 的 curl 命令）；响应状态码标签应展示在「运行结果」区域右上角（独立运行结果页展示在页面标题行右侧）；仅在底层请求失败（无 HTTP 响应，例如 DNS、连接、TLS 错误）时，在状态码标签旁追加一个 danger 类型标签展示错误原因，请求成功完成时不展示该标签，避免冗余文案。
- 运行工作台：左侧展示按 OpenAPI/Swagger `tags` 分组的树状 API 列表并可从接口节点打开请求模板；**服务**在左侧 API 列表上方通过单独的下拉框选择，不在树节点中展示；服务选择与「服务与环境」页共用同一套项目内持久化键；API 列表区域宽度应支持用户拖拽调节，调节结果在本地持久化以便下次进入恢复；左侧 API 列表与右侧运行控制台应限制在工作台高度内各自独立滚动，避免页面整体滚动互相带动；API 列表与运行控制台之间的间隙及 API 列表卡片内边距应保持紧凑；某接口仅关联一条可运行模板时，该接口在树中仅占一行（不再出现接口行 + 子级模板行）；接口下多条模板时仍保留接口为父节点、模板为子节点；右侧展示运行控制台；多个接口可同时打开，并通过右侧顶部横向 Tab 切换；顶部 Tab 仅展示 API `summary`（缺省回退路径或模板名称），不展示 HTTP 方法、运行状态或可编辑用例名；运行控制台打开的全部接口 Tab、当前激活 Tab 与每个接口尚未保存的请求参数草稿应按项目/接口在浏览器本地持久化，切换菜单或刷新后自动恢复仍存在且可访问的全部接口 Tab（若 URL 指向某个接口，则恢复后激活该接口），避免重复填写；用户关闭某接口 Tab 时视为丢弃该接口本地草稿，下次重新打开该接口应展示模板默认状态；运行控制台内的状态、结果状态与耗时应使用颜色和图标紧凑展示，避免占用过多空间；运行工作台内左右区域、Tab 条、请求配置和运行结果等框体之间应有清晰边界与背景层次。
- 运行控制台发送模块上方不展示接口 summary 或自动生成模板名称（如 `Happy path POST /xxx`）；接口识别通过工作台顶部 Tab 的 API `summary` 承载。
- 运行控制台应支持在运行前弹出编辑当前选择的环境配置，复用服务与环境管理已有字段、接口和交互模式；环境选择应作为整体控制放在运行控制台右上角，编辑入口位于环境下拉列表**每一行**环境名称的右侧、以图标触发，且默认隐藏，仅当鼠标悬停**该条**环境项时显示；运行控制台不展示显式“关闭当前”和“关闭页签”按钮。
- 运行控制台展示的本次 API 请求耗时应根据响应时间着色：小于 100ms 显示为绿色（与“通过”同色板），达到或超过 100ms 显示为黄色，未运行时保持中性色。
- 运行控制台和场景编排的路径参数应保留并显示 OpenAPI 风格的 `{变量名}`，例如 `/api/v1/users/{id}`；动态值（如 `{{$steps[N].body.data.id}}`）应填写在 Path 参数表对应变量值中，保存时保持路径模板与变量值分离，仅在运行时替换。
- 运行控制台和场景编排的请求体 JSON Body 输入框本身需支持 JSON 语法高亮与对象折叠，长行应自动折行而非横向滚动，交互能力应与响应 Body 的 JSON 展示保持一致。
- 运行控制台的一键自动生成参数需覆盖查询参数、请求头、路径参数和 JSON Body；对 OpenAPI/Swagger 的 `$ref`、`example`、`default`、`enum` 与基础类型 schema 应生成可直接编辑的默认值。
- 请求模板应支持从被测业务数据库执行 SQL 查询获得动态参数：平台需提供**按项目维护**的业务数据源配置（不与环境或服务绑定），以及可被请求模板内联引用、仍按项目与服务维度管理的 SQL 参数源；SQL 参数源通过 `{{sql.<sourceKey>.<column>}}` 默认取第一行列值，通过 `{{sql.<sourceKey>[<filterColumn>=<filterValue>].<column>}}` 在查询结果中按简单等值过滤后取第一条匹配，无需绑定式参数源或结果映射 JSON。
- 管理后台应提供业务数据源与 SQL 参数源管理界面：业务数据源仅按全局项目维护与测试连接；SQL 参数源按项目与服务维护，配置可复用 SQL、入参 JSON、预览结果，并展示可在路径、查询参数、请求头和请求体中使用的内联模板示例；不再提供绑定式参数源、保存绑定顺序、结果映射 JSON 或绑定式多行数据驱动配置。
- SQL 参数源的新增与编辑弹窗中应支持在未保存前根据当前表单内容测试 SQL（只读查询），与列表「预览」一致返回首行和过滤引用可使用的原始结果执行快照。
- SQL 参数源管理页应在界面中说明「动态参数」：入参 JSON 数组项与 SQL 位置参数 `$1、$2…` 顺序对应；可通过 `name` 从运行合并变量取值，或通过 `value` 中的 `{{变量名}}` 模板拼接；`required` 语义与预览变量 JSON 的对应关系。
- 请求定义中的路径、查询参数、请求头和请求体字符串应支持 Mustache 风格内联 SQL 引用：`{{sql.<sourceKey>.<column>}}` 默认取 SQL 参数源结果第一行列值，`{{sql.<sourceKey>[<filterColumn>=<filterValue>].<column>}}` 在多行结果中按简单等值过滤后取第一条匹配；Runner 应在运行前自动扫描并执行引用到的 SQL 参数源，无需强制绑定或配置结果映射，找不到来源、匹配行或列时返回明确错误。SQL 参数源应提供适合模板使用、同项目服务下唯一的可读 key，并兼容历史名称或 ID 作为引用兜底。
- 请求定义中的路径、查询参数、请求头、请求体字符串以及环境/请求变量值应支持 Mustache 风格的**运行时数据模拟标签** `{{$mock.<helper>}}` 与 `{{$mock.<helper>(arg1,arg2,...)}}`：每次请求渲染时实时生成一个值，多次出现各自独立生成，不写入用例快照模板。helper 名大小写不敏感，参数支持单引号或双引号包裹（用于含逗号或空白的参数）；常用 helper 至少应覆盖 `uuid`、`now(layout?)`、`timestamp(unit?)`（`s`/`ms`/`ns`，默认 `s`）、`int(min?,max?)`、`float(min?,max?,prec?)`、`bool`、`string(n?)`、`word`、`sentence(words?)`（生成中文句子）、`name`（中文姓名）、`firstName`（中文名）、`lastName`（中文姓）、`email`、`phone`（中国手机号格式）、`url`、`ipv4`、`ipv6`、`city`、`country`、`address`（中文地址）、`company`、`color`、`date(layout?)`、`dateTime(layout?)`、`pick(a,b,c,...)`（`oneOf` 为别名）；未识别的 helper 应保留原始标签字面量并视为错误（避免与未来变量冲突）。Mock Server 响应模板与运行控制台/场景 API 步骤的请求渲染管线均需支持上述标签；标签解析必须在场景步骤引用 `{{$steps[N].*}}`、SQL 内联引用 `{{sql.*}}` 与普通变量 `{{varName}}` 替换之前完成，且不参与这些引用的扫描收集，避免与之冲突。
- 管理后台「平台资源」分组下提供独立的「模板与变量参考」子页面（路由 `/template-reference`），按类别集中说明所有占位符与函数：环境与运行变量 `{{varName}}`、场景步骤引用 `{{$steps[N].*}}`、SQL 内联引用 `{{$sql.*}}`（兼容历史 `{{sql.*}}`）、测试数据内联引用 `{{$ds.*}}`、OpenAPI 路径参数 `{name}`、运行时模拟标签 `{{$mock.*}}`、Mock 响应模板 `{{$req.*}}`（兼容历史 `{{request.*}}`）；每类需展示语法、生效范围、示例代码与注意事项，并展示统一的渲染顺序说明（先模拟标签 → 步骤引用 → 测试数据/SQL 内联引用 → 普通变量）；模拟标签部分需提供搜索过滤与一键复制示例。前端 helper 列表应以单一事实来源（`web/admin/src/utils/templateTokens.js`）维护，与后端 `internal/mockdata.ListHelpers` 同步增减。平台内置命名空间统一约定为 `{{$<namespace>.<...>}}`（`$mock`、`$steps`、`$ds`、`$sql`、`$req`），无 `$` 前缀的 `{{varName}}` 表示用户自定义环境/运行变量；历史 `{{sql.*}}` / `{{request.*}}` 形式继续兼容但被视为 deprecated，新代码与界面文案应使用带 `$` 的形式。模板语法的扫描、归一化与渲染由 `internal/templating` 统一负责，所有调用方（HTTP Runner、Mock Server、SQL 参数源 Executor、测试数据服务）必须通过该包消费占位符，禁止在调用方再维护各自的正则。
- 管理后台支持整体字体大小调整和配色修改；字体大小需提供多个宽松等级，用户选择需在本地持久化并作用于全局界面，表单、下拉、表格、弹窗、菜单和弹层等组件字体应尽量保持一致。
- 管理后台应内置多套参考主流设计最佳实践的配色方案，覆盖浅色、深色、科技感、极简、自然、温暖和金融等常见风格，并继续支持自定义主色。
- 管理后台左侧菜单：**项目管理**为独立一级菜单（主从页维护服务与环境、业务数据源等，依赖顶部全局项目）；**测试数据**为独立分组（含 SQL 参数源与「测试数据表」；后续测试数据构造功能继续归入该分组），依赖顶部全局项目；**平台资源**（全平台共享的脚本库与项目级 AI 提供商配置）与「API 管理」「运行控制台」「场景编排」「系统管理」等并列；路由路径保持不变。
- **测试数据表**（`/test-data-tables`）：项目级，跨服务可复用，由表（`key`、名称、描述）+ 多列（`key`、名称、类型、说明、生成方式）+ 多行（每个单元格为字符串值）组成。列生成方式三选一：「手动输入」（仅占位，由用户自行填值）、「数据模拟函数」（复用 `{{$mock.*}}` helper，调用 `internal/mockdata.Eval`）、「AI 生成」（仅在列上勾选 `kind=ai` 启用，不再保存列级 providerId / prompt / model）。AI 生成在调用 `action=generate_case_data` 时会先读取项目级 Prompt 管理（`project_ai_prompts`）：`SystemPrompt` / `providerId` / `defaultModel` 由该配置控制；`providerId` 为空时回退到项目默认 AI 提供商（`is_default=true && enabled=true`），都没有则返回明确中文错误「请先在项目 Prompt 管理或 AI 提供商中配置 generate_case_data 的提供商」。AI user 上下文由后端组装：表名、表说明、列定义（key/name/type/description）、目标列、要生成的行数、已有非空行作为 JSON 上下文随 `Context` 字段传给 AI；模型回填响应中的 `cases:[{colKey:...}]`、裸数组或顶层 `{colKey:...}` 对象，按目标列读出值。行批量生成支持指定 N 行（默认上限 100 行），可选「保留已手填」单元格；提供按列重新生成入口（`POST /test-data-tables/{tableID}/rows/{rowID}/regenerate`），两者请求体均不再接受 `providerId`；行 JSON 字段保持 `values:{colKey:string,...}` 形式。AI 列生成失败、provider 缺失、prompt 未配置或 helper 未识别时返回明确错误或聚合的 `warnings`，不影响其他列。前端 `TestDataTableList` 列定义里 `kind=ai` 不再展示 provider 下拉、prompt 文本、model 输入，只显示提示「AI 生成由项目 Prompt 管理（generate_case_data）配置；未配置时回退到项目默认 AI 提供商」并提供跳转到项目管理 Prompt 标签（`/projects?tab=prompts`）的链接；批量生成与按列重生成弹窗不再保留 provider 覆盖字段。旧版列定义中保留的 `generator.ai.providerId / prompt / model` 在反序列化时被静默忽略，不阻塞读取。
- 请求模板与场景 API 步骤的路径、查询参数、请求头、请求体字符串以及环境/请求变量值应支持 Mustache 风格内联测试数据引用：`{{$ds.<tableKey>.<col>}}` 取目标表首行的指定列值，`{{$ds.<tableKey>[<filterCol>=<filterValue>].<col>}}` 在表行中按等值过滤后取首条匹配。Runner 在执行前自动扫描并解析 `{{$ds.*}}`，找不到表、行或列时返回明确错误，并以 `testdata.Snapshot` 形式记录到运行结果（与 `paramsource.ExecutionSnapshot` 并列）。解析顺序与 SQL 内联引用一致：先 `{{$mock.*}}` → `{{$steps[N].*}}` → `{{$ds.*}}` 与 `{{sql.*}}` → `{{varName}}`，互不冲突。
- 测试数据表权限：viewer 可读，developer 可写（与 SQL 参数源、Mock Server 一致）；`GET /api/v1/test-data/mock-helpers` 直接复用 `internal/mockdata.ListHelpers()` 返回 helper 元数据。
- 平台资源支持配置项目级 **AI 提供商**：每个项目可维护多份 AI 提供商配置，支持类型 `deepseek`、`xiaomi`、`openai`、`anthropic`、`kimi`、`ollama`；配置含名称、Base URL、API Key（API 仅返回脱敏 mask 形式）、默认模型、`extraConfig`（如 OpenAI 组织 ID、自定义 headers、Anthropic 版本）、启用与是否默认；同一项目最多一个默认提供商；developer+ 可写、viewer 可读；提供「测试连接」入口验证可达；运行控制台「生成参数」按钮旁、断言/脚本编辑区与场景脚本步骤区提供统一的「AI 生成」入口，支持 `generate_params`（生成请求参数 JSON，按 path/query/headers/body 合并到当前编辑值；系统提示需告知模型可用 `{{$mock.*}}` 运行时标签，并要求模型对 id、email、createdAt 等动态字段优先输出 `{{$mock.<helper>}}` 字符串占位，使运行时每次自动生成新值）、`generate_assertion`（生成 Postman 风格 pm.test 脚本，追加到当前断言/脚本编辑区；**须在弹窗中填写非空「测试意图」**，描述希望校验的响应要点或业务规则；后端拒绝空意图）、`generate_case_data`（生成多行测试数据 JSON，预览后供后续手动应用）。Ollama 类型可不填 API Key；Anthropic 走官方 Messages API，其余类型统一走 OpenAI 兼容 `/v1/chat/completions`。
- AI 生成请求参数（`generate_params`）传给模型的 context 必须基于 `api_endpoints` 上真实存在的字段构造，禁止引用 `endpoint` 上未定义的属性（例如 `endpoint.parameters`、`endpoint.responses`、`endpoint.description` 等都嵌套在 `requestSchema` / `responseSchema` 内，前端不得直接展开）。context 至少需包含：`method`、`path`、`pathVarNames`（依据 OpenAPI 占位符与 `requestSchema.parameters[*].in=path` 合并去重得到，明确告诉模型 `path` 子对象只能出现这些键）、`endpoint.summary` / `operationId` / `tags` / `requestSchema` / `responseSchema`、以及 `currentRequest.{pathVars,query,headers,body}`（仅传入用户已启用且非空的行）。系统提示需显式声明这些字段含义，并要求模型**保留 currentRequest 中已存在且非空的值**，仅补全缺失字段，避免覆盖用户在表单上手填的真实业务值；在 user 消息前自动追加一段任务说明，确保模型不把上下文当作无关噪声。`buildMessages` 行为应通过单元测试锁死，包含「禁止 markdown 围栏」「`pathVarNames` 约束」「`currentRequest` 保留已填值」等关键约束词。
- 「项目管理」或其他入口进入的 **项目级 Prompt（`project_ai_prompts`）可按动作维护 System Prompt / 默认模型等**；每项可**可选**绑定 `providerId`：**留空或未设置**时表示该动作仍跟随项目默认 AI 提供商；若填写则须为同项目且未删除的提供商记录；前台「AI 生成」在满足启用等条件时使用该提供商，否则回落到原先默认提供商选择逻辑。
- 管理后台菜单侧边栏应支持手动收起；收起后仅展示菜单图标，保留路由导航与当前菜单高亮。
- 主应用左侧导航菜单区域不允许单独滚动（不出现侧边栏内纵向滚动条）；主布局应限制在视口高度内，主内容过长时仅在右侧主内容区（如运行控制台）滚动，左侧导航不随页面整体纵向滚动。
- 管理后台侧边栏左下角展示当前管理员头像（支持头像图片 URL，缺省时展示姓名缩写）与姓名；退出登录仅能通过点击头像后在下拉菜单中选择，不得在侧栏常驻展示退出登录入口。
- 管理后台应提供符合接口自动化测试平台定位的品牌 logo，并在登录页、主应用侧边栏和浏览器标签页中展示。

## 工程约束

- 主力编程语言：Go。
- 数据库：PostgreSQL。
- 不要编辑 `/Users/jiangyang/.cursor/plans/接口测试平台设计_3a0d824a.plan.md`。
- 除非明确要求，不要创建长篇文档。
- 不要编辑其他并行工作创建的无关前端或后端文件。
- 不要回滚或格式化其他任务产生的无关变更。
- 根目录提供 `Makefile` 初始化入口，支持读取 `.env` 并优先连接外部 PostgreSQL 执行数据库就绪等待和 migrations 初始化；需要 Docker Compose 托管 PostgreSQL 时应通过显式开关启用。
- 对话中出现的长期有用信息，应在合适时沉淀到 Cursor 项目规则或需求归档中，以提高后续协作质量。
- 后续新增需求、需求变更和范围澄清必须同步到本文档。
