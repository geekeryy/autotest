package testprofile

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const maxExamples = 10

// SelectExamples picks the highest-quality test cases from a project to serve
// as few-shot examples for AI generation. Quality is determined by:
//   - Has manual assertions (human-verified) → highest priority
//   - Has been run successfully → good signal
//   - Diverse coverage (different tags/paths) → prevents over-fitting
func SelectExamples(cases []CaseInfo, targetMethod, targetPath string) []uuid.UUID {
	type scored struct {
		id    uuid.UUID
		score float64
		path  string
	}

	var candidates []scored
	for _, c := range cases {
		if c.Source == "derived" {
			continue
		}
		score := scoreCaseQuality(c, targetMethod, targetPath)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, scored{id: c.ID, score: score, path: c.Path})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Deduplicate by path to ensure diversity.
	selected := make([]uuid.UUID, 0, maxExamples)
	seenPaths := map[string]int{}
	for _, c := range candidates {
		if len(selected) >= maxExamples {
			break
		}
		basePath := normalizePath(c.path)
		if seenPaths[basePath] >= 2 {
			continue
		}
		selected = append(selected, c.id)
		seenPaths[basePath]++
	}
	return selected
}

// FormatExamplesContext formats selected cases into a context string suitable
// for injection into AI prompts as few-shot examples.
func FormatExamplesContext(cases []CaseInfo, selectedIDs []uuid.UUID) string {
	if len(selectedIDs) == 0 {
		return ""
	}

	idSet := make(map[uuid.UUID]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		idSet[id] = true
	}

	var sb strings.Builder
	sb.WriteString("【项目已有高质量用例参考】\n")
	sb.WriteString("以下用例来自同项目已验证的测试模板，请参考其参数风格和断言模式：\n\n")

	count := 0
	for _, c := range cases {
		if !idSet[c.ID] {
			continue
		}
		count++
		sb.WriteString("示例 ")
		sb.WriteString(strings.Repeat("", 0))
		sb.WriteString(string(rune('0' + count)))
		sb.WriteString(": ")
		sb.WriteString(strings.ToUpper(c.Method))
		sb.WriteString(" ")
		sb.WriteString(c.Path)
		sb.WriteString("\n")

		if len(c.Request) > 0 {
			compact := compactJSONStr(c.Request, 500)
			sb.WriteString("Request: ")
			sb.WriteString(compact)
			sb.WriteString("\n")
		}
		if len(c.Assertions) > 0 && string(c.Assertions) != "null" && string(c.Assertions) != "[]" {
			compact := compactJSONStr(c.Assertions, 300)
			sb.WriteString("Assertions: ")
			sb.WriteString(compact)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")

		if count >= maxExamples {
			break
		}
	}
	return sb.String()
}

func scoreCaseQuality(c CaseInfo, targetMethod, targetPath string) float64 {
	score := 0.0

	// Base: must have request data.
	if len(c.Request) == 0 || string(c.Request) == "{}" {
		return 0
	}

	// Manual assertions are high quality.
	if c.HasManualAssertions {
		score += 3.0
	}

	// Has been run and succeeded.
	if c.SuccessCount > 0 {
		score += 2.0
	} else if c.RunCount > 0 {
		score += 0.5
	}

	// Same HTTP method as target.
	if strings.EqualFold(c.Method, targetMethod) {
		score += 1.0
	}

	// Similar path structure.
	if targetPath != "" && pathSimilarity(c.Path, targetPath) > 0.5 {
		score += 1.5
	}

	// Non-auto source (manual = human crafted).
	if c.Source == "manual" {
		score += 1.0
	}

	// Has response snapshot (more context available).
	if len(c.LastResponseSnapshot) > 0 && string(c.LastResponseSnapshot) != "null" {
		score += 0.5
	}

	return score
}

func pathSimilarity(a, b string) float64 {
	aParts := strings.Split(strings.Trim(a, "/"), "/")
	bParts := strings.Split(strings.Trim(b, "/"), "/")

	matches := 0
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	if maxLen == 0 {
		return 0
	}

	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		if strings.EqualFold(aParts[i], bParts[i]) {
			matches++
		} else if isPathParam(aParts[i]) || isPathParam(bParts[i]) {
			matches++
		}
	}
	return float64(matches) / float64(maxLen)
}

func isPathParam(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func normalizePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var result []string
	for _, p := range parts {
		if isPathParam(p) {
			result = append(result, "{}")
		} else {
			result = append(result, strings.ToLower(p))
		}
	}
	return strings.Join(result, "/")
}

func compactJSONStr(raw json.RawMessage, maxLen int) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		s := string(raw)
		if len(s) > maxLen {
			return s[:maxLen] + "..."
		}
		return s
	}
	compact, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	s := string(compact)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
