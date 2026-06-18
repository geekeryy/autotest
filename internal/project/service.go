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

func (s *ServiceLayer) CreateProject(ctx context.Context, input CreateProjectInput, createdBy uuid.UUID) (*Project, error) {
	if input.Name == "" {
		return nil, errors.New("project name is required")
	}
	return s.repo.CreateProject(ctx, input, createdBy)
}

// ListProjects returns all projects for admin users.
func (s *ServiceLayer) ListProjects(ctx context.Context) ([]Project, error) {
	return s.repo.ListProjects(ctx)
}

// ListProjectsForUser returns only projects the user is a member of.
func (s *ServiceLayer) ListProjectsForUser(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	return s.repo.ListProjectsForUser(ctx, userID)
}

func (s *ServiceLayer) GetMembership(ctx context.Context, projectID, userID uuid.UUID) (*ProjectMember, error) {
	return s.repo.GetMembership(ctx, projectID, userID)
}

func (s *ServiceLayer) ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	return s.repo.ListMembers(ctx, projectID)
}

func (s *ServiceLayer) AddMember(ctx context.Context, projectID uuid.UUID, input AddMemberInput) (*ProjectMember, error) {
	if input.UserID == uuid.Nil {
		return nil, errors.New("userId is required")
	}
	if input.Role == "" {
		input.Role = ProjectRoleDeveloper
	}
	if !ProjectRoleAtLeast(input.Role, ProjectRoleViewer) || !ProjectRoleAtLeast(ProjectRoleOwner, input.Role) {
		return nil, errors.New("invalid project role")
	}
	existing, err := s.repo.GetMembership(ctx, projectID, input.UserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrMemberAlreadyExists
	}
	return s.repo.AddMember(ctx, projectID, input)
}

func (s *ServiceLayer) UpdateMember(ctx context.Context, projectID, userID uuid.UUID, input UpdateMemberInput) (*ProjectMember, error) {
	if !ProjectRoleAtLeast(input.Role, ProjectRoleViewer) || !ProjectRoleAtLeast(ProjectRoleOwner, input.Role) {
		return nil, errors.New("invalid project role")
	}
	return s.repo.UpdateMember(ctx, projectID, userID, input)
}

func (s *ServiceLayer) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.repo.RemoveMember(ctx, projectID, userID)
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

func (s *ServiceLayer) GetService(ctx context.Context, projectID, serviceID uuid.UUID) (*Service, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	return s.repo.GetService(ctx, projectID, serviceID)
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
