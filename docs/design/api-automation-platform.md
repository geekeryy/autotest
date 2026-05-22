# 接口自动化测试平台设计总览

本文档仅作为设计总览和导航。业务需求索引见 `docs/requirements.md`；工程规范和 agent 协作约束见 `.cursor/rules/*.mdc`。

## 设计文档

- [平台基础与服务模型](platform-core.md)：术语、项目/服务/环境、Runner、断言、报告、示例服务和 migrations。
- [API 导入与运行控制台](api-management-and-runner.md)：OpenAPI/Swagger 导入、API 管理、运行控制台、请求编辑、响应展示、环境认证继承。
- [场景编排](scenario-orchestration.md)：场景步骤、变量传递、脚本步骤、控制流、克隆、步骤 Tab、运行结果。
- [Mock、模板变量与测试数据](mock-template-and-test-data.md)：Mock Server、Mock Value Sets、运行时模拟标签、模板变量、SQL 参数源、测试数据表。
- [AI 能力](ai-capabilities.md)：AI 提供商、平台 Prompt、AI 生成、失败分析、spec 变更影响分析。
- [管理后台与访问控制](admin-and-access.md)：后台布局、全局项目上下文、服务环境管理、菜单、视觉品牌、脚本库、API Key。

## 待用户决策

- `generate_params` / 「一键生成参数」的覆盖范围存在历史冲突：一种说法要求仅覆盖 path、query、body，且不返回或覆盖 headers/security；另一种说法要求覆盖 query、headers、path、JSON Body。相关实现变更必须先由用户明确决策后再继续。
