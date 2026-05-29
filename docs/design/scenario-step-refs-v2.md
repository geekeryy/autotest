# 场景步骤引用 v2

## 步骤输出结构（API 步骤）

```json
{
  "response": {
    "status": 200,
    "headers": { "X-Request-Id": "..." },
    "body": { "data": { "token": "..." } }
  },
  "request": {
    "query": {},
    "pathvar": {},
    "body": {}
  }
}
```

数据库步骤仍使用顶层 `firstRow` / `rows`；脚本步骤使用 `stdout` / `stderr` / `stdoutJson`。

## 引用语法

| 写法 | 说明 |
|------|------|
| `{{$steps[N].response.body.<path>}}` | 按 **step_seq** 引用响应 JSON（规范写法） |
| `{{$steps["步骤名"].response.body.<path>}}` | 按步骤 **name** 引用（名称需唯一） |
| `{{$steps.slug.response.body.<path>}}` | 按 name 的 slug 引用（小写、非字母数字变 `_`） |
| `{{$steps[N].request.query.x}}` | 引用该步实际发出的请求参数 |
| `{{authToken}}` | 使用步骤 **extracts** 提取到场景变量的值 |

### 兼容（deprecated）

- `{{$steps[N].body.*}}` / `{{$steps[N].status}}` 映射到 `response.*`
- `response.body.token` 缺失时尝试 `response.body.data.token`

## 变量提取（extracts）

API / 数据库 / 脚本步骤的 `config` 可配置：

```json
{
  "extracts": [
    { "name": "authToken", "from": "response.body.data.token" }
  ]
}
```

步骤**成功执行后**，将 `from` 路径相对于**本步输出**解析，写入场景变量，供后续步骤 `{{authToken}}` 使用。`name` 须匹配 `[A-Za-z_][A-Za-z0-9_]*`。

登录链推荐：登录步配置 `authToken` 提取，后续步骤 `Authorization: Bearer {{authToken}}`。
