package aitools

import (
	"context"
)

const defaultRRFK = 60

// searchCatalogHybrid merges keyword and optional vector retrieval. When
// vectors or query embedding are unavailable it degrades to keyword-only
// searchCatalog without changing the wire contract.
func searchCatalogHybrid(ctx context.Context, cat *Catalog, query, domain string, limit int) []FindToolResult {
	if cat == nil {
		return []FindToolResult{}
	}
	keyword := searchCatalogKeyword(cat, query, domain, 0)

	idx := cat.embeddings
	if idx == nil {
		return trimResults(keyword, limit)
	}

	embedder, ok := QueryEmbedderFromContext(ctx)
	if !ok {
		return trimResults(keyword, limit)
	}
	queryVec, err := embedder.EmbedQuery(ctx, query)
	if err != nil || len(queryVec) == 0 {
		return trimResults(keyword, limit)
	}
	if idx.Model != "" && embedder.Model() != "" && idx.Model != embedder.Model() {
		// Model mismatch would rank poorly; stay on keywords.
		return trimResults(keyword, limit)
	}

	vector := idx.VectorSearch(cat, queryVec, domain, 0)
	if len(vector) == 0 {
		return trimResults(keyword, limit)
	}
	if len(keyword) == 0 {
		return trimResults(vector, limit)
	}
	return trimResults(reciprocalRankFusion(keyword, vector, defaultRRFK), limit)
}

func trimResults(in []FindToolResult, limit int) []FindToolResult {
	if limit <= 0 || len(in) <= limit {
		return in
	}
	return in[:limit]
}

// reciprocalRankFusion merges two ranked name lists using RRF.
func reciprocalRankFusion(a, b []FindToolResult, k int) []FindToolResult {
	if k <= 0 {
		k = defaultRRFK
	}
	scores := map[string]float64{}
	meta := map[string]FindToolResult{}

	add := func(list []FindToolResult) {
		for rank, r := range list {
			scores[r.Name] += 1.0 / float64(k+rank+1)
			if _, ok := meta[r.Name]; !ok {
				meta[r.Name] = r
			}
		}
	}
	add(a)
	add(b)

	names := make([]string, 0, len(scores))
	for name := range scores {
		names = append(names, name)
	}
	sortNamesByScore(names, scores, meta)

	out := make([]FindToolResult, 0, len(names))
	for _, name := range names {
		out = append(out, meta[name])
	}
	return out
}

func sortNamesByScore(names []string, scores map[string]float64, meta map[string]FindToolResult) {
	// insertion sort on small N is fine; keeps stable tie-break by name.
	for i := 1; i < len(names); i++ {
		j := i
		for j > 0 {
			a, b := names[j-1], names[j]
			sa, sb := scores[a], scores[b]
			if sa > sb || (sa == sb && a < b) {
				break
			}
			names[j-1], names[j] = names[j], names[j-1]
			j--
		}
	}
}
