package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// MetadataRepository defines the interface for semantic metadata storage.
type MetadataRepository interface {
	// CRUD operations
	GetOrCreate(ctx context.Context, workspaceID entity.WorkspaceID, relativePath, extension string) (*entity.FileMetadata, error)
	Get(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.FileMetadata, error)
	GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileMetadata, error)
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// Tag operations
	AddTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error
	RemoveTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error
	GetAllTags(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error)
	GetTagCounts(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	ListByTag(ctx context.Context, workspaceID entity.WorkspaceID, tag string, opts FileListOptions) ([]*entity.FileMetadata, error)

	// Context operations
	AddContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error
	RemoveContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error
	GetAllContexts(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error)
	GetContextCounts(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error)
	ListByContext(ctx context.Context, workspaceID entity.WorkspaceID, context string, opts FileListOptions) ([]*entity.FileMetadata, error)

	// Suggestion operations
	AddSuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error
	RemoveSuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error
	ClearSuggestedContexts(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error
	GetAllSuggestedContexts(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error)
	ListBySuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, context string, opts FileListOptions) ([]*entity.FileMetadata, error)
	GetFilesWithSuggestions(ctx context.Context, workspaceID entity.WorkspaceID, opts FileListOptions) ([]*entity.FileMetadata, error)

	// Notes operations
	UpdateNotes(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, notes string) error

	// Language detection operations
	UpdateDetectedLanguage(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, languageCode string) error

	// AI Summary operations
	UpdateAISummary(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, summary entity.AISummary) error
	ClearAISummary(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// AI Category operations
	UpdateAICategory(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, category entity.AICategory) error
	ClearAICategory(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// AI Related operations
	UpdateAIRelated(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, related []entity.RelatedFile) error

	// AI Context operations
	UpdateAIContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, aiContext *entity.AIContext) error
	ClearAIContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// Enrichment Data operations
	UpdateEnrichmentData(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, enrichmentData *entity.EnrichmentData) error
	ClearEnrichmentData(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// Mirror operations
	UpdateMirror(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, mirror entity.MirrorMetadata) error
	ClearMirror(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error

	// Batch operations
	EnsureMetadataForFiles(ctx context.Context, workspaceID entity.WorkspaceID, files []FileInfo) (int, error)
	BatchAddTag(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, tag string) (int, error)
	BatchAddContext(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, context string) (int, error)

	// Query operations
	ListByType(ctx context.Context, workspaceID entity.WorkspaceID, fileType string, opts FileListOptions) ([]*entity.FileMetadata, error)
	GetAllTypes(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error)
	// ListByFolderPrefix returns all files with paths starting with the given folder prefix.
	// Used for propagating project/context assignments to child files.
	ListByFolderPrefix(ctx context.Context, workspaceID entity.WorkspaceID, folderPrefix string, opts FileListOptions) ([]*entity.FileMetadata, error)

	// Faceting operations
	GetLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAICategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAuthorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetPublicationYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSentimentFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetDuplicateTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetOrganizationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetContentTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetPurposeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudienceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetDomainFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSubdomainFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetTopicFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetFolderNameFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetEventFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetCitationTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetRelationshipTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
}

// FileInfo contains minimal file information for batch operations.
type FileInfo struct {
	RelativePath string
	Extension    string
}
