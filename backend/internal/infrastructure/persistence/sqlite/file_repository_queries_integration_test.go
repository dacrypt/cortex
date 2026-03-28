package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestFileRepositoryQueries(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFileRepository(conn)
	now := time.Now()
	root := t.TempDir()

	files := []*entity.FileEntry{
		entity.NewFileEntry(root, "docs/readme.md", 120, now),
		entity.NewFileEntry(root, "docs/guide/intro.md", 220, now),
		entity.NewFileEntry(root, "src/main.go", 320, now),
		entity.NewFileEntry(root, "src/utils/helper.go", 420, now),
		entity.NewFileEntry(root, "notes.txt", 20, now),
	}

	for _, file := range files {
		file.Enhanced = &entity.EnhancedMetadata{
			IndexedState: entity.IndexedState{Basic: true},
		}
		if file.Extension == ".go" {
			file.Enhanced.IndexedState.Mime = true
		}
		if err := repo.Upsert(ctx, workspace.ID, file); err != nil {
			t.Fatalf("upsert %s: %v", file.RelativePath, err)
		}
	}

	opts := repository.DefaultFileListOptions()
	goFiles, err := repo.ListByExtension(ctx, workspace.ID, ".go", opts)
	if err != nil {
		t.Fatalf("list by extension: %v", err)
	}
	if len(goFiles) != 2 {
		t.Fatalf("expected 2 .go files, got %d", len(goFiles))
	}

	docsFlat, err := repo.ListByFolder(ctx, workspace.ID, "docs", false, opts)
	if err != nil {
		t.Fatalf("list by folder flat: %v", err)
	}
	if len(docsFlat) != 1 || docsFlat[0].RelativePath != "docs/readme.md" {
		t.Fatalf("unexpected flat docs list: %#v", docsFlat)
	}

	docsRecursive, err := repo.ListByFolder(ctx, workspace.ID, "docs", true, opts)
	if err != nil {
		t.Fatalf("list by folder recursive: %v", err)
	}
	if len(docsRecursive) != 2 {
		t.Fatalf("expected 2 docs files, got %d", len(docsRecursive))
	}

	searchResults, err := repo.Search(ctx, workspace.ID, "intro", opts)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(searchResults) != 1 || searchResults[0].RelativePath != "docs/guide/intro.md" {
		t.Fatalf("unexpected search results: %#v", searchResults)
	}

	unindexed, err := repo.GetUnindexedFiles(ctx, workspace.ID, "mime", 10)
	if err != nil {
		t.Fatalf("get unindexed: %v", err)
	}
	if len(unindexed) != 3 {
		t.Fatalf("expected 3 mime-unindexed files, got %d", len(unindexed))
	}

	if err := repo.UpdateIndexedState(ctx, workspace.ID, files[0].ID, entity.IndexedState{Basic: true, Mime: true}); err != nil {
		t.Fatalf("update indexed state: %v", err)
	}
	unindexed, err = repo.GetUnindexedFiles(ctx, workspace.ID, "mime", 10)
	if err != nil {
		t.Fatalf("get unindexed after update: %v", err)
	}
	if len(unindexed) != 2 {
		t.Fatalf("expected 2 mime-unindexed files after update, got %d", len(unindexed))
	}
}
