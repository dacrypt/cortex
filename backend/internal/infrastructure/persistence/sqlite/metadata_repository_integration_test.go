package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestMetadataRepositoryFlows(t *testing.T) {
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

	repo := NewMetadataRepository(conn)
	relativePath := "docs/readme.md"
	meta, err := repo.GetOrCreate(ctx, workspace.ID, relativePath, ".md")
	if err != nil {
		t.Fatalf("get or create: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata")
	}

	if err := repo.AddTag(ctx, workspace.ID, meta.FileID, "docs"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := repo.AddContext(ctx, workspace.ID, meta.FileID, "project-alpha"); err != nil {
		t.Fatalf("add context: %v", err)
	}

	tags, err := repo.GetAllTags(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get all tags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "docs" {
		t.Fatalf("unexpected tags: %#v", tags)
	}

	tagCounts, err := repo.GetTagCounts(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get tag counts: %v", err)
	}
	if tagCounts["docs"] != 1 {
		t.Fatalf("unexpected tag count: %#v", tagCounts)
	}

	opts := repository.DefaultFileListOptions()
	listByTag, err := repo.ListByTag(ctx, workspace.ID, "docs", opts)
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(listByTag) != 1 {
		t.Fatalf("expected 1 item for tag, got %d", len(listByTag))
	}

	contexts, err := repo.GetAllContexts(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get all contexts: %v", err)
	}
	if len(contexts) != 1 || contexts[0] != "project-alpha" {
		t.Fatalf("unexpected contexts: %#v", contexts)
	}

	contextCounts, err := repo.GetContextCounts(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get context counts: %v", err)
	}
	if contextCounts["project-alpha"] != 1 {
		t.Fatalf("unexpected context count: %#v", contextCounts)
	}

	listByContext, err := repo.ListByContext(ctx, workspace.ID, "project-alpha", opts)
	if err != nil {
		t.Fatalf("list by context: %v", err)
	}
	if len(listByContext) != 1 {
		t.Fatalf("expected 1 item for context, got %d", len(listByContext))
	}

	if err := repo.AddSuggestedContext(ctx, workspace.ID, meta.FileID, "suggested"); err != nil {
		t.Fatalf("add suggested context: %v", err)
	}
	suggestions, err := repo.GetAllSuggestedContexts(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get suggested contexts: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0] != "suggested" {
		t.Fatalf("unexpected suggestions: %#v", suggestions)
	}

	listBySuggested, err := repo.ListBySuggestedContext(ctx, workspace.ID, "suggested", opts)
	if err != nil {
		t.Fatalf("list by suggested context: %v", err)
	}
	if len(listBySuggested) != 1 {
		t.Fatalf("expected 1 item for suggested context, got %d", len(listBySuggested))
	}

	withSuggestions, err := repo.GetFilesWithSuggestions(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("files with suggestions: %v", err)
	}
	if len(withSuggestions) != 1 {
		t.Fatalf("expected 1 file with suggestions, got %d", len(withSuggestions))
	}

	if err := repo.ClearSuggestedContexts(ctx, workspace.ID, meta.FileID); err != nil {
		t.Fatalf("clear suggestions: %v", err)
	}
	withSuggestions, err = repo.GetFilesWithSuggestions(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("files with suggestions after clear: %v", err)
	}
	if len(withSuggestions) != 0 {
		t.Fatalf("expected no suggestions after clear, got %d", len(withSuggestions))
	}

	note := "note"
	if err := repo.UpdateNotes(ctx, workspace.ID, meta.FileID, note); err != nil {
		t.Fatalf("update notes: %v", err)
	}
	meta, err = repo.Get(ctx, workspace.ID, meta.FileID)
	if err != nil {
		t.Fatalf("get after notes: %v", err)
	}
	if meta.Notes == nil || *meta.Notes != note {
		t.Fatalf("expected notes to persist")
	}
}

func TestMetadataRepositoryFacets(t *testing.T) {
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

	fileRepo := NewFileRepository(conn)
	now := time.Now()
	root := t.TempDir()

	files := []*entity.FileEntry{
		entity.NewFileEntry(root, "docs/event-a.txt", 120, now),
		entity.NewFileEntry(root, "docs/event-b.txt", 220, now),
	}

	for _, file := range files {
		file.Enhanced = &entity.EnhancedMetadata{
			IndexedState: entity.IndexedState{Basic: true},
		}
		if err := fileRepo.Upsert(ctx, workspace.ID, file); err != nil {
			t.Fatalf("upsert %s: %v", file.RelativePath, err)
		}
	}

	eventInsert := `
		INSERT INTO file_events (id, workspace_id, file_id, name, date, location, context, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := conn.Exec(
		ctx,
		eventInsert,
		"event-1",
		workspace.ID.String(),
		files[0].ID.String(),
		"World War II",
		"1939",
		"Europe",
		"context",
		0.9,
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("insert event-1: %v", err)
	}
	if _, err := conn.Exec(
		ctx,
		eventInsert,
		"event-2",
		workspace.ID.String(),
		files[1].ID.String(),
		"World War II",
		"1945",
		"Europe",
		"context",
		0.8,
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("insert event-2: %v", err)
	}
	if _, err := conn.Exec(
		ctx,
		eventInsert,
		"event-3",
		workspace.ID.String(),
		files[0].ID.String(),
		"Cold War",
		"1947",
		"Global",
		"context",
		0.7,
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("insert event-3: %v", err)
	}

	citationInsert := `
		INSERT INTO file_citations (id, workspace_id, file_id, text, authors, title, year, doi, url, type, confidence, page, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := conn.Exec(
		ctx,
		citationInsert,
		"citation-1",
		workspace.ID.String(),
		files[0].ID.String(),
		"Some citation",
		"Author A",
		"Book Title",
		"2000",
		"",
		"",
		"book",
		0.8,
		1,
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("insert citation-1: %v", err)
	}
	if _, err := conn.Exec(
		ctx,
		citationInsert,
		"citation-2",
		workspace.ID.String(),
		files[1].ID.String(),
		"Another citation",
		"Author B",
		"Article Title",
		"2010",
		"",
		"",
		"article",
		0.7,
		2,
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("insert citation-2: %v", err)
	}

	repo := NewMetadataRepository(conn)

	eventCounts, err := repo.GetEventFacet(ctx, workspace.ID, nil)
	if err != nil {
		t.Fatalf("get event facet: %v", err)
	}
	if eventCounts["World War II"] != 2 {
		t.Fatalf("unexpected event count: %#v", eventCounts)
	}
	if eventCounts["Cold War"] != 1 {
		t.Fatalf("unexpected event count: %#v", eventCounts)
	}

	citationCounts, err := repo.GetCitationTypeFacet(ctx, workspace.ID, nil)
	if err != nil {
		t.Fatalf("get citation type facet: %v", err)
	}
	if citationCounts["book"] != 1 || citationCounts["article"] != 1 {
		t.Fatalf("unexpected citation counts: %#v", citationCounts)
	}
}

func TestMetadataRepositoryRelationshipFacet(t *testing.T) {
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

	fileRepo := NewFileRepository(conn)
	now := time.Now()
	root := t.TempDir()

	fileA := entity.NewFileEntry(root, "docs/a.md", 120, now)
	fileA.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
	fileB := entity.NewFileEntry(root, "docs/b.md", 220, now)
	fileB.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
	fileC := entity.NewFileEntry(root, "docs/c.md", 320, now)
	fileC.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}

	for _, file := range []*entity.FileEntry{fileA, fileB, fileC} {
		if err := fileRepo.Upsert(ctx, workspace.ID, file); err != nil {
			t.Fatalf("upsert %s: %v", file.RelativePath, err)
		}
	}

	docInsert := `
		INSERT INTO documents (id, workspace_id, file_id, relative_path, title, checksum, state, state_changed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	nowMillis := time.Now().UnixMilli()
	if _, err := conn.Exec(ctx, docInsert, "doc-a", workspace.ID.String(), fileA.ID.String(), "docs/a.md", "A", "chk-a", "indexed", nowMillis, nowMillis, nowMillis); err != nil {
		t.Fatalf("insert doc-a: %v", err)
	}
	if _, err := conn.Exec(ctx, docInsert, "doc-b", workspace.ID.String(), fileB.ID.String(), "docs/b.md", "B", "chk-b", "indexed", nowMillis, nowMillis, nowMillis); err != nil {
		t.Fatalf("insert doc-b: %v", err)
	}
	if _, err := conn.Exec(ctx, docInsert, "doc-c", workspace.ID.String(), fileC.ID.String(), "docs/c.md", "C", "chk-c", "indexed", nowMillis, nowMillis, nowMillis); err != nil {
		t.Fatalf("insert doc-c: %v", err)
	}

	relInsert := `
		INSERT INTO document_relationships (id, workspace_id, from_document_id, to_document_id, type, strength, metadata, confidence, discovery_method, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := conn.Exec(ctx, relInsert, "rel-1", workspace.ID.String(), "doc-a", "doc-b", "references", 0.5, "{}", 0.8, "explicit", time.Now().UnixMilli()); err != nil {
		t.Fatalf("insert rel-1: %v", err)
	}
	if _, err := conn.Exec(ctx, relInsert, "rel-2", workspace.ID.String(), "doc-b", "doc-c", "related", 0.4, "{}", 0.7, "implicit", time.Now().UnixMilli()); err != nil {
		t.Fatalf("insert rel-2: %v", err)
	}
	if _, err := conn.Exec(ctx, relInsert, "rel-3", workspace.ID.String(), "doc-a", "doc-c", "references", 0.6, "{}", 0.9, "explicit", time.Now().UnixMilli()); err != nil {
		t.Fatalf("insert rel-3: %v", err)
	}

	repo := NewMetadataRepository(conn)
	counts, err := repo.GetRelationshipTypeFacet(ctx, workspace.ID, nil)
	if err != nil {
		t.Fatalf("get relationship type facet: %v", err)
	}
	if counts["references"] != 2 || counts["related"] != 1 {
		t.Fatalf("unexpected relationship type counts: %#v", counts)
	}
}
