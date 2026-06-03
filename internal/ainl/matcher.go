package ainl

import (
	"fmt"
	"strings"

	"autotest/internal/spec"
)

// MatchResult 表示端点匹配的结果。
type MatchResult struct {
	Endpoint   *spec.Endpoint `json:"endpoint,omitempty"`
	Confidence string         `json:"confidence"` // exact, high, medium, low, none
	Reason     string         `json:"reason,omitempty"`
}

// EndpointMatcher 将 LLM 输出的操作映射到实际的 API 端点。
type EndpointMatcher struct {
	endpoints []spec.Endpoint
	// index 按 "METHOD:/path" 建立精确匹配索引
	index map[string]*spec.Endpoint
}

// NewEndpointMatcher 创建端点匹配器。
func NewEndpointMatcher(endpoints []spec.Endpoint) *EndpointMatcher {
	m := &EndpointMatcher{
		endpoints: endpoints,
		index:     make(map[string]*spec.Endpoint, len(endpoints)),
	}
	for i := range endpoints {
		key := strings.ToUpper(endpoints[i].Method) + ":" + endpoints[i].Path
		m.index[key] = &endpoints[i]
	}
	return m
}

// Match 精确匹配端点。优先精确匹配（method + path），其次模糊匹配。
func (m *EndpointMatcher) Match(method, path string) MatchResult {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)

	// 1. 精确匹配
	if ep, ok := m.index[method+":"+path]; ok {
		return MatchResult{
			Endpoint:   ep,
			Confidence: "exact",
			Reason:     "method + path 精确匹配",
		}
	}

	// 2. 模糊匹配：path 相似度 + method 一致
	bestScore := 0.0
	var bestEP *spec.Endpoint
	for i := range m.endpoints {
		ep := &m.endpoints[i]
		if !strings.EqualFold(ep.Method, method) {
			continue
		}
		score := pathSimilarity(path, ep.Path)
		if score > bestScore {
			bestScore = score
			bestEP = ep
		}
	}

	if bestEP != nil && bestScore >= 0.6 {
		confidence := "medium"
		if bestScore >= 0.85 {
			confidence = "high"
		}
		return MatchResult{
			Endpoint:   bestEP,
			Confidence: confidence,
			Reason:     "path 模糊匹配（相似度 " + formatScore(bestScore) + "）",
		}
	}

	// 3. method 匹配但 path 差异较大
	if bestEP != nil {
		return MatchResult{
			Endpoint:   bestEP,
			Confidence: "low",
			Reason:     "仅 method 匹配，path 差异较大（相似度 " + formatScore(bestScore) + "）",
		}
	}

	return MatchResult{
		Confidence: "none",
		Reason:     "未找到匹配的端点",
	}
}

// MatchAll 将 LLM 输出的步骤列表全部映射到端点。
// 返回每步的匹配结果（与输入 steps 一一对应）。
func (m *EndpointMatcher) MatchAll(steps []LLMStep) []MatchResult {
	results := make([]MatchResult, len(steps))
	for i, step := range steps {
		if step.StepType != "api" {
			results[i] = MatchResult{
				Confidence: "skip",
				Reason:     "非 API 类型步骤，无需端点匹配",
			}
			continue
		}
		results[i] = m.Match(step.Method, step.Path)
	}
	return results
}

// pathSimilarity 计算两个路径的相似度（0.0 ~ 1.0）。
// 策略：按 "/" 分段，计算最长公共子序列比率。
func pathSimilarity(a, b string) float64 {
	// 归一化：去除尾部斜杠，统一小写
	a = strings.TrimSuffix(strings.ToLower(a), "/")
	b = strings.TrimSuffix(strings.ToLower(b), "/")

	if a == b {
		return 1.0
	}

	segmentsA := strings.Split(a, "/")
	segmentsB := strings.Split(b, "/")

	// 处理路径参数：{id} 等同于任意段
	normalizeParams := func(segs []string) []string {
		out := make([]string, len(segs))
		for i, s := range segs {
			if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
				out[i] = "*"
			} else {
				out[i] = s
			}
		}
		return out
	}

	segmentsA = normalizeParams(segmentsA)
	segmentsB = normalizeParams(segmentsB)

	// LCS 计算
	lcs := longestCommonSubsequence(segmentsA, segmentsB)
	maxLen := len(segmentsA)
	if len(segmentsB) > maxLen {
		maxLen = len(segmentsB)
	}
	if maxLen == 0 {
		return 1.0
	}

	baseScore := float64(lcs) / float64(maxLen)

	// 额外加分：前缀匹配
	prefixBonus := 0.0
	minLen := len(segmentsA)
	if len(segmentsB) < minLen {
		minLen = len(segmentsB)
	}
	commonPrefix := 0
	for i := 0; i < minLen; i++ {
		if segmentsA[i] == segmentsB[i] || segmentsA[i] == "*" || segmentsB[i] == "*" {
			commonPrefix++
		} else {
			break
		}
	}
	if commonPrefix > 0 {
		prefixBonus = float64(commonPrefix) / float64(maxLen) * 0.2
	}

	score := baseScore + prefixBonus
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// longestCommonSubsequence 计算两个字符串切片的 LCS 长度。
func longestCommonSubsequence(a, b []string) int {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return 0
	}
	// 使用滚动数组优化空间
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = prev[j]
				if curr[j-1] > curr[j] {
					curr[j] = curr[j-1]
				}
			}
		}
		prev, curr = curr, prev
		for k := range curr {
			curr[k] = 0
		}
	}
	return prev[n]
}

// formatScore 格式化相似度分数为百分比字符串。
func formatScore(score float64) string {
	pct := int(score * 100)
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("%d%%", pct)
}
