# 管理后台与访问控制设计

本文档记录管理后台布局、全局项目上下文、菜单、服务环境管理、RBAC、API Key 与品牌界面的业务设计。

## 技术与登录

- 管理后台使用 Vue 3 和 Element Plus 构建。
- 管理后台提供登录流程、主应用布局、路由守卫、接口自动化测试平台页面、用户与权限管理、RBAC 风格访问控制。
- 支持两种登录方式并存：
  - **本地用户名密码**：`POST /auth/login`；`active=false` 的本地用户与错误密码一样返回 401，避免账号状态枚举。本地密码登录始终可用，不可在后台关闭（bootstrap 能力）。
  - **OAuth（GitHub / GitLab / Google）**：登录页调用 `GET /auth/login-providers` 获取已启用的 IdP 列表（**仅返回 `callbackUrl` 域名与当前 API 请求域名一致的项**，便于多环境共用库时按后端实例过滤）；点击跳转 `GET /auth/oauth/{slug}/login?frontendUrl={window.location.origin}`；后端按**当前登录方式**配置的 `trusted_frontend_origins` 校验 `frontendUrl`，写入 signed OAuth state；回调 `GET /auth/oauth/{slug}/callback` 成功后 302 到 `{frontendUrl}/login/oauth/callback#token=...`（hash 承载 JWT，避免 token 进入服务端 access log）。
- OAuth 登录方式在「**用户管理 → 登录方式**」Tab 配置（权限 `users:manage`）：支持 CRUD、`slug`（公开标识，唯一）、`clientSecret` 掩码、`callbackUrl`（保存时由前端根据 `VITE_API_BASE_URL` + slug 生成并持久化，列表只读展示）、**可信前端域名**（`trustedFrontendOrigins`，完整 scheme+host+port，无路径；**仅**用于该登录方式的 OAuth `frontendUrl` 回跳校验，与 CORS 无关；`development` 额外内置 localhost 默认项）、测试连接（构造授权 URL）。`client_secret` 第一版明文存库，API 响应掩码（同 AI Provider）。
- 内置适配器：`github`、`gitlab`（extraConfig.baseUrl，默认 `https://gitlab.com`）、`google`。同类型可配置多条；**slug** 为 OAuth 路由与 IdP 回调 URL 的稳定公开标识（如 `github`、`company-github`），UUID 仅作内部主键。多开发环境可共用同一 GitHub OAuth App：各环境配置 slug=`github` 时回调路径一致（如 `http://localhost:8080/api/v1/auth/oauth/github/callback`），便于在 IdP 只登记一次。
- 外部身份绑定表 `user_external_identities`（`provider_type` + `external_id` 唯一）；保留 `users.github_id` 双读兼容。用户名前缀：`gh_` / `gl_` / `gg_`。
- 任意 OAuth 账号可发起登录；首次成功回调且 `auto_register=true` 时自动建档（`default_active` 默认 `false`、无 RBAC 角色）。已绑定用户仅更新 displayName/email/头像，不自动改 `active`。`auto_register=false` 时未绑定用户回调失败，跳转 `/login?error=registration_disabled`。
- 待审核用户（`active=false`）仍可拿到 JWT，但除 `GET /auth/me`、`POST /auth/logout` 外其余 API 返回 **403**（`account pending approval`）；前端路由仅允许 `/pending-approval` 与 OAuth 回调页，403 响应不清 token、跳转待审核页。
- 管理员在「用户管理」启用 OAuth 用户并分配角色后，用户刷新或重新登录进入正常流程。
- **环境变量**：OAuth 回调 URL 不再由后端环境变量计算，而是在管理端保存登录方式时根据 `VITE_API_BASE_URL` + slug 生成并写入 `auth_providers.callback_url`。OAuth 成功回跳前端地址来自登录请求 query `frontendUrl`（signed state 传递），不再依赖 `ADMIN_FRONTEND_URL`。
- **旧路由兼容**：`GET /auth/github/status`（DB 有 enabled github）、`GET /auth/github/login`（第一个 enabled github DB 配置）、`GET /auth/github/callback`（同上，回调 URL 为 DB 中持久化的 `callback_url`）。OAuth 失败时回调 302 到 `/login?error=<code>`（如 `invalid_state`、`exchange_failed`、`registration_disabled`）。
- 管理 API（`users:manage`）：`GET /auth-provider-types`、`GET/POST/PUT/DELETE /auth-providers`、`POST /auth-providers/{id}/test`。
- 本地开发：根目录 `.env` 在 `APP_ENV=production` 时不由 API 加载；其他环境启动时 `internal/config.Load()` 会自动 `godotenv.Load()`。详见 [架构与运维 — 配置与环境](architecture.md#配置与环境)。
- 前端路由守卫按用户 permissions 数组过滤路由；无权限访问时当前跳转到 `/dashboard`，没有独立 403 页面。
- 服务启动时用代码内固定 bootstrap 账号 `admin` / `admin` 与库中默认管理员对齐；**仅当该用户尚未完成首次改密**（`must_change_password=true`）时才覆盖密码哈希。
- 默认管理员首次本地密码登录成功后须修改密码；改密成功后清除 `must_change_password`，客户端退出并要求用新密码重新登录。改密完成前除 `GET /auth/me`、`POST /auth/logout`、`POST /auth/change-password` 外其余 API 返回 **403**（`must change password`）；前端仅允许 `/change-password` 页面。

## 全局项目上下文

- 项目选择属于全局上下文，在主应用顶部统一选择。
- 服务与环境、场景编排、API 管理与运行控制台等页面复用当前全局项目。
- 浏览器默认记住上次选择的全局项目和运行环境，并在下次打开时优先恢复仍然有效的选择。

## 站内通知

- 管理后台顶栏「项目」选择器与「外观」按钮之间提供通知铃铛入口，展示未读角标与下拉列表。
- 通知按登录用户隔离：仅 JWT 用户可调用 `GET /notifications`、`GET /notifications/stream`、`PATCH /notifications/{id}/read`、`POST /notifications/read-all`、`POST /notifications/clear-all`；API Key 来源不可访问通知接口（`RejectAPIKey`）。
- 不新增 RBAC 权限点；后端按 `principal.UserID` 过滤读写。
- 实时推送：`GET /notifications/stream` 返回 `text/event-stream`，连接建立时先发 `snapshot`（`items` + `unreadCount`），新通知写入后推送 `notification`（单条 + `unreadCount`），`POST /notifications/clear-all` 成功后推送空 `snapshot`（`items: []`、`unreadCount: 0`）；15s 注释心跳保活；与 AI 助理 SSE 一样绕过全局 HTTP 超时。
- 前端登录后订阅 SSE，断线指数退避重连；标签页重新可见时仅 REST 拉取一次作同步，打开下拉时立即刷新列表；下拉提供「全部标为已读」与「一键清空」（调用 `clear-all`，本地立即清空列表与角标）。
- `spec_import` 通知正文展示**本次实际变动**接口数（新增 + 更新，且 `SyncEndpoints` 仅在 fingerprint 变化或恢复软删时计为更新），不将 OpenAPI 文档总路径数当作「更新数量」；`payload` 含 `changedEndpoints`、`createdEndpoints`、`updatedEndpoints` 等字段。
- `user_pending` 通知在 OAuth 首次登录创建待审核用户后写入，收件人为拥有 `users:manage` 权限的已启用用户；正文含 displayName / username / email，点击跳转 `/users`。
- 点击单条通知：标为已读、若 `payload.projectId` 与当前全局项目不同则切换项目，并跳转到 `/cases`（API 管理）；`user_pending` 类型跳转 `/users`。
- 表结构 `type` 预留扩展其他事件类型。

## 服务与环境管理

- 服务与环境管理页基于当前全局项目展示服务与环境配置。
- 服务与环境管理支持新增、编辑、删除。
- 未选择全局项目时新增服务不可操作并提示。
- 服务与环境以树形展示，环境作为服务子级节点。
- 当前项目下上次选择的服务本地持久化。
- 环境配置弹窗中的变量 JSON 与认证 JSON 提供提示图标、格式化示例、自适应高度，并在失焦后自动格式化。
- 业务数据源、AI 能力管理（含原 AI 提供商、AI 助理配置、Prompt 管理）已归入「平台资源」分组；历史 `/data-sources` 重定向到 `/platform/data-sources` 等；`/platform/ai-providers`、`/ai-providers`、`/platform/ai-assistant-settings`、`/platform/ai-prompts` 重定向到 `/platform/ai-management` 对应 Tab。

## 菜单与布局

- **用户管理**页（`/users`）以 Tab 聚合：用户、角色、权限、登录方式；历史路径 `/roles`、`/permissions`、`/auth-providers` 重定向到 `/users?tab=...`。
- 侧边栏「项目管理」仅维护服务与环境（主从布局：左侧项目列表 + 右侧服务环境树）。
- 「平台资源」分组包含：脚本库、业务数据源、AI 能力管理、命名值集合、模板参考。
- 历史路径 `/services` 重定向到项目管理；`/data-sources` 重定向到平台资源对应页。
- 左侧菜单包含项目管理、测试数据、平台资源、API 管理、运行控制台、场景编排、系统管理等。
- 「模板参考」当前无单独路由 permission，登录用户可见。
- 侧边栏支持手动收起。
- 主应用左侧导航菜单区域不允许单独滚动。
- 主布局限制在视口高度内，主内容过长时仅右侧主内容区滚动。

## 服务端授权现状

- 全局 RBAC 权限当前主要在用户、角色、权限管理和 API Key 管理等系统接口上由后端中间件强制。
- Mock Server、Mock Value Sets、测试数据表等项目路径已按 viewer/developer 项目角色控制；业务数据源、AI 提供商、平台 Prompt 管理 API 使用全局 RBAC（`projects:read` / `projects:write`）。
- cases、scenarios、runner、部分 spec 只读/导入和 SQL 参数源等接口当前未全部以同一套项目角色或 `cases:*`/`specs:import` 后端中间件强制；涉及权限加固时应先确认是否会改变现有可访问行为。
- 前端路由权限与后端服务端授权不是同一层能力，文档或实现变更时需要分别说明。

## 视觉与品牌

- Web 界面时间展示统一使用 `YYYY-MM-DD HH:mm:ss` 格式。
- 管理后台支持整体字体大小调整和多套配色方案，用户选择在本地持久化。
- 侧边栏左下角展示当前管理员头像与姓名。
- 退出登录仅通过点击头像后的下拉菜单选择。
- 管理后台应提供符合接口自动化测试平台定位的品牌 logo，并在登录页、主应用侧边栏和浏览器标签页中展示。

## 脚本库

- 脚本断言、场景 API 步骤断言与脚本步骤编辑区提供「脚本库」。
- 脚本库支持内置模板和当前全局项目自定义模板。

## API Key

- 支持 API Key 认证用于 CI/CD 调用。
- 用户在「系统管理 → API Key」管理本人令牌。
- API Key 列表、增删改、重置仅作用于当前登录用户创建的 Key。
- 明文仅展示一次，库内仅存 SHA-256 哈希与前后缀掩码。
- 令牌通过 `Authorization: Bearer at-...` 调用平台 API。
- 校验通过后以该 Key 所属用户权限快照执行请求。
- 当前阶段仅允许 API Key 调用 OpenAPI/Swagger 导入接口（scope=`specs:import`）。
- 其余接口对 API Key 来源拒绝调用，当前错误语义为「当前接口不支持 API Key 调用」。
- 支持禁用、过期、最近使用时间审计和 rotate。

## 审计日志

- 平台持久化审计事件至 `audit_logs` 表；当前阶段记录所有用户**登录成功与失败**（action=`auth.login`），涵盖本地密码（`resource=local`）与 OAuth 回调（`resource=oauth:{slug}`）。
- 失败事件在 `detail.reason` 中记录内部原因码（如 `invalid_password`、`account_inactive`、`exchange_failed`），本地密码登录失败时另在 `detail.attemptedPassword` 记录用户提交的明文密码；仅供管理员排查；对外 HTTP 响应仍保持既有语义（本地密码统一 401，OAuth 失败跳转 `/login?error=...`）。
- 每条记录包含：操作者用户名、可选 `actor_user_id`、客户端 IP、User-Agent、成功与否、方式（resource）与时间戳。
- 查询 API：`GET /audit-logs`（JWT、RejectAPIKey），需全局 RBAC 权限 `audit:read`；支持按 `action`、`success`、`username` 过滤与分页（`limit`/`offset`）。
- 管理后台「系统管理 → 审计日志」（`/audit-logs`）展示登录审计列表；默认 admin 角色 bootstrap 时自动获得 `audit:read`。
- 后续其他 mutating 操作可复用同一表结构与 `internal/auditlog` 包扩展 action 类型。
- 重置后原 token 立即失效，新明文仅在独立一次性弹窗展示。
