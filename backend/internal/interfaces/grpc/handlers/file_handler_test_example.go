// +build ignore

// This file demonstrates how to test FileHandler using mocks of the service interfaces.
// It's marked with build ignore so it doesn't get compiled, but serves as documentation.

package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// MockFileIndexer is a mock implementation of service.FileIndexer for testing.
type MockFileIndexer struct {
	entries      []*entity.FileEntry
	scanError    error
	scanFileFunc func(ctx context.Context, relativePath string) (*entity.FileEntry, error)
}

func (m *MockFileIndexer) Scan(ctx context.Context, progress chan<- service.ScanProgress) ([]*entity.FileEntry, error) {
	if progress != nil {
		progress <- service.ScanProgress{
			FilesScanned: len(m.entries),
			FilesTotal:   len(m.entries),
			Phase:        "complete",
			Completed:    true,
		}
	}
	return m.entries, m.scanError
}

func (m *MockFileIndexer) ScanFile(ctx context.Context, relativePath string) (*entity.FileEntry, error) {
	if m.scanFileFunc != nil {
		return m.scanFileFunc(ctx, relativePath)
	}
	// Default: find entry by path
	for _, entry := range m.entries {
		if entry.RelativePath == relativePath {
			return entry, nil
		}
	}
	return nil, nil
}

func (m *MockFileIndexer) GetFileInfo(relativePath string) (service.FileInfo, error) {
	// Mock implementation
	return nil, nil
}

func (m *MockFileIndexer) Exists(relativePath string) bool {
	for _, entry := range m.entries {
		if entry.RelativePath == relativePath {
			return true
		}
	}
	return false
}

func (m *MockFileIndexer) ReadFile(relativePath string) ([]byte, error) {
	return nil, nil
}

func (m *MockFileIndexer) ReadFileHead(relativePath string, n int) ([]byte, error) {
	return nil, nil
}

func (m *MockFileIndexer) CollectStats(ctx context.Context) (*service.IndexStats, error) {
	return &service.IndexStats{
		TotalFiles: len(m.entries),
	}, nil
}

func (m *MockFileIndexer) UpdateConfig(config *entity.WorkspaceConfig) {}

// MockFileWatcher is a mock implementation of service.FileWatcher for testing.
type MockFileWatcher struct {
	events chan service.WatchEvent
	errors chan error
}

func (m *MockFileWatcher) Start() error {
	return nil
}

func (m *MockFileWatcher) Stop() error {
	close(m.events)
	close(m.errors)
	return nil
}

func (m *MockFileWatcher) Events() <-chan service.WatchEvent {
	return m.events
}

func (m *MockFileWatcher) Errors() <-chan error {
	return m.errors
}

// Example test using mocks
func ExampleFileHandler_WithMocks(t *testing.T) {
	// Create mock indexer with test data
	mockIndexer := &MockFileIndexer{
		entries: []*entity.FileEntry{
			entity.NewFileEntry("/workspace", "test.txt", 100, time.Now()),
		},
	}

	// Create mock watcher
	mockWatcher := &MockFileWatcher{
		events: make(chan service.WatchEvent, 10),
		errors: make(chan error, 10),
	}

	// Create handler with mocks
	handler := NewFileHandler(FileHandlerConfig{
		Indexer:  mockIndexer,
		Watcher:  mockWatcher,
		// ... other config
	})

	// Test scanning
	ctx := context.Background()
	entries, err := handler.scanFiles(ctx, "/workspace", nil, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}






