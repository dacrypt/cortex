package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestWorkspaceRepositoryCRUD(t *testing.T) {
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

	repo := NewWorkspaceRepository(conn)
	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	workspace.FileCount = 5

	if err := repo.Create(ctx, workspace); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.ID != workspace.ID {
		t.Fatalf("unexpected workspace: %#v", got)
	}
	if got.FileCount != 5 {
		t.Fatalf("file count mismatch: %d", got.FileCount)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 workspace, got %d", count)
	}

	opts := repository.DefaultWorkspaceListOptions()
	list, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 workspace in list, got %d", len(list))
	}

	if err := repo.SetActive(ctx, workspace.ID, false); err != nil {
		t.Fatalf("set active: %v", err)
	}
	got, err = repo.Get(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get after set active: %v", err)
	}
	if got.Active {
		t.Fatalf("expected inactive workspace")
	}

	if err := repo.UpdateFileCount(ctx, workspace.ID, 12); err != nil {
		t.Fatalf("update file count: %v", err)
	}
	got, err = repo.Get(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get after file count: %v", err)
	}
	if got.FileCount != 12 {
		t.Fatalf("file count mismatch after update: %d", got.FileCount)
	}

	config := entity.DefaultWorkspaceConfig()
	config.AutoIndex = false
	config.CustomSettings["mode"] = "test"
	if err := repo.UpdateConfig(ctx, workspace.ID, config); err != nil {
		t.Fatalf("update config: %v", err)
	}
	got, err = repo.Get(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get after config: %v", err)
	}
	if got.Config.AutoIndex {
		t.Fatalf("expected AutoIndex=false after config update")
	}
	if got.Config.CustomSettings["mode"] != "test" {
		t.Fatalf("expected custom setting to persist")
	}

	if err := repo.UpdateLastIndexed(ctx, workspace.ID); err != nil {
		t.Fatalf("update last indexed: %v", err)
	}
	got, err = repo.Get(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get after last indexed: %v", err)
	}
	if got.LastIndexed == nil || time.Since(*got.LastIndexed) > time.Minute {
		t.Fatalf("last indexed not updated")
	}

	if err := repo.Delete(ctx, workspace.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	count, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 workspaces after delete, got %d", count)
	}
}
