// Package service defines domain service interfaces.
package service

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ScanProgress reports scanning progress.
type ScanProgress struct {
	FilesScanned int
	FilesTotal   int
	CurrentPath  string
	Percentage   float64
	Phase        string
	Completed    bool
	Error        *string
}

// FileIndexer defines the interface for file indexing operations.
// This abstraction allows for different implementations (local filesystem, remote, etc.)
type FileIndexer interface {
	// Scan scans the workspace and returns all files.
	// Progress updates are sent through the progress channel if provided.
	Scan(ctx context.Context, progress chan<- ScanProgress) ([]*entity.FileEntry, error)

	// ScanFile scans a single file and returns its entry.
	ScanFile(ctx context.Context, relativePath string) (*entity.FileEntry, error)

	// GetFileInfo gets file info without creating an entry.
	GetFileInfo(relativePath string) (FileInfo, error)

	// Exists checks if a file exists.
	Exists(relativePath string) bool

	// ReadFile reads the content of a file.
	ReadFile(relativePath string) ([]byte, error)

	// ReadFileHead reads the first n bytes of a file.
	ReadFileHead(relativePath string, n int) ([]byte, error)

	// CollectStats collects statistics about the workspace.
	CollectStats(ctx context.Context) (*IndexStats, error)

	// UpdateConfig updates the indexer configuration.
	UpdateConfig(config *entity.WorkspaceConfig)
}

// FileInfo contains basic file information.
type FileInfo interface {
	// Name returns the base name of the file.
	Name() string

	// Size returns the length in bytes.
	Size() int64

	// Mode returns the file mode bits.
	Mode() uint32

	// ModTime returns the modification time.
	ModTime() time.Time

	// IsDir returns true if the file is a directory.
	IsDir() bool
}

// IndexStats contains workspace statistics.
type IndexStats struct {
	TotalFiles      int
	TotalSize       int64
	ExtensionCounts map[string]int
	FolderCounts    map[string]int
	SizeBuckets     map[string]int
	MostRecent      *time.Time
}

