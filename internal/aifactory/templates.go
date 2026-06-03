package aifactory

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// Locale 定义数据生成的区域设置。
type Locale string

const (
	LocaleZhCN Locale = "zh_CN"
	LocaleEnUS Locale = "en_US"
)

// 默认 locale
const defaultLocale = LocaleZhCN

// NormalizeLocale 规范化 locale 字符串，无效值回退到默认。
func NormalizeLocale(raw string) Locale {
	switch strings.TrimSpace(strings.ToLower(strings.ReplaceAll(raw, "-", "_"))) {
	case "zh_cn", "zhcn", "zh":
		return LocaleZhCN
	case "en_us", "enus", "en":
		return LocaleEnUS
	default:
		return defaultLocale
	}
}

// DataTemplate 根据语义类型和 locale 生成测试数据值。
type DataTemplate func(locale Locale, scenario string) any

// templateRegistry 注册了每种语义类型对应的模板函数。
var templateRegistry = map[SemanticType]DataTemplate{
	SemanticName:        genName,
	SemanticFirstName:   genFirstName,
	SemanticLastName:    genLastName,
	SemanticUsername:    genUsername,
	SemanticEmail:       genEmail,
	SemanticPhone:       genPhone,
	SemanticAddress:     genAddress,
	SemanticCity:        genCity,
	SemanticProvince:    genProvince,
	SemanticCountry:     genCountry,
	SemanticIDCard:      genIDCard,
	SemanticAge:         genAge,
	SemanticGender:      genGender,
	SemanticPrice:       genPrice,
	SemanticTitle:       genTitle,
	SemanticDescription: genDescription,
	SemanticURL:         genURL,
	SemanticAvatar:      genAvatar,
	SemanticCompany:     genCompany,
	SemanticTimestamp:   genTimestamp,
	SemanticBoolean:     genBool,
	SemanticInteger:     genInteger,
	SemanticNumber:      genNumber,
	SemanticPassword:    genPassword,
	SemanticStatus:      genStatus,
	SemanticColor:       genColor,
	SemanticIP:          genIP,
	SemanticUUID:        genUUID,
	SemanticGeneric:     genGeneric,
}

// GenerateValue 根据语义类型、locale 和场景模式生成一个值。
func GenerateValue(sem FieldSemantics, locale Locale, scenario string) any {
	// 如果有 enum 约束，优先从 enum 中选取
	if len(sem.EnumValues) > 0 {
		return pickEnum(sem.EnumValues, scenario)
	}

	// 查找模板
	tpl, ok := templateRegistry[sem.Type]
	if !ok {
		tpl = genGeneric
	}

	value := tpl(locale, scenario)

	// 根据场景模式调整值
	value = applyScenarioAdjustment(value, sem, scenario)
	value = applyLengthConstraints(value, sem)

	return value
}

// pickEnum 从 enum 值中选取，boundary 模式取边界值。
func pickEnum(enumValues []any, scenario string) any {
	if len(enumValues) == 0 {
		return nil
	}
	if scenario == "boundary" && len(enumValues) >= 2 {
		if gofakeit.Bool() {
			return enumValues[0]
		}
		return enumValues[len(enumValues)-1]
	}
	return enumValues[gofakeit.IntRange(0, len(enumValues)-1)]
}

// applyScenarioAdjustment 根据场景模式调整生成的值。
func applyScenarioAdjustment(value any, sem FieldSemantics, scenario string) any {
	switch scenario {
	case "boundary":
		return applyBoundary(value, sem)
	case "stress":
		return applyStress(value, sem)
	default:
		return value
	}
}

// applyBoundary 对边界场景做调整。
func applyBoundary(value any, sem FieldSemantics) any {
	switch sem.Type {
	case SemanticAge:
		// 年龄边界：0 或 150
		if gofakeit.Bool() {
			return 0
		}
		return 150
	case SemanticPrice:
		// 金额边界：0 或极大值
		if gofakeit.Bool() {
			return "0.00"
		}
		return "99999999.99"
	case SemanticInteger:
		if sem.Minimum != nil {
			return int(*sem.Minimum)
		}
		if sem.Maximum != nil {
			return int(*sem.Maximum)
		}
		if gofakeit.Bool() {
			return 0
		}
		return 2147483647
	case SemanticNumber:
		if sem.Minimum != nil {
			return *sem.Minimum
		}
		if sem.Maximum != nil {
			return *sem.Maximum
		}
		return 0.0
	case SemanticName, SemanticTitle, SemanticDescription:
		// 文本边界：空字符串或超长
		if gofakeit.Bool() {
			return ""
		}
		return strings.Repeat("测", 500)
	}
	return value
}

// applyLengthConstraints 根据 schema 的 minLength/maxLength 裁剪或填充字符串。
func applyLengthConstraints(value any, sem FieldSemantics) any {
	s, ok := value.(string)
	if !ok {
		return value
	}
	runes := []rune(s)
	if sem.MaxLength != nil && len(runes) > *sem.MaxLength {
		if *sem.MaxLength <= 0 {
			return ""
		}
		return string(runes[:*sem.MaxLength])
	}
	if sem.MinLength != nil && len(runes) < *sem.MinLength {
		pad := strings.Repeat("测", *sem.MinLength-len(runes))
		return s + pad
	}
	return value
}

// applyStress 对压力场景做调整（生成较大数据量的值）。
func applyStress(value any, sem FieldSemantics) any {
	switch sem.Type {
	case SemanticDescription:
		// 压力场景生成较长文本
		if s, ok := value.(string); ok {
			return s + strings.Repeat("。压力测试补充数据填充内容。", 10)
		}
	case SemanticPrice:
		return "99999.99"
	case SemanticInteger:
		return gofakeit.IntRange(900000, 999999)
	}
	return value
}

// ── 具体模板函数 ───────────────────────────────────────────────────────

func genName(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.Name()
	default:
		return genChineseName()
	}
}

func genFirstName(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.FirstName()
	default:
		return genChineseFirstName()
	}
}

func genLastName(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.LastName()
	default:
		return genChineseLastName()
	}
}

func genUsername(_ Locale, _ string) any {
	return gofakeit.Username()
}

func genEmail(_ Locale, _ string) any {
	return gofakeit.Email()
}

func genPhone(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.PhoneFormatted()
	default:
		return genChineseMobile()
	}
}

func genAddress(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.Address().Address
	default:
		return genChineseAddress()
	}
}

func genCity(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.City()
	default:
		return pickFrom(chineseCities)
	}
}

func genProvince(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.State()
	default:
		return pickFrom(chineseProvinces)
	}
}

func genCountry(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.Country()
	default:
		return "中国"
	}
}

func genIDCard(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		// 美国没有统一身份证号，生成 SSN
		return gofakeit.SSN()
	default:
		return genChineseIDCard()
	}
}

func genAge(_ Locale, scenario string) any {
	switch scenario {
	case "boundary":
		if gofakeit.Bool() {
			return 0
		}
		return 150
	case "stress":
		return gofakeit.IntRange(1, 120)
	default:
		return gofakeit.IntRange(18, 65)
	}
}

func genGender(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		if gofakeit.Bool() {
			return "male"
		}
		return "female"
	default:
		if gofakeit.Bool() {
			return "男"
		}
		return "女"
	}
}

func genPrice(_ Locale, scenario string) any {
	switch scenario {
	case "boundary":
		if gofakeit.Bool() {
			return "0.00"
		}
		return "99999999.99"
	case "stress":
		return fmt.Sprintf("%.2f", gofakeit.Float64Range(10000, 999999))
	default:
		return fmt.Sprintf("%.2f", gofakeit.Float64Range(0.01, 9999.99))
	}
}

func genTitle(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.ProductName()
	default:
		return genChineseTitle()
	}
}

func genDescription(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.ProductDescription()
	default:
		return genChineseSentence(8)
	}
}

func genURL(_ Locale, _ string) any {
	return gofakeit.URL()
}

func genAvatar(_ Locale, _ string) any {
	return fmt.Sprintf("https://picsum.photos/seed/%s/200/200", gofakeit.UUID())
}

func genCompany(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.Company()
	default:
		return genChineseCompany()
	}
}

func genTimestamp(_ Locale, _ string) any {
	return gofakeit.Date().Format("2006-01-02 15:04:05")
}

func genBool(_ Locale, _ string) any {
	return gofakeit.Bool()
}

func genInteger(_ Locale, scenario string) any {
	switch scenario {
	case "boundary":
		if gofakeit.Bool() {
			return 0
		}
		return 2147483647
	case "stress":
		return gofakeit.IntRange(100000, 999999)
	default:
		return gofakeit.IntRange(1, 1000)
	}
}

func genNumber(_ Locale, scenario string) any {
	switch scenario {
	case "boundary":
		return 0.0
	case "stress":
		return gofakeit.Float64Range(10000, 999999)
	default:
		return gofakeit.Float64Range(0.01, 9999.99)
	}
}

func genPassword(_ Locale, _ string) any {
	return gofakeit.Password(true, true, true, true, false, 16)
}

func genStatus(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		statuses := []string{"active", "inactive", "pending", "suspended", "banned"}
		return pickFrom(statuses)
	default:
		statuses := []string{"启用", "禁用", "待审核", "已暂停", "已封禁"}
		return pickFrom(statuses)
	}
}

func genColor(_ Locale, _ string) any {
	return gofakeit.HexColor()
}

func genIP(_ Locale, _ string) any {
	return gofakeit.IPv4Address()
}

func genUUID(_ Locale, _ string) any {
	return gofakeit.UUID()
}

func genGeneric(locale Locale, _ string) any {
	switch locale {
	case LocaleEnUS:
		return gofakeit.Word()
	default:
		return genChineseWord()
	}
}

// ── 中文数据池 ─────────────────────────────────────────────────────────

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
	"琳", "雪", "佳", "慧", "悦", "怡", "晨", "思", "涵",
}

var chineseCities = []string{
	"北京", "上海", "广州", "深圳", "杭州", "成都", "武汉", "南京",
	"重庆", "西安", "天津", "苏州", "长沙", "郑州", "青岛", "大连",
}

var chineseProvinces = []string{
	"北京市", "上海市", "广东省", "浙江省", "江苏省", "四川省", "湖北省",
	"湖南省", "山东省", "河南省", "河北省", "福建省", "安徽省", "重庆市",
	"天津市", "陕西省", "辽宁省", "云南省", "贵州省", "黑龙江省",
}

var chineseRoads = []string{
	"人民路", "建设路", "解放路", "中山路", "和平路", "长江路",
	"文化路", "科技路", "新华路", "幸福路",
}

var chineseCompanySuffixes = []string{
	"科技有限公司", "信息技术有限公司", "电子商务有限公司", "网络科技有限公司",
	"数据科技有限公司", "智能科技有限公司", "软件有限公司", "创新科技有限公司",
	"数字科技有限公司", "云计算有限公司",
}

var chineseTitleWords = []string{
	"关于", "关于加强", "关于推进", "关于印发", "关于开展",
	"通知", "方案", "计划", "报告", "总结",
}

var chineseSentenceWords = []string{
	"用户", "系统", "平台", "接口", "数据", "流程", "请求", "响应",
	"服务", "场景", "自动", "生成", "校验", "记录", "同步", "更新",
	"创建", "查询", "提交", "返回", "成功", "稳定", "高效", "安全",
	"完整", "清晰", "快速", "准确", "持续", "灵活",
}

// 中国行政区划码（用于身份证号生成）
var chineseIDCardAreaCodes = []string{
	"110101", "110102", "110105", "110106", "110108",
	"120101", "120102", "120103", "120104", "120105",
	"310101", "310104", "310105", "310106", "310115",
	"500101", "500103", "500104", "500106", "500108",
	"320102", "320104", "320106", "320111", "320114",
	"330102", "330104", "330105", "330106", "330108",
	"440103", "440104", "440105", "440106", "440111",
	"440303", "440304", "440305", "440306", "440307",
}

// ── 辅助生成函数 ───────────────────────────────────────────────────────

func pickFrom(pool []string) string {
	return pool[gofakeit.IntRange(0, len(pool)-1)]
}

func genChineseName() string {
	return genChineseLastName() + genChineseFirstName()
}

func genChineseFirstName() string {
	length := gofakeit.IntRange(1, 2)
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteString(chineseGivenNameChars[gofakeit.IntRange(0, len(chineseGivenNameChars)-1)])
	}
	return b.String()
}

func genChineseLastName() string {
	return chineseSurnames[gofakeit.IntRange(0, len(chineseSurnames)-1)]
}

func genChineseMobile() string {
	var b strings.Builder
	b.WriteByte('1')
	b.WriteString(strconv.Itoa(gofakeit.IntRange(3, 9)))
	for i := 0; i < 9; i++ {
		b.WriteString(strconv.Itoa(gofakeit.IntRange(0, 9)))
	}
	return b.String()
}

func genChineseAddress() string {
	city := chineseCities[gofakeit.IntRange(0, len(chineseCities)-1)]
	road := chineseRoads[gofakeit.IntRange(0, len(chineseRoads)-1)]
	number := gofakeit.IntRange(1, 999)
	return city + "市" + road + strconv.Itoa(number) + "号"
}

func genChineseIDCard() string {
	area := chineseIDCardAreaCodes[gofakeit.IntRange(0, len(chineseIDCardAreaCodes)-1)]
	year := gofakeit.IntRange(1960, 2005)
	month := gofakeit.IntRange(1, 12)
	day := gofakeit.IntRange(1, daysInMonth(year, month))
	birthday := fmt.Sprintf("%04d%02d%02d", year, month, day)
	sequence := fmt.Sprintf("%03d", gofakeit.IntRange(0, 999))
	body := area + birthday + sequence
	return body + idCardCheckDigit(body)
}

// idCardCheckDigit 按 GB 11643-1999 ISO 7064 MOD 11-2 算法计算校验位。
func idCardCheckDigit(body17 string) string {
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkChars := []string{"1", "0", "X", "9", "8", "7", "6", "5", "4", "3", "2"}
	sum := 0
	for i := 0; i < 17; i++ {
		digit := int(body17[i] - '0')
		if digit < 0 || digit > 9 {
			return "0"
		}
		sum += digit * weights[i]
	}
	return checkChars[sum%11]
}

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

func genChineseCompany() string {
	prefix := genChineseLastName()
	suffix := chineseCompanySuffixes[gofakeit.IntRange(0, len(chineseCompanySuffixes)-1)]
	return prefix + suffix
}

func genChineseTitle() string {
	prefix := chineseTitleWords[gofakeit.IntRange(0, len(chineseTitleWords)-1)]
	suffix := genChineseWord()
	return prefix + suffix + "的方案"
}

func genChineseSentence(wordCount int) string {
	if wordCount < 1 {
		wordCount = 4
	}
	words := make([]string, 0, wordCount)
	for i := 0; i < wordCount; i++ {
		words = append(words, chineseSentenceWords[gofakeit.IntRange(0, len(chineseSentenceWords)-1)])
	}
	return strings.Join(words, "") + "。"
}

func genChineseWord() string {
	return chineseSentenceWords[gofakeit.IntRange(0, len(chineseSentenceWords)-1)]
}
