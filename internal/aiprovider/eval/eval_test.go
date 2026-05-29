package eval

import (
	"path/filepath"
	"testing"
)

// goldenDir resolves the repo's testdata/ai_assistant_eval relative to this
// package (internal/aiprovider/eval).
func goldenDir() string {
	return filepath.Join("..", "..", "..", "testdata", "ai_assistant_eval")
}

// TestGoldenSet runs the rule-based Router over the committed Golden Set and
// fails if any case regresses (missing expected tool, leaked forbidden tool,
// or uncovered domain). This is the CI regression gate referenced by
// `make ai-eval`.
func TestGoldenSet(t *testing.T) {
	cases, err := LoadCases(goldenDir())
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	if len(cases) < 2 {
		t.Fatalf("expected at least 2 golden cases, got %d", len(cases))
	}
	cat := DefaultCatalog()
	rep := Evaluate(cases, cat)
	for _, r := range rep.Results {
		if r.Passed {
			continue
		}
		t.Errorf("case %q failed: missingTools=%v forbidHits=%v missingDomain=%v (recall=%.2f)",
			r.Name, r.MissingTools, r.ForbidHits, r.MissingDomain, r.RecallRate())
	}
	if !rep.Passed() {
		t.Fatalf("golden set has failures; mean recall=%.2f misroute=%.2f", rep.MeanRecall(), rep.MisrouteRate())
	}
}

func TestDefaultCatalogNonEmpty(t *testing.T) {
	cat := DefaultCatalog()
	if cat.Len() < 30 {
		t.Fatalf("expected a populated catalog, got %d tools", cat.Len())
	}
}
