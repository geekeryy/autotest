// templateTokens.js
//
// 单一事实来源：管理后台展示「变量与函数引用」相关帮助信息。
// 运行控制台、场景编排、模板参考页等需要列出可用占位符的位置
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
  { name: 'pick', example: '{{$mock.pick(a,b,c)}}', description: '从参数列表随机挑一个（oneOf/choice 是别名）' },
  { name: 'set', example: '{{$mock.set.<key>}} / {{$mock.set.<key>[0]}}', description: '项目「命名值集合」；候选值可为 JSON 类型，占位输出紧凑 JSON（纯字符串取原文）' },
  { name: 'idCard', example: '{{$mock.idCard}}', description: '中国二代身份证 18 位（含 GB 11643 校验位，仅供测试用）' },
  { name: 'plateNumber', example: '{{$mock.plateNumber}}', description: '中国车牌号（省份简称 + · + 1 字母 + 5 位字母数字）' },
  { name: 'bankCard', example: '{{$mock.bankCard}} / {{$mock.bankCard(16)}}', description: '16-19 位 Luhn 合法银行卡号，默认 19 位，可指定长度' },
  { name: 'unifiedSocialCreditCode', example: '{{$mock.unifiedSocialCreditCode}}', description: '统一社会信用代码 18 位（GB 32100，校验位合法）' },
  { name: 'sku', example: '{{$mock.sku}} / {{$mock.sku(12)}}', description: 'SKU 编号，默认 `[A-Z]{2}-\\d{6}`，可指定总长' }
]

// 命名值集合（mock value sets）模板语法说明，与后端 internal/mockset 共同
// 维护：示例片段也用于 TemplateReference 页面以及 MockValueSetList 顶部说明区。
export const mockSetSyntax = [
  {
    name: '{{$mock.set.<key>}}',
    example: '{{$mock.set.colors}}',
    description: '按权重随机抽样（无权重时均匀随机）；集合项可为 JSON 对象/数组等，渲染为紧凑 JSON 文本'
  },
  {
    name: '{{$mock.set.<key>[N]}}',
    example: '{{$mock.set.colors[1]}}',
    description: '按 0-based 索引取值；越界时 Runner 报明确中文错误'
  },
  {
    name: '{{$mock.set.<key>[*]}}',
    example: '{{$mock.set.colors[*]}}',
    description: '同一次 run 内按顺序循环遍历（不同 run 互不影响；mockserver 按请求维度独立）'
  }
]

export const stepRefFields = [
  { name: 'response.status', example: '{{$steps[1].response.status}}', description: 'API 步骤 HTTP 状态码' },
  { name: 'response.headers.<Name>', example: '{{$steps[1].response.headers.X-Request-Id}}', description: 'API 步骤响应头' },
  { name: 'response.body.<path>', example: '{{$steps[1].response.body.data.token}}', description: 'API 步骤响应 JSON，支持 a.b[0].c' },
  { name: '按步骤名', example: '{{$steps["用户登录"].response.body.data.token}}', description: '用步骤 name 代替序号（名称需唯一）' },
  { name: '按 slug', example: '{{$steps.login.response.body.data.token}}', description: '步骤名 slug 引用（小写、空格等变下划线）' },
  { name: 'request.query.<name>', example: '{{$steps[1].request.query.userId}}', description: 'API 步骤实际发出的查询参数' },
  { name: 'request.pathvar.<name>', example: '{{$steps[1].request.pathvar.id}}', description: 'API 步骤实际发出的路径参数' },
  { name: 'request.body.<path>', example: '{{$steps[1].request.body.user.email}}', description: 'API 步骤实际发出的请求体字段' },
  { name: 'extracts → {{var}}', example: '{{authToken}}', description: '在步骤 config.extracts 中从本步输出提取后，用场景变量名引用' },
  { name: 'firstRow.<column>', example: '{{$steps[2].firstRow.user_id}}', description: '数据库步骤结果首行字段' },
  { name: 'rows[<index>].<column>', example: '{{$steps[2].rows[0].name}}', description: '数据库步骤指定行字段' },
  { name: 'stdout', example: '{{$steps[3].stdout}}', description: '脚本步骤的标准输出（按 console.log 累计）' },
  { name: 'stdoutJson.<path>', example: '{{$steps[3].stdoutJson.count}}', description: '脚本 stdout 解析为 JSON 后的字段值（无法解析则失败）' }
]

export const sqlInlineFields = [
  {
    name: '{{$sql.<sourceKey>.<column>}}',
    example: '{{$sql.userSeed.id}}',
    description: '取 SQL 参数源结果第一行的指定列值（兼容历史 `{{sql.*}}`）'
  },
  {
    name: '{{$sql.<sourceKey>[<filterColumn>=<filterValue>].<column>}}',
    example: '{{$sql.userSeed[status=active].id}}',
    description: '在结果中按等值过滤后取第一条匹配行的列值（兼容历史 `{{sql.*}}`）'
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
  { name: '$req.method', example: '{{$req.method}}', description: '当前请求方法（兼容历史 `{{request.method}}`）' },
  { name: '$req.path', example: '{{$req.path}}', description: '请求 URL 的路径部分（兼容历史 `{{request.path}}`）' },
  { name: '$req.url', example: '{{$req.url}}', description: '请求 URL 的完整 RequestURI（含 query；兼容历史 `{{request.url}}`）' },
  { name: '$req.bodyRaw', example: '{{$req.bodyRaw}}', description: '原始请求体字符串（兼容历史 `{{request.bodyRaw}}`）' },
  { name: '$req.pathvar.<name>', example: '{{$req.pathvar.id}}', description: '路径参数占位符的值，需在规则路径中声明 {id}（兼容历史 `{{request.pathvar.*}}`）' },
  { name: '$req.query.<name>', example: '{{$req.query.tenant}}', description: '查询参数取值（兼容历史 `{{request.query.*}}`）' },
  { name: '$req.header.<Name>', example: '{{$req.header.X-Trace}}', description: '请求头取值，大小写不敏感（兼容历史 `{{request.header.*}}`）' },
  { name: '$req.body.<path>', example: '{{$req.body.user.id}}', description: '请求体 JSON 字段，支持点路径（兼容历史 `{{request.body.*}}`）' }
]

export const renderingPipeline = [
  { stage: '1', label: '$mock.* 模拟标签', detail: '每次请求实时生成新值；多次出现互不相同；未识别 helper 保留字面量。' },
  { stage: '2', label: '$steps[...].* / extracts', detail: '场景编排：$steps 按 step_seq/名称/slug 解析 response.* 与 request.*；步骤 extracts 写入场景变量。' },
  { stage: '3', label: '$ds.* / $sql.* 测试数据与 SQL 引用', detail: 'Runner 在发请求前解析测试数据表引用，并自动执行对应 SQL 参数源，按列名取值；继续兼容历史 `{{sql.*}}`。' },
  { stage: '4', label: '{{varName}} 普通变量', detail: '从环境变量、场景变量、运行覆盖变量、模板默认变量合并取值。' }
]
