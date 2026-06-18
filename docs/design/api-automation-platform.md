# 接口自动化测试平台设计总览

本文档作为设计文档导航。需求与文档索引见 [requirements.md](../requirements.md)；架构与部署见 [architecture.md](architecture.md)；快速上手见根目录 [README.md](../../README.md)。工程规范和 agent 协作约束见 `.cursor/rules/*.mdc`。

## 设计文档

| 文档 | 内容 |
|------|------|
| [architecture.md](architecture.md) | 系统架构、配置与环境变量、部署（Firebase 分离为默认，All-in-One 可选） |
| [platform-core.md](platform-core.md) | 术语、项目/服务/环境、Runner、断言、报告、示例服务、AI 能力模块概览 |
| [api-management-and-runner.md](api-management-and-runner.md) | OpenAPI/Swagger 导入、API 管理、运行控制台、环境认证 |
| [mcp-automation.md](mcp-automation.md) | MCP 服务（stdio）、全量自动化工具与 Cursor 配置；Swagger 导入见 [mcp-swagger-import.md](mcp-swagger-import.md) |
| [scenario-orchestration.md](scenario-orchestration.md) | 场景步骤、变量传递、控制流、克隆、运行报告 |
| [mock-template-and-test-data.md](mock-template-and-test-data.md) | Mock Server、Mock Value Sets、模板变量、SQL 参数源、测试数据、Schema Mock 生成、录制回放 |
| [ai-capabilities.md](ai-capabilities.md) | AI 提供商、Prompt、生成/分析、Tool Calling、全局助理浮窗、断言推断、语义数据工厂、NL 编排、记忆/技能/反馈/自评估 |
| [admin-and-access.md](admin-and-access.md) | 登录与 OAuth、RBAC、菜单、通知、API Key |

## 待用户决策

- `generate_params` / 「一键生成参数」的覆盖范围存在历史冲突：一种说法要求仅覆盖 path、query、body，且不返回或覆盖 headers/security；另一种说法要求覆盖 query、headers、path、JSON Body。相关实现变更必须先由用户明确决策后再继续。
