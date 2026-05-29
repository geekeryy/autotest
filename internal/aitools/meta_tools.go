package aitools

// meta_tools.go implements the always-on "meta" tools that power the
// on-demand tool surface: find_tools (keyword discovery over the Catalog
// index) and describe_tools (full JSON Schema lookup for a small subset
// the model actually wants to call).
//
// Both tools are read-only (Mutating=false) and expose schemas with
// additionalProperties:false. Like every other built-in they never accept
// a projectId argument — project scoping is enforced from the session
// CallerContext, not from model input.
//
// describe_tools needs a way to tell the conversation loop "these tool
// names should now be mounted on the wire for the next hop". We do that
// with a per-stream DescribeCollector stamped onto the context: the tool
// appends the requested names, the loop reads them after each hop and
// merges them into its DescribedTools set. This keeps the loop coupling
// minimal (a tiny ctx handshake) and makes both halves independently
// testable.

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// DescribeCollector accumulates the tool names the model asked
// describe_tools to expand during a single stream. It is safe for
// concurrent use even though the assistant loop is single-threaded per
// session — tool execution may in principle run on a different goroutine.
type DescribeCollector struct {
	mu    sync.Mutex
	names []string
	seen  map[string]struct{}
}

// NewDescribeCollector returns an empty collector.
func NewDescribeCollector() *DescribeCollector {
	return &DescribeCollector{seen: map[string]struct{}{}}
}

// Add records one or more tool names, de-duplicating while preserving
// first-seen order.
func (d *DescribeCollector) Add(names ...string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = map[string]struct{}{}
	}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := d.seen[n]; ok {
			continue
		}
		d.seen[n] = struct{}{}
		d.names = append(d.names, n)
	}
}

// Names returns a copy of the accumulated names in first-seen order.
func (d *DescribeCollector) Names() []string {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.names))
	copy(out, d.names)
	return out
}

type describeCollectorKey struct{}

// WithDescribeCollector stamps a collector onto ctx so describe_tools can
// report expanded tool names back to the conversation loop.
func WithDescribeCollector(ctx context.Context, c *DescribeCollector) context.Context {
	return context.WithValue(ctx, describeCollectorKey{}, c)
}

// DescribeCollectorFromContext returns the collector stamped earlier, if
// any. The second return is false when no collector is present (e.g. the
// analysis path, where describe_tools is not mounted).
func DescribeCollectorFromContext(ctx context.Context) (*DescribeCollector, bool) {
	c, ok := ctx.Value(describeCollectorKey{}).(*DescribeCollector)
	return c, ok && c != nil
}

const (
	// FindToolsName / DescribeToolsName are the wire names of the meta
	// tools. They are exported so the assistant loop and Router can refer
	// to them as constants rather than magic strings.
	FindToolsName     = "find_tools"
	DescribeToolsName = "describe_tools"

	defaultFindToolsLimit = 8
	maxFindToolsLimit     = 30
)

// FindToolResult is a single hit returned by find_tools. It deliberately
// omits the JSON Schema (callers escalate to describe_tools for that).
type FindToolResult struct {
	Name    string `json:"name"`
	Domain  string `json:"domain,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// FindToolsTool builds the find_tools meta tool over the given catalog.
//
// V1 retrieval uses keyword scoring; when catalog embeddings and a query
// embedder are available, searchCatalogHybrid merges vector similarity via
// RRF (see docs/design/ai-capabilities.md).
func FindToolsTool(cat *Catalog) Tool {
	return Tool{
		Name: FindToolsName,
		Description: "在平台工具目录中按关键词检索可用工具，返回 {name, domain, summary} 列表（不含参数 Schema）。" +
			"当你不确定该用哪个工具来完成某个操作时先调用它发现候选工具，再用 describe_tools 获取参数定义。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query":  {"type": "string", "description": "中文或英文关键词，例如\"创建场景\"、\"mock 路由\"、\"测试数据\""},
				"domain": {"type": "string", "description": "可选，限定域：meta|cases|scenarios|mock|mockset|testdata|paramsource|scripts|runs|spec"},
				"limit":  {"type": "integer", "description": "可选，返回条数上限，默认 8"}
			},
			"required": ["query"],
			"additionalProperties": false
		}`),
		Mutating: false,
		Domain:   "meta",
		Summary:  "按关键词检索平台可用工具目录",
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Query  string `json:"query"`
				Domain string `json:"domain"`
				Limit  int    `json:"limit"`
			}
			if len(strings.TrimSpace(string(args))) > 0 {
				if err := json.Unmarshal(args, &in); err != nil {
					return nil, err
				}
			}
			limit := in.Limit
			if limit <= 0 {
				limit = defaultFindToolsLimit
			}
			if limit > maxFindToolsLimit {
				limit = maxFindToolsLimit
			}
			results := searchCatalogHybrid(ctx, cat, in.Query, in.Domain, limit)
			return map[string]any{
				"results": results,
				"count":   len(results),
			}, nil
		},
	}
}

// DescribeToolsTool builds the describe_tools meta tool over the given
// catalog. It returns the full wire definitions (description + JSON
// Schema) for the named tools and, when a DescribeCollector is present on
// ctx, records those names so the loop mounts them on subsequent hops.
func DescribeToolsTool(cat *Catalog) Tool {
	return Tool{
		Name: DescribeToolsName,
		Description: "获取一组工具的完整定义（描述 + 参数 JSON Schema）。" +
			"在调用任何写工具之前，先用本工具确认其确切参数，避免凭空猜测入参。未知工具名会被安全跳过。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"names": {
					"type": "array",
					"items": {"type": "string"},
					"description": "工具名数组，例如 [\"create_scenario_with_steps\", \"create_case_from_endpoint\"]"
				}
			},
			"required": ["names"],
			"additionalProperties": false
		}`),
		Mutating: false,
		Domain:   "meta",
		Summary:  "获取指定工具的完整参数 Schema",
		Run: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Names []string `json:"names"`
			}
			if len(strings.TrimSpace(string(args))) > 0 {
				if err := json.Unmarshal(args, &in); err != nil {
					return nil, err
				}
			}
			defs := cat.Lookup(in.Names)
			// Record only the names that actually resolved, so the loop
			// never tries to mount a hallucinated name.
			if collector, ok := DescribeCollectorFromContext(ctx); ok {
				resolved := make([]string, 0, len(defs))
				for _, d := range defs {
					resolved = append(resolved, d.Name)
				}
				collector.Add(resolved...)
			}
			return map[string]any{
				"tools": defs,
				"count": len(defs),
			}, nil
		},
	}
}

// scoredEntry pairs an index entry with its computed relevance score.
type scoredEntry struct {
	entry IndexEntry
	score int
}

// searchCatalogKeyword runs keyword retrieval. The scoring is a simple
// additive heuristic: a name match outranks a summary match outranks a
// domain match, and an exact name equality always floats to the top.
func searchCatalogKeyword(cat *Catalog, query, domain string, limit int) []FindToolResult {
	if cat == nil {
		return []FindToolResult{}
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	tokens := tokenizeQuery(query)
	index := cat.Index()

	scored := make([]scoredEntry, 0, len(index))
	for _, e := range index {
		if domain != "" && strings.ToLower(e.Domain) != domain {
			continue
		}
		// find_tools must never surface the meta tools themselves — they
		// are always mounted, so suggesting them wastes the model's
		// attention.
		if e.Name == FindToolsName || e.Name == DescribeToolsName {
			continue
		}
		score := scoreEntry(e, tokens)
		// With no usable query tokens but a domain filter, list the whole
		// domain (score 1) so `find_tools(domain=scenarios)` is useful.
		if len(tokens) == 0 && domain != "" {
			score = 1
		}
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredEntry{entry: e, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].entry.Name < scored[j].entry.Name
	})

	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]FindToolResult, 0, len(scored))
	for _, s := range scored {
		out = append(out, FindToolResult{
			Name:    s.entry.Name,
			Domain:  s.entry.Domain,
			Summary: s.entry.Summary,
		})
	}
	return out
}

// tokenizeQuery lowercases and splits the query on whitespace and common
// separators. CJK text is kept as whole runs (we do not word-segment
// Chinese in V1), so substring matching against the summary still works.
func tokenizeQuery(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', '，', '、', '/', '\\', '_', '-', '.', ':', '：', ';', '；':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func scoreEntry(e IndexEntry, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}
	name := strings.ToLower(e.Name)
	summary := strings.ToLower(e.Summary)
	dom := strings.ToLower(e.Domain)
	score := 0
	for _, tok := range tokens {
		switch {
		case name == tok:
			score += 10
		case strings.Contains(name, tok):
			score += 5
		}
		if strings.Contains(summary, tok) {
			score += 3
		}
		if dom == tok {
			score += 2
		}
	}
	return score
}
