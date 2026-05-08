package projectprompt

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Service wraps the repository and applies business validation.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns all active prompts for a project.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]ProjectPrompt, error) {
	return s.repo.List(ctx, projectID)
}

// GetByAction returns the prompt override for a specific action.
func (s *Service) GetByAction(ctx context.Context, projectID uuid.UUID, action string) (*ProjectPrompt, error) {
	return s.repo.GetByAction(ctx, projectID, action)
}

// Create validates and inserts a new prompt record.
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*ProjectPrompt, error) {
	input.Action = strings.TrimSpace(input.Action)
	if !validAction(input.Action) {
		return nil, errors.New("invalid action: must be one of generate_params, generate_assertion, generate_case_data, raw")
	}
	if err := s.validateOptionalProvider(ctx, projectID, input.ProviderID); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, projectID, input)
}

// Update validates and mutates an existing prompt record.
func (s *Service) Update(ctx context.Context, projectID, promptID uuid.UUID, input UpdateInput) (*ProjectPrompt, error) {
	if err := s.validateOptionalProvider(ctx, projectID, input.ProviderID); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, projectID, promptID, input)
}

// Delete soft-deletes a prompt record.
func (s *Service) Delete(ctx context.Context, projectID, promptID uuid.UUID) error {
	return s.repo.Delete(ctx, projectID, promptID)
}

func (s *Service) validateOptionalProvider(ctx context.Context, projectID uuid.UUID, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	ok, err := s.repo.ProviderExistsForProject(ctx, projectID, *id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("providerId must refer to an AI provider in this project that is not deleted")
	}
	return nil
}
