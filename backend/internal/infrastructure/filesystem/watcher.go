package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// WatchEvent represents a file system event.
// This is an alias for service.WatchEvent to maintain backward compatibility.
type WatchEvent = service.WatchEvent

// Watcher watches a directory for file changes.
type Watcher struct {
	workspaceRoot string
	config        *entity.WorkspaceConfig
	watcher       *fsnotify.Watcher
	events        chan WatchEvent
	errors        chan error
	debounceTime  time.Duration
	pending       map[string]*pendingEvent
	pendingMu     sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

type pendingEvent struct {
	event WatchEvent
	timer *time.Timer
}

// NewWatcher creates a new file watcher.
func NewWatcher(workspaceRoot string, config *entity.WorkspaceConfig) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cfg := config
	if cfg == nil {
		defaultCfg := entity.DefaultWorkspaceConfig()
		cfg = &defaultCfg
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		workspaceRoot: workspaceRoot,
		config:        cfg,
		watcher:       fsWatcher,
		events:        make(chan WatchEvent, 100),
		errors:        make(chan error, 10),
		debounceTime:  500 * time.Millisecond, // Increased from 100ms to 500ms to reduce rapid reindexing
		pending:       make(map[string]*pendingEvent),
		ctx:           ctx,
		cancel:        cancel,
	}

	return w, nil
}

// Ensure Watcher implements service.FileWatcher interface.
var _ service.FileWatcher = (*Watcher)(nil)

// Start starts watching the workspace.
func (w *Watcher) Start() error {
	// Add root directory
	if err := w.addDirRecursive(w.workspaceRoot); err != nil {
		return err
	}

	// Start event processing
	w.wg.Add(1)
	go w.processEvents()

	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	w.cancel()
	w.wg.Wait()

	close(w.events)
	close(w.errors)

	return w.watcher.Close()
}

// Events returns the event channel.
func (w *Watcher) Events() <-chan WatchEvent {
	return w.events
}

// Errors returns the error channel.
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// addDirRecursive adds a directory and all subdirectories to watch.
func (w *Watcher) addDirRecursive(path string) error {
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, _ := filepath.Rel(w.workspaceRoot, p)

		// Skip excluded directories
		if relPath != "." && w.config.ShouldExcludePath(relPath) {
			return filepath.SkipDir
		}

		return w.watcher.Add(p)
	})
}

func (w *Watcher) processEvents() {
	defer w.wg.Done()

	var renameOldPath string

	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Get relative path
			relPath, err := filepath.Rel(w.workspaceRoot, event.Name)
			if err != nil {
				continue
			}

			// Skip excluded paths
			if w.config.ShouldExcludePath(relPath) {
				continue
			}

			// Skip excluded extensions
			ext := strings.ToLower(filepath.Ext(event.Name))
			if w.config.ShouldExcludeExtension(ext) {
				continue
			}

			// Determine event type
			var eventType entity.FileEventType
			var oldPath *string

			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				eventType = entity.FileEventCreated

				// If it's a directory, add it to watch
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addDirRecursive(event.Name)
					continue // Don't emit event for directories
				}

			case event.Op&fsnotify.Write == fsnotify.Write:
				eventType = entity.FileEventModified

			case event.Op&fsnotify.Remove == fsnotify.Remove:
				eventType = entity.FileEventDeleted

			case event.Op&fsnotify.Rename == fsnotify.Rename:
				// fsnotify sends rename as two events: old path then new path
				if renameOldPath == "" {
					renameOldPath = relPath
					continue
				}
				eventType = entity.FileEventRenamed
				oldPath = &renameOldPath
				renameOldPath = ""

			case event.Op&fsnotify.Chmod == fsnotify.Chmod:
				// Ignore chmod events
				continue

			default:
				continue
			}

			// Create and debounce the event
			w.debounceEvent(WatchEvent{
				Type:         eventType,
				RelativePath: relPath,
				AbsolutePath: event.Name,
				OldPath:      oldPath,
				Timestamp:    time.Now(),
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
				// Error channel full, drop error
			}
		}
	}
}

func (w *Watcher) debounceEvent(event WatchEvent) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	key := event.RelativePath

	// Cancel existing pending event for this path
	if existing, ok := w.pending[key]; ok {
		existing.timer.Stop()
		delete(w.pending, key)
	}

	// For delete events, emit immediately
	if event.Type == entity.FileEventDeleted {
		select {
		case w.events <- event:
		default:
			// Event channel full
		}
		return
	}

	// Create timer for debounced emit
	timer := time.AfterFunc(w.debounceTime, func() {
		w.pendingMu.Lock()
		if pending, ok := w.pending[key]; ok {
			delete(w.pending, key)
			w.pendingMu.Unlock()

			select {
			case w.events <- pending.event:
			default:
				// Event channel full
			}
		} else {
			w.pendingMu.Unlock()
		}
	})

	w.pending[key] = &pendingEvent{
		event: event,
		timer: timer,
	}
}
