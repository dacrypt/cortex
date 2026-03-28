package project

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Service provides project management operations.
type Service struct {
	projectRepo repository.ProjectRepository
}

// NewService creates a new project service.
func NewService(projectRepo repository.ProjectRepository) *Service {
	return &Service{
		projectRepo: projectRepo,
	}
}

// CreateProject creates a new project.
func (s *Service) CreateProject(ctx context.Context, workspaceID entity.WorkspaceID, name, description string, parentID *entity.ProjectID) (*entity.Project, error) {
	return s.CreateProjectWithNature(ctx, workspaceID, name, description, parentID, entity.NatureGeneric, nil)
}

// CreateProjectWithNature creates a new project with a specific nature and attributes.
func (s *Service) CreateProjectWithNature(ctx context.Context, workspaceID entity.WorkspaceID, name, description string, parentID *entity.ProjectID, nature entity.ProjectNature, attributes *entity.ProjectAttributes) (*entity.Project, error) {
	project := entity.NewProjectWithNature(workspaceID, name, parentID, nature)
	project.Description = description
	if attributes != nil {
		project.Attributes = attributes
	}

	if err := s.projectRepo.Create(ctx, workspaceID, project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
}

// GetProject retrieves a project by ID.
func (s *Service) GetProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) (*entity.Project, error) {
	return s.projectRepo.Get(ctx, workspaceID, projectID)
}

// GetProjectByName retrieves a project by name.
func (s *Service) GetProjectByName(ctx context.Context, workspaceID entity.WorkspaceID, name string) (*entity.Project, error) {
	return s.projectRepo.GetByName(ctx, workspaceID, name, nil)
}

// UpdateProject updates an existing project.
func (s *Service) UpdateProject(ctx context.Context, workspaceID entity.WorkspaceID, project *entity.Project) error {
	return s.projectRepo.Update(ctx, workspaceID, project)
}

// DeleteProject deletes a project.
func (s *Service) DeleteProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) error {
	return s.projectRepo.Delete(ctx, workspaceID, projectID)
}

// ListProjects lists projects with optional filtering.
func (s *Service) ListProjects(ctx context.Context, workspaceID entity.WorkspaceID, parentID *entity.ProjectID) ([]*entity.Project, error) {
	// If parentID is specified, get children; otherwise get all
	if parentID != nil {
		return s.projectRepo.GetChildren(ctx, workspaceID, *parentID)
	}
	return s.projectRepo.List(ctx, workspaceID)
}

// GetProjectChildren returns child projects.
func (s *Service) GetProjectChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.ProjectID) ([]*entity.Project, error) {
	return s.projectRepo.GetChildren(ctx, workspaceID, parentID)
}

// GetProjectParents returns parent projects.
func (s *Service) GetProjectParents(ctx context.Context, workspaceID entity.WorkspaceID, childID entity.ProjectID) ([]*entity.Project, error) {
	return s.projectRepo.GetAncestors(ctx, workspaceID, childID)
}
