package promptlayer

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Service provides prompt layer operations.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateLayer creates a new prompt layer.
func (s *Service) CreateLayer(ctx context.Context, input CreateLayerInput) (*PromptLayer, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("name 不能为空")
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, errors.New("content 不能为空")
	}
	return s.repo.Create(ctx, input)
}

// ListLayers returns layers with optional filtering.
func (s *Service) ListLayers(ctx context.Context, opts ListLayersOptions) ([]PromptLayer, error) {
	return s.repo.List(ctx, opts)
}

// GetLayer returns a single layer.
func (s *Service) GetLayer(ctx context.Context, id uuid.UUID) (*PromptLayer, error) {
	return s.repo.Get(ctx, id)
}

// GetLatestVersion returns the latest enabled version of a layer.
func (s *Service) GetLatestVersion(ctx context.Context, name string) (*PromptLayer, error) {
	return s.repo.GetLatestVersion(ctx, name)
}

// GetByVersion returns a specific version of a layer.
func (s *Service) GetByVersion(ctx context.Context, name string, version int) (*PromptLayer, error) {
	return s.repo.GetByVersion(ctx, name, version)
}

// UpdateLayer creates a new version of a layer.
func (s *Service) UpdateLayer(ctx context.Context, name string, input UpdateLayerInput) (*PromptLayer, error) {
	return s.repo.Update(ctx, name, input)
}

// DeleteLayer removes a layer.
func (s *Service) DeleteLayer(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// EnableVersion enables a specific version and disables others.
func (s *Service) EnableVersion(ctx context.Context, name string, version int) error {
	return s.repo.EnableVersion(ctx, name, version)
}

// GetDomainLayers returns all enabled layers for a domain.
func (s *Service) GetDomainLayers(ctx context.Context, domain string) ([]PromptLayer, error) {
	enabled := true
	return s.repo.List(ctx, ListLayersOptions{
		Domain:  domain,
		Enabled: &enabled,
	})
}

// BuildDomainPrompt builds a combined prompt from all enabled layers for a domain.
func (s *Service) BuildDomainPrompt(ctx context.Context, domain string) (string, error) {
	layers, err := s.GetDomainLayers(ctx, domain)
	if err != nil {
		return "", err
	}
	if len(layers) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for i, layer := range layers {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(layer.Content)
	}
	return sb.String(), nil
}

// --- Experiment methods ---

// CreateExperiment creates a new A/B test experiment.
func (s *Service) CreateExperiment(ctx context.Context, input CreateExperimentInput) (*PromptExperiment, error) {
	return s.repo.CreateExperiment(ctx, input)
}

// ListExperiments returns experiments with the given status filter.
func (s *Service) ListExperiments(ctx context.Context, status string) ([]PromptExperiment, error) {
	return s.repo.ListExperiments(ctx, status)
}

// StopExperiment stops a running experiment.
func (s *Service) StopExperiment(ctx context.Context, id uuid.UUID) error {
	return s.repo.StopExperiment(ctx, id)
}
