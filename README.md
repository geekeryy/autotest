# Autotest

API 自动化测试平台，支持接口请求模板管理、场景编排、Mock Server 与 AI 辅助生成。

## Features

- **接口管理** -- 导入 OpenAPI Spec，自动解析接口定义并生成请求模板
- **场景编排** -- 拖拽编排多步骤测试流程，支持脚本、断言与参数化
- **Mock Server** -- 按项目配置 Mock 规则，拦截并模拟接口响应
- **AI 辅助** -- 接入 DeepSeek / OpenAI / Anthropic / Ollama 等大模型，一键生成请求参数、断言脚本与测试数据
- **RBAC 权限** -- 基于角色的访问控制，细粒度管理项目与资源
- **运行控制台** -- 实时查看执行状态、响应详情与断言结果
- **CI/CD 集成** -- 支持 API Key 与 API 调用，可用于自动化测试与持续集成

## Quick Start

### Prerequisites

- Go >= 1.25
- Node.js >= 18
- PostgreSQL >= 14 (本地已安装，或使用 Docker Compose)

### 1. 本地开发

```bash
make init   # 等待数据库就绪 + 执行迁移
make run-api
make web-dev
```

默认连接 `postgres://autotest:autotest@localhost:5432/autotest`，启动后自动创建管理员账号：

访问 `http://localhost:5173`，账号 `admin`，密码 `admin`，
开发服务器自动将 `/api` 代理到后端 `http://localhost:8080`。
首次登录须修改密码，改密成功后使用新密码重新登录。

### 2. All-in-One

单镜像快速试用（内嵌前端）：

```bash
cp deploy/all-in-one/.env.example deploy/all-in-one/.env
make all-in-one-up
```

访问 `http://localhost:8080`。详见 [architecture.md — All-in-One（可选）](docs/design/architecture.md#all-in-one可选)。

## Configuration

配置由 `internal/config` 在启动时统一加载。根目录创建 `.env`（参考 `.env.example`），`Makefile` 会自动 `include` 并 export。

| 环境      | APP_ENV               | 说明                                               |
| --------- | --------------------- | -------------------------------------------------- |
| 本地开发  | `development`（默认） | 自动读 `.env`，开启 dev CORS                       |
| 测试 / CI | `test`                | `make test` 已内置；集成测试用 `make test-integration`（见 [architecture.md](docs/design/architecture.md#3-测试--ci)） |
| 生产      | `production`          | API 不读 `.env`；启动时校验 JWT 与数据库密码强度 |

完整变量清单、加载顺序、Docker Compose 与部署方式见 [docs/design/architecture.md](docs/design/architecture.md)。需求与文档索引见 [docs/requirements.md](docs/requirements.md)。

## MCP（测试自动化）

可通过 MCP 完成 Swagger 导入、用例/场景编排、执行与结果查询（供 Cursor 等客户端调用）。在管理后台创建 API Key 并勾选所需 scope（如 `specs:import`、`cases:read`、`scenarios:write`、`runs:execute`）。

**与 API 同进程（推荐）**：在 `.env` 中设置 `MCP_HTTP_ENABLED=true` 后 `make run-api`，端点默认为 `http://localhost:8080/mcp`；客户端在请求头携带 `Authorization: Bearer at-...`。

```bash
# .env
MCP_HTTP_ENABLED=true
AUTOTEST_PROJECT_ID=<project-uuid>
AUTOTEST_SERVICE_ID=<service-uuid>
AUTOTEST_ENVIRONMENT_ID=<environment-uuid>
make run-api
```

**独立 stdio 进程**（可选）：

```bash
export AUTOTEST_API_KEY=at-your-key
export AUTOTEST_PROJECT_ID=<project-uuid>
make run-mcp
```

配置示例见 [.cursor/mcp.json.example](.cursor/mcp.json.example)，设计说明见 [docs/design/mcp-automation.md](docs/design/mcp-automation.md)。

## TODO

- 基于 OpenAPI Spec 自动生成完整测试场景
- 自动生成正向、反向用例
- 集成通知（飞书/钉钉/Slack webhook）
- 变量引用与函数计算增强
- API 文档页面
- CI/CD 集成与自动化触发
- 项目级测试大盘与 CI 触发（场景报告与运行历史已支持，见场景编排）
- 断言能力增强
- gRPC / WebSocket / TCP 协议支持
- 性能测试能力
- 定时任务

## Contributing

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/my-feature`)
3. 提交变更 (`git commit -m 'feat: add my feature'`)
4. 推送到远程 (`git push origin feature/my-feature`)
5. 创建 Pull Request

## License

MIT
