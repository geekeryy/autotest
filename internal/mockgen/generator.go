// Package mockgen 根据 JSON Schema 自动生成符合类型和约束的 mock 数据。
// 它复用 internal/mockdata 中的 gofakeit helpers 生成真实中文数据，
// 并通过字段名语义推断在 schema 缺少 format/example 时仍能生成合理的值。
//
// 核心优先级：enum > example > default > format > type 约束 > 语义推断。
package mockgen

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// Generator 根据 JSON Schema 生成 mock 数据。
type Generator struct {
	semantic *SemanticInferrer
	maxDepth int // 防止无限递归，默认 8
	rng      *rand.Rand
}

// Default 返回一个使用默认配置的 Generator。
func Default() *Generator {
	return &Generator{
		semantic: DefaultSemanticInferrer(),
		maxDepth: 8,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Generate 根据 schema 生成一个 mock 值，返回 JSON-ready 的值。
// schema 应为从 ResponseSchema.body 中提取的 JSON Schema map。
func (g *Generator) Generate(schema map[string]any) any {
	return g.generateValue(schema, 0)
}

// GenerateJSON 根据 schema 生成 JSON 字节。
func (g *Generator) GenerateJSON(schema map[string]any) ([]byte, error) {
	value := g.Generate(schema)
	return json.Marshal(value)
}

// generateValue 递归生成值，depth 用于防止无限递归。
func (g *Generator) generateValue(schema map[string]any, depth int) any {
	if depth > g.maxDepth {
		return nil
	}
	if schema == nil {
		return nil
	}

	// 1. enum 优先
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return enum[g.rng.Intn(len(enum))]
	}

	// 2. example 次之
	if example, ok := schema["example"]; ok && example != nil {
		return example
	}

	// 3. default
	if def, ok := schema["default"]; ok && def != nil {
		return def
	}

	// 4. nullable: 10% 概率返回 nil
	if nullable, ok := schema["nullable"].(bool); ok && nullable {
		if g.rng.Float64() < 0.1 {
			return nil
		}
	}

	// 5. 按 type 分发
	return g.generateByType(schema, depth)
}

// generateByType 根据 schema 的 type 字段分发到具体的生成逻辑。
func (g *Generator) generateByType(schema map[string]any, depth int) any {
	types := getTypes(schema)
	if len(types) == 0 {
		// 无 type，尝试从 properties 推断为 object
		if _, ok := schema["properties"]; ok {
			return g.generateObject(schema, depth)
		}
		// 尝试 allOf/oneOf/anyOf
		if merged := g.mergeComposedSchemas(schema, depth); merged != nil {
			return merged
		}
		return nil
	}

	// 取第一个 type（多 type 场景如 ["string", "null"] 已通过 nullable 处理）
	t := types[0]
	switch t {
	case "string":
		return g.generateString(schema)
	case "integer":
		return g.generateInteger(schema)
	case "number":
		return g.generateNumber(schema)
	case "boolean":
		return gofakeit.Bool()
	case "array":
		return g.generateArray(schema, depth)
	case "object":
		return g.generateObject(schema, depth)
	default:
		return nil
	}
}

// getTypes 从 schema 中提取 type，支持 string 和 []any 两种格式。
func getTypes(schema map[string]any) []string {
	v, ok := schema["type"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var types []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
		return types
	}
	return nil
}

// getString 从 schema 中安全提取 string 字段。
func getString(schema map[string]any, key string) string {
	v, ok := schema[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// getInt 从 schema 中安全提取数值字段（支持 int/float64/json.Number）。
func getInt(schema map[string]any, key string) (int, bool) {
	v, ok := schema[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
}

// getFloat 从 schema 中安全提取浮点数值字段。
func getFloat(schema map[string]any, key string) (float64, bool) {
	v, ok := schema[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// getBool 从 schema 中安全提取 bool 字段。
func getBool(schema map[string]any, key string) bool {
	v, ok := schema[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// getList 从 schema 中安全提取 []any 字段。
func getList(schema map[string]any, key string) []any {
	v, ok := schema[key]
	if !ok {
		return nil
	}
	l, _ := v.([]any)
	return l
}

// getMap 从 schema 中安全提取 map[string]any 字段。
func getMap(schema map[string]any, key string) map[string]any {
	v, ok := schema[key]
	if !ok {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

// formatNumber 根据精度格式化数字。
func formatNumber(v float64, precision int) any {
	if precision == 0 && v == float64(int64(v)) {
		return int64(v)
	}
	return fmt.Sprintf("%.*f", precision, v)
}
