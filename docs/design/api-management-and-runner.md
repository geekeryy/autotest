# API 导入与运行控制台设计

本文档记录 OpenAPI/Swagger 导入、API 管理、请求模板、运行控制台和环境认证继承的业务设计。

## 待用户决策

- `generate_params` / 「一键生成参数」的覆盖范围存在历史冲突：一种说法要求仅覆盖 path、query、body，且不返回或覆盖 headers/security；另一种说法要求覆盖 query、headers、path、JSON Body。相关实现变更必须先由用户明确决策后再继续。

## OpenAPI/Swagger 导入

- **MCP 导入**：独立进程 `cmd/mcp` 通过 MCP 工具 `import_swagger`、`import_swagger_from_url` 调用同一 `POST .../specs/import` 接口（API Key + `specs:import`）。配置与 Cursor 示例见 [mcp-swagger-import.md](mcp-swagger-import.md)。
- 支持 **OpenAPI 3.0.x / 3.1.x** 原生导入，以及 **Swagger 2.0**（经 kin-openapi 转换为 OpenAPI 3 后解析）。导入器统一产出 OpenAPI 3 语义的 `requestSchema` / `responseSchema`。
- 对 vendor 文档中常见的非严格校验问题（schema 上误用 `examples`、default 与 enum 标签不一致、额外 sibling 字段如 `BDCZLXXLIST`）采用宽松校验，保证 imperfect spec 仍可导入端点与约束。
- PathItem 级 `parameters` 会合并进各 operation 的 `requestSchema.parameters`；同名同 `in` 时 operation 级覆盖 path 级。
- `POST .../specs/import` 响应体仅返回导入统计信息，不返回完整端点列表或 `normalized_snapshot` 等大字段；完整 data 通过既有只读接口获取。
- 当前导入响应已使用统计摘要；当前 spec 列表模型仍可能序列化 `normalizedSnapshot`，前端不应依赖列表里的大字段作为展示主数据。
- 重复导入同一服务下内容相同的 OpenAPI/Swagger 应保持幂等，不因内容哈希唯一约束报错。
- 幂等导入不产生重复接口定义或重复自动生成接口请求模板，并应继续刷新接口定义与模板。
- 自动生成模板名称优先使用 OpenAPI/Swagger `summary`，缺省回退 `operationId`，再回退 `METHOD PATH`，不拼接生成规则名称。
- 当前自动生成实际只产出 happy path 模板；其他生成规则属于预留扩展点。
- 导入 OpenAPI/Swagger 时必须将请求/响应字段约束保留进 `requestSchema` / `responseSchema`。
- **API Key 导入通知**：当 `POST .../specs/import` 由 API Key（`principal.Source == apikey`）调用且导入成功时，为 API Key 所属用户（`principal.UserID`，即 Key 的 `created_by`）写入一条 `spec_import` 站内通知；通知写入失败仅记 warn 日志，不影响 201 导入响应。
- **JWT 页面导入不触发通知**：管理后台 API 管理页或历史 Spec 导入组件内的 JWT 手动导入不产生站内通知。

## API 管理

- API 管理页面合并 OpenAPI/Swagger 导入与接口列表。
- 独立 Spec 导入页已并入 API 管理入口；仓库中若保留旧组件文件，不代表仍有独立路由入口。
- 页面内统一选择服务，上方承载导入和历史记录，下方展示接口请求模板列表。
- 前端路由/按钮权限使用 `cases:read`、`specs:import`、`cases:write`；当前后端 cases/spec 扁平路由未全部使用这些全局 permission 强制拦截，不能把前端权限视为完整服务端授权。

## 运行控制台

- 运行控制台参考 Apifox 工作台风格，提供左侧接口树、顶部接口 Tab、请求方法与发送工具栏、环境选择、请求参数分区、响应结果分区等核心交互。
- 支持从运行控制台选择环境并独立运行接口请求模板。
- 运行前可命名本次运行、手动修改请求参数、自动生成默认参数，并可将本次请求和响应作为参考。
- 运行界面提供「保存用例」将当前请求参数固化为测试用例快照，按接口模板 ID 隔离，Tab 展示，并可被场景编排引用。
- HTTP 请求与响应需按 URL、Params、Headers、Body、断言等部位分区编辑和展示，避免把完整请求或结果堆在单个纯文本框中；当前没有独立命名为「变量」的请求 Tab，变量通常在请求字段、环境变量或模板表达式中体现。
- Params 分区内单独划分 Path 与 Query，并展示 OpenAPI/Swagger 参数说明。
- Path 路径参数子区按 `{变量名}` 占位与模板来源判断显隐。
- Query 参数表格不使用参数类型、图标或「必填/非必填」视觉标签标识必填性，仅在必填参数的参数值后方显示红色 `*`。
- 非必填参数默认不自动填充模拟数据。
- 请求 Body 输入框与响应 Body 展示需支持 JSON 语法高亮、对象折叠和长行自动折行。
- 请求 Body 编辑器和响应 Body JSON 框右侧需展示结构、类型、必填标记、字段限制和字段含义。
- 字段限制覆盖 enum、长度、数值范围、数组数量、对象属性数量、pattern、multipleOf、nullable、readOnly/writeOnly、deprecated 等约束。
- 字段含义优先来自 OpenAPI/Swagger `description`，缺省回退 `title`。
- 运行结果的「响应」区先展示响应 Body，其下方通过子 Tab 切换「响应头」与「实际请求」（基于本次 requestSnapshot 的 curl 命令）。
- 响应状态码标签展示在运行结果区域右上角；仅底层请求失败时追加错误原因标签。
- 运行完成后默认定位到响应 tab。
- 运行控制台发送模块上方不展示接口 summary 或自动生成模板名称；接口识别通过工作台顶部 Tab 的 API `summary` 承载。
- 运行控制台展示的 API 请求耗时按响应时间着色：小于 100ms 绿色，达到或超过 100ms 黄色，未运行时中性色。

## 工作台状态持久化

- 运行控制台打开的接口 Tab 与当前激活 Tab 当前按项目在浏览器本地持久化；切换服务不会自动清空已打开 Tab。
- 请求参数草稿当前按 `caseId` 在浏览器本地持久化，并有数量上限。
- 左侧接口列表宽度当前使用全局本地存储值，不按项目或服务分桶。
- 关闭接口 Tab 视为丢弃该接口本地草稿。

## 环境认证继承

- 运行声明了 OpenAPI/Swagger `security` 的请求模板时，应基于所选环境的认证配置自动附带认证信息。
- 环境认证配置支持在同一环境内维护多套 auth profile。
- 不同 OpenAPI/Swagger `security` 可配置不同参数或 token。
- Runner 根据请求模板中的 `security` 与请求路径选择对应 profile，并兼容历史单一认证配置。
- 当请求体中未携带 `security`，且环境认证配置使用多 profile 并配置 `defaultProfile` 时，Runner 使用 `defaultProfile` 兜底。
- legacy 单一认证配置在请求未声明 `security` 时仍保持不附带认证。
- 场景编排与运行控制台选择环境后，应基于当前环境认证配置、请求 `security` 与路径规则动态展示将由环境继承的认证参数。
- 继承行未修改时不写入请求覆盖；用户修改后作为覆盖保存并参与发送。
- 认证覆盖值可使用普通变量、`$steps`、`$mock` 等模板变量。
- 前端运行控制台与场景编辑区共享 `web/admin/src/utils/authInheritance.js` 解析和展示继承认证；后端 Runner 另有 Go 实现负责真实发送前认证注入，两侧语义需要保持一致。

## 运行控制台环境编辑

- 运行控制台支持在运行前编辑当前选择的环境配置。
- 环境编辑入口位于环境下拉列表每一行环境名称右侧，鼠标悬停该条环境项时显示。
