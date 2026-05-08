package scenario

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateScenarioInput) (*Scenario, error) {
	if input.Name == "" {
		return nil, errors.New("scenario name is required")
	}
	if input.ProjectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if input.ServiceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	return s.repo.Create(ctx, input)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input UpdateScenarioInput) (*Scenario, error) {
	if input.Name == "" {
		return nil, errors.New("scenario name is required")
	}
	return s.repo.Update(ctx, id, input)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Scenario, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Scenario, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) UpsertStep(ctx context.Context, scenarioID uuid.UUID, input UpsertStepInput) (*Step, error) {
	if input.StepOrder <= 0 {
		return nil, errors.New("stepOrder must be >= 1")
	}
	if input.StepType == "" {
		input.StepType = StepTypeAPI
	}
	if input.AttachUnderParent != nil {
		if err := validateAttachUnderParent(input.AttachUnderParent); err != nil {
			return nil, err
		}
	}
	switch input.StepType {
	case StepTypeAPI:
		if input.TestCaseID == uuid.Nil {
			return nil, errors.New("testCaseId is required for api steps")
		}
	case StepTypeDatabase, StepTypeScript, StepTypeFor, StepTypeCondition:
		if len(input.Config) == 0 {
			input.Config = []byte(`{}`)
		}
		var cfg map[string]any
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return nil, errors.New("config must be a valid JSON object")
		}
	default:
		return nil, errors.New("stepType must be api, database, script, for or condition")
	}
	attach := input.AttachUnderParent
	input.AttachUnderParent = nil

	step, err := s.repo.UpsertStep(ctx, scenarioID, input)
	if err != nil {
		return nil, err
	}
	if attach != nil {
		if err := s.attachStepUnderParent(ctx, scenarioID, step, *attach); err != nil {
			return nil, err
		}
	}
	return step, nil
}

func (s *Service) DeleteStep(ctx context.Context, stepID uuid.UUID) error {
	return s.repo.DeleteStep(ctx, stepID)
}

func (s *Service) ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]Step, error) {
	return s.repo.ListSteps(ctx, scenarioID)
}

func (s *Service) ReorderSteps(ctx context.Context, scenarioID uuid.UUID, input ReorderStepsInput) error {
	if len(input.StepIDs) == 0 {
		return errors.New("stepIds is required")
	}
	if _, err := s.repo.Get(ctx, scenarioID); err != nil {
		return err
	}
	return s.repo.ReorderSteps(ctx, scenarioID, input.StepIDs)
}
