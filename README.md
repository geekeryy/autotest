# Autotest MVP

Go + PostgreSQL 后端和 Vue 3 + Element UI 管理后台组成的接口自动化测试平台 MVP。

## 本地运行

```bash
make init
make run-api
```

默认连接：

```text
postgres://autotest:autotest@localhost:5432/autotest?sslmode=disable
```

默认后台管理员：

```text
用户名：admin
密码：admin123
```

可通过 `ADMIN_USERNAME`、`ADMIN_PASSWORD` 和 `JWT_SECRET` 覆盖本地默认值。应用启动时会确保默认管理员、管理员角色和基础权限点存在。

根目录 `Makefile` 会自动读取 `.env`（如果存在），并在 `make init` 中准备 PostgreSQL、等待数据库就绪、按顺序执行 `migrations/*.sql`。默认使用外部 PostgreSQL，不依赖 Docker Compose。常用环境变量：

```text
DB_MANAGED=external
POSTGRES_DB=autotest
POSTGRES_USER=autotest
POSTGRES_PASSWORD=autotest
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
DATABASE_URL=postgres://autotest:autotest@localhost:5432/autotest?sslmode=disable
ADDR=:8080
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123
JWT_SECRET=autotest-dev-secret-change-me
```

`DB_MANAGED` 默认值为 `external`，表示使用 `.env` 中的 `POSTGRES_HOST` / `POSTGRES_PORT` 指向的已有 PostgreSQL，`make init` 不会启动 Docker Compose。需要使用项目内 `docker-compose.yml` 托管 PostgreSQL 时，可设置 `DB_MANAGED=docker` 后再执行 `make init`。

## 管理后台

```bash
cd web/admin
npm install
npm run dev
```

默认开发地址为 `http://localhost:5173`，开发服务器会把 `/api` 代理到 `http://localhost:8080`。生产构建：

```bash
cd web/admin
npm run build
```

核心接口：

- `GET /healthz`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/me`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{projectID}/services`
- `POST /api/v1/projects/{projectID}/services`
- `GET /api/v1/projects/{projectID}/environments`
- `POST /api/v1/projects/{projectID}/environments`
- `POST /api/v1/projects/{projectID}/services/{serviceID}/specs/import`
- `GET /api/v1/projects/{projectID}/services/{serviceID}/specs`
- `GET /api/v1/projects/{projectID}/services/{serviceID}/endpoints`
- `GET /api/v1/cases`
- `POST /api/v1/cases`
- `GET /api/v1/suites`
- `POST /api/v1/suites`
- `GET /api/v1/suites/{suiteID}/items`
- `POST /api/v1/suites/{suiteID}/items`
- `GET/POST/PUT/DELETE /api/v1/users`
- `GET/POST/PUT/DELETE /api/v1/roles`
- `GET/POST /api/v1/permissions`

除 `GET /healthz` 和 `POST /api/v1/auth/login` 外，`/api/v1` 下管理接口都需要 Bearer Token。Migration 文件位于 `migrations/`，可用 goose 或 psql 执行。

## TODO
- 支持本地 ollama 生成接口请求参数（待实现）

