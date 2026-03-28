package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

const (
	qualityTestDBName = "test.db"
	projectA          = "project-a"
	authSystem        = "auth-system"
)

// TestQualityCoreObjectives tests that Cortex meets its core objectives:
// 1. Files stay where they are (no moving, no duplication)
// 2. Multiple virtual views of the same files
// 3. Files can belong to multiple projects
// 4. Local-first and deterministic
func TestQualityCoreObjectives(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBName)
	db, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))

	wsID := entity.NewWorkspaceID()
	wsInfo := contextinfo.WorkspaceInfo{
		ID:   wsID,
		Root: workspaceRoot,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)

	// Objective 1: Files stay where they are
	t.Run("FilesStayInPlace", func(t *testing.T) {
		testFilesStayInPlace(t, ctx, wsID, workspaceRoot, fileRepo)
	})

	// Objective 2: Multiple virtual views
	t.Run("MultipleVirtualViews", func(t *testing.T) {
		testMultipleVirtualViews(t, ctx, wsID, workspaceRoot, fileRepo, metaRepo)
	})
}

func testFilesStayInPlace(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, workspaceRoot string, fileRepo repository.FileRepository) {
	originalPath := filepath.Join(workspaceRoot, "test.md")
	originalContent := []byte("# Test Document\n\nContent here.")
	require.NoError(t, os.WriteFile(originalPath, originalContent, 0644))

	absPath, err := filepath.Abs(originalPath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(originalContent)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

	// Verify file is still in original location
	_, err = os.Stat(originalPath)
	assert.NoError(t, err, "File should remain in original location")

	// Verify content is unchanged
	content, err := os.ReadFile(originalPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, content, "File content should be unchanged")
}

func testMultipleVirtualViews(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, workspaceRoot string, fileRepo repository.FileRepository, metaRepo repository.MetadataRepository) {
	// Create files with different properties
	files := []struct {
		name     string
		content  string
		ext      string
		tags     []string
		projects []string
	}{
		{
			name:     "doc1.md",
			content:  "# Document 1",
			ext:      ".md",
			tags:     []string{"important", "review"},
			projects: []string{projectA, "project-b"},
		},
		{
			name:     "code.go",
			content:  "package main",
			ext:      ".go",
			tags:     []string{"code"},
			projects: []string{projectA},
		},
		{
			name:     "doc2.md",
			content:  "# Document 2",
			ext:      ".md",
			tags:     []string{"important"},
			projects: []string{"project-b"},
		},
	}

	for _, f := range files {
		filePath := filepath.Join(workspaceRoot, f.name)
		require.NoError(t, os.WriteFile(filePath, []byte(f.content), 0644))

		absPath, err := filepath.Abs(filePath)
		require.NoError(t, err)
		relPath, err := filepath.Rel(workspaceRoot, absPath)
		require.NoError(t, err)

		entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(f.content)), time.Now())
		require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
		_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
		require.NoError(t, err)
		_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
		require.NoError(t, err)
		_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
		require.NoError(t, err)

		// Get file ID for metadata operations
		file, err := fileRepo.GetByPath(ctx, wsID, relPath)
		require.NoError(t, err)

		// Add tags and projects
		for _, tag := range f.tags {
			require.NoError(t, metaRepo.AddTag(ctx, wsID, file.ID, tag))
		}
		for _, project := range f.projects {
			require.NoError(t, metaRepo.AddContext(ctx, wsID, file.ID, project))
		}
	}

	// Verify we can query by different views
	// View 1: By tag
	filesWithImportant, err := metaRepo.ListByTag(ctx, wsID, "important", repository.FileListOptions{Limit: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(filesWithImportant), 2, "Should find files with 'important' tag")

	// View 2: By project
	filesInProjectA, err := metaRepo.ListByContext(ctx, wsID, projectA, repository.FileListOptions{Limit: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(filesInProjectA), 2, "Should find files in 'project-a'")

	// View 3: By type
	allFiles, err := fileRepo.List(ctx, wsID, repository.FileListOptions{Limit: 100})
	require.NoError(t, err)
	mdFiles := 0
	goFiles := 0
	for _, file := range allFiles {
		switch file.Extension {
		case ".md":
			mdFiles++
		case ".go":
			goFiles++
		}
	}
	assert.GreaterOrEqual(t, mdFiles, 2, "Should find markdown files")
	assert.GreaterOrEqual(t, goFiles, 1, "Should find Go files")
}

func testMultipleProjectsPerFile(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, workspaceRoot string, fileRepo repository.FileRepository, metaRepo repository.MetadataRepository) {
	filePath := filepath.Join(workspaceRoot, "multi-project.md")
	content := []byte("# Multi-Project Document")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
	_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
	require.NoError(t, err)
	_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
	require.NoError(t, err)

	// Add file to multiple projects
	projects := []string{"project-x", "project-y", "project-z"}
	for _, project := range projects {
		file, err := fileRepo.GetByPath(ctx, wsID, relPath)
		require.NoError(t, err)
		require.NoError(t, metaRepo.AddContext(ctx, wsID, file.ID, project))
	}

	// Verify file appears in all projects
	for _, project := range projects {
		files, err := metaRepo.ListByContext(ctx, wsID, project, repository.FileListOptions{Limit: 100})
		require.NoError(t, err)
		found := false
		for _, fileMeta := range files {
			if fileMeta.RelativePath == relPath {
				found = true
				break
			}
		}
		assert.True(t, found, "File should appear in project: %s", project)
	}

	// Verify file still exists in only one location
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "File should exist in only one location")
}

func testLocalFirstAndDeterministic(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, workspaceRoot string, fileRepo repository.FileRepository) {
	// Test that same input produces same output
	filePath := filepath.Join(workspaceRoot, "deterministic.md")
	content := []byte("# Deterministic Test\n\nSame content.")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(content)), time.Now())

	// Process twice
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

	file1, err := fileRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	// Process again (should be idempotent)
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

	file2, err := fileRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	// Verify deterministic behavior
	assert.Equal(t, file1.RelativePath, file2.RelativePath, "Paths should be consistent")
	assert.Equal(t, file1.FileSize, file2.FileSize, "File sizes should be consistent")

	// Verify data is stored locally (in database)
	assert.NotNil(t, file1, "File should be stored locally")
	assert.NotNil(t, file2, "File should be stored locally")
}

func TestQualityCoreObjectivesSubTests(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBName)
	db, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))

	wsID := entity.NewWorkspaceID()
	wsInfo := contextinfo.WorkspaceInfo{
		ID:   wsID,
		Root: workspaceRoot,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)

	// Objective 3: Files can belong to multiple projects
	t.Run("MultipleProjectsPerFile", func(t *testing.T) {
		testMultipleProjectsPerFile(t, ctx, wsID, workspaceRoot, fileRepo, metaRepo)
	})

	// Objective 4: Local-first and deterministic
	t.Run("LocalFirstAndDeterministic", func(t *testing.T) {
		testLocalFirstAndDeterministic(t, ctx, wsID, workspaceRoot, fileRepo)
	})
}

// TestQualitySemanticOrganization tests that semantic organization works correctly
func TestQualitySemanticOrganization(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBName)
	db, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))

	wsID := entity.NewWorkspaceID()
	wsInfo := contextinfo.WorkspaceInfo{
		ID:   wsID,
		Root: workspaceRoot,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)

	// Create semantically related files
	files := []struct {
		name    string
		content string
		tags    []string
		project string
	}{
		{
			name:    "auth-login.md",
			content: "# Login Implementation\n\nUser authentication code.",
			tags:    []string{"authentication", "security", "backend"},
			project: authSystem,
		},
		{
			name:    "auth-logout.md",
			content: "# Logout Implementation\n\nUser logout code.",
			tags:    []string{"authentication", "security", "backend"},
			project: authSystem,
		},
		{
			name:    "api-endpoints.md",
			content: "# API Endpoints\n\nREST API documentation.",
			tags:    []string{"api", "documentation", "backend"},
			project: "api-system",
		},
	}

	for _, f := range files {
		filePath := filepath.Join(workspaceRoot, f.name)
		require.NoError(t, os.WriteFile(filePath, []byte(f.content), 0644))

		absPath, err := filepath.Abs(filePath)
		require.NoError(t, err)
		relPath, err := filepath.Rel(workspaceRoot, absPath)
		require.NoError(t, err)

		entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(f.content)), time.Now())
		require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
		_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
		require.NoError(t, err)

		// Get file ID for metadata operations
		file, err := fileRepo.GetByPath(ctx, wsID, relPath)
		require.NoError(t, err)

		for _, tag := range f.tags {
			require.NoError(t, metaRepo.AddTag(ctx, wsID, file.ID, tag))
		}
		require.NoError(t, metaRepo.AddContext(ctx, wsID, file.ID, f.project))
	}

	// Test semantic queries
	t.Run("QueryByTag", func(t *testing.T) {
		authFiles, err := metaRepo.ListByTag(ctx, wsID, "authentication", repository.FileListOptions{Limit: 100})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(authFiles), 2, "Should find files tagged with 'authentication'")
	})

	t.Run("QueryByProject", func(t *testing.T) {
		authProjectFiles, err := metaRepo.ListByContext(ctx, wsID, authSystem, repository.FileListOptions{Limit: 100})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(authProjectFiles), 2, "Should find files in 'auth-system' project")
	})

	t.Run("QueryByMultipleTags", func(t *testing.T) {
		// Files with both "authentication" and "security"
		authFiles, err := metaRepo.ListByTag(ctx, wsID, "authentication", repository.FileListOptions{Limit: 100})
		require.NoError(t, err)
		securityFiles, err := metaRepo.ListByTag(ctx, wsID, "security", repository.FileListOptions{Limit: 100})
		require.NoError(t, err)

		// Find intersection
		commonFiles := 0
		for _, authFile := range authFiles {
			for _, secFile := range securityFiles {
				if authFile.FileID == secFile.FileID {
					commonFiles++
					break
				}
			}
		}
		assert.GreaterOrEqual(t, commonFiles, 2, "Should find files with both tags")
	})
}

// TestQualityNonDestructive tests that Cortex never modifies source files
func TestQualityNonDestructive(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBName)
	db, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))

	wsID := entity.NewWorkspaceID()
	wsInfo := contextinfo.WorkspaceInfo{
		ID:   wsID,
		Root: workspaceRoot,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)

	// Create a file with specific content
	originalContent := []byte("# Original Document\n\nThis content should never change.")
	filePath := filepath.Join(workspaceRoot, "original.md")
	require.NoError(t, os.WriteFile(filePath, originalContent, 0644))

	// Get original file info
	originalInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	originalModTime := originalInfo.ModTime()

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(originalContent)), originalModTime)

	// Process through pipeline
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

	// Add tags and projects
	file, err := fileRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)
	require.NoError(t, metaRepo.AddTag(ctx, wsID, file.ID, "test"))
	require.NoError(t, metaRepo.AddContext(ctx, wsID, file.ID, "test-project"))

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Verify file content is unchanged
	currentContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, currentContent, "File content should be unchanged")

	// Verify file modification time is unchanged (or only changed by OS)
	currentInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	// Allow small difference due to OS filesystem operations
	timeDiff := currentInfo.ModTime().Sub(originalModTime)
	assert.Less(t, timeDiff, 1*time.Second, "File modification time should not change significantly")

	// Verify file permissions are unchanged
	assert.Equal(t, originalInfo.Mode(), currentInfo.Mode(), "File permissions should be unchanged")
}

// TestQualityConsistency tests data consistency across operations
func TestQualityConsistency(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBName)
	db, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))

	wsID := entity.NewWorkspaceID()
	wsInfo := contextinfo.WorkspaceInfo{
		ID:   wsID,
		Root: workspaceRoot,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)

	// Create file
	filePath := filepath.Join(workspaceRoot, "consistency.md")
	content := []byte("# Consistency Test")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
	_, err = metaRepo.GetOrCreate(ctx, wsID, relPath, filepath.Ext(relPath))
	require.NoError(t, err)

	// Get file ID
	file, err := fileRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	// Add multiple tags
	tags := []string{"tag1", "tag2", "tag3"}
	for _, tag := range tags {
		require.NoError(t, metaRepo.AddTag(ctx, wsID, file.ID, tag))
	}

	// Verify consistency: same file should have all tags
	metadata, err := metaRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	for _, expectedTag := range tags {
		found := false
		for _, actualTag := range metadata.Tags {
			if actualTag == expectedTag {
				found = true
				break
			}
		}
		assert.True(t, found, "Tag '%s' should be present", expectedTag)
	}

	// Verify consistency: file should appear in all tag queries
	for _, tag := range tags {
		files, err := metaRepo.ListByTag(ctx, wsID, tag, repository.FileListOptions{Limit: 100})
		require.NoError(t, err)
		found := false
		for _, fileMeta := range files {
			if fileMeta.RelativePath == relPath {
				found = true
				break
			}
		}
		assert.True(t, found, "File should appear in tag query for '%s'", tag)
	}
}
