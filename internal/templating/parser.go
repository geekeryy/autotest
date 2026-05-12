package templating

import (
	"regexp"
	"strconv"
	"strings"

	"autotest/internal/mockdata"
)

// Tokenize splits input into a sequence of literal and placeholder tokens.
//
// Guarantees:
//   - Concatenating every Token.Raw in order reproduces the original input
//     exactly, so callers may use Tokenize as a non-destructive scanner.
//   - Each non-literal placeholder is fully parsed up-front; downstream code
//     does not need to re-match regexes to read e.g. the helper name of a
//     `{{$mock.uuid}}` token.
//
// Brace-count tolerance is preserved per kind to keep historical user data
// rendering identically:
//   - $mock, $ds, $sql, request require exactly two braces on each side
//     (the canonical `{{...}}` shape).
//   - $steps tolerates a single-brace shape (`{$steps[1].body.id}`) because
//     the historical scanRE was lenient.
//   - Plain variables tolerate any over-wrapped form (`{{{{id}}}}`) because
//     the historical replacement regex used `\{\{+key\}\}+`.
func Tokenize(input string) []Token {
	if input == "" {
		return nil
	}
	var out []Token
	i := 0
	n := len(input)
	last := 0
	for i < n {
		if input[i] != '{' {
			i++
			continue
		}

		runStart := i
		for i < n && input[i] == '{' {
			i++
		}
		openCount := i - runStart

		bodyStart := i
		for i < n && input[i] != '{' && input[i] != '}' {
			i++
		}
		if i >= n || input[i] != '}' {
			continue
		}
		bodyEnd := i
		closeStart := i
		for i < n && input[i] == '}' {
			i++
		}
		closeCount := i - closeStart

		raw := input[runStart:i]
		tok, ok := classify(raw, input[bodyStart:bodyEnd], openCount, closeCount)
		if !ok {
			continue
		}

		if runStart > last {
			out = append(out, Token{Kind: KindLiteral, Raw: input[last:runStart]})
		}
		out = append(out, tok)
		last = i
	}
	if last < n {
		out = append(out, Token{Kind: KindLiteral, Raw: input[last:n]})
	}
	return out
}

// HasMockToken reports whether s contains at least one `{{$mock.*}}` token.
// It mirrors `mockdata.HasToken` and is exposed so call sites can keep their
// existing fast-path checks without re-tokenising the input.
func HasMockToken(s string) bool {
	return mockdata.HasToken(s)
}

// HasStepToken reports whether s contains at least one `{{$steps[N]...}}`
// reference. Backwards-compatible with the legacy single-brace shape.
func HasStepToken(s string) bool {
	return strings.Contains(s, "$steps[")
}

// HasSQLToken reports whether s contains at least one `{{sql.*}}` or
// `{{$sql.*}}` reference.
func HasSQLToken(s string) bool {
	return strings.Contains(s, "sql.") && sqlBodyHintRE.MatchString(s)
}

// HasDSToken reports whether s contains at least one `{{$ds.*}}` reference.
func HasDSToken(s string) bool {
	return strings.Contains(s, "$ds.") && dsBodyHintRE.MatchString(s)
}

// HasRequestToken reports whether s contains at least one `{{request.*}}` or
// `{{$req.*}}` reference (Mock Server only).
func HasRequestToken(s string) bool {
	return requestBodyHintRE.MatchString(s)
}

// Iterate is a convenience wrapper that calls fn for every non-literal
// token. fn may stop iteration early by returning false.
func Iterate(input string, fn func(Token) bool) {
	for _, tok := range Tokenize(input) {
		if tok.Kind == KindLiteral {
			continue
		}
		if !fn(tok) {
			return
		}
	}
}

// ── Classification helpers ─────────────────────────────────────────────

var (
	// `\s*` tolerance around the dot matches the original mockdata regex.
	mockBodyRE = regexp.MustCompile(`^\s*\$mock\s*\.\s*([A-Za-z][A-Za-z0-9_]*)\s*(?:\(([^()]*)\))?\s*$`)

	// `$mock.set.<key>` with optional `[N]` (numeric index) or `[*]`
	// (sequential cursor). `<key>` matches the same character class enforced
	// by mockset.Service.validateKey so the parser refuses references that
	// could not match a real value-set record.
	mockSetBodyRE = regexp.MustCompile(`^\s*\$mock\s*\.\s*set\s*\.\s*([A-Za-z0-9_-]+)\s*(?:\[\s*(\d+|\*)\s*\])?\s*$`)

	// $steps[N]<rest> — `rest` may start with `.field` or `[idx]` or be empty.
	stepBodyRE = regexp.MustCompile(`^\s*\$steps\[(\d+)\]([^{}]*?)\s*$`)

	// sql.<source>[<f=v>].<col>
	// Both the legacy `sql.` form and the `$sql.` alias are accepted; the
	// captured Expression always uses the canonical legacy spelling so
	// existing variable maps (keyed on `sql.userSeed.id`) keep matching.
	sqlBodyRE = regexp.MustCompile(`^\s*\$?(sql\.([A-Za-z0-9_-]+)(?:\[([A-Za-z0-9_]+)=([^\]\r\n]+)\])?\.([A-Za-z0-9_]+))\s*$`)

	// $ds.<table>[<f=v>].<col>
	dsBodyRE = regexp.MustCompile(`^\s*(\$ds\.([A-Za-z0-9_-]+)(?:\[([A-Za-z0-9_]+)=([^\]\r\n]+)\])?\.([A-Za-z0-9_]+))\s*$`)

	// request.<key> or $req.<key>; the bare `request` form (no dot) is also
	// accepted because Mock helpers like `request.body` / `request.bodyRaw`
	// require sub-paths but `request` alone is reserved for future use.
	requestBodyRE = regexp.MustCompile(`^\s*(?:request|\$req)(?:\.([^{}]+?))?\s*$`)

	// Cheap upstream hints used by HasXxxToken helpers.
	sqlBodyHintRE     = regexp.MustCompile(`\{\{\s*\$?sql\.[^{}]+\}\}`)
	dsBodyHintRE      = regexp.MustCompile(`\{\{\s*\$ds[^{}]*\}\}`)
	requestBodyHintRE = regexp.MustCompile(`\{\{\s*(?:request|\$req)(?:\.[^{}]+)?\s*\}\}`)
)

// ScanSQLLiterals returns every raw `{{sql.*}}` / `{{$sql.*}}` substring in
// input (lenient match). Callers typically pair this with ParseSQLReference
// to surface descriptive errors for malformed references that would
// otherwise be silently dropped.
func ScanSQLLiterals(input string) []string {
	if !strings.Contains(input, "sql.") {
		return nil
	}
	return sqlBodyHintRE.FindAllString(input, -1)
}

// ScanDSLiterals is the test-data counterpart to ScanSQLLiterals. It returns
// every raw `{{$ds...}}` substring; callers validate each via
// ParseDSReference.
func ScanDSLiterals(input string) []string {
	if !strings.Contains(input, "$ds") {
		return nil
	}
	return dsBodyHintRE.FindAllString(input, -1)
}

// ParseSQLReference parses a single strict `{{sql.<source>[<f=v>].<col>}}` /
// `{{$sql.<source>[<f=v>].<col>}}` literal. Returns ok=false when literal
// does not match the canonical shape; the caller surfaces a kind-specific
// "invalid sql inline reference" error.
func ParseSQLReference(literal string) (Token, bool) {
	tokens := Tokenize(literal)
	if len(tokens) != 1 || tokens[0].Kind != KindSQL {
		return Token{}, false
	}
	return tokens[0], true
}

// ParseDSReference parses a single strict `{{$ds.<table>[<f=v>].<col>}}` literal.
func ParseDSReference(literal string) (Token, bool) {
	tokens := Tokenize(literal)
	if len(tokens) != 1 || tokens[0].Kind != KindDS {
		return Token{}, false
	}
	return tokens[0], true
}

func classify(raw, body string, openCount, closeCount int) (Token, bool) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return Token{}, false
	}

	if openCount == 2 && closeCount == 2 {
		// Set tokens take precedence over the generic mock-helper match
		// because `$mock.set.colors` would otherwise be classified as the
		// non-existent helper "set" with no args.
		if m := mockSetBodyRE.FindStringSubmatch(body); m != nil {
			tok := MockSetToken{
				Key:  m[1],
				Mode: MockSetModeRandom,
			}
			switch indicator := m[2]; {
			case indicator == "":
				// No bracket → random.
			case indicator == "*":
				tok.Mode = MockSetModeSequential
			default:
				idx, err := strconv.Atoi(indicator)
				if err != nil {
					return Token{}, false
				}
				tok.Mode = MockSetModeIndex
				tok.Index = idx
			}
			body := "$mock.set." + tok.Key
			switch tok.Mode {
			case MockSetModeIndex:
				body += "[" + strconv.Itoa(tok.Index) + "]"
			case MockSetModeSequential:
				body += "[*]"
			}
			return Token{
				Kind:      KindMock,
				Raw:       raw,
				Namespace: "$mock",
				Body:      body,
				Mock: MockToken{
					Helper: "set",
					Set:    tok,
				},
			}, true
		}
		if m := mockBodyRE.FindStringSubmatch(body); m != nil {
			return Token{
				Kind:      KindMock,
				Raw:       raw,
				Namespace: "$mock",
				Body:      "$mock." + m[1],
				Mock: MockToken{
					Helper: m[1],
					Args:   mockdata.ParseArgsForTest(m[2]),
				},
			}, true
		}
	}

	if openCount >= 1 && closeCount >= 1 {
		if m := stepBodyRE.FindStringSubmatch(body); m != nil {
			seq, err := strconv.Atoi(m[1])
			if err != nil {
				return Token{}, false
			}
			path := strings.TrimPrefix(strings.TrimSpace(m[2]), ".")
			expression := "$steps[" + m[1] + "]" + strings.TrimSpace(m[2])
			return Token{
				Kind:      KindStep,
				Raw:       raw,
				Namespace: "$steps",
				Body:      expression,
				Step: StepToken{
					Seq:  seq,
					Path: path,
				},
			}, true
		}
	}

	if openCount == 2 && closeCount == 2 {
		if m := sqlBodyRE.FindStringSubmatch(body); m != nil {
			legacy := !strings.Contains(strings.TrimSpace(body), "$sql.")
			return Token{
				Kind:       KindSQL,
				Raw:        raw,
				Namespace:  "$sql",
				LegacyForm: legacy,
				Body:       m[1],
				SQL: SQLToken{
					Expression:   m[1],
					SourceKey:    m[2],
					FilterColumn: m[3],
					FilterValue:  unquoteFilterValue(m[4]),
					Column:       m[5],
				},
			}, true
		}
	}

	if openCount == 2 && closeCount == 2 {
		if m := dsBodyRE.FindStringSubmatch(body); m != nil {
			return Token{
				Kind:      KindDS,
				Raw:       raw,
				Namespace: "$ds",
				Body:      m[1],
				DS: SQLToken{
					Expression:   m[1],
					SourceKey:    m[2],
					FilterColumn: m[3],
					FilterValue:  unquoteFilterValue(m[4]),
					Column:       m[5],
				},
			}, true
		}
	}

	if openCount == 2 && closeCount == 2 {
		if m := requestBodyRE.FindStringSubmatch(body); m != nil {
			legacy := strings.HasPrefix(strings.TrimSpace(body), "request")
			key := strings.TrimSpace(m[1])
			body := "request"
			if key != "" {
				body = "request." + key
			}
			return Token{
				Kind:       KindRequest,
				Raw:        raw,
				Namespace:  "$req",
				LegacyForm: legacy,
				Body:       body,
				Request:    RequestToken{Key: key},
			}, true
		}
	}

	if openCount >= 2 && closeCount >= 2 {
		// Plain variable: any non-empty body without `{` / `}` qualifies.
		// We deliberately accept exotic names (e.g. injected step-ref keys
		// like `$steps[11].body.data.id`) so callers can keep using the
		// existing pre-injection pattern.
		return Token{
			Kind:      KindVar,
			Raw:       raw,
			Namespace: "",
			Body:      trimmed,
			Var:       VarToken{Name: trimmed},
		}, true
	}

	// Single-brace shapes that don't match KindStep are left to the caller
	// (e.g. OpenAPI `{name}` path variables) — we don't emit a token and the
	// scanner treats the run as plain literal characters.
	return Token{}, false
}

func unquoteFilterValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	first, last := value[0], value[len(value)-1]
	if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
		return value[1 : len(value)-1]
	}
	return value
}
