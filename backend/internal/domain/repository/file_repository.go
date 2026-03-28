// Package repository defines repository interfaces for domain entities.
package repository

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// FileRepository defines the interface for file index storage.
type FileRepository interface {
	// Single file operations
	GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) (*entity.FileEntry, error)
	GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileEntry, error)
	Upsert(ctx context.Context, workspaceID entity.WorkspaceID, file *entity.FileEntry) error
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) error

	// Batch operations
	BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, files []*entity.FileEntry) (int, error)
	BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FileID) (int, error)

	// Query operations
	List(ctx context.Context, workspaceID entity.WorkspaceID, opts FileListOptions) ([]*entity.FileEntry, error)
	ListByExtension(ctx context.Context, workspaceID entity.WorkspaceID, ext string, opts FileListOptions) ([]*entity.FileEntry, error)
	ListByFolder(ctx context.Context, workspaceID entity.WorkspaceID, folder string, recursive bool, opts FileListOptions) ([]*entity.FileEntry, error)
	ListByDateRange(ctx context.Context, workspaceID entity.WorkspaceID, start, end time.Time, opts FileListOptions) ([]*entity.FileEntry, error)
	ListBySizeRange(ctx context.Context, workspaceID entity.WorkspaceID, minSize, maxSize int64, opts FileListOptions) ([]*entity.FileEntry, error)
	ListByContentType(ctx context.Context, workspaceID entity.WorkspaceID, category string, opts FileListOptions) ([]*entity.FileEntry, error)
	Search(ctx context.Context, workspaceID entity.WorkspaceID, query string, opts FileListOptions) ([]*entity.FileEntry, error)

	// Stats
	GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*FileStats, error)
	Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error)

	// Faceting operations
	GetExtensionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetMimeTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetMimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]DateRangeCount, error)
	GetCreatedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]DateRangeCount, error)
	GetAccessedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]DateRangeCount, error)
	GetChangedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]DateRangeCount, error)
	GetOwnerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetIndexingStatusFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetIndexingErrorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetComplexityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetProjectScoreRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetTemporalPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetReadabilityLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetFunctionCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetLinesOfCodeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetCommentPercentageRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetVideoResolutionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetPermissionLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetContentQualityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetImageFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageColorSpaceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetCameraMakeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetCameraModelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageGPSLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageOrientationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageTransparencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageAnimatedFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetImageColorDepthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetImageISORangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetImageApertureRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetImageFocalLengthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetImageDimensionsRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetAudioDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetAudioBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetAudioSampleRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioGenreFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioArtistFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioAlbumFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioChannelsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAudioHasAlbumArtFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetVideoBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetVideoFrameRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetVideoCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoContainerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoAspectRatioFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoHasSubtitlesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoSubtitleLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoHasChaptersFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoIs3DFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetVideoQualityTierFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetContentEncodingFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetLanguageConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]NumericRangeCount, error)
	GetFilesystemTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetMountPointFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSecurityCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSecurityAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetHasACLsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetACLComplexityFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetOwnerTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetGroupCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAccessRelationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetOwnershipPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetAccessFrequencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetTimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSystemFileTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetFileSystemCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSystemAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
	GetSystemFeaturesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)

	// Indexing state
	GetUnindexedFiles(ctx context.Context, workspaceID entity.WorkspaceID, phase string, limit int) ([]*entity.FileEntry, error)
	UpdateIndexedState(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, state entity.IndexedState) error
	UpdateEnhancedMetadata(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, enhanced *entity.EnhancedMetadata) error

	// OS metadata operations
	UpsertSystemUser(ctx context.Context, workspaceID entity.WorkspaceID, user *entity.SystemUser) error
	UpsertFileOwnership(ctx context.Context, workspaceID entity.WorkspaceID, ownership *entity.FileOwnership) error
}

// NumericRangeCount represents a numeric range and its count for faceting.
type NumericRangeCount struct {
	Label string
	Min   float64
	Max   float64
	Count int
}

// DateRangeCount represents a date range and its count for faceting.
type DateRangeCount struct {
	Label string
	Start time.Time
	End   time.Time
	Count int
}

// FileListOptions contains options for listing files.
type FileListOptions struct {
	Offset   int
	Limit    int
	SortBy   string
	SortDesc bool
}

// DefaultFileListOptions returns default list options.
func DefaultFileListOptions() FileListOptions {
	return FileListOptions{
		Offset:   0,
		Limit:    1000,
		SortBy:   "relative_path",
		SortDesc: false,
	}
}

// FileStats contains file index statistics.
type FileStats struct {
	TotalFiles      int
	TotalSize       int64
	ExtensionCounts map[string]int
	FolderCounts    map[string]int
	LastIndexed     *time.Time
}
