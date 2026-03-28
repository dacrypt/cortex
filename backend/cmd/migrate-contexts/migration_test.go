package main

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestMigrateContextsToProjects(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.sqlite"
	conn, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	require.NoError(t, conn.Migrate(ctx))

	workspaceID := entity.NewWorkspaceID()
	metaRepo := sqlite.NewMetadataRepository(conn)
	projectRepo := sqlite.NewProjectRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)

	// Create a test file and document
	fileID := entity.NewFileID("test.md")
	_, err = metaRepo.GetOrCreate(ctx, workspaceID, "test.md", ".md")
	require.NoError(t, err)

	// Add a context
	err = metaRepo.AddContext(ctx, workspaceID, fileID, "Test Project")
	require.NoError(t, err)

	// Create a document for the file
	docID := entity.NewDocumentID("test.md")
	doc := &entity.Document{
		ID:           docID,
		FileID:       fileID,
		RelativePath: "test.md",
		Title:        "Test Document",
		Checksum:     "test-checksum",
	}
	require.NoError(t, docRepo.UpsertDocument(ctx, workspaceID, doc))

	// Run migration
	logger := zerolog.New(io.Discard)
	err = migrateContextsToProjects(ctx, conn, workspaceID, false, logger)
	require.NoError(t, err)

	// Verify project was created
	project, err := projectRepo.GetByName(ctx, workspaceID, "Test Project", nil)
	require.NoError(t, err)
	assert.NotNil(t, project)
	assert.Equal(t, "Test Project", project.Name)

	// Verify document is associated with project
	docIDs, err := projectRepo.GetDocuments(ctx, workspaceID, project.ID, false)
	require.NoError(t, err)
	assert.Contains(t, docIDs, docID)
}

