package stages

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
	"github.com/rs/zerolog"
)

// EnrichmentStage performs various enrichment techniques on documents.
type EnrichmentStage struct {
	llmRouter            *llm.Router
	metaRepo             repository.MetadataRepository
	docRepo              repository.DocumentRepository
	vectorStore          repository.VectorStore
	embedder             embedding.Embedder
	duplicateDetector    *metadata.DuplicateDetector
	ocrService           *metadata.OCRService
	tableExtractor       *metadata.TableExtractor
	formulaExtractor     *metadata.FormulaExtractor
	dependencyExtractor  *metadata.DependencyExtractor
	transcriptionService *metadata.TranscriptionService
	enrichmentService    *metadata.MetadataEnrichmentService
	logger               zerolog.Logger
	config               EnrichmentConfig
}

// EnrichmentConfig controls enrichment behavior.
type EnrichmentConfig struct {
	Enabled                   bool
	NEREnabled                bool
	CitationsEnabled          bool
	SentimentEnabled          bool
	OCREnabled                bool
	TablesEnabled             bool
	FormulasEnabled           bool
	DependenciesEnabled       bool
	TranscriptionEnabled      bool
	DuplicateDetectionEnabled bool
	ISBNEnrichmentEnabled     bool
}

// NewEnrichmentStage creates a new enrichment stage.
func NewEnrichmentStage(
	llmRouter *llm.Router,
	metaRepo repository.MetadataRepository,
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	logger zerolog.Logger,
	config EnrichmentConfig,
) *EnrichmentStage {
	stage := &EnrichmentStage{
		llmRouter:   llmRouter,
		metaRepo:    metaRepo,
		docRepo:     docRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		logger:      logger.With().Str("stage", "enrichment").Logger(),
		config:      config,
	}

	// Initialize services
	if config.DuplicateDetectionEnabled {
		stage.duplicateDetector = metadata.NewDuplicateDetector(
			vectorStore,
			docRepo,
			embedder,
			logger,
			0.85,
		)
	}

	if config.OCREnabled {
		stage.ocrService = metadata.NewOCRService(logger)
	}

	if config.TablesEnabled {
		stage.tableExtractor = metadata.NewTableExtractor(logger)
	}

	if config.FormulasEnabled {
		stage.formulaExtractor = metadata.NewFormulaExtractor(logger)
	}

	if config.DependenciesEnabled {
		stage.dependencyExtractor = metadata.NewDependencyExtractor(logger)
	}

	if config.TranscriptionEnabled {
		stage.transcriptionService = metadata.NewTranscriptionService(logger)
	}

	if config.ISBNEnrichmentEnabled {
		stage.enrichmentService = metadata.NewMetadataEnrichmentService(logger)
	}

	return stage
}

// Name returns the stage name.
func (s *EnrichmentStage) Name() string {
	return "enrichment"
}

// CanProcess returns true if enrichment is enabled.
func (s *EnrichmentStage) CanProcess(entry *entity.FileEntry) bool {
	return s.config.Enabled
}

// Process performs enrichment on the document.
func (s *EnrichmentStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.config.Enabled {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return nil
	}

	// Get file metadata
	if s.metaRepo == nil {
		s.logger.Debug().Str("path", entry.RelativePath).Msg("Metadata repository not available, skipping enrichment")
		return nil
	}
	fileMeta, err := s.metaRepo.GetByPath(ctx, wsInfo.ID, entry.RelativePath)
	if err != nil || fileMeta == nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("File metadata not found, skipping enrichment")
		return nil
	}

	// Initialize enrichment data
	if fileMeta.EnrichmentData == nil {
		fileMeta.EnrichmentData = &entity.EnrichmentData{
			ExtractedAt: time.Now(),
		}
	}

	enrichment := fileMeta.EnrichmentData

	// Get document content
	if s.docRepo == nil {
		s.logger.Debug().Str("path", entry.RelativePath).Msg("Document repository not available, skipping enrichment")
		return nil
	}
	doc, err := s.docRepo.GetDocumentByPath(ctx, wsInfo.ID, entry.RelativePath)
	if err != nil || doc == nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("Document not found, skipping enrichment")
		return nil
	}

	// Get document chunks for content
	chunks, err := s.docRepo.GetChunksByDocument(ctx, wsInfo.ID, doc.ID)
	if err != nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("Failed to get chunks")
		return nil
	}

	// Build content from chunks
	content := ""
	for _, chunk := range chunks {
		content += chunk.Text + "\n\n"
	}

	// Get summary if available
	summary := ""
	if fileMeta.AISummary != nil {
		summary = fileMeta.AISummary.Summary
	}

	// 1. Named Entity Recognition
	if s.config.NEREnabled && s.llmRouter != nil {
		entities, err := s.llmRouter.ExtractNamedEntities(ctx, content, summary)
		if err == nil {
			enrichment.NamedEntities = entities
			s.logger.Debug().Int("entities", len(entities)).Str("path", entry.RelativePath).Msg("Extracted named entities")
		} else {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to extract named entities")
		}
	}

	// 2. Citation Extraction
	if s.config.CitationsEnabled && s.llmRouter != nil {
		citations, err := s.llmRouter.ExtractCitations(ctx, content, summary)
		if err == nil {
			enrichment.Citations = citations
			s.logger.Debug().Int("citations", len(citations)).Str("path", entry.RelativePath).Msg("Extracted citations")
		} else {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to extract citations")
		}
	}

	// 3. Sentiment Analysis
	if s.config.SentimentEnabled && s.llmRouter != nil {
		sentiment, err := s.llmRouter.AnalyzeSentiment(ctx, content, summary)
		if err == nil {
			enrichment.Sentiment = sentiment
			s.logger.Debug().Str("sentiment", sentiment.OverallSentiment).Str("path", entry.RelativePath).Msg("Analyzed sentiment")
		} else {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to analyze sentiment")
		}
	}

	// 4. OCR (for images and scanned PDFs)
	if s.config.OCREnabled && s.ocrService != nil {
		if s.ocrService.IsAvailable() {
			ocrDir := filepath.Join(wsInfo.Root, ".cortex", "mirror")
			// Check if file is an image
			if isImage(entry.Extension) {
				ocrResult, err := s.ocrService.ExtractTextFromImage(ctx, entry.AbsolutePath, "", ocrDir)
				if err == nil {
					enrichment.OCRText = ocrResult
					s.logger.Debug().Str("path", entry.RelativePath).Msg("Extracted text from image via OCR")
				}
			} else if entry.Extension == ".pdf" {
				// Try OCR on PDF (might be scanned)
				ocrResult, err := s.ocrService.ExtractTextFromPDF(ctx, entry.AbsolutePath, "", ocrDir)
				if err == nil && ocrResult.Text != "" {
					enrichment.OCRText = ocrResult
					s.logger.Debug().Str("path", entry.RelativePath).Msg("Extracted text from PDF via OCR")
				}
			}
		}
	}

	// 5. Table Extraction
	if s.config.TablesEnabled && s.tableExtractor != nil {
		if entry.Extension == ".pdf" {
			tables, err := s.tableExtractor.ExtractTables(ctx, entry.AbsolutePath)
			if err == nil {
				enrichment.Tables = tables
				s.logger.Debug().Int("tables", len(tables)).Str("path", entry.RelativePath).Msg("Extracted tables")
			}
		} else if entry.Extension == ".md" {
			tables, err := s.tableExtractor.ExtractTablesFromMarkdown(ctx, content)
			if err == nil {
				enrichment.Tables = tables
				s.logger.Debug().Int("tables", len(tables)).Str("path", entry.RelativePath).Msg("Extracted tables from markdown")
			}
		}
	}

	// 6. Formula Extraction
	if s.config.FormulasEnabled && s.formulaExtractor != nil {
		formulas, err := s.formulaExtractor.ExtractFormulas(ctx, content)
		if err == nil {
			enrichment.Formulas = formulas
			s.logger.Debug().Int("formulas", len(formulas)).Str("path", entry.RelativePath).Msg("Extracted formulas")
		}
	}

	// 7. Dependency Extraction (for code files)
	if s.config.DependenciesEnabled && s.dependencyExtractor != nil {
		if isCodeFile(entry.Extension) {
			deps, err := s.dependencyExtractor.ExtractDependencies(ctx, entry.AbsolutePath, content)
			if err == nil {
				enrichment.Dependencies = deps
				s.logger.Debug().Int("dependencies", len(deps)).Str("path", entry.RelativePath).Msg("Extracted dependencies")
			}
		}
	}

	// 8. Transcription (for audio/video files)
	if s.config.TranscriptionEnabled && s.transcriptionService != nil {
		if isAudioFile(entry.Extension) {
			transcription, err := s.transcriptionService.TranscribeAudio(ctx, entry.AbsolutePath, "")
			if err == nil {
				enrichment.Transcription = transcription
				s.logger.Debug().Str("path", entry.RelativePath).Msg("Transcribed audio")
			}
		} else if isVideoFile(entry.Extension) {
			transcription, err := s.transcriptionService.TranscribeVideo(ctx, entry.AbsolutePath, "")
			if err == nil {
				enrichment.Transcription = transcription
				s.logger.Debug().Str("path", entry.RelativePath).Msg("Transcribed video")
			}
		}
	}

	// 9. Duplicate Detection
	if s.config.DuplicateDetectionEnabled && s.duplicateDetector != nil {
		duplicates, err := s.duplicateDetector.FindDuplicates(ctx, wsInfo.ID, doc.ID, entry.RelativePath)
		if err == nil {
			enrichment.Duplicates = duplicates
			s.logger.Debug().Int("duplicates", len(duplicates)).Str("path", entry.RelativePath).Msg("Found duplicates")
		}
	}

	// 10. ISBN Enrichment (if AIContext has ISBN)
	if s.config.ISBNEnrichmentEnabled && s.enrichmentService != nil && fileMeta.AIContext != nil {
		if fileMeta.AIContext.ISBN != nil {
			enriched, err := s.enrichmentService.EnrichAIContext(ctx, fileMeta.AIContext)
			if err == nil {
				s.logger.Debug().Str("path", entry.RelativePath).Str("source", enriched.Source).Msg("Enriched metadata with ISBN")
			}
		}
	}

	// Save enrichment data to metadata
	fileMeta.EnrichmentData = enrichment

	// Persist enrichment data to database
	if err := s.metaRepo.UpdateEnrichmentData(ctx, wsInfo.ID, entry.ID, enrichment); err != nil {
		s.logger.Warn().
			Err(err).
			Str("path", entry.RelativePath).
			Msg("Failed to persist enrichment data")
	} else {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Int("named_entities", len(enrichment.NamedEntities)).
			Int("citations", len(enrichment.Citations)).
			Int("tables", len(enrichment.Tables)).
			Int("formulas", len(enrichment.Formulas)).
			Msg("Enrichment data generated and persisted")
	}

	return nil
}

// Helper functions
func isImage(ext string) bool {
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tiff", ".webp"}
	ext = strings.ToLower(ext)
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

func isCodeFile(ext string) bool {
	codeExts := []string{".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".java", ".rb", ".rs", ".cpp", ".c", ".h"}
	ext = strings.ToLower(ext)
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

func isAudioFile(ext string) bool {
	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a"}
	ext = strings.ToLower(ext)
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

func isVideoFile(ext string) bool {
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".webm", ".flv"}
	ext = strings.ToLower(ext)
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}
