# MCP 测试自动化

本文档说明如何通过 [Model Context Protocol](https://modelcontextprotocol.io)（MCP）在 Cursor、Claude Desktop 等客户端中完成 **Swagger 导入、用例/场景编排、执行与结果查询**。仅 Swagger 导入的简要说明见 [mcp-swagger-import.md](mcp-swagger-import.md)。

## 能力范围

- **方案 A（推荐）**：`cmd/api` 在 `MCP_HTTP_ENABLED=true` 时挂载 **Streamable HTTP** 端点（默认路径 `/mcp`），与 API 同进程启动/停止。
- **方案 B**：独立进程 `cmd/mcp`，经 **stdio** 与 Cursor 等客户端通信。
- 认证：`Authorization: Bearer at-...`（API Key），按 Key 上配置的 **scope** 与所属用户的项目成员权限执行。
- 与 HTTP API 白名单一致：未开放的接口对 API Key 返回 403。

### 工具一览

| 分类 | 工具 | 所需 scope |
|------|------|------------|
| Spec | `import_swagger`, `import_swagger_from_url` | `specs:import` |
| Meta | `list_services`, `list_environments`, `list_endpoints` | `cases:read`（端点亦可用 `specs:import`） |
| Case | `list_cases`, `get_case`, `create_case`, `patch_case` | `cases:read` / `cases:write` |
| Scenario | `list_scenarios`, `get_scenario`, `create_scenario_with_steps`, `update_scenario`, `delete_scenario`, `upsert_scenario_step`, `delete_scenario_step`, `reorder_scenario_steps` | `scenarios:read` / `scenarios:write` |
| Run | `list_runs`, `get_run_result`, `run_case`, `run_scenario` | `runs:read` / `runs:execute` |
| 参考 | `describe_template_syntax` | 无 HTTP（本地 Markdown） |

步骤类型支持：`api`、`database`、`script`、`for`、`condition`。控制流子步骤在 `create_scenario_with_steps` / `upsert_scenario_step` 中可用 **stepOrder** 引用，平台会改写为内部 `step_seq`。

## 环境变量

### 方案 A：API 内嵌 HTTP MCP

| 变量 | 必填 | 说明 |
|------|------|------|
| `MCP_HTTP_ENABLED` | 是 | `true` / `1` / `on` 时随 `make run-api` 暴露 MCP |
| `MCP_HTTP_PATH` | 否 | 默认 `/mcp` |
| `MCP_HTTP_API_BASE_URL` | 否 | 工具回环调 REST 的 base；默认按 `ADDR` 推导为 `http://127.0.0.1:<port>/api/v1` |
| `AUTOTEST_PROJECT_ID` | 否 | 默认项目 UUID；工具参数可覆盖 |
| `AUTOTEST_SERVICE_ID` | 否 | 默认服务 UUID |
| `AUTOTEST_ENVIRONMENT_ID` | 否 | `run_case` / `run_scenario` 的默认环境 UUID |

客户端（Cursor 等）在 **每个 MCP 请求** 的 `Authorization` 头携带 `Bearer at-...`；无需在服务端配置 Key 明文。

### 方案 B：独立 stdio（`cmd/mcp`）

| 变量 | 必填 | 说明 |
|------|------|------|
| `AUTOTEST_API_KEY` | 是 | API Key 明文（`at-` 前缀） |
| `AUTOTEST_API_BASE_URL` | 否 | 默认 `http://localhost:8080/api/v1` |
| `AUTOTEST_PROJECT_ID` | 否 | 默认项目 UUID；工具参数可覆盖 |
| `AUTOTEST_SERVICE_ID` | 否 | 默认服务 UUID |
| `AUTOTEST_ENVIRONMENT_ID` | 否 | `run_case` / `run_scenario` 的默认环境 UUID |

在「系统管理 → API Key」创建 Key 时勾选所需 scope；作用域创建后不可修改，需新建 Key。

### 服务级开关（管理后台）

在 **项目 → 服务与环境** 编辑服务时，可开启 **MCP 自动化**。开启后页面展示：

- 服务端是否已启用 `MCP_HTTP_ENABLED`
- 建议 API Key scope 列表
- 默认运行环境选择
- 可复制到 Cursor 的 HTTP / stdio 配置片段（含本服务的 `projectId` / `serviceId` / `environmentId`）
- **一键安装到 Cursor**：使用官方 deeplink `cursor://anysphere.cursor-deeplink/mcp/install?name=...&config=...`（见 [Cursor MCP Install Links](https://cursor.com/docs/mcp/install-links)）；配置内已嵌入 API Key
- **自动 API Key**：首次打开接入说明时（`ensureApiKey=1`，默认）由当前用户自动创建名为 `MCP · {服务名}` 的 Key（`apikey.MCPAutomationScopes()`），绑定 `services.mcp_api_key_id`；明文仅在创建/轮换的当次响应 `apiKeyToken` 中返回

字段 `services.mcp_enabled`、`services.mcp_api_key_id` 持久化；接入说明：`GET .../mcp-integration?ensureApiKey=1&regenerate=0`（生成 Key 需项目 **developer+**），响应含 `cursorInstallHttpLink` / `cursorInstallStdioLink` / `apiKeyToken` / `apiKeyMask`。

## 推荐工作流

1. `import_swagger` 或 `import_swagger_from_url` 导入 OpenAPI。
2. `list_endpoints` / `list_cases` 查找端点与请求模板。
3. 无现成模板时用 `create_case`；再 `create_scenario_with_steps` 组装场景（或分步 `upsert_scenario_step`）。
4. `list_environments` 确认环境后 `run_scenario` 或 `run_case`。
5. `list_runs` / `get_run_result` 查看结果。

## 本地运行

### 方案 A（与 API 同启）

```bash
# .env
MCP_HTTP_ENABLED=true
AUTOTEST_PROJECT_ID=<uuid>
AUTOTEST_SERVICE_ID=<uuid>
AUTOTEST_ENVIRONMENT_ID=<uuid>
make run-api
# MCP 端点：http://localhost:8080/mcp（路径以 MCP_HTTP_PATH 为准）
```

Cursor 可配置 URL 型 MCP（见 [.cursor/mcp.json.example](../../.cursor/mcp.json.example) 中 `autotest-http` 示例），请求头带 API Key。

### 方案 B（stdio）

```bash
make run-api    # 另开终端
export AUTOTEST_API_KEY=at-your-key
export AUTOTEST_PROJECT_ID=<uuid>
make run-mcp
```

或 `go run ./cmd/mcp`。推荐 `make build-mcp` 后用二进制。

## HTTP 白名单（API Key）

路由按 scope 分组挂载（JWT 不受影响）：

- `specs:import`：仅 `POST .../specs/import`
- `cases:read` 或 `specs:import`：用例 GET、spec 端点列表、项目服务/环境 GET
- `cases:write`：`POST/PATCH /cases`
- `scenarios:read` / `scenarios:write`：场景与步骤 CRUD
- `runs:read` / `runs:execute`：运行列表、结果、执行

其余 `/api/v1` 接口仍 `RejectAPIKey`。

## 共享逻辑

场景步骤构建与 `stepOrder → step_seq` 重写由 `internal/scenariobuild` 实现，AI 内置工具与 MCP HTTP 客户端共用。
