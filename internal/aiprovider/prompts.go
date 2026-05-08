package aiprovider

import (
	"encoding/json"
	"strings"

	"autotest/internal/aiprovider/client"
)

// buildMessages constructs the system + user messages for an action.
// The structured `context` (as raw JSON) lets the frontend ship arbitrary metadata.
// When systemOverride is non-empty it replaces the built-in system prompt.
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
	default:
		system = rawSystem
	}
	if systemOverride != "" {
		system = systemOverride
	}

	user := strings.TrimSpace(prompt)
	if action == ActionGenerateAssertion && user != "" {
		user = "以下为测试者应校验的意图描述：\n" + user
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
	if user == "" {
		user = "请根据系统说明开始生成。"
	}

	return []client.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, jsonOnly
}

const generateParamsSystem = `你是接口自动化测试平台的「请求参数生成器」。
- 输入会包含一段 OpenAPI/Swagger 描述（包括 method、path、parameters、requestBody schema 等）以及当前用户已有的参数。
- 输出**必须是单个 JSON 对象**，结构如下：
{
  "path": { "<参数名>": "<取值>" },
  "query": { "<参数名>": "<取值>" },
  "headers": { "<参数名>": "<取值>" },
  "body": <按 schema 生成的合法请求体>
}
- 严禁输出任何解释文字或 Markdown，只输出 JSON。
- 优先使用 schema 中的 example、default、enum；其次按 format/字段名启发式生成真实仿真值。
- 参数名严格保持与输入一致（区分大小写）。
- 不需要的分区可省略，但顶层至少包含一个非空字段。`

const generateAssertionSystem = "你是接口自动化测试平台的「断言脚本生成器」。\n" +
	"- 用户在弹窗中必须提供非空的测试意图（中文）；用户消息中会包含该意图说明。\n" +
	"- 输入还包含响应快照（status、headers、body）等结构化上下文。\n" +
	"- 输出 Postman 风格 JavaScript 断言代码，可直接粘贴到平台脚本断言编辑器中。\n" +
	"- 可用 API：pm.test('名称', () => { ... })、pm.response.code、pm.response.json()、pm.expect(...).to.equal(...) 等。\n" +
	"- 至少包含一个 pm.test，覆盖响应状态码与关键业务字段。\n" +
	"- 不要输出 Markdown 围栏（不要使用三个反引号）；直接输出 JS 源代码，不要任何额外解释。"

const generateCaseDataSystem = `你是接口自动化测试平台的「测试用例数据生成器」。
- 输入包含接口的请求 schema、字段约束、以及希望生成的用例数量与场景描述。
- 输出**必须是单个 JSON 对象**，结构如下：
{
  "cases": [
    { "name": "<用例名称，简短中文>", "body": <符合 schema 的请求体>, "notes": "<可选说明>" }
  ]
}
- 至少覆盖正向用例与典型边界/异常用例（如必填缺失、枚举值边界、字符串极限长度）。
- 严禁输出 JSON 之外的任何文字。`

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
