// Package specdiff computes a structured diff between two OpenAPI/Swagger
// normalized snapshots produced by `internal/spec.Importer`.
//
// The diff is intentionally generic: it walks two JSON trees side by side and
// emits add / remove / change entries with stable field paths so downstream
// consumers (notably the AI analysis prompt) can describe the impact in
// natural language without re-implementing schema diff logic.
//
// The comparator has one OpenAPI-aware tweak: `requestSchema.parameters` is
// an array but ordering carries no semantic meaning. We rekey it by
// `name + in` before recursive comparison so reordering does not register as
// a change but renaming or removing a parameter does.
package specdiff

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Diff is the top-level diff result returned to callers.
//
// Each slice is non-nil even when empty so JSON consumers can iterate without
// nil checks. Endpoint comparison keys on `method + path`; renaming either
// surfaces as a remove + add pair (not a modify), which matches the
// principle that an endpoint with a different path is a different endpoint.
type Diff struct {
	AddedEndpoints    []EndpointSummary  `json:"addedEndpoints"`
	RemovedEndpoints  []EndpointSummary  `json:"removedEndpoints"`
	ModifiedEndpoints []ModifiedEndpoint `json:"modifiedEndpoints"`
}

// EndpointSummary is a lightweight identifier for an endpoint, used for
// added/removed lists and as the parent key on modified entries.
type EndpointSummary struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary,omitempty"`
}

// ModifiedEndpoint describes one endpoint whose `method + path` exists in
// both snapshots but whose request or response schema differs.
type ModifiedEndpoint struct {
	Method  string        `json:"method"`
	Path    string        `json:"path"`
	Summary string        `json:"summary,omitempty"`
	Changes []FieldChange `json:"changes"`
}

// FieldChange captures a single delta on an endpoint. `FieldPath` is a
// dot/bracket-joined path rooted at the endpoint object (e.g.
// `requestSchema.parameters[id:path].schema.type`,
// `requestSchema.body.properties.email.required`).
type FieldChange struct {
	FieldPath string `json:"fieldPath"`
	Kind      string `json:"kind"` // ChangeKindAdd | ChangeKindRemove | ChangeKindChange
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

// Change kinds. Constants are exported so tests and AI prompt builders can
// reference them without hard-coding strings.
const (
	ChangeKindAdd    = "add"
	ChangeKindRemove = "remove"
	ChangeKindChange = "change"
)

// snapshotShape mirrors the structure produced by `internal/spec.Importer`:
//
//	{
//	  "title":     "...",
//	  "version":   "...",
//	  "endpoints": [ { method, path, summary, requestSchema, responseSchema, ... } ]
//	}
type snapshotShape struct {
	Title     string                   `json:"title,omitempty"`
	Version   string                   `json:"version,omitempty"`
	Endpoints []map[string]any         `json:"endpoints,omitempty"`
}

// Diff computes the structural difference between two snapshots.
//
// Either side may be empty (`nil` or `[]`) to express "no previous spec" /
// "no current spec". When both are non-empty but JSON-malformed the function
// returns the parse error so callers can surface it to the user instead of
// silently producing an empty diff.
func DiffSnapshots(prev, curr json.RawMessage) (Diff, error) {
	prevShape, err := parseSnapshot(prev)
	if err != nil {
		return Diff{}, fmt.Errorf("解析旧 spec 快照失败: %w", err)
	}
	currShape, err := parseSnapshot(curr)
	if err != nil {
		return Diff{}, fmt.Errorf("解析新 spec 快照失败: %w", err)
	}

	prevByKey := indexEndpoints(prevShape.Endpoints)
	currByKey := indexEndpoints(currShape.Endpoints)

	keys := unionKeys(prevByKey, currByKey)

	out := Diff{
		AddedEndpoints:    []EndpointSummary{},
		RemovedEndpoints:  []EndpointSummary{},
		ModifiedEndpoints: []ModifiedEndpoint{},
	}

	for _, key := range keys {
		prevEndpoint, hasPrev := prevByKey[key]
		currEndpoint, hasCurr := currByKey[key]
		switch {
		case !hasPrev && hasCurr:
			out.AddedEndpoints = append(out.AddedEndpoints, summaryOf(currEndpoint))
		case hasPrev && !hasCurr:
			out.RemovedEndpoints = append(out.RemovedEndpoints, summaryOf(prevEndpoint))
		default:
			changes := diffEndpoint(prevEndpoint, currEndpoint)
			if len(changes) > 0 {
				out.ModifiedEndpoints = append(out.ModifiedEndpoints, ModifiedEndpoint{
					Method:  toString(currEndpoint["method"]),
					Path:    toString(currEndpoint["path"]),
					Summary: toString(currEndpoint["summary"]),
					Changes: changes,
				})
			}
		}
	}

	return out, nil
}

func parseSnapshot(raw json.RawMessage) (snapshotShape, error) {
	if len(raw) == 0 {
		return snapshotShape{}, nil
	}
	var shape snapshotShape
	if err := json.Unmarshal(raw, &shape); err != nil {
		return snapshotShape{}, err
	}
	return shape, nil
}

// indexEndpoints groups endpoint objects by their `METHOD path` key. Method
// is uppercased to match the importer's normalization; path is left as-is so
// `{id}` placeholders compare exactly.
func indexEndpoints(endpoints []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(endpoints))
	for _, endpoint := range endpoints {
		method := upper(toString(endpoint["method"]))
		path := toString(endpoint["path"])
		if method == "" || path == "" {
			continue
		}
		out[method+" "+path] = endpoint
	}
	return out
}

func unionKeys(a, b map[string]map[string]any) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func summaryOf(endpoint map[string]any) EndpointSummary {
	return EndpointSummary{
		Method:  upper(toString(endpoint["method"])),
		Path:    toString(endpoint["path"]),
		Summary: toString(endpoint["summary"]),
	}
}

// diffEndpoint compares the parts of a single endpoint that affect runtime
// behaviour (summary, tags, requestSchema, responseSchema). Identifier-only
// fields like `id`, `serviceId`, `specId`, `fingerprint`, `createdAt`,
// `updatedAt` are intentionally ignored so reimporting the same spec does
// not produce noise.
func diffEndpoint(prev, curr map[string]any) []FieldChange {
	var changes []FieldChange
	fields := []string{"summary", "operationId", "tags", "requestSchema", "responseSchema"}
	for _, field := range fields {
		prevValue, prevHas := prev[field]
		currValue, currHas := curr[field]
		if !prevHas && !currHas {
			continue
		}
		// `requestSchema.parameters` is an unordered array keyed by
		// `name + in`; rewrite it to a map before recursion so the
		// comparator does not mis-attribute reordered slots.
		if field == "requestSchema" {
			prevValue = preprocessRequestSchema(prevValue)
			currValue = preprocessRequestSchema(currValue)
		}
		changes = appendChanges(changes, field, prevValue, currValue, prevHas, currHas)
	}
	return changes
}

// preprocessRequestSchema rewrites the `parameters` array (if any) to a map
// keyed by `name|in` so a reordering does not look like a series of changes.
// Other parts of requestSchema (body, security) are left untouched so
// diffValue can recurse normally.
func preprocessRequestSchema(value any) any {
	asMap, ok := value.(map[string]any)
	if !ok {
		return value
	}
	clone := make(map[string]any, len(asMap))
	for key, val := range asMap {
		clone[key] = val
	}
	rawParams, has := clone["parameters"]
	if !has {
		return clone
	}
	asSlice, ok := rawParams.([]any)
	if !ok {
		return clone
	}
	keyed := make(map[string]any, len(asSlice))
	for index, raw := range asSlice {
		paramMap, ok := raw.(map[string]any)
		if !ok {
			keyed[fmt.Sprintf("[%d]", index)] = raw
			continue
		}
		name := toString(paramMap["name"])
		in := toString(paramMap["in"])
		key := name + "|" + in
		if name == "" && in == "" {
			key = fmt.Sprintf("[%d]", index)
		}
		keyed[key] = paramMap
	}
	clone["parameters"] = keyed
	return clone
}

// appendChanges decides whether to emit add/remove/change for a single field
// pair, then delegates into diffValue for nested differences.
func appendChanges(changes []FieldChange, fieldPath string, prevValue, currValue any, prevHas, currHas bool) []FieldChange {
	switch {
	case !prevHas && currHas:
		return append(changes, FieldChange{
			FieldPath: fieldPath,
			Kind:      ChangeKindAdd,
			After:     currValue,
		})
	case prevHas && !currHas:
		return append(changes, FieldChange{
			FieldPath: fieldPath,
			Kind:      ChangeKindRemove,
			Before:    prevValue,
		})
	default:
		return append(changes, diffValue(fieldPath, prevValue, currValue)...)
	}
}

// diffValue is the workhorse recursive comparator. It walks two JSON trees in
// parallel, emitting `FieldChange` entries with stable dot/bracket paths.
// The function intentionally tolerates `any` (decoded from json.Unmarshal) so
// callers do not need a typed schema — the shape of `requestSchema` /
// `responseSchema` varies across OpenAPI documents.
func diffValue(path string, prev, curr any) []FieldChange {
	if jsonEqual(prev, curr) {
		return nil
	}
	switch prevTyped := prev.(type) {
	case map[string]any:
		currTyped, ok := curr.(map[string]any)
		if !ok {
			return []FieldChange{{FieldPath: path, Kind: ChangeKindChange, Before: prev, After: curr}}
		}
		return diffMap(path, prevTyped, currTyped)
	case []any:
		currTyped, ok := curr.([]any)
		if !ok {
			return []FieldChange{{FieldPath: path, Kind: ChangeKindChange, Before: prev, After: curr}}
		}
		return diffSlice(path, prevTyped, currTyped)
	default:
		return []FieldChange{{FieldPath: path, Kind: ChangeKindChange, Before: prev, After: curr}}
	}
}

func diffMap(path string, prev, curr map[string]any) []FieldChange {
	keys := mapUnionKeys(prev, curr)
	var out []FieldChange
	for _, key := range keys {
		prevValue, prevHas := prev[key]
		currValue, currHas := curr[key]
		out = appendChanges(out, joinPath(path, key), prevValue, currValue, prevHas, currHas)
	}
	return out
}

func mapUnionKeys(a, b map[string]any) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func diffSlice(path string, prev, curr []any) []FieldChange {
	if len(prev) == 0 && len(curr) == 0 {
		return nil
	}
	max := len(prev)
	if len(curr) > max {
		max = len(curr)
	}
	var out []FieldChange
	for index := 0; index < max; index++ {
		entryPath := fmt.Sprintf("%s[%d]", path, index)
		switch {
		case index >= len(prev):
			out = append(out, FieldChange{FieldPath: entryPath, Kind: ChangeKindAdd, After: curr[index]})
		case index >= len(curr):
			out = append(out, FieldChange{FieldPath: entryPath, Kind: ChangeKindRemove, Before: prev[index]})
		default:
			out = append(out, diffValue(entryPath, prev[index], curr[index])...)
		}
	}
	return out
}

// jsonEqual compares two decoded JSON values for deep structural equality. We
// implement this manually rather than using reflect.DeepEqual so we can
// tolerate `json.Number` vs `float64` mismatches that arise when one side was
// decoded with `UseNumber` and the other was not.
func jsonEqual(a, b any) bool {
	rawA, errA := json.Marshal(a)
	rawB, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(rawA) == string(rawB)
}

func joinPath(parent, key string) string {
	// `parameters[id|path]` style: when the key contains `|` we treat it as
	// a synthetic compound key produced by preprocessRequestSchema and use
	// bracket notation for readability.
	if isCompoundKey(key) {
		return fmt.Sprintf("%s[%s]", parent, key)
	}
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func isCompoundKey(key string) bool {
	for _, ch := range key {
		if ch == '|' {
			return true
		}
	}
	return false
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func upper(s string) string {
	out := make([]byte, 0, len(s))
	for _, ch := range []byte(s) {
		if ch >= 'a' && ch <= 'z' {
			out = append(out, ch-32)
			continue
		}
		out = append(out, ch)
	}
	return string(out)
}
