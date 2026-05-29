package aitools

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

//go:embed data/tool_embeddings.json
var embeddedToolEmbeddingsFS embed.FS

// ToolEmbeddingIndex holds precomputed catalog vectors keyed by tool name.
type ToolEmbeddingIndex struct {
	Model      string               `json:"model"`
	Dimensions int                  `json:"dimensions"`
	Vectors    map[string][]float32 `json:"vectors"`
}

// LoadEmbeddedToolEmbeddings parses the committed catalog embedding file
// bundled with the binary. An empty vectors map yields a nil index without
// error so callers can treat "no file yet" as keyword-only mode.
func LoadEmbeddedToolEmbeddings() (*ToolEmbeddingIndex, error) {
	raw, err := embeddedToolEmbeddingsFS.ReadFile("data/tool_embeddings.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded tool embeddings: %w", err)
	}
	return LoadToolEmbeddingsFromFS(raw)
}

// LoadToolEmbeddingsFromFS reads embeddings from an arbitrary JSON blob.
// Used by the generator and tests.
func LoadToolEmbeddingsFromFS(raw []byte) (*ToolEmbeddingIndex, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var idx ToolEmbeddingIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, err
	}
	if len(idx.Vectors) == 0 {
		return nil, nil
	}
	if idx.Dimensions <= 0 {
		for _, v := range idx.Vectors {
			idx.Dimensions = len(v)
			break
		}
	}
	for name, vec := range idx.Vectors {
		if len(vec) != idx.Dimensions {
			return nil, fmt.Errorf("tool %s embedding dim %d != %d", name, len(vec), idx.Dimensions)
		}
	}
	return &idx, nil
}

// WriteToolEmbeddingsJSON serialises an index for commit / code generation.
func WriteToolEmbeddingsJSON(idx *ToolEmbeddingIndex) ([]byte, error) {
	if idx == nil {
		return nil, fmt.Errorf("index is nil")
	}
	names := make([]string, 0, len(idx.Vectors))
	for name := range idx.Vectors {
		names = append(names, name)
	}
	sort.Strings(names)
	ordered := make(map[string][]float32, len(names))
	for _, name := range names {
		ordered[name] = idx.Vectors[name]
	}
	payload := ToolEmbeddingIndex{
		Model:      idx.Model,
		Dimensions: idx.Dimensions,
		Vectors:    ordered,
	}
	return json.MarshalIndent(payload, "", "  ")
}

// VectorSearch ranks catalog tools by cosine similarity to queryVec.
func (idx *ToolEmbeddingIndex) VectorSearch(cat *Catalog, queryVec []float32, domain string, limit int) []FindToolResult {
	if idx == nil || cat == nil || len(queryVec) == 0 {
		return nil
	}
	if idx.Dimensions > 0 && len(queryVec) != idx.Dimensions {
		return nil
	}
	domain = strings.ToLower(strings.TrimSpace(domain))

	type scored struct {
		entry IndexEntry
		score float64
	}
	scoredList := make([]scored, 0, len(idx.Vectors))
	for name, vec := range idx.Vectors {
		if name == FindToolsName || name == DescribeToolsName {
			continue
		}
		t, ok := cat.Get(name)
		if !ok {
			continue
		}
		entry := IndexEntry{
			Name:            t.Name,
			Domain:          t.Domain,
			Summary:         t.Summary,
			Mutating:        t.Mutating,
			RequiresConfirm: t.RequiresConfirm,
		}
		if domain != "" && strings.ToLower(entry.Domain) != domain {
			continue
		}
		sim := cosineSimilarity(queryVec, vec)
		if sim <= 0 {
			continue
		}
		scoredList = append(scoredList, scored{entry: entry, score: sim})
	}

	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].entry.Name < scoredList[j].entry.Name
	})
	if limit > 0 && len(scoredList) > limit {
		scoredList = scoredList[:limit]
	}
	out := make([]FindToolResult, 0, len(scoredList))
	for _, s := range scoredList {
		out = append(out, FindToolResult{
			Name:    s.entry.Name,
			Domain:  s.entry.Domain,
			Summary: s.entry.Summary,
		})
	}
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// BuildToolEmbeddingIndex constructs an index from parallel names and vectors.
func BuildToolEmbeddingIndex(model string, names []string, vectors [][]float32) (*ToolEmbeddingIndex, error) {
	if len(names) != len(vectors) {
		return nil, fmt.Errorf("names/vectors length mismatch")
	}
	idx := &ToolEmbeddingIndex{
		Model:   strings.TrimSpace(model),
		Vectors: make(map[string][]float32, len(names)),
	}
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		vec := vectors[i]
		if len(vec) == 0 {
			return nil, fmt.Errorf("empty vector for %s", name)
		}
		if idx.Dimensions == 0 {
			idx.Dimensions = len(vec)
		}
		if len(vec) != idx.Dimensions {
			return nil, fmt.Errorf("vector dim mismatch for %s", name)
		}
		idx.Vectors[name] = vec
	}
	if len(idx.Vectors) == 0 {
		return nil, fmt.Errorf("no vectors")
	}
	return idx, nil
}

// EmbeddingsDataPath is the repo-relative path written by the generator CLI.
const EmbeddingsDataPath = "internal/aitools/data/tool_embeddings.json"
