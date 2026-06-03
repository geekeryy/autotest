package mockgen

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// generateString 根据 schema 约束生成字符串值。
// 优先级：format > 语义推断 > minLength/maxLength 随机。
func (g *Generator) generateString(schema map[string]any) any {
	format := getString(schema, "format")
	if format != "" {
		if v := g.generateByFormat(format, schema); v != nil {
			return v
		}
	}

	// 尝试从 title/description 推断字段名语义
	if fieldName := extractFieldName(schema); fieldName != "" {
		if v := g.semantic.Infer(fieldName); v != "" {
			return v
		}
	}

	// 根据 minLength/maxLength 生成随机字符串
	minLen := 1
	maxLen := 20
	if v, ok := getInt(schema, "minLength"); ok && v > 0 {
		minLen = v
	}
	if v, ok := getInt(schema, "maxLength"); ok && v > 0 {
		maxLen = v
	}
	if maxLen < minLen {
		maxLen = minLen + 20
	}
	length := g.rng.Intn(maxLen-minLen+1) + minLen
	if length > 100 {
		length = 100
	}
	return gofakeit.LetterN(uint(length))
}

// generateByFormat 根据 JSON Schema format 生成对应类型的值。
func (g *Generator) generateByFormat(format string, schema map[string]any) any {
	switch strings.ToLower(format) {
	case "email":
		return gofakeit.Email()
	case "uri", "url":
		return gofakeit.URL()
	case "date-time", "datetime":
		return gofakeit.Date().UTC().Format(time.RFC3339)
	case "date":
		return gofakeit.Date().UTC().Format("2006-01-02")
	case "time":
		return gofakeit.Date().UTC().Format("15:04:05")
	case "ipv4":
		return gofakeit.IPv4Address()
	case "ipv6":
		return gofakeit.IPv6Address()
	case "uuid":
		return gofakeit.UUID()
	case "hostname":
		return gofakeit.DomainName()
	case "phone":
		return chineseMobilePhone()
	}
	return nil
}

// generateInteger 根据 schema 约束生成整数值。
func (g *Generator) generateInteger(schema map[string]any) any {
	min, max := 0, 1000
	if v, ok := getInt(schema, "minimum"); ok {
		min = v
	}
	if v, ok := getInt(schema, "maximum"); ok {
		max = v
	}
	if getBool(schema, "exclusiveMinimum") {
		min++
	}
	if getBool(schema, "exclusiveMaximum") {
		max--
	}
	if max < min {
		max = min + 1000
	}
	return g.rng.Intn(max-min+1) + min
}

// generateNumber 根据 schema 约束生成浮点数值。
func (g *Generator) generateNumber(schema map[string]any) any {
	min := 0.0
	max := 1000.0
	if v, ok := getFloat(schema, "minimum"); ok {
		min = v
	}
	if v, ok := getFloat(schema, "maximum"); ok {
		max = v
	}
	if getBool(schema, "exclusiveMinimum") {
		min += 0.01
	}
	if getBool(schema, "exclusiveMaximum") {
		max -= 0.01
	}
	if max < min {
		max = min + 1000
	}
	precision := 2
	if v, ok := getFloat(schema, "multipleOf"); ok && v > 0 {
		precision = decimalPlaces(v)
	}
	v := gofakeit.Float64Range(min, max)
	return formatNumber(v, precision)
}

// generateArray 根据 schema 约束生成数组。
func (g *Generator) generateArray(schema map[string]any, depth int) any {
	minItems := 1
	maxItems := 3
	if v, ok := getInt(schema, "minItems"); ok && v > 0 {
		minItems = v
	}
	if v, ok := getInt(schema, "maxItems"); ok && v > 0 {
		maxItems = v
	}
	if maxItems < minItems {
		maxItems = minItems
	}
	// 限制数组大小，防止生成过大的数据
	if maxItems > 10 {
		maxItems = 10
	}
	length := g.rng.Intn(maxItems-minItems+1) + minItems

	itemsSchema := getMap(schema, "items")
	if itemsSchema == nil {
		// 无 items 定义，生成字符串数组
		result := make([]any, length)
		for i := range result {
			result[i] = gofakeit.Word()
		}
		return result
	}

	result := make([]any, length)
	for i := range result {
		result[i] = g.generateValue(itemsSchema, depth+1)
	}
	return result
}

// generateObject 根据 schema 约束生成对象。
func (g *Generator) generateObject(schema map[string]any, depth int) any {
	properties := getMap(schema, "properties")
	if properties == nil {
		return map[string]any{}
	}

	required := make(map[string]bool)
	if reqList := getList(schema, "required"); reqList != nil {
		for _, r := range reqList {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}

	result := make(map[string]any, len(properties))
	for name, prop := range properties {
		propSchema, ok := prop.(map[string]any)
		if !ok {
			continue
		}

		// 可选字段 80% 概率生成
		if !required[name] && g.rng.Float64() > 0.8 {
			continue
		}

		// 跳过 readOnly 字段（通常是响应中的 id 等）
		if getBool(propSchema, "readOnly") {
			continue
		}

		// 为属性注入字段名，供语义推断使用
		enrichedSchema := enrichSchema(propSchema, name)
		result[name] = g.generateValue(enrichedSchema, depth+1)
	}
	return result
}

// mergeComposedSchemas 处理 allOf/oneOf/anyOf 组合 schema。
func (g *Generator) mergeComposedSchemas(schema map[string]any, depth int) any {
	// allOf: 合并所有 schema
	if allOf := getList(schema, "allOf"); allOf != nil {
		merged := map[string]any{}
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				mergeSchema(merged, sub)
			}
		}
		return g.generateValue(merged, depth+1)
	}

	// oneOf/anyOf: 随机选一个
	for _, key := range []string{"oneOf", "anyOf"} {
		if options := getList(schema, key); options != nil && len(options) > 0 {
			idx := g.rng.Intn(len(options))
			if sub, ok := options[idx].(map[string]any); ok {
				return g.generateValue(sub, depth+1)
			}
		}
	}
	return nil
}

// enrichSchema 将字段名注入 schema，供语义推断使用。
func enrichSchema(schema map[string]any, fieldName string) map[string]any {
	enriched := make(map[string]any, len(schema)+1)
	for k, v := range schema {
		enriched[k] = v
	}
	// 如果没有 title，用字段名作为 _fieldName 供语义推断
	if _, ok := enriched["title"]; !ok {
		enriched["_fieldName"] = fieldName
	}
	return enriched
}

// extractFieldName 从 schema 中提取字段名（用于语义推断）。
func extractFieldName(schema map[string]any) string {
	// 优先使用 _fieldName（由 enrichSchema 注入）
	if name, ok := schema["_fieldName"].(string); ok {
		return name
	}
	// 尝试从 title 推断
	if title, ok := schema["title"].(string); ok {
		return title
	}
	return ""
}

// mergeSchema 将 src 的属性合并到 dst 中。
func mergeSchema(dst, src map[string]any) {
	for k, v := range src {
		if k == "properties" {
			// properties 需要深层合并
			if dstProps, ok := dst["properties"].(map[string]any); ok {
				if srcProps, ok := v.(map[string]any); ok {
					for name, prop := range srcProps {
						dstProps[name] = prop
					}
					continue
				}
			}
		}
		dst[k] = v
	}
}

// chineseMobilePhone 生成随机中国手机号。
func chineseMobilePhone() string {
	var b strings.Builder
	b.WriteByte('1')
	b.WriteByte(byte('0' + gofakeit.IntRange(3, 9)))
	for i := 0; i < 9; i++ {
		b.WriteByte(byte('0' + gofakeit.IntRange(0, 9)))
	}
	return b.String()
}

// decimalPlaces 计算浮点数的小数位数（基于 multipleOf 推断）。
func decimalPlaces(v float64) int {
	if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 2
	}
	s := fmt.Sprintf("%.10f", v)
	dot := strings.Index(s, ".")
	if dot < 0 {
		return 0
	}
	// 从末尾去掉多余的零
	end := len(s)
	for end > dot+1 && s[end-1] == '0' {
		end--
	}
	return end - dot - 1
}
