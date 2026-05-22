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

// List returns all active platform prompts.
func (s *Service) List(ctx context.Context) ([]ProjectPrompt, error) {
	return s.repo.List(ctx)
}

// GetByAction returns the prompt override for a specific action.
func (s *Service) GetByAction(ctx context.Context, action string) (*ProjectPrompt, error) {
	return s.repo.GetByAction(ctx, action)
}

// Create validates and inserts a new prompt record.
func (s *Service) Create(ctx context.Context, input CreateInput) (*ProjectPrompt, error) {
	input.Action = strings.TrimSpace(input.Action)
	if !validAction(input.Action) {
		return nil, errors.New("invalid action: must be one of generate_params, generate_assertion, generate_case_data, raw")
	}
	if err := s.validateOptionalProvider(ctx, input.ProviderID); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, input)
}

// Update validates and mutates an existing prompt record.
func (s *Service) Update(ctx context.Context, promptID uuid.UUID, input UpdateInput) (*ProjectPrompt, error) {
	if err := s.validateOptionalProvider(ctx, input.ProviderID); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, promptID, input)
}

// Delete soft-deletes a prompt record.
func (s *Service) Delete(ctx context.Context, promptID uuid.UUID) error {
	return s.repo.Delete(ctx, promptID)
}

func (s *Service) validateOptionalProvider(ctx context.Context, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	ok, err := s.repo.ProviderExists(ctx, *id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("providerId must refer to a platform AI provider that is not deleted")
	}
	return nil
}
