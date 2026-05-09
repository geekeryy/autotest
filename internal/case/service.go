package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/sampler"

	"github.com/google/uuid"
)

type Service struct {
	repo           *Repository
	endpointSchema EndpointSchemaGetter
}

func NewService(repo *Repository, endpointSchema EndpointSchemaGetter) *Service {
	return &Service{repo: repo, endpointSchema: endpointSchema}
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
		return nil, errors.New("parent case id is required")
	}
	return s.repo.ListSaved(ctx, parentCaseID)
}

func (s *Service) Get(ctx context.Context, testCaseID uuid.UUID) (*TestCase, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("testCaseId is required")
	}
	return s.repo.Get(ctx, testCaseID)
}

func (s *Service) CreateSaved(ctx context.Context, parentCaseID uuid.UUID, input CreateSavedInput) (*TestCase, error) {
	if parentCaseID == uuid.Nil {
		return nil, errors.New("parent case id is required")
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
		return errors.New("case id is required")
	}
	return s.repo.DeleteSaved(ctx, parentCaseID, savedCaseID)
}

func (s *Service) Rename(ctx context.Context, testCaseID uuid.UUID, input RenameInput) (*TestCase, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("testCaseId is required")
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
		return nil, errors.New("testCaseId is required")
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

// GenerateParams generates default request parameter values for a test case based on its
// associated endpoint's OpenAPI request schema.
func (s *Service) GenerateParams(ctx context.Context, testCaseID uuid.UUID) (*GeneratedParams, error) {
	if testCaseID == uuid.Nil {
		return nil, errors.New("testCaseId is required")
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

	// 运行控制台「一键生成参数」走 PreferMockTags 模式：fallback 字符串字段
	// 输出 `{{$mock.<helper>}}` 占位，由 Runner 在每次发请求时实时生成新值，
	// 无需用户为 id / email / createdAt 等动态字段反复点击「生成参数」。
	sample := sampler.FromSchemaWithOptions(requestSchema, sampler.Options{PreferMockTags: true})
	return &GeneratedParams{
		Query: sample.Query,
		Path:  sample.Path,
		Body:  sample.Body,
	}, nil
}
