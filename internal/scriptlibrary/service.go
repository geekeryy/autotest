package scriptlibrary

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrTemplateNotFound        = errors.New("script library template not found")
	ErrBuiltinTemplateReadonly = errors.New("built-in script template cannot be deleted")
)

const (
	scopeAssertion = "assertion"
	scopeScenario  = "scenario"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Template, error) {
	normalizeTemplateInput(&input.Name, &input.Category, &input.Description, &input.Scopes)
	if err := validateCreate(input); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, input)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input UpdateInput) (*Template, error) {
	if id == uuid.Nil {
		return nil, errors.New("id is required")
	}
	normalizeTemplateUpdate(&input.Name, &input.Category, &input.Description, &input.Scopes)
	if err := validateUpdate(input); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, input)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("id is required")
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.IsBuiltin {
		return ErrBuiltinTemplateReadonly
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]Template, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	return s.repo.ListByProject(ctx, projectID)
}

func validateCreate(in CreateInput) error {
	if in.ProjectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	return validateUpdate(UpdateInput{
		Name:        in.Name,
		Category:    in.Category,
		Description: in.Description,
		Code:        in.Code,
		Scopes:      in.Scopes,
	})
}

func validateUpdate(in UpdateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(in.Code) == "" {
		return errors.New("code is required")
	}
	// 当用户主动传入 scopes 时，必须至少包含一个有效作用域（assertion 或 scenario）；
	// 留空才允许回退到默认值（兼容旧客户端不传该字段的场景）。
	if len(in.Scopes) > 0 && len(filterValidScopes(in.Scopes)) == 0 {
		return errors.New("scopes must include at least one of assertion, scenario")
	}
	return nil
}

func filterValidScopes(scopes []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range scopes {
		s := strings.TrimSpace(strings.ToLower(raw))
		switch s {
		case scopeAssertion, scopeScenario:
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				out = append(out, s)
			}
		}
	}
	return out
}

func normalizeScopes(scopes []string) []string {
	out := filterValidScopes(scopes)
	if len(out) == 0 {
		return []string{scopeAssertion, scopeScenario}
	}
	return out
}

func normalizeTemplateInput(name, category, description *string, scopes *[]string) {
	*name = strings.TrimSpace(*name)
	*category = strings.TrimSpace(*category)
	if *category == "" {
		*category = "自定义"
	}
	*description = strings.TrimSpace(*description)
	if scopes != nil {
		*scopes = normalizeScopes(*scopes)
	}
}

func normalizeTemplateUpdate(name, category, description *string, scopes *[]string) {
	normalizeTemplateInput(name, category, description, scopes)
}
