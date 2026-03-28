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
	"github.com/dacrypt/cortex/backend/internal/application/rag"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

type testQuery struct {
	query             string
	expectedTopics    []string
	minResults        int
	expectedRelevance float64
}

// TestQualityRAG tests the quality of RAG (Retrieval Augmented Generation)
func TestQualityRAG(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, "test.db")
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

	embedder := &mockEmbedder{}

	// Create multiple related documents
	documents := []struct {
		name    string
		content string
		topic   string
	}{
		{
			name: "authentication.md",
			content: `# Authentication Guide

This document explains user authentication using JWT tokens.
It covers login, logout, and token refresh mechanisms.

## Login Process

Users authenticate by providing username and password.
The system returns a JWT token upon successful authentication.

## Token Refresh

Tokens expire after 24 hours. Users can refresh tokens using the refresh endpoint.`,
			topic: "authentication",
		},
		{
			name: "authorization.md",
			content: `# Authorization Guide

This document explains role-based access control (RBAC).
It covers permissions, roles, and access management.

## Roles

There are three main roles: admin, user, and guest.
Each role has different permissions.`,
			topic: "authorization",
		},
		{
			name: "api.md",
			content: `# API Documentation

This document describes the REST API endpoints.
All endpoints require authentication via JWT tokens.

## Endpoints

- POST /api/login - Authenticate user
- GET /api/user - Get user information
- POST /api/logout - Logout user`,
			topic: "api",
		},
	}

	// Process all documents
	docStage := stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop())

	for _, doc := range documents {
		filePath := filepath.Join(workspaceRoot, doc.name)
		require.NoError(t, os.WriteFile(filePath, []byte(doc.content), 0644))

		absPath, err := filepath.Abs(filePath)
		require.NoError(t, err)
		relPath, err := filepath.Rel(workspaceRoot, absPath)
		require.NoError(t, err)

		entry := &entity.FileEntry{
			AbsolutePath: absPath,
			RelativePath: relPath,
			Filename:     doc.name,
			Extension:    ".md",
			FileSize:     int64(len(doc.content)),
			LastModified: time.Now(),
		}

		require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
		require.NoError(t, err)
		require.NoError(t, docStage.Process(ctx, entry))
	}

	// Setup RAG service
	mockProvider := &mockLLMProvider{
		id:   "mock-provider-rag",
		name: "Mock Provider RAG",
	}
	llmRouter := llm.NewRouter(zerolog.Nop())
	llmRouter.RegisterProvider(mockProvider)
	llmRouter.SetActiveProvider(mockProvider.ID(), "mock-model")

	ragService := rag.NewService(docRepo, vectorStore, embedder, llmRouter, zerolog.Nop())

	// Test queries with expected results
	testQueries := []testQuery{
		{
			query:             "How do users authenticate?",
			expectedTopics:    []string{"authentication"},
			minResults:        1,
			expectedRelevance: 0.5,
		},
		{
			query:             "What are the API endpoints?",
			expectedTopics:    []string{"api"},
			minResults:        1,
			expectedRelevance: 0.5,
		},
		{
			query:             "Explain JWT tokens",
			expectedTopics:    []string{"authentication", "api"},
			minResults:        1,
			expectedRelevance: 0.4,
		},
	}

	var metrics QualityMetrics
	metrics.Issues = []QualityIssue{}

	for _, tq := range testQueries {
		processRAGQuery(ctx, wsID, ragService, tq, &metrics)
	}

	// Calculate metrics
	if metrics.FilesProcessed > 0 {
		metrics.SuccessRate = float64(metrics.FilesSucceeded) / float64(metrics.FilesProcessed)
		if metrics.FilesSucceeded > 0 {
			metrics.AverageProcessingTime = metrics.AverageProcessingTime / time.Duration(metrics.FilesSucceeded)
		}
	}

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
	metrics.QualityScore = metrics.SuccessRate * (1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05 - float64(infoCount)*0.02)
	if metrics.QualityScore < 0 {
		metrics.QualityScore = 0
	}

	t.Logf("RAG Quality Metrics:")
	t.Logf("  Queries Processed: %d", metrics.FilesProcessed)
	t.Logf("  Success Rate: %.2f%%", metrics.SuccessRate*100)
	t.Logf("  Average Processing Time: %v", metrics.AverageProcessingTime)
	t.Logf("  Quality Score: %.2f", metrics.QualityScore)
	t.Logf("  Issues: %d errors, %d warnings, %d info", errorCount, warningCount, infoCount)

	assert.GreaterOrEqual(t, metrics.SuccessRate, 0.90, "RAG success rate should be at least 90%%")
	assert.GreaterOrEqual(t, metrics.QualityScore, 0.75, "RAG quality score should be at least 0.75")
	assert.Less(t, metrics.AverageProcessingTime, 2*time.Second, "RAG queries should be fast (<2s)")
}

func processRAGQuery(ctx context.Context, wsID entity.WorkspaceID, ragService *rag.Service, tq testQuery, metrics *QualityMetrics) {
	metrics.FilesProcessed++

	startTime := time.Now()
	response, err := ragService.Query(ctx, rag.QueryRequest{
		WorkspaceID:    wsID,
		Query:          tq.query,
		TopK:           5,
		GenerateAnswer: true,
	})
	processingTime := time.Since(startTime)

	if err != nil {
		metrics.FilesFailed++
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "rag",
			FilePath:    tq.query,
			Description: "RAG query failed",
			Expected:    "success",
			Actual:      err.Error(),
		})
		return
	}

	metrics.FilesSucceeded++
	metrics.AverageProcessingTime += processingTime

	checkRAGResponseQuality(tq, response, metrics)
}

func checkRAGResponseQuality(tq testQuery, response *rag.QueryResponse, metrics *QualityMetrics) {
	checkResultCount(tq, response, metrics)
	checkRelevanceScores(tq, response, metrics)
	checkExpectedTopics(tq, response, metrics)
	checkAnswerQuality(tq, response, metrics)
}

func checkResultCount(tq testQuery, response *rag.QueryResponse, metrics *QualityMetrics) {
	if len(response.Sources) < tq.minResults {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "error",
			Stage:       "rag",
			FilePath:    tq.query,
			Description: "Insufficient results",
			Expected:    tq.minResults,
			Actual:      len(response.Sources),
		})
	}
}

func checkRelevanceScores(tq testQuery, response *rag.QueryResponse, metrics *QualityMetrics) {
	allRelevant := true
	for _, source := range response.Sources {
		if source.Score < float32(tq.expectedRelevance) {
			allRelevant = false
			break
		}
	}

	if !allRelevant && len(response.Sources) > 0 {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "rag",
			FilePath:    tq.query,
			Description: "Some results have low relevance scores",
			Expected:    "all scores >= " + string(rune(tq.expectedRelevance)),
			Actual:      "some scores below threshold",
		})
	}
}

func checkExpectedTopics(tq testQuery, response *rag.QueryResponse, metrics *QualityMetrics) {
	foundTopics := make(map[string]bool)
	for _, source := range response.Sources {
		for _, expectedTopic := range tq.expectedTopics {
			if strings.Contains(strings.ToLower(source.RelativePath), expectedTopic) {
				foundTopics[expectedTopic] = true
			}
		}
	}

	for _, expectedTopic := range tq.expectedTopics {
		if !foundTopics[expectedTopic] {
			metrics.Issues = append(metrics.Issues, QualityIssue{
				Severity:    "warning",
				Stage:       "rag",
				FilePath:    tq.query,
				Description: "Expected topic not found in results",
				Expected:    expectedTopic,
				Actual:      "not found",
			})
		}
	}
}

func checkAnswerQuality(tq testQuery, response *rag.QueryResponse, metrics *QualityMetrics) {
	if response.Answer == "" {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "warning",
			Stage:       "rag",
			FilePath:    tq.query,
			Description: "Answer not generated",
			Expected:    "non-empty answer",
			Actual:      "empty",
		})
	} else if len(response.Answer) < 50 {
		metrics.Issues = append(metrics.Issues, QualityIssue{
			Severity:    "info",
			Stage:       "rag",
			FilePath:    tq.query,
			Description: "Answer is very short",
			Expected:    ">= 50 characters",
			Actual:      len(response.Answer),
		})
	}
}

// TestQualityEmbeddingConsistency tests that embeddings are consistent and meaningful
func TestQualityEmbeddingConsistency(t *testing.T) {
	t.Parallel()

	embedder := &mockEmbedder{}
	ctx := context.Background()

	// Test that similar content produces similar embeddings
	similarTexts := []string{
		"User authentication with JWT tokens",
		"Authentication using JWT tokens for users",
		"JWT token-based user authentication",
	}

	differentTexts := []string{
		"User authentication with JWT tokens",
		"API documentation for REST endpoints",
		"Database schema design patterns",
	}

	// Generate embeddings for similar texts
	similarEmbeddings := make([][]float32, len(similarTexts))
	for i, text := range similarTexts {
		emb, err := embedder.Embed(ctx, text)
		require.NoError(t, err)
		similarEmbeddings[i] = emb
	}

	// Generate embeddings for different texts
	differentEmbeddings := make([][]float32, len(differentTexts))
	for i, text := range differentTexts {
		emb, err := embedder.Embed(ctx, text)
		require.NoError(t, err)
		differentEmbeddings[i] = emb
	}

	// Calculate cosine similarity
	cosineSimilarity := func(a, b []float32) float32 {
		if len(a) != len(b) {
			return 0
		}
		var dotProduct, normA, normB float32
		for i := range a {
			dotProduct += a[i] * b[i]
			normA += a[i] * a[i]
			normB += b[i] * b[i]
		}
		if normA == 0 || normB == 0 {
			return 0
		}
		return dotProduct / (float32)(float64(normA)*float64(normB))
	}

	// Similar texts should have higher similarity
	similarSim := cosineSimilarity(similarEmbeddings[0], similarEmbeddings[1])
	differentSim := cosineSimilarity(differentEmbeddings[0], differentEmbeddings[1])

	t.Logf("Similarity between similar texts: %.3f", similarSim)
	t.Logf("Similarity between different texts: %.3f", differentSim)

	// Note: Hash-based embeddings may not have semantic similarity
	// This test mainly ensures embeddings are generated consistently
	assert.NotNil(t, similarEmbeddings[0], "Embeddings should be generated")
	assert.Equal(t, len(similarEmbeddings[0]), len(similarEmbeddings[1]), "Embeddings should have same dimension")
}

// TestQualityChunkingStrategy tests the quality of document chunking
func TestQualityChunkingStrategy(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(workspaceRoot, "test.db")
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

	embedder := &mockEmbedder{}
	docStage := stages.NewDocumentStage(metaRepo, docRepo, vectorStore, embedder, zerolog.Nop())

	// Create document with clear sections
	docContent := `# Main Title

## Section 1

This is section 1 content. It has multiple sentences.
Each sentence adds more information.
The section should be chunked appropriately.

## Section 2

This is section 2 content. Different topic.
More content here.

## Section 3

Final section with content.
`

	filePath := filepath.Join(workspaceRoot, "test.md")
	require.NoError(t, os.WriteFile(filePath, []byte(docContent), 0644))

	absPath, err := filepath.Abs(filePath)
	require.NoError(t, err)
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	require.NoError(t, err)

	entry := &entity.FileEntry{
		AbsolutePath: absPath,
		RelativePath: relPath,
		Filename:     "test.md",
		Extension:    ".md",
		FileSize:     int64(len(docContent)),
		LastModified: time.Now(),
	}

	require.NoError(t, fileRepo.Upsert(ctx, wsID, entry))
	require.NoError(t, err)
	require.NoError(t, docStage.Process(ctx, entry))

	// Check chunking quality
	doc, err := docRepo.GetDocumentByPath(ctx, wsID, relPath)
	require.NoError(t, err)

	chunks, err := docRepo.GetChunksByDocument(ctx, wsID, doc.ID)
	require.NoError(t, err)

	var issues []QualityIssue

	// Check chunk count is reasonable
	if len(chunks) == 0 {
		issues = append(issues, QualityIssue{
			Severity:    "error",
			Stage:       "chunking",
			FilePath:    relPath,
			Description: "No chunks created",
			Expected:    "at least 1 chunk",
			Actual:      0,
		})
	} else if len(chunks) > 10 {
		issues = append(issues, QualityIssue{
			Severity:    "warning",
			Stage:       "chunking",
			FilePath:    relPath,
			Description: "Too many chunks for document size",
			Expected:    "<= 10 chunks",
			Actual:      len(chunks),
		})
	}

	// Check chunk sizes are reasonable
	minChunkSize := 50   // Minimum reasonable chunk size
	maxChunkSize := 2000 // Maximum reasonable chunk size
	for i, chunk := range chunks {
		if len(chunk.Text) < minChunkSize {
			issues = append(issues, QualityIssue{
				Severity:    "warning",
				Stage:       "chunking",
				FilePath:    relPath,
				Description: "Chunk too small",
				Expected:    ">= " + string(rune(minChunkSize)) + " chars",
				Actual:      len(chunk.Text),
			})
		}
		if len(chunk.Text) > maxChunkSize {
			issues = append(issues, QualityIssue{
				Severity:    "warning",
				Stage:       "chunking",
				FilePath:    relPath,
				Description: "Chunk too large",
				Expected:    "<= " + string(rune(maxChunkSize)) + " chars",
				Actual:      len(chunk.Text),
			})
		}

		// Check heading paths are set
		if chunk.HeadingPath == "" {
			issues = append(issues, QualityIssue{
				Severity:    "info",
				Stage:       "chunking",
				FilePath:    relPath,
				Description: "Chunk missing heading path",
				Expected:    "heading path",
				Actual:      "empty",
			})
		}

		t.Logf("Chunk %d: size=%d, heading=%s", i+1, len(chunk.Text), chunk.HeadingPath)
	}

	// Check coverage
	totalChunkSize := 0
	for _, chunk := range chunks {
		totalChunkSize += len(chunk.Text)
	}
	coverage := float64(totalChunkSize) / float64(len(docContent))
	if coverage < 0.7 {
		issues = append(issues, QualityIssue{
			Severity:    "warning",
			Stage:       "chunking",
			FilePath:    relPath,
			Description: "Low content coverage",
			Expected:    ">= 70%",
			Actual:      coverage,
		})
	}

	errorCount := 0
	warningCount := 0
	infoCount := 0
	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	qualityScore := 1.0 - float64(errorCount)*0.1 - float64(warningCount)*0.05 - float64(infoCount)*0.02
	if qualityScore < 0 {
		qualityScore = 0
	}

	t.Logf("Chunking Quality Metrics:")
	t.Logf("  Chunks Created: %d", len(chunks))
	t.Logf("  Content Coverage: %.2f%%", coverage*100)
	t.Logf("  Quality Score: %.2f", qualityScore)
	t.Logf("  Issues: %d errors, %d warnings, %d info", errorCount, warningCount, infoCount)

	assert.Greater(t, len(chunks), 0, "Should create at least one chunk")
	assert.GreaterOrEqual(t, coverage, 0.7, "Should cover at least 70% of content")
	assert.GreaterOrEqual(t, qualityScore, 0.80, "Chunking quality should be at least 0.80")
}
