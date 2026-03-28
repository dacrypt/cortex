package visualization

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestGenerateNodes_EmptyWorkspace(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.sqlite"
	conn, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	require.NoError(t, conn.Migrate(ctx))

	workspaceID := entity.NewWorkspaceID()
	projectRepo := sqlite.NewProjectRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	stateRepo := sqlite.NewDocumentStateRepository(conn)
	usageRepo := sqlite.NewUsageRepository(conn)

	// Generate nodes for empty workspace
	nodes, err := GenerateNodes(ctx, workspaceID, projectRepo, docRepo, stateRepo, usageRepo, nil)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestGenerateNodes_WithProject(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.sqlite"
	conn, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	require.NoError(t, conn.Migrate(ctx))

	workspaceID := entity.NewWorkspaceID()
	projectRepo := sqlite.NewProjectRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	stateRepo := sqlite.NewDocumentStateRepository(conn)
	usageRepo := sqlite.NewUsageRepository(conn)

	// Create a project
	project := entity.NewProject(workspaceID, "Test Project", nil)
	require.NoError(t, projectRepo.Create(ctx, workspaceID, project))

	// Generate nodes
	nodes, err := GenerateNodes(ctx, workspaceID, projectRepo, docRepo, stateRepo, usageRepo, nil)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "project", nodes[0].Type)
	assert.Equal(t, "Test Project", nodes[0].Label)
}

