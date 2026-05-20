# 管理后台与访问控制设计

本文档记录管理后台布局、全局项目上下文、菜单、服务环境管理、RBAC、API Key 与品牌界面的业务设计。

## 技术与登录

- 管理后台使用 Vue 3 和 Element Plus 构建。
- 管理后台提供登录流程、主应用布局、路由守卫、接口自动化测试平台页面、用户与权限管理、RBAC 风格访问控制。
- 前端路由守卫按用户 permissions 数组过滤路由；无权限访问时当前跳转到 `/dashboard`，没有独立 403 页面。
- 服务启动时按 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 与库中默认管理员账户对齐密码。
- `ADMIN_USERNAME` / `ADMIN_PASSWORD` 未设置时默认 `admin` / `admin123`。
- 生产环境应通过环境变量设置强密码。

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
- 点击单条通知：标为已读、若 `payload.projectId` 与当前全局项目不同则切换项目，并跳转到 `/cases`（API 管理）。
- 表结构 `type` 预留扩展其他事件类型。

## 服务与环境管理

- 服务与环境管理页基于当前全局项目展示服务与环境配置。
- 服务与环境管理支持新增、编辑、删除。
- 未选择全局项目时新增服务不可操作并提示。
- 服务与环境以树形展示，环境作为服务子级节点。
- 当前项目下上次选择的服务本地持久化。
- 环境配置弹窗中的变量 JSON 与认证 JSON 提供提示图标、格式化示例、自适应高度，并在失焦后自动格式化。
- 项目管理页当前还承载业务数据源、AI 提供商和 Prompt 管理标签；历史 `/ai-providers` 路径重定向到项目管理页对应标签。

## 菜单与布局

- 用户管理、角色管理、权限菜单归入独立二级菜单分组。
- 侧边栏保留「项目管理」进入主从布局页面，右侧标签区分「服务与环境」「业务数据源」「AI 提供商」「Prompt 管理」等。
- 历史路径 `/services`、`/data-sources`、`/ai-providers` 重定向到项目管理页对应标签。
- 左侧菜单包含项目管理、测试数据、平台资源、API 管理、运行控制台、场景编排、系统管理等。
- 「模板与变量参考」当前无单独路由 permission，登录用户可见。
- 侧边栏支持手动收起。
- 主应用左侧导航菜单区域不允许单独滚动。
- 主布局限制在视口高度内，主内容过长时仅右侧主内容区滚动。

## 服务端授权现状

- 全局 RBAC 权限当前主要在用户、角色、权限管理和 API Key 管理等系统接口上由后端中间件强制。
- Mock Server、Mock Value Sets、测试数据表、AI 提供商、项目 Prompt 等项目路径已按 viewer/developer 项目角色控制。
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
- 重置后原 token 立即失效，新明文仅在独立一次性弹窗展示。
