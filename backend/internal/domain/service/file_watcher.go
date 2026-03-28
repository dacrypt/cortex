// Package service defines domain service interfaces.
package service

import (
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// WatchEvent represents a file system event.
type WatchEvent struct {
	Type         entity.FileEventType
	RelativePath string
	AbsolutePath string
	OldPath      *string // For rename events
	Timestamp    time.Time
}

// FileWatcher defines the interface for file system watching.
// This abstraction allows for different implementations (fsnotify, polling, etc.)
type FileWatcher interface {
	// Start starts watching the workspace.
	Start() error

	// Stop stops the watcher.
	Stop() error

	// Events returns the event channel.
	Events() <-chan WatchEvent

	// Errors returns the error channel.
	Errors() <-chan error
}






