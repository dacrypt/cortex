package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	metadataApp "github.com/dacrypt/cortex/backend/internal/application/metadata"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/application/rag"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	embeddingInfra "github.com/dacrypt/cortex/backend/internal/infrastructure/embedding"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	llmProviders "github.com/dacrypt/cortex/backend/internal/infrastructure/llm/providers"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/mirror"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/trace"
)

const (
	separatorLine          = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	estadoLabel            = "  Estado:"
	testWorkspacePath      = "test-workspace"
	ollamaEndpoint         = "http://localhost:11434"
	relationshipMoreFormat = "    ... (%d relaciones más)"
	relationshipItemFormat = "    • %s -> %s (fuerza: %.2f)"
	projectItemFormat      = "    • %s (ID: %s, %d documentos)"
	document1Label         = "  Documento 1:"
	document2Label         = "  Documento 2:"
	document3Label         = "  Documento 3:"
)

// setupVerboseTestWorkspace finds and sets up the test workspace, returning the absolute path
func setupVerboseTestWorkspace(t *testing.T, logger zerolog.Logger) string {
	workspaceRoot := filepath.Join("..", "..", "..", "..", testWorkspacePath)
	absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		workspaceRoot = t.TempDir()
		absWorkspaceRoot = workspaceRoot
		logger.Warn().Str("workspace", workspaceRoot).Msg("Workspace de prueba no encontrado, usando directorio temporal")
	} else {
		logger.Info().Str("workspace", absWorkspaceRoot).Msg("Workspace de prueba encontrado")
	}
	return absWorkspaceRoot
}

// findPDFFile searches for a PDF file in the workspace
func findPDFFile(t *testing.T, absWorkspaceRoot string, logger zerolog.Logger, preferredName string) *entity.FileEntry {
	pdfPath := filepath.Join(absWorkspaceRoot, "Libros", preferredName)
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		librosDir := filepath.Join(absWorkspaceRoot, "Libros")
		if entries, err := os.ReadDir(librosDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
					pdfPath = filepath.Join(librosDir, entry.Name())
					logger.Info().Str("pdf", pdfPath).Msg("Usando PDF encontrado")
					break
				}
			}
		}
	}

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("Archivo PDF no encontrado: %s", pdfPath)
		return nil
	}

	logger.Info().Str("pdf_path", pdfPath).Msg("✅ Archivo PDF encontrado")

	fileInfo, err := os.Stat(pdfPath)
	require.NoError(t, err)

	relPath, err := filepath.Rel(absWorkspaceRoot, pdfPath)
	require.NoError(t, err)

	entry := &entity.FileEntry{
		ID:           entity.NewFileID(relPath),
		AbsolutePath: pdfPath,
		RelativePath: relPath,
		Filename:     filepath.Base(pdfPath),
		Extension:    ".pdf",
		FileSize:     fileInfo.Size(),
		LastModified: fileInfo.ModTime(),
	}

	logger.Info().
		Str("relative_path", entry.RelativePath).
		Int64("file_size", entry.FileSize).
		Time("last_modified", entry.LastModified).
		Msg("✅ FileEntry creado")

	return entry
}

// findPDFFiles searches for multiple PDF files in the workspace
// If preferredFiles is provided, it will prioritize those files
func findPDFFiles(t *testing.T, absWorkspaceRoot string, logger zerolog.Logger, count int, preferredFiles ...string) []*entity.FileEntry {
	librosDir := filepath.Join(absWorkspaceRoot, "Libros")

	pdfFiles := findPreferredPDFFiles(librosDir, logger, preferredFiles)
	pdfFiles = findAdditionalPDFFiles(librosDir, pdfFiles, count)

	if len(pdfFiles) < count {
		t.Skipf("Se requieren al menos %d archivos PDF en %s/Libros para este test", count, testWorkspacePath)
		return nil
	}

	entries := createFileEntries(t, absWorkspaceRoot, pdfFiles)
	logger.Info().
		Int("count", len(entries)).
		Msg("✅ Archivos PDF encontrados")
	return entries
}

// findPreferredPDFFiles finds and returns preferred PDF files.
func findPreferredPDFFiles(librosDir string, logger zerolog.Logger, preferredFiles []string) []string {
	var pdfFiles []string
	for _, preferred := range preferredFiles {
		preferredPath := filepath.Join(librosDir, preferred)
		if _, err := os.Stat(preferredPath); err == nil {
			pdfFiles = append(pdfFiles, preferredPath)
			logger.Info().Str("file", preferred).Msg("✅ Archivo preferido encontrado")
		}
	}
	return pdfFiles
}

// findAdditionalPDFFiles finds additional PDF files if needed.
func findAdditionalPDFFiles(librosDir string, existingFiles []string, count int) []string {
	if len(existingFiles) >= count {
		return existingFiles
	}

	foundPreferred := make(map[string]bool)
	for _, file := range existingFiles {
		foundPreferred[filepath.Base(file)] = true
	}

	entries, err := os.ReadDir(librosDir)
	if err != nil {
		return existingFiles
	}

	for _, entry := range entries {
		if len(existingFiles) >= count {
			break
		}
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			if foundPreferred[entry.Name()] {
				continue
			}
			pdfPath := filepath.Join(librosDir, entry.Name())
			existingFiles = append(existingFiles, pdfPath)
		}
	}

	return existingFiles
}

// createFileEntries creates FileEntry objects from file paths.
func createFileEntries(t *testing.T, absWorkspaceRoot string, pdfFiles []string) []*entity.FileEntry {
	var entries []*entity.FileEntry
	for _, pdfPath := range pdfFiles {
		fileInfo, err := os.Stat(pdfPath)
		require.NoError(t, err)

		relPath, err := filepath.Rel(absWorkspaceRoot, pdfPath)
		require.NoError(t, err)

		entry := &entity.FileEntry{
			ID:           entity.NewFileID(relPath),
			AbsolutePath: pdfPath,
			RelativePath: relPath,
			Filename:     filepath.Base(pdfPath),
			Extension:    ".pdf",
			LastModified: fileInfo.ModTime(),
			FileSize:     fileInfo.Size(),
		}
		entries = append(entries, entry)
	}
	return entries
}

// reportLLMTraces reports LLM traces for a file.
func reportLLMTraces(ctx context.Context, logger zerolog.Logger, traceRepo *sqlite.TraceRepository, workspaceID entity.WorkspaceID, relativePath string) {
	logger.Info().Msg("")
	logger.Info().Msg("📝 PASO 4.5: Traces de LLM (Prompts y Respuestas)")
	traces, err := traceRepo.ListTracesByFile(ctx, workspaceID, relativePath, 100)
	if err != nil || len(traces) == 0 {
		logger.Info().Msg("ℹ️  No se encontraron traces de LLM")
		return
	}
	logger.Info().Int("trace_count", len(traces)).Msg("Traces encontrados:")
	for i, tr := range traces {
		reportSingleTrace(logger, tr, i+1)
	}
}

// reportSingleTrace reports a single LLM trace.
func reportSingleTrace(logger zerolog.Logger, tr entity.ProcessingTrace, traceNum int) {
	logger.Info().Msg("")
	logger.Info().
		Int("trace_num", traceNum).
		Str("stage", tr.Stage).
		Str("operation", tr.Operation).
		Str("model", tr.Model).
		Int64("duration_ms", tr.DurationMs).
		Msgf(separatorLine)
	logger.Info().Msgf("📝 TRACE %d: %s / %s", traceNum, tr.Stage, tr.Operation)
	logger.Info().Msg(separatorLine)

	reportTracePrompt(logger, tr.PromptPath)
	reportTraceOutput(logger, tr.OutputPath)
}

// reportTracePrompt reports the prompt content from a trace.
func reportTracePrompt(logger zerolog.Logger, promptPath string) {
	if promptPath == "" {
		return
	}
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		return
	}
	logger.Info().Msg("")
	logger.Info().Msg("📤 PROMPT:")
	logger.Info().Msg(string(promptContent))
}

// reportTraceOutput reports the output content from a trace.
func reportTraceOutput(logger zerolog.Logger, outputPath string) {
	if outputPath == "" {
		return
	}
	outputContent, err := os.ReadFile(outputPath)
	if err != nil {
		return
	}
	logger.Info().Msg("")
	logger.Info().Msg("📥 RESPUESTA:")
	logger.Info().Msg(string(outputContent))
}

// setupTestDatabase creates and migrates the test database, returning all repositories
type testRepositories struct {
	fileRepo         *sqlite.FileRepository
	metaRepo         *sqlite.MetadataRepository
	suggestedRepo    *sqlite.SuggestedMetadataRepository
	docRepo          *sqlite.DocumentRepository
	vectorStore      *sqlite.VectorStore
	workspaceRepo    *sqlite.WorkspaceRepository
	projectRepo      *sqlite.ProjectRepository
	relationshipRepo *sqlite.RelationshipRepository
	stateRepo        *sqlite.DocumentStateRepository
	traceRepo        *sqlite.TraceRepository
	conn             *sqlite.Connection
}

func setupTestDatabase(t *testing.T, ctx context.Context, absWorkspaceRoot string, workspaceID entity.WorkspaceID, logger zerolog.Logger) *testRepositories {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	require.NoError(t, err)

	require.NoError(t, conn.Migrate(ctx))

	repos := &testRepositories{
		fileRepo:         sqlite.NewFileRepository(conn),
		metaRepo:         sqlite.NewMetadataRepository(conn),
		suggestedRepo:    sqlite.NewSuggestedMetadataRepository(conn),
		docRepo:          sqlite.NewDocumentRepository(conn),
		vectorStore:      sqlite.NewVectorStore(conn),
		workspaceRepo:    sqlite.NewWorkspaceRepository(conn),
		projectRepo:      sqlite.NewProjectRepository(conn),
		relationshipRepo: sqlite.NewRelationshipRepository(conn),
		stateRepo:        sqlite.NewDocumentStateRepository(conn),
		traceRepo:        sqlite.NewTraceRepository(conn),
		conn:             conn,
	}

	workspace := entity.NewWorkspace(absWorkspaceRoot, testWorkspacePath)
	workspace.ID = workspaceID
	require.NoError(t, repos.workspaceRepo.Create(ctx, workspace))

	logger.Info().Str("workspace_id", workspaceID.String()).Msg("✅ Workspace creado")

	return repos
}

// setupLLMAndEmbedder configures LLM router and embedder with Ollama
func setupLLMAndEmbedder(t *testing.T, ctx context.Context, logger zerolog.Logger, traceRepo *sqlite.TraceRepository) (*llm.Router, embedding.Embedder) {
	logger.Info().Msg("🤖 Configurando LLM Router...")
	llmRouter := llm.NewRouter(logger)

	ollamaProvider := llmProviders.NewOllamaProvider("ollama", "Ollama", ollamaEndpoint)
	available, err := ollamaProvider.IsAvailable(ctx)
	if err != nil || !available {
		t.Fatalf("❌ Ollama LLM NO DISPONIBLE - Este test requiere Ollama real para calidad de producción. Error: %v", err)
	}
	llmRouter.RegisterProvider(ollamaProvider)
	llmRouter.SetActiveProvider("ollama", "llama3.2")
	logger.Info().Msg("✅ Ollama LLM DISPONIBLE - usando LLM REAL")

	traceWriter := trace.NewWriter(traceRepo, logger)
	llmRouter.SetTraceWriter(traceWriter)

	logger.Info().Msg("🔢 Configurando Embedder...")
	ollamaEmbedder := embeddingInfra.NewOllamaEmbedder(ollamaEndpoint, "nomic-embed-text")
	testVector, testErr := ollamaEmbedder.Embed(ctx, "test")
	if testErr != nil || len(testVector) == 0 {
		t.Fatalf("❌ Ollama embedder NO DISPONIBLE - Este test requiere Ollama real para calidad de producción. Error: %v", testErr)
	}
	realEmbedder := newTimedEmbedder(ollamaEmbedder, logger)
	logger.Info().Msg("✅ Ollama embeddings DISPONIBLE - usando embedder REAL con medición de tiempos")

	return llmRouter, realEmbedder
}

// setupPipelineStages configures all pipeline stages
func setupPipelineStages(t *testing.T, orchestrator *Orchestrator, repos *testRepositories, llmRouter *llm.Router, realEmbedder embedding.Embedder, logger zerolog.Logger) *project.Service {
	mirrorExtractor := &mirror.Extractor{
		Logger:      logger.With().Str("component", "mirror").Logger(),
		MaxFileSize: 25 * 1024 * 1024,
	}
	mirrorStage := stages.NewMirrorStage(mirrorExtractor, repos.metaRepo, nil, logger)
	require.NoError(t, orchestrator.InsertStage(2, newTimedStage(mirrorStage, logger)))

	metadataRegistry := metadata.NewRegistry()
	metadataRegistry.Register(metadata.NewPDFExtractor(logger))
	metadataRegistry.Register(metadata.NewImageExtractor(logger))
	metadataRegistry.Register(metadata.NewAudioExtractor(logger))
	metadataRegistry.Register(metadata.NewVideoExtractor(logger))
	metadataRegistry.Register(metadata.NewUniversalExtractor(logger))
	metadataStage := stages.NewMetadataStage(metadataRegistry, logger)
	orchestrator.AddStage(newTimedStage(metadataStage, logger))

	docStage := stages.NewDocumentStage(repos.metaRepo, repos.docRepo, repos.vectorStore, realEmbedder, logger)
	orchestrator.AddStage(newTimedStage(docStage, logger))

	relationshipStage := stages.NewRelationshipStageWithRAG(
		repos.docRepo,
		repos.relationshipRepo,
		repos.metaRepo,
		repos.projectRepo,
		repos.vectorStore,
		realEmbedder,
		logger,
	)
	orchestrator.AddStage(newTimedStage(relationshipStage, logger))

	stateStage := stages.NewStateStage(repos.docRepo, repos.stateRepo, repos.relationshipRepo, logger)
	orchestrator.AddStage(newTimedStage(stateStage, logger))

	ragService := rag.NewService(repos.docRepo, repos.vectorStore, realEmbedder, llmRouter, logger)
	ragAdapter := metadataApp.NewRAGServiceAdapter(ragService)
	llmAdapter := metadataApp.NewLLMServiceAdapter(llmRouter)
	projectService := project.NewService(repos.projectRepo)
	suggestionService := metadataApp.NewSuggestionService(
		ragAdapter,
		llmAdapter,
		repos.metaRepo,
		repos.docRepo,
		repos.projectRepo,
		logger,
	)
	suggestionStage := stages.NewSuggestionStage(
		suggestionService,
		repos.metaRepo,
		repos.suggestedRepo,
		logger,
		true,
	)
	currentStages := orchestrator.GetStages()
	aiStageIndex := len(currentStages) - 1
	require.NoError(t, orchestrator.InsertStage(aiStageIndex, newTimedStage(suggestionStage, logger)))

	aiConfig := stages.AIStageConfig{
		Enabled:                true,
		AutoSummaryEnabled:     true,
		AutoIndexEnabled:       true,
		ApplyTags:              true,
		ApplyProjects:          true,
		UseSuggestedContexts:   false,
		MaxFileSize:            10 * 1024 * 1024,
		MaxTags:                10,
		RequestTimeout:         5 * time.Minute,
		CategoryEnabled:        true,
		RelatedEnabled:         true,
		RelatedMaxResults:      5,
		RelatedCandidates:      10,
		UseRAGForCategories:    true,
		UseRAGForTags:          true,
		UseRAGForProjects:      true,
		UseRAGForRelated:       true,
		UseRAGForSummary:       true,
		RAGSimilarityThreshold: 0.5,
	}
	aiStage := stages.NewAIStageWithRAG(
		llmRouter,
		repos.metaRepo,
		repos.suggestedRepo,
		repos.fileRepo,
		repos.docRepo,
		repos.vectorStore,
		realEmbedder,
		repos.projectRepo,
		nil,
		projectService,
		logger,
		aiConfig,
	)
	orchestrator.AddStage(newTimedStage(aiStage, logger))

	enrichmentConfig := stages.EnrichmentConfig{
		Enabled:                   true,
		NEREnabled:                true,
		CitationsEnabled:          true,
		SentimentEnabled:          true,
		OCREnabled:                true,
		TablesEnabled:             true,
		FormulasEnabled:           true,
		DependenciesEnabled:       true,
		TranscriptionEnabled:      true,
		DuplicateDetectionEnabled: true,
		ISBNEnrichmentEnabled:     true,
	}
	enrichmentStage := stages.NewEnrichmentStage(
		llmRouter,
		repos.metaRepo,
		repos.docRepo,
		repos.vectorStore,
		realEmbedder,
		logger,
		enrichmentConfig,
	)
	orchestrator.AddStage(newTimedStage(enrichmentStage, logger))

	return projectService
}

// verifyStageResults verifies results from all pipeline stages.
func verifyStageResults(
	t *testing.T,
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	entry *entity.FileEntry,
	savedEntry *entity.FileEntry,
	workspaceID entity.WorkspaceID,
	realEmbedder embedding.Embedder,
	projectService *project.Service,
) *entity.FileMetadata {
	verifyBasicStage(logger, savedEntry)
	verifyMimeStage(logger, savedEntry)
	verifyMirrorStage(logger, savedEntry)
	verifyMetadataStage(logger, savedEntry)
	doc, docID := verifyDocumentStage(ctx, logger, repos, entry, workspaceID, realEmbedder)
	verifyRelationshipStage(ctx, logger, repos, workspaceID, docID)
	verifySuggestionStage(ctx, logger, repos, workspaceID, entry.ID)
	verifyStateStage(ctx, logger, repos, workspaceID, docID)
	meta := verifyAIStage(ctx, logger, repos, entry, workspaceID, realEmbedder, doc, docID, projectService)
	verifyEnrichmentStage(logger, meta)
	return meta
}

// verifyBasicStage verifies basic stage results.
func verifyBasicStage(logger zerolog.Logger, savedEntry *entity.FileEntry) {
	logger.Info().Msg("")
	logger.Info().Msg("1️⃣  BASIC STAGE:")
	logger.Info().
		Bool("indexed", savedEntry.Enhanced.IndexedState.Basic).
		Str("folder", savedEntry.Enhanced.Folder).
		Int("depth", savedEntry.Enhanced.Depth).
		Msg(estadoLabel)
}

// verifyMimeStage verifies MIME stage results.
func verifyMimeStage(logger zerolog.Logger, savedEntry *entity.FileEntry) {
	logger.Info().Msg("")
	logger.Info().Msg("2️⃣  MIME STAGE:")
	if savedEntry.Enhanced.MimeType != nil {
		logger.Info().
			Str("mime_type", savedEntry.Enhanced.MimeType.MimeType).
			Str("category", savedEntry.Enhanced.MimeType.Category).
			Msg("  Tipo MIME detectado:")
	}
}

// verifyMirrorStage verifies mirror stage results.
func verifyMirrorStage(logger zerolog.Logger, savedEntry *entity.FileEntry) {
	logger.Info().Msg("")
	logger.Info().Msg("3️⃣  MIRROR STAGE:")
	logger.Info().
		Bool("indexed", savedEntry.Enhanced.IndexedState.Mirror).
		Msg(estadoLabel)
}

// verifyMetadataStage verifies metadata stage results.
func verifyMetadataStage(logger zerolog.Logger, savedEntry *entity.FileEntry) {
	logger.Info().Msg("")
	logger.Info().Msg("4️⃣  METADATA STAGE:")
	if savedEntry.Enhanced.DocumentMetrics != nil {
		dm := savedEntry.Enhanced.DocumentMetrics
		title := ""
		if dm.Title != nil {
			title = *dm.Title
		}
		author := ""
		if dm.Author != nil {
			author = *dm.Author
		}
		logger.Info().
			Str("title", title).
			Str("author", author).
			Int("page_count", dm.PageCount).
			Int("word_count", dm.WordCount).
			Int("character_count", dm.CharacterCount).
			Msg("  Metadatos extraídos:")
	}
}

// verifyDocumentStage verifies document stage results.
func verifyDocumentStage(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	entry *entity.FileEntry,
	workspaceID entity.WorkspaceID,
	realEmbedder embedding.Embedder,
) (*entity.Document, entity.DocumentID) {
	logger.Info().Msg("")
	logger.Info().Msg("5️⃣  DOCUMENT STAGE:")
	savedEntry, _ := repos.fileRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
	if savedEntry != nil {
		logger.Info().
			Bool("indexed", savedEntry.Enhanced.IndexedState.Document).
			Msg(estadoLabel)
	}

	docID := entity.NewDocumentID(entry.RelativePath)
	doc, err := repos.docRepo.GetDocument(ctx, workspaceID, docID)
	if err != nil || doc == nil {
		return nil, docID
	}

	logger.Info().
		Str("document_id", doc.ID.String()).
		Str("title", doc.Title).
		Time("created_at", doc.CreatedAt).
		Msg("  Documento creado:")

	verifyDocumentChunks(ctx, logger, repos, workspaceID, docID, realEmbedder)
	return doc, docID
}

// verifyDocumentChunks verifies document chunks and embeddings.
func verifyDocumentChunks(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	realEmbedder embedding.Embedder,
) {
	chunks, err := repos.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
	if err != nil {
		return
	}

	logger.Info().
		Int("chunk_count", len(chunks)).
		Msg("  Chunks creados:")

	logger.Info().Msg("  🔍 Verificando embeddings enriquecidos:")
	logger.Info().Msg("    ℹ️  Los chunks originales no tienen metadata enriquecida.")
	logger.Info().Msg("    ℹ️  Los embeddings se regeneran en AIStage con metadata enriquecida.")
	logger.Info().
		Int("total_chunks", len(chunks)).
		Msg("    Chunks procesados:")

	if len(chunks) == 0 {
		return
	}

	testChunk := chunks[0]
	testVector, err := realEmbedder.Embed(ctx, testChunk.Text)
	if err != nil {
		return
	}

	similarChunks, err := repos.vectorStore.Search(ctx, workspaceID, testVector, 3)
	if err == nil && len(similarChunks) > 0 {
		logger.Info().
			Int("similar_chunks", len(similarChunks)).
			Float32("top_similarity", similarChunks[0].Similarity).
			Msg("    ✅ Embeddings funcionando correctamente")
	}
}

// verifyRelationshipStage verifies relationship stage results.
func verifyRelationshipStage(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) {
	logger.Info().Msg("")
	logger.Info().Msg("6️⃣  RELATIONSHIP STAGE:")
	relationships, err := repos.relationshipRepo.GetAllOutgoing(ctx, workspaceID, docID)
	if err != nil {
		return
	}

	if len(relationships) == 0 {
		logger.Info().Msg("  ℹ️  No se encontraron relaciones (puede ser normal si es el primer documento)")
		return
	}

	logger.Info().
		Int("count", len(relationships)).
		Msg("  Relaciones encontradas:")

	for i, rel := range relationships {
		if i >= 5 {
			logger.Info().Msgf(relationshipMoreFormat, len(relationships)-5)
			break
		}
		logger.Info().
			Str("type", string(rel.Type)).
			Str("target_doc", rel.ToDocument.String()).
			Float64("strength", rel.Strength).
			Msgf(relationshipItemFormat, string(rel.Type), rel.ToDocument.String(), rel.Strength)
	}
}

// verifySuggestionStage verifies suggestion stage results.
func verifySuggestionStage(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	fileID entity.FileID,
) {
	logger.Info().Msg("")
	logger.Info().Msg("7️⃣  SUGGESTION STAGE:")
	suggestedMeta, err := repos.suggestedRepo.Get(ctx, workspaceID, fileID)
	if err != nil {
		logger.Warn().Err(err).Msg("  No se pudo obtener sugerencias")
		return
	}

	if suggestedMeta != nil && suggestedMeta.HasSuggestions() {
		logger.Info().
			Int("suggested_tags", len(suggestedMeta.SuggestedTags)).
			Int("suggested_projects", len(suggestedMeta.SuggestedProjects)).
			Msg("  Sugerencias generadas:")
	}
}

// verifyStateStage verifies state stage results.
func verifyStateStage(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) {
	logger.Info().Msg("")
	logger.Info().Msg("8️⃣  STATE STAGE:")
	state, err := repos.stateRepo.GetState(ctx, workspaceID, docID)
	if err != nil {
		logger.Warn().Err(err).Msg("  No se pudo obtener estado")
		return
	}

	stateStr := string(state)
	logger.Info().
		Str("state", stateStr).
		Msg("  Estado del documento:")

	stateExplanation := map[string]string{
		"active":   "Documento activo - en uso reciente",
		"archived": "Documento archivado - no en uso activo",
		"draft":    "Borrador - trabajo en progreso",
		"final":    "Versión final - completado",
	}
	if explanation, ok := stateExplanation[stateStr]; ok {
		logger.Info().
			Str("explanation", explanation).
			Msg("    Significado:")
	}
}

// verifyAIStage verifies AI stage results.
func verifyAIStage(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	entry *entity.FileEntry,
	workspaceID entity.WorkspaceID,
	realEmbedder embedding.Embedder,
	doc *entity.Document,
	docID entity.DocumentID,
	projectService *project.Service,
) *entity.FileMetadata {
	logger.Info().Msg("")
	logger.Info().Msg("9️⃣  AI STAGE:")
	meta, err := repos.metaRepo.GetOrCreate(ctx, workspaceID, entry.RelativePath, entry.Extension)
	if err != nil {
		logger.Warn().Err(err).Msg("  No se pudo obtener metadata")
		return nil
	}

	if meta == nil {
		return nil
	}

	verifyAISummary(logger, meta)
	verifyAICategory(logger, meta)
	verifyAIContext(ctx, logger, repos, entry, workspaceID, meta, projectService)
	verifyRAG(ctx, logger, repos, workspaceID, docID, doc, realEmbedder)

	return meta
}

// verifyAISummary verifies AI summary.
func verifyAISummary(logger zerolog.Logger, meta *entity.FileMetadata) {
	if meta.AISummary == nil {
		return
	}
	logger.Info().
		Str("summary", truncateString(meta.AISummary.Summary, 200)).
		Strs("key_terms", meta.AISummary.KeyTerms).
		Msg("  Resumen AI:")
}

// verifyAICategory verifies AI category.
func verifyAICategory(logger zerolog.Logger, meta *entity.FileMetadata) {
	if meta.AICategory == nil {
		return
	}
	logger.Info().
		Str("category", meta.AICategory.Category).
		Float64("confidence", meta.AICategory.Confidence).
		Msg("  Categoría AI:")
}

// verifyAIContext verifies AI context extraction.
func verifyAIContext(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	entry *entity.FileEntry,
	workspaceID entity.WorkspaceID,
	meta *entity.FileMetadata,
	projectService *project.Service,
) {
	meta = reloadMetadataIfNeeded(ctx, repos, workspaceID, entry.RelativePath, meta)

	if meta.AIContext == nil || !meta.AIContext.HasAnyData() {
		verifyAIContextReload(ctx, logger, repos, workspaceID, entry.RelativePath)
		return
	}

	reportAIContextSummary(logger, meta.AIContext)
	reportAIContextDetails(logger, meta.AIContext)
	verifyAIContextProjects(ctx, logger, repos, meta, projectService, workspaceID)
}

// reloadMetadataIfNeeded reloads metadata from database if available.
func reloadMetadataIfNeeded(
	ctx context.Context,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	relativePath string,
	meta *entity.FileMetadata,
) *entity.FileMetadata {
	metaLatest, err := repos.metaRepo.GetByPath(ctx, workspaceID, relativePath)
	if err == nil && metaLatest != nil {
		return metaLatest
	}
	return meta
}

// verifyAIContextReload attempts to reload and verify AI context.
func verifyAIContextReload(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	relativePath string,
) {
	metaReload, err := repos.metaRepo.GetByPath(ctx, workspaceID, relativePath)
	if err != nil || metaReload == nil || metaReload.AIContext == nil || !metaReload.AIContext.HasAnyData() {
		logger.Info().Msg("  ⚠️  AIContext no extraído o no persistido")
		return
	}

	logger.Info().Msg("  ✅ Contexto AI Extraído (recargado desde BD):")
	ctxData := metaReload.AIContext
	logger.Info().
		Int("authors", len(ctxData.Authors)).
		Int("locations", len(ctxData.Locations)).
		Int("people", len(ctxData.PeopleMentioned)).
		Int("organizations", len(ctxData.Organizations)).
		Msg("    Resumen:")
}

// reportAIContextSummary reports a summary of AI context.
func reportAIContextSummary(logger zerolog.Logger, ctxData *entity.AIContext) {
	logger.Info().Msg("  ✅ Contexto AI Extraído y Persistido:")
	logger.Info().
		Int("authors", len(ctxData.Authors)).
		Int("locations", len(ctxData.Locations)).
		Int("people", len(ctxData.PeopleMentioned)).
		Int("organizations", len(ctxData.Organizations)).
		Int("events", len(ctxData.HistoricalEvents)).
		Int("references", len(ctxData.References)).
		Float64("confidence", ctxData.Confidence).
		Str("source", ctxData.Source).
		Time("extracted_at", ctxData.ExtractedAt).
		Msg("    Resumen:")
}

// reportAIContextDetails reports detailed AI context information.
func reportAIContextDetails(logger zerolog.Logger, ctxData *entity.AIContext) {
	if len(ctxData.Authors) > 0 {
		logger.Info().Msg("    Autores:")
		for _, author := range ctxData.Authors {
			affiliation := ""
			if author.Affiliation != nil {
				affiliation = " (" + *author.Affiliation + ")"
			}
			logger.Info().
				Str("name", author.Name).
				Str("role", author.Role).
				Msgf("      • %s - %s%s", author.Name, author.Role, affiliation)
		}
	}

	if len(ctxData.Locations) > 0 {
		logger.Info().Msg("    Ubicaciones:")
		for _, loc := range ctxData.Locations {
			logger.Info().
				Str("name", loc.Name).
				Str("type", loc.Type).
				Msgf("      • %s (%s)", loc.Name, loc.Type)
		}
	}

	if len(ctxData.PeopleMentioned) > 0 {
		logger.Info().Msg("    Personas Mencionadas:")
		for _, person := range ctxData.PeopleMentioned {
			logger.Info().
				Str("name", person.Name).
				Str("role", person.Role).
				Msgf("      • %s - %s", person.Name, person.Role)
		}
	}

	if len(ctxData.Organizations) > 0 {
		logger.Info().Msg("    Organizaciones:")
		for _, org := range ctxData.Organizations {
			logger.Info().
				Str("name", org.Name).
				Str("type", org.Type).
				Msgf("      • %s (%s)", org.Name, org.Type)
		}
	}

	if ctxData.Publisher != nil {
		logger.Info().Str("publisher", *ctxData.Publisher).Msg("    Editorial:")
	}
	if ctxData.PublicationYear != nil {
		logger.Info().Int("year", *ctxData.PublicationYear).Msg("    Año de Publicación:")
	}
	if ctxData.ISBN != nil {
		logger.Info().Str("isbn", *ctxData.ISBN).Msg("    ISBN:")
	}
}

// verifyAIContextProjects verifies projects associated with AI context.
func verifyAIContextProjects(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	meta *entity.FileMetadata,
	projectService *project.Service,
	workspaceID entity.WorkspaceID,
) {
	if len(meta.Contexts) == 0 {
		logger.Info().Msg("  ℹ️  No hay proyectos asignados")
		return
	}

	logger.Info().Msg("")
	logger.Info().Msg("  📁 Proyectos Asociados:")
	for _, projectName := range meta.Contexts {
		proj, err := projectService.GetProjectByName(ctx, workspaceID, projectName)
		if err != nil || proj == nil {
			logger.Info().
				Str("project_name", projectName).
				Msgf("    • %s (proyecto no encontrado en repositorio)", projectName)
			continue
		}

		docIDs, err := repos.projectRepo.GetDocuments(ctx, workspaceID, proj.ID, false)
		if err == nil {
			logger.Info().
				Str("project_id", proj.ID.String()).
				Str("project_name", projectName).
				Int("document_count", len(docIDs)).
				Msgf(projectItemFormat, projectName, proj.ID.String(), len(docIDs))
		} else {
			logger.Info().
				Str("project_id", proj.ID.String()).
				Str("project_name", projectName).
				Msgf("    • %s (ID: %s)", projectName, proj.ID.String())
		}
	}
}

// verifyRAG verifies RAG functionality.
func verifyRAG(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	doc *entity.Document,
	realEmbedder embedding.Embedder,
) {
	if doc == nil {
		return
	}

	logger.Info().Msg("")
	logger.Info().Msg("  🔎 Verificación de RAG:")
	chunks, err := repos.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
	if err != nil || len(chunks) == 0 {
		return
	}

	testChunk := chunks[0]
	testVector, err := realEmbedder.Embed(ctx, testChunk.Text)
	if err != nil {
		return
	}

	similarDocs, err := repos.vectorStore.Search(ctx, workspaceID, testVector, 5)
	if err != nil {
		return
	}

	logger.Info().
		Int("similar_docs", len(similarDocs)).
		Msg("    Búsqueda RAG:")

	if len(similarDocs) > 0 {
		logger.Info().Msg("    ✅ RAG funcionando correctamente")
	}
}

// verifyEnrichmentStage verifies enrichment stage results.
func verifyEnrichmentStage(logger zerolog.Logger, meta *entity.FileMetadata) {
	logger.Info().Msg("")
	logger.Info().Msg("🔟 ENRICHMENT STAGE:")
	if meta == nil || meta.EnrichmentData == nil {
		logger.Info().Msg("  ℹ️  No se generaron datos de enriquecimiento")
		return
	}

	enrichment := meta.EnrichmentData
	logger.Info().
		Int("named_entities", len(enrichment.NamedEntities)).
		Int("citations", len(enrichment.Citations)).
		Int("tables", len(enrichment.Tables)).
		Int("formulas", len(enrichment.Formulas)).
		Msg("  Datos de enriquecimiento:")

	if enrichment.Sentiment != nil {
		logger.Info().
			Str("sentiment", enrichment.Sentiment.OverallSentiment).
			Float64("score", enrichment.Sentiment.Score).
			Msg("    Sentimiento:")
	}
}

// reportQualityMetrics reports quality metrics.
func reportQualityMetrics(logger zerolog.Logger, meta *entity.FileMetadata, processingTime time.Duration) {
	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("📈 MÉTRICAS DE CALIDAD")
	logger.Info().Msg(separatorLine)

	metadataFields := 0
	if meta != nil {
		if meta.AISummary != nil {
			metadataFields++
		}
		if meta.AICategory != nil {
			metadataFields++
		}
		if meta.AIContext != nil {
			metadataFields++
		}
		if len(meta.Tags) > 0 {
			metadataFields++
		}
		if len(meta.Contexts) > 0 {
			metadataFields++
		}
	}

	logger.Info().
		Int("metadata_fields", metadataFields).
		Int("max_possible", 5).
		Float64("coverage", float64(metadataFields)/5.0*100.0).
		Msg("  Cobertura de metadata:")

	logger.Info().
		Dur("total_time", processingTime).
		Float64("seconds", processingTime.Seconds()).
		Msg("  Tiempo total de procesamiento:")
}

// TestVerbosePipelineSingleFile procesa un solo archivo PDF real a través de todo el pipeline
// y reporta cada paso de forma muy verbosa para facilitar el debugging.
// Este test requiere Ollama LLM y embeddings reales para ejecutarse.
func TestVerbosePipelineSingleFile(t *testing.T) {
	// Configurar logger verboso
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)

	logger.Info().Msg(separatorLine)
	logger.Info().Msg("🚀 TEST E2E VERBOSE - PROCESAMIENTO COMPLETO DE ARCHIVO PDF")
	logger.Info().Msg(separatorLine)

	// PASO 1: Configurar workspace y usar archivo PDF real
	logger.Info().Msg("")
	logger.Info().Msg("📁 PASO 1: Configurando workspace y localizando archivo PDF")

	absWorkspaceRoot := setupVerboseTestWorkspace(t, logger)
	entry := findPDFFile(t, absWorkspaceRoot, logger, "40 Conferencias.pdf")
	if entry == nil {
		return
	}

	// PASO 2: Configurar base de datos y repositorios
	logger.Info().Msg("")
	logger.Info().Msg("💾 PASO 2: Configurando base de datos y repositorios")

	workspaceID := entity.NewWorkspaceID()
	ctx := context.Background()
	repos := setupTestDatabase(t, ctx, absWorkspaceRoot, workspaceID, logger)
	defer repos.conn.Close()

	// Configurar contexto con workspace info
	wsInfo := contextinfo.WorkspaceInfo{
		ID:            workspaceID,
		Root:          absWorkspaceRoot,
		Config:        entity.WorkspaceConfig{},
		ForceFullScan: true,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	// PASO 3: Configurar el pipeline con todas las etapas
	logger.Info().Msg("")
	logger.Info().Msg("⚙️  PASO 3: Configurando pipeline con todas las etapas")

	orchestrator := NewOrchestrator(nil, logger)
	llmRouter, realEmbedder := setupLLMAndEmbedder(t, ctx, logger, repos.traceRepo)
	projectService := setupPipelineStages(t, orchestrator, repos, llmRouter, realEmbedder, logger)

	// Listar todas las etapas configuradas
	allStages := orchestrator.GetStages()
	logger.Info().Msg("")
	logger.Info().Msg("📋 ETAPAS DEL PIPELINE CONFIGURADAS:")
	for i, stageName := range allStages {
		logger.Info().
			Int("order", i+1).
			Str("stage", stageName).
			Msgf("  %d. %s", i+1, stageName)
	}

	// PASO 4: Procesar el archivo a través del pipeline con medición de tiempos
	logger.Info().Msg("")
	logger.Info().Msg("🔄 PASO 4: Procesando archivo a través del pipeline")
	startTime := time.Now()

	// Guardar el archivo en el repositorio primero
	require.NoError(t, repos.fileRepo.Upsert(ctx, workspaceID, entry))

	// Procesar a través del pipeline
	require.NoError(t, orchestrator.Process(ctx, entry), "Pipeline processing should succeed")

	processingTime := time.Since(startTime)
	logger.Info().
		Dur("total_time", processingTime).
		Float64("seconds", processingTime.Seconds()).
		Msg("✅ Pipeline completado")

	// Reportar tiempos de embeddings si está disponible
	if timedEmbed, ok := realEmbedder.(*timedEmbedder); ok {
		timedEmbed.ReportEmbeddingTimes()
	}

	// Reportar tiempos de stages
	reportStageTimes(logger)

	// PASO 4.5: Mostrar todos los traces de LLM (prompts y respuestas)
	reportLLMTraces(ctx, logger, repos.traceRepo, workspaceID, entry.RelativePath)

	// PASO 5: Guardar el archivo en el repositorio
	logger.Info().Msg("")
	logger.Info().Msg("💾 PASO 5: Guardando archivo en el repositorio")
	savedEntry, err := repos.fileRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
	require.NoError(t, err)
	require.NotNil(t, savedEntry)
	logger.Info().Msg("✅ Archivo guardado en repositorio")

	// PASO 6: Verificar resultados de cada etapa
	logger.Info().Msg("")
	logger.Info().Msg("✅ PASO 6: Verificando resultados de cada etapa")
	meta := verifyStageResults(t, ctx, logger, repos, entry, savedEntry, workspaceID, realEmbedder, projectService)

	// Métricas de calidad
	reportQualityMetrics(logger, meta, processingTime)

	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("✅ TEST COMPLETADO EXITOSAMENTE")
	logger.Info().Msg(separatorLine)
}

// timedStage wraps a stage to measure execution time
type timedStage struct {
	stage      Stage
	logger     zerolog.Logger
	stageTimes map[string]time.Duration
	mu         sync.Mutex
}

func newTimedStage(stage Stage, logger zerolog.Logger) Stage {
	return &timedStage{
		stage:      stage,
		logger:     logger,
		stageTimes: make(map[string]time.Duration),
	}
}

func (t *timedStage) Name() string {
	return t.stage.Name()
}

func (t *timedStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	start := time.Now()
	err := t.stage.Process(ctx, entry)
	duration := time.Since(start)

	t.mu.Lock()
	t.stageTimes[t.stage.Name()] = duration
	t.mu.Unlock()

	t.logger.Info().
		Str("stage", t.stage.Name()).
		Dur("duration", duration).
		Float64("seconds", duration.Seconds()).
		Msgf("⏱️  Stage '%s' completado", t.stage.Name())

	return err
}

func reportStageTimes(logger zerolog.Logger) {
	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("⏱️  TIEMPOS POR STAGE:")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("ℹ️  Los tiempos se muestran en los logs de cada stage")
}

// timedEmbedder envuelve un embedder para medir tiempos
type timedEmbedder struct {
	embedder embedding.Embedder
	logger   zerolog.Logger
	times    []time.Duration
	mu       sync.Mutex
}

func newTimedEmbedder(e embedding.Embedder, logger zerolog.Logger) embedding.Embedder {
	return &timedEmbedder{
		embedder: e,
		logger:   logger,
		times:    make([]time.Duration, 0),
	}
}

func (t *timedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()

	result, err := t.embedder.Embed(ctx, text)

	duration := time.Since(start)
	t.mu.Lock()
	t.times = append(t.times, duration)
	totalCalls := len(t.times)
	var totalTime time.Duration
	for _, d := range t.times {
		totalTime += d
	}
	avgTime := totalTime / time.Duration(totalCalls)
	t.mu.Unlock()

	t.logger.Debug().
		Dur("duration", duration).
		Dur("avg_duration", avgTime).
		Int("total_calls", totalCalls).
		Int("text_length", len(text)).
		Msg("  ⏱️  Embedding generado")

	return result, err
}

func (t *timedEmbedder) ReportEmbeddingTimes() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.times) == 0 {
		return
	}

	var totalTime time.Duration
	for _, d := range t.times {
		totalTime += d
	}
	avgTime := totalTime / time.Duration(len(t.times))

	t.logger.Info().Msg("")
	t.logger.Info().Msg(separatorLine)
	t.logger.Info().Msg("⏱️  TIEMPOS DE EMBEDDINGS:")
	t.logger.Info().Msg(separatorLine)
	t.logger.Info().
		Int("total_embeddings", len(t.times)).
		Dur("total_time", totalTime).
		Dur("avg_time", avgTime).
		Float64("avg_seconds", avgTime.Seconds()).
		Msg("  Estadísticas:")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// mockLLMProvider y mockEmbedder están definidos en integration_test.go

// processFileThroughPipeline processes a file through the pipeline and returns processing time.
func processFileThroughPipeline(
	t *testing.T,
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	orchestrator *Orchestrator,
	entry *entity.FileEntry,
	workspaceID entity.WorkspaceID,
	fileLabel string,
) time.Duration {
	logger.Info().Str("file", entry.RelativePath).Msg(fileLabel)
	require.NoError(t, repos.fileRepo.Upsert(ctx, workspaceID, entry))
	startTime := time.Now()
	require.NoError(t, orchestrator.Process(ctx, entry))
	processingTime := time.Since(startTime)
	logger.Info().
		Dur("time", processingTime).
		Float64("seconds", processingTime.Seconds()).
		Msg("✅ Archivo procesado")
	return processingTime
}

// verifyDocumentsCreated verifies that documents were created for the entries.
func verifyDocumentsCreated(
	t *testing.T,
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	entry1, entry2, entry3 *entity.FileEntry,
) (*entity.Document, *entity.Document, *entity.Document, entity.DocumentID, entity.DocumentID, entity.DocumentID) {
	docID1 := entity.NewDocumentID(entry1.RelativePath)
	docID2 := entity.NewDocumentID(entry2.RelativePath)
	docID3 := entity.NewDocumentID(entry3.RelativePath)

	doc1, err := repos.docRepo.GetDocument(ctx, workspaceID, docID1)
	require.NoError(t, err)
	require.NotNil(t, doc1)

	doc2, err := repos.docRepo.GetDocument(ctx, workspaceID, docID2)
	require.NoError(t, err)
	require.NotNil(t, doc2)

	doc3, err := repos.docRepo.GetDocument(ctx, workspaceID, docID3)
	require.NoError(t, err)
	require.NotNil(t, doc3)

	logger.Info().Msg("")
	logger.Info().Msg("📄 DOCUMENTOS CREADOS:")
	logger.Info().
		Str("doc1_id", doc1.ID.String()).
		Str("doc1_title", doc1.Title).
		Str("doc1_path", entry1.RelativePath).
		Msg(document1Label)
	logger.Info().
		Str("doc2_id", doc2.ID.String()).
		Str("doc2_title", doc2.Title).
		Str("doc2_path", entry2.RelativePath).
		Msg(document2Label)
	logger.Info().
		Str("doc3_id", doc3.ID.String()).
		Str("doc3_title", doc3.Title).
		Str("doc3_path", entry3.RelativePath).
		Msg(document3Label)

	return doc1, doc2, doc3, docID1, docID2, docID3
}

// reportDocumentRelationships reports relationships for a document.
func reportDocumentRelationships(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	doc *entity.Document,
	label string,
) []*entity.DocumentRelationship {
	relationships, err := repos.relationshipRepo.GetAllOutgoing(ctx, workspaceID, docID)
	if err != nil {
		return nil
	}

	logger.Info().
		Int("count", len(relationships)).
		Str("from", doc.ID.String()).
		Msg(label)

	for i, rel := range relationships {
		if i >= 10 {
			logger.Info().Msgf(relationshipMoreFormat, len(relationships)-10)
			break
		}
		logger.Info().
			Str("type", string(rel.Type)).
			Str("to", rel.ToDocument.String()).
			Float64("strength", rel.Strength).
			Msgf(relationshipItemFormat, string(rel.Type), rel.ToDocument.String(), rel.Strength)
	}

	return relationships
}

// verifyDirectRelations verifies direct relationships between documents.
func verifyDirectRelations(
	logger zerolog.Logger,
	relationships1, relationships2, relationships3 []*entity.DocumentRelationship,
	docID1, docID2, docID3 entity.DocumentID,
) (bool, bool, bool) {
	hasDirectRelation12 := checkDirectRelation(logger, relationships1, docID2, "Doc1 -> Doc2")
	hasDirectRelation13 := checkDirectRelation(logger, relationships1, docID3, "Doc1 -> Doc3")

	if !hasDirectRelation12 {
		hasDirectRelation12 = checkDirectRelation(logger, relationships2, docID1, "Doc2 -> Doc1")
	}

	hasDirectRelation23 := checkDirectRelation(logger, relationships2, docID3, "Doc2 -> Doc3")

	if !hasDirectRelation13 {
		hasDirectRelation13 = checkDirectRelation(logger, relationships3, docID1, "Doc3 -> Doc1")
	}

	if !hasDirectRelation23 {
		hasDirectRelation23 = checkDirectRelation(logger, relationships3, docID2, "Doc3 -> Doc2")
	}

	totalDirectRelations := 0
	if hasDirectRelation12 {
		totalDirectRelations++
	}
	if hasDirectRelation13 {
		totalDirectRelations++
	}
	if hasDirectRelation23 {
		totalDirectRelations++
	}
	if totalDirectRelations == 0 {
		logger.Info().Msg("  ℹ️  No se encontraron relaciones directas entre los documentos")
	}

	return hasDirectRelation12, hasDirectRelation13, hasDirectRelation23
}

// checkDirectRelation checks if a direct relation exists and logs it.
func checkDirectRelation(logger zerolog.Logger, relationships []*entity.DocumentRelationship, targetDocID entity.DocumentID, label string) bool {
	for _, rel := range relationships {
		if rel.ToDocument == targetDocID {
			logger.Info().
				Str("type", string(rel.Type)).
				Float64("strength", rel.Strength).
				Msgf("  ✅ Relación directa encontrada: %s", label)
			return true
		}
	}
	return false
}

// verifyDocumentProjects verifies projects assigned to a document.
func verifyDocumentProjects(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	projectService *project.Service,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	docID entity.DocumentID,
	label string,
) ([]entity.ProjectID, *entity.FileMetadata) {
	meta, err := repos.metaRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
	if err != nil {
		return nil, nil
	}

	projects, err := repos.projectRepo.GetProjectsForDocument(ctx, workspaceID, docID)
	if err != nil {
		return nil, meta
	}

	logger.Info().
		Str("doc", entry.RelativePath).
		Int("contexts", len(meta.Contexts)).
		Int("projects", len(projects)).
		Msg(label)

	if len(meta.Contexts) > 0 {
		for _, projName := range meta.Contexts {
			proj, err := projectService.GetProjectByName(ctx, workspaceID, projName)
			if err == nil && proj != nil {
				docIDs, err := repos.projectRepo.GetDocuments(ctx, workspaceID, proj.ID, false)
				if err == nil {
					logger.Info().
						Str("project_id", proj.ID.String()).
						Str("project_name", projName).
						Int("document_count", len(docIDs)).
						Msgf(projectItemFormat, projName, proj.ID.String(), len(docIDs))
				}
			}
		}
	}

	return projects, meta
}

// verifySharedProjects verifies shared projects between documents.
func verifySharedProjects(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	projectService *project.Service,
	workspaceID entity.WorkspaceID,
	projects1, projects2, projects3 []entity.ProjectID,
) map[string]*entity.Project {
	logger.Info().Msg("")
	logger.Info().Msg("🔗 RELACIONES DE PROYECTOS:")
	sharedProjects := make(map[string]*entity.Project)

	findSharedProjects(ctx, logger, repos, projectService, workspaceID, projects1, projects2, sharedProjects, "Doc1, Doc2")
	findSharedProjects(ctx, logger, repos, projectService, workspaceID, projects1, projects3, sharedProjects, "Doc1, Doc3")
	findSharedProjects(ctx, logger, repos, projectService, workspaceID, projects2, projects3, sharedProjects, "Doc2, Doc3")

	if len(sharedProjects) == 0 {
		logger.Info().Msg("  ℹ️  Los documentos no comparten proyectos")
	}

	reportSharedProjectDocuments(ctx, logger, repos, workspaceID, sharedProjects)

	return sharedProjects
}

// findSharedProjects finds projects shared between two document sets.
func findSharedProjects(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	projectService *project.Service,
	workspaceID entity.WorkspaceID,
	projects1, projects2 []entity.ProjectID,
	sharedProjects map[string]*entity.Project,
	label string,
) {
	for _, projID1 := range projects1 {
		for _, projID2 := range projects2 {
			if projID1 == projID2 {
				proj, err := projectService.GetProject(ctx, workspaceID, projID1)
				if err == nil && proj != nil {
					if _, exists := sharedProjects[projID1.String()]; !exists {
						sharedProjects[projID1.String()] = proj
					}
					logger.Info().
						Str("project_id", proj.ID.String()).
						Str("project_name", proj.Name).
						Msgf("  ✅ Proyecto compartido (%s): %s", label, proj.Name)
				}
			}
		}
	}
}

// reportSharedProjectDocuments reports documents in shared projects.
func reportSharedProjectDocuments(
	ctx context.Context,
	logger zerolog.Logger,
	repos *testRepositories,
	workspaceID entity.WorkspaceID,
	sharedProjects map[string]*entity.Project,
) {
	for projIDStr, proj := range sharedProjects {
		projID := entity.ProjectID(projIDStr)
		docIDs, err := repos.projectRepo.GetDocuments(ctx, workspaceID, projID, false)
		if err != nil {
			continue
		}

		logger.Info().
			Str("project_name", proj.Name).
			Int("document_count", len(docIDs)).
			Msgf("  📁 Proyecto '%s' contiene %d documentos:", proj.Name, len(docIDs))

		for i, docID := range docIDs {
			if i >= 10 {
				logger.Info().Msgf("    ... (%d documentos más)", len(docIDs)-10)
				break
			}
			doc, err := repos.docRepo.GetDocument(ctx, workspaceID, docID)
			if err == nil && doc != nil {
				logger.Info().
					Str("doc_id", docID.String()).
					Str("doc_title", doc.Title).
					Msgf("    • %s", doc.Title)
			}
		}
	}
}

// verifySharedTags verifies shared tags between documents.
func verifySharedTags(
	logger zerolog.Logger,
	meta1, meta2, meta3 *entity.FileMetadata,
) {
	logger.Info().Msg("")
	logger.Info().Msg("🏷️  TAGS:")
	logger.Info().
		Strs("tags1", meta1.Tags).
		Msg(document1Label)
	logger.Info().
		Strs("tags2", meta2.Tags).
		Msg(document2Label)
	logger.Info().
		Strs("tags3", meta3.Tags).
		Msg(document3Label)

	sharedTags := findSharedTagsAll(meta1, meta2, meta3)
	if len(sharedTags) > 0 {
		logger.Info().
			Strs("shared_tags", sharedTags).
			Msg("  ✅ Tags compartidos entre los 3 documentos:")
		return
	}

	sharedTags12 := findSharedTagsPair(meta1, meta2)
	sharedTags13 := findSharedTagsPair(meta1, meta3)
	sharedTags23 := findSharedTagsPair(meta2, meta3)

	if len(sharedTags12) > 0 || len(sharedTags13) > 0 || len(sharedTags23) > 0 {
		if len(sharedTags12) > 0 {
			logger.Info().Strs("tags", sharedTags12).Msg("  ✅ Tags compartidos (Doc1, Doc2):")
		}
		if len(sharedTags13) > 0 {
			logger.Info().Strs("tags", sharedTags13).Msg("  ✅ Tags compartidos (Doc1, Doc3):")
		}
		if len(sharedTags23) > 0 {
			logger.Info().Strs("tags", sharedTags23).Msg("  ✅ Tags compartidos (Doc2, Doc3):")
		}
	} else {
		logger.Info().Msg("  ℹ️  No hay tags compartidos")
	}
}

// findSharedTagsAll finds tags shared by all three documents.
func findSharedTagsAll(meta1, meta2, meta3 *entity.FileMetadata) []string {
	tags1Map := make(map[string]bool)
	for _, tag := range meta1.Tags {
		tags1Map[strings.ToLower(tag)] = true
	}
	tags2Map := make(map[string]bool)
	for _, tag := range meta2.Tags {
		tags2Map[strings.ToLower(tag)] = true
	}

	var sharedTags []string
	for _, tag := range meta3.Tags {
		tagLower := strings.ToLower(tag)
		if tags1Map[tagLower] && tags2Map[tagLower] {
			sharedTags = append(sharedTags, tag)
		}
	}
	return sharedTags
}

// findSharedTagsPair finds tags shared between two documents.
func findSharedTagsPair(meta1, meta2 *entity.FileMetadata) []string {
	tags1Map := make(map[string]bool)
	for _, tag := range meta1.Tags {
		tags1Map[strings.ToLower(tag)] = true
	}

	var sharedTags []string
	for _, tag := range meta2.Tags {
		if tags1Map[strings.ToLower(tag)] {
			sharedTags = append(sharedTags, tag)
		}
	}
	return sharedTags
}

// reportFinalMetrics reports final test metrics.
func reportFinalMetrics(
	logger zerolog.Logger,
	processingTime1, processingTime2, processingTime3 time.Duration,
	relationships1, relationships2, relationships3 []*entity.DocumentRelationship,
	sharedProjects map[string]*entity.Project,
	sharedTags []string,
	totalDirectRelations int,
) {
	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("📈 MÉTRICAS FINALES")
	logger.Info().Msg(separatorLine)

	totalTime := processingTime1 + processingTime2 + processingTime3
	logger.Info().
		Dur("time1", processingTime1).
		Dur("time2", processingTime2).
		Dur("time3", processingTime3).
		Dur("total_time", totalTime).
		Float64("total_seconds", totalTime.Seconds()).
		Msg("  Tiempos de procesamiento:")

	logger.Info().
		Int("relationships1", len(relationships1)).
		Int("relationships2", len(relationships2)).
		Int("relationships3", len(relationships3)).
		Int("shared_projects", len(sharedProjects)).
		Int("shared_tags_all", len(sharedTags)).
		Int("direct_relations", totalDirectRelations).
		Msg("  Relaciones y conexiones:")
}

// TestVerbosePipelineTwoFiles procesa dos archivos PDF reales a través de todo el pipeline
// y verifica relaciones entre documentos, proyectos, y validaciones completas.
func TestVerbosePipelineTwoFiles(t *testing.T) {
	// Configurar logger verboso
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)

	logger.Info().Msg(separatorLine)
	logger.Info().Msg("🚀 TEST E2E VERBOSE - PROCESAMIENTO DE 3 ARCHIVOS PDF CON RELACIONES")
	logger.Info().Msg(separatorLine)

	// PASO 1: Configurar workspace y localizar archivos PDF
	logger.Info().Msg("")
	logger.Info().Msg("📁 PASO 1: Configurando workspace y localizando archivos PDF")

	absWorkspaceRoot := setupVerboseTestWorkspace(t, logger)
	// Buscar 3 archivos, priorizando "La Sabana Santa.pdf"
	pdfEntries := findPDFFiles(t, absWorkspaceRoot, logger, 3, "La Sabana Santa.pdf")
	if len(pdfEntries) < 3 {
		return
	}
	entry1, entry2, entry3 := pdfEntries[0], pdfEntries[1], pdfEntries[2]

	logger.Info().
		Str("file1", entry1.RelativePath).
		Int64("size1", entry1.FileSize).
		Str("file2", entry2.RelativePath).
		Int64("size2", entry2.FileSize).
		Str("file3", entry3.RelativePath).
		Int64("size3", entry3.FileSize).
		Msg("✅ FileEntries creados")

	// PASO 2: Configurar base de datos y repositorios
	workspaceID := entity.NewWorkspaceID()
	ctx := context.Background()
	repos := setupTestDatabase(t, ctx, absWorkspaceRoot, workspaceID, logger)
	defer repos.conn.Close()

	// Configurar contexto con workspace info
	wsInfo := contextinfo.WorkspaceInfo{
		ID:            workspaceID,
		Root:          absWorkspaceRoot,
		Config:        entity.WorkspaceConfig{},
		ForceFullScan: true,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	// PASO 3: Configurar el pipeline
	orchestrator := NewOrchestrator(nil, logger)
	llmRouter, realEmbedder := setupLLMAndEmbedder(t, ctx, logger, repos.traceRepo)
	projectService := setupPipelineStages(t, orchestrator, repos, llmRouter, realEmbedder, logger)

	logger.Info().Msg("✅ Pipeline configurado con todas las etapas")

	// PASO 4: Procesar primer archivo
	logger.Info().Msg("")
	logger.Info().Msg("🔄 PASO 4: Procesando primer archivo")
	processingTime1 := processFileThroughPipeline(t, ctx, logger, repos, orchestrator, entry1, workspaceID, "📄 Archivo 1:")

	// PASO 5: Procesar segundo archivo
	logger.Info().Msg("")
	logger.Info().Msg("🔄 PASO 5: Procesando segundo archivo")
	processingTime2 := processFileThroughPipeline(t, ctx, logger, repos, orchestrator, entry2, workspaceID, "📄 Archivo 2:")

	// PASO 5.5: Procesar tercer archivo
	logger.Info().Msg("")
	logger.Info().Msg("🔄 PASO 5.5: Procesando tercer archivo")
	processingTime3 := processFileThroughPipeline(t, ctx, logger, repos, orchestrator, entry3, workspaceID, "📄 Archivo 3:")

	// PASO 6: Verificar resultados y relaciones
	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("✅ PASO 6: Verificando resultados y relaciones")
	logger.Info().Msg(separatorLine)

	doc1, doc2, doc3, docID1, docID2, docID3 := verifyDocumentsCreated(t, ctx, logger, repos, workspaceID, entry1, entry2, entry3)

	// Verificar relaciones entre documentos
	logger.Info().Msg("")
	logger.Info().Msg("🔗 RELACIONES ENTRE DOCUMENTOS:")
	relationships1 := reportDocumentRelationships(ctx, logger, repos, workspaceID, docID1, doc1, "  Relaciones salientes del Documento 1:")
	relationships2 := reportDocumentRelationships(ctx, logger, repos, workspaceID, docID2, doc2, "  Relaciones salientes del Documento 2:")
	relationships3 := reportDocumentRelationships(ctx, logger, repos, workspaceID, docID3, doc3, document3Label+" Relaciones salientes")

	hasDirectRelation12, hasDirectRelation13, hasDirectRelation23 := verifyDirectRelations(logger, relationships1, relationships2, relationships3, docID1, docID2, docID3)
	totalDirectRelations := 0
	if hasDirectRelation12 {
		totalDirectRelations++
	}
	if hasDirectRelation13 {
		totalDirectRelations++
	}
	if hasDirectRelation23 {
		totalDirectRelations++
	}

	// Verificar proyectos asignados
	logger.Info().Msg("")
	logger.Info().Msg("📁 PROYECTOS ASIGNADOS:")
	projects1, meta1 := verifyDocumentProjects(ctx, logger, repos, projectService, workspaceID, entry1, docID1, document1Label)
	projects2, meta2 := verifyDocumentProjects(ctx, logger, repos, projectService, workspaceID, entry2, docID2, document2Label)
	projects3, meta3 := verifyDocumentProjects(ctx, logger, repos, projectService, workspaceID, entry3, docID3, document3Label)

	sharedProjects := verifySharedProjects(ctx, logger, repos, projectService, workspaceID, projects1, projects2, projects3)

	// Verificar tags compartidos
	sharedTags := findSharedTagsAll(meta1, meta2, meta3)
	verifySharedTags(logger, meta1, meta2, meta3)

	// Métricas finales
	reportFinalMetrics(logger, processingTime1, processingTime2, processingTime3, relationships1, relationships2, relationships3, sharedProjects, sharedTags, totalDirectRelations)

	logger.Info().Msg("")
	logger.Info().Msg(separatorLine)
	logger.Info().Msg("✅ TEST COMPLETADO EXITOSAMENTE")
	logger.Info().Msg(separatorLine)
}
