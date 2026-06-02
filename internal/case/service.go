package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/sampler"
	"autotest/internal/testprofile"

	"github.com/google/uuid"
)

// ProfileGetter retrieves a project's test profile. Satisfied by *testprofile.Service.
type ProfileGetter interface {
	Get(ctx context.Context, projectID uuid.UUID) (*testprofile.Profile, error)
}

type Service struct {
	repo           *Repository
	endpointSchema EndpointSchemaGetter
	profileGetter  ProfileGetter // optional; nil disables profile-enriched generation
}

func NewService(repo *Repository, endpointSchema EndpointSchemaGetter) *Service {
	return &Service{repo: repo, endpointSchema: endpointSchema}
}

// WithProfileGetter returns a copy of the service configured with a profile
// source. When set, GenerateParams will use historically observed field values
// from the project test profile.
func (s *Service) WithProfileGetter(pg ProfileGetter) *Service {
	return &Service{
		repo:           s.repo,
		endpointSchema: s.endpointSchema,
		profileGetter:  pg,
	}
}

func (s *Service) CreateManual(ctx context.Context, input CreateManualInput) (*TestCase, error) {
	if input.ProjectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if input.ServiceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	if input.Name == "" {
		return nil, errors.New("接口名称不能为空")
	}
	if input.Method == "" || input.Path == "" {
		return nil, errors.New("method and path are required")
	}
	return s.repo.CreateManual(ctx, input)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]TestCase, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) ListSaved(ctx context.Context, parentCaseID uuid.UUID) ([]TestCase, error) {
	if parentCaseID == uuid.Nil {
		return nil, errors.New("接口模板 ID 不能为空")
	}
	return s.repo.ListSaved(ctx, parentCaseID)
}

func (s *Service) Get(ctx context.Context, testCaseID uuid.UUID) (*TestCase, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("接口模板或测试用例 ID 不能为空")
	}
	return s.repo.Get(ctx, testCaseID)
}

func (s *Service) CreateSaved(ctx context.Context, parentCaseID uuid.UUID, input CreateSavedInput) (*TestCase, error) {
	if parentCaseID == uuid.Nil {
		return nil, errors.New("接口模板 ID 不能为空")
	}
	if input.Name == "" {
		return nil, errors.New("用例名称不能为空")
	}
	if input.Method == "" || input.Path == "" {
		return nil, errors.New("method and path are required")
	}
	tc, err := s.repo.CreateSaved(ctx, parentCaseID, input)
	if errors.Is(err, ErrTestCaseNotFound) {
		return nil, ErrTestCaseNotFound
	}
	return tc, err
}

func (s *Service) DeleteSaved(ctx context.Context, parentCaseID, savedCaseID uuid.UUID) error {
	if parentCaseID == uuid.Nil || savedCaseID == uuid.Nil {
		return errors.New("接口模板 ID 和已保存测试用例 ID 不能为空")
	}
	return s.repo.DeleteSaved(ctx, parentCaseID, savedCaseID)
}

func (s *Service) Rename(ctx context.Context, testCaseID uuid.UUID, input RenameInput) (*TestCase, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("接口模板或测试用例 ID 不能为空")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("用例名称不能为空")
	}
	return s.repo.Rename(ctx, testCaseID, name)
}

// Patch applies a partial update to a test case. Currently supports name and assertions.
func (s *Service) Patch(ctx context.Context, testCaseID uuid.UUID, input PatchInput) (*TestCase, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("接口模板或测试用例 ID 不能为空")
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("用例名称不能为空")
		}
		tc, err := s.repo.Rename(ctx, testCaseID, name)
		if err != nil {
			return tc, err
		}
		if input.Assertions == nil {
			return tc, nil
		}
	}
	if input.Assertions != nil {
		return s.repo.PatchAssertions(ctx, testCaseID, input.Assertions)
	}
	return s.repo.Get(ctx, testCaseID)
}

// UpsertGenerated creates or updates an auto-generated request template from a draft.
func (s *Service) UpsertGenerated(ctx context.Context, draft Draft) (*TestCase, error) {
	return s.repo.UpsertGenerated(ctx, draft)
}

// GenerateParams generates default request parameter values for a test case based on its
// associated endpoint's OpenAPI request schema. When a ProfileGetter is configured,
// historically observed values from the project test profile are used to produce
// more realistic parameters.
func (s *Service) GenerateParams(ctx context.Context, testCaseID uuid.UUID) (*GeneratedParams, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("接口模板 ID 不能为空")
	}

	tc, err := s.repo.Get(ctx, testCaseID)
	if err != nil {
		return nil, err
	}
	if tc.EndpointID == nil || *tc.EndpointID == uuid.Nil {
		return nil, fmt.Errorf("该接口未关联 OpenAPI 接口定义，无法自动生成参数")
	}
	if s.endpointSchema == nil {
		return nil, fmt.Errorf("endpoint schema provider not configured")
	}

	requestSchema, err := s.endpointSchema.GetEndpointRequestSchema(ctx, *tc.EndpointID)
	if err != nil {
		return nil, fmt.Errorf("fetch endpoint schema: %w", err)
	}

	opts := sampler.Options{PreferMockTags: true}
	var depInfos []FieldDependencyInfo

	// When a profile is available, use historically observed field values
	// and dependency graph to produce more realistic parameters.
	if s.profileGetter != nil {
		profile, perr := s.profileGetter.Get(ctx, tc.ProjectID)
		if perr == nil && profile != nil {
			if len(profile.FieldProfiles) > 0 {
				opts.Profile = sampler.NewFieldProfileAdapter(tc.Method, tc.Path, profile.FieldProfiles)
			}
			// Build dependency hints from the profile's dependency graph.
			if len(profile.DependencyGraph) > 0 {
				hints := buildDependencyHints(tc.Method, tc.Path, profile.DependencyGraph)
				if hints != nil && len(hints.Hints) > 0 {
					opts.Dependencies = hints
					depInfos = buildFieldDependencyInfos(tc.Method, tc.Path, profile.DependencyGraph)
				}
			}
		}
	}

	sample := sampler.FromSchemaWithOptions(requestSchema, opts)
	return &GeneratedParams{
		Query:        sample.Query,
		Path:         sample.Path,
		Body:         sample.Body,
		Dependencies: depInfos,
	}, nil
}

// buildDependencyHints converts profile EndpointDependency entries targeting
// the given method+path into sampler.DependencyHints.
func buildDependencyHints(method, path string, deps []testprofile.EndpointDependency) *sampler.DependencyHints {
	var hints []sampler.DependencyHint
	for _, dep := range deps {
		if !strings.EqualFold(dep.TargetMethod, method) || dep.TargetPath != path {
			continue
		}
		if dep.Confidence < 0.5 {
			continue
		}
		fieldPath := dep.TargetField
		// Strip location prefix (e.g. "path.id" → "id", "body.userId" → "userId").
		if idx := strings.Index(fieldPath, "."); idx >= 0 {
			fieldPath = fieldPath[idx+1:]
		}
		hints = append(hints, sampler.DependencyHint{
			FieldPath:    fieldPath,
			SourceMethod: dep.SourceMethod,
			SourcePath:   dep.SourcePath,
			SourceField:  dep.SourceField,
		})
	}
	if len(hints) == 0 {
		return nil
	}
	return &sampler.DependencyHints{Hints: hints}
}

// buildFieldDependencyInfos converts profile EndpointDependency entries into
// user-facing FieldDependencyInfo for the API response.
func buildFieldDependencyInfos(method, path string, deps []testprofile.EndpointDependency) []FieldDependencyInfo {
	var infos []FieldDependencyInfo
	for _, dep := range deps {
		if !strings.EqualFold(dep.TargetMethod, method) || dep.TargetPath != path {
			continue
		}
		if dep.Confidence < 0.5 {
			continue
		}
		fieldPath := dep.TargetField
		if idx := strings.Index(fieldPath, "."); idx >= 0 {
			fieldPath = fieldPath[idx+1:]
		}
		infos = append(infos, FieldDependencyInfo{
			FieldPath:  dep.TargetField,
			DependsOn:  fmt.Sprintf("%s %s → %s", strings.ToUpper(dep.SourceMethod), dep.SourcePath, dep.SourceField),
			Suggestion: fmt.Sprintf("在场景中使用 {{$steps[N].%s}}", dep.SourceField),
		})
	}
	return infos
}
