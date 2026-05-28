package testprofile

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatProfileForPrompt formats a project test profile into a context string
// suitable for injection into AI prompts. This gives the LLM project-specific
// knowledge to produce better test cases and scenarios.
func FormatProfileForPrompt(profile *Profile) string {
	if profile == nil {
		return ""
	}

	var sb strings.Builder

	// Response convention.
	if conv := profile.ResponseConvention; conv != nil && conv.Confidence >= 0.5 {
		sb.WriteString("【项目响应约定】\n")
		sb.WriteString("该项目 API 使用统一响应格式")
		if len(conv.WrapperFields) > 0 {
			sb.WriteString("，顶层字段: ")
			sb.WriteString(strings.Join(conv.WrapperFields, ", "))
		}
		sb.WriteString("。\n")
		if conv.SuccessField != "" {
			sb.WriteString(fmt.Sprintf("- 成功标志: `%s` == %v\n", conv.SuccessField, conv.SuccessValue))
		}
		if conv.DataField != "" {
			sb.WriteString(fmt.Sprintf("- 数据字段: `%s`\n", conv.DataField))
		}
		if conv.MessageField != "" {
			sb.WriteString(fmt.Sprintf("- 消息字段: `%s`\n", conv.MessageField))
		}
		sb.WriteString("生成断言时请基于此约定，除了 status_code 外还应检查业务状态码和数据字段。\n\n")
	}

	// Auth patterns.
	if len(profile.AuthPatterns) > 0 {
		sb.WriteString("【认证模式】\n")
		for _, ap := range profile.AuthPatterns {
			sb.WriteString(fmt.Sprintf("- %s: %s (%.0f%% 请求使用)\n", ap.HeaderName, ap.ValuePattern, ap.Frequency*100))
		}
		sb.WriteString("\n")
	}

	// Field enums (only top ones for prompt size control).
	enumFields := filterEnumFields(profile.FieldProfiles)
	if len(enumFields) > 0 {
		sb.WriteString("【字段枚举值（从历史请求学习）】\n")
		sb.WriteString("以下字段有确定的取值范围，生成参数时请从中选取：\n")
		for _, fp := range enumFields {
			values := formatValues(fp.ObservedValues)
			sb.WriteString(fmt.Sprintf("- %s %s → %s.%s: [%s]\n",
				strings.ToUpper(fp.Method), fp.Path, fp.Location, fp.FieldPath, values))
		}
		sb.WriteString("\n")
	}

	// Dependency graph.
	if len(profile.DependencyGraph) > 0 {
		sb.WriteString("【接口依赖关系】\n")
		sb.WriteString("以下接口存在数据依赖，编排场景时需按依赖顺序排列并传递变量：\n")
		for _, dep := range profile.DependencyGraph {
			if dep.Confidence < 0.5 {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s %s 的 `%s` → %s %s 的 `%s`\n",
				strings.ToUpper(dep.SourceMethod), dep.SourcePath, dep.SourceField,
				strings.ToUpper(dep.TargetMethod), dep.TargetPath, dep.TargetField))
		}
		sb.WriteString("场景步骤中应使用 `{{$steps[N].response.body.xxx}}` 传递依赖数据。\n\n")
	}

	return sb.String()
}

// FormatConventionForPrompt formats just the response convention part for
// narrower injection (e.g. generate_params context).
func FormatConventionForPrompt(conv *ResponseConvention) string {
	if conv == nil || conv.Confidence < 0.5 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("该项目 API 响应格式约定: ")
	if conv.SuccessField != "" {
		sb.WriteString(fmt.Sprintf("%s==%v 表示成功", conv.SuccessField, conv.SuccessValue))
		if conv.DataField != "" {
			sb.WriteString(fmt.Sprintf(", 业务数据在 %s 字段", conv.DataField))
		}
	}
	sb.WriteString("。")
	return sb.String()
}

func filterEnumFields(fields []FieldProfile) []FieldProfile {
	const maxPromptEnums = 20
	var enums []FieldProfile
	for _, fp := range fields {
		if fp.IsEnum && len(fp.ObservedValues) > 0 && len(fp.ObservedValues) <= 10 {
			enums = append(enums, fp)
		}
		if len(enums) >= maxPromptEnums {
			break
		}
	}
	return enums
}

func formatValues(values []any) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		switch val := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s"`, val))
		default:
			b, _ := json.Marshal(val)
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, ", ")
}
