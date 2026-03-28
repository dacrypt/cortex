// Package filesystem provides factory functions for creating file system services.
package filesystem

import (
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// NewFileIndexer creates a new FileIndexer implementation.
// This factory function provides a clean way to create indexers
// and makes it easy to swap implementations in the future.
func NewFileIndexer(workspaceRoot string, config *entity.WorkspaceConfig) service.FileIndexer {
	return NewScanner(workspaceRoot, config)
}

// NewFileWatcher creates a new FileWatcher implementation.
// This factory function provides a clean way to create watchers
// and makes it easy to swap implementations in the future.
func NewFileWatcher(workspaceRoot string, config *entity.WorkspaceConfig) (service.FileWatcher, error) {
	return NewWatcher(workspaceRoot, config)
}

// CreateIndexerAndWatcher creates both an indexer and watcher for a workspace.
// This is a convenience function for common use cases.
func CreateIndexerAndWatcher(workspaceRoot string, config *entity.WorkspaceConfig) (service.FileIndexer, service.FileWatcher, error) {
	indexer := NewFileIndexer(workspaceRoot, config)
	watcher, err := NewFileWatcher(workspaceRoot, config)
	if err != nil {
		return nil, nil, err
	}
	return indexer, watcher, nil
}






