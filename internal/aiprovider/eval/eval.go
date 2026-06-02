// Package eval is the offline evaluation harness for the AI assistant's
// on-demand tool routing. It loads a YAML "Golden Set" of cases and scores
// the rule-based Router selection against expectations:
//
//   - Tool Recall@Hop1: of the expected first-hop tools, how many the Router
//     actually offers.
//   - Mis-route Rate: any forbidden tool that leaks into the first hop.
//   - Domain coverage: whether each expected domain is represented.
//
// The harness is designed to run WITHOUT a real LLM: each case supplies a
// mocked planner output, and we exercise the deterministic Router + the
// first-hop tool-surface computation. A future real-LLM mode (running the
// actual Planner) can be added behind a build tag or env flag without
// changing the case schema.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"autotest/internal/aiprovider"
	"autotest/internal/aitools"
	"autotest/internal/aitools/builtin"

	"gopkg.in/yaml.v3"
)

// Case is one Golden Set entry. PlannerOutput is provided directly so the
// rule-based Router can be evaluated without an LLM.
type Case struct {
	Name                string         `yaml:"name"`
	User                string         `yaml:"user"`
	PageContext         map[string]any `yaml:"pageContext"`
	Planner             PlannerSpec    `yaml:"planner"`
	ExpectToolsFirstHop []string       `yaml:"expectToolsFirstHop"`
	ForbidTools         []string       `yaml:"forbidTools"`
	ExpectDomains       []string       `yaml:"expectDomains"`
	ExpectFinish        string         `yaml:"expectFinish"`
}

// PlannerSpec mirrors aiprovider.PlannerOutput in YAML-friendly form.
type PlannerSpec struct {
	Intent      string   `yaml:"intent"`
	Domains     []string `yaml:"domains"`
	Workflow    []string `yaml:"workflow"`
	NeedsWrite  bool     `yaml:"needsWrite"`
	Ambiguities []string `yaml:"ambiguities"`
}

func (p PlannerSpec) toOutput() aiprovider.PlannerOutput {
	return aiprovider.PlannerOutput{
		Intent:      p.Intent,
		Domains:     p.Domains,
		Workflow:    p.Workflow,
		NeedsWrite:  p.NeedsWrite,
		Ambiguities: p.Ambiguities,
	}
}

type fileDoc struct {
	Cases []Case `yaml:"cases"`
}

// CaseResult is the per-case scoring outcome.
type CaseResult struct {
	Name          string
	FirstHopTools []string
	RecallHit     int
	RecallTotal   int
	MissingTools  []string
	ForbidHits    []string
	MissingDomain []string
	Passed        bool
}

// RecallRate returns the Tool Recall@Hop1 for this case (1.0 when no
// expectations were set).
func (r CaseResult) RecallRate() float64 {
	if r.RecallTotal == 0 {
		return 1
	}
	return float64(r.RecallHit) / float64(r.RecallTotal)
}

// Report aggregates all case results.
type Report struct {
	Results []CaseResult
}

// Passed reports whether every case passed.
func (rep Report) Passed() bool {
	for _, r := range rep.Results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// MeanRecall is the average Tool Recall@Hop1 across cases.
func (rep Report) MeanRecall() float64 {
	if len(rep.Results) == 0 {
		return 0
	}
	var sum float64
	for _, r := range rep.Results {
		sum += r.RecallRate()
	}
	return sum / float64(len(rep.Results))
}

// MisrouteRate is the fraction of cases that offered at least one forbidden
// tool on the first hop.
func (rep Report) MisrouteRate() float64 {
	if len(rep.Results) == 0 {
		return 0
	}
	var bad int
	for _, r := range rep.Results {
		if len(r.ForbidHits) > 0 {
			bad++
		}
	}
	return float64(bad) / float64(len(rep.Results))
}

// LoadCases reads every *.yaml file under dir and returns the parsed cases.
func LoadCases(dir string) ([]Case, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var cases []Case
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var doc fileDoc
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		cases = append(cases, doc.Cases...)
	}
	return cases, nil
}

// DefaultCatalog builds a catalog over the full built-in DOMAIN tool set
// using empty Deps — exactly mirroring production (main.go builds the
// catalog from builtin.All, NOT including the always-on meta tools). Tool
// Run closures are never invoked by the Router, so empty deps are safe;
// only the Domain/Summary metadata matters here.
func DefaultCatalog() *aitools.Catalog {
	return aitools.NewCatalog(builtin.All(builtin.Deps{}))
}

// Evaluate runs the rule-based Router over each case and scores it.
func Evaluate(cases []Case, cat *aitools.Catalog) Report {
	router := aiprovider.NewRouter()
	rep := Report{}
	for _, c := range cases {
		routed := router.Route(c.Planner.toOutput(), pageContextJSON(c.PageContext), cat, nil)
		firstHop := aiprovider.FirstHopToolNames(routed, cat)
		rep.Results = append(rep.Results, scoreCase(c, firstHop, cat))
	}
	return rep
}

func scoreCase(c Case, firstHop []string, cat *aitools.Catalog) CaseResult {
	have := map[string]struct{}{}
	for _, n := range firstHop {
		have[n] = struct{}{}
	}
	res := CaseResult{Name: c.Name, FirstHopTools: firstHop, RecallTotal: len(c.ExpectToolsFirstHop)}
	for _, want := range c.ExpectToolsFirstHop {
		if _, ok := have[want]; ok {
			res.RecallHit++
		} else {
			res.MissingTools = append(res.MissingTools, want)
		}
	}
	for _, forbid := range c.ForbidTools {
		if _, ok := have[forbid]; ok {
			res.ForbidHits = append(res.ForbidHits, forbid)
		}
	}
	// Domain coverage: a domain is covered when at least one offered tool
	// belongs to it (per the catalog metadata).
	domainsOffered := map[string]struct{}{}
	for _, n := range firstHop {
		if t, ok := cat.Get(n); ok && t.Domain != "" {
			domainsOffered[t.Domain] = struct{}{}
		}
	}
	for _, d := range c.ExpectDomains {
		if _, ok := domainsOffered[strings.ToLower(d)]; !ok {
			res.MissingDomain = append(res.MissingDomain, d)
		}
	}
	res.Passed = len(res.MissingTools) == 0 && len(res.ForbidHits) == 0 && len(res.MissingDomain) == 0
	return res
}

// pageContextJSON re-encodes the YAML page context map as JSON for the
// Router (which consumes json.RawMessage). Returns nil on empty/failure.
func pageContextJSON(m map[string]any) []byte {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}
