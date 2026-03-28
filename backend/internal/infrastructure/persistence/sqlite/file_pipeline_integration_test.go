package sqlite

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/filesystem"
)

func TestScanPipelineAndPersistFiles(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "nested"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	goFile := filepath.Join(workspaceRoot, "nested", "main.go")
	goContents := []byte("package main\n\n// TODO: test\nfunc main() {}\n")
	if err := os.WriteFile(goFile, goContents, 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	txtFile := filepath.Join(workspaceRoot, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatalf("write txt file: %v", err)
	}

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

	workspaceRepo := NewWorkspaceRepository(conn)
	workspace := entity.NewWorkspace(workspaceRoot, "test-workspace")
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	fileRepo := NewFileRepository(conn)
	logger := zerolog.New(io.Discard)
	orchestrator := pipeline.NewOrchestrator(nil, logger)

	scanner := filesystem.NewScanner(workspaceRoot, &workspace.Config)
	entries, err := scanner.Scan(ctx, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	for _, entry := range entries {
		if err := orchestrator.Process(ctx, entry); err != nil {
			t.Fatalf("process %s: %v", entry.RelativePath, err)
		}
		if err := fileRepo.Upsert(ctx, workspace.ID, entry); err != nil {
			t.Fatalf("upsert %s: %v", entry.RelativePath, err)
		}
	}

	opts := repository.DefaultFileListOptions()
	list, err := fileRepo.List(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}

	goEntry, err := fileRepo.GetByPath(ctx, workspace.ID, filepath.Join("nested", "main.go"))
	if err != nil {
		t.Fatalf("get go file: %v", err)
	}
	if goEntry.Enhanced == nil || goEntry.Enhanced.CodeMetrics == nil {
		t.Fatalf("expected code metrics for go file")
	}
	if goEntry.Enhanced.MimeType == nil || goEntry.Enhanced.MimeType.MimeType == "" {
		t.Fatalf("expected mime info for go file")
	}

	txtEntry, err := fileRepo.GetByPath(ctx, workspace.ID, "notes.txt")
	if err != nil {
		t.Fatalf("get txt file: %v", err)
	}
	if txtEntry.Enhanced == nil || txtEntry.Enhanced.MimeType == nil {
		t.Fatalf("expected mime info for txt file")
	}
	if txtEntry.Enhanced.CodeMetrics != nil {
		t.Fatalf("did not expect code metrics for txt file")
	}
}
