package templating

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// kinds returns the Kind sequence of tokens, useful for assertions that only
// care about the parsed shape and not the inner fields.
func kinds(tokens []Token) []Kind {
	out := make([]Kind, len(tokens))
	for i, t := range tokens {
		out[i] = t.Kind
	}
	return out
}

func TestTokenizeRoundTripsInput(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"plain",
		"prefix {{$mock.uuid}} middle {{tenant}} suffix",
		"/api/v1/users/{{$steps[2].body.data.id}}/orders/{{sql.userSeed.id}}",
		"{{$ds.users[role=admin].email}} + {{$ds.users.name}}",
		"{{{{id}}}}",
		"{$steps[1].body.id}",
		"{{request.method}} - {{request.body.user.name}}",
		"{{unknown.helper.with.dots}}",
		"{just.an.opener",
		"{{ multi spaces }}",
	}
	for _, in := range inputs {
		got := Tokenize(in)
		var b strings.Builder
		for _, tok := range got {
			b.WriteString(tok.Raw)
		}
		if b.String() != in {
			t.Errorf("tokenize round-trip mismatch for %q\n  want: %q\n  got : %q", in, in, b.String())
		}
	}
}

func TestTokenizeMockToken(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("hi {{$mock.int(1,3)}} {{$mock.uuid}} done")
	want := []Kind{KindLiteral, KindMock, KindLiteral, KindMock, KindLiteral}
	if !reflect.DeepEqual(kinds(tokens), want) {
		t.Fatalf("kinds = %v, want %v", kinds(tokens), want)
	}
	if got := tokens[1]; got.Mock.Helper != "int" || len(got.Mock.Args) != 2 || got.Mock.Args[0] != "1" || got.Mock.Args[1] != "3" {
		t.Fatalf("first mock token unexpected: %#v", got.Mock)
	}
	if got := tokens[3]; got.Mock.Helper != "uuid" || len(got.Mock.Args) != 0 {
		t.Fatalf("second mock token unexpected: %#v", got.Mock)
	}
}

func TestTokenizeMockKeepsQuotedCommaArgs(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("{{$mock.pick('a, b','c')}}")
	if len(tokens) != 1 || tokens[0].Kind != KindMock {
		t.Fatalf("expected single mock token, got %#v", tokens)
	}
	args := tokens[0].Mock.Args
	if len(args) != 2 || args[0] != "a, b" || args[1] != "c" {
		t.Fatalf("expected quoted args preserved, got %#v", args)
	}
}

func TestTokenizeStepToken(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("/api/v1/users/{{$steps[11].body.data.id}}/items/{{$steps[2].rows[0].name}}")
	steps := []Token{}
	for _, tok := range tokens {
		if tok.Kind == KindStep {
			steps = append(steps, tok)
		}
	}
	if len(steps) != 2 {
		t.Fatalf("expected two step tokens, got %d (%#v)", len(steps), tokens)
	}
	if steps[0].Step.Seq != 11 || steps[0].Step.Path != "body.data.id" {
		t.Fatalf("first step token unexpected: %#v", steps[0])
	}
	if steps[0].Body != "$steps[11].body.data.id" {
		t.Fatalf("first step body = %q", steps[0].Body)
	}
	if steps[1].Step.Seq != 2 || steps[1].Step.Path != "rows[0].name" {
		t.Fatalf("second step token unexpected: %#v", steps[1])
	}
}

func TestTokenizeStepTokenByName(t *testing.T) {
	t.Parallel()

	var nameTok *Token
	for _, tok := range Tokenize(`Bearer {{$steps["用户登录"].response.body.data.token}}`) {
		if tok.Kind == KindStep {
			nameTok = &tok
			break
		}
	}
	if nameTok == nil {
		t.Fatal("expected step token by name")
	}
	if nameTok.Step.Name != "用户登录" || nameTok.Step.Path != "response.body.data.token" {
		t.Fatalf("unexpected name token: %#v", nameTok.Step)
	}

	var slugTok *Token
	for _, tok := range Tokenize(`{{$steps.login.response.body.token}}`) {
		if tok.Kind == KindStep {
			slugTok = &tok
			break
		}
	}
	if slugTok == nil {
		t.Fatal("expected slug step token")
	}
	if slugTok.Step.Name != "login" || slugTok.Step.Path != "response.body.token" {
		t.Fatalf("unexpected slug token: %#v", slugTok.Step)
	}
}

func TestTokenizeStepTokenLenientBraces(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"{{$steps[1].body.id}}",
		"{{{$steps[1].body.id}}}",
		"{$steps[1].body.id}",
	} {
		tokens := Tokenize(in)
		if len(tokens) != 1 || tokens[0].Kind != KindStep {
			t.Fatalf("%q: expected lenient step parse, got %#v", in, tokens)
		}
		if tokens[0].Step.Seq != 1 || tokens[0].Step.Path != "body.id" {
			t.Fatalf("%q: parsed %#v", in, tokens[0])
		}
	}
}

func TestTokenizeSQLAlias(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"{{sql.userSeed.id}}",
		"{{$sql.userSeed.id}}",
	} {
		tokens := Tokenize(in)
		if len(tokens) != 1 || tokens[0].Kind != KindSQL {
			t.Fatalf("%q: expected single sql token, got %#v", in, tokens)
		}
		if tokens[0].SQL.SourceKey != "userSeed" || tokens[0].SQL.Column != "id" {
			t.Fatalf("%q: parsed %#v", in, tokens[0].SQL)
		}
		if tokens[0].SQL.Expression != "sql.userSeed.id" {
			t.Fatalf("%q: expression should normalise to legacy spelling, got %q", in, tokens[0].SQL.Expression)
		}
	}
}

func TestTokenizeSQLFilterAndQuotes(t *testing.T) {
	t.Parallel()

	tokens := Tokenize(`{{sql.userSeed[role="ad min"].email}}`)
	if len(tokens) != 1 || tokens[0].Kind != KindSQL {
		t.Fatalf("expected sql token, got %#v", tokens)
	}
	got := tokens[0].SQL
	if got.FilterColumn != "role" || got.FilterValue != "ad min" || got.Column != "email" {
		t.Fatalf("unexpected sql filter parse: %#v", got)
	}
}

func TestTokenizeDSToken(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("{{$ds.users[role=admin].email}}")
	if len(tokens) != 1 || tokens[0].Kind != KindDS {
		t.Fatalf("expected ds token, got %#v", tokens)
	}
	got := tokens[0].DS
	if got.SourceKey != "users" || got.FilterColumn != "role" || got.FilterValue != "admin" || got.Column != "email" {
		t.Fatalf("unexpected ds parse: %#v", got)
	}
	if got.Expression != "$ds.users[role=admin].email" {
		t.Fatalf("expression preserved with $ds prefix, got %q", got.Expression)
	}
}

func TestTokenizeRequestAlias(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		input  string
		key    string
		legacy bool
	}{
		{"{{request.method}}", "method", true},
		{"{{request.body.user.name}}", "body.user.name", true},
		{"{{$req.method}}", "method", false},
		{"{{$req.body.user.name}}", "body.user.name", false},
	} {
		tokens := Tokenize(c.input)
		if len(tokens) != 1 || tokens[0].Kind != KindRequest {
			t.Fatalf("%q: expected request token, got %#v", c.input, tokens)
		}
		if tokens[0].Request.Key != c.key {
			t.Fatalf("%q: key = %q, want %q", c.input, tokens[0].Request.Key, c.key)
		}
		if tokens[0].LegacyForm != c.legacy {
			t.Fatalf("%q: LegacyForm = %v, want %v", c.input, tokens[0].LegacyForm, c.legacy)
		}
	}
}

func TestTokenizeVariableHandlesOverWrapping(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"{{id}}", "{{{{id}}}}"} {
		tokens := Tokenize(in)
		if len(tokens) != 1 || tokens[0].Kind != KindVar {
			t.Fatalf("%q: expected single var token, got %#v", in, tokens)
		}
		if tokens[0].Var.Name != "id" {
			t.Fatalf("%q: var name = %q", in, tokens[0].Var.Name)
		}
	}
}

func TestTokenizeUnknownPreservesRaw(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("noise {{ unknown helper with spaces }} more")
	if len(tokens) != 3 {
		t.Fatalf("expected three tokens, got %#v", tokens)
	}
	mid := tokens[1]
	if mid.Kind != KindVar {
		t.Fatalf("expected var classification for free-form body, got %s (%#v)", mid.Kind, mid)
	}
	if mid.Raw != "{{ unknown helper with spaces }}" {
		t.Fatalf("raw preserved, got %q", mid.Raw)
	}
}

func TestTokenizeKeepsOpenAPIBraceLiteral(t *testing.T) {
	t.Parallel()

	tokens := Tokenize("/api/v1/users/{id}/orders")
	for _, tok := range tokens {
		if tok.Kind != KindLiteral {
			t.Fatalf("expected only literal tokens, got %#v", tok)
		}
	}
}

func TestTokenizeMixedRequest(t *testing.T) {
	t.Parallel()

	tokens := Tokenize(`{"id":"{{request.pathvar.id}}","trace":"{{$mock.uuid}}","note":"{{request.unknown.field}}"}`)
	var kinds []Kind
	for _, tok := range tokens {
		if tok.Kind != KindLiteral {
			kinds = append(kinds, tok.Kind)
		}
	}
	want := []Kind{KindRequest, KindMock, KindRequest}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
}

func TestRenderRendersMockFirstThenVariables(t *testing.T) {
	t.Parallel()

	got := Render("user-{{$mock.uuid}}-{{tenant}}", Resolver{
		Mock: ExpandMock,
		Var:  VarsResolver(map[string]string{"tenant": "acme"}),
	})
	if !strings.HasSuffix(got, "-acme") {
		t.Fatalf("expected suffix substitution, got %q", got)
	}
	if strings.Contains(got, "$mock") || strings.Contains(got, "{{") {
		t.Fatalf("expected mock token to expand, got %q", got)
	}
}

func TestRenderWithoutMockHookLeavesTokensAlone(t *testing.T) {
	t.Parallel()

	got := Render("user-{{$mock.uuid}}-{{tenant}}", Resolver{
		Var: VarsResolver(map[string]string{"tenant": "acme"}),
	})
	if got != "user-{{$mock.uuid}}-acme" {
		t.Fatalf("expected mock token to be preserved without hook, got %q", got)
	}
}

func TestRenderFallsBackToVarsForStepRefs(t *testing.T) {
	t.Parallel()

	got := Render("/api/v1/users/{{$steps[11].body.data.id}}", Resolver{
		Var: VarsResolver(map[string]string{"$steps[11].body.data.id": "site-42"}),
	})
	if got != "/api/v1/users/site-42" {
		t.Fatalf("expected step fallback via vars, got %q", got)
	}
}

func TestRenderRunsMultipleIterationsForNestedVars(t *testing.T) {
	t.Parallel()

	got := Render("/api/v1/users/{{id}}", Resolver{
		Var: VarsResolver(map[string]string{
			"id":              "{{sql.userSeed.id}}",
			"sql.userSeed.id": "99",
		}),
	})
	if got != "/api/v1/users/99" {
		t.Fatalf("expected nested expansion, got %q", got)
	}
}

func TestRenderRequestHookResolves(t *testing.T) {
	t.Parallel()

	got := Render("/echo/{{request.pathvar.id}}/{{$req.method}}", Resolver{
		Request: func(t Token) (string, bool) {
			switch t.Request.Key {
			case "pathvar.id":
				return "42", true
			case "method":
				return "GET", true
			}
			return "", false
		},
	})
	if got != "/echo/42/GET" {
		t.Fatalf("unexpected render: %q", got)
	}
}

// mockSetJSONStrings 将若干 Go 字符串值编码为 JSON 数组元素（每项为 JSON 字符串）。
func mockSetJSONStrings(ss ...string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(ss))
	for _, s := range ss {
		b, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		out = append(out, b)
	}
	return out
}

func TestTokenizeMockSetSyntax(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		key      string
		mode     string
		index    int
		bodyHint string
	}{
		{"{{$mock.set.colors}}", "colors", MockSetModeRandom, 0, "$mock.set.colors"},
		{"{{$mock.set.colors[2]}}", "colors", MockSetModeIndex, 2, "$mock.set.colors[2]"},
		{"{{$mock.set.colors[*]}}", "colors", MockSetModeSequential, 0, "$mock.set.colors[*]"},
		{"{{ $mock.set.users-prod[10] }}", "users-prod", MockSetModeIndex, 10, "$mock.set.users-prod[10]"},
	}
	for _, tc := range cases {
		tokens := Tokenize(tc.input)
		if len(tokens) != 1 || tokens[0].Kind != KindMock {
			t.Fatalf("%q: expected single mock token, got %#v", tc.input, tokens)
		}
		got := tokens[0]
		if got.Mock.Helper != "set" {
			t.Fatalf("%q: helper = %q, want set", tc.input, got.Mock.Helper)
		}
		if got.Mock.Set.Key != tc.key {
			t.Fatalf("%q: key = %q, want %q", tc.input, got.Mock.Set.Key, tc.key)
		}
		if got.Mock.Set.Mode != tc.mode {
			t.Fatalf("%q: mode = %q, want %q", tc.input, got.Mock.Set.Mode, tc.mode)
		}
		if tc.mode == MockSetModeIndex && got.Mock.Set.Index != tc.index {
			t.Fatalf("%q: index = %d, want %d", tc.input, got.Mock.Set.Index, tc.index)
		}
		if got.Body != tc.bodyHint {
			t.Fatalf("%q: body = %q, want %q", tc.input, got.Body, tc.bodyHint)
		}
	}
}

func TestRenderMockSetRandom(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		if key != "colors" {
			return nil, nil, false
		}
		return mockSetJSONStrings("red", "green", "blue"), nil, true
	})
	hook := NewMockExpander(MockExpanderConfig{Sets: resolver})
	got := Render("color={{$mock.set.colors}}", Resolver{Mock: hook})
	if got == "color={{$mock.set.colors}}" {
		t.Fatalf("expected token to expand, got %q", got)
	}
	prefix := "color="
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("expected prefix %q, got %q", prefix, got)
	}
	value := strings.TrimPrefix(got, prefix)
	switch value {
	case "red", "green", "blue":
	default:
		t.Fatalf("unexpected random pick %q", value)
	}
}

func TestRenderMockSetIndex(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return mockSetJSONStrings("red", "green", "blue"), nil, true
	})
	hook := NewMockExpander(MockExpanderConfig{Sets: resolver})
	if got := Render("{{$mock.set.colors[1]}}", Resolver{Mock: hook}); got != "green" {
		t.Fatalf("expected green, got %q", got)
	}
}

func TestRenderMockSetIndexOutOfRangePreservesToken(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return mockSetJSONStrings("red"), nil, true
	})
	var captured []string
	hook := NewMockExpander(MockExpanderConfig{
		Sets: resolver,
		OnError: func(err error) {
			captured = append(captured, err.Error())
		},
	})
	got := Render("{{$mock.set.colors[5]}}", Resolver{Mock: hook})
	if got != "{{$mock.set.colors[5]}}" {
		t.Fatalf("expected token preserved, got %q", got)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "越界") {
		t.Fatalf("expected 越界 error, got %#v", captured)
	}
}

func TestRenderMockSetMissingResolverReportsError(t *testing.T) {
	t.Parallel()

	var captured []string
	hook := NewMockExpander(MockExpanderConfig{OnError: func(err error) {
		captured = append(captured, err.Error())
	}})
	got := Render("{{$mock.set.colors}}", Resolver{Mock: hook})
	if got != "{{$mock.set.colors}}" {
		t.Fatalf("expected token preserved, got %q", got)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "未配置") {
		t.Fatalf("expected 未配置 error, got %#v", captured)
	}
}

func TestRenderMockSetMissingSet(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return nil, nil, false
	})
	var captured []string
	hook := NewMockExpander(MockExpanderConfig{
		Sets:    resolver,
		OnError: func(err error) { captured = append(captured, err.Error()) },
	})
	got := Render("{{$mock.set.colors}}", Resolver{Mock: hook})
	if got != "{{$mock.set.colors}}" {
		t.Fatalf("expected token preserved, got %q", got)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "未配置或值为空") {
		t.Fatalf("expected missing-set error, got %#v", captured)
	}
}

func TestRenderMockSetSequentialAdvancesCursor(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return mockSetJSONStrings("a", "b", "c"), nil, true
	})
	cursors := map[string]*int{}
	hook := NewMockExpander(MockExpanderConfig{Sets: resolver, Cursors: cursors})
	r := Resolver{Mock: hook}
	got := Render("{{$mock.set.x[*]}}|{{$mock.set.x[*]}}|{{$mock.set.x[*]}}|{{$mock.set.x[*]}}", r)
	if got != "a|b|c|a" {
		t.Fatalf("expected wrap-around order, got %q", got)
	}
	// 不同 set 的游标互相独立。
	got = Render("{{$mock.set.y[*]}}|{{$mock.set.y[*]}}", Resolver{Mock: NewMockExpander(MockExpanderConfig{Sets: resolver, Cursors: cursors})})
	if got != "a|b" {
		t.Fatalf("expected y to start from 0, got %q", got)
	}
}

func TestRenderMockSetWeightsBiasDistribution(t *testing.T) {
	t.Parallel()

	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return mockSetJSONStrings("red", "green", "blue"), []float64{99, 0, 0}, true
	})
	hook := NewMockExpander(MockExpanderConfig{Sets: resolver})
	for i := 0; i < 30; i++ {
		got := Render("{{$mock.set.colors}}", Resolver{Mock: hook})
		if got != "red" {
			t.Fatalf("expected red bias, got %q on iteration %d", got, i)
		}
	}
}

func TestRenderMockSetIndexJSONCompact(t *testing.T) {
	t.Parallel()

	obj := map[string]any{"a": 1, "b": "x"}
	rawObj, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	resolver := MockSetResolverFunc(func(key string) ([]json.RawMessage, []float64, bool) {
		return []json.RawMessage{
			rawObj,
			json.RawMessage(`null`),
		}, nil, true
	})
	hook := NewMockExpander(MockExpanderConfig{Sets: resolver})
	if got := Render("{{$mock.set.t[0]}}", Resolver{Mock: hook}); got != `{"a":1,"b":"x"}` {
		t.Fatalf("object: got %q", got)
	}
	if got := Render("{{$mock.set.t[1]}}", Resolver{Mock: hook}); got != "null" {
		t.Fatalf("null: got %q", got)
	}
}

func TestExpandMockTokensIndependentValues(t *testing.T) {
	t.Parallel()

	got := ExpandMockTokens("{{$mock.uuid}}|{{$mock.uuid}}")
	parts := strings.Split(got, "|")
	if len(parts) != 2 {
		t.Fatalf("expected two segments, got %q", got)
	}
	if parts[0] == parts[1] {
		t.Fatalf("expected independent uuids, got %q", got)
	}
}
