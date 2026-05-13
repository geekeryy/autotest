# AI 能力设计

本文档记录 AI 提供商、项目级 Prompt、AI 生成与 AI 分析能力的业务设计。

## AI 提供商

- 平台支持项目级 AI 提供商配置。
- 提供商类型包括 deepseek、xiaomi、openai、anthropic、kimi、ollama。
- 每个项目可维护多份 AI 提供商配置。
- 配置包含名称、Base URL、API Key 脱敏、默认模型、extraConfig、启用状态与是否默认。
- 同一项目最多一个默认提供商。

## 项目级 Prompt

- 项目级 Prompt（`project_ai_prompts`）可按动作维护 System Prompt / 默认模型。
- 项目级 Prompt 可选绑定 providerId。
- providerId 留空时跟随项目默认 AI 提供商。

## 统一 AI 入口

- `/projects/{projectID}/ai/chat` 统一 AI 入口支持 `generate_params`、`generate_assertion`、`generate_case_data`、`analyze_failure`、`analyze_spec_changes` 等 action。
- 运行控制台的「一键生成参数」还存在非 LLM 采样接口 `GET /cases/{caseID}/generate-params`；它与 LLM `generate_params` 是两条路径，文档和实现变更时需要区分。
- `generate_assertion` 必须在弹窗中填写非空测试意图，后端拒绝空意图。
- `generate_case_data` 使用项目级 Prompt 配置。
- `generate_case_data` 未配置 provider 或 prompt 时返回明确中文错误。

## 生成请求参数

- `generate_params` 的 context 必须基于 `api_endpoints` 真实存在字段构造。
- 禁止引用 endpoint 上未定义属性。
- context 至少包含 method、path、pathVarNames、endpoint summary/operationId/tags/requestSchema/responseSchema、currentRequest。
- Mock Value Sets 摘要可注入 `generate_params` 上下文，供模型优先选择 `{{$mock.set.<key>}}`。
- 系统提示需声明字段含义。
- 系统提示需要求模型保留 currentRequest 中已存在且非空的值，仅补全缺失字段。
- 系统提示需禁止 markdown 围栏。

## AI 分析

- 测试失败分析基于本次 run 请求/响应快照、断言失败明细、失败步骤摘要调用 AI。
- 测试失败分析返回原因推断与修复建议。
- spec 变更影响分析基于本次导入 spec 与同一 service 上一条 spec 对比。
- spec 变更影响分析结合当前服务下接口模板与场景步骤摘要输出影响评估与建议动作。
- 当前既有 `/ai/chat` action，也有专用 HTTP 分析入口（如运行失败分析、spec 变更分析）；分析结果当前不写库。

## 与其他能力的关系

- Mock Value Sets 参与 `generate_params` 的上下文注入，详见 `docs/design/mock-template-and-test-data.md`。
- 运行控制台中的 AI 生成参数入口与请求编辑能力关系见 `docs/design/api-management-and-runner.md`。
