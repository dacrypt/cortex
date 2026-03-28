package filesystem

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

func TestWatcherEmitsCreateEvent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	config := entity.DefaultWorkspaceConfig()
	config.ExcludedPaths = []string{}
	config.ExcludedExtensions = []string{}

	watcher, err := NewWatcher(root, &config)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("start watcher: %v", err)
	}

	target := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case evt := <-watcher.Events():
			if evt.RelativePath == "hello.txt" &&
				(evt.Type == entity.FileEventCreated || evt.Type == entity.FileEventModified) {
				return
			}
		case err := <-watcher.Errors():
			t.Fatalf("watcher error: %v", err)
		case <-timer.C:
			t.Fatalf("timed out waiting for create event")
		}
	}
}
