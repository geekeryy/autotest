package aitools

import (
	"fmt"
	"strings"
)

const maxEmbedDescriptionRunes = 400

// ToolEmbedText builds the canonical text blob embedded for catalog retrieval.
// Keep field order stable so regenerated vectors stay comparable across releases.
func ToolEmbedText(t Tool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", strings.TrimSpace(t.Name))
	if d := strings.TrimSpace(t.Domain); d != "" {
		fmt.Fprintf(&b, "domain: %s\n", d)
	}
	if s := strings.TrimSpace(t.Summary); s != "" {
		fmt.Fprintf(&b, "summary: %s\n", s)
	}
	if desc := strings.TrimSpace(t.Description); desc != "" {
		desc = truncateRunes(desc, maxEmbedDescriptionRunes)
		fmt.Fprintf(&b, "description: %s\n", desc)
	}
	return strings.TrimSpace(b.String())
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
