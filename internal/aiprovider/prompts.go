package aiprovider

import (
	"encoding/json"
	"strings"

	"autotest/internal/aiprovider/client"
)

// buildMessages constructs the system + user messages for an action.
// The structured `context` (as raw JSON) lets the frontend ship arbitrary metadata.
// When systemOverride is non-empty it replaces the built-in system prompt.
//
// The user message is composed of three parts so the model can never miss any
// of them:
//
//  1. A short, action-specific task preamble naming the structured context
//     fields it will see (so it does not treat them as unrelated noise).
//  2. The original user-supplied prompt (e.g. assertion intent).
//  3. The full structured context, pretty-printed as JSON inside a fenced
//     block so providers that can't read raw JSON still see clear delimiters.
func buildMessages(action string, prompt string, ctx json.RawMessage, systemOverride string) ([]client.Message, bool) {
	jsonOnly := false
	system := ""
	switch action {
	case ActionGenerateParams:
		jsonOnly = true
		system = generateParamsSystem
	case ActionGenerateAssertion:
		system = generateAssertionSystem
	case ActionGenerateCaseData:
		jsonOnly = true
		system = generateCaseDataSystem
	case ActionAnalyzeFailure:
		system = analyzeFailureSystem
	case ActionAnalyzeSpecChanges:
		system = analyzeSpecChangesSystem
	default:
		system = rawSystem
	}
	if systemOverride != "" {
		system = systemOverride
	}

	preamble := userPreamble(action)

	user := strings.TrimSpace(prompt)
	if action == ActionGenerateAssertion && user != "" {
		user = "测试意图（中文）：\n" + user
	}

	if len(ctx) > 0 {
		var indented string
		var v any
		if err := json.Unmarshal(ctx, &v); err == nil {
			if buf, err := json.MarshalIndent(v, "", "  "); err == nil {
				indented = string(buf)
			}
		}
		if indented == "" {
			indented = string(ctx)
		}
		if user != "" {
			user += "\n\n"
		}
		user += "上下文（JSON）：\n```json\n" + indented + "\n```"
	}

	if preamble != "" {
		if user != "" {
			user = preamble + "\n\n" + user
		} else {
			user = preamble
		}
	}
	if user == "" {
		user = "请根据系统说明开始生成。"
	}

	return []client.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, jsonOnly
}

// userPreamble returns a short instruction line telling the model what to do
// with the structured context for each action. It is prepended to the user
// message so the model does not silently skip context fields.
func userPreamble(action string) string {
	switch action {
	case ActionGenerateParams:
		return "请基于下方上下文生成请求参数 JSON。务必先理解 endpoint.requestSchema、pathVarNames 与 currentRequest，再按系统说明的规则输出。"
	case ActionGenerateAssertion:
		return "请基于下方测试意图与上下文生成 Postman 风格 pm.test 断言代码。"
	case ActionGenerateCaseData:
		return "请基于下方上下文生成多行测试数据 JSON。"
	case ActionAnalyzeFailure:
		return "请基于下方运行失败上下文（请求/响应快照、断言失败明细、场景步骤摘要）按系统说明的三段结构输出中文 markdown 失败分析。"
	case ActionAnalyzeSpecChanges:
		return "请基于下方 spec 结构化 diff 与受影响的接口模板/场景步骤清单，按系统说明的四段结构输出中文 markdown 变更影响分析。"
	default:
		return ""
	}
}

const generateParamsSystem = `你是接口自动化测试平台的「请求参数生成器」。仅输出符合下文规范的单个 JSON 对象。

【输入上下文字段】
用户消息中会附带一段 JSON 上下文，可能包含以下字段（任何一项都可能缺失，需按字段名识别，不要凭直觉重命名）：
- method：HTTP 方法（GET/POST/PUT/PATCH/DELETE …）。
- path：含 OpenAPI 占位符（` + "`" + `{var}` + "`" + `）的请求路径，例如 ` + "`" + `/api/v1/users/{id}` + "`" + `。
- pathVarNames：当前路径需要填值的所有路径参数名（数组，字段名保留历史兼容）。**输出 JSON 顶层 ` + "`" + `path` + "`" + ` 子对象的 key 必须是该数组的子集**；不在数组中的键一律不要出现。
- endpoint.summary / endpoint.operationId / endpoint.tags：接口语义提示，用于推断字段含义。
- endpoint.requestSchema：请求 schema，含 ` + "`" + `parameters` + "`" + `（query/header/path 列表，每项包含 name/in/schema/description/required）、` + "`" + `body` + "`" + `（JSON Schema：properties/items/example/default/enum/format/description 等）、` + "`" + `security` + "`" + `。
- endpoint.responseSchema：响应 schema，仅作语义参考，不要照搬到请求里。
- currentRequest：用户已经填写并启用的请求参数，按 ` + "`" + `pathVars` + "`" + `、` + "`" + `query` + "`" + `、` + "`" + `headers` + "`" + ` 三个键值对象 + ` + "`" + `body` + "`" + `（已解析的 JSON 或字符串）组织。
- availableMockSets：当前项目已配置的「命名值集合（mock value sets）」摘要数组，每项形如 ` + "`" + `{key, name, valuesPreview, hasWeights, totalValueSize}` + "`" + `。若字段含义匹配某 availableMockSets，**优先输出 `+"`{{$mock.set.<key>}}`"+`**（运行时按权重抽样），不要编造取值；如需固定索引或顺序遍历，可使用 `+"`{{$mock.set.<key>[N]}}`"+` / `+"`{{$mock.set.<key>[*]}}`"+`。

【输出格式】
**必须**且**只能**输出单个 JSON 对象，禁止输出任何解释文字、注释或 Markdown 围栏（不要使用三个反引号）：
{
  "path":    { "<pathVarNames 内的变量名>": "<取值>" },
  "query":   { "<查询参数名>": "<取值>" },
  "headers": { "<请求头名>": "<取值>" },
  "body":    <符合 requestSchema.body 的合法 JSON>
}
- 不需要的分区可省略，但顶层至少包含一个非空字段。
- 参数名严格保持与输入一致（区分大小写、保留连字符/下划线）。

【字段取值的优先级】
1. **保留 currentRequest 中已存在且非空的值**（用户已在表单上手填或上次保留的值）。除非用户在 prompt 中明确要求"重新生成 / 覆盖"，否则不要修改这些值。
2. 对 currentRequest 中缺失或为空字符串的字段，按以下优先级生成：
   schema.example → schema.default → schema.enum[0] → 运行时数据模拟标签（仅字符串字段；见下文）→ 按 schema.format / 字段名启发式生成具体仿真值。
3. 不要凭空捏造 schema 之外的字段；body 字段集合应与 ` + "`" + `requestSchema.body.properties` + "`" + ` 完全对齐（包含必填字段，可省略明显冗余字段）。

【运行时数据模拟标签】
平台 Runner 在每次发请求前会把字符串中的 ` + "`" + `{{$mock.<helper>}}` + "`" + ` / ` + "`" + `{{$mock.<helper>(args)}}` + "`" + ` 实时替换为新值；多次出现各自独立。
- **仅对字符串字段**输出模拟标签；整数、浮点、布尔、对象、数组、null 必须保持具体 JSON 类型，不要把它们包成字符串。
- 对动态/不希望写死的字段（id、uuid、email、phone、createdAt、updatedAt、requestId、token、url、ipv4 等）**优先输出模拟标签**，使运行时每次自动获得新值。
- 对枚举或语义明确（example/default/enum、固定状态、固定路径片段、用户已填的真实业务值）的字段保留具体值，不要替换为模拟标签。

可用 helper（helper 名大小写不敏感；含逗号或空格的参数请用单/双引号包裹）：
- uuid                 → "{{$mock.uuid}}"                 随机 UUID v4
- now                  → "{{$mock.now}}" / "{{$mock.now('2006-01-02 15:04:05')}}" 当前时间，可选 Go time 布局
- timestamp            → "{{$mock.timestamp}}" / "{{$mock.timestamp(ms)}}" 当前 Unix 时间戳（s/ms/ns）
- int / integer        → "{{$mock.int(1,100)}}"  仅当字段类型为字符串时使用；JSON number 字段请直接给具体数字
- float / number       → "{{$mock.float(0,1,4)}}" 同上
- bool / boolean       → "{{$mock.bool}}" 同上
- string(n)            → "{{$mock.string(8)}}" 指定长度的字母字符串
- word / sentence(n)   → "{{$mock.word}}" / "{{$mock.sentence(6)}}"，sentence 生成中文句子
- name / firstName / lastName → "{{$mock.name}}" / "{{$mock.firstName}}" / "{{$mock.lastName}}"
- email / phone / url  → "{{$mock.email}}" / "{{$mock.phone}}" / "{{$mock.url}}"
- ipv4 / ipv6          → "{{$mock.ipv4}}" / "{{$mock.ipv6}}"
- city / country / address / company / color
- date / dateTime      → "{{$mock.date}}" / "{{$mock.dateTime}}"
- pick / oneOf         → "{{$mock.pick(admin,tester,viewer)}}" 从列表随机挑一个
- idCard / plateNumber / bankCard / unifiedSocialCreditCode / sku → 中国二代身份证 / 车牌 / 银行卡 / 统一社会信用代码 / SKU 样本（业务字段优先）
- set                  → "{{$mock.set.<key>}}" 项目命名值集合，请配合 availableMockSets 字段使用，不要编造 key

【完整示例】
若 path = "/api/v1/users/{id}/orders"，pathVarNames = ["id"]，requestSchema.body 含 userId(string)/email(string)/age(integer)/active(boolean)/role(enum:admin|tester)：
{
  "path":    { "id": "{{$mock.uuid}}" },
  "query":   { "page": 1, "keyword": "{{$mock.word}}" },
  "headers": { "X-Request-Id": "{{$mock.uuid}}" },
  "body": {
    "userId":    "{{$mock.uuid}}",
    "email":     "{{$mock.email}}",
    "age":       28,
    "active":    true,
    "role":      "admin"
  }
}

最后再次强调：只输出单个 JSON 对象，不要任何解释、注释或 Markdown 围栏。`

const generateAssertionSystem = "你是接口自动化测试平台的「断言脚本生成器」。\n" +
	"- 用户在弹窗中必须提供非空的测试意图（中文）；用户消息中会包含该意图说明。\n" +
	"- 输入还包含响应快照（status、headers、body）等结构化上下文。\n" +
	"- 输出 Postman 风格 JavaScript 断言代码，可直接粘贴到平台脚本断言编辑器中。\n" +
	"- 可用 API：pm.test('名称', () => { ... })、pm.response.code、pm.response.json()、pm.expect(...).to.equal(...) 等。\n" +
	"- 至少包含一个 pm.test，覆盖响应状态码与关键业务字段。\n" +
	"- 不要输出 Markdown 围栏（不要使用三个反引号）；直接输出 JS 源代码，不要任何额外解释。"

const generateCaseDataSystem = `你是接口自动化测试平台的「测试数据行生成器」。
- 输入包含测试数据表结构、字段约束、已有行上下文、以及希望生成的数据行数量与场景描述。
- 输出**必须是单个 JSON 对象**，结构如下：
{
  "rows": [
    { "<列 key>": "<单元格值>" }
  ]
}
- 尽量生成贴近业务语义、便于接口测试复用的数据行；不要输出表结构之外的列。
- 严禁输出 JSON 之外的任何文字。`

const analyzeFailureSystem = `你是接口自动化测试平台的「测试失败分析助手」。

【输入说明】
- 用户消息中会附带一段 JSON 上下文，可能包含以下字段（任何一项都可能缺失，需按字段名识别）：
  - run：本次运行的元信息（id、name、status、scenarioId、environmentId 等）。
  - case：单接口运行时的接口模板摘要（method、path、name、source）。
  - scenario：场景运行时的场景摘要（id、name）。
  - result：单接口运行结果（包含 requestSnapshot、responseSnapshot、assertions、error）。
  - stepResults：场景运行的逐步结果数组，每项包含 step（stepSeq/name/stepType/testCaseId）、result（同上 result 结构）、stepErrors。
  - assertionFailures：扁平的断言失败列表，每项含 assertion 类型、name、expected、actual、message。
- requestSnapshot 中至少有 method/url/headers/body，responseSnapshot 至少有 statusCode/headers/body；body 字段可能是字符串或对象。
- 上下文是只读证据，请勿编造未出现的字段。

【输出要求】
- **必须**严格使用以下三段中文 markdown 结构（标题原样输出，不要加编号或额外标题）：
  - "## 失败原因"：1-3 段，定位最可能的根因（HTTP 状态码异常、断言不匹配、依赖步骤错误、参数缺失、鉴权失败等），并指出关键证据所在字段。
  - "## 关键证据"：要点列表，逐条引用上下文中真实出现的字段值（例如 ` + "`" + `responseSnapshot.statusCode = 500` + "`" + `、断言 ` + "`" + `body.code` + "`" + ` 期望 0 实际 1）。**禁止虚构字段或值**。
  - "## 修复建议"：要点列表，给出可立即执行的下一步动作（修改请求参数、调整断言、修正前置步骤、检查环境变量等），并按优先级排序。
- 全程使用中文；可使用 markdown 列表、行内代码（反引号）、表格，但不要输出整段 JSON 或冗长堆栈。
- 若上下文为空或不足以推断，请在「失败原因」段直接说明"上下文不足，建议补充：xxx"，不要编造。`

const analyzeSpecChangesSystem = `你是接口自动化测试平台的「OpenAPI/Swagger 变更影响分析助手」。

【输入说明】
- 用户消息中会附带一段 JSON 上下文，可能包含以下字段：
  - service：当前服务摘要（id、name）。
  - prevSpec / currSpec：前后两次 spec 元信息（version、contentHash、createdAt、title、apiVersion）。
  - diff：结构化 diff，含 ` + "`" + `addedEndpoints` + "`" + `（新增 endpoint 列表，每项 method+path+summary）、` + "`" + `removedEndpoints` + "`" + `（删除）、` + "`" + `modifiedEndpoints` + "`" + `（每项含 method、path、changes 字段，每个 change 含 fieldPath、kind=add/remove/change、before、after）。
  - affectedTemplates：当前服务下相关的 ` + "`" + `test_cases` + "`" + ` 摘要数组，每项含 caseId、name、method、path、source（auto/manual/derived）。
  - affectedScenarioSteps：当前服务下相关的场景步骤摘要数组，每项含 scenarioId、scenarioName、stepSeq、stepName、testCaseId、method、path。
- diff 字段的 fieldPath 形如 ` + "`" + `parameters[0].schema.type` + "`" + ` 或 ` + "`" + `requestBody.content.application/json.schema.properties.userId.type` + "`" + `。
- 所有清单都是真实证据，请勿编造未出现的 endpoint、模板或步骤。

【输出要求】
- **必须**严格使用以下四段中文 markdown 结构（标题原样输出）：
  - "## 变更概览"：1-2 段，总结新增/删除/修改 endpoint 的数量、显著的高风险变更（必填字段变化、删除 endpoint、鉴权方式变化等）。
  - "## 详细变更清单"：使用三个二级要点（"### 新增"、"### 删除"、"### 修改"）分组列出 endpoint；修改组下逐 endpoint 列出受影响字段、变更类型、before→after 摘要，没有变更时写"无"。
  - "## 对现有资产影响"：分两小节"### 接口模板"和"### 场景步骤"，分别列出 affectedTemplates 与 affectedScenarioSteps 中受 diff 影响的条目并指出受影响原因（路径被删除、字段类型变化、必填字段新增等）；若上下文显示无受影响条目则写"无显著影响"。
  - "## 建议动作"：按 endpoint 给出优先级排序的动作清单，每条动作必须明确归类为以下三种之一：` + "`" + `继续可用` + "`" + `（无需调整）、` + "`" + `需调整` + "`" + `（说明需要修改的字段或脚本）、` + "`" + `应废弃` + "`" + `（建议下线或重写）。
- 全程使用中文；不要输出整段 JSON 或重复粘贴 diff，要点应概括并引用关键 fieldPath。
- 若 diff 为空（spec 内容相同）请在"变更概览"直接说明"无结构化差异"，并在"建议动作"建议保留现状。`

const rawSystem = `你是接口自动化测试平台的通用 AI 助手，用中文给出简洁、可执行的回答。`

// extractParsedJSON tries to pull a JSON object from a free-form model response.
// It returns the parsed JSON bytes (stable, indented) and an empty parseWarnings on success.
func extractParsedJSON(text string) (json.RawMessage, string) {
	body := stripCodeFence(strings.TrimSpace(text))
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, "模型返回内容为空"
	}

	if msg := tryParseJSON(body); msg != nil {
		return msg, ""
	}

	if start := strings.IndexAny(body, "{["); start >= 0 {
		end := matchingClose(body, start)
		if end > start {
			candidate := body[start : end+1]
			if msg := tryParseJSON(candidate); msg != nil {
				return msg, ""
			}
		}
	}
	return nil, "未能从模型回复中解析出 JSON 结构，已展示原文。"
}

func tryParseJSON(body string) json.RawMessage {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return nil
	}
	indented, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil
	}
	return json.RawMessage(indented)
}

func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	rest := strings.TrimPrefix(s, "```")
	if newline := strings.IndexByte(rest, '\n'); newline >= 0 {
		rest = rest[newline+1:]
	}
	if idx := strings.LastIndex(rest, "```"); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

func matchingClose(s string, start int) int {
	if start < 0 || start >= len(s) {
		return -1
	}
	open := s[start]
	var close byte
	switch open {
	case '{':
		close = '}'
	case '[':
		close = ']'
	default:
		return -1
	}

	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
