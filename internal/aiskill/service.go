package aiskill

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service provides skill operations with user/project isolation.
type Service struct {
	repo            *Repository
	candidateRepo   *CandidateRepository
	registry        *Registry
	discovery       *DiscoveryDeps
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// WithCandidateRepo injects the candidate repository for skill discovery.
func (s *Service) WithCandidateRepo(repo *CandidateRepository) *Service {
	s.candidateRepo = repo
	return s
}

// WithDiscovery injects the discovery dependencies.
func (s *Service) WithDiscovery(d *DiscoveryDeps) *Service {
	s.discovery = d
	return s
}

// CreateSkill creates a new skill.
func (s *Service) CreateSkill(ctx context.Context, projectID *uuid.UUID, userID uuid.UUID, input CreateSkillInput) (*Skill, error) {
	if userID == uuid.Nil {
		return nil, errors.New("userId 不能为空")
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("name 不能为空")
	}
	if strings.TrimSpace(input.Label) == "" {
		return nil, errors.New("label 不能为空")
	}
	if strings.TrimSpace(input.PromptTemplate) == "" {
		return nil, errors.New("promptTemplate 不能为空")
	}

	return s.repo.Create(ctx, projectID, userID, input)
}

// ListSkills returns skills for a project.
func (s *Service) ListSkills(ctx context.Context, opts ListSkillsOptions) ([]Skill, error) {
	return s.repo.List(ctx, opts)
}

// GetSkill returns a single skill.
func (s *Service) GetSkill(ctx context.Context, id uuid.UUID) (*Skill, error) {
	return s.repo.Get(ctx, id)
}

// GetSkillByName returns a skill by name.
func (s *Service) GetSkillByName(ctx context.Context, name string, projectID *uuid.UUID) (*Skill, error) {
	return s.repo.GetByName(ctx, name, projectID)
}

// UpdateSkill modifies a skill.
func (s *Service) UpdateSkill(ctx context.Context, id uuid.UUID, input UpdateSkillInput) (*Skill, error) {
	return s.repo.Update(ctx, id, input)
}

// DeleteSkill removes a skill.
func (s *Service) DeleteSkill(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// MatchSkills finds skills matching the input and page context.
func (s *Service) MatchSkills(ctx context.Context, projectID *uuid.UUID, input string, pageContext []byte) ([]*Skill, error) {
	// Load enabled skills
	enabled := true
	skills, err := s.repo.List(ctx, ListSkillsOptions{
		Enabled:   &enabled,
		ProjectID: projectID,
		Limit:     100,
	})
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	skillPtrs := make([]*Skill, len(skills))
	for i := range skills {
		skillPtrs[i] = &skills[i]
	}

	// Create registry and match
	registry := NewRegistry(skillPtrs)
	return registry.Match(input, pageContext), nil
}

// InitPresetSkills initializes preset skills if they don't exist.
func (s *Service) InitPresetSkills(ctx context.Context) error {
	presets := PresetSkills()
	for _, preset := range presets {
		// Check if skill already exists
		_, err := s.repo.GetByName(ctx, preset.Name, nil)
		if err == nil {
			continue // Already exists
		}

		// Create the skill
		_, err = s.repo.Create(ctx, nil, uuid.Nil, CreateSkillInput{
			Name:           preset.Name,
			Label:          preset.Label,
			Description:    preset.Description,
			PromptTemplate: preset.PromptTemplate,
			ToolDomains:    preset.ToolDomains,
			Triggers:       preset.Triggers,
		})
		if err != nil {
			// Log error but continue with other presets
			continue
		}
	}
	return nil
}

// ListCandidates returns skill candidates with the given status.
func (s *Service) ListCandidates(ctx context.Context, status string, limit int) ([]SkillCandidate, error) {
	if s.candidateRepo == nil {
		return nil, errors.New("candidate repository not configured")
	}
	return s.candidateRepo.List(ctx, status, limit)
}

// ApproveCandidate approves a pending candidate and creates a正式技能.
func (s *Service) ApproveCandidate(ctx context.Context, id uuid.UUID) error {
	if s.candidateRepo == nil {
		return errors.New("candidate repository not configured")
	}
	return s.candidateRepo.UpdateStatus(ctx, id, "approved")
}

// RejectCandidate rejects a pending candidate.
func (s *Service) RejectCandidate(ctx context.Context, id uuid.UUID) error {
	if s.candidateRepo == nil {
		return errors.New("candidate repository not configured")
	}
	return s.candidateRepo.UpdateStatus(ctx, id, "rejected")
}

// DiscoverCandidates runs the skill discovery pipeline.
func (s *Service) DiscoverCandidates(ctx context.Context, since time.Time) ([]SkillCandidate, error) {
	if s.discovery == nil {
		return nil, errors.New("discovery not configured")
	}
	return s.discovery.DiscoverCandidates(ctx, since)
}
