package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// WorkspaceRepository defines the interface for workspace persistence.
type WorkspaceRepository interface {
	// CRUD operations
	Create(ctx context.Context, workspace *entity.Workspace) error
	Get(ctx context.Context, id entity.WorkspaceID) (*entity.Workspace, error)
	GetByPath(ctx context.Context, path string) (*entity.Workspace, error)
	Update(ctx context.Context, workspace *entity.Workspace) error
	Delete(ctx context.Context, id entity.WorkspaceID) error

	// Queries
	List(ctx context.Context, opts WorkspaceListOptions) ([]*entity.Workspace, error)
	ListActive(ctx context.Context) ([]*entity.Workspace, error)
	Count(ctx context.Context) (int, error)

	// Status updates
	SetActive(ctx context.Context, id entity.WorkspaceID, active bool) error
	UpdateLastIndexed(ctx context.Context, id entity.WorkspaceID) error
	UpdateFileCount(ctx context.Context, id entity.WorkspaceID, count int) error
	UpdateConfig(ctx context.Context, id entity.WorkspaceID, config entity.WorkspaceConfig) error

	// Data management
	ClearWorkspaceData(ctx context.Context, id entity.WorkspaceID, workspaceRoot string) error
	// ClearFileData clears only file-related data (files, metadata, tags, contexts)
	// but preserves documents, chunks, embeddings, and clusters for incremental clustering.
	ClearFileData(ctx context.Context, id entity.WorkspaceID, workspaceRoot string) error
}

// WorkspaceListOptions contains options for listing workspaces.
type WorkspaceListOptions struct {
	ActiveOnly bool
	Offset     int
	Limit      int
}

// DefaultWorkspaceListOptions returns default workspace list options.
func DefaultWorkspaceListOptions() WorkspaceListOptions {
	return WorkspaceListOptions{
		ActiveOnly: false,
		Offset:     0,
		Limit:      100,
	}
}
