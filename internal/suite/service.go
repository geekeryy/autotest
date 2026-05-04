package suite

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateSuiteInput) (*Suite, error) {
	if input.Name == "" {
		return nil, errors.New("suite name is required")
	}
	if input.Kind == "" {
		input.Kind = KindManual
	}
	return s.repo.Create(ctx, input)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Suite, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) AddItem(ctx context.Context, suiteID uuid.UUID, input AddItemInput) (*Item, error) {
	if input.TestCaseID == uuid.Nil {
		return nil, errors.New("testCaseId is required")
	}
	return s.repo.AddItem(ctx, suiteID, input)
}

func (s *Service) ListItems(ctx context.Context, suiteID uuid.UUID) ([]Item, error) {
	return s.repo.ListItems(ctx, suiteID)
}
