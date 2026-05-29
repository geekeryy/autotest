package templating

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"autotest/internal/mockdata"
)

// MockSetResolver looks up the value pool for a project-level mock value set.
// Implementations are typically a thin adapter over `mockset.Service.Lookup`.
//
// Returning ok=false signals that the set does not exist; the renderer hook
// will surface a descriptive error via the OnError callback (see
// NewMockExpander) so the runner can fail the request rather than silently
// producing an empty string.
type MockSetResolver interface {
	Lookup(key string) (values []json.RawMessage, weights []float64, ok bool)
}

// MockSetResolverFunc adapts a plain function into a MockSetResolver.
type MockSetResolverFunc func(key string) (values []json.RawMessage, weights []float64, ok bool)

// Lookup implements MockSetResolver.
func (f MockSetResolverFunc) Lookup(key string) ([]json.RawMessage, []float64, bool) {
	return f(key)
}

// MockExpanderConfig wires the optional set resolver and per-call cursor map
// into the standard Mock hook expected by Resolver.Mock.
type MockExpanderConfig struct {
	// Sets is consulted for `{{$mock.set.<key>}}` tokens. nil disables set
	// resolution (tokens are preserved verbatim and reported via OnError).
	Sets MockSetResolver
	// Cursors holds per-key sequential indices for `[*]` mode. nil disables
	// sequential mode (it falls back to random; OnError sees a warning).
	Cursors map[string]*int
	// OnError is invoked for any non-fatal expansion problem (missing set,
	// out-of-range index, unconfigured cursors, etc.). The hook continues to
	// preserve the offending token verbatim so the caller may still log a
	// warning while letting other tokens render.
	OnError func(error)
}

// NewMockExpander returns a Mock hook for templating.Render that handles the
// standard mockdata helpers and `$mock.set.<key>` lookups consistently.
//
// When cfg.Sets is nil (or the resolver returns ok=false) for a set token,
// the renderer leaves the token verbatim and signals OnError so the caller
// can fail the run rather than silently producing an empty string.
func NewMockExpander(cfg MockExpanderConfig) func(Token) (string, bool) {
	return func(tok Token) (string, bool) {
		if tok.Kind != KindMock {
			return "", false
		}
		if tok.Mock.Helper != "set" {
			return ExpandMock(tok)
		}
		set := tok.Mock.Set
		if cfg.Sets == nil {
			cfg.report(fmt.Errorf("运行时 mock 集合 %q 未配置：当前调用方未提供 set 解析器", set.Key))
			return "", false
		}
		values, weights, ok := cfg.Sets.Lookup(set.Key)
		if !ok || len(values) == 0 {
			cfg.report(fmt.Errorf("运行时 mock 集合 %q 未配置或值为空", set.Key))
			return "", false
		}
		var raw json.RawMessage
		switch set.Mode {
		case MockSetModeIndex:
			if set.Index < 0 || set.Index >= len(values) {
				cfg.report(fmt.Errorf("运行时 mock 集合 %q 索引 [%d] 越界（共 %d 项）", set.Key, set.Index, len(values)))
				return "", false
			}
			raw = values[set.Index]
		case MockSetModeSequential:
			if cfg.Cursors == nil {
				cfg.report(fmt.Errorf("运行时 mock 集合 %q 顺序模式 [*] 缺少 run 上下文，已退化为随机抽样", set.Key))
				raw = pickWeightedRaw(values, weights)
			} else {
				cur, ok := cfg.Cursors[set.Key]
				if !ok {
					idx := 0
					cfg.Cursors[set.Key] = &idx
					cur = &idx
				}
				raw = values[*cur%len(values)]
				*cur++
			}
		default:
			raw = pickWeightedRaw(values, weights)
		}
		out, err := FormatMockSetPlaceholderValue(raw)
		if err != nil {
			cfg.report(fmt.Errorf("运行时 mock 集合 %q 取值编码失败: %v", set.Key, err))
			return "", false
		}
		return out, true
	}
}

func (cfg MockExpanderConfig) report(err error) {
	if cfg.OnError != nil && err != nil {
		cfg.OnError(err)
	}
}

// FormatMockSetPlaceholderValue 将命名值集合中的一项 JSON 转为模板替换文本：
// JSON 字符串取内部文本；null/bool/number 为 JSON 字面量；object/array 为紧凑 JSON（无多余空格）。
func FormatMockSetPlaceholderValue(raw json.RawMessage) (string, error) {
	b := bytes.TrimSpace(raw)
	if len(b) == 0 {
		return "", fmt.Errorf("空值")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return "", fmt.Errorf("多余 JSON 内容")
	}
	return formatDecodedMockSetJSON(v)
}

func formatDecodedMockSetJSON(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case nil:
		return "null", nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case json.Number:
		return x.String(), nil
	case float64:
		b, err := json.Marshal(x)
		return string(b), err
	default:
		b, err := json.Marshal(v)
		return string(b), err
	}
}

func pickWeightedIndex(n int, weights []float64) int {
	if n <= 0 {
		return 0
	}
	if len(weights) != n {
		return rand.Intn(n)
	}
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return rand.Intn(n)
	}
	target := rand.Float64() * total
	acc := 0.0
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		acc += w
		if target < acc {
			return i
		}
	}
	return n - 1
}

func pickWeightedRaw(values []json.RawMessage, weights []float64) json.RawMessage {
	if len(values) == 0 {
		return nil
	}
	return values[pickWeightedIndex(len(values), weights)]
}

// Resolver carries the per-kind callbacks Render uses to look up a value for
// a placeholder. Every hook is optional: a nil hook means "this caller does
// not resolve tokens of that kind, leave them untouched". Var is consulted as
// a fallback for KindStep / KindSQL / KindDS tokens whose primary hook
// returns ok=false; this preserves the historical "pre-inject the ref as a
// variable key" pattern used by `runner.injectStepRefs`,
// `runner.applyInlineSQLReferences` and `runner.applyInlineTestDataReferences`.
type Resolver struct {
	Mock    func(Token) (string, bool)
	Step    func(Token) (string, bool)
	SQL     func(Token) (string, bool)
	DS      func(Token) (string, bool)
	Request func(Token) (string, bool)
	Var     func(Token) (string, bool)
}

// MaxIterations is the upper bound on how many times Render re-tokenises the
// rendered string. It matches the historical 5-iteration loop in
// `runner.renderVariables` (needed when a variable's value itself contains
// another placeholder).
const MaxIterations = 5

// Render walks the input through one or more passes of placeholder
// substitution. It runs up to MaxIterations passes and stops early once a
// pass produces no further changes. Hooks may return ok=false to indicate
// that the token should be preserved verbatim.
func Render(input string, r Resolver) string {
	if input == "" || !strings.ContainsAny(input, "{}") {
		return input
	}
	rendered := input
	for i := 0; i < MaxIterations; i++ {
		next := renderOnce(rendered, r)
		if next == rendered {
			break
		}
		rendered = next
	}
	return rendered
}

func renderOnce(input string, r Resolver) string {
	tokens := Tokenize(input)
	if len(tokens) == 0 {
		return input
	}
	var b strings.Builder
	b.Grow(len(input))
	for _, tok := range tokens {
		if tok.Kind == KindLiteral || tok.Kind == KindUnknown {
			b.WriteString(tok.Raw)
			continue
		}
		value, ok := resolve(tok, r)
		if !ok {
			b.WriteString(tok.Raw)
			continue
		}
		b.WriteString(value)
	}
	return b.String()
}

func resolve(tok Token, r Resolver) (string, bool) {
	switch tok.Kind {
	case KindMock:
		if r.Mock != nil {
			return r.Mock(tok)
		}
		// No implicit expansion: callers that want `{{$mock.*}}` evaluated
		// must opt in by assigning `templating.ExpandMock` to Resolver.Mock.
		// This preserves the behaviour of `paramsource.renderTemplate`,
		// which historically only substituted plain variables.
	case KindStep:
		if r.Step != nil {
			if v, ok := r.Step(tok); ok {
				return v, true
			}
		}
		if r.Var != nil {
			ftok := tok
			ftok.Var.Name = tok.Body
			return r.Var(ftok)
		}
	case KindSQL:
		if r.SQL != nil {
			if v, ok := r.SQL(tok); ok {
				return v, true
			}
		}
		if r.Var != nil {
			ftok := tok
			ftok.Var.Name = tok.SQL.Expression
			return r.Var(ftok)
		}
	case KindDS:
		if r.DS != nil {
			if v, ok := r.DS(tok); ok {
				return v, true
			}
		}
		if r.Var != nil {
			ftok := tok
			ftok.Var.Name = tok.DS.Expression
			return r.Var(ftok)
		}
	case KindRequest:
		if r.Request != nil {
			return r.Request(tok)
		}
	case KindVar:
		if r.Var != nil {
			return r.Var(tok)
		}
	}
	return "", false
}

// ExpandMock evaluates a KindMock token using the built-in mockdata helper
// catalogue. It returns ok=false for unknown helper names so callers can
// preserve the original token (mirroring `mockdata.Expand`).
func ExpandMock(tok Token) (string, bool) {
	if tok.Kind != KindMock {
		return "", false
	}
	value, ok := mockdata.Eval(tok.Mock.Helper, tok.Mock.Args)
	if !ok {
		return "", false
	}
	return value, true
}

// ExpandMockTokens replaces every `{{$mock.*}}` occurrence in input with a
// freshly generated helper value. Unknown helpers are preserved verbatim.
// It is the templating-package equivalent of `mockdata.Expand` and is used by
// migrated call sites that previously called the latter directly.
func ExpandMockTokens(input string) string {
	if !mockdata.HasToken(input) {
		return input
	}
	return Render(input, Resolver{
		Mock: ExpandMock,
	})
}

// VarsResolver returns a Var hook that looks up names in vars. It mirrors
// the historical `\{\{+name\}\}+` regex by accepting both the canonical and
// over-wrapped brace forms (the parser already collapses multi-brace tokens
// into a single KindVar, so the look-up is a plain map read here).
func VarsResolver(vars map[string]string) func(Token) (string, bool) {
	return func(t Token) (string, bool) {
		name := t.Var.Name
		if name == "" {
			return "", false
		}
		v, ok := vars[name]
		return v, ok
	}
}

// StepRefs scans input for `{{$steps[N].*}}` tokens and resolves each via
// resolveStep. Resolved values are written into out so the caller's
// downstream renderVariables pass can substitute them. resolveStep returns
// ok=false when the reference cannot be resolved (missing step / path).
//
// This helper preserves the exact semantics of the legacy
// `runner.injectStepRefs` function: tokens that are already keys in out are
// skipped, and tokens that fail to resolve are simply not written so the
// caller can decide whether to fail loudly or render-through.
func StepRefs(input string, out map[string]string, resolveStep func(StepToken) (string, bool)) {
	if out == nil || resolveStep == nil {
		return
	}
	if !HasStepToken(input) {
		return
	}
	Iterate(input, func(t Token) bool {
		if t.Kind != KindStep {
			return true
		}
		key := t.Body
		if _, exists := out[key]; exists {
			return true
		}
		v, ok := resolveStep(t.Step)
		if ok {
			out[key] = v
		}
		return true
	})
}
