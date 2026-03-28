package main

import (
	"context"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockFileIndexer is a mock implementation of service.FileIndexer for testing.
type MockFileIndexer struct {
	mock.Mock
}

func (m *MockFileIndexer) Scan(ctx context.Context, progress chan<- service.ScanProgress) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, progress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileIndexer) ScanFile(ctx context.Context, relativePath string) (*entity.FileEntry, error) {
	args := m.Called(ctx, relativePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileIndexer) GetFileInfo(relativePath string) (service.FileInfo, error) {
	args := m.Called(relativePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(service.FileInfo), args.Error(1)
}

func (m *MockFileIndexer) Exists(relativePath string) bool {
	args := m.Called(relativePath)
	return args.Bool(0)
}

func (m *MockFileIndexer) ReadFile(relativePath string) ([]byte, error) {
	args := m.Called(relativePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileIndexer) ReadFileHead(relativePath string, n int) ([]byte, error) {
	args := m.Called(relativePath, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileIndexer) CollectStats(ctx context.Context) (*service.IndexStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.IndexStats), args.Error(1)
}

func (m *MockFileIndexer) UpdateConfig(config *entity.WorkspaceConfig) {
	m.Called(config)
}

// MockFileRepository is a mock implementation of repository.FileRepository for testing.
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) Create(ctx context.Context, workspaceID entity.WorkspaceID, entry *entity.FileEntry) error {
	args := m.Called(ctx, workspaceID, entry)
	return args.Error(0)
}

func (m *MockFileRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) (*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, relativePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, entry *entity.FileEntry) error {
	args := m.Called(ctx, workspaceID, entry)
	return args.Error(0)
}

func (m *MockFileRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	args := m.Called(ctx, workspaceID, fileID)
	return args.Error(0)
}

func (m *MockFileRepository) List(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	args := m.Called(ctx, workspaceID)
	return args.Int(0), args.Error(1)
}

func (m *MockFileRepository) Search(ctx context.Context, workspaceID entity.WorkspaceID, query string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, query, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, files []*entity.FileEntry) (int, error) {
	args := m.Called(ctx, workspaceID, files)
	return args.Int(0), args.Error(1)
}

func (m *MockFileRepository) BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FileID) (int, error) {
	args := m.Called(ctx, workspaceID, ids)
	return args.Int(0), args.Error(1)
}

func (m *MockFileRepository) ListByExtension(ctx context.Context, workspaceID entity.WorkspaceID, ext string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, ext, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) ListByFolder(ctx context.Context, workspaceID entity.WorkspaceID, folder string, recursive bool, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, folder, recursive, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) ListByDateRange(ctx context.Context, workspaceID entity.WorkspaceID, start, end time.Time, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, start, end, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) ListBySizeRange(ctx context.Context, workspaceID entity.WorkspaceID, minSize, maxSize int64, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, minSize, maxSize, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) ListByContentType(ctx context.Context, workspaceID entity.WorkspaceID, category string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, category, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.FileStats, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.FileStats), args.Error(1)
}

// Stub implementations for all facet methods (not used in these tests, but required by interface)
func (m *MockFileRepository) GetExtensionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return []repository.DateRangeCount{}, nil
}

func (m *MockFileRepository) GetCreatedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return []repository.DateRangeCount{}, nil
}

func (m *MockFileRepository) GetAccessedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return []repository.DateRangeCount{}, nil
}

func (m *MockFileRepository) GetChangedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return []repository.DateRangeCount{}, nil
}

func (m *MockFileRepository) GetOwnerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetIndexingStatusFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetComplexityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetProjectScoreRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetTemporalPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetReadabilityLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetFunctionCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetLinesOfCodeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetCommentPercentageRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetVideoResolutionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetPermissionLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetContentQualityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetImageFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageColorSpaceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetCameraMakeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetCameraModelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageGPSLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageOrientationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageTransparencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageAnimatedFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetImageColorDepthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetImageISORangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetImageApertureRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetImageFocalLengthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetImageDimensionsRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetAudioDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetAudioBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetAudioSampleRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioGenreFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioArtistFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioAlbumFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioChannelsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAudioHasAlbumArtFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetVideoBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetVideoFrameRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetVideoCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoContainerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoAspectRatioFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoHasSubtitlesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoSubtitleLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoHasChaptersFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoIs3DFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetVideoQualityTierFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetUnindexedFiles(ctx context.Context, workspaceID entity.WorkspaceID, phase string, limit int) ([]*entity.FileEntry, error) {
	return []*entity.FileEntry{}, nil
}

func (m *MockFileRepository) UpdateIndexedState(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, state entity.IndexedState) error {
	return nil
}

func (m *MockFileRepository) UpdateEnhancedMetadata(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, enhanced *entity.EnhancedMetadata) error {
	return nil
}

func (m *MockFileRepository) GetMimeTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetMimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetIndexingErrorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetContentEncodingFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetLanguageConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	return []repository.NumericRangeCount{}, nil
}

func (m *MockFileRepository) GetFilesystemTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetMountPointFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSecurityCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSecurityAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetHasACLsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetACLComplexityFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetOwnerTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetGroupCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAccessRelationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetOwnershipPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetAccessFrequencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetTimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSystemFileTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetFileSystemCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSystemAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) GetSystemFeaturesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockFileRepository) UpsertSystemUser(ctx context.Context, workspaceID entity.WorkspaceID, user *entity.SystemUser) error {
	return nil
}

func (m *MockFileRepository) UpsertFileOwnership(ctx context.Context, workspaceID entity.WorkspaceID, ownership *entity.FileOwnership) error {
	return nil
}

// MockWorkspaceRepository is a mock implementation of repository.WorkspaceRepository for testing.
type MockWorkspaceRepository struct {
	mock.Mock
}

func (m *MockWorkspaceRepository) Create(ctx context.Context, workspace *entity.Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.Workspace, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) GetByPath(ctx context.Context, path string) (*entity.Workspace, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) UpdateFileCount(ctx context.Context, workspaceID entity.WorkspaceID, count int) error {
	args := m.Called(ctx, workspaceID, count)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) UpdateLastIndexed(ctx context.Context, workspaceID entity.WorkspaceID) error {
	args := m.Called(ctx, workspaceID)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) List(ctx context.Context, opts repository.WorkspaceListOptions) ([]*entity.Workspace, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) ListActive(ctx context.Context) ([]*entity.Workspace, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockWorkspaceRepository) SetActive(ctx context.Context, id entity.WorkspaceID, active bool) error {
	args := m.Called(ctx, id, active)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Update(ctx context.Context, workspace *entity.Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Delete(ctx context.Context, id entity.WorkspaceID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) UpdateConfig(ctx context.Context, id entity.WorkspaceID, config entity.WorkspaceConfig) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) ClearWorkspaceData(ctx context.Context, id entity.WorkspaceID, workspaceRoot string) error {
	args := m.Called(ctx, id, workspaceRoot)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) ClearFileData(ctx context.Context, id entity.WorkspaceID, workspaceRoot string) error {
	args := m.Called(ctx, id, workspaceRoot)
	return args.Error(0)
}

// MockPipelineProcessor is a mock implementation of pipeline.Processor for testing.
type MockPipelineProcessor struct {
	mock.Mock
}

func (m *MockPipelineProcessor) Process(ctx context.Context, entry *entity.FileEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

// Helper function to create a test file entry
func createTestFileEntry(relativePath string, fileSize int64, lastModified time.Time) *entity.FileEntry {
	return &entity.FileEntry{
		ID:           entity.NewFileID(relativePath),
		RelativePath: relativePath,
		AbsolutePath: "/test/" + relativePath,
		Filename:     relativePath,
		Extension:    ".txt",
		FileSize:     fileSize,
		LastModified: lastModified,
	}
}

// TestInitialScan_ProcessesNewFiles tests that initialScan processes files that don't exist in the database.
func TestInitialScan_ProcessesNewFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 100, now)
	entries := []*entity.FileEntry{testEntry}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)
	// GetByPath returns nil, nil when file doesn't exist (checking err == nil && existing == nil)
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(nil, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockIndexer.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
	mockWorkspaceRepo.AssertExpectations(t)
	mockOrchestrator.AssertExpectations(t)
}

// TestInitialScan_SkipsUnchangedFiles tests that initialScan skips files that haven't changed.
func TestInitialScan_SkipsUnchangedFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - same timestamp and size
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 100, now)
	existingEntry := createTestFileEntry("test.txt", 100, now)
	entries := []*entity.FileEntry{testEntry}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	// Orchestrator.Process should NOT be called
	// Upsert should NOT be called
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockIndexer.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
	mockWorkspaceRepo.AssertExpectations(t)
	mockOrchestrator.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	mockFileRepo.AssertNotCalled(t, "Upsert", ctx, ws.ID, testEntry)
}

// TestInitialScan_ProcessesChangedFiles_SizeChanged tests that initialScan processes files when size changes.
func TestInitialScan_ProcessesChangedFiles_SizeChanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - different size
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 200, now)     // New size: 200
	existingEntry := createTestFileEntry("test.txt", 100, now) // Old size: 100
	entries := []*entity.FileEntry{testEntry}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockOrchestrator.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestInitialScan_ProcessesChangedFiles_TimestampChanged tests that initialScan processes files when timestamp changes significantly.
func TestInitialScan_ProcessesChangedFiles_TimestampChanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - timestamp changed by more than 1 second
	oldTime := time.Now()
	newTime := oldTime.Add(2 * time.Second)
	testEntry := createTestFileEntry("test.txt", 100, newTime)
	existingEntry := createTestFileEntry("test.txt", 100, oldTime)
	entries := []*entity.FileEntry{testEntry}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockOrchestrator.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestInitialScan_SkipsFilesWithSmallTimestampDifference tests that initialScan skips files when timestamp difference is less than 1 second.
func TestInitialScan_SkipsFilesWithSmallTimestampDifference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - timestamp changed by less than 1 second (500ms)
	oldTime := time.Now()
	newTime := oldTime.Add(500 * time.Millisecond)
	testEntry := createTestFileEntry("test.txt", 100, newTime)
	existingEntry := createTestFileEntry("test.txt", 100, oldTime)
	entries := []*entity.FileEntry{testEntry}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	// Orchestrator.Process should NOT be called (tolerance applies)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockOrchestrator.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	mockFileRepo.AssertNotCalled(t, "Upsert", ctx, ws.ID, testEntry)
}

// TestInitialScan_MixedFiles tests that initialScan handles a mix of new, unchanged, and changed files correctly.
func TestInitialScan_MixedFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data
	now := time.Now()
	newFile := createTestFileEntry("new.txt", 100, now)
	unchangedFile := createTestFileEntry("unchanged.txt", 100, now)
	changedFile := createTestFileEntry("changed.txt", 200, now) // Size changed
	existingUnchanged := createTestFileEntry("unchanged.txt", 100, now)
	existingChanged := createTestFileEntry("changed.txt", 100, now) // Old size: 100

	entries := []*entity.FileEntry{newFile, unchangedFile, changedFile}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).Return(entries, nil)

	// new.txt - doesn't exist
	mockFileRepo.On("GetByPath", ctx, ws.ID, "new.txt").Return(nil, nil)
	mockOrchestrator.On("Process", mock.Anything, newFile).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, newFile).Return(nil)

	// unchanged.txt - exists and unchanged
	mockFileRepo.On("GetByPath", ctx, ws.ID, "unchanged.txt").Return(existingUnchanged, nil)
	// Should NOT process

	// changed.txt - exists but changed
	mockFileRepo.On("GetByPath", ctx, ws.ID, "changed.txt").Return(existingChanged, nil)
	mockOrchestrator.On("Process", mock.Anything, changedFile).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, changedFile).Return(nil)

	mockFileRepo.On("Count", ctx, ws.ID).Return(3, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 3).Return(nil)
	mockWorkspaceRepo.On("UpdateLastIndexed", ctx, ws.ID).Return(nil)

	// Execute
	err := initialScan(ctx, ws, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, logger)

	// Assert
	require.NoError(t, err)
	mockOrchestrator.AssertNumberOfCalls(t, "Process", 2) // new.txt and changed.txt
	mockFileRepo.AssertNumberOfCalls(t, "Upsert", 2)      // new.txt and changed.txt
}

// MockFileWatcher is a mock implementation of service.FileWatcher for testing.
type MockFileWatcher struct {
	mock.Mock
	events chan service.WatchEvent
	errors chan error
}

func NewMockFileWatcher() *MockFileWatcher {
	return &MockFileWatcher{
		events: make(chan service.WatchEvent, 10),
		errors: make(chan error, 10),
	}
}

func (m *MockFileWatcher) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockFileWatcher) Stop() error {
	args := m.Called()
	close(m.events)
	close(m.errors)
	return args.Error(0)
}

func (m *MockFileWatcher) Events() <-chan service.WatchEvent {
	return m.events
}

func (m *MockFileWatcher) Errors() <-chan error {
	return m.errors
}

// TestHandleWatchEvents_FileEventCreated_ProcessesNewFile tests that FileEventCreated always processes the file.
func TestHandleWatchEvents_FileEventCreated_ProcessesNewFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 100, now)
	event := service.WatchEvent{
		Type:         entity.FileEventCreated,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    now,
	}

	// Setup expectations
	mockIndexer.On("ScanFile", ctx, "test.txt").Return(testEntry, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockIndexer.AssertExpectations(t)
	mockOrchestrator.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestHandleWatchEvents_FileEventModified_SkipsUnchangedFile tests that FileEventModified skips files that haven't changed.
func TestHandleWatchEvents_FileEventModified_SkipsUnchangedFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - same timestamp and size
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 100, now)
	existingEntry := createTestFileEntry("test.txt", 100, now)
	event := service.WatchEvent{
		Type:         entity.FileEventModified,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    now,
	}

	// Setup expectations
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockIndexer.On("ScanFile", ctx, "test.txt").Return(testEntry, nil)
	// Orchestrator.Process should NOT be called
	// Upsert should NOT be called
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockOrchestrator.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	mockFileRepo.AssertNotCalled(t, "Upsert", ctx, ws.ID, testEntry)
}

// TestHandleWatchEvents_FileEventModified_ProcessesChangedFile tests that FileEventModified processes files that have changed.
func TestHandleWatchEvents_FileEventModified_ProcessesChangedFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - different size
	now := time.Now()
	testEntry := createTestFileEntry("test.txt", 200, now)     // New size: 200
	existingEntry := createTestFileEntry("test.txt", 100, now) // Old size: 100
	event := service.WatchEvent{
		Type:         entity.FileEventModified,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    now,
	}

	// Setup expectations
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockIndexer.On("ScanFile", ctx, "test.txt").Return(testEntry, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockOrchestrator.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestHandleWatchEvents_FileEventModified_SkipsWithSmallTimestampDifference tests that FileEventModified skips files when timestamp difference is within tolerance.
func TestHandleWatchEvents_FileEventModified_SkipsWithSmallTimestampDifference(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - timestamp changed by less than 1 second (500ms)
	oldTime := time.Now()
	newTime := oldTime.Add(500 * time.Millisecond)
	testEntry := createTestFileEntry("test.txt", 100, newTime)
	existingEntry := createTestFileEntry("test.txt", 100, oldTime)
	event := service.WatchEvent{
		Type:         entity.FileEventModified,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    newTime,
	}

	// Setup expectations
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockIndexer.On("ScanFile", ctx, "test.txt").Return(testEntry, nil)
	// Orchestrator.Process should NOT be called (tolerance applies)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockOrchestrator.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	mockFileRepo.AssertNotCalled(t, "Upsert", ctx, ws.ID, testEntry)
}

// TestHandleWatchEvents_FileEventModified_ProcessesWhenTimestampChanged tests that FileEventModified processes files when timestamp changes significantly.
func TestHandleWatchEvents_FileEventModified_ProcessesWhenTimestampChanged(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data - timestamp changed by more than 1 second
	oldTime := time.Now()
	newTime := oldTime.Add(2 * time.Second)
	testEntry := createTestFileEntry("test.txt", 100, newTime)
	existingEntry := createTestFileEntry("test.txt", 100, oldTime)
	event := service.WatchEvent{
		Type:         entity.FileEventModified,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    newTime,
	}

	// Setup expectations
	mockFileRepo.On("GetByPath", ctx, ws.ID, "test.txt").Return(existingEntry, nil)
	mockIndexer.On("ScanFile", ctx, "test.txt").Return(testEntry, nil)
	mockOrchestrator.On("Process", mock.Anything, testEntry).Return(nil)
	mockFileRepo.On("Upsert", ctx, ws.ID, testEntry).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(1, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 1).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockOrchestrator.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestHandleWatchEvents_FileEventDeleted_DeletesFile tests that FileEventDeleted removes the file from the index.
func TestHandleWatchEvents_FileEventDeleted_DeletesFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: true,
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data
	event := service.WatchEvent{
		Type:         entity.FileEventDeleted,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    time.Now(),
	}
	fileID := entity.NewFileID("test.txt")

	// Setup expectations
	mockFileRepo.On("Delete", ctx, ws.ID, fileID).Return(nil)
	mockFileRepo.On("Count", ctx, ws.ID).Return(0, nil)
	mockWorkspaceRepo.On("UpdateFileCount", ctx, ws.ID, 0).Return(nil)

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert
	mockFileRepo.AssertExpectations(t)
}

// TestHandleWatchEvents_AutoIndexDisabled_SkipsProcessing tests that events are skipped when AutoIndex is disabled.
func TestHandleWatchEvents_AutoIndexDisabled_SkipsProcessing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.Nop()
	ws := &entity.Workspace{
		ID:   entity.NewWorkspaceID(),
		Path: "/test",
		Config: entity.WorkspaceConfig{
			AutoIndex: false, // Disabled
		},
	}

	// Create mocks
	mockWatcher := NewMockFileWatcher()
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)
	mockOrchestrator := new(MockPipelineProcessor)

	// Setup test data
	event := service.WatchEvent{
		Type:         entity.FileEventCreated,
		RelativePath: "test.txt",
		AbsolutePath: "/test/test.txt",
		Timestamp:    time.Now(),
	}

	// Send event and close channel
	go func() {
		mockWatcher.events <- event
		close(mockWatcher.events)
		cancel() // Cancel context to exit the loop
	}()

	// Execute
	handleWatchEvents(ctx, mockWatcher, mockIndexer, mockOrchestrator, mockFileRepo, mockWorkspaceRepo, ws, logger)

	// Assert - nothing should be called
	mockIndexer.AssertNotCalled(t, "ScanFile", mock.Anything, mock.Anything)
	mockOrchestrator.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	mockFileRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything, mock.Anything)
}
