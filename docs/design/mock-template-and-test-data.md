# Mock、模板变量与测试数据设计

本文档记录 Mock Server、Mock Value Sets、运行时模拟标签、模板变量、SQL 参数源和测试数据表的业务设计。

## Mock Server

- 支持项目级 Mock Server。
- 每个项目可维护多个 Mock Server 配置，按端口在主 API 进程内动态启动/停止独立 HTTP 服务。
- Mock Server 规则包含方法、路径、优先级、启用状态、query/header/bodyContains/bodyJson 匹配条件及响应模式（普通响应 / HTTP 重定向 / Schema 自动生成 / 录制回放）、状态码、响应头、响应体、响应类型和延迟。
- 重定向模式用于模拟 SSO/OAuth 授权端点：返回 3xx 状态码并设置 `Location`；`redirectLocation` 支持与响应体相同的 `{{request.*}}` / `{{$req.*}}` / `{{$mock.*}}` 模板，便于把 `redirect_uri`、`state` 等请求参数拼回回调地址。
- 规则响应体支持 Mustache 风格引用当前请求参数。
- 运行中的 Mock 请求实时从数据库读取规则，以便编辑后立即生效。
- Mock 服务提供基础 CORS 与 OPTIONS 预检支持。
- 管理 API 读操作需项目 viewer 权限，新增、编辑、删除、启动和停止需 developer 权限。
- Mock 规则列表提供复制、JSON 输入框失焦格式化、测试弹窗等能力。
- 动态模板响应的自动响应校验应基于本次测试请求渲染后的期望响应执行。
- 访问日志持久化到 PostgreSQL：Mock 运行时记录每次 HTTP 访问（含未匹配 404 与渲染失败），OPTIONS 预检不记录；请求/响应 Body 各截断至 4KB，`Authorization` 等敏感请求头脱敏；删除 Mock Server 时日志随 FK 级联删除。viewer 可分页查询，developer 可清空当前 Server 日志；管理页提供筛选、详情抽屉与运行中 5 秒自动刷新。

## Mock Value Sets

- 平台支持「命名值集合（Mock Value Sets）」用于自定义运行时模拟标签取值池。
- Mock Value Sets 为项目级管理，归「平台资源」分组。
- 字段包含 `key`、`name`、`description`、`values: jsonb[]`、可选 `weights`。
- `key` 项目内唯一，仅允许字母、数字、下划线、中划线。
- 模板语法支持 `{{$mock.set.<key>}}`、`{{$mock.set.<key>[N]}}`、`{{$mock.set.<key>[*]}}`。
- `[*]` 在 Runner 的单接口/场景一次 run 会话内顺序遍历不重复，并共享同一 run 的游标。
- Mock Server 渲染响应时按每个 HTTP request 独立创建游标，因此同一 Mock Server 的不同请求之间不共享 `[*]` 计数。
- `set` 仅一维数组语义，不支持过滤。
- `$ds` / `$sql` 的过滤语法保持 `[col=val]` 不变。
- 后端通过 `internal/mockset` 注入到 `internal/templating` 的 Mock resolver。
- `internal/mockdata` 保持纯函数。
- 内置业务 helper 与 set 并存，覆盖 idCard、plateNumber、bankCard、unifiedSocialCreditCode、sku 等。

## 运行时模拟标签

- 请求定义中的路径、查询参数、请求头、请求体字符串以及环境/请求变量值支持 Mustache 风格运行时数据模拟标签 `{{$mock.<helper>}}` 与 `{{$mock.<helper>(...)}}`。
- Mock 标签每次请求渲染实时生成值。
- 多次出现的 Mock 标签各自独立生成。
- Mock 标签不写入用例快照模板。
- helper 名大小写不敏感。
- 参数支持单引号或双引号。
- 未识别 helper 应保留原始标签字面量并视为错误。

## 模板变量

- 标签解析顺序为：模拟标签 → 步骤引用 → 测试数据/SQL 内联引用 → 普通变量。
- 管理后台「平台资源」分组下提供「模板参考」页面。
- 模板参考页集中说明环境变量、步骤引用、SQL 引用、测试数据引用、OpenAPI 路径参数、Mock 标签、Mock 响应模板等。
- 平台内置命名空间统一约定为 `{{$<namespace>.<...>}}`。
- 历史 `{{sql.*}}` / `{{request.*}}` 形式继续兼容但视为 deprecated。
- 模板语法扫描、归一化与渲染由 `internal/templating` 统一负责。
- 调用方禁止再维护各自正则。

## SQL 参数源

- 业务数据源为**全局平台资源**（不按项目隔离），在「平台资源 → 业务数据源」维护；SQL 参数源仍按项目 + 服务维护，可引用任意平台业务数据源。
- 请求模板可通过 `{{$sql.<sourceKey>.<column>}}` 内联引用 SQL 查询结果。
- 请求模板可通过过滤形式内联引用 SQL 查询结果。
- 历史 `{{sql.*}}` 形式继续兼容但视为 deprecated，新文档和界面文案应使用 `{{$sql.*}}`。
- Runner 执行前自动扫描并解析 SQL 引用。
- 找不到来源、匹配行或列时返回明确错误。
- 当前 SQL 参数源与脚本库 HTTP 接口主要依赖登录态和 `projectId` 参数区分项目，未像 Mock Server、Mock Value Sets、测试数据表一样统一挂项目角色中间件；涉及权限加固时需先确认是否改变现有访问模型。

## 测试数据表

- 支持项目级测试数据表。
- 测试数据表由表、列、行组成。
- 列生成方式包括手动输入、数据模拟函数、AI 生成。
- 请求模板与场景 API 步骤支持 `{{$ds.<tableKey>.<col>}}` 与过滤形式内联测试数据引用。
- Runner 执行前自动扫描并解析测试数据引用。
- 找不到表、行或列时返回明确错误。
- 测试数据表权限为 viewer 可读、developer 可写。
- `GET /api/v1/test-data/mock-helpers` 复用 `internal/mockdata.ListHelpers()`。
- 当前测试数据表页面会调用后端 helper 列表；模板参考页主要使用前端静态 `templateTokens.js`，维护 helper 文案时需同步前后端。

## AI 集成

- `generate_params` 的 user context 自动带上当前项目所有 Mock Value Sets 摘要。
- 系统提示要求模型对业务字段优先输出 `{{$mock.set.<key>}}`，避免编造。
- 测试数据表的 AI 生成规则见 `docs/design/ai-capabilities.md`。

## Schema Mock 数据生成（`internal/mockgen`）

- 平台提供基于 JSON Schema 的自动 Mock 数据生成能力，用于 Mock Server 的 `responseMode=schema` 模式。
- **生成优先级**：enum > example > default > nullable（10% 概率 nil）> type 约束 > 语义推断。
- **类型支持**：string（含 format/语义推断）、integer（min/max 约束）、number（multipleOf 精度）、boolean、array（minItems/maxItems/items）、object（properties/required/additionalProperties）。
- **组合 Schema**：支持 `allOf`（合并）、`oneOf`/`anyOf`（随机选一个）。
- **递归保护**：最大深度 8 层，防止无限递归。
- **语义推断**（`SemanticInferrer`）：当 schema 缺少 format/example 时，根据字段名模式（email、phone、name、address、url、ip、uuid、date、idCard、plateNumber、bankCard、uscc 等 30+ 种）自动生成语义合理的值。支持中文手机号、身份证号、车牌号、银行卡号、统一社会信用代码等中国特色业务数据。
- **Schema 约束遵循**：
  - string：format（email/uri/date-time/ipv4/uuid 等）、minLength/maxLength、pattern（预留）。
  - integer：minimum/maximum、exclusiveMinimum/exclusiveMaximum。
  - number：minimum/maximum、multipleOf（精度推断）。
  - array：minItems/maxItems（上限 10）、items 递归。
  - object：可选字段 80% 概率生成，readOnly 字段跳过。
- `mockgen.Default()` 返回默认配置的 Generator，复用 `internal/mockdata` 的 gofakeit helpers。
- Mock Server 的 `responseMode=schema` 路由在每次请求时调用 `Generator.GenerateJSON(schema)` 实时生成响应数据。

## Mock 录制与回放（`internal/mockrecord`）

- 平台支持 Mock 路由级别的**录制与回放**能力，用于将真实服务的响应录制下来后回放。
- **录制模式**：将 Mock 路由的 `responseMode` 设为 `record`，配置 `recordTargetURL`（真实服务地址）。请求到达时，`Recorder.RecordAndProxy` 将请求转发到真实服务，录制响应并返回。
  - 转发时复制原始请求头（排除 hop-by-hop 头）；
  - Query 参数规范化后哈希（排序、忽略空值），用于精确匹配；
  - 录制数据持久化到 `mock_recordings` 表（method、path、queryHash、requestBody、statusCode、headers、body、hitCount）。
- **回放模式**：将 `responseMode` 设为 `replay`。请求到达时，`Player` 按 method+path 精确匹配（优先 queryHash 一致的录制），返回录制的响应。每次命中 `hitCount++`。
- **管理 API**（挂载在 `/projects/{projectID}/mock-servers/{serverID}/routes/{routeID}` 下）：
  - `GET /recordings`：列出路由的录制数据（viewer 权限）。
  - `POST /record`：启动录制模式，需指定 `targetUrl`（developer 权限）。
  - `POST /replay`：启用回放模式（developer 权限）。
  - `DELETE /recordings`：清空所有录制数据（developer 权限）。
  - `DELETE /recordings/{recordingID}`：删除单条录制（developer 权限）。
- 录制数据按 `(mock_server_id, mock_route_id)` 组织，删除 Mock Server 时级联删除。
