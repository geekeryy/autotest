// templateTokens.js
//
// 单一事实来源：管理后台展示「变量与函数引用」相关帮助信息。
// 运行控制台、场景编排、模板与变量参考页等需要列出可用占位符的位置
// 都从这里读取，避免文案分散后不同步。
//
// helper 列表与后端 internal/mockdata.ListHelpers 同步维护；新增 helper
// 时请同步更新两处，并在 docs/requirements.md 的相关条目里登记。

export const mockHelperList = [
  { name: 'uuid', example: '{{$mock.uuid}}', description: '随机 UUID v4' },
  { name: 'now', example: "{{$mock.now}} / {{$mock.now('2006-01-02 15:04:05')}}", description: '当前时间，可选 Go time 布局' },
  { name: 'timestamp', example: '{{$mock.timestamp}} / {{$mock.timestamp(ms)}}', description: '当前 Unix 时间戳，单位 s/ms/ns' },
  { name: 'int', example: '{{$mock.int(1,100)}}', description: '随机整数（默认 1-100）' },
  { name: 'float', example: '{{$mock.float(0,1,4)}}', description: '随机浮点（默认 0-100，保留 2 位小数）' },
  { name: 'bool', example: '{{$mock.bool}}', description: '随机布尔' },
  { name: 'string', example: '{{$mock.string(8)}}', description: '指定长度字母字符串' },
  { name: 'word', example: '{{$mock.word}}', description: '随机英文单词' },
  { name: 'sentence', example: '{{$mock.sentence(6)}}', description: 'N 个中文词语的句子' },
  { name: 'name', example: '{{$mock.name}}', description: '随机中文姓名（fullname 是别名）' },
  { name: 'firstName', example: '{{$mock.firstName}}', description: '随机中文名' },
  { name: 'lastName', example: '{{$mock.lastName}}', description: '随机中文姓（surname 是别名）' },
  { name: 'email', example: '{{$mock.email}}', description: '随机邮箱（mail 是别名）' },
  { name: 'phone', example: '{{$mock.phone}}', description: '随机中国手机号（mobile 是别名）' },
  { name: 'url', example: '{{$mock.url}}', description: '随机 URL（uri 是别名）' },
  { name: 'ipv4', example: '{{$mock.ipv4}}', description: '随机 IPv4（ip 是别名）' },
  { name: 'ipv6', example: '{{$mock.ipv6}}', description: '随机 IPv6' },
  { name: 'city', example: '{{$mock.city}}', description: '随机城市' },
  { name: 'country', example: '{{$mock.country}}', description: '随机国家' },
  { name: 'address', example: '{{$mock.address}}', description: '随机中文地址' },
  { name: 'company', example: '{{$mock.company}}', description: '随机公司名（organization/org 是别名）' },
  { name: 'color', example: '{{$mock.color}}', description: '随机颜色（colour 是别名）' },
  { name: 'date', example: '{{$mock.date}}', description: '随机日期 yyyy-MM-dd' },
  { name: 'dateTime', example: '{{$mock.dateTime}}', description: '随机日期时间 RFC3339' },
  { name: 'pick', example: '{{$mock.pick(a,b,c)}}', description: '从参数列表随机挑一个（oneOf/choice 是别名）' }
]

export const stepRefFields = [
  { name: 'status', example: '{{$steps[1].status}}', description: 'API 步骤的 HTTP 响应状态码（数字）' },
  { name: 'headers.<Name>', example: '{{$steps[1].headers.X-Request-Id}}', description: 'API 步骤的响应头取值' },
  { name: 'body.<path>', example: '{{$steps[1].body.data.token}}', description: 'API 步骤响应 JSON 字段，支持 a.b[0].c 形式' },
  { name: 'request.query.<name>', example: '{{$steps[1].request.query.userId}}', description: 'API 步骤实际发出的查询参数值' },
  { name: 'request.pathvar.<name>', example: '{{$steps[1].request.pathvar.id}}', description: 'API 步骤实际发出的路径参数值（兼容历史 pathvar 语法）' },
  { name: 'request.body.<path>', example: '{{$steps[1].request.body.user.email}}', description: 'API 步骤实际发出的请求体字段' },
  { name: 'firstRow.<column>', example: '{{$steps[2].firstRow.user_id}}', description: '数据库步骤结果首行字段' },
  { name: 'rows[<index>].<column>', example: '{{$steps[2].rows[0].name}}', description: '数据库步骤指定行字段' },
  { name: 'stdout', example: '{{$steps[3].stdout}}', description: '脚本步骤的标准输出（按 console.log 累计）' },
  { name: 'stdoutJson.<path>', example: '{{$steps[3].stdoutJson.count}}', description: '脚本 stdout 解析为 JSON 后的字段值（无法解析则失败）' }
]

export const sqlInlineFields = [
  {
    name: '{{sql.<sourceKey>.<column>}}',
    example: '{{sql.userSeed.id}}',
    description: '取 SQL 参数源结果第一行的指定列值'
  },
  {
    name: '{{sql.<sourceKey>[<filterColumn>=<filterValue>].<column>}}',
    example: '{{sql.userSeed[status=active].id}}',
    description: '在结果中按等值过滤后取第一条匹配行的列值'
  }
]

export const testDataInlineFields = [
  {
    name: '{{$ds.<tableKey>.<column>}}',
    example: '{{$ds.users.email}}',
    description: '取测试数据表首行的指定列值'
  },
  {
    name: '{{$ds.<tableKey>[<filterColumn>=<filterValue>].<column>}}',
    example: '{{$ds.users[role=admin].email}}',
    description: '在测试数据表行中按等值过滤后取第一条匹配行的列值'
  }
]

export const mockResponseFields = [
  { name: 'request.method', example: '{{request.method}}', description: '当前请求方法' },
  { name: 'request.path', example: '{{request.path}}', description: '请求 URL 的路径部分' },
  { name: 'request.url', example: '{{request.url}}', description: '请求 URL 的完整 RequestURI（含 query）' },
  { name: 'request.bodyRaw', example: '{{request.bodyRaw}}', description: '原始请求体字符串（未解析）' },
  { name: 'request.pathvar.<name>', example: '{{request.pathvar.id}}', description: '路径参数占位符的值，需在规则路径中声明 {id}（兼容历史 pathvar 语法）' },
  { name: 'request.query.<name>', example: '{{request.query.tenant}}', description: '查询参数取值' },
  { name: 'request.header.<Name>', example: '{{request.header.X-Trace}}', description: '请求头取值（大小写不敏感）' },
  { name: 'request.body.<path>', example: '{{request.body.user.id}}', description: '请求体 JSON 字段，支持点路径' }
]

export const renderingPipeline = [
  { stage: '1', label: '$mock.* 模拟标签', detail: '每次请求实时生成新值；多次出现互不相同；未识别 helper 保留字面量。' },
  { stage: '2', label: '$steps[N].* 场景步骤引用', detail: '仅在场景编排中生效；按上一步输出按 JSONPath 解析。' },
  { stage: '3', label: '$ds.* / sql.* 测试数据与 SQL 引用', detail: 'Runner 在发请求前解析测试数据表引用，并自动执行对应 SQL 参数源，按列名取值。' },
  { stage: '4', label: '{{varName}} 普通变量', detail: '从环境变量、场景变量、运行覆盖变量、模板默认变量合并取值。' }
]
