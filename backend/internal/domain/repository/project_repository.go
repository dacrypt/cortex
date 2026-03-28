package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ProjectRepository defines storage for projects and their hierarchy.
type ProjectRepository interface {
	// CRUD operations
	Create(ctx context.Context, workspaceID entity.WorkspaceID, project *entity.Project) error
	Get(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) (*entity.Project, error)
	GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, path string) (*entity.Project, error)
	GetByName(ctx context.Context, workspaceID entity.WorkspaceID, name string, parentID *entity.ProjectID) (*entity.Project, error)
	Update(ctx context.Context, workspaceID entity.WorkspaceID, project *entity.Project) error
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) error
	List(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.Project, error)

	// Graph operations
	GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.ProjectID) ([]*entity.Project, error)
	GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) ([]*entity.Project, error)
	GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) ([]*entity.Project, error)
	GetRootProjects(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.Project, error)

	// Project-document relationships
	AddDocument(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID, role entity.ProjectDocumentRole) error
	RemoveDocument(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID) error
	GetDocuments(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, includeSubprojects bool) ([]entity.DocumentID, error)
	GetProjectsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]entity.ProjectID, error)
}

