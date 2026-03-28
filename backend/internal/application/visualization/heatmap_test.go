package visualization

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestGenerateHeatmap_EmptyWorkspace(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.sqlite"
	conn, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	require.NoError(t, conn.Migrate(ctx))

	workspaceID := entity.NewWorkspaceID()
	docRepo := sqlite.NewDocumentRepository(conn)
	usageRepo := sqlite.NewUsageRepository(conn)

	since := time.Now().AddDate(-1, 0, 0)

	// Generate heatmap for empty workspace
	heatmapData, err := GenerateHeatmap(ctx, workspaceID, docRepo, usageRepo, since, nil)
	require.NoError(t, err)
	assert.NotNil(t, heatmapData)
	assert.Empty(t, heatmapData.Documents)
	assert.Empty(t, heatmapData.Matrix)
}

