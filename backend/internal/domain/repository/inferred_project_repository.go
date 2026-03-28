// Package repository defines repository interfaces for domain entities.
package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// InferredProject represents a project automatically detected from folder structure.
type InferredProject struct {
	ID               string
	WorkspaceID      string
	Name             string
	FolderPath       string
	Nature           string
	Confidence       float64
	FileCount        int
	IndicatorFiles   []string
	DominantLanguage string
	Description      string
	AutoCreated      bool
	LinkedProjectID  *string // Link to manually created project
}

// InferredProjectRepository defines the interface for inferred project storage.
type InferredProjectRepository interface {
	// Single project operations
	GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id string) (*InferredProject, error)
	GetByFolderPath(ctx context.Context, workspaceID entity.WorkspaceID, folderPath string) (*InferredProject, error)
	Upsert(ctx context.Context, workspaceID entity.WorkspaceID, project *InferredProject) error
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, id string) error

	// Batch operations
	BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, projects []*InferredProject) (int, error)
	BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []string) (int, error)

	// Query operations
	List(ctx context.Context, workspaceID entity.WorkspaceID, opts InferredProjectListOptions) ([]*InferredProject, error)
	ListByNature(ctx context.Context, workspaceID entity.WorkspaceID, nature string, opts InferredProjectListOptions) ([]*InferredProject, error)
	ListByLanguage(ctx context.Context, workspaceID entity.WorkspaceID, language string, opts InferredProjectListOptions) ([]*InferredProject, error)
	ListAboveConfidence(ctx context.Context, workspaceID entity.WorkspaceID, minConfidence float64, opts InferredProjectListOptions) ([]*InferredProject, error)

	// Linking operations
	LinkToProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string, projectID string) error
	UnlinkFromProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string) error
	GetLinkedProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string) (*string, error)

	// Stats
	GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*InferredProjectStats, error)
	Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error)

	// Faceting operations
	GetNatureFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	GetLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	GetConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]NumericRangeCount, error)
	GetFileCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]NumericRangeCount, error)

	// Clear all inferred projects for workspace
	DeleteAll(ctx context.Context, workspaceID entity.WorkspaceID) error
}

// InferredProjectListOptions contains options for listing inferred projects.
type InferredProjectListOptions struct {
	Offset   int
	Limit    int
	SortBy   string
	SortDesc bool
}

// DefaultInferredProjectListOptions returns default list options.
func DefaultInferredProjectListOptions() InferredProjectListOptions {
	return InferredProjectListOptions{
		Offset:   0,
		Limit:    100,
		SortBy:   "confidence",
		SortDesc: true,
	}
}

// InferredProjectStats contains inferred project statistics.
type InferredProjectStats struct {
	TotalProjects      int
	LinkedProjects     int
	UnlinkedProjects   int
	NatureCounts       map[string]int
	LanguageCounts     map[string]int
	AverageConfidence  float64
	HighConfidenceCount int // >0.8
}
