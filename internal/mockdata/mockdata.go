// Package mockdata expands runtime mock-data tokens of the form
// `{{$mock.helper}}` and `{{$mock.helper(args...)}}` inside arbitrary
// strings. Each occurrence is evaluated independently so multiple uses of
// the same helper in one request produce different values.
//
// The package is intentionally dependency-light: it relies on `gofakeit`
// (already used by `internal/sampler`) for realistic random data, and on
// the standard library for time/number helpers. It is shared between the
// HTTP runner and the Mock Server response template engine.
package mockdata

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
)

// tokenPattern matches a single {{$mock.helper}} or {{$mock.helper(args)}}
// occurrence. Whitespace inside the braces and around the dot is tolerated.
//
// Helper names are restricted to letters/digits/underscores; the optional
// argument list is captured as raw text (no nested parentheses) and parsed
// later by parseArgs so quoted strings can carry commas or spaces.
var tokenPattern = regexp.MustCompile(`\{\{\s*\$mock\s*\.\s*([A-Za-z][A-Za-z0-9_]*)\s*(?:\(([^()]*)\))?\s*\}\}`)

// HasToken reports whether s contains at least one mock-data token. Callers
// can use this as a fast pre-check before allocating temporary buffers.
func HasToken(s string) bool {
	return strings.Contains(s, "$mock") && tokenPattern.MatchString(s)
}

// Expand replaces every `{{$mock.*}}` occurrence in s with a freshly
// generated value. Unknown helper names are left as-is so the surrounding
// renderer can surface them; this matches the behaviour of `{{varName}}`
// placeholders that fail to resolve.
func Expand(s string) string {
	if !HasToken(s) {
		return s
	}
	return tokenPattern.ReplaceAllStringFunc(s, func(match string) string {
		groups := tokenPattern.FindStringSubmatch(match)
		helper := groups[1]
		args := parseArgs(groups[2])
		value, ok := Eval(helper, args)
		if !ok {
			return match
		}
		return value
	})
}

// Eval resolves a single helper invocation. Helper names are matched
// case-insensitively. The bool return is false when the helper is unknown
// so callers can distinguish "no replacement" from "produced empty string".
func Eval(name string, args []string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "uuid":
		return uuid.NewString(), true
	case "now":
		return formatTime(time.Now().UTC(), firstArg(args, time.RFC3339)), true
	case "timestamp":
		return timestampHelper(args), true
	case "int", "integer":
		return intHelper(args), true
	case "float", "number":
		return floatHelper(args), true
	case "bool", "boolean":
		return strconv.FormatBool(gofakeit.Bool()), true
	case "string":
		return stringHelper(args), true
	case "word":
		return gofakeit.Word(), true
	case "sentence":
		return chineseSentenceHelper(args), true
	case "name", "fullname":
		return chineseFullName(), true
	case "firstname":
		return chineseFirstName(), true
	case "lastname", "surname":
		return chineseLastName(), true
	case "email", "mail":
		return gofakeit.Email(), true
	case "phone", "mobile":
		return chineseMobilePhone(), true
	case "url", "uri":
		return gofakeit.URL(), true
	case "ipv4", "ip":
		return gofakeit.IPv4Address(), true
	case "ipv6":
		return gofakeit.IPv6Address(), true
	case "city":
		return gofakeit.City(), true
	case "country":
		return gofakeit.Country(), true
	case "address":
		return chineseAddress(), true
	case "company", "organization", "org":
		return gofakeit.Company(), true
	case "color", "colour":
		return gofakeit.Color(), true
	case "date":
		return formatTime(gofakeit.Date().UTC(), firstArg(args, "2006-01-02")), true
	case "datetime":
		return formatTime(gofakeit.Date().UTC(), firstArg(args, time.RFC3339)), true
	case "pick", "oneof", "choice":
		return pickHelper(args), true
	}
	return "", false
}

// ── Helper implementations ─────────────────────────────────────────────

func timestampHelper(args []string) string {
	now := time.Now().UTC()
	unit := strings.ToLower(strings.TrimSpace(firstArg(args, "s")))
	switch unit {
	case "ms", "milli", "millis", "millisecond", "milliseconds":
		return strconv.FormatInt(now.UnixMilli(), 10)
	case "ns", "nano", "nanos", "nanosecond", "nanoseconds":
		return strconv.FormatInt(now.UnixNano(), 10)
	default:
		return strconv.FormatInt(now.Unix(), 10)
	}
}

func intHelper(args []string) string {
	min, max := 1, 100
	switch len(args) {
	case 0:
	case 1:
		max = intArg(args, 0, max)
	default:
		min = intArg(args, 0, min)
		max = intArg(args, 1, max)
	}
	if max < min {
		min, max = max, min
	}
	return strconv.Itoa(gofakeit.IntRange(min, max))
}

func floatHelper(args []string) string {
	min, max := 0.0, 100.0
	prec := 2
	switch len(args) {
	case 0:
	case 1:
		max = floatArg(args, 0, max)
	case 2:
		min = floatArg(args, 0, min)
		max = floatArg(args, 1, max)
	default:
		min = floatArg(args, 0, min)
		max = floatArg(args, 1, max)
		prec = intArg(args, 2, prec)
	}
	if max < min {
		min, max = max, min
	}
	if prec < 0 {
		prec = 0
	}
	value := gofakeit.Float64Range(min, max)
	return strconv.FormatFloat(value, 'f', prec, 64)
}

func stringHelper(args []string) string {
	n := intArg(args, 0, 8)
	if n < 1 {
		n = 1
	}
	if n > 4096 {
		n = 4096
	}
	return gofakeit.LetterN(uint(n))
}

func chineseSentenceHelper(args []string) string {
	count := intArg(args, 0, 8)
	if count < 1 {
		count = 1
	}
	if count > 64 {
		count = 64
	}
	words := make([]string, 0, count)
	for i := 0; i < count; i++ {
		words = append(words, chineseSentenceWords[gofakeit.IntRange(0, len(chineseSentenceWords)-1)])
	}
	return strings.Join(words, "") + "。"
}

func chineseFullName() string {
	return chineseLastName() + chineseFirstName()
}

func chineseFirstName() string {
	length := gofakeit.IntRange(1, 2)
	var name strings.Builder
	for i := 0; i < length; i++ {
		name.WriteString(chineseGivenNameChars[gofakeit.IntRange(0, len(chineseGivenNameChars)-1)])
	}
	return name.String()
}

func chineseLastName() string {
	return chineseSurnames[gofakeit.IntRange(0, len(chineseSurnames)-1)]
}

func chineseMobilePhone() string {
	var b strings.Builder
	b.WriteByte('1')
	b.WriteString(strconv.Itoa(gofakeit.IntRange(3, 9)))
	for i := 0; i < 9; i++ {
		b.WriteString(strconv.Itoa(gofakeit.IntRange(0, 9)))
	}
	return b.String()
}

func chineseAddress() string {
	prefix := chineseAddressPrefixes[gofakeit.IntRange(0, len(chineseAddressPrefixes)-1)]
	road := chineseRoads[gofakeit.IntRange(0, len(chineseRoads)-1)]
	number := gofakeit.IntRange(1, 999)
	return prefix + road + strconv.Itoa(number) + "号"
}

func pickHelper(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[gofakeit.IntRange(0, len(args)-1)]
}

var chineseSurnames = []string{
	"赵", "钱", "孙", "李", "周", "吴", "郑", "王", "冯", "陈",
	"褚", "卫", "蒋", "沈", "韩", "杨", "朱", "秦", "尤", "许",
	"何", "吕", "施", "张", "孔", "曹", "严", "华", "金", "魏",
	"陶", "姜", "戚", "谢", "邹", "喻", "柏", "水", "窦", "章",
	"云", "苏", "潘", "葛", "奚", "范", "彭", "郎", "鲁", "韦",
	"昌", "马", "苗", "凤", "花", "方", "俞", "任", "袁", "柳",
}

var chineseGivenNameChars = []string{
	"伟", "刚", "勇", "毅", "俊", "峰", "强", "军", "平", "东",
	"文", "辉", "力", "明", "永", "健", "世", "广", "志", "义",
	"兴", "良", "海", "山", "仁", "波", "宁", "贵", "福", "生",
	"龙", "元", "全", "国", "胜", "学", "祥", "才", "发", "武",
	"新", "利", "清", "飞", "彬", "富", "顺", "信", "子", "杰",
	"涛", "昌", "成", "康", "星", "光", "天", "达", "安", "岩",
	"欣", "雅", "雨", "婷", "静", "敏", "丽", "娜", "洁", "琪",
	"琳", "雪", "佳", "慧", "宁", "悦", "怡", "晨", "思", "涵",
}

var chineseAddressPrefixes = []string{
	"北京市朝阳区", "上海市浦东新区", "广东省广州市天河区", "广东省深圳市南山区", "浙江省杭州市西湖区",
	"江苏省南京市鼓楼区", "四川省成都市锦江区", "湖北省武汉市武昌区", "湖南省长沙市岳麓区", "福建省厦门市思明区",
}

var chineseRoads = []string{
	"人民路", "建设路", "解放路", "中山路", "和平路", "长江路", "文化路", "科技路", "新华路", "幸福路",
}

var chineseSentenceWords = []string{
	"用户",
	"系统",
	"平台",
	"接口",
	"数据",
	"流程",
	"请求",
	"响应",
	"服务",
	"场景",
	"自动",
	"生成",
	"校验",
	"记录",
	"同步",
	"更新",
	"创建",
	"查询",
	"提交",
	"返回",
	"成功",
	"稳定",
	"高效",
	"安全",
	"完整",
	"清晰",
	"快速",
	"准确",
	"持续",
	"灵活",
}

// ── Argument parsing helpers ───────────────────────────────────────────

func firstArg(args []string, fallback string) string {
	if len(args) == 0 {
		return fallback
	}
	if v := strings.TrimSpace(args[0]); v != "" {
		return v
	}
	return fallback
}

func intArg(args []string, idx int, fallback int) int {
	if idx >= len(args) {
		return fallback
	}
	v := strings.TrimSpace(args[idx])
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatArg(args []string, idx int, fallback float64) float64 {
	if idx >= len(args) {
		return fallback
	}
	v := strings.TrimSpace(args[idx])
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

// formatTime renders t with a Go time layout, falling back to RFC3339 when
// the layout is empty or invalid (Go's Format never errors but a malformed
// layout produces unhelpful output; we keep it permissive on purpose).
func formatTime(t time.Time, layout string) string {
	layout = strings.TrimSpace(layout)
	if layout == "" {
		layout = time.RFC3339
	}
	return t.Format(layout)
}

// parseArgs splits a raw argument string by commas while respecting single
// and double quote pairs. Inside a quoted segment commas, leading/trailing
// whitespace and surrounding quotes are preserved verbatim. Outside quotes
// each segment is trimmed of surrounding whitespace.
//
// Examples:
//
//	""                       → []
//	"a, b, c"                → ["a", "b", "c"]
//	"'hello, world', 5"      → ["hello, world", "5"]
//	"\"a,b\", \"c\""         → ["a,b", "c"]
//	"  '  spaced  '  ,  x"   → ["  spaced  ", "x"] (only quoted run kept)
func parseArgs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var (
		out     []string
		current strings.Builder
		quote   rune
	)
	flush := func() {
		out = append(out, strings.TrimSpace(current.String()))
		current.Reset()
	}
	for _, r := range raw {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	// Strip empty trailing segments produced by stray commas, but keep
	// internal empties so callers can pass intentional empty arguments.
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// ListHelpers returns the canonical helper names, sorted for deterministic
// presentation. It is exported so the admin UI / docs surface can render a
// single source-of-truth list.
func ListHelpers() []HelperInfo {
	return []HelperInfo{
		{Name: "uuid", Example: "{{$mock.uuid}}", Description: "随机 UUID v4"},
		{Name: "now", Example: "{{$mock.now}} / {{$mock.now('2006-01-02 15:04:05')}}", Description: "当前时间，可选 Go time 布局"},
		{Name: "timestamp", Example: "{{$mock.timestamp}} / {{$mock.timestamp(ms)}}", Description: "当前 Unix 时间戳，单位 s/ms/ns"},
		{Name: "int", Example: "{{$mock.int}} / {{$mock.int(1,100)}}", Description: "随机整数，默认范围 [1,100]"},
		{Name: "float", Example: "{{$mock.float}} / {{$mock.float(0,1,4)}}", Description: "随机浮点，默认范围 [0,100] 保留 2 位"},
		{Name: "bool", Example: "{{$mock.bool}}", Description: "随机布尔值"},
		{Name: "string", Example: "{{$mock.string(8)}}", Description: "指定长度的随机字母字符串"},
		{Name: "word", Example: "{{$mock.word}}", Description: "随机英文单词"},
		{Name: "sentence", Example: "{{$mock.sentence(6)}}", Description: "由 N 个中文词语组成的句子"},
		{Name: "name", Example: "{{$mock.name}}", Description: "随机中文姓名"},
		{Name: "firstName", Example: "{{$mock.firstName}}", Description: "随机中文名"},
		{Name: "lastName", Example: "{{$mock.lastName}}", Description: "随机中文姓"},
		{Name: "email", Example: "{{$mock.email}}", Description: "随机邮箱"},
		{Name: "phone", Example: "{{$mock.phone}}", Description: "随机中国手机号"},
		{Name: "url", Example: "{{$mock.url}}", Description: "随机 URL"},
		{Name: "ipv4", Example: "{{$mock.ipv4}}", Description: "随机 IPv4 地址"},
		{Name: "ipv6", Example: "{{$mock.ipv6}}", Description: "随机 IPv6 地址"},
		{Name: "city", Example: "{{$mock.city}}", Description: "随机城市名"},
		{Name: "country", Example: "{{$mock.country}}", Description: "随机国家名"},
		{Name: "address", Example: "{{$mock.address}}", Description: "随机中文地址"},
		{Name: "company", Example: "{{$mock.company}}", Description: "随机公司名"},
		{Name: "color", Example: "{{$mock.color}}", Description: "随机颜色名"},
		{Name: "date", Example: "{{$mock.date}}", Description: "随机日期 yyyy-MM-dd（可自定义布局）"},
		{Name: "dateTime", Example: "{{$mock.dateTime}}", Description: "随机日期时间 RFC3339（可自定义布局）"},
		{Name: "pick", Example: "{{$mock.pick(a,b,c)}}", Description: "从参数列表随机挑一个，oneOf 是别名"},
	}
}

// HelperInfo describes a single mock helper for documentation surfaces.
// Field tags use lowerCamelCase to match the management UI contract.
type HelperInfo struct {
	Name        string `json:"name"`
	Example     string `json:"example"`
	Description string `json:"description"`
}

// MustHelperInfo is a convenience accessor used by template callers that
// want to look up a single helper's documentation; it returns false when
// the name is unknown.
func MustHelperInfo(name string) (HelperInfo, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, h := range ListHelpers() {
		if strings.ToLower(h.Name) == target {
			return h, true
		}
	}
	return HelperInfo{}, false
}

// ParseArgsForTest exposes parseArgs for the package tests; we keep it
// behind an explicit name so callers do not casually depend on the format.
func ParseArgsForTest(raw string) []string { return parseArgs(raw) }
