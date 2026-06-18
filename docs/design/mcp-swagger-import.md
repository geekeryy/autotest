# MCP Swagger 导入

> **完整 MCP 能力**（用例/场景/运行/模板说明）见 **[mcp-automation.md](mcp-automation.md)**。本文保留 Swagger 导入专项说明。

本文档说明如何通过 [Model Context Protocol](https://modelcontextprotocol.io)（MCP）将 OpenAPI/Swagger 文档导入 autotest，供 Cursor、Claude Desktop 等 AI 客户端调用。

## 能力范围

- MCP 服务进程：`cmd/mcp`，通过 **stdio** 与客户端通信。
- 工具：
  - `import_swagger`：从本地 `filePath` 或内联 `content` 导入。
  - `import_swagger_from_url`：从 HTTP(S) URL 下载后导入。
- 底层调用既有 `POST /api/v1/projects/{projectId}/services/{serviceId}/specs/import`，需使用带 **`specs:import`** 作用域的 API Key（`Authorization: Bearer at-...`）。
- 导入语义、幂等与统计字段与 [api-management-and-runner.md](api-management-and-runner.md) 中 OpenAPI 导入一致；API Key 成功导入会触发 `spec_import` 站内通知。
- 工具可选参数 `syncMode`：`merge`（默认）或 `overwrite`，对应 HTTP query `?sync=merge|overwrite`。

## 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `AUTOTEST_API_KEY` | 是 | API Key 明文（`at-` 前缀），在管理后台「系统管理 → API Key」创建并勾选 `specs:import` |
| `AUTOTEST_API_BASE_URL` | 否 | API 根路径，默认 `http://localhost:8080/api/v1` |
| `AUTOTEST_PROJECT_ID` | 否 | 默认项目 UUID；工具参数可覆盖 |
| `AUTOTEST_SERVICE_ID` | 否 | 默认服务 UUID；工具参数可覆盖 |

## 本地运行

```bash
# 先启动 API 并完成迁移
make run-api

# 另开终端，配置环境变量后运行 MCP 服务（stdio，由客户端拉起）
export AUTOTEST_API_KEY=at-your-key
export AUTOTEST_PROJECT_ID=<project-uuid>
export AUTOTEST_SERVICE_ID=<service-uuid>
go run ./cmd/mcp
```

或使用 Makefile：`make run-mcp`（需已在 `.env` 或环境中设置上述变量）。

## Cursor 配置示例

先构建 MCP 二进制（推荐，不依赖 `go run` 与工作目录）：

```bash
make build-mcp
```

在项目或用户 MCP 配置中加入：

```json
{
  "mcpServers": {
    "autotest": {
      "type": "stdio",
      "command": "/path/to/autotest/bin/autotest-mcp",
      "env": {
        "AUTOTEST_API_BASE_URL": "http://localhost:8080/api/v1",
        "AUTOTEST_API_KEY": "at-xxxxxxxx",
        "AUTOTEST_PROJECT_ID": "00000000-0000-0000-0000-000000000001",
        "AUTOTEST_SERVICE_ID": "00000000-0000-0000-0000-000000000002"
      }
    }
  }
}
```

项目级配置（`.cursor/mcp.json`）可用 `${workspaceFolder}/bin/autotest-mcp` 代替绝对路径。

**注意**：Cursor 当前 MCP 配置不支持可靠的 `cwd` 字段；勿使用 `go run ./cmd/mcp` 这类依赖工作目录的启动方式。

## 工具参数摘要

### import_swagger

| 参数 | 说明 |
|------|------|
| `projectId` | 可选，缺省用 `AUTOTEST_PROJECT_ID` |
| `serviceId` | 可选，缺省用 `AUTOTEST_SERVICE_ID` |
| `filePath` | 本地 spec 文件路径（与 `content` 二选一） |
| `content` | 内联 YAML/JSON 文本 |
| `contentType` | 可选，`application/json` 或 `application/yaml` |

### import_swagger_from_url

| 参数 | 说明 |
|------|------|
| `url` | 必填，spec 文档 URL |
| 其余 | 同 `import_swagger` |

成功时返回 JSON 格式的 `ImportSummary`（`specId`、`apiCount`、`createdEndpoints` 等）。
