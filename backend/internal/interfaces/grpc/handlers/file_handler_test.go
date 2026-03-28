package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *MockFileRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, relativePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, entry *entity.FileEntry) error {
	// Upsert: insert or update operation
	args := m.Called(ctx, workspaceID, entry)
	_ = entry // distinguish from Create
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

func (m *MockFileRepository) ListByExtension(ctx context.Context, workspaceID entity.WorkspaceID, extension string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, extension, opts)
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

func (m *MockFileRepository) GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) (*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, files []*entity.FileEntry) (int, error) {
	args := m.Called(ctx, workspaceID, files)
	return args.Int(0), args.Error(1)
}

func (m *MockFileRepository) BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FileID) (int, error) {
	args := m.Called(ctx, workspaceID, ids)
	return args.Int(0), args.Error(1)
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

func (m *MockFileRepository) GetExtensionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	// GetTypeFacet: returns type facet (not extension)
	args := m.Called(ctx, workspaceID, fileIDs)
	_ = len(fileIDs) // distinguish from GetExtensionFacet
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.DateRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetCreatedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	// GetCreatedDateRangeFacet: returns created date facet (not modified)
	args := m.Called(ctx, workspaceID, fileIDs)
	_ = len(fileIDs) // distinguish from GetDateRangeFacet
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.DateRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetAccessedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.DateRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetChangedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.DateRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetOwnerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetIndexingStatusFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetIndexingErrorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetMimeTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetMimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetComplexityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetProjectScoreRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetTemporalPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetReadabilityLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetFunctionCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetLinesOfCodeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetCommentPercentageRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetVideoResolutionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetPermissionLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetContentQualityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetImageFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageColorSpaceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetCameraMakeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetCameraModelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageGPSLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageOrientationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageTransparencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageAnimatedFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetImageColorDepthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetImageISORangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetImageApertureRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetImageFocalLengthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetImageDimensionsRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetAudioDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetAudioBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetAudioSampleRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioGenreFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioArtistFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioAlbumFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioChannelsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAudioHasAlbumArtFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetVideoBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetVideoFrameRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetVideoCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoContainerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoAspectRatioFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoHasSubtitlesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoSubtitleLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoHasChaptersFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoIs3DFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetVideoQualityTierFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetContentEncodingFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetLanguageConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.NumericRangeCount), args.Error(1)
}

func (m *MockFileRepository) GetFilesystemTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetMountPointFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSecurityCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSecurityAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetHasACLsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetACLComplexityFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetOwnerTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetGroupCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAccessRelationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetOwnershipPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetAccessFrequencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetTimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSystemFileTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetFileSystemCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSystemAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetSystemFeaturesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	args := m.Called(ctx, workspaceID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockFileRepository) GetUnindexedFiles(ctx context.Context, workspaceID entity.WorkspaceID, phase string, limit int) ([]*entity.FileEntry, error) {
	args := m.Called(ctx, workspaceID, phase, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

func (m *MockFileRepository) UpdateIndexedState(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, state entity.IndexedState) error {
	args := m.Called(ctx, workspaceID, id, state)
	return args.Error(0)
}

func (m *MockFileRepository) UpdateEnhancedMetadata(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, enhanced *entity.EnhancedMetadata) error {
	args := m.Called(ctx, workspaceID, id, enhanced)
	return args.Error(0)
}

func (m *MockFileRepository) UpsertSystemUser(ctx context.Context, workspaceID entity.WorkspaceID, user *entity.SystemUser) error {
	args := m.Called(ctx, workspaceID, user)
	return args.Error(0)
}

func (m *MockFileRepository) UpsertFileOwnership(ctx context.Context, workspaceID entity.WorkspaceID, ownership *entity.FileOwnership) error {
	args := m.Called(ctx, workspaceID, ownership)
	return args.Error(0)
}

// TestFileHandlerScanWorkspace tests the ScanWorkspace method with a mock indexer.
func TestFileHandlerScanWorkspace(t *testing.T) {
	ctx := context.Background()
	workspaceID := "test-workspace"
	workspacePath := "/test/workspace"

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)

	// Setup test data
	testEntries := []*entity.FileEntry{
		entity.NewFileEntry(workspacePath, "file1.txt", 100, time.Now()),
		entity.NewFileEntry(workspacePath, "file2.txt", 200, time.Now()),
	}
	testWorkspace := &entity.Workspace{
		ID:     entity.WorkspaceID(workspaceID),
		Path:   workspacePath,
		Config: entity.DefaultWorkspaceConfig(),
	}

	// Setup expectations
	mockIndexer.On("Scan", ctx, mock.Anything).
		Return(testEntries, nil)
	mockWorkspaceRepo.On("Get", ctx, entity.WorkspaceID(workspaceID)).
		Return(testWorkspace, nil)

	mockFileRepo.On("Upsert", ctx, entity.WorkspaceID(workspaceID), mock.AnythingOfType("*entity.FileEntry")).
		Return(nil).Times(len(testEntries))

	// Create handler
	handler := NewFileHandler(FileHandlerConfig{
		Indexer:       mockIndexer,
		FileRepo:      mockFileRepo,
		WorkspaceRepo: mockWorkspaceRepo,
		Pipeline:      pipeline.NewOrchestrator(nil, zerolog.Nop()),
		Logger:        zerolog.Nop(),
	})

	// Execute
	err := handler.ScanWorkspace(ctx, workspaceID, workspacePath, false, nil)

	// Assert
	assert.NoError(t, err)
	mockIndexer.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// TestFileHandlerProcessFile tests the ProcessFile method with a mock indexer.
func TestFileHandlerProcessFile(t *testing.T) {
	ctx := context.Background()
	workspaceID := "test-workspace"
	relativePath := "test.txt"

	// Create mocks
	mockIndexer := new(MockFileIndexer)
	mockFileRepo := new(MockFileRepository)
	mockWorkspaceRepo := new(MockWorkspaceRepository)

	// Setup test data
	testEntry := entity.NewFileEntry("/workspace", relativePath, 100, time.Now())
	testWorkspace := &entity.Workspace{
		ID:     entity.WorkspaceID(workspaceID),
		Path:   "/workspace",
		Config: entity.DefaultWorkspaceConfig(),
	}

	// Setup expectations
	mockWorkspaceRepo.On("Get", ctx, entity.WorkspaceID(workspaceID)).
		Return(testWorkspace, nil)

	mockIndexer.On("ScanFile", ctx, relativePath).
		Return(testEntry, nil)

	mockFileRepo.On("Upsert", ctx, entity.WorkspaceID(workspaceID), testEntry).
		Return(nil)

	// Create handler
	handler := NewFileHandler(FileHandlerConfig{
		Indexer:       mockIndexer,
		FileRepo:      mockFileRepo,
		WorkspaceRepo: mockWorkspaceRepo,
		Pipeline:      pipeline.NewOrchestrator(nil, zerolog.Nop()),
		Logger:        zerolog.Nop(),
	})

	// Execute
	entry, err := handler.ProcessFile(ctx, workspaceID, relativePath)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, relativePath, entry.RelativePath)
	mockIndexer.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
	mockWorkspaceRepo.AssertExpectations(t)
}

// MockWorkspaceRepository is a minimal mock for testing.
type MockWorkspaceRepository struct {
	mock.Mock
}

func (m *MockWorkspaceRepository) Get(ctx context.Context, id entity.WorkspaceID) (*entity.Workspace, error) {
	args := m.Called(ctx, id)
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

// Add other required methods as needed for compilation
func (m *MockWorkspaceRepository) Create(ctx context.Context, ws *entity.Workspace) error {
	args := m.Called(ctx, ws)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Update(ctx context.Context, ws *entity.Workspace) error {
	// Update: modify existing workspace
	args := m.Called(ctx, ws)
	_ = ws // distinguish from Create
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Delete(ctx context.Context, id entity.WorkspaceID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) List(ctx context.Context, opts repository.WorkspaceListOptions) ([]*entity.Workspace, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) UpdateFileCount(ctx context.Context, id entity.WorkspaceID, count int) error {
	args := m.Called(ctx, id, count)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) UpdateLastIndexed(ctx context.Context, id entity.WorkspaceID) error {
	// UpdateLastIndexed: updates timestamp only
	args := m.Called(ctx, id)
	_ = id // distinguish from Delete
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

func (m *MockWorkspaceRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockWorkspaceRepository) ListActive(ctx context.Context) ([]*entity.Workspace, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) SetActive(ctx context.Context, id entity.WorkspaceID, active bool) error {
	args := m.Called(ctx, id, active)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) UpdateConfig(ctx context.Context, id entity.WorkspaceID, config entity.WorkspaceConfig) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}
