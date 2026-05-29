package aitools

import (
	"sort"
	"strings"

	"autotest/internal/aiprovider/client"
)

// Catalog is an immutable, name-indexed view over a set of tools built for
// the on-demand tool surface (Router selection + find_tools / describe_tools).
//
// It deliberately separates two shapes:
//
//   - Index() emits a compact, schema-free listing (name / domain / summary /
//     mutating / requiresConfirm) cheap enough to feed wholesale to the
//     Router or a find_tools retrieval step.
//   - Lookup() emits the full wire definitions (name / description /
//     parameters JSON Schema) for the small subset the model actually wants
//     to call, as used by describe_tools.
//
// A Catalog does not own Run closures beyond what the source Tool carried;
// it is a metadata/index helper, not a Registry replacement.
type Catalog struct {
	byName map[string]Tool
	// order holds names sorted lexicographically for deterministic output.
	order []string
	// embeddings holds optional precomputed tool vectors for find_tools.
	embeddings *ToolEmbeddingIndex
}

// NewCatalog builds a catalog from the given tools. Tools with an empty
// name are skipped; on duplicate names the first occurrence wins so callers
// can pass ReadOnly+Mutating concatenations without surprises.
func NewCatalog(tools []Tool) *Catalog {
	c := &Catalog{byName: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		if _, exists := c.byName[name]; exists {
			continue
		}
		c.byName[name] = t
		c.order = append(c.order, name)
	}
	sort.Strings(c.order)
	return c
}

// IndexEntry is the compact, serialisable descriptor used by the Router and
// find_tools. It intentionally omits the JSON Schema and verbose
// description to keep prompt payloads small.
type IndexEntry struct {
	Name            string `json:"name"`
	Domain          string `json:"domain,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Mutating        bool   `json:"mutating"`
	RequiresConfirm bool   `json:"requiresConfirm"`
}

// Index returns every tool's compact descriptor, sorted by name. The result
// never contains JSON Schema or the full Description.
func (c *Catalog) Index() []IndexEntry {
	out := make([]IndexEntry, 0, len(c.order))
	for _, name := range c.order {
		t := c.byName[name]
		out = append(out, IndexEntry{
			Name:            t.Name,
			Domain:          t.Domain,
			Summary:         t.Summary,
			Mutating:        t.Mutating,
			RequiresConfirm: t.RequiresConfirm,
		})
	}
	return out
}

// Lookup returns the full wire definitions (name / description / parameters
// JSON Schema) for the named tools, preserving the caller's order. Unknown
// or blank names are skipped silently so a hallucinated name from the model
// degrades gracefully instead of erroring the whole call.
func (c *Catalog) Lookup(names []string) []client.ToolDefinition {
	out := make([]client.ToolDefinition, 0, len(names))
	for _, n := range names {
		t, ok := c.byName[strings.TrimSpace(n)]
		if !ok {
			continue
		}
		out = append(out, t.ToDefinition())
	}
	return out
}

// ByDomain returns the names of tools in the given domain, sorted. Used by
// the Router to select a tool package. An unknown domain yields an empty,
// non-nil slice.
func (c *Catalog) ByDomain(domain string) []string {
	out := make([]string, 0)
	for _, name := range c.order {
		if c.byName[name].Domain == domain {
			out = append(out, name)
		}
	}
	return out
}

// Get returns the full Tool metadata for a name.
func (c *Catalog) Get(name string) (Tool, bool) {
	t, ok := c.byName[strings.TrimSpace(name)]
	return t, ok
}

// Names returns all tool names in the catalog, sorted.
func (c *Catalog) Names() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// Len reports how many tools the catalog holds.
func (c *Catalog) Len() int { return len(c.order) }

// WithEmbeddings returns the same catalog with precomputed tool vectors
// attached for hybrid find_tools retrieval.
func (c *Catalog) WithEmbeddings(idx *ToolEmbeddingIndex) *Catalog {
	if c == nil {
		return c
	}
	c.embeddings = idx
	return c
}

// Embeddings returns attached tool vectors, if any.
func (c *Catalog) Embeddings() *ToolEmbeddingIndex {
	if c == nil {
		return nil
	}
	return c.embeddings
}
