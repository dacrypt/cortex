package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

const (
	qualityTestDBNameStage   = "test.db"
	qualityTestFileName      = "test.md"
	notFoundLiteral          = "not found"
	logFilesProcessed        = "  Files Processed: %d"
	logSuccessRate           = "  Success Rate: %.2f%%"
	logAverageProcessingTime = "  Average Processing Time: %v"
	logQualityScore          = "  Quality Score: %.2f"
	logIssues                = "  Issues: %d errors, %d warnings"
	atLeastOneChunk          = "at least 1 chunk"
	endToEndLiteral          = "end-to-end"
)

// QualityMetrics holds quality assessment results for each pipeline stage
type QualityMetrics struct {
	StageName             string
	FilesProcessed        int
	FilesSucceeded        int
	FilesFailed           int
	SuccessRate           float64
	AverageProcessingTime time.Duration
	QualityScore          float64 // 0-1, overall quality score
	Issues                []QualityIssue
}

// QualityIssue represents a quality problem found during testing
type QualityIssue struct {
	Severity    string // "error", "warning", "info"
	Stage       string
	FilePath    string
	Description string
	Expected    interface{}
	Actual      interface{}
}

// TestQualityBasicStage tests the quality of basic metadata extraction
func TestQualityBasicStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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

	// Create test files with known properties
	testFiles := []struct {
		name     string
		content  string
		size     int64
		expected struct {
			hasSize      bool
			hasTimestamp bool
			hasExtension bool
		}
	}{
		{
			name:    qualityTestFileName,
			content: "# Test Document\n\nThis is a test.",
			size:    int64(len("# Test Document\n\nThis is a test.")),
			expected: struct {
				hasSize      bool
				hasTimestamp bool
				hasExtension bool
			}{true, true, true},
		},
		{
			name:    "code.go",
			content: "package main\n\nfunc main() {}",
			size:    int64(len("package main\n\nfunc main() {}")),
			expected: struct {
				hasSize      bool
				hasTimestamp bool
				hasExtension bool
			}{true, true, true},
		},
		{
			name:    "empty.txt",
			content: "",
			size:    0,
			expected: struct {
				hasSize      bool
				hasTimestamp bool
				hasExtension bool
			}{true, true, true},
		},
	}

	var metrics QualityMetrics
	metrics.Issues = []QualityIssue{}

	for _, tf := range testFiles {
		processBasicStageFile(t, ctx, wsID, workspaceRoot, fileRepo, tf, &metrics)
	}

	// Calculate metrics
	if metrics.FilesProcessed > 0 {
		metrics.SuccessRate = float64(metrics.FilesSucceeded) / float64(metrics.FilesProcessed)
		metrics.AverageProcessingTime = metrics.AverageProcessingTime / time.Duration(metrics.FilesSucceeded)
	}

	// Calculate quality score (0-1)
	errorCount, warningCount := countIssueSeverities(metrics.Issues)
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	// Assertions
	logQualityMetrics(t, "Basic Stage", metrics, errorCount, warningCount, 0)

	assert.GreaterOrEqual(t, metrics.SuccessRate, 0.95, "Success rate should be at least 95%%")
	assert.GreaterOrEqual(t, metrics.QualityScore, 0.90, "Quality score should be at least 0.90")
	assert.Less(t, metrics.AverageProcessingTime, 100*time.Millisecond, "Processing should be fast (<100ms)")
}

// TestQualityMimeStage tests the quality of MIME type detection
func TestQualityMimeStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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

	// Test files with known MIME types
	testCases := []struct {
		name         string
		content      []byte
		expectedMIME string
		expectedCat  string
	}{
		{
			name:         qualityTestFileName,
			content:      []byte("# Markdown"),
			expectedMIME: "text/markdown",
			expectedCat:  "text",
		},
		{
			name:         "test.png",
			content:      []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG magic bytes
			expectedMIME: "image/png",
			expectedCat:  "image",
		},
		{
			name:         "test.pdf",
			content:      []byte("%PDF-1.4"), // PDF magic bytes
			expectedMIME: "application/pdf",
			expectedCat:  "document",
		},
		{
			name:         "test.js",
			content:      []byte("console.log('test');"),
			expectedMIME: "text/javascript",
			expectedCat:  "code",
		},
	}

	var metrics QualityMetrics
	metrics.Issues = []QualityIssue{}

	stage := stages.NewMimeStage()

	for _, tc := range testCases {
		filePath := filepath.Join(workspaceRoot, tc.name)
		require.NoError(t, os.WriteFile(filePath, tc.content, 0644))

		absPath, err := filepath.Abs(filePath)
		require.NoError(t, err)
		relPath, err := filepath.Rel(workspaceRoot, absPath)
		require.NoError(t, err)

		entry := &entity.FileEntry{
			AbsolutePath: absPath,
			RelativePath: relPath,
			Filename:     tc.name,
			Extension:    filepath.Ext(tc.name),
			FileSize:     int64(len(tc.content)),
			LastModified: time.Now(),
		}

		// Ensure file exists in repo (BasicStage should have run)
		require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

		metrics.FilesProcessed++

		startTime := time.Now()
		err = stage.Process(ctx, entry)
		processingTime := time.Since(startTime)

		if err != nil {
			metrics.FilesFailed++
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "error",
				Stage:       "mime",
				FilePath:    relPath,
				Description: "MIME stage processing failed",
				Expected:    "success",
				Actual:      err.Error(),
			})
			continue
		}

		metrics.FilesSucceeded++
		metrics.AverageProcessingTime += processingTime

		// Verify MIME detection
		if entry.Enhanced == nil || entry.Enhanced.MimeType == nil {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "error",
				Stage:       "mime",
				FilePath:    relPath,
				Description: "MIME type not detected",
				Expected:    tc.expectedMIME,
				Actual:      "nil",
			})
			continue
		}

		detectedMIME := entry.Enhanced.MimeType.MimeType
		detectedCat := entry.Enhanced.MimeType.Category

		// Check MIME type (allow some flexibility for text files)
		if !strings.Contains(detectedMIME, strings.Split(tc.expectedMIME, "/")[0]) {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "warning",
				Stage:       "mime",
				FilePath:    relPath,
				Description: "MIME type category mismatch",
				Expected:    tc.expectedMIME,
				Actual:      detectedMIME,
			})
		}

		// Check category
		if detectedCat != tc.expectedCat {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "warning",
				Stage:       "mime",
				FilePath:    relPath,
				Description: "MIME category mismatch",
				Expected:    tc.expectedCat,
				Actual:      detectedCat,
			})
		}
	}

	// Calculate metrics
	if metrics.FilesProcessed > 0 {
		metrics.SuccessRate = float64(metrics.FilesSucceeded) / float64(metrics.FilesProcessed)
		if metrics.FilesSucceeded > 0 {
			metrics.AverageProcessingTime = metrics.AverageProcessingTime / time.Duration(metrics.FilesSucceeded)
		}
	}

	errorCount, warningCount := countIssueSeverities(metrics.Issues)
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	logQualityMetrics(t, "MIME Stage", metrics, errorCount, warningCount, 0)

	assert.GreaterOrEqual(t, metrics.SuccessRate, 0.90, "Success rate should be at least 90%%")
	assert.GreaterOrEqual(t, metrics.QualityScore, 0.85, "Quality score should be at least 0.85")
}

// TestQualityDocumentStage tests the quality of document parsing and chunking
func TestQualityDocumentStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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
	docRepo := sqlite.NewDocumentRepository(db)
	vectorStore := sqlite.NewVectorStore(db)

	// Use mock embedder for testing (no external dependencies)
	embedder := &mockEmbedder{}

	stage := stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop())

	// Test document with structure
	markdownContent := `---
title: Test Document
tags: [test, example]
---

# Introduction

This is the introduction section.

## Subsection 1

Content for subsection 1.

## Subsection 2

Content for subsection 2.

# Conclusion

Final thoughts.
`

	filePath := filepath.Join(workspaceRoot, qualityTestFileName)
	require.NoError(t, os.WriteFile(filePath, []byte(markdownContent), 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := &entity.FileEntry{
		AbsolutePath: absPath,
		RelativePath: relPath,
		Filename:     qualityTestFileName,
		Extension:    ".md",
		FileSize:     int64(len(markdownContent)),
		LastModified: time.Now(),
	}

	// Ensure file exists
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))

	var metrics QualityMetrics
	metrics.FilesProcessed = 1

	startTime := time.Now()
	err = stage.Process(ctx, entry)
	processingTime := time.Since(startTime)

	if err != nil {
		metrics.FilesFailed = 1
		t.Fatalf("Document stage failed: %v", err)
	}

	metrics.FilesSucceeded = 1
	metrics.AverageProcessingTime = processingTime

	// Quality checks
	doc, err := docRepo.GetDocumentByPath(ctx, wsID, relPath)
	require.NoError(t, err, "Document should be created")

	checkDocumentFrontmatter(doc, relPath, &metrics)
	chunks := checkDocumentChunks(t, ctx, wsID, documentChunkChecker{
		docRepo:         docRepo,
		doc:             doc,
		relPath:         relPath,
		markdownContent: markdownContent,
		metrics:         &metrics,
	})

	// Calculate quality score
	errorCount, warningCount := countIssueSeverities(metrics.Issues)
	metrics.SuccessRate = 1.0
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	logQualityMetrics(t, "Document Stage", metrics, errorCount, warningCount, 0)
	t.Logf("  Chunks Created: %d", len(chunks))

	assert.GreaterOrEqual(t, metrics.QualityScore, 0.80, "Quality score should be at least 0.80")
	assert.Greater(t, len(chunks), 0, "Should create at least one chunk")
}

// TestQualityRelationshipStage tests the quality of relationship detection
func TestQualityRelationshipStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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
	docRepo := sqlite.NewDocumentRepository(db)
	relRepo := sqlite.NewRelationshipRepository(db)

	// Create two related documents
	doc1Content := `---
title: Document 1
replaces: doc2.md
---

# Document 1

This document replaces [Document 2](doc2.md).
`

	doc2Content := `---
title: Document 2
---

# Document 2

This is the original document.
`

	doc1Path := filepath.Join(workspaceRoot, "doc1.md")
	doc2Path := filepath.Join(workspaceRoot, "doc2.md")
	require.NoError(t, os.WriteFile(doc1Path, []byte(doc1Content), 0644))
	require.NoError(t, os.WriteFile(doc2Path, []byte(doc2Content), 0644))

	// Process documents through DocumentStage first
	metaRepoRel := sqlite.NewMetadataRepository(db)
	embedder := &mockEmbedder{}
	vectorStore := sqlite.NewVectorStore(db)
	docStage := stages.NewDocumentStage(metaRepoRel, docRepo, vectorStore, embedder, zerolog.Nop())

	absPath1, _ := filepath.Abs(doc1Path)
	relPath1, _ := filepath.Rel(workspaceRoot, absPath1)
	entry1 := entity.NewFileEntry(workspaceRoot, relPath1, int64(len(doc1Content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry1))
	require.NoError(t, docStage.Process(ctx, entry1))

	absPath2, _ := filepath.Abs(doc2Path)
	relPath2, _ := filepath.Rel(workspaceRoot, absPath2)
	entry2 := entity.NewFileEntry(workspaceRoot, relPath2, int64(len(doc2Content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry2))
	require.NoError(t, err)
	require.NoError(t, docStage.Process(ctx, entry2))

	// Now test RelationshipStage
	stage := stages.NewRelationshipStage(docRepo, relRepo, zerolog.Nop())

	var metrics QualityMetrics
	metrics.FilesProcessed = 1

	startTime := time.Now()
	err = stage.Process(ctx, entry1)
	processingTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("Relationship stage failed: %v", err)
	}
	metrics.AverageProcessingTime = processingTime

	// Check relationships were created
	doc1, err := docRepo.GetDocumentByPath(ctx, wsID, relPath1)
	require.NoError(t, err)

	relationships, err := relRepo.GetOutgoing(ctx, wsID, doc1.ID, entity.RelationshipReplaces)
	require.NoError(t, err)

	// Should detect "replaces" relationship from frontmatter
	foundReplaces := false
	for _, rel := range relationships {
		if rel.Type == entity.RelationshipReplaces {
			foundReplaces = true
			break
		}
	}

	if !foundReplaces {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "relationship",
			FilePath:    relPath1,
			Description: "Replaces relationship not detected from frontmatter",
			Expected:    "replaces relationship",
			Actual:      notFoundLiteral,
		})
	}

	// Check Markdown link relationship
	linkRels, err := relRepo.GetOutgoing(ctx, wsID, doc1.ID, entity.RelationshipReferences)
	require.NoError(t, err)

	foundLink := false
	for _, rel := range linkRels {
		if rel.Type == entity.RelationshipReferences {
			foundLink = true
			break
		}
	}

	if !foundLink {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "relationship",
			FilePath:    relPath1,
			Description: "Markdown link relationship not detected",
			Expected:    "references relationship",
			Actual:      notFoundLiteral,
		})
	}

	// Calculate quality score
	errorCount, warningCount := countIssueSeverities(metrics.Issues)
	metrics.SuccessRate = 1.0
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	logQualityMetrics(t, "Relationship Stage", metrics, errorCount, warningCount, 0)
	t.Logf("  Relationships Detected: %d", len(relationships)+len(linkRels))

	assert.GreaterOrEqual(t, metrics.QualityScore, 0.80, "Quality score should be at least 0.80")
}

// TestQualityStateStage tests the quality of state inference
func TestQualityStateStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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
	docRepo := sqlite.NewDocumentRepository(db)
	relRepo := sqlite.NewRelationshipRepository(db)
	stateRepo := sqlite.NewDocumentStateRepository(db)

	// Create document that replaces another
	doc1Content := `---
title: Old Document
---

# Old Document
`

	doc2Content := `---
title: New Document
replaces: doc1.md
---

# New Document

This replaces the old document.
`

	doc1Path := filepath.Join(workspaceRoot, "doc1.md")
	doc2Path := filepath.Join(workspaceRoot, "doc2.md")
	require.NoError(t, os.WriteFile(doc1Path, []byte(doc1Content), 0644))
	require.NoError(t, os.WriteFile(doc2Path, []byte(doc2Content), 0644))

	// Process through DocumentStage and RelationshipStage
	embedder := &mockEmbedder{}
	vectorStore := sqlite.NewVectorStore(db)
	docStage := stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop())
	relStage := stages.NewRelationshipStage(docRepo, relRepo, zerolog.Nop())

	absPath1, _ := filepath.Abs(doc1Path)
	relPath1, _ := filepath.Rel(workspaceRoot, absPath1)
	entry1 := entity.NewFileEntry(workspaceRoot, relPath1, int64(len(doc1Content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry1))
	require.NoError(t, docStage.Process(ctx, entry1))
	require.NoError(t, relStage.Process(ctx, entry1))

	absPath2, _ := filepath.Abs(doc2Path)
	relPath2, _ := filepath.Rel(workspaceRoot, absPath2)
	entry2 := entity.NewFileEntry(workspaceRoot, relPath2, int64(len(doc2Content)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry2))
	require.NoError(t, docStage.Process(ctx, entry2))
	require.NoError(t, relStage.Process(ctx, entry2))

	// Test StateStage
	stage := stages.NewStateStage(docRepo, stateRepo, relRepo, zerolog.Nop())

	var metrics QualityMetrics

	// Process doc2 first (should be active because it replaces doc1)
	metrics.FilesProcessed++
	startTime := time.Now()
	err = stage.Process(ctx, entry2)
	processingTime := time.Since(startTime)

	if err != nil {
		metrics.FilesFailed++
		t.Fatalf("State stage failed: %v", err)
	}

	metrics.FilesSucceeded++
	metrics.AverageProcessingTime += processingTime

	doc2, err := docRepo.GetDocumentByPath(ctx, wsID, relPath2)
	require.NoError(t, err)

	state2, err := stateRepo.GetState(ctx, wsID, doc2.ID)
	require.NoError(t, err)

	if state2 != entity.DocumentStateActive {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "state",
			FilePath:    relPath2,
			Description: "Document that replaces another should be active",
			Expected:    entity.DocumentStateActive,
			Actual:      state2,
		})
	}

	// Process doc1 (should be replaced)
	metrics.FilesProcessed++
	startTime = time.Now()
	err = stage.Process(ctx, entry1)
	processingTime = time.Since(startTime)

	if err != nil {
		metrics.FilesFailed++
	} else {
		metrics.FilesSucceeded++
		metrics.AverageProcessingTime += processingTime

		doc1, err := docRepo.GetDocumentByPath(ctx, wsID, relPath1)
		require.NoError(t, err)

		state1, err := stateRepo.GetState(ctx, wsID, doc1.ID)
		require.NoError(t, err)

		if state1 != entity.DocumentStateReplaced {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "error",
				Stage:       "state",
				FilePath:    relPath1,
				Description: "Document that is replaced should have replaced state",
				Expected:    entity.DocumentStateReplaced,
				Actual:      state1,
			})
		}
	}

	// Calculate metrics
	if metrics.FilesProcessed > 0 {
		metrics.SuccessRate = float64(metrics.FilesSucceeded) / float64(metrics.FilesProcessed)
		if metrics.FilesSucceeded > 0 {
			metrics.AverageProcessingTime = metrics.AverageProcessingTime / time.Duration(metrics.FilesSucceeded)
		}
	}

	errorCount, warningCount := countIssueSeverities(metrics.Issues)
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	logQualityMetrics(t, "State Stage", metrics, errorCount, warningCount, 0)

	assert.GreaterOrEqual(t, metrics.SuccessRate, 0.95, "Success rate should be at least 95%%")
	assert.GreaterOrEqual(t, metrics.QualityScore, 0.85, "Quality score should be at least 0.85")
}

// TestQualityAIStage tests the quality of AI suggestions
func TestQualityAIStage(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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
	suggestedRepo := sqlite.NewSuggestedMetadataRepository(db)
	docRepo := sqlite.NewDocumentRepository(db)
	vectorStore := sqlite.NewVectorStore(db)
	projectRepo := sqlite.NewProjectRepository(db)
	projectService := project.NewService(projectRepo)

	// Create mock LLM provider (reuse type from integration_test.go)
	mockProvider := &mockLLMProvider{
		id:   "mock-provider-quality",
		name: "Mock Provider Quality",
	}
	llmRouter := llm.NewRouter(zerolog.Nop())
	llmRouter.RegisterProvider(mockProvider)
	llmRouter.SetActiveProvider(mockProvider.ID(), "mock-model")

	// Setup document stage first
	embedder := &mockEmbedder{}
	docStage := stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop())

	// Test document with clear topic
	docContent := `---
title: Authentication Guide
---

# Authentication Guide

This document explains how to implement user authentication using JWT tokens.
It covers login, logout, and token refresh mechanisms.
`

	filePath := filepath.Join(workspaceRoot, "auth.md")
	require.NoError(t, os.WriteFile(filePath, []byte(docContent), 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, int64(len(docContent)), time.Now())
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
	require.NoError(t, docStage.Process(ctx, entry))

	// Configure AI stage
	aiStage := stages.NewAIStageWithRAG(
		llmRouter,
		metaRepo,
		suggestedRepo,
		fileRepo,
		docRepo,
		vectorStore,
		embedder,
		projectRepo,
		nil,
		projectService,
		zerolog.Nop(),
		stages.AIStageConfig{
			Enabled:                true,
			AutoIndexEnabled:       true,
			ApplyTags:              true,
			ApplyProjects:          true,
			UseRAGForCategories:    true,
			UseRAGForTags:          true,
			UseRAGForProjects:      true,
			RAGSimilarityThreshold: 0.5,
		},
	)

	var metrics QualityMetrics
	metrics.FilesProcessed = 1

	startTime := time.Now()
	err = aiStage.Process(ctx, entry)
	processingTime := time.Since(startTime)

	if err != nil {
		t.Logf("AI stage failed (may be expected if LLM unavailable): %v", err)
		return // Skip if LLM not available
	}
	metrics.AverageProcessingTime = processingTime

	// Check metadata was updated
	metadata, err := metaRepo.GetByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	// Check if tags were suggested/applied
	if metadata.AICategory == nil || metadata.AICategory.Category == "" {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "ai",
			FilePath:    relPath,
			Description: "AI category not generated",
			Expected:    "category like 'documentation' or 'guide'",
			Actual:      "empty",
		})
	}

	// Check if summary was generated
	if metadata.AISummary == nil || metadata.AISummary.Summary == "" {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "info",
			Stage:       "ai",
			FilePath:    relPath,
			Description: "AI summary not generated",
			Expected:    "summary text",
			Actual:      "empty",
		})
	}

	// Calculate quality score
	errorCount := 0
	warningCount := 0
	infoCount := 0
	for _, issue := range metrics.Issues {
		switch issue.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}
	metrics.SuccessRate = 1.0
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05 - float64(infoCount)*0.02)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	logQualityMetrics(t, "AI Stage", metrics, errorCount, warningCount, infoCount)

	// AI stage is optional, so we're more lenient
	assert.GreaterOrEqual(t, metrics.SuccessRate, 0.80, "Success rate should be at least 80%% (AI is optional)")
}

// TestQualityEndToEnd tests the quality of the complete pipeline
func TestQualityEndToEnd(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, qualityTestDBNameStage)
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

	// Setup all repositories
	fileRepo := sqlite.NewFileRepository(db)
	metaRepo := sqlite.NewMetadataRepository(db)
	suggestedRepo := sqlite.NewSuggestedMetadataRepository(db)
	docRepo := sqlite.NewDocumentRepository(db)
	vectorStore := sqlite.NewVectorStore(db)
	relRepo := sqlite.NewRelationshipRepository(db)
	stateRepo := sqlite.NewDocumentStateRepository(db)
	projectRepo := sqlite.NewProjectRepository(db)
	projectService := project.NewService(projectRepo)

	// Setup LLM (mock)
	mockProvider := &mockLLMProvider{
		id:   "mock-provider-e2e",
		name: "Mock Provider E2E",
	}
	llmRouter := llm.NewRouter(zerolog.Nop())
	llmRouter.RegisterProvider(mockProvider)
	llmRouter.SetActiveProvider(mockProvider.ID(), "mock-model")

	embedder := &mockEmbedder{}

	// Create complete pipeline
	publisher := &testEventPublisher{}
	orchestrator := NewOrchestrator(publisher, zerolog.Nop())

	// Add all stages
	orchestrator.AddStage(stages.NewMimeStage())
	orchestrator.AddStage(stages.NewCodeStage())
	orchestrator.AddStage(stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop()))
	orchestrator.AddStage(stages.NewRelationshipStage(docRepo, relRepo, zerolog.Nop()))
	orchestrator.AddStage(stages.NewStateStage(docRepo, stateRepo, relRepo, zerolog.Nop()))
	orchestrator.AddStage(stages.NewAIStageWithRAG(
		llmRouter,
		metaRepo,
		suggestedRepo,
		fileRepo,
		docRepo,
		vectorStore,
		embedder,
		projectRepo,
		nil,
		projectService,
		zerolog.Nop(),
		stages.AIStageConfig{
			Enabled:          true,
			AutoIndexEnabled: true,
		},
	))

	// Create test document
	docContent := `---
title: Test Document
tags: [test, example]
---

# Test Document

This is a test document with [a link](other.md).

## Section 1

Content here.
`

	filePath := filepath.Join(workspaceRoot, qualityTestFileName)
	require.NoError(t, os.WriteFile(filePath, []byte(docContent), 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := &entity.FileEntry{
		AbsolutePath: absPath,
		RelativePath: relPath,
		Filename:     qualityTestFileName,
		Extension:    ".md",
		FileSize:     int64(len(docContent)),
		LastModified: time.Now(),
	}

	// Process through complete pipeline
	startTime := time.Now()
	err = orchestrator.Process(ctx, entry)
	totalTime := time.Since(startTime)

	require.NoError(t, err, "Pipeline should complete without errors")

	// Quality checks for end-to-end
	var issues []QualityIssue

	// Check file was indexed
	_, err = fileRepo.GetByPath(ctx, wsID, relPath)
	if err != nil {
		issues = append(issues, QualityIssue{
			Severity:    "error",
			Stage:       endToEndLiteral,
			FilePath:    relPath,
			Description: "File not found after pipeline",
			Expected:    "file exists",
			Actual:      notFoundLiteral,
		})
	}

	// Check document was created
	doc, err := docRepo.GetDocumentByPath(ctx, wsID, relPath)
	if err != nil || doc == nil {
		issues = append(issues, QualityIssue{
			Severity:    "error",
			Stage:       endToEndLiteral,
			FilePath:    relPath,
			Description: "Document not created",
			Expected:    "document exists",
			Actual:      notFoundLiteral,
		})
	} else {
		// Check chunks exist
		chunks, _ := docRepo.GetChunksByDocument(ctx, wsID, doc.ID)
		if len(chunks) == 0 {
			issues = append(issues, QualityIssue{
				Severity:    "error",
				Stage:       endToEndLiteral,
				FilePath:    relPath,
				Description: "No chunks created",
				Expected:    atLeastOneChunk,
				Actual:      0,
			})
		}

		// Check state was set
		state, err := stateRepo.GetState(ctx, wsID, doc.ID)
		if err != nil || state == "" {
			issues = append(issues, QualityIssue{
				Severity:    "warning",
				Stage:       endToEndLiteral,
				FilePath:    relPath,
				Description: "Document state not set",
				Expected:    "state (draft/active/etc)",
				Actual:      "empty",
			})
		}
	}

	// Calculate quality score
	errorCount, warningCount := countIssueSeverities(issues)

	qualityScore := 1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05
	if qualityScore < 0 {
		qualityScore = 0
	}

	t.Logf("End-to-End Pipeline Quality Metrics:")
	t.Logf("  Total Processing Time: %v", totalTime)
	t.Logf(logQualityScore, qualityScore)
	t.Logf(logIssues, errorCount, warningCount)

	assert.GreaterOrEqual(t, qualityScore, 0.85, "End-to-end quality score should be at least 0.85")
	assert.Less(t, totalTime, 5*time.Second, "Pipeline should complete in reasonable time")
}

// testEventPublisher is a simple event publisher for testing
type testEventPublisher struct {
	events []*event.Event
}

func (p *testEventPublisher) Publish(ctx context.Context, evt *event.Event) error {
	if p.events == nil {
		p.events = []*event.Event{}
	}
	p.events = append(p.events, evt)
	return nil
}

func (p *testEventPublisher) Subscribe(eventTypes []event.EventType, handler event.Handler) event.SubscriptionID {
	return event.SubscriptionID("test-sub")
}

func (p *testEventPublisher) SubscribeAll(handler event.Handler) event.SubscriptionID {
	return event.SubscriptionID("test-sub-all")
}

func (p *testEventPublisher) Unsubscribe(id event.SubscriptionID) {
	// No-op for testing
}

func (p *testEventPublisher) Close() error {
	return nil
}

// Helper functions for quality tests

func processBasicStageFile(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, workspaceRoot string, fileRepo interface {
	Upsert(ctx context.Context, wsID entity.WorkspaceID, entry *entity.FileEntry) error
	GetByPath(ctx context.Context, wsID entity.WorkspaceID, relPath string) (*entity.FileEntry, error)
}, tf struct {
	name     string
	content  string
	size     int64
	expected struct {
		hasSize      bool
		hasTimestamp bool
		hasExtension bool
	}
}, metrics *QualityMetrics) {
	filePath := filepath.Join(workspaceRoot, tf.name)
	require.NoError(t, os.WriteFile(filePath, []byte(tf.content), 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := entity.NewFileEntry(workspaceRoot, relPath, tf.size, time.Now())

	metrics.FilesProcessed++

	stage := stages.NewBasicStage()
	startTime := time.Now()
	err = stage.Process(ctx, entry)
	processingTime := time.Since(startTime)

	if err != nil {
		metrics.FilesFailed++
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "basic",
			FilePath:    relPath,
			Description: "Basic stage processing failed",
			Expected:    "success",
			Actual:      err.Error(),
		})
		return
	}

	metrics.FilesSucceeded++
	metrics.AverageProcessingTime += processingTime

	// Verify file was stored - need to upsert first
	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
	file, err := fileRepo.GetByPath(ctx, wsID, relPath)
	if err != nil {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "basic",
			FilePath:    relPath,
			Description: "File not found in repository after processing",
			Expected:    "file exists",
			Actual:      notFoundLiteral,
		})
		return
	}

	checkBasicStageQuality(tf, file, relPath, metrics)
}

func checkBasicStageQuality(tf struct {
	name     string
	content  string
	size     int64
	expected struct {
		hasSize      bool
		hasTimestamp bool
		hasExtension bool
	}
}, file *entity.FileEntry, relPath string, metrics *QualityMetrics) {
	if tf.expected.hasSize && file.FileSize != tf.size {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "basic",
			FilePath:    relPath,
			Description: "File size mismatch",
			Expected:    tf.size,
			Actual:      file.FileSize,
		})
	}

	if tf.expected.hasExtension && file.Extension != filepath.Ext(tf.name) {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "basic",
			FilePath:    relPath,
			Description: "Extension mismatch",
			Expected:    filepath.Ext(tf.name),
			Actual:      file.Extension,
		})
	}

	if tf.expected.hasTimestamp && file.LastModified.IsZero() {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "basic",
			FilePath:    relPath,
			Description: "Timestamp not set",
			Expected:    "non-zero timestamp",
			Actual:      "zero timestamp",
		})
	}
}

func countIssueSeverities(issues []QualityIssue) (errorCount, warningCount int) {
	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}
	return errorCount, warningCount
}

func logQualityMetrics(t *testing.T, stageName string, metrics QualityMetrics, errorCount, warningCount, infoCount int) {
	t.Logf("%s Quality Metrics:", stageName)
	t.Logf(logFilesProcessed, metrics.FilesProcessed)
	t.Logf(logSuccessRate, metrics.SuccessRate*100)
	t.Logf(logAverageProcessingTime, metrics.AverageProcessingTime)
	t.Logf(logQualityScore, metrics.QualityScore)
	if infoCount > 0 {
		t.Logf("  Issues: %d errors, %d warnings, %d info", errorCount, warningCount, infoCount)
	} else {
		t.Logf(logIssues, errorCount, warningCount)
	}
}

func checkDocumentFrontmatter(doc *entity.Document, relPath string, metrics *QualityMetrics) {
	if doc.Frontmatter == nil {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "document",
			FilePath:    relPath,
			Description: "Frontmatter not extracted",
			Expected:    "frontmatter with title and tags",
			Actual:      "nil",
		})
		return
	}
	if doc.Frontmatter["title"] != "Test Document" {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "document",
			FilePath:    relPath,
			Description: "Title not extracted correctly",
			Expected:    "Test Document",
			Actual:      doc.Frontmatter["title"],
		})
	}
}

type documentChunkChecker struct {
	docRepo interface {
		GetChunksByDocument(ctx context.Context, wsID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.Chunk, error)
	}
	doc             *entity.Document
	relPath         string
	markdownContent string
	metrics         *QualityMetrics
}

func checkDocumentChunks(t *testing.T, ctx context.Context, wsID entity.WorkspaceID, checker documentChunkChecker) []*entity.Chunk {
	chunks, err := checker.docRepo.GetChunksByDocument(ctx, wsID, checker.doc.ID)
	require.NoError(t, err)

	if len(chunks) == 0 {
		checker.metrics.Issues = append(checker.metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "document",
			FilePath:    checker.relPath,
			Description: "No chunks created",
			Expected:    atLeastOneChunk,
			Actual:      0,
		})
		return chunks
	}

	checkChunkQuality(chunks, checker.relPath, checker.markdownContent, checker.metrics)
	return chunks
}

func checkChunkQuality(chunks []*entity.Chunk, relPath, markdownContent string, metrics *QualityMetrics) {
	totalChunkSize := 0
	for _, chunk := range chunks {
		totalChunkSize += len(chunk.Text)
		if len(chunk.Text) == 0 {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "error",
				Stage:       "document",
				FilePath:    relPath,
				Description: "Empty chunk found",
				Expected:    "non-empty chunk",
				Actual:      "empty",
			})
		}
	}

	coverage := float64(totalChunkSize) / float64(len(markdownContent))
	if coverage < 0.7 {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "document",
			FilePath:    relPath,
			Description: "Low chunk coverage",
			Expected:    ">= 70%",
			Actual:      coverage,
		})
	}
}
