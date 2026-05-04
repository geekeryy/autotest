package runner

import "regexp"

func RenderVariables(input string, vars map[string]string) string {
	rendered := input
	for key, value := range vars {
		if key == "" {
			continue
		}
		pattern := regexp.MustCompile(`\{\{+` + regexp.QuoteMeta(key) + `\}\}+`)
		rendered = pattern.ReplaceAllStringFunc(rendered, func(string) string {
			return value
		})
	}
	return rendered
}

func RenderMap(input map[string]string, vars map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[RenderVariables(key, vars)] = RenderVariables(value, vars)
	}
	return out
}
