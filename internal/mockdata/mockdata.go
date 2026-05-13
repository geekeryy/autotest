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
	"fmt"
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
	case "idcard", "id_card":
		return chineseIDCard(), true
	case "platenumber", "plate_number", "plate":
		return chinesePlateNumber(), true
	case "bankcard", "bank_card", "bankcardnumber":
		return bankCard(args), true
	case "unifiedsocialcreditcode", "unified_social_credit_code", "uscc":
		return unifiedSocialCreditCode(), true
	case "sku":
		return skuHelper(args), true
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

// ── 业务身份与编码 helper ──────────────────────────────────────────────
//
// 这些 helper 仅生成「形式合法」的样本数据用于自动化测试场景，**不**对应
// 任何真实人员、车辆或法人实体；校验位算法严格遵循国家标准（GB 11643 /
// Luhn / GB 32100），便于业务系统在前端表单或服务端做格式校验时通过。

// chineseIDCard 生成一个 18 位中国二代身份证号样本：
//   - 前 6 位行政区划码：从 chineseIDCardAreaCodes 中随机取一项；
//   - 中间 8 位生日：1960-01-01 到 2010-12-31 之间合法日期；
//   - 第 15-17 位顺序码：随机 000-999；
//   - 第 18 位校验位：按 GB 11643-1999 ISO 7064 MOD 11-2 算法。
func chineseIDCard() string {
	area := chineseIDCardAreaCodes[gofakeit.IntRange(0, len(chineseIDCardAreaCodes)-1)]
	year := gofakeit.IntRange(1960, 2010)
	month := gofakeit.IntRange(1, 12)
	day := gofakeit.IntRange(1, daysInMonth(year, month))
	birthday := fmt.Sprintf("%04d%02d%02d", year, month, day)
	sequence := fmt.Sprintf("%03d", gofakeit.IntRange(0, 999))
	body := area + birthday + sequence
	return body + idCardCheckDigit(body)
}

// idCardWeights 与 idCardCheckChars 是 GB 11643-1999 中规定的常量。
var (
	idCardWeights    = []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	idCardCheckChars = []string{"1", "0", "X", "9", "8", "7", "6", "5", "4", "3", "2"}
)

func idCardCheckDigit(body17 string) string {
	sum := 0
	for i := 0; i < 17; i++ {
		// 严格断言每位都是数字，避免越界 panic 在生产环境难以排查。
		digit := int(body17[i] - '0')
		if digit < 0 || digit > 9 {
			return "0"
		}
		sum += digit * idCardWeights[i]
	}
	return idCardCheckChars[sum%11]
}

// daysInMonth 不依赖完整时区计算，避免边界月份生成无效日期（如 2 月 30）。
func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
			return 29
		}
		return 28
	}
	return 28
}

// chinesePlateNumber 生成一个普通中国车牌号样本：
// 「省份简称 + · + 字母 1 位 + 字母数字 5 位」，例如 `京·B12AB3`。
//
// 不覆盖新能源（绿牌）、警车、军车等特殊车牌；如需扩展请添加专用 helper。
func chinesePlateNumber() string {
	prefix := chinesePlateProvinces[gofakeit.IntRange(0, len(chinesePlateProvinces)-1)]
	letter := string('A' + byte(gofakeit.IntRange(0, 25)))
	tail := make([]byte, 5)
	for i := range tail {
		// 0-9 占索引 0-9, A-Z 占索引 10-35。
		idx := gofakeit.IntRange(0, 35)
		if idx < 10 {
			tail[i] = byte('0' + idx)
		} else {
			tail[i] = byte('A' + idx - 10)
		}
	}
	return prefix + "·" + letter + string(tail)
}

// bankCard 生成一个 16-19 位 Luhn 合法的银行卡号：
//   - 长度可由 args[0] 指定（限制在 [16,19]，默认 19）；
//   - 首位从 4/5/6 中随机选取（覆盖最常见的 Visa/Master/UnionPay 起始位）；
//   - 中间位随机数字；
//   - 末位为 Luhn 校验位。
func bankCard(args []string) string {
	length := intArg(args, 0, 19)
	if length < 16 {
		length = 16
	}
	if length > 19 {
		length = 19
	}
	prefixOptions := []byte{'4', '5', '6'}
	digits := make([]byte, length)
	digits[0] = prefixOptions[gofakeit.IntRange(0, len(prefixOptions)-1)]
	for i := 1; i < length-1; i++ {
		digits[i] = byte('0' + gofakeit.IntRange(0, 9))
	}
	digits[length-1] = byte('0' + luhnCheckDigit(digits[:length-1]))
	return string(digits)
}

// luhnCheckDigit 计算给定数字串的 Luhn 校验位（0-9）。
// 要求 prefix 的每个字节都在 '0'-'9' 之间。
func luhnCheckDigit(prefix []byte) int {
	sum := 0
	// 末位为校验位，prefix 长度即校验位之前的位数；从右往左每隔一位 ×2。
	double := true
	for i := len(prefix) - 1; i >= 0; i-- {
		d := int(prefix[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return (10 - (sum % 10)) % 10
}

// unifiedSocialCreditCode 生成 18 位「统一社会信用代码」样本：
//   - 第 1 位登记管理部门码（GB 32100，常用 1/5/9 之一）；
//   - 第 2 位机构类别码（按管理部门派生，0-9/A-Z）；
//   - 第 3-8 位行政区划码（取自 chineseIDCardAreaCodes 的 6 位前缀）；
//   - 第 9-17 位主体识别码（业务上由组织机构代码派生，本 helper 用 9 位
//     随机字母数字代替）；
//   - 第 18 位校验位（GB 32100 ISO 7064 MOD 31-3）。
func unifiedSocialCreditCode() string {
	// 登记管理部门 + 机构类别常用组合：
	//   91 — 工商 / 企业, 51 — 民政 / 社会团体, 92 — 工商 / 个体工商户, 12 — 编办 / 机关。
	deptCategoryPairs := []string{"91", "92", "93", "51", "12", "11", "31", "52", "53", "59"}
	pair := deptCategoryPairs[gofakeit.IntRange(0, len(deptCategoryPairs)-1)]
	area := chineseIDCardAreaCodes[gofakeit.IntRange(0, len(chineseIDCardAreaCodes)-1)]
	body := pair + area
	for i := 0; i < 9; i++ {
		body += string(usccCharSet[gofakeit.IntRange(0, len(usccCharSet)-1)])
	}
	return body + usccCheckDigit(body)
}

// usccCharSet：GB 32100 允许的字符集（数字 + 大写字母去掉 I/O/S/V/Z），共 31 个。
var usccCharSet = []byte("0123456789ABCDEFGHJKLMNPQRTUWXY")

var usccWeights = []int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}

// usccCheckDigit 计算 17 位主体码的校验位（GB 32100 ISO 7064 MOD 31-3）。
func usccCheckDigit(body17 string) string {
	indexOf := func(b byte) int {
		for i, c := range usccCharSet {
			if c == b {
				return i
			}
		}
		return -1
	}
	sum := 0
	for i := 0; i < 17 && i < len(body17); i++ {
		val := indexOf(body17[i])
		if val < 0 {
			return "0"
		}
		sum += val * usccWeights[i]
	}
	check := 31 - (sum % 31)
	if check == 31 {
		check = 0
	}
	return string(usccCharSet[check])
}

// skuHelper 生成 SKU 样本，默认 `[A-Z]{2}-\d{6}`；
//   - args[0]：可选总长度（含分隔符 `-`），范围 [4, 32]；超出则截到默认长度。
//
// 长度 < 4 时无法保证 `<字母前缀>-<数字尾缀>` 结构，会被忽略并回落到默认。
func skuHelper(args []string) string {
	length := intArg(args, 0, 9)
	if length < 4 || length > 32 {
		length = 9
	}
	letters := length / 3
	if letters < 1 {
		letters = 1
	}
	if letters > 6 {
		letters = 6
	}
	digits := length - letters - 1
	if digits < 1 {
		digits = 1
	}
	var b strings.Builder
	for i := 0; i < letters; i++ {
		b.WriteByte(byte('A' + gofakeit.IntRange(0, 25)))
	}
	b.WriteByte('-')
	for i := 0; i < digits; i++ {
		b.WriteByte(byte('0' + gofakeit.IntRange(0, 9)))
	}
	return b.String()
}

// chineseIDCardAreaCodes：常用省/市/区 6 位行政区划码（来自 GB/T 2260）。
// 仅保留覆盖度高的城市样本，避免硬编码全国所有区县；测试场景下足够。
var chineseIDCardAreaCodes = []string{
	"110101", "110102", "110105", "110106", "110108", // 北京 东城 / 西城 / 朝阳 / 丰台 / 海淀
	"120101", "120102", "120103", "120104", "120105", // 天津 和平 / 河东 / 河西 / 南开 / 河北
	"310101", "310104", "310105", "310106", "310115", // 上海 黄浦 / 徐汇 / 长宁 / 静安 / 浦东
	"500101", "500103", "500104", "500106", "500108", // 重庆 万州 / 渝中 / 大渡口 / 沙坪坝 / 南岸
	"320102", "320104", "320106", "320111", "320114", // 南京 玄武 / 秦淮 / 鼓楼 / 浦口 / 雨花
	"330102", "330104", "330105", "330106", "330108", // 杭州 上城 / 江干 / 拱墅 / 西湖 / 滨江
	"440103", "440104", "440105", "440106", "440111", // 广州 荔湾 / 越秀 / 海珠 / 天河 / 白云
	"440303", "440304", "440305", "440306", "440307", // 深圳 罗湖 / 福田 / 南山 / 宝安 / 龙岗
	"510104", "510105", "510106", "510107", "510108", // 成都 锦江 / 金牛 / 武侯 / 成华 / 青羊
	"420102", "420103", "420104", "420105", "420106", // 武汉 江岸 / 江汉 / 硚口 / 汉阳 / 武昌
}

// chinesePlateProvinces：中国大陆 31 个省/直辖市/自治区车牌简称。
var chinesePlateProvinces = []string{
	"京", "津", "沪", "渝", "冀", "豫", "云", "辽", "黑", "湘",
	"皖", "鲁", "新", "苏", "浙", "赣", "鄂", "桂", "甘", "晋",
	"蒙", "陕", "吉", "闽", "贵", "粤", "青", "藏", "川", "宁", "琼",
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
		{Name: "set", Example: "{{$mock.set.<key>}} / {{$mock.set.<key>[0]}}", Description: "项目「命名值集合」；候选值可为 JSON 字符串/数字/布尔/null/对象/数组，占位输出紧凑 JSON（纯字符串取原文），详见平台资源命名值集合"},
		{Name: "idCard", Example: "{{$mock.idCard}}", Description: "中国二代身份证 18 位（含 GB 11643 校验位，仅供测试用）"},
		{Name: "plateNumber", Example: "{{$mock.plateNumber}}", Description: "中国车牌号（省份简称 + · + 1 字母 + 5 位字母数字）"},
		{Name: "bankCard", Example: "{{$mock.bankCard}} / {{$mock.bankCard(16)}}", Description: "16-19 位 Luhn 合法银行卡号，默认 19 位"},
		{Name: "unifiedSocialCreditCode", Example: "{{$mock.unifiedSocialCreditCode}}", Description: "统一社会信用代码 18 位（GB 32100，校验位合法）"},
		{Name: "sku", Example: "{{$mock.sku}} / {{$mock.sku(12)}}", Description: "SKU 编号，默认 `[A-Z]{2}-\\d{6}`，可指定总长"},
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
