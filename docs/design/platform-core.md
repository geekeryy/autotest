# 平台基础与服务模型设计

本文档记录接口自动化测试平台的基础业务模型。需求索引见 [requirements.md](../requirements.md)。

## 术语

- **接口**：OpenAPI/Swagger 导入写入 `api_endpoints`；「API管理」列表中的每一行对应库表 `test_cases` 中一条与该 HTTP 接口关联的可运行请求模板。Runner 与场景编排中的 API 步骤引用的是该模板。
- **测试用例**：在运行控制台通过「保存用例」固化的一组请求参数快照，保存到数据库并关联当前接口模板；展示为路径栏右侧 Tab，用于多套参数切换与复跑，并可在场景编排中引用。
- **测试集**：已移除。原测试集功能由场景编排统一承载；历史数据库表 `test_suites` / `test_suite_items` 保留不删除，数据可迁移为场景。
- 对内 REST 路径仍为 `/cases`（历史兼容），文档与界面用「接口」指代上述请求模板。

## 基础能力

- 构建一个 API/接口自动化测试平台，支持导入 OpenAPI/Swagger，将 API 定义纳入平台管理。
- 支持基于导入的 API 定义自动生成接口请求模板。
- 当前自动生成规则实际落地为 happy path 请求模板；其他规则类型是预留扩展点，不应在未实现前写成已产出多类模板。
- 支持在 API 管理中手动新建接口请求模板，并在同一平台模型中统一管理手动模板与自动生成模板。
- 不单独提供测试集功能；批量执行多个接口通过场景编排实现。
- 支持删除项目；删除项目时对项目及其关联业务数据执行软删除，默认业务查询不展示已软删除数据。

## 项目、服务与环境

- 项目可包含多个服务，每个服务维护名称与描述；可选开启 **MCP 自动化**（`mcpEnabled`），在「服务与环境」编辑页展示 Cursor 接入配置，详见 [mcp-automation.md](mcp-automation.md)。
- 服务不设置服务级 Base Path；请求由环境 baseURL 与导入的接口路径组合。
- 每个服务独立维护环境配置列表。
- 服务环境包含环境名称、baseURL、变量 JSON 与认证 JSON。

## Runner、断言与报告

- Runner 根据请求模板所属服务和选择的服务环境执行请求。
- Runner 支持接口请求模板运行与场景编排运行。
- 平台支持断言，用于校验 API 响应和测试结果。
- 平台为测试运行生成执行报告。
- 运行结果会写入运行记录与结果快照；涉及字段变更时需要同时考虑历史运行记录兼容性。

## 示例服务与迁移

- 在 `tests` 目录提供可运行的示例 API 服务，用于生成 OpenAPI/Swagger 文件并作为平台导入与执行的被测服务。
- 示例 API 支持 JWT 登录，业务接口通过 Bearer Token 访问。
- `/api/v1/admin` 示例接口使用独立于普通业务接口的鉴权密钥。
- 通过 migrations 维护数据库结构变更。

## AI 能力模块概览

平台 AI 能力分布在以下模块中，详细设计见 [ai-capabilities.md](ai-capabilities.md)：

| 模块 | 职责 |
|------|------|
| `internal/aiprovider` | AI 提供商管理、Planner（意图分类）、Router（工具包选择）、SSE 对话引擎 |
| `internal/aitools` | Tool Calling 框架：Registry、Catalog、Meta 工具（find_tools/describe_tools）、向量检索 |
| `internal/aitools/builtin` | 60+ 内置工具实现（只读/自动写/删除写），按域拆分 |
| `internal/aisession` | AI 会话与消息持久化、SSE 流式推送、Token 用量统计 |
| `internal/aiassert` | 三层断言推断引擎（Schema 规则/历史分析/语义推断） |
| `internal/aifactory` | 语义测试数据工厂（字段名语义推断、多场景/locale） |
| `internal/ainl` | 自然语言→测试场景编排（LLM 解析 + 端点匹配） |
| `internal/aimemory` | AI 助理持久化记忆（用户偏好、项目约定） |
| `internal/aiskill` | 技能发现（从路由日志识别高频操作模式） |
| `internal/aifeedback` | AI 反馈收集（用户满意度、自进化驱动） |
| `internal/evalagent` | 自评估代理（Golden Set 定期评测、指标回归告警） |
| `internal/aianalysis` | 失败分析与 spec 变更影响分析 |
| `internal/aiconfig` | AI 助理平台配置（singleton 行，热更新） |
| `internal/projectprompt` | 平台 Prompt 管理（action 级 System Prompt） |
| `internal/promptlayer` | Prompt 分层系统（基础层/画像层/行为层叠加） |
| `internal/genagent` | 执行-修复闭环 Agent（异步场景生成→验证→修复） |
| `internal/scenariogen` | 依赖图驱动的覆盖场景生成 |

## 其他新增模块

| 模块 | 职责 |
|------|------|
| `internal/mockgen` | 基于 JSON Schema 的 Mock 数据生成（语义推断、中文业务数据） |
| `internal/mockrecord` | Mock Server 录制与回放（转发真实服务、精确匹配回放） |
| `internal/testprofile` | 项目测试画像（从历史运行数据学习项目特征） |
| `internal/specdiff` | Spec 版本差异比较 |
| `internal/sampler` | 响应采样与 profile 适配（从 schema 生成请求参数） |
