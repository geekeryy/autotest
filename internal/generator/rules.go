package generator

import (
	"encoding/json"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/sampler"
	"autotest/internal/testprofile"
)

const (
	RuleHappyPath       = "happy_path"
	RuleRequiredMissing = "required_missing"
	RuleTypeMismatch    = "type_mismatch"
)

type HappyPathRule struct{}

func (HappyPathRule) ID() string {
	return RuleHappyPath
}

func (r HappyPathRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	return r.generate(endpoint, nil)
}

func (r HappyPathRule) GenerateWithProfile(endpoint Endpoint, profile *testprofile.Profile) ([]testcase.Draft, error) {
	return r.generate(endpoint, profile)
}

func (HappyPathRule) generate(endpoint Endpoint, profile *testprofile.Profile) ([]testcase.Draft, error) {
	var opts sampler.Options
	if profile != nil && len(profile.FieldProfiles) > 0 {
		opts.Profile = sampler.NewFieldProfileAdapter(endpoint.Method, endpoint.Path, profile.FieldProfiles)
	}
	sample := sampler.FromSchemaWithOptions(endpoint.RequestSchema, opts)
	request := map[string]any{
		"method":    strings.ToUpper(endpoint.Method),
		"path":      endpoint.Path,
		"headers":   sample.Headers,
		"query":     sample.Query,
		"variables": sample.Path,
		"body":      sample.Body,
	}
	if sample.Security != nil {
		request["security"] = sample.Security
	}

	expectedStatus := defaultExpectedStatus(endpoint.ResponseSchema)
	var assertions []map[string]any

	if profile != nil && profile.ResponseConvention != nil {
		assertions = testprofile.BuildConventionAssertions(profile.ResponseConvention, expectedStatus)
	} else {
		assertions = []map[string]any{
			{"type": "status_code", "expected": expectedStatus},
		}
	}

	rawRequest, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	rawAssertions, err := json.Marshal(assertions)
	if err != nil {
		return nil, err
	}

	endpointID := endpoint.ID
	draft := testcase.Draft{
		ProjectID:        endpoint.ProjectID,
		ServiceID:        endpoint.ServiceID,
		EndpointID:       &endpointID,
		Source:           testcase.SourceAuto,
		Name:             caseName(endpoint),
		Method:           endpoint.Method,
		Path:             endpoint.Path,
		GenerationRuleID: RuleHappyPath,
		Request:          rawRequest,
		Assertions:       rawAssertions,
		Status:           testcase.StatusActive,
	}
	draft.Fingerprint = buildFingerprint(endpoint, RuleHappyPath)
	return []testcase.Draft{draft}, nil
}

type RequiredMissingRule struct{}

func (RequiredMissingRule) ID() string {
	return RuleRequiredMissing
}

func (RequiredMissingRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	// MVP only wires the rule point. The rule can later emit one draft per required field.
	return nil, nil
}

type TypeMismatchRule struct{}

func (TypeMismatchRule) ID() string {
	return RuleTypeMismatch
}

func (TypeMismatchRule) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	// MVP only wires the rule point. The rule can later mutate generated samples by schema type.
	return nil, nil
}
