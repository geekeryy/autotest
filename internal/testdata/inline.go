package testdata

import (
	"strings"

	"autotest/internal/templating"
)

// HasInlineToken returns true when s contains any `{{$ds.*}}` occurrence. It
// is a cheap pre-check before allocating regex capture buffers.
func HasInlineToken(s string) bool {
	return templating.HasDSToken(s)
}

// ParseInlineReferences returns every `{{$ds.*}}` reference parsed from input.
// Tokens that do not match the expected shape produce a descriptive error to
// surface configuration mistakes early.
func ParseInlineReferences(input string) ([]InlineReference, error) {
	literals := templating.ScanDSLiterals(input)
	if len(literals) == 0 {
		return nil, nil
	}
	refs := make([]InlineReference, 0, len(literals))
	for _, literal := range literals {
		tok, ok := templating.ParseDSReference(literal)
		if !ok {
			return nil, &InvalidReferenceError{Token: literal}
		}
		refs = append(refs, InlineReference{
			Expression:   tok.DS.Expression,
			TableKey:     tok.DS.SourceKey,
			FilterColumn: tok.DS.FilterColumn,
			FilterValue:  tok.DS.FilterValue,
			Column:       tok.DS.Column,
		})
	}
	return refs, nil
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
