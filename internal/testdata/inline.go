package testdata

import (
	"regexp"
	"strings"
)

// inlineTokenPattern matches every `{{$ds...}}` occurrence inside a string.
// It is intentionally lenient so malformed tokens still surface as parse
// errors instead of being silently ignored.
var inlineTokenPattern = regexp.MustCompile(`\{\{\s*\$ds[^}]*\}\}`)

// inlineRefPattern is anchored and parses a single well-formed token into its
// components: table key, optional filter column/value and target column.
var inlineRefPattern = regexp.MustCompile(`^\{\{\s*\$ds\.([A-Za-z0-9_-]+)(?:\[([A-Za-z0-9_]+)=([^\]\r\n]+)\])?\.([A-Za-z0-9_]+)\s*\}\}$`)

// HasInlineToken returns true when s contains any `{{$ds.*}}` occurrence. It
// is a cheap pre-check before allocating regex capture buffers.
func HasInlineToken(s string) bool {
	return strings.Contains(s, "$ds") && inlineTokenPattern.MatchString(s)
}

// ParseInlineReferences returns every `{{$ds.*}}` reference parsed from input.
// Tokens that do not match the expected shape produce a descriptive error to
// surface configuration mistakes early.
func ParseInlineReferences(input string) ([]InlineReference, error) {
	if !HasInlineToken(input) {
		return nil, nil
	}
	tokens := inlineTokenPattern.FindAllString(input, -1)
	refs := make([]InlineReference, 0, len(tokens))
	for _, token := range tokens {
		ref, err := parseInlineToken(token)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func parseInlineToken(token string) (InlineReference, error) {
	match := inlineRefPattern.FindStringSubmatch(token)
	if match == nil {
		return InlineReference{}, &InvalidReferenceError{Token: token}
	}
	return InlineReference{
		Expression:   strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(token), "{{"), "}}"),
		TableKey:     match[1],
		FilterColumn: match[2],
		FilterValue:  unquoteFilterValue(match[3]),
		Column:       match[4],
	}, nil
}

// InvalidReferenceError is returned when an `{{$ds.*}}` token cannot be parsed.
type InvalidReferenceError struct {
	Token string
}

func (e *InvalidReferenceError) Error() string {
	return "invalid test data inline reference " + e.Token + ", expected {{$ds.<tableKey>.<col>}} or {{$ds.<tableKey>[<filterCol>=<filterValue>].<col>}}"
}

// DedupeReferences returns refs without duplicate expressions while preserving
// the original order.
func DedupeReferences(refs []InlineReference) []InlineReference {
	seen := map[string]struct{}{}
	out := make([]InlineReference, 0, len(refs))
	for _, ref := range refs {
		expression := strings.TrimSpace(ref.Expression)
		if expression == "" {
			continue
		}
		if _, ok := seen[expression]; ok {
			continue
		}
		seen[expression] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func unquoteFilterValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
		return value[1 : len(value)-1]
	}
	return value
}
