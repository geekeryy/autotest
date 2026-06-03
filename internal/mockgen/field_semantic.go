package mockgen

import (
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// SemanticInferrer 根据字段名推断合理的 mock 值。
// 当 schema 缺少 format/example 时，通过字段名模式匹配生成语义合理的数据。
type SemanticInferrer struct {
	rules []semanticRule
}

// semanticRule 定义一条语义推断规则。
type semanticRule struct {
	patterns  []string      // 匹配的字段名模式（全小写）
	generator func() string // 生成函数
}

// DefaultSemanticInferrer 返回默认的语义推断器，内置常见字段名规则。
func DefaultSemanticInferrer() *SemanticInferrer {
	return &SemanticInferrer{rules: defaultSemanticRules}
}

// Infer 根据字段名推断并生成一个值。
// 返回空字符串表示无法推断。
func (si *SemanticInferrer) Infer(fieldName string) string {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	for _, rule := range si.rules {
		for _, pattern := range rule.patterns {
			if matchFieldName(name, pattern) {
				return rule.generator()
			}
		}
	}
	return ""
}

// matchFieldName 检查字段名是否匹配模式。
// 支持精确匹配和包含匹配（如 "user_name" 包含 "name"）。
func matchFieldName(name, pattern string) bool {
	if name == pattern {
		return true
	}
	// 检查是否包含模式（用下划线或驼峰分隔）
	// 例如 "user_name" 匹配 "name"，"userName" 匹配 "name"
	if strings.Contains(name, pattern) {
		return true
	}
	return false
}

// defaultSemanticRules 是默认的语义推断规则列表。
// 规则按优先级排列，先匹配的先返回。
var defaultSemanticRules = []semanticRule{
	// === 人员信息 ===
	{
		[]string{"name", "username", "nickname", "realname", "real_name", "display_name"},
		func() string { return gofakeit.Name() },
	},
	{
		[]string{"firstname", "first_name", "given_name"},
		func() string { return gofakeit.FirstName() },
	},
	{
		[]string{"lastname", "last_name", "surname", "family_name"},
		func() string { return gofakeit.LastName() },
	},
	{
		[]string{"phone", "mobile", "tel", "telephone", "cellphone"},
		chineseMobilePhone,
	},
	{
		[]string{"email", "mail"},
		func() string { return gofakeit.Email() },
	},
	{
		[]string{"avatar", "headimg", "head_img", "portrait", "photo", "image", "img"},
		func() string { return fmt.Sprintf("https://picsum.photos/%d/%d", gofakeit.IntRange(100, 800), gofakeit.IntRange(100, 800)) },
	},
	{
		[]string{"id_card", "idcard", "identity", "identity_no", "cert_no"},
		chineseIDCard,
	},
	{
		[]string{"age"},
		func() string { return fmt.Sprintf("%d", gofakeit.IntRange(18, 65)) },
	},
	{
		[]string{"gender", "sex"},
		func() string {
			if gofakeit.Bool() {
				return "male"
			}
			return "female"
		},
	},
	{
		[]string{"birthday", "birth_date", "birthdate", "dob"},
		func() string { return gofakeit.Date().UTC().Format("2006-01-02") },
	},

	// === 地址信息 ===
	{
		[]string{"city"},
		func() string { return gofakeit.City() },
	},
	{
		[]string{"country"},
		func() string { return gofakeit.Country() },
	},
	{
		[]string{"province", "state", "region"},
		func() string { return gofakeit.State() },
	},
	{
		[]string{"address", "addr", "street"},
		func() string { return gofakeit.Address().Address },
	},
	{
		[]string{"zipcode", "zip_code", "postal_code", "zip"},
		func() string { return gofakeit.Zip() },
	},

	// === 联系方式 ===
	{
		[]string{"url", "link", "href", "website"},
		func() string { return gofakeit.URL() },
	},
	{
		[]string{"ip", "ip_address"},
		func() string { return gofakeit.IPv4Address() },
	},

	// === 商业/业务 ===
	{
		[]string{"company", "organization", "org", "enterprise"},
		func() string { return gofakeit.Company() },
	},
	{
		[]string{"title", "subject"},
		func() string { return gofakeit.Sentence(3) },
	},
	{
		[]string{"desc", "description", "remark", "summary", "content", "body", "text"},
		func() string { return gofakeit.Sentence(10) },
	},
	{
		[]string{"price", "amount", "fee", "cost", "total", "balance"},
		func() string {
			return fmt.Sprintf("%.2f", gofakeit.Float64Range(0.01, 99999.99))
		},
	},
	{
		[]string{"quantity", "qty", "count", "num", "number"},
		func() string { return fmt.Sprintf("%d", gofakeit.IntRange(1, 100)) },
	},
	{
		[]string{"sku"},
		func() string {
			return fmt.Sprintf("%c%c-%06d",
				'A'+byte(gofakeit.IntRange(0, 25)),
				'A'+byte(gofakeit.IntRange(0, 25)),
				gofakeit.IntRange(0, 999999))
		},
	},
	{
		[]string{"code", "no", "serial", "serial_no"},
		func() string { return strings.ToUpper(gofakeit.LetterN(8)) },
	},
	{
		[]string{"plate", "plate_number", "platenumber", "car_no"},
		chinesePlateNumber,
	},
	{
		[]string{"bank_card", "bankcard", "card_no", "card_number"},
		chineseBankCard,
	},
	{
		[]string{"uscc", "unified_social_credit_code", "credit_code"},
		chineseUSCC,
	},

	// === 状态/类型 ===
	{
		[]string{"status", "state"},
		func() string {
			statuses := []string{"active", "inactive", "pending", "disabled"}
			return statuses[gofakeit.IntRange(0, len(statuses)-1)]
		},
	},
	{
		[]string{"type", "category"},
		func() string {
			types := []string{"type_a", "type_b", "type_c", "default"}
			return types[gofakeit.IntRange(0, len(types)-1)]
		},
	},

	// === 时间 ===
	{
		[]string{"created_at", "updated_at", "created_time", "updated_time", "createtime", "updatetime", "create_time", "update_time"},
		func() string { return gofakeit.Date().UTC().Format("2006-01-02 15:04:05") },
	},

	// === 其他 ===
	{
		[]string{"color", "colour"},
		func() string { return gofakeit.Color() },
	},
	{
		[]string{"version"},
		func() string {
			return fmt.Sprintf("%d.%d.%d",
				gofakeit.IntRange(1, 5),
				gofakeit.IntRange(0, 20),
				gofakeit.IntRange(0, 99))
		},
	},
	{
		[]string{"tag", "label"},
		func() string { return gofakeit.Word() },
	},
}

// === 辅助生成函数 ===

// chineseIDCard 生成随机中国身份证号（简化版，仅格式合法）。
func chineseIDCard() string {
	area := []string{"110101", "310101", "440103", "510104", "330102"}[gofakeit.IntRange(0, 4)]
	year := gofakeit.IntRange(1960, 2005)
	month := gofakeit.IntRange(1, 12)
	day := gofakeit.IntRange(1, 28)
	seq := gofakeit.IntRange(0, 999)
	return fmt.Sprintf("%s%04d%02d%02d%03d%d", area, year, month, day, seq, gofakeit.IntRange(0, 9))
}

// chinesePlateNumber 生成随机中国车牌号。
func chinesePlateNumber() string {
	provinces := []string{"京", "津", "沪", "渝", "冀", "豫", "云", "辽", "黑", "湘",
		"皖", "鲁", "新", "苏", "浙", "赣", "鄂", "桂", "甘", "晋",
		"蒙", "陕", "吉", "闽", "贵", "粤", "青", "藏", "川", "宁", "琼"}
	prefix := provinces[gofakeit.IntRange(0, len(provinces)-1)]
	letter := string(rune('A' + gofakeit.IntRange(0, 25)))
	tail := make([]byte, 5)
	for i := range tail {
		idx := gofakeit.IntRange(0, 35)
		if idx < 10 {
			tail[i] = byte('0' + idx)
		} else {
			tail[i] = byte('A' + idx - 10)
		}
	}
	return prefix + "·" + letter + string(tail)
}

// chineseBankCard 生成随机银行卡号（简化版）。
func chineseBankCard() string {
	prefixes := []string{"622", "621", "620", "436"}
	prefix := prefixes[gofakeit.IntRange(0, len(prefixes)-1)]
	digits := prefix
	for i := len(digits); i < 18; i++ {
		digits += fmt.Sprintf("%d", gofakeit.IntRange(0, 9))
	}
	return digits
}

// chineseUSCC 生成随机统一社会信用代码（简化版）。
func chineseUSCC() string {
	pairs := []string{"91", "92", "93", "51", "12"}
	pair := pairs[gofakeit.IntRange(0, len(pairs)-1)]
	area := []string{"110101", "310101", "440103", "510104", "330102"}[gofakeit.IntRange(0, 4)]
	chars := "0123456789ABCDEFGHJKLMNPQRTUWXY"
	body := pair + area
	for i := 0; i < 9; i++ {
		body += string(chars[gofakeit.IntRange(0, len(chars)-1)])
	}
	return body + string(chars[gofakeit.IntRange(0, len(chars)-1)])
}
