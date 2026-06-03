// Package aifactory 实现语义感知的测试数据工厂引擎。
// 通过分析 OpenAPI schema 中字段的名称、描述、格式，推断业务语义，
// 并根据语义生成有意义的测试数据（而非纯随机值）。
package aifactory

import (
	"strings"
)

// SemanticType 表示字段的业务语义类型。
type SemanticType string

const (
	SemanticName        SemanticType = "name"        // 姓名
	SemanticFirstName   SemanticType = "first_name"  // 名
	SemanticLastName    SemanticType = "last_name"    // 姓
	SemanticEmail       SemanticType = "email"        // 邮箱
	SemanticPhone       SemanticType = "phone"        // 手机号
	SemanticAddress     SemanticType = "address"      // 地址
	SemanticAge         SemanticType = "age"          // 年龄
	SemanticPrice       SemanticType = "price"        // 金额/价格
	SemanticTitle       SemanticType = "title"        // 标题
	SemanticDescription SemanticType = "description"  // 描述文本
	SemanticURL         SemanticType = "url"          // URL
	SemanticIDCard      SemanticType = "id_card"      // 身份证号
	SemanticCompany     SemanticType = "company"      // 公司名
	SemanticTimestamp   SemanticType = "timestamp"    // 时间戳
	SemanticBoolean     SemanticType = "boolean"      // 布尔值
	SemanticInteger     SemanticType = "integer"      // 整数
	SemanticNumber      SemanticType = "number"       // 浮点数
	SemanticGender      SemanticType = "gender"       // 性别
	SemanticCountry     SemanticType = "country"      // 国家
	SemanticCity        SemanticType = "city"         // 城市
	SemanticProvince    SemanticType = "province"     // 省份
	SemanticAvatar      SemanticType = "avatar"       // 头像URL
	SemanticPassword    SemanticType = "password"     // 密码
	SemanticUsername    SemanticType = "username"      // 用户名
	SemanticStatus      SemanticType = "status"       // 状态
	SemanticColor       SemanticType = "color"        // 颜色
	SemanticIP          SemanticType = "ip"           // IP地址
	SemanticUUID        SemanticType = "uuid"         // UUID
	SemanticGeneric     SemanticType = "generic"      // 通用/未识别
)

// FieldSemantics 描述一个字段推断出的业务语义。
type FieldSemantics struct {
	Type         SemanticType // 推断出的语义类型
	FieldName    string       // 原始字段名
	Description  string       // schema 中的 description
	Format       string       // schema 中的 format
	EnumValues   []any        // schema 中的 enum 约束
	MinLength    *int         // 字符串最小长度
	MaxLength    *int         // 字符串最大长度
	Minimum      *float64     // 数值最小值
	Maximum      *float64     // 数值最大值
	Required     bool         // 是否为必填字段
	IsArray      bool         // 是否为数组类型
	JSONType     string       // 原始 JSON Schema type
}

// InferSemantics 根据字段名和 OpenAPI schema 推断业务语义。
// 优先根据字段名匹配已知模式，其次根据 schema 的 format/type 辅助判断。
func InferSemantics(fieldName string, schema map[string]any, required bool) FieldSemantics {
	fs := FieldSemantics{
		FieldName: fieldName,
		Required:  required,
		Type:      SemanticGeneric,
	}

	// 从 schema 中提取元信息
	if desc, ok := schema["description"].(string); ok {
		fs.Description = desc
	}
	if fmt, ok := schema["format"].(string); ok {
		fs.Format = fmt
	}
	if enum, ok := schema["enum"].([]any); ok {
		fs.EnumValues = enum
	}
	if t, ok := schema["type"].(string); ok {
		fs.JSONType = t
	}
	if min, ok := schema["minimum"].(float64); ok {
		fs.Minimum = &min
	}
	if max, ok := schema["maximum"].(float64); ok {
		fs.Maximum = &max
	}
	if minLen, ok := schema["minLength"].(float64); ok {
		v := int(minLen)
		fs.MinLength = &v
	}
	if maxLen, ok := schema["maxLength"].(float64); ok {
		v := int(maxLen)
		fs.MaxLength = &v
	}

	// 根据字段名推断语义类型
	fs.Type = inferTypeFromName(fieldName)

	// 根据 format 补充或覆盖推断
	if fs.Type == SemanticGeneric {
		fs.Type = inferTypeFromFormat(fs.Format)
	}

	// 根据 JSON type 做最终兜底
	if fs.Type == SemanticGeneric {
		fs.Type = inferTypeFromJSONType(fs.JSONType)
	}

	return fs
}

// inferTypeFromName 根据字段名（小写、去下划线/连字符后匹配）推断语义。
func inferTypeFromName(name string) SemanticType {
	n := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, "-", "_"), " ", "_"))

	// 姓名相关
	if n == "name" || n == "fullname" || n == "full_name" || n == "real_name" || n == "realname" ||
		n == "display_name" || n == "displayname" || n == "nick_name" || n == "nickname" {
		return SemanticName
	}
	if n == "first_name" || n == "firstname" || n == "given_name" || n == "givenname" {
		return SemanticFirstName
	}
	if n == "last_name" || n == "lastname" || n == "surname" || n == "family_name" || n == "familyname" {
		return SemanticLastName
	}
	if n == "username" || n == "user_name" || n == "login_name" || n == "loginname" {
		return SemanticUsername
	}

	// 联系方式
	if n == "email" || n == "mail" || n == "e_mail" {
		return SemanticEmail
	}
	if n == "phone" || n == "mobile" || n == "mobile_phone" || n == "mobilephone" ||
		n == "telephone" || n == "tel" || n == "phone_number" || n == "phonenumber" ||
		n == "cell_phone" || n == "cellphone" {
		return SemanticPhone
	}

	// 地址相关
	if n == "address" || n == "street" || n == "street_address" || n == "streetaddress" ||
		n == "home_address" || n == "homeaddress" {
		return SemanticAddress
	}
	if n == "city" {
		return SemanticCity
	}
	if n == "province" || n == "state" {
		return SemanticProvince
	}
	if n == "country" || n == "nation" {
		return SemanticCountry
	}

	// 身份信息
	if n == "id_card" || n == "idcard" || n == "identity" || n == "identity_number" ||
		n == "identitynumber" || n == "id_number" || n == "idnumber" || n == "cert_no" {
		return SemanticIDCard
	}
	if n == "age" {
		return SemanticAge
	}
	if n == "gender" || n == "sex" {
		return SemanticGender
	}

	// 金额
	if n == "price" || n == "amount" || n == "cost" || n == "fee" || n == "total" ||
		n == "total_price" || n == "totalprice" || n == "unit_price" || n == "unitprice" ||
		n == "balance" || n == "salary" || n == "wage" {
		return SemanticPrice
	}

	// 文本
	if n == "title" || n == "subject" || n == "headline" {
		return SemanticTitle
	}
	if n == "description" || n == "desc" || n == "summary" || n == "content" || n == "body" ||
		n == "remark" || n == "remarks" || n == "comment" || n == "note" || n == "memo" {
		return SemanticDescription
	}

	// URL
	if n == "url" || n == "link" || n == "website" || n == "homepage" || n == "href" {
		return SemanticURL
	}
	if n == "avatar" || n == "avatar_url" || n == "avatarurl" || n == "photo" || n == "image" ||
		n == "image_url" || n == "imageurl" || n == "pic" || n == "picture" {
		return SemanticAvatar
	}

	// 公司
	if n == "company" || n == "company_name" || n == "companyname" || n == "organization" ||
		n == "org" || n == "org_name" || n == "orgname" || n == "enterprise" {
		return SemanticCompany
	}

	// 时间
	if n == "created_at" || n == "createdat" || n == "updated_at" || n == "updatedat" ||
		n == "deleted_at" || n == "deletedat" || n == "created_time" || n == "updated_time" ||
		n == "start_time" || n == "starttime" || n == "end_time" || n == "endtime" ||
		n == "birthday" || n == "birth_date" || n == "birthdate" {
		return SemanticTimestamp
	}

	// 布尔
	if strings.HasPrefix(n, "is_") || strings.HasPrefix(n, "has_") || strings.HasPrefix(n, "can_") ||
		strings.HasPrefix(n, "enable") || strings.HasPrefix(n, "disable") || n == "active" ||
		n == "visible" || n == "verified" || n == "approved" {
		return SemanticBoolean
	}

	// UUID / token
	if n == "uuid" || n == "guid" {
		return SemanticUUID
	}
	if n == "token" || n == "api_key" || n == "apikey" || n == "access_token" || n == "accesstoken" {
		return SemanticUUID
	}

	// 密码
	if n == "password" || n == "passwd" || n == "pwd" || n == "secret" {
		return SemanticPassword
	}

	// 状态
	if n == "status" || n == "state" || n == "phase" || n == "stage" || n == "level" {
		return SemanticStatus
	}

	// 颜色
	if n == "color" || n == "colour" || n == "background_color" || n == "bgcolor" ||
		n == "text_color" || n == "font_color" {
		return SemanticColor
	}

	// IP
	if n == "ip" || n == "ip_address" || n == "ipaddress" || n == "ipv4" || n == "ipv6" {
		return SemanticIP
	}

	return SemanticGeneric
}

// inferTypeFromFormat 根据 OpenAPI format 字段推断语义类型。
func inferTypeFromFormat(format string) SemanticType {
	switch strings.ToLower(format) {
	case "email":
		return SemanticEmail
	case "uri", "url":
		return SemanticURL
	case "phone", "tel":
		return SemanticPhone
	case "date", "date-time", "datetime":
		return SemanticTimestamp
	case "ipv4":
		return SemanticIP
	case "ipv6":
		return SemanticIP
	case "uuid":
		return SemanticUUID
	case "password":
		return SemanticPassword
	default:
		return SemanticGeneric
	}
}

// inferTypeFromJSONType 根据 JSON Schema type 做最终兜底推断。
func inferTypeFromJSONType(jsonType string) SemanticType {
	switch strings.ToLower(jsonType) {
	case "boolean":
		return SemanticBoolean
	case "integer":
		return SemanticInteger
	case "number":
		return SemanticNumber
	default:
		return SemanticGeneric
	}
}
