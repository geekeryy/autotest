# Autotest

API 自动化测试平台，支持接口请求模板管理、场景编排、Mock Server 与 AI 辅助生成。

## Features

- **接口管理** -- 导入 OpenAPI Spec，自动解析接口定义并生成请求模板
- **场景编排** -- 拖拽编排多步骤测试流程，支持脚本、断言与参数化
- **Mock Server** -- 按项目配置 Mock 规则，拦截并模拟接口响应
- **AI 辅助** -- 接入 DeepSeek / OpenAI / Anthropic / Ollama 等大模型，一键生成请求参数、断言脚本与测试数据
- **RBAC 权限** -- 基于角色的访问控制，细粒度管理项目与资源
- **运行控制台** -- 实时查看执行状态、响应详情与断言结果

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Frontend (Vue 3)                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │
│  │ Projects │ │Requests  │ │Scenarios │ │  Spec Import     │ │
│  │ Services │ │Templates │ │  Runner  │ │  Mock / RBAC     │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘ │
└──────────────────────────┬───────────────────────────────────┘
                           │ /api/v1/*
┌──────────────────────────┴───────────────────────────────────┐
│                       Backend (Go)                           │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                    HTTP Layer (Gin)                     │ │
│  │   auth middleware · JWT · RBAC permission check         │ │
│  └─────────────────────────┬───────────────────────────────┘ │
│                            │                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │
│  │   spec   │ │ request  │ │ scenario │ │   mockserver     │ │
│  │ importer │ │ template │ │  runner  │ │  matcher/runtime │ │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────────────┘ │
│       │            │            │             │              │
│  ┌────┴────────────┴────────────┴─────────────┴────────────┐ │
│  │              Shared Services & Utils                    │ │
│  │  httpx · assertion · sampler · generator · paramsource  │ │
│  │  aiprovider · scriptlibrary · projectprompt · report    │ │
│  └──────────────────────────┬──────────────────────────────┘ │
│                             │                                │
│  ┌──────────────────────────┴──────────────────────────────┐ │
│  │                    Store (PostgreSQL)                   │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go >= 1.23
- Node.js >= 18
- PostgreSQL >= 14 (本地已安装，或使用 Docker Compose)

### 1. 启动后端

```bash
make init   # 等待数据库就绪 + 执行迁移
make run-api
```

默认连接 `postgres://autotest:autotest@localhost:5432/autotest`，启动后自动创建管理员账号：

| 项目   | 默认值     |
| ------ | ---------- |
| 用户名 | `admin`    |
| 密码   | `admin123` |

### 2. 启动前端

```bash
cd web/admin
npm install
npm run dev
```

访问 `http://localhost:5173`，开发服务器自动将 `/api` 代理到后端 `http://localhost:8080`。

### 3. 生产构建

```bash
cd web/admin
npm run build
```

## Configuration

根目录创建 `.env` 文件覆盖默认值，`Makefile` 会自动加载：

```env
# PostgreSQL
DB_MANAGED=external          # external | docker
POSTGRES_DB=autotest
POSTGRES_USER=autotest
POSTGRES_PASSWORD=autotest
POSTGRES_HOST=localhost
POSTGRES_PORT=5432

# Application
ADDR=:8080
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123
JWT_SECRET=autotest-dev-secret-change-me
```

> `DB_MANAGED=external` 使用外部 PostgreSQL；设为 `docker` 则通过 `docker compose up -d postgres` 启动容器。

## API Key (CI/CD)

平台支持通过 API Key 让 CI/CD 流水线以非交互方式调用后端开放接口。

- 管理后台「系统管理 → API Key」生成令牌，明文仅在创建时弹窗一次性展示；数据库只保存 SHA-256 哈希与前后缀掩码（形如 `at-7vqj2…2yag`）。
- 令牌带 `at-` 前缀，可禁用、可设过期时间、自动记录最近使用时间用于审计。
- 当前阶段 API Key 仅获得 `specs:import` 作用域，**只允许调用 OpenAPI/Swagger 导入接口**。调用其他接口会被路由层 `RejectAPIKey` 守卫拦截并返回 403。
- 仅拥有 `apikeys:manage` 权限的角色（默认 `admin`）可创建、编辑、删除 API Key。

导入 OpenAPI/Swagger 示例：

```bash
curl -X POST "http://localhost:8080/api/v1/projects/$PROJ/services/$SVC/specs/import" \
  -H "Authorization: Bearer at-7vqj2k9xm3t8p4nrh5w6cs8d1bf2yzag" \
  -H "Content-Type: application/yaml" \
  --data-binary @openapi.yaml
```

## TODO
- 基于 OpenAPI Spec 自动生成完整测试场景
- 现在不做 MCP，但保留 MCP 友好的内部结构
- 自动生成正向、反向用例
- 集成通知（飞书/钉钉/Slack webhook）
- 认证方式扩展（OAuth2 等）
- 变量引用与函数计算增强
- 审计日志
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
