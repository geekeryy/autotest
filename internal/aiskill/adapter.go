package aiskill

import (
	"context"

	"autotest/internal/aiprovider"

	"github.com/google/uuid"
)

// ServiceAdapter bridges aiskill.Service to aiprovider.SkillMatcher.
type ServiceAdapter struct {
	svc *Service
}

// NewServiceAdapter wraps Service so it can be plugged into
// aiprovider.Service.WithSkillMatcher().
func NewServiceAdapter(svc *Service) *ServiceAdapter {
	return &ServiceAdapter{svc: svc}
}

// compile-time interface assertion
var _ aiprovider.SkillMatcher = (*ServiceAdapter)(nil)

// MatchSkills finds skills matching the input and returns them in the
// format expected by the Router.
func (a *ServiceAdapter) MatchSkills(ctx context.Context, projectID *uuid.UUID, input string, pageContext []byte) ([]*aiprovider.MatchedSkill, error) {
	skills, err := a.svc.MatchSkills(ctx, projectID, input, pageContext)
	if err != nil {
		return nil, err
	}
	out := make([]*aiprovider.MatchedSkill, 0, len(skills))
	for _, sk := range skills {
		out = append(out, &aiprovider.MatchedSkill{
			Name:        sk.Name,
			ToolDomains: sk.ToolDomains,
		})
	}
	return out, nil
}
