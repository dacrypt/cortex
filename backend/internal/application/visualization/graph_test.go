package visualization

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestGenerateGraph_EmptyWorkspace(t *testing.T) {
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
	relRepo := sqlite.NewRelationshipRepository(conn)

	// Generate graph for empty workspace
	graphData, err := GenerateGraph(ctx, workspaceID, projectRepo, docRepo, relRepo, nil, true, true)
	require.NoError(t, err)
	assert.NotNil(t, graphData)
	assert.Empty(t, graphData.Nodes)
	assert.Empty(t, graphData.Edges)
}

func TestGenerateGraph_WithProjects(t *testing.T) {
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
	relRepo := sqlite.NewRelationshipRepository(conn)

	// Create a project
	project := entity.NewProject(workspaceID, "Test Project", nil)
	require.NoError(t, projectRepo.Create(ctx, workspaceID, project))

	// Generate graph
	graphData, err := GenerateGraph(ctx, workspaceID, projectRepo, docRepo, relRepo, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, graphData)
	assert.Len(t, graphData.Nodes, 1)
	assert.Equal(t, "project", graphData.Nodes[0].Type)
	assert.Equal(t, "Test Project", graphData.Nodes[0].Label)
}

