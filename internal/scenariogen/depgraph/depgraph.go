// Package depgraph infers producer-consumer dependencies between API
// endpoints from OpenAPI request/response schemas (RESTler-style).
package depgraph

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"autotest/internal/spec"
)

// EdgeKind classifies a dependency between two endpoints.
type EdgeKind string

const (
	EdgeProducerConsumer EdgeKind = "producer_consumer"
	EdgeAuth             EdgeKind = "auth"
	EdgeLifecycle        EdgeKind = "lifecycle"
)

// FieldMapping describes how a prior step's output feeds a later step.
type FieldMapping struct {
	ProducerIndex  int    `json:"producerIndex"`
	ConsumerIndex  int    `json:"consumerIndex"`
	ProducerPath   string `json:"producerPath"`
	ConsumerTarget string `json:"consumerTarget"`
	ConsumerKind   string `json:"consumerKind"` // path_param | body_field
}

// ResourceGroup clusters endpoints that share a resource prefix.
type ResourceGroup struct {
	Name      string `json:"name"`
	Endpoint  []int  `json:"endpointIndices"`
	LoginIdx  int    `json:"loginIndex,omitempty"`
	HasAuth   bool   `json:"hasAuth"`
}

// Graph is the dependency graph for one service's endpoints.
type Graph struct {
	Endpoints    []spec.Endpoint `json:"-"`
	Mappings     []FieldMapping  `json:"mappings"`
	TopoOrder    []int           `json:"topoOrder"`
	Groups       []ResourceGroup `json:"groups"`
	LoginIndices []int           `json:"loginIndices"`
}

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

// Build constructs a dependency graph from api_endpoints rows.
func Build(endpoints []spec.Endpoint) *Graph {
	g := &Graph{Endpoints: append([]spec.Endpoint(nil), endpoints...)}
	if len(g.Endpoints) == 0 {
		return g
	}
	g.discoverProducersConsumers()
	g.discoverAuthEdges()
	g.sortLifecycle()
	g.Groups = g.buildResourceGroups()
	g.TopoOrder = g.topologicalSort()
	return g
}

func (g *Graph) discoverProducersConsumers() {
	producers := make([][]producerField, len(g.Endpoints))
	consumers := make([][]consumerField, len(g.Endpoints))

	for i, ep := range g.Endpoints {
		producers[i] = extractProducers(ep)
		consumers[i] = extractConsumers(ep)
	}

	for consIdx, consList := range consumers {
		for _, cons := range consList {
			bestProd := -1
			bestScore := 0
			for prodIdx, prodList := range producers {
				if prodIdx >= consIdx {
					continue
				}
				for _, prod := range prodList {
					score := matchScore(prod, cons, g.Endpoints[prodIdx].Path, g.Endpoints[consIdx].Path)
					if score > bestScore {
						bestScore = score
						bestProd = prodIdx
					}
				}
			}
			if bestProd >= 0 && bestScore >= 2 {
				g.Mappings = append(g.Mappings, FieldMapping{
					ProducerIndex:  bestProd,
					ConsumerIndex:  consIdx,
					ProducerPath:   cons.resolvedProducerPath(producers[bestProd], cons),
					ConsumerTarget: cons.name,
					ConsumerKind:   cons.kind,
				})
			}
		}
	}
}

func (g *Graph) discoverAuthEdges() {
	for i, ep := range g.Endpoints {
		if isLoginEndpoint(ep) {
			g.LoginIndices = append(g.LoginIndices, i)
		}
	}
	sort.Ints(g.LoginIndices)
}

func (g *Graph) sortLifecycle() {
	// lifecycle ordering is applied per group during planCoverage; topo sort handles global deps.
}

func (g *Graph) buildResourceGroups() []ResourceGroup {
	byTag := map[string][]int{}
	for i, ep := range g.Endpoints {
		tag := "default"
		if len(ep.Tags) > 0 && strings.TrimSpace(ep.Tags[0]) != "" {
			tag = ep.Tags[0]
		}
		byTag[tag] = append(byTag[tag], i)
	}
	tags := make([]string, 0, len(byTag))
	for t := range byTag {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	var groups []ResourceGroup
	for _, tag := range tags {
		indices := byTag[tag]
		sort.Slice(indices, func(a, b int) bool {
			return methodPriority(g.Endpoints[indices[a]]) < methodPriority(g.Endpoints[indices[b]])
		})
		grp := ResourceGroup{Name: tag, Endpoint: indices}
		for _, idx := range indices {
			if isLoginEndpoint(g.Endpoints[idx]) {
				grp.LoginIdx = idx
				break
			}
		}
		for _, idx := range indices {
			if needsBearer(g.Endpoints[idx]) {
				grp.HasAuth = true
				break
			}
		}
		groups = append(groups, grp)
	}
	return groups
}

func (g *Graph) topologicalSort() []int {
	n := len(g.Endpoints)
	indeg := make([]int, n)
	adj := make([][]int, n)
	for _, m := range g.Mappings {
		if m.ProducerIndex == m.ConsumerIndex {
			continue
		}
		adj[m.ProducerIndex] = append(adj[m.ProducerIndex], m.ConsumerIndex)
		indeg[m.ConsumerIndex]++
	}
	// login endpoints first when no incoming edges
	queue := make([]int, 0, n)
	for i := range g.Endpoints {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	sort.Slice(queue, func(a, b int) bool {
		return methodPriority(g.Endpoints[queue[a]]) < methodPriority(g.Endpoints[queue[b]])
	})
	order := make([]int, 0, n)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, next := range adj[cur] {
			indeg[next]--
			if indeg[next] == 0 {
				queue = append(queue, next)
			}
		}
		sort.Slice(queue, func(a, b int) bool {
			return methodPriority(g.Endpoints[queue[a]]) < methodPriority(g.Endpoints[queue[b]])
		})
	}
	if len(order) < n {
		seen := map[int]bool{}
		for _, i := range order {
			seen[i] = true
		}
		for i := range g.Endpoints {
			if !seen[i] {
				order = append(order, i)
			}
		}
	}
	return order
}

// MappingsForConsumer returns field mappings targeting endpoint index.
func (g *Graph) MappingsForConsumer(consumerIdx int) []FieldMapping {
	var out []FieldMapping
	for _, m := range g.Mappings {
		if m.ConsumerIndex == consumerIdx {
			out = append(out, m)
		}
	}
	return out
}

// LoginForGroup returns the login endpoint index for a group, or -1.
func (g *Graph) LoginForGroup(grp ResourceGroup) int {
	if grp.LoginIdx >= 0 && grp.LoginIdx < len(g.Endpoints) && isLoginEndpoint(g.Endpoints[grp.LoginIdx]) {
		return grp.LoginIdx
	}
	for _, idx := range g.LoginIndices {
		for _, gi := range grp.Endpoint {
			if gi == idx {
				return idx
			}
		}
	}
	return -1
}

type producerField struct {
	name     string
	path     string
	resource string
}

type consumerField struct {
	name     string
	kind     string
	resource string
}

func (c consumerField) resolvedProducerPath(prods []producerField, cons consumerField) string {
	for _, p := range prods {
		if strings.EqualFold(p.name, cons.name) || idLikeMatch(p.name, cons.name) {
			return p.path
		}
	}
	if len(prods) > 0 {
		return prods[0].path
	}
	return "body.id"
}

// TokenResponsePath returns the step-output path for a login JWT (e.g. body.token or body.data.token).
func TokenResponsePath(ep spec.Endpoint) string {
	var found string
	walkJSONSchema(jsonValue(ep.ResponseSchema), "response.body", "", func(path, name, _ string) {
		if found == "" && strings.EqualFold(name, "token") {
			found = path
		}
	})
	if found != "" {
		return found
	}
	return "response.body.token"
}

func extractProducers(ep spec.Endpoint) []producerField {
	var out []producerField
	resource := resourcePrefix(ep.Path)
	walkJSONSchema(jsonValue(ep.ResponseSchema), "response.body", resource, func(path, name, _ string) {
		if isIDLike(name) {
			out = append(out, producerField{name: name, path: path, resource: resource})
		}
	})
	if len(out) == 0 {
		out = append(out, producerField{name: "id", path: "response.body.id", resource: resource})
	}
	return out
}

func extractConsumers(ep spec.Endpoint) []consumerField {
	var out []consumerField
	resource := resourcePrefix(ep.Path)
	for _, name := range pathParamRe.FindAllStringSubmatch(ep.Path, -1) {
		if len(name) > 1 {
			out = append(out, consumerField{name: name[1], kind: "path_param", resource: resource})
		}
	}
	walkJSONSchema(jsonValue(ep.RequestSchema), "body", resource, func(path, name, _ string) {
		if isIDLike(name) && !strings.Contains(path, "security") {
			out = append(out, consumerField{name: name, kind: "body_field", resource: resource})
		}
	})
	return out
}

func walkJSONSchema(node any, prefix, resource string, visit func(path, name, typ string)) {
	switch v := node.(type) {
	case map[string]any:
		if props, ok := v["properties"].(map[string]any); ok {
			for name, sub := range props {
				p := prefix + "." + name
				visit(p, name, schemaType(sub))
				walkJSONSchema(sub, p, resource, visit)
			}
		}
		if items, ok := v["items"].(map[string]any); ok {
			walkJSONSchema(items, prefix+"[0]", resource, visit)
		}
	}
}

func schemaType(node any) string {
	m, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	if t, ok := m["type"].(string); ok {
		return t
	}
	return ""
}

func jsonValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return v
}

func isIDLike(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "id" || n == "uuid" {
		return true
	}
	return strings.HasSuffix(n, "id") || strings.HasSuffix(n, "_id")
}

func idLikeMatch(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	return a == b || strings.TrimSuffix(a, "id") == strings.TrimSuffix(b, "id")
}

func matchScore(prod producerField, cons consumerField, prodPath, consPath string) int {
	score := 0
	if strings.EqualFold(prod.name, cons.name) {
		score += 3
	} else if idLikeMatch(prod.name, cons.name) {
		score += 2
	}
	if prod.resource != "" && cons.resource != "" && strings.HasPrefix(consPath, prod.resource) {
		score += 2
	}
	if resourcePrefix(prodPath) == resourcePrefix(consPath) {
		score += 1
	}
	return score
}

func resourcePrefix(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) >= 2 {
		return "/" + strings.Join(parts[:len(parts)-1], "/")
	}
	return "/" + parts[0]
}

func isLoginEndpoint(ep spec.Endpoint) bool {
	if !strings.EqualFold(ep.Method, "POST") {
		return false
	}
	p := strings.ToLower(ep.Path)
	return strings.Contains(p, "/login") || strings.Contains(p, "/auth/login")
}

func needsBearer(ep spec.Endpoint) bool {
	var req map[string]any
	if json.Unmarshal(ep.RequestSchema, &req) != nil {
		return false
	}
	sec, _ := req["security"].([]any)
	return len(sec) > 0
}

func methodPriority(ep spec.Endpoint) int {
	if isLoginEndpoint(ep) {
		return 0
	}
	if !needsBearer(ep) {
		return 1
	}
	switch strings.ToUpper(ep.Method) {
	case "GET":
		return 2
	case "POST":
		return 3
	case "PUT", "PATCH":
		return 4
	case "DELETE":
		return 5
	}
	return 6
}
