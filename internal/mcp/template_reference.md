# Autotest 模板变量参考（MCP / CI）

模板字符串在**运行**时由 Runner 解析；MCP 工具将模板原样写入 `request`、`requestOverride` 或 `config`，不在 MCP 层求值。

## 命名空间

| 命名空间 | 写法示例 | 用途 |
|----------|----------|------|
| 场景变量 | `{{authToken}}` | 前序步骤 `config.extracts` 提取后，后续 header/body 引用 |
| 步骤引用（推荐按名） | `{{$steps["登录"].response.body.data.token}}` | 创建场景时无需知道 `step_seq` |
| 步骤引用（按 seq） | `{{$steps[2].response.body.id}}` | 步骤创建后 seq 固定，适合二次编辑 |
| 测试数据 | `{{$ds.users.email}}`、`{{$ds.users[role=admin].email}}` | 请求模板 / requestOverride |
| SQL 参数源 | `{{$sql.sourceKey.column}}` | 同上 |
| Mock | `{{$mock.email}}`、`{{$mock.set.users}}` | 同上 |
| 环境变量 | `{{baseUrl}}` | 环境配置 + `run_scenario` 的 `variables` 覆盖 |

## API 步骤 requestOverride

可覆盖模板字段：`headers`、`variables`、`body`、`path`、`query`（JSON 对象）。示例：

```json
{
  "headers": { "Authorization": "Bearer {{authToken}}" },
  "body": { "userId": "{{$steps[\"查询用户\"].response.body.data.id}}" }
}
```

## 登录链 + extracts 示例

1. 登录步 `config`：`{"extracts":[{"name":"authToken","from":"response.body.data.token"}]}`
2. 下一步 `requestOverride.headers`：`Authorization: Bearer {{authToken}}`

## 控制流子步骤

创建场景时在 `config` 使用 `bodyStepOrders` / `thenStepOrders` / `branches[].stepOrders`（引用同次 payload 中的 **stepOrder**）；平台会转换为内部 `step_seq`。

## 禁止

勿使用 `__FILL_*` 等占位符；应使用真实 body、断言或 `{{$mock.*}}` / `{{$ds.*}}`。
