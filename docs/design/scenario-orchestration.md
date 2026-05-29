# 场景编排设计

本文档记录测试场景编排、步骤编辑、变量传递、控制流、克隆和运行结果的业务设计。

## 场景与步骤

- 场景由有序步骤组成。
- 步骤类型支持 API、数据库操作、脚本、For 循环和条件分支。
- API 步骤引用 API 管理中的请求模板（`test_cases`）。
- 数据库步骤复用当前项目下的业务数据源。
- 脚本步骤在平台内以 JavaScript（goja 沙箱）执行。

## 变量与脚本

- 步骤间通过 **extracts**（见 [场景步骤引用 v2](scenario-step-refs-v2.md)）与 `{{$steps[...].*}}` 传递数据。
- 规范引用：`{{$steps[N].response.body.<path>}}`、`{{$steps["步骤名"].response.*}}`、`{{$steps.slug.response.*}}`；请求侧为 `{{$steps[N].request.query|pathvar|body.*}}`（`N` 为 step_seq）。
- 步骤 `config.extracts` 将本步输出字段提取为场景变量（如 `authToken`），后续步骤使用 `{{authToken}}`。
- 历史扁平路径（`body.*`、`status`）与 `response.body.token`→`data.token` 回退仍兼容，见 v2 设计说明。
- 脚本步骤支持 Postman 风格 `pm.variables` / `pm.environment` / `pm.test` / `console`。
- 脚本步骤带超时，逻辑错误时失败。
- console 输出映射为 stdout/stderr 快照。
- `pm.variables.set` 写回场景变量（与 extracts 等效，适合脚本内动态提取）。

## 步骤覆盖与运行环境

- 步骤可配置独立变量覆盖和请求覆盖（JSON 合并）。
- 场景运行环境仅在场景级选择一次，步骤内不重复选择环境。
- API 步骤中的环境认证继承规则见 `docs/design/api-management-and-runner.md`。

## 步骤编辑与列表交互

- 步骤编辑区支持步骤级 Tab。
- 点击步骤或新增入口时打开/切换 Tab，关闭 Tab 丢弃该 Tab 的内存草稿。
- 浏览器本地仅持久化打开的步骤 Tab 列表与当前激活 Tab，按项目、服务、场景隔离。
- 刷新后重新基于服务端最新数据装载，草稿不持久化。
- 步骤列表支持拖拽排序、点击序号启用/停用步骤、悬停删除、宽度拖拽调节并本地持久化。

## 步骤删除

- 步骤删除采用软删除。
- `(scenario_id, step_order)` 槽位在删除后视为释放。
- 同一 stepOrder 再保存必须落库为全新步骤记录，不复用已软删除行。

## 控制流

- For 循环可按次数或 JSON 数组表达式迭代。
- For 循环提供当前项变量与索引变量，并限制最大迭代次数。
- 数组表达式返回对象时可自动展开常见数组字段。
- 条件步骤支持多个「条件-子步骤组」按书写顺序匹配，并可配置「否则」分支。
- 条件步骤兼容旧版二分支 JSON。
- 控制流步骤通过稳定 `step_seq` 引用子步骤。
- 运行器需记录控制步骤结果、跳过未进入分支、阻断循环自引用与过深嵌套。
- 任一执行步骤失败时场景失败。

## 步骤克隆

- 场景编排「新增步骤」支持克隆入口。
- 克隆源可来自同项目任意场景。
- 克隆为深拷贝并生成全新记录。
- 控制流配置中的 step_seq 需重映射到克隆后的新步骤。

## 运行结果

- 管理后台场景运行结果面板支持拖拽调整高度。
- 步骤结果保留「详情」「断言」「步骤输出」Tab。
- 详情中请求与响应左右排版，并支持 JSON 折叠与语法高亮。

## 测试报告与运行历史

- 每次场景运行写入 `test_runs` / `test_run_results`；历史报告仅展示**已落库的执行步骤**（未进入分支而跳过的步骤不在历史中展示）。
- **运行记录**（管理后台 `/reports/runs`）：按当前项目分页列出场景运行记录，支持场景、状态、时间范围筛选；可跳转报告详情。
- **报告详情**（`/runs/{runId}`）：可分享深链（需登录且具备项目 viewer 权限）；含运行摘要、步骤时间线（For 循环多轮为多条记录）、详情/断言/输出；支持导出 JSON/HTML；失败时可调用 AI 分析；返回导航至项目运行记录列表。
- **导出**：`GET /runs/{runId}/export?format=json|html`，HTML 为单文件内联样式，超长 body 截断（与 AI 分析一致）。
- 场景编辑器提供「查看完整报告」入口（打开 `/runs/{runId}`）；运行记录统一从侧栏 **测试报告 → 运行记录** 进入。
- API：`GET /projects/{id}/runs`（项目运行列表）、`GET /scenarios/{id}/runs`、`GET /scenarios/{id}/runs/stats`（场景维度，供后续扩展）、`GET /runs/{id}/report`、`GET /runs/{id}/export?format=json|html`；保留 `GET /runs/{id}/result`。
