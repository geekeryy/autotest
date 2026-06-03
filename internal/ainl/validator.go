package ainl

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError 表示校验错误。
type ValidationError struct {
	StepOrder int    `json:"stepOrder"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

// ValidationResult 是校验结果。
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate 校验 LLM 生成的步骤列表。
// 检查项：
// 1. 所有 API 步骤的端点都已匹配（matchResult 不为 none）
// 2. 步骤间引用正确（引用的步骤序号存在且在当前步骤之前）
// 3. API 步骤必填参数已填充
func Validate(steps []LLMStep, matchResults []MatchResult) ValidationResult {
	result := ValidationResult{Valid: true}

	if len(steps) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			StepOrder: 0,
			Field:     "steps",
			Message:   "步骤列表不能为空",
		})
		return result
	}

	// 建立 stepOrder 存在性集合
	existingOrders := make(map[int]bool, len(steps))
	for _, step := range steps {
		existingOrders[step.StepOrder] = true
	}

	// 收集所有步骤响应中的 token 引用模式
	// 格式：{{steps[N].response.xxx}}
	stepRefPattern := regexp.MustCompile(`\{\{steps\[(\d+)\]\.response\.`)

	for i, step := range steps {
		order := step.StepOrder

		// 校验 stepOrder 有效性
		if order < 1 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				StepOrder: order,
				Field:     "stepOrder",
				Message:   fmt.Sprintf("第 %d 个步骤的 stepOrder 必须 >= 1", i+1),
			})
		}

		// 校验步骤名称
		if strings.TrimSpace(step.Name) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				StepOrder: order,
				Field:     "name",
				Message:   fmt.Sprintf("步骤 %d 缺少名称", order),
			})
		}

		// API 步骤特定校验
		if step.StepType == "api" || step.StepType == "" {
			// 校验端点匹配
			if i < len(matchResults) {
				mr := matchResults[i]
				if mr.Confidence == "none" {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						StepOrder: order,
						Field:     "endpoint",
						Message:   fmt.Sprintf("步骤 %d 的端点 %s %s 无法匹配到已知接口：%s", order, step.Method, step.Path, mr.Reason),
					})
				}
			}

			// 校验 method 和 path 非空
			if strings.TrimSpace(step.Method) == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					StepOrder: order,
					Field:     "method",
					Message:   fmt.Sprintf("API 步骤 %d 缺少 HTTP method", order),
				})
			}
			if strings.TrimSpace(step.Path) == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					StepOrder: order,
					Field:     "path",
					Message:   fmt.Sprintf("API 步骤 %d 缺少请求路径", order),
				})
			}

			// 校验断言非空（至少应有一个断言）
			if len(step.Assertions) == 0 {
				// 这是警告而非错误，不设 valid=false
				result.Errors = append(result.Errors, ValidationError{
					StepOrder: order,
					Field:     "assertions",
					Message:   fmt.Sprintf("API 步骤 %d 没有断言，建议至少添加 status_code 断言", order),
				})
			}
		}
	}

	// 校验步骤间引用
	for _, step := range steps {
		raw := marshalStepRequest(step.Request)
		matches := stepRefPattern.FindAllStringSubmatch(raw, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			var refOrder int
			if _, err := fmt.Sscanf(match[1], "%d", &refOrder); err != nil {
				continue
			}
			if !existingOrders[refOrder] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					StepOrder: step.StepOrder,
					Field:     "request",
					Message: fmt.Sprintf("步骤 %d 引用了不存在的步骤 %d（steps[%d].response）",
						step.StepOrder, refOrder, refOrder),
				})
			}
			if refOrder >= step.StepOrder {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					StepOrder: step.StepOrder,
					Field:     "request",
					Message: fmt.Sprintf("步骤 %d 引用了后续步骤 %d 的响应，必须引用之前的步骤",
						step.StepOrder, refOrder),
				})
			}
		}
	}

	return result
}

// marshalStepRequest 将 request map 序列化为字符串用于引用检测。
func marshalStepRequest(req map[string]any) string {
	if req == nil {
		return ""
	}
	// 简单递归序列化为字符串用于正则匹配
	return marshalAny(req)
}

// marshalAny 递归将任意值序列化为字符串。
func marshalAny(v any) string {
	switch val := v.(type) {
	case map[string]any:
		var sb strings.Builder
		sb.WriteString("{")
		for k, v2 := range val {
			sb.WriteString(k)
			sb.WriteString(":")
			sb.WriteString(marshalAny(v2))
			sb.WriteString(",")
		}
		sb.WriteString("}")
		return sb.String()
	case []any:
		var sb strings.Builder
		sb.WriteString("[")
		for _, item := range val {
			sb.WriteString(marshalAny(item))
			sb.WriteString(",")
		}
		sb.WriteString("]")
		return sb.String()
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// MatchResultsFromEndpoints 为已精确匹配的端点构建 MatchResult 列表。
// 用于当端点匹配已由 EndpointMatcher 完成时直接构建校验输入。
func MatchResultsFromEndpoints(steps []LLMStep, matcher *EndpointMatcher) []MatchResult {
	return matcher.MatchAll(steps)
}
