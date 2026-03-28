package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// SuggestedMetadataRepository defines the interface for suggested metadata persistence.
type SuggestedMetadataRepository interface {
	// Upsert stores or updates suggested metadata for a file.
	Upsert(ctx context.Context, workspaceID entity.WorkspaceID, suggested *entity.SuggestedMetadata) error

	// Get retrieves suggested metadata for a file by ID.
	Get(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.SuggestedMetadata, error)

	// GetByPath retrieves suggested metadata by relative path.
	GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.SuggestedMetadata, error)

	// Delete removes suggested metadata for a file.
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// AcceptTag accepts a suggested tag and removes it from suggestions.
	AcceptTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error

	// AcceptProject accepts a suggested project and removes it from suggestions.
	AcceptProject(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, projectName string) error
}







