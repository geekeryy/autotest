package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"autotest/internal/templating"
)

// mockHook returns the templating.Resolver.Mock function for the current
// rendering pass. When mockCfg is nil callers fall back to the standard
// mockdata catalogue (no mock value set support); otherwise the hook also
// resolves `{{$mock.set.<key>}}` family tokens through the configured
// MockSetResolver.
func mockHook(mockCfg *templating.MockExpanderConfig) func(templating.Token) (string, bool) {
	if mockCfg == nil {
		return templating.ExpandMock
	}
	return templating.NewMockExpander(*mockCfg)
}

// renderVariables substitutes Mustache placeholders in the input string.
//
// Substitution order matters and is performed each iteration:
//  1. `{{$mock.*}}` runtime data tokens are expanded first so that every
//     occurrence produces an independent random value.
//  2. Regular `{{varName}}` placeholders are replaced from the vars map.
//
// The loop runs up to five times to support nested cases where a variable's
// value itself contains another placeholder; mock-data tokens are
// re-expanded each pass so values produced by chained substitutions remain
// fully rendered while still emitting a fresh random value at the moment
// they first appear.
//
// mockCfg is optional: when non-nil it enables `{{$mock.set.*}}` resolution
// against the project-scoped value sets and the run-shared cursor map.
func renderVariables(input string, vars map[string]string, mockCfg *templating.MockExpanderConfig) string {
	return templating.Render(input, templating.Resolver{
		Mock: mockHook(mockCfg),
		Var:  templating.VarsResolver(vars),
	})
}

// renderPathVariables substitutes both Mustache-style variables and OpenAPI
// path variables like {id}. Path parameter values may themselves contain
// runtime templates, so each substituted value is rendered before use.
func renderPathVariables(input string, vars map[string]string, mockCfg *templating.MockExpanderConfig) string {
	rendered := renderVariables(input, vars, mockCfg)
	for i := 0; i < 5; i++ {
		before := rendered
		rendered = renderOpenAPIPathVariables(rendered, vars, mockCfg)
		rendered = renderVariables(rendered, vars, mockCfg)
		if rendered == before {
			break
		}
	}
	return rendered
}

func renderOpenAPIPathVariables(input string, vars map[string]string, mockCfg *templating.MockExpanderConfig) string {
	var b strings.Builder
	for i := 0; i < len(input); {
		if input[i] == '{' && (i+1 >= len(input) || input[i+1] != '{') {
			if rel := strings.IndexByte(input[i+1:], '}'); rel >= 0 {
				end := i + 1 + rel
				name := strings.TrimSpace(input[i+1 : end])
				if pathVarIdentRE.MatchString(name) {
					if value, ok := vars[name]; ok {
						b.WriteString(renderVariables(value, vars, mockCfg))
						i = end + 1
						continue
					}
				}
			}
		}
		b.WriteByte(input[i])
		i++
	}
	return b.String()
}

func renderMap(input map[string]string, vars map[string]string, mockCfg *templating.MockExpanderConfig) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[renderVariables(key, vars, mockCfg)] = renderVariables(value, vars, mockCfg)
	}
	return out
}

func renderAny(input any, vars map[string]string, mockCfg *templating.MockExpanderConfig) any {
	switch value := input.(type) {
	case string:
		return renderVariables(value, vars, mockCfg)
	case []any:
		out := make([]any, len(value))
		for idx, item := range value {
			out[idx] = renderAny(item, vars, mockCfg)
		}
		return out
	case map[string]any:
		if token, ok := bareTemplateToken(value); ok {
			return renderBareTemplateValue(token, vars, mockCfg)
		}
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[renderVariables(key, vars, mockCfg)] = renderAny(item, vars, mockCfg)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[renderVariables(key, vars, mockCfg)] = renderVariables(item, vars, mockCfg)
		}
		return out
	default:
		return input
	}
}

// exactBareTemplateToken classifies a string that is wholly a single
// placeholder so the JSON-body marshaller can restore the rendered value's
// native scalar type (mirroring the historical behaviour of the
// exactMockValuePattern / exactStepValuePattern regexes).
func exactBareTemplateToken(input string) (templating.Token, bool) {
	trimmed := strings.TrimSpace(input)
	tokens := templating.Tokenize(trimmed)
	if len(tokens) != 1 {
		return templating.Token{}, false
	}
	switch tokens[0].Kind {
	case templating.KindMock, templating.KindStep:
		return tokens[0], true
	}
	return templating.Token{}, false
}

func bareTemplateToken(input map[string]any) (string, bool) {
	if len(input) != 1 {
		return "", false
	}
	token, ok := input["__autotestBareTemplate"].(string)
	return token, ok && token != ""
}

func renderBareTemplateValue(input string, vars map[string]string, mockCfg *templating.MockExpanderConfig) any {
	rendered := renderVariables(input, vars, mockCfg)
	tok, ok := exactBareTemplateToken(input)
	if !ok {
		return rendered
	}
	if tok.Kind == templating.KindStep {
		if value, ok := parseBareTemplateScalar(rendered); ok {
			return value
		}
		return rendered
	}
	switch strings.ToLower(strings.TrimSpace(tok.Mock.Helper)) {
	case "int", "integer", "timestamp":
		if n, err := strconv.ParseInt(rendered, 10, 64); err == nil {
			return n
		}
	case "float", "number":
		if n, err := strconv.ParseFloat(rendered, 64); err == nil {
			return n
		}
	case "bool", "boolean":
		if b, err := strconv.ParseBool(rendered); err == nil {
			return b
		}
	case "set":
		if v, ok := parseBareTemplateScalar(rendered); ok {
			return v
		}
		dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(rendered)))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return rendered
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return rendered
		}
		switch v.(type) {
		case map[string]any, []any:
			return v
		default:
			return rendered
		}
	}
	return rendered
}

func parseBareTemplateScalar(input string) (any, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(trimmed)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false
	}
	switch v := value.(type) {
	case nil, bool, string:
		return v, true
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			if n, err := strconv.ParseFloat(v.String(), 64); err == nil {
				return n, true
			}
			return nil, false
		}
		if n, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return n, true
		}
		if n, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return n, true
		}
		return nil, false
	default:
		return nil, false
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

func injectStepRefs(text string, stepOutputs map[int]any, vars map[string]string) {
	if len(stepOutputs) == 0 {
		return
	}
	templating.StepRefs(text, vars, func(seq int, path string) (string, bool) {
		return resolveStepRef(seq, path, stepOutputs)
	})
}

func resolveStepRef(seq int, path string, stepOutputs map[int]any) (string, bool) {
	output, ok := stepOutputs[seq]
	if !ok {
		return "", false
	}
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

var pathVarIdentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// extractPathVarNames returns the unique variable names referenced in a path
// template. Both {{varName}} (runner style) and {varName} (OpenAPI style) are
// recognised so the function works whether or not the path has been converted
// by the frontend before being stored. Token kinds that carry their own
// semantics (step refs, $mock helpers, SQL/DS references) are intentionally
// ignored: they are resolved by the runner's render pipeline rather than the
// path-parameter table.
func extractPathVarNames(path string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, tok := range templating.Tokenize(path) {
		if tok.Kind != templating.KindVar {
			continue
		}
		name := strings.TrimSpace(tok.Var.Name)
		if pathVarIdentRE.MatchString(name) && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, name := range extractOpenAPIPathVarNames(path) {
		if pathVarIdentRE.MatchString(name) && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func extractOpenAPIPathVarNames(input string) []string {
	var names []string
	for i := 0; i < len(input); {
		if input[i] == '{' && (i+1 >= len(input) || input[i+1] != '{') {
			if rel := strings.IndexByte(input[i+1:], '}'); rel >= 0 {
				end := i + 1 + rel
				name := strings.TrimSpace(input[i+1 : end])
				if pathVarIdentRE.MatchString(name) {
					names = append(names, name)
					i = end + 1
					continue
				}
			}
		}
		i++
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
