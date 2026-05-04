package project

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ServiceLayer struct {
	repo *Repository
}

func NewServiceLayer(repo *Repository) *ServiceLayer {
	return &ServiceLayer{repo: repo}
}

func (s *ServiceLayer) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if input.Name == "" {
		return nil, errors.New("project name is required")
	}
	return s.repo.CreateProject(ctx, input)
}

func (s *ServiceLayer) ListProjects(ctx context.Context) ([]Project, error) {
	return s.repo.ListProjects(ctx)
}

func (s *ServiceLayer) DeleteProject(ctx context.Context, projectID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	return s.repo.DeleteProject(ctx, projectID)
}

func (s *ServiceLayer) CreateService(ctx context.Context, projectID uuid.UUID, input CreateServiceInput) (*Service, error) {
	if input.Name == "" {
		return nil, errors.New("service name is required")
	}
	return s.repo.CreateService(ctx, projectID, input)
}

func (s *ServiceLayer) UpdateService(ctx context.Context, projectID, serviceID uuid.UUID, input UpdateServiceInput) (*Service, error) {
	if input.Name == "" {
		return nil, errors.New("service name is required")
	}
	return s.repo.UpdateService(ctx, projectID, serviceID, input)
}

func (s *ServiceLayer) DeleteService(ctx context.Context, projectID, serviceID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return errors.New("serviceId is required")
	}
	return s.repo.DeleteService(ctx, projectID, serviceID)
}

func (s *ServiceLayer) ListServices(ctx context.Context, projectID uuid.UUID) ([]Service, error) {
	return s.repo.ListServices(ctx, projectID)
}

func (s *ServiceLayer) CreateEnvironment(ctx context.Context, projectID uuid.UUID, input CreateEnvironmentInput) (*Environment, error) {
	if input.Name == "" {
		return nil, errors.New("environment name is required")
	}
	if input.BaseURL == "" {
		return nil, errors.New("environment baseUrl is required")
	}
	return s.repo.CreateEnvironment(ctx, projectID, input)
}

func (s *ServiceLayer) UpdateEnvironment(ctx context.Context, projectID, environmentID uuid.UUID, input UpdateEnvironmentInput) (*Environment, error) {
	if input.Name == "" {
		return nil, errors.New("environment name is required")
	}
	if input.BaseURL == "" {
		return nil, errors.New("environment baseUrl is required")
	}
	return s.repo.UpdateEnvironment(ctx, projectID, environmentID, input)
}

func (s *ServiceLayer) DeleteEnvironment(ctx context.Context, projectID, environmentID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if environmentID == uuid.Nil {
		return errors.New("environmentId is required")
	}
	return s.repo.DeleteEnvironment(ctx, projectID, environmentID)
}

func (s *ServiceLayer) ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]Environment, error) {
	return s.repo.ListEnvironments(ctx, projectID)
}

func (s *ServiceLayer) GetEnvironment(ctx context.Context, projectID, environmentID uuid.UUID) (*Environment, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if environmentID == uuid.Nil {
		return nil, errors.New("environmentId is required")
	}
	return s.repo.GetEnvironment(ctx, projectID, environmentID)
}

func (s *ServiceLayer) CreateServiceEnvironment(ctx context.Context, projectID, serviceID uuid.UUID, input CreateEnvironmentInput) (*Environment, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	if input.Name == "" {
		return nil, errors.New("environment name is required")
	}
	if input.BaseURL == "" {
		return nil, errors.New("environment baseUrl is required")
	}
	return s.repo.CreateServiceEnvironment(ctx, projectID, serviceID, input)
}

func (s *ServiceLayer) UpdateServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID, input UpdateEnvironmentInput) (*Environment, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	if environmentID == uuid.Nil {
		return nil, errors.New("environmentId is required")
	}
	if input.Name == "" {
		return nil, errors.New("environment name is required")
	}
	if input.BaseURL == "" {
		return nil, errors.New("environment baseUrl is required")
	}
	return s.repo.UpdateServiceEnvironment(ctx, projectID, serviceID, environmentID, input)
}

func (s *ServiceLayer) DeleteServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return errors.New("serviceId is required")
	}
	if environmentID == uuid.Nil {
		return errors.New("environmentId is required")
	}
	return s.repo.DeleteServiceEnvironment(ctx, projectID, serviceID, environmentID)
}

func (s *ServiceLayer) ListServiceEnvironments(ctx context.Context, projectID, serviceID uuid.UUID) ([]Environment, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	return s.repo.ListServiceEnvironments(ctx, projectID, serviceID)
}

func (s *ServiceLayer) GetServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID) (*Environment, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	if environmentID == uuid.Nil {
		return nil, errors.New("environmentId is required")
	}
	return s.repo.GetServiceEnvironment(ctx, projectID, serviceID, environmentID)
}
