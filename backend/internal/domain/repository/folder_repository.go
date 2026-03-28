// Package repository defines repository interfaces for domain entities.
package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// FolderRepository defines the interface for folder index storage.
type FolderRepository interface {
	// Single folder operations
	GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FolderID) (*entity.FolderEntry, error)
	GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FolderEntry, error)
	Upsert(ctx context.Context, workspaceID entity.WorkspaceID, folder *entity.FolderEntry) error
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FolderID) error

	// Batch operations
	BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, folders []*entity.FolderEntry) (int, error)
	BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FolderID) (int, error)

	// Query operations
	List(ctx context.Context, workspaceID entity.WorkspaceID, opts FolderListOptions) ([]*entity.FolderEntry, error)
	ListByParent(ctx context.Context, workspaceID entity.WorkspaceID, parentPath string, opts FolderListOptions) ([]*entity.FolderEntry, error)
	ListByDepth(ctx context.Context, workspaceID entity.WorkspaceID, depth int, opts FolderListOptions) ([]*entity.FolderEntry, error)
	ListByNature(ctx context.Context, workspaceID entity.WorkspaceID, nature entity.FolderNature, opts FolderListOptions) ([]*entity.FolderEntry, error)

	// Hierarchy operations
	GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentPath string) ([]*entity.FolderEntry, error)
	GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, path string) ([]*entity.FolderEntry, error)
	GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, path string) ([]*entity.FolderEntry, error)

	// Stats
	GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*FolderStats, error)
	Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error)

	// Faceting operations
	GetNatureFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	GetDepthFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[int]int, error)
	GetDominantTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	GetFileSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]NumericRangeCount, error)
	GetFileCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]NumericRangeCount, error)

	// Clear all folders for workspace
	DeleteAll(ctx context.Context, workspaceID entity.WorkspaceID) error
}

// FolderListOptions contains options for listing folders.
type FolderListOptions struct {
	Offset   int
	Limit    int
	SortBy   string
	SortDesc bool
}

// DefaultFolderListOptions returns default list options.
func DefaultFolderListOptions() FolderListOptions {
	return FolderListOptions{
		Offset:   0,
		Limit:    1000,
		SortBy:   "relative_path",
		SortDesc: false,
	}
}

// FolderStats contains folder index statistics.
type FolderStats struct {
	TotalFolders       int
	MaxDepth           int
	NatureCounts       map[string]int
	TotalFilesInAll    int
	TotalSizeInAll     int64
	AverageFilesPerDir float64
}
