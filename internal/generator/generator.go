package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/testprofile"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	ServiceID      uuid.UUID
	Method         string
	Path           string
	OperationID    string
	Summary        string
	RequestSchema  json.RawMessage
	ResponseSchema json.RawMessage
}

// Rule generates test case drafts from an endpoint definition.
type Rule interface {
	ID() string
	Generate(endpoint Endpoint) ([]testcase.Draft, error)
}

// ProfileAwareRule is an optional extension for rules that can leverage a
// project test profile to produce higher-quality assertions and parameters.
type ProfileAwareRule interface {
	Rule
	GenerateWithProfile(endpoint Endpoint, profile *testprofile.Profile) ([]testcase.Draft, error)
}

type Generator struct {
	rules   []Rule
	profile *testprofile.Profile
}

func NewDefault() *Generator {
	return &Generator{
		rules: []Rule{
			HappyPathRule{},
			RequiredMissingRule{},
			TypeMismatchRule{},
		},
	}
}

// WithProfile returns a copy of the generator configured with a project profile.
// Rules that implement ProfileAwareRule will use the profile for enriched generation.
func (g *Generator) WithProfile(profile *testprofile.Profile) *Generator {
	return &Generator{rules: g.rules, profile: profile}
}

func (g *Generator) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	var drafts []testcase.Draft
	for _, rule := range g.rules {
		var ruleDrafts []testcase.Draft
		var err error

		if par, ok := rule.(ProfileAwareRule); ok && g.profile != nil {
			ruleDrafts, err = par.GenerateWithProfile(endpoint, g.profile)
		} else {
			ruleDrafts, err = rule.Generate(endpoint)
		}
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", rule.ID(), err)
		}
		drafts = append(drafts, ruleDrafts...)
	}
	return drafts, nil
}

func buildFingerprint(endpoint Endpoint, ruleID string) string {
	return testcase.NewFingerprint(
		endpoint.ProjectID.String(),
		endpoint.ServiceID.String(),
		endpoint.ID.String(),
		ruleID,
		endpoint.Method,
		endpoint.Path,
	)
}

func defaultExpectedStatus(responseSchema json.RawMessage) int {
	if len(responseSchema) == 0 {
		return 200
	}
	var schema map[string]any
	if err := json.Unmarshal(responseSchema, &schema); err != nil {
		return 200
	}

	// Try to extract from OpenAPI responses map.
	if responses, ok := schema["responses"].(map[string]any); ok {
		if code := firstSuccessCode(responses); code > 0 {
			return code
		}
	}

	// Fallback: the schema itself might be a responses map (Swagger 2.0 style).
	for key := range schema {
		if len(key) == 3 && key[0] >= '2' && key[0] <= '2' {
			code := 0
			for _, c := range key {
				if c < '0' || c > '9' {
					code = 0
					break
				}
				code = code*10 + int(c-'0')
			}
			if code >= 200 && code < 300 {
				return code
			}
		}
	}

	return 200
}

// firstSuccessCode returns the first 2xx status code from a responses map.
func firstSuccessCode(responses map[string]any) int {
	best := 0
	for key := range responses {
		if len(key) != 3 {
			continue
		}
		code := 0
		valid := true
		for _, c := range key {
			if c < '0' || c > '9' {
				valid = false
				break
			}
			code = code*10 + int(c-'0')
		}
		if !valid || code < 100 {
			continue
		}
		if code >= 200 && code < 300 {
			if best == 0 {
				best = code
			}
		} else if best == 0 {
			best = code
		}
	}
	return best
}

func caseName(endpoint Endpoint) string {
	name := endpoint.Summary
	if name == "" {
		name = endpoint.OperationID
	}
	if name == "" {
		name = strings.ToUpper(endpoint.Method) + " " + endpoint.Path
	}
	return name
}
