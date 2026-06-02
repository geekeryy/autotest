package testprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistence interface for project test profiles.
type Repository interface {
	Get(ctx context.Context, projectID uuid.UUID) (*Profile, error)
	Upsert(ctx context.Context, profile *Profile) error
}

// DataSource provides the raw data needed to build a profile.
type DataSource interface {
	ListEndpoints(ctx context.Context, projectID uuid.UUID) ([]EndpointInfo, error)
	ListCases(ctx context.Context, projectID uuid.UUID) ([]CaseInfo, error)
	ListSuccessfulResults(ctx context.Context, projectID uuid.UUID, limit int) ([]RunResultInfo, error)
}

// Service builds and retrieves project test profiles.
type Service struct {
	repo Repository
	data DataSource
}

func NewService(repo Repository, data DataSource) *Service {
	return &Service{repo: repo, data: data}
}

// Get retrieves the current profile for a project. Returns nil without error
// if no profile has been built yet.
func (s *Service) Get(ctx context.Context, projectID uuid.UUID) (*Profile, error) {
	return s.repo.Get(ctx, projectID)
}

// Build analyzes all available data for a project and builds/updates its
// test profile. This is idempotent and safe to call repeatedly.
func (s *Service) Build(ctx context.Context, projectID uuid.UUID) (*Profile, error) {
	endpoints, err := s.data.ListEndpoints(ctx, projectID)
	if err != nil {
		return nil, err
	}

	cases, err := s.data.ListCases(ctx, projectID)
	if err != nil {
		return nil, err
	}

	results, err := s.data.ListSuccessfulResults(ctx, projectID, 500)
	if err != nil {
		return nil, err
	}

	profile := s.buildProfile(projectID, endpoints, cases, results)

	if err := s.repo.Upsert(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// GetAssertions returns enriched assertions for a given endpoint based on the
// project profile. Falls back to basic status_code assertion if no profile exists.
func (s *Service) GetAssertions(ctx context.Context, projectID uuid.UUID, expectedStatus int) (json.RawMessage, error) {
	profile, err := s.repo.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var assertions []map[string]any
	if profile != nil && profile.ResponseConvention != nil {
		assertions = BuildConventionAssertions(profile.ResponseConvention, expectedStatus)
	} else {
		assertions = []map[string]any{
			{"type": "status_code", "expected": expectedStatus},
		}
	}

	return json.Marshal(assertions)
}

// GetPromptContext builds a context string suitable for injection into AI
// prompts. It includes response convention info, field enums, and dependency hints.
func (s *Service) GetPromptContext(ctx context.Context, projectID uuid.UUID) (string, error) {
	profile, err := s.repo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return "", err
	}
	return FormatProfileForPrompt(profile), nil
}

// GetConvention returns just the response convention and field enum data
// for narrow injection into generate_assertion prompts.
func (s *Service) GetConvention(ctx context.Context, projectID uuid.UUID) (*ConventionBrief, error) {
	profile, err := s.repo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return nil, err
	}
	brief := &ConventionBrief{}
	if profile.ResponseConvention != nil {
		brief.Convention = FormatConventionForPrompt(profile.ResponseConvention)
	}
	enumFields := filterEnumFields(profile.FieldProfiles)
	if len(enumFields) > 0 {
		brief.Enums = formatEnumFieldsForPrompt(enumFields)
	}
	if brief.Convention == "" && brief.Enums == "" {
		return nil, nil
	}
	return brief, nil
}

// ConventionBrief holds narrow profile data for assertion generation.
type ConventionBrief struct {
	Convention string `json:"convention,omitempty"`
	Enums      string `json:"enums,omitempty"`
}

func formatEnumFieldsForPrompt(fields []FieldProfile) string {
	var sb strings.Builder
	sb.WriteString("以下字段有确定的取值范围，断言中可用于验证返回值：\n")
	for _, fp := range fields {
		values := formatValues(fp.ObservedValues)
		sb.WriteString(fmt.Sprintf("- %s %s → %s.%s: [%s]\n",
			strings.ToUpper(fp.Method), fp.Path, fp.Location, fp.FieldPath, values))
	}
	return sb.String()
}

func (s *Service) buildProfile(projectID uuid.UUID, endpoints []EndpointInfo, cases []CaseInfo, results []RunResultInfo) *Profile {
	profile := &Profile{
		ID:        uuid.New(),
		ProjectID: projectID,
		UpdatedAt: time.Now().UTC(),
	}

	// 1. Infer response convention from successful response snapshots.
	var responses []json.RawMessage
	for _, r := range results {
		if len(r.ResponseSnapshot) > 0 {
			body := extractResponseBody(r.ResponseSnapshot)
			if body != nil {
				responses = append(responses, body)
			}
		}
	}
	for _, c := range cases {
		if len(c.LastResponseSnapshot) > 0 {
			body := extractResponseBody(c.LastResponseSnapshot)
			if body != nil {
				responses = append(responses, body)
			}
		}
	}
	profile.ResponseConvention = InferResponseConvention(responses)

	// 2. Learn field profiles from run results.
	profile.FieldProfiles = LearnFieldProfiles(results)

	// 3. Infer dependency graph from endpoint schemas.
	profile.DependencyGraph = InferDependencies(endpoints)

	// 4. Detect auth patterns from successful requests.
	profile.AuthPatterns = InferAuthPatterns(results)

	// 5. Select example cases for few-shot.
	profile.ExampleCaseIDs = SelectExamples(cases, "", "")

	return profile
}

func extractResponseBody(snapshot json.RawMessage) json.RawMessage {
	var snap map[string]json.RawMessage
	if err := json.Unmarshal(snapshot, &snap); err != nil {
		return nil
	}
	if body, ok := snap["body"]; ok && len(body) > 2 {
		return body
	}
	return nil
}

// InferAuthPatterns detects common authentication headers from successful requests.
func InferAuthPatterns(results []RunResultInfo) []AuthPattern {
	if len(results) == 0 {
		return nil
	}

	headerCounts := map[string]map[string]int{}
	total := 0

	for _, r := range results {
		if r.Status != "passed" {
			continue
		}
		var reqSnap map[string]json.RawMessage
		if err := json.Unmarshal(r.RequestSnapshot, &reqSnap); err != nil {
			continue
		}
		total++

		var headers map[string]string
		if err := json.Unmarshal(reqSnap["headers"], &headers); err != nil {
			continue
		}
		for name, value := range headers {
			lower := normalizeHeaderName(name)
			if !isAuthHeader(lower) {
				continue
			}
			pattern := classifyHeaderValue(value)
			if _, ok := headerCounts[lower]; !ok {
				headerCounts[lower] = map[string]int{}
			}
			headerCounts[lower][pattern]++
		}
	}

	if total == 0 {
		return nil
	}

	var patterns []AuthPattern
	for name, patternMap := range headerCounts {
		for pattern, count := range patternMap {
			freq := float64(count) / float64(total)
			if freq >= 0.3 {
				patterns = append(patterns, AuthPattern{
					HeaderName:   name,
					ValuePattern: pattern,
					Frequency:    freq,
				})
			}
		}
	}
	return patterns
}

func normalizeHeaderName(name string) string {
	parts := splitHeaderName(name)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = string(upper(p[0])) + lower(p[1:])
		}
	}
	return join(parts, "-")
}

func splitHeaderName(name string) []string {
	return splitBy(name, '-')
}

func splitBy(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func upper(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 32
	}
	return b
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func join(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

func isAuthHeader(name string) bool {
	lower := strings.ToLower(name)
	return lower == "authorization" || lower == "x-token" || lower == "x-api-key" ||
		lower == "x-access-token" || lower == "token" || lower == "x-auth-token"
}

func classifyHeaderValue(value string) string {
	if strings.HasPrefix(value, "Bearer ") {
		return "Bearer {{token}}"
	}
	if strings.HasPrefix(value, "Basic ") {
		return "Basic {{credentials}}"
	}
	if len(value) > 20 {
		return "{{token}}"
	}
	return value
}
