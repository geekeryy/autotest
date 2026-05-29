package genagent

import (
	"encoding/json"
	"testing"
)

func TestParseClassificationJSON(t *testing.T) {
	t.Parallel()
	out, err := parseClassification(`{"category":"test_data","reason":"凭据缺失","suggestedFix":"补全用户名"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Category != "test_data" {
		t.Fatalf("category=%q", out.Category)
	}
}

func TestParseClassificationFromMarkdownWrapped(t *testing.T) {
	t.Parallel()
	out, err := parseClassification("说明\n```json\n{\"category\":\"real_defect\",\"reason\":\"500\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if out.Category != "real_defect" {
		t.Fatalf("category=%q", out.Category)
	}
}

func parseClassification(text string) (failureClass, error) {
	text = trimJSON(text)
	var out failureClass
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return failureClass{}, err
	}
	out.Category = normalizeCategory(out.Category)
	return out, nil
}

func trimJSON(text string) string {
	start := -1
	end := -1
	for i, r := range text {
		if r == '{' && start < 0 {
			start = i
		}
		if r == '}' {
			end = i
		}
	}
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func normalizeCategory(c string) string {
	switch c {
	case "test_data", "generation_bug", "real_defect":
		return c
	default:
		return "generation_bug"
	}
}
