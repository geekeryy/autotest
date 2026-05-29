package aitools

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	if got := cosineSimilarity([]float32{1, 0}, []float32{1, 0}); math.Abs(got-1) > 1e-6 {
		t.Fatalf("expected 1, got %v", got)
	}
	if got := cosineSimilarity([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestReciprocalRankFusion_PrefersBothLists(t *testing.T) {
	a := []FindToolResult{{Name: "alpha"}, {Name: "beta"}}
	b := []FindToolResult{{Name: "beta"}, {Name: "gamma"}}
	merged := reciprocalRankFusion(a, b, 60)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged hits, got %d", len(merged))
	}
	if merged[0].Name != "beta" {
		t.Fatalf("expected beta on top, got %s", merged[0].Name)
	}
}

func TestSearchCatalogHybrid_VectorOnlyWhenKeywordMisses(t *testing.T) {
	cat := sampleCatalog()
	idx, err := BuildToolEmbeddingIndex("test-model", []string{
		"create_scenario_with_steps",
		"create_mock_route",
	}, [][]float32{
		{1, 0, 0},
		{0, 1, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	cat = cat.WithEmbeddings(idx)

	ctx := WithQueryEmbedder(context.Background(), &ClientQueryEmbedder{
		ModelName: "test-model",
		EmbedFn: func(_ context.Context, _ string, inputs []string) ([][]float32, error) {
			// Semantic-ish query maps to scenario tool vector.
			if len(inputs) == 1 && inputs[0] == "编排测试流程" {
				return [][]float32{{0.95, 0.05, 0}}, nil
			}
			return nil, nil
		},
	})

	out := searchCatalogHybrid(ctx, cat, "编排测试流程", "", 3)
	if len(out) == 0 || out[0].Name != "create_scenario_with_steps" {
		t.Fatalf("expected scenario tool first, got %+v", out)
	}
}

func TestSearchCatalogHybrid_FallsBackToKeywordWithoutEmbedder(t *testing.T) {
	cat := sampleCatalog().WithEmbeddings(&ToolEmbeddingIndex{
		Model:      "test-model",
		Dimensions: 3,
		Vectors: map[string][]float32{
			"create_scenario_with_steps": {1, 0, 0},
		},
	})
	out := searchCatalogHybrid(context.Background(), cat, "创建场景", "", 5)
	if len(out) == 0 || out[0].Name != "create_scenario_with_steps" {
		t.Fatalf("keyword fallback failed: %+v", out)
	}
}

func TestToolEmbedText_StableFields(t *testing.T) {
	text := ToolEmbedText(Tool{
		Name:        "create_scenario_with_steps",
		Domain:      "scenarios",
		Summary:     "一次性创建场景及其全部步骤",
		Description: "Creates a scenario with steps.",
	})
	if text == "" {
		t.Fatal("expected non-empty embed text")
	}
	for _, want := range []string{"name:", "domain:", "summary:", "description:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}
