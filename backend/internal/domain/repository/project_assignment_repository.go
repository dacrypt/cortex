package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ProjectAssignmentRepository defines storage for project assignments with scoring.
type ProjectAssignmentRepository interface {
	Upsert(ctx context.Context, assignment *entity.ProjectAssignment) error
	ListByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.ProjectAssignment, error)
	ListByProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) ([]*entity.ProjectAssignment, error)
	DeleteByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error
}

