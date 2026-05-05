package runner

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func renderVariables(input string, vars map[string]string) string {
	rendered := input
	for i := 0; i < 5; i++ {
		before := rendered
		for key, value := range vars {
			if key == "" {
				continue
			}
			pattern := regexp.MustCompile(`\{\{+` + regexp.QuoteMeta(key) + `\}\}+`)
			rendered = pattern.ReplaceAllStringFunc(rendered, func(string) string {
				return value
			})
		}
		if rendered == before {
			break
		}
	}
	return rendered
}

func renderMap(input map[string]string, vars map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[renderVariables(key, vars)] = renderVariables(value, vars)
	}
	return out
}

func renderAny(input any, vars map[string]string) any {
	switch value := input.(type) {
	case string:
		return renderVariables(value, vars)
	case []any:
		out := make([]any, len(value))
		for idx, item := range value {
			out[idx] = renderAny(item, vars)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[renderVariables(key, vars)] = renderAny(item, vars)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(value))
		for key, item := range value {
			out[renderVariables(key, vars)] = renderVariables(item, vars)
		}
		return out
	default:
		return input
	}
}

// ── Step reference resolution ──────────────────────────────────────────────
//
// Scenario steps can reference outputs of previous steps using the syntax:
//
//	{{$steps[N].path.to.value}}
//
// where N is the permanent step_seq number (never changes on reorder) and
// the remainder is a dot-separated JSONPath into the step's output object.
//
// API step output fields:
//
//	{{$steps[1].status}}                     → HTTP status code (int → string)
//	{{$steps[1].headers.X-Request-Id}}       → response header value
//	{{$steps[1].body.data.token}}            → response JSON body field
//	{{$steps[1].request.query.userId}}       → request query parameter
//	{{$steps[1].request.pathvar.id}}         → request path variable value
//	{{$steps[1].request.body.data.field}}    → request body field
//
// Database step output fields:
//
//	{{$steps[2].firstRow.user_id}}     → first DB result row field
//	{{$steps[2].rows[0].name}}         → specific DB row field
//
// Script step output fields:
//
//	{{$steps[3].stdout}}               → script stdout
//	{{$steps[3].stdoutJson.count}}     → parsed JSON from script stdout
//
// injectStepRefs scans text for all {{$steps[N].*}} patterns, resolves each
// against stepOutputs, and inserts the resolved string into vars so that the
// existing renderVariables pass can perform the substitution.
//
// Callers must create a per-step copy of the vars map before calling this
// function so the injected keys do not leak into subsequent steps.

var stepRefScanRE = regexp.MustCompile(`\{\{(\$steps\[\d+\][^}]*)\}\}`)
var stepRefKeyRE = regexp.MustCompile(`^\$steps\[(\d+)\](.*)$`)

func injectStepRefs(text string, stepOutputs map[int]any, vars map[string]string) {
	if len(stepOutputs) == 0 || !strings.Contains(text, "$steps[") {
		return
	}
	for _, m := range stepRefScanRE.FindAllStringSubmatch(text, -1) {
		key := m[1]
		if _, exists := vars[key]; exists {
			continue
		}
		if val, ok := resolveStepRef(key, stepOutputs); ok {
			vars[key] = val
		}
	}
}

func resolveStepRef(key string, stepOutputs map[int]any) (string, bool) {
	m := stepRefKeyRE.FindStringSubmatch(key)
	if m == nil {
		return "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return "", false
	}
	output, ok := stepOutputs[n]
	if !ok {
		return "", false
	}
	path := strings.TrimPrefix(m[2], ".")
	if path == "" {
		b, _ := json.Marshal(output)
		return string(b), true
	}
	val, err := traverseOutputPath(output, path)
	if err != nil {
		return "", false
	}
	return stepOutputStr(val), true
}

// traverseOutputPath walks a dot-separated JSONPath into a decoded JSON value.
// Array indexing like "items[0]" and "[0]" are supported.
func traverseOutputPath(node any, path string) (any, error) {
	if path == "" {
		return node, nil
	}
	segment, rest, hasMore := strings.Cut(path, ".")
	field, indices, err := parseOutputSegment(segment)
	if err != nil {
		return nil, err
	}
	current := node
	if field != "" {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object at %q, got %T", field, current)
		}
		val, exists := m[field]
		if !exists {
			return nil, fmt.Errorf("field %q not found", field)
		}
		current = val
	}
	for _, idx := range indices {
		arr, ok := current.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array for index [%d], got %T", idx, current)
		}
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("index [%d] out of range (len=%d)", idx, len(arr))
		}
		current = arr[idx]
	}
	if !hasMore {
		return current, nil
	}
	return traverseOutputPath(current, rest)
}

// parseOutputSegment splits a path segment like "items[0][1]" into ("items", [0, 1]).
func parseOutputSegment(segment string) (field string, indices []int, err error) {
	bracket := strings.Index(segment, "[")
	if bracket < 0 {
		return segment, nil, nil
	}
	field = segment[:bracket]
	rest := segment[bracket:]
	for len(rest) > 0 {
		if rest[0] != '[' {
			return "", nil, fmt.Errorf("unexpected character %q in segment %q", rest[0], segment)
		}
		close := strings.Index(rest, "]")
		if close < 0 {
			return "", nil, fmt.Errorf("missing closing ']' in segment %q", segment)
		}
		idx, convErr := strconv.Atoi(rest[1:close])
		if convErr != nil {
			return "", nil, fmt.Errorf("array index %q is not an integer in segment %q", rest[1:close], segment)
		}
		indices = append(indices, idx)
		rest = rest[close+1:]
	}
	return field, indices, nil
}

// pathVarNameRE matches {{varName}} placeholders (2+ braces) used in path templates.
var pathVarNameRE = regexp.MustCompile(`\{\{+([^{}]+)\}\}+`)

// pathVarNameSingleRE matches {varName} placeholders (single braces, OpenAPI style).
var pathVarNameSingleRE = regexp.MustCompile(`(?:^|[^{])\{([^{}]+)\}(?:[^}]|$)`)

// extractPathVarNames returns the unique variable names referenced in a path
// template. Both {{varName}} (runner style) and {varName} (OpenAPI style) are
// recognised so the function works whether or not the path has been converted
// by the frontend before being stored.
func extractPathVarNames(path string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, m := range pathVarNameRE.FindAllStringSubmatch(path, -1) {
		name := strings.TrimSpace(m[1])
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, m := range pathVarNameSingleRE.FindAllStringSubmatch(path, -1) {
		name := strings.TrimSpace(m[1])
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// stepOutputStr converts an arbitrary step output value to a string suitable
// for variable substitution.
func stepOutputStr(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
