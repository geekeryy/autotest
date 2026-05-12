// Package templating provides the single source of truth for tokenising and
// rendering the platform's Mustache-style template syntax. Several call sites
// (HTTP runner, mock server, SQL parameter source executor, test data
// service, etc.) previously each maintained their own ad-hoc regexes for the
// same handful of placeholder shapes; this package consolidates them so that
// a request body, a Mock response template and an inline SQL reference are
// all parsed against the same grammar.
//
// Supported placeholder kinds (recognised by Tokenize):
//
//   - `{{$mock.<helper>(args?)}}`            runtime data mock token
//   - `{{$steps[N].<json-path>}}`            scenario step reference
//   - `{{sql.<source>[<f=v>].<col>}}`        SQL inline reference (legacy)
//   - `{{$ds.<table>[<f=v>].<col>}}`         test data inline reference
//   - `{{request.<key>}}`                    Mock Server request reference
//   - `{{varName}}` / `{{{{varName}}}}`      user-defined environment / run
//     variable (over-wrapped multi-brace form preserved for backward
//     compatibility with existing user data)
//   - `{{$sql.<source>[...].<col>}}`         (PR 2) alias for `{{sql.*}}`
//   - `{{$req.<key>}}`                       (PR 2) alias for `{{request.*}}`
//
// OpenAPI-style single-brace path parameters (`{name}`) live in their own
// pre-render pass inside the runner and intentionally do NOT enter this
// parser.
package templating

// Kind enumerates the placeholder shapes the tokeniser recognises.
type Kind int

const (
	// KindLiteral is a stretch of plain text between placeholders.
	KindLiteral Kind = iota
	// KindUnknown is a `{{...}}` shape whose body did not match any known
	// kind; the renderer preserves the original literal so unknown helpers
	// are not silently dropped.
	KindUnknown
	// KindMock matches `{{$mock.<helper>(args?)}}`.
	KindMock
	// KindStep matches `{{$steps[N].<path>}}` (and lenient single-brace
	// historical variants).
	KindStep
	// KindSQL matches `{{sql.<source>[<f=v>].<col>}}` and, from PR 2 on, the
	// `{{$sql.*}}` alias.
	KindSQL
	// KindDS matches `{{$ds.<table>[<f=v>].<col>}}`.
	KindDS
	// KindRequest matches `{{request.<key>}}` and, from PR 2 on, the
	// `{{$req.*}}` alias.
	KindRequest
	// KindVar matches `{{varName}}` and the over-wrapped multi-brace form
	// `{{{{varName}}}}`.
	KindVar
)

// String returns a short label suitable for tests and debug logs.
func (k Kind) String() string {
	switch k {
	case KindLiteral:
		return "literal"
	case KindUnknown:
		return "unknown"
	case KindMock:
		return "mock"
	case KindStep:
		return "step"
	case KindSQL:
		return "sql"
	case KindDS:
		return "ds"
	case KindRequest:
		return "request"
	case KindVar:
		return "var"
	}
	return "?"
}

// Token is one segment produced by Tokenize. The concatenation of Token.Raw
// values for every token returned by Tokenize equals the original input, so
// callers can reproduce the input losslessly by walking the slice.
type Token struct {
	Kind Kind
	Raw  string

	// Namespace is the canonical prefix used by the platform regardless of
	// the alias the user wrote (always begins with `$` for non-Var kinds):
	//   KindMock     → "$mock"
	//   KindStep     → "$steps"
	//   KindSQL      → "$sql"
	//   KindDS       → "$ds"
	//   KindRequest  → "$req"
	//   KindVar      → ""
	//   KindUnknown  → ""
	Namespace string

	// LegacyForm is true when the user wrote a deprecated alias that this
	// parser normalised to the canonical namespace (currently only `sql.*`
	// and `request.*`). Callers wanting to preserve the legacy spelling in
	// inline-reference snapshots can read this flag.
	LegacyForm bool

	// Body is the canonical placeholder content WITHOUT the surrounding
	// braces, normalised so the legacy and `$`-prefixed forms render
	// identically. For Mock/Step tokens it always begins with the canonical
	// namespace (e.g. `$mock.uuid`, `$steps[1].body.token`). For SQL the
	// canonical body uses the legacy `sql.` prefix so already-migrated user
	// data (e.g. existing `vars` maps keyed on `sql.userSeed.id`) keeps
	// working unmodified.
	Body string

	// Kind-specific parsed fields:
	Mock    MockToken
	Step    StepToken
	SQL     SQLToken
	DS      SQLToken
	Request RequestToken
	Var     VarToken
}

// MockToken carries the parsed helper invocation for a KindMock token.
//
// `Set` is only populated when the token uses the `$mock.set.<key>` family
// (see MockSetToken). For these tokens Helper is normalised to "set" so
// downstream switch-case logic can branch cheaply.
type MockToken struct {
	Helper string
	Args   []string
	Set    MockSetToken
}

// MockSetToken describes a `{{$mock.set.<key>}}` family token.
//
// Three modes are recognised, matching the user-facing template syntax:
//   - MockSetModeRandom     (`$mock.set.<key>`)        → weighted random pick
//   - MockSetModeIndex      (`$mock.set.<key>[N]`)     → 0-based fixed index
//   - MockSetModeSequential (`$mock.set.<key>[*]`)     → run-scoped cursor
//
// Index is only valid when Mode == MockSetModeIndex.
type MockSetToken struct {
	Key   string
	Mode  string
	Index int
}

// Mock-set mode identifiers exported for callers (runner / mockserver) that
// want to dispatch on the mode from outside the package.
const (
	MockSetModeRandom     = "random"
	MockSetModeIndex      = "index"
	MockSetModeSequential = "sequential"
)

// StepToken carries the parsed `$steps[N].path` reference. Path is the
// dot-separated sub-path AFTER the bracketed sequence (without a leading
// dot), e.g. `body.data.token` or `rows[0].name`.
type StepToken struct {
	Seq  int
	Path string
}

// SQLToken describes an inline SQL or test-data reference. The same struct is
// used for both kinds because they share the `<source>[<f=v>].<col>` shape.
// Expression is the placeholder body without braces (e.g. `sql.userSeed.id`
// or `$ds.users[role=admin].email`) and is used as the variable key in the
// runner so existing data does not need re-keying.
type SQLToken struct {
	Expression   string
	SourceKey    string
	FilterColumn string
	FilterValue  string
	Column       string
}

// RequestToken carries the parsed Mock-Server request reference. Key is the
// part after the canonical `request.` prefix, e.g. `method`, `bodyRaw`,
// `pathvar.id`, `query.tenant` or `body.user.name`. An empty Key represents
// the bare `{{request}}` form (currently unused but reserved).
type RequestToken struct {
	Key string
}

// VarToken carries the parsed plain `{{name}}` variable reference.
type VarToken struct {
	Name string
}
