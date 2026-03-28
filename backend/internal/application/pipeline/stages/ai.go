// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// AIStage generates summaries/tags/projects using the active LLM provider.
type AIStage struct {
	llmRouter      *llm.Router
	metaRepo       repository.MetadataRepository
	suggestedRepo  repository.SuggestedMetadataRepository // For accessing suggested metadata
	fileRepo       repository.FileRepository
	docRepo        repository.DocumentRepository
	vectorStore    repository.VectorStore
	embedder       embedding.Embedder
	projectRepo    repository.ProjectRepository
	assignRepo     repository.ProjectAssignmentRepository
	projectService *project.Service
	logger         zerolog.Logger
	config         AIStageConfig
}

// AIStageConfig controls backend AI indexing behavior.
type AIStageConfig struct {
	Enabled                bool
	AutoSummaryEnabled     bool
	AutoIndexEnabled       bool
	ApplyTags              bool
	ApplyProjects          bool
	UseSuggestedContexts   bool
	MaxFileSize            int64
	MaxTags                int
	RequestTimeout         time.Duration
	MustSucceed            bool
	CategoryEnabled        bool
	RelatedEnabled         bool
	RelatedMaxResults      int
	RelatedCandidates      int
	UseRAGForCategories    bool
	UseRAGForTags          bool
	UseRAGForProjects      bool
	UseRAGForRelated       bool
	UseRAGForSummary       bool
	RAGSimilarityThreshold float32
}

// NewAIStage creates a new AI stage without RAG support.
func NewAIStage(
	llmRouter *llm.Router,
	metaRepo repository.MetadataRepository,
	suggestedRepo repository.SuggestedMetadataRepository,
	fileRepo repository.FileRepository,
	projectRepo repository.ProjectRepository,
	assignRepo repository.ProjectAssignmentRepository,
	projectService *project.Service,
	logger zerolog.Logger,
	config AIStageConfig,
) *AIStage {
	return &AIStage{
		llmRouter:      llmRouter,
		metaRepo:       metaRepo,
		suggestedRepo:  suggestedRepo,
		fileRepo:       fileRepo,
		projectRepo:    projectRepo,
		assignRepo:     assignRepo,
		projectService: projectService,
		logger:         logger.With().Str("stage", "ai").Logger(),
		config:         config,
	}
}

// NewAIStageWithRAG creates a new AI stage with RAG support.
func NewAIStageWithRAG(
	llmRouter *llm.Router,
	metaRepo repository.MetadataRepository,
	suggestedRepo repository.SuggestedMetadataRepository,
	fileRepo repository.FileRepository,
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	projectRepo repository.ProjectRepository,
	assignRepo repository.ProjectAssignmentRepository,
	projectService *project.Service,
	logger zerolog.Logger,
	config AIStageConfig,
) *AIStage {
	return &AIStage{
		llmRouter:      llmRouter,
		metaRepo:       metaRepo,
		suggestedRepo:  suggestedRepo,
		fileRepo:       fileRepo,
		docRepo:        docRepo,
		vectorStore:    vectorStore,
		embedder:       embedder,
		projectRepo:    projectRepo,
		assignRepo:     assignRepo,
		projectService: projectService,
		logger:         logger.With().Str("stage", "ai").Logger(),
		config:         config,
	}
}

// Name returns the stage name.
func (s *AIStage) Name() string {
	return "ai"
}

// CanProcess returns true if AI is enabled.
func (s *AIStage) CanProcess(entry *entity.FileEntry) bool {
	return s.config.Enabled && s.metaRepo != nil && s.llmRouter != nil
}

// Process runs AI summary/tag/project extraction.
func (s *AIStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Bool("enabled", s.config.Enabled).
			Bool("has_meta_repo", s.metaRepo != nil).
			Bool("has_llm_router", s.llmRouter != nil).
			Msg("AI stage cannot process file (CanProcess returned false)")
		return nil
	}
	if entry == nil || entry.FileSize <= 0 {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Int64("file_size", entry.FileSize).
			Msg("AI stage skipping file (nil entry or zero size)")
		return nil
	}
	// For files with mirror formats (PDFs, Office docs), check mirror file size instead of original
	// The mirror file is the extracted text content, which is what we actually process
	var sizeToCheck int64 = entry.FileSize
	ext := strings.ToLower(entry.Extension)
	if format, ok := mirrorFormats[ext]; ok {
		wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
		if ok {
			mirrorPath := filepath.Join(wsInfo.Root, ".cortex", "mirror", filepath.FromSlash(entry.RelativePath)+"."+format)
			if mirrorInfo, err := os.Stat(mirrorPath); err == nil {
				sizeToCheck = mirrorInfo.Size()
			}
		}
	}

	if s.config.MaxFileSize > 0 && sizeToCheck > s.config.MaxFileSize {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Int64("file_size", entry.FileSize).
			Int64("size_checked", sizeToCheck).
			Int64("max_file_size", s.config.MaxFileSize).
			Msg("AI stage skipping file (exceeds MaxFileSize)")
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		s.logger.Debug().Str("path", entry.RelativePath).Msg("Workspace info missing; skipping AI stage")
		return nil
	}

	ragAvailable, _ := s.canUseRAG() // Check RAG availability (error logged in canUseRAG)
	s.logger.Info().
		Str("path", entry.RelativePath).
		Bool("rag_available", ragAvailable).
		Bool("llm_available", s.llmRouter.IsAvailable(ctx)).
		Msg("AI stage starting")

	traceBase := llm.TraceInfo{
		WorkspaceID:   wsInfo.ID.String(),
		WorkspaceRoot: wsInfo.Root,
		FileID:        entry.ID.String(),
		RelativePath:  entry.RelativePath,
		Stage:         "ai",
	}
	traceCtx := func(op string) context.Context {
		info := traceBase
		info.Operation = op
		return llm.WithTraceInfo(ctx, info)
	}

	// Scale timeout based on file size to avoid long multi-step AI runs timing out.
	timeout := s.config.RequestTimeout
	if entry.FileSize > 0 {
		estimated := time.Duration(entry.FileSize/20000) * time.Second
		if estimated < timeout {
			estimated = timeout
		}
		if estimated > 5*time.Minute {
			estimated = 5 * time.Minute
		}
		timeout = estimated
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !s.llmRouter.IsAvailable(ctx) {
		if s.config.MustSucceed {
			s.logger.Warn().
				Str("path", entry.RelativePath).
				Bool("llm_enabled", s.config.Enabled).
				Bool("auto_index_enabled", s.config.AutoIndexEnabled).
				Msg("LLM unavailable; waiting for provider to recover (must_succeed=true)")
			backoff := 2 * time.Second
			for {
				if s.llmRouter.IsAvailable(context.Background()) {
					break
				}
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
			}
		} else {
			err := fmt.Errorf("CRITICAL: LLM not available - AI stage requires LLM to be available")
			s.logger.Error().
				Err(err).
				Str("path", entry.RelativePath).
				Bool("llm_enabled", s.config.Enabled).
				Bool("auto_index_enabled", s.config.AutoIndexEnabled).
				Msg("CRITICAL: LLM not available; AI stage cannot proceed")
			return err
		}
	}

	content, source, err := resolveAIContent(wsInfo.Root, entry)
	if err != nil {
		if errors.Is(err, errNoContent) {
			s.logger.Info().
				Str("path", entry.RelativePath).
				Str("extension", entry.Extension).
				Str("source", source).
				Str("workspace_root", wsInfo.Root).
				Msg("No AI content available for file (skipping AI processing) - file is not text-based and has no mirror")
			return nil
		}
		// Check if error is about mirror file not found
		if strings.Contains(err.Error(), "mirror file not found") {
			s.logger.Info().
				Err(err).
				Str("path", entry.RelativePath).
				Str("extension", entry.Extension).
				Str("workspace_root", wsInfo.Root).
				Msg("Mirror file not found - MirrorStage may not have run yet or file type not supported")
			return nil // Don't fail, just skip AI processing for this file
		}
		s.logger.Warn().
			Err(err).
			Str("path", entry.RelativePath).
			Str("extension", entry.Extension).
			Msg("Failed to resolve AI content")
		return err
	}

	s.logger.Debug().
		Str("path", entry.RelativePath).
		Str("source", source).
		Int("content_length", len(content)).
		Msg("Resolved AI content successfully")

	contentHash := hashContent(content)
	meta, err := s.metaRepo.GetOrCreate(ctx, wsInfo.ID, entry.RelativePath, entry.Extension)
	if err != nil {
		return err
	}

	// Track if we regenerated the summary so we can use the new values for subsequent operations
	var currentSummary string
	var currentDescription string
	var detectedLanguage string

	// Detect and store language if not already detected or if forceFullScan
	var languageConfidence float64
	if meta.DetectedLanguage == nil || wsInfo.ForceFullScan {
		// Use LLM to detect language (with heuristic fallback)
		langCode, err := s.llmRouter.DetectLanguage(traceCtx("language"), content)
		if err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Language detection failed, using heuristic")
			// Fallback to heuristic - check for Spanish and Portuguese characters
			contentLower := strings.ToLower(content)
			hasSpanishChars := strings.ContainsAny(contentLower, "áéíóúñü¿¡")
			hasPortugueseChars := strings.ContainsAny(contentLower, "áéíóúãõâêôçàü")
			if hasSpanishChars {
				langCode = "es"
			} else if hasPortugueseChars {
				langCode = "pt"
			} else {
				langCode = "en"
			}
			// Heuristic detection has lower confidence
			languageConfidence = 0.5
		} else {
			// LLM detection has higher confidence
			languageConfidence = 0.9
		}
		detectedLanguage = langCode

		// Store detected language and confidence in metadata
		if err := s.metaRepo.UpdateDetectedLanguage(ctx, wsInfo.ID, entry.ID, langCode); err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Str("language", langCode).Msg("Failed to store detected language")
		} else {
			s.logger.Info().Str("path", entry.RelativePath).Str("language", langCode).Float64("confidence", languageConfidence).Msg("Language detected and stored")
		}

		// Store language confidence in EnhancedMetadata
		if entry.Enhanced == nil {
			entry.Enhanced = &entity.EnhancedMetadata{}
		}
		entry.Enhanced.LanguageConfidence = &languageConfidence
	} else {
		// Use stored language
		detectedLanguage = *meta.DetectedLanguage
		// If we have stored confidence, use it; otherwise assume medium confidence for stored values
		if entry.Enhanced != nil && entry.Enhanced.LanguageConfidence != nil {
			languageConfidence = *entry.Enhanced.LanguageConfidence
		} else {
			languageConfidence = 0.7 // Medium confidence for previously stored values
		}
		s.logger.Debug().Str("path", entry.RelativePath).Str("language", detectedLanguage).Float64("confidence", languageConfidence).Msg("Using stored language")
	}

	if s.config.AutoSummaryEnabled {
		// Force regeneration if forceFullScan is true, otherwise check hash
		needsSummary := wsInfo.ForceFullScan || meta.AISummary == nil || meta.AISummary.ContentHash != contentHash
		if needsSummary {
			var summary string
			var err error

			if s.config.UseRAGForSummary {
				ragAvailable, err := s.canUseRAG()
				if err != nil {
					return err
				}
				if ragAvailable {
					s.logger.Info().
						Str("path", entry.RelativePath).
						Msg("Using RAG for summary generation")
					summary, err = s.generateSummaryWithRAG(ctx, wsInfo, content, 400, traceCtx)
				} else {
					summary, err = s.llmRouter.GenerateSummary(traceCtx("summary"), content, 400)
				}
			} else {
				summary, err = s.llmRouter.GenerateSummary(traceCtx("summary"), content, 400)
			}

			if err != nil {
				s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("AI summary failed")
			} else if summary != "" {
				// Generate key terms using LLM from summary (better quality) or fallback to extraction from summary
				var keyTerms []string
				if s.llmRouter != nil && s.llmRouter.IsAvailable(ctx) {
					keyTerms, err = s.generateKeyTermsFromSummary(ctx, summary, traceCtx)
					if err != nil {
						s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("LLM key terms generation failed, using extraction from summary")
						// Extract from summary instead of raw content (summary is more representative)
						keyTerms = extractKeyTerms(summary, 12)
					}
				} else {
					// Extract from summary instead of raw content
					keyTerms = extractKeyTerms(summary, 12)
				}
				err = s.metaRepo.UpdateAISummary(ctx, wsInfo.ID, entry.ID, entity.AISummary{
					Summary:     summary,
					ContentHash: contentHash,
					KeyTerms:    keyTerms,
					GeneratedAt: time.Now(),
				})
				if err != nil {
					s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store AI summary")
				} else {
					// Store new summary and description for use in subsequent operations
					currentSummary = summary
					if len(keyTerms) > 0 {
						currentDescription = strings.Join(keyTerms, ", ")
					}
					// Reload meta to get updated AISummary
					meta, err = s.metaRepo.GetOrCreate(ctx, wsInfo.ID, entry.RelativePath, entry.Extension)
					if err != nil {
						s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to reload metadata after summary update")
					}
				}
			}
		} else if meta.AISummary != nil && len(meta.AISummary.KeyTerms) == 0 {
			// Generate key terms using LLM from summary (better quality) or fallback to extraction
			var keyTerms []string
			var err error
			if meta.AISummary.Summary != "" && s.llmRouter != nil && s.llmRouter.IsAvailable(ctx) {
				keyTerms, err = s.generateKeyTermsFromSummary(ctx, meta.AISummary.Summary, traceCtx)
				if err != nil {
					s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("LLM key terms generation failed, using extraction from summary")
					// Extract from summary instead of raw content
					keyTerms = extractKeyTerms(meta.AISummary.Summary, 12)
				}
			} else if meta.AISummary.Summary != "" {
				// Extract from summary if available
				keyTerms = extractKeyTerms(meta.AISummary.Summary, 12)
			} else {
				// Fallback to content only if no summary available
				keyTerms = extractKeyTerms(content, 12)
			}
			if len(keyTerms) > 0 {
				generatedAt := meta.AISummary.GeneratedAt
				if generatedAt.IsZero() {
					generatedAt = time.Now()
				}
				err = s.metaRepo.UpdateAISummary(ctx, wsInfo.ID, entry.ID, entity.AISummary{
					Summary:     meta.AISummary.Summary,
					ContentHash: contentHash,
					KeyTerms:    keyTerms,
					GeneratedAt: generatedAt,
				})
				if err == nil {
					// Store for use in subsequent operations
					currentSummary = meta.AISummary.Summary
					if len(keyTerms) > 0 {
						currentDescription = strings.Join(keyTerms, ", ")
					}
					// Reload meta to get updated AISummary
					meta, _ = s.metaRepo.GetOrCreate(ctx, wsInfo.ID, entry.RelativePath, entry.Extension)
				}
			}
		}

		// Use current summary/description if we just generated them, otherwise use from meta
		if currentSummary == "" && meta.AISummary != nil {
			currentSummary = meta.AISummary.Summary
		}
		if currentDescription == "" && meta.AISummary != nil && len(meta.AISummary.KeyTerms) > 0 {
			currentDescription = strings.Join(meta.AISummary.KeyTerms, ", ")
		}
	}

	// Extract contextual information (AIContext) if we have summary
	if currentSummary != "" && s.llmRouter != nil {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Msg("Extracting contextual information (AIContext)")

		// Get RAG context snippets for better extraction
		var contextSnippets []string
		if s.config.UseRAGForSummary {
			ragAvailable, err := s.canUseRAG()
			if err == nil && ragAvailable {
				// Use RAG to find similar documents for context
				embeddingText := truncateForEmbedding(content, 2000)
				if vector, err := s.embedder.Embed(ctx, embeddingText); err == nil {
					if matches, err := s.vectorStore.Search(ctx, wsInfo.ID, vector, 3); err == nil {
						for _, match := range matches {
							if chunks, err := s.docRepo.GetChunksByIDs(ctx, wsInfo.ID, []entity.ChunkID{match.ChunkID}); err == nil && len(chunks) > 0 {
								chunkText := chunks[0].Text
								if len(chunkText) > 300 {
									chunkText = chunkText[:300] + "..."
								}
								contextSnippets = append(contextSnippets, chunkText)
							}
						}
					}
				}
			}
		}

		// Extract contextual information
		// Pass file last modified date for publication year validation
		fileLastModified := &entry.LastModified
		aiContext, err := s.llmRouter.ExtractContextualInfoParsed(
			traceCtx("contextual_info"),
			content,
			currentSummary,
			currentDescription,
			contextSnippets,
			fileLastModified,
			detectedLanguage,
		)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("path", entry.RelativePath).
				Msg("Failed to extract contextual information")
		} else if aiContext != nil && aiContext.HasAnyData() {
			// Store AIContext in metadata and persist it
			meta.AIContext = aiContext
			if err := s.metaRepo.UpdateAIContext(ctx, wsInfo.ID, entry.ID, aiContext); err != nil {
				s.logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Msg("Failed to persist AIContext")
			} else {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Int("authors", len(aiContext.Authors)).
					Int("locations", len(aiContext.Locations)).
					Int("people", len(aiContext.PeopleMentioned)).
					Int("organizations", len(aiContext.Organizations)).
					Int("events", len(aiContext.HistoricalEvents)).
					Int("references", len(aiContext.References)).
					Msg("Contextual information extracted and persisted successfully")
			}
		}
	}

	// Log AI stage configuration for debugging
	s.logger.Info().
		Str("path", entry.RelativePath).
		Bool("enabled", s.config.Enabled).
		Bool("auto_index_enabled", s.config.AutoIndexEnabled).
		Bool("auto_summary_enabled", s.config.AutoSummaryEnabled).
		Bool("apply_tags", s.config.ApplyTags).
		Bool("apply_projects", s.config.ApplyProjects).
		Int64("max_file_size", s.config.MaxFileSize).
		Int64("file_size", entry.FileSize).
		Msg("AI stage configuration check")

	if !s.config.AutoIndexEnabled {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Bool("auto_index_enabled", s.config.AutoIndexEnabled).
			Msg("AutoIndexEnabled is false, skipping AI indexing")
		return nil
	}

	if s.config.ApplyTags {
		maxTags := s.config.MaxTags
		if maxTags <= 0 {
			maxTags = 8
		}

		// Use current summary/description (from regeneration if available, otherwise from meta)
		summary := currentSummary
		description := currentDescription
		if summary == "" && meta.AISummary != nil && meta.AISummary.Summary != "" {
			summary = meta.AISummary.Summary
		}
		if description == "" && meta.AISummary != nil && len(meta.AISummary.KeyTerms) > 0 {
			description = strings.Join(meta.AISummary.KeyTerms, ", ")
		}

		var tags []string
		if s.config.UseRAGForTags {
			var err error
			ragAvailable, err := s.canUseRAG()
			if err != nil {
				return err
			}
			if ragAvailable {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Msg("Using RAG for tag suggestion")
				tags, err = s.suggestTagsWithRAG(ctx, wsInfo, content, summary, description, maxTags, traceCtx)
			} else {
				if summary != "" {
					tags, err = s.llmRouter.SuggestTagsWithContextAndSummary(traceCtx("tags"), summary, description, content, maxTags, []string{})
				} else {
					tags, err = s.llmRouter.SuggestTags(traceCtx("tags"), content, maxTags)
				}
			}
			if err != nil {
				s.logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Int("existing_tags", len(meta.Tags)).
					Msg("AI tag suggestion failed")
			}
		}

		if len(tags) > 0 {
			existingTags := map[string]struct{}{}
			for _, tag := range meta.Tags {
				normalized := entity.NormalizeTag(tag)
				if normalized != "" {
					existingTags[normalized] = struct{}{}
				}
			}
			s.logger.Debug().
				Str("path", entry.RelativePath).
				Int("suggested_tags", len(tags)).
				Int("existing_tags", len(meta.Tags)).
				Msg("Processing suggested tags")
			tagsAdded := 0
			for _, tag := range tags {
				normalized := entity.NormalizeTag(tag)
				if normalized == "" {
					continue
				}
				if entity.IsTagGeneric(normalized) {
					s.logger.Debug().
						Str("path", entry.RelativePath).
						Str("tag", normalized).
						Msg("Tag too generic, skipping")
					continue
				}
				if _, exists := existingTags[normalized]; exists {
					s.logger.Debug().
						Str("path", entry.RelativePath).
						Str("tag", normalized).
						Msg("Tag already exists, skipping")
					continue
				}
				if isTagSimilarToAny(normalized, existingTags) {
					s.logger.Debug().
						Str("path", entry.RelativePath).
						Str("tag", normalized).
						Msg("Tag is similar to existing tag, skipping")
					continue
				}
				// Use a separate context with longer timeout for database operations
				// This prevents "context deadline exceeded" errors during heavy load
				dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
				if err := s.metaRepo.AddTag(dbCtx, wsInfo.ID, entry.ID, normalized); err != nil {
					s.logger.Warn().
						Err(err).
						Str("path", entry.RelativePath).
						Str("tag", normalized).
						Msg("Failed to store AI tag")
				} else {
					tagsAdded++
					existingTags[normalized] = struct{}{}
					s.logger.Debug().
						Str("path", entry.RelativePath).
						Str("tag", normalized).
						Msg("Successfully stored AI tag")
				}
				dbCancel()
			}
			if tagsAdded > 0 {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Int("tags_added", tagsAdded).
					Int("total_suggested", len(tags)).
					Msg("AI tags added to file")
			}
		}
	}

	if s.config.ApplyProjects {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Bool("apply_projects", s.config.ApplyProjects).
			Bool("auto_index_enabled", s.config.AutoIndexEnabled).
			Bool("has_project_service", s.projectService != nil).
			Bool("has_project_repo", s.projectRepo != nil).
			Str("workspace_id", wsInfo.ID.String()).
			Msg("Processing project suggestions using all metadata")

		// Collect projects from all sources:
		// 1. Confirmed contexts
		// 2. Suggested contexts
		// 3. Suggested metadata (from SuggestionStage)
		// 4. LLM suggestions (existing logic)
		// 5. Metadata-based inference (tags, taxonomy, etc.)

		// Get all project candidates from multiple sources
		projectCandidates := s.collectAllProjectCandidates(ctx, wsInfo, entry, meta, currentSummary, currentDescription)

		// Get confirmed contexts for LLM context
		// Use a separate context with longer timeout for database operations
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		projects, err := s.metaRepo.GetAllContexts(dbCtx, wsInfo.ID)
		dbCancel()
		if err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to load contexts")
			projects = []string{}
		}

		autoThreshold := 0.7
		suggestedThreshold := 0.5
		hasCandidate := false

		existingAssignments := map[string]*entity.ProjectAssignment{}
		if s.assignRepo != nil {
			// Use a separate context with longer timeout for database operations
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			assignments, err := s.assignRepo.ListByFile(dbCtx, wsInfo.ID, entry.ID)
			dbCancel()
			if err != nil {
				s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to load existing project assignments")
			} else {
				for _, assignment := range assignments {
					existingAssignments[assignment.ProjectName] = assignment
				}
			}
		}

		for _, candidate := range projectCandidates {
			if candidate.Score < suggestedThreshold {
				continue
			}
			hasCandidate = true

			status := entity.ProjectAssignmentSuggested
			if candidate.Score >= autoThreshold {
				status = entity.ProjectAssignmentAuto
			}

			var projectID entity.ProjectID
			if candidate.ProjectID != "" {
				projectID = candidate.ProjectID
			}

			existing := existingAssignments[candidate.ProjectName]
			if existing != nil {
				switch existing.Status {
				case entity.ProjectAssignmentManual, entity.ProjectAssignmentRejected:
					status = existing.Status
					if existing.ProjectID != "" {
						projectID = existing.ProjectID
					}
				}
			}

			if status == entity.ProjectAssignmentAuto && existing != nil && existing.Status == entity.ProjectAssignmentRejected {
				status = entity.ProjectAssignmentRejected
			}

			if status == entity.ProjectAssignmentAuto && s.projectService != nil && s.projectRepo != nil {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Str("project", candidate.ProjectName).
					Float64("score", candidate.Score).
					Strs("sources", candidate.Sources).
					Msg("Assigning project from metadata candidate")
				assignedID, err := s.assignProjectToFile(ctx, wsInfo, entry, meta, candidate.ProjectName, projectID)
				if err != nil {
					s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to assign project from metadata")
					status = entity.ProjectAssignmentSuggested
				} else if assignedID != "" {
					projectID = assignedID
				}
			}

			if projectID == "" && s.projectService != nil {
				if proj, err := s.projectService.GetProjectByName(ctx, wsInfo.ID, candidate.ProjectName); err == nil && proj != nil {
					projectID = proj.ID
				}
			}

			if s.assignRepo != nil {
				assignment := &entity.ProjectAssignment{
					WorkspaceID: wsInfo.ID,
					FileID:      entry.ID,
					ProjectID:   projectID,
					ProjectName: candidate.ProjectName,
					Score:       candidate.Score,
					Sources:     candidate.Sources,
					Status:      status,
				}
				if err := s.assignRepo.Upsert(ctx, assignment); err != nil {
					s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store project assignment")
				}
			}
		}

		// Fall back to LLM suggestion if no metadata candidates were strong enough
		if err == nil && !hasCandidate {
			s.logger.Debug().
				Str("path", entry.RelativePath).
				Int("existing_projects_count", len(projects)).
				Msg("Loaded existing projects for suggestion")
			// Use current summary/description (from regeneration if available, otherwise from meta)
			summary := currentSummary
			description := currentDescription
			if summary == "" && meta.AISummary != nil && meta.AISummary.Summary != "" {
				summary = meta.AISummary.Summary
			}
			if description == "" && meta.AISummary != nil && len(meta.AISummary.KeyTerms) > 0 {
				description = strings.Join(meta.AISummary.KeyTerms, ", ")
			}

			var project string
			if s.config.UseRAGForProjects {
				ragAvailable, err := s.canUseRAG()
				if err != nil {
					return err
				}
				if ragAvailable {
					s.logger.Info().
						Str("path", entry.RelativePath).
						Int("existing_projects", len(projects)).
						Bool("has_summary", summary != "").
						Bool("has_description", description != "").
						Msg("Using RAG for project suggestion")
					project, err = s.suggestProjectWithRAG(ctx, wsInfo, entry, content, summary, description, projects, traceCtx)
				} else {
					s.logger.Info().
						Str("path", entry.RelativePath).
						Int("existing_projects", len(projects)).
						Msg("RAG not available, using standard LLM project suggestion")
					if len(projects) > 5 {
						projects = projects[:5]
					}
					if summary != "" {
						project, err = s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, projects, detectedLanguage)
					} else {
						project, err = s.llmRouter.SuggestProject(traceCtx("project"), content, projects)
					}
				}
			} else {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Int("existing_projects", len(projects)).
					Msg("RAG disabled, using standard LLM project suggestion")
				if len(projects) > 5 {
					projects = projects[:5]
				}
				if summary != "" {
					project, err = s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, projects, detectedLanguage)
				} else {
					project, err = s.llmRouter.SuggestProject(traceCtx("project"), content, projects)
				}
			}

			if err != nil {
				s.logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Str("summary", summary).
					Str("description", description).
					Int("existing_projects", len(projects)).
					Msg("AI project suggestion failed")
			} else if project != "" {
				// Validate and normalize project name length
				project = s.normalizeProjectName(project)
				s.logger.Info().
					Str("path", entry.RelativePath).
					Str("suggested_project", project).
					Int("existing_projects", len(projects)).
					Bool("use_rag", s.config.UseRAGForProjects).
					Bool("has_project_service", s.projectService != nil).
					Bool("has_project_repo", s.projectRepo != nil).
					Msg("AI suggested project")
				exists := false
				for _, existing := range meta.Contexts {
					if strings.EqualFold(existing, project) {
						exists = true
						s.logger.Debug().
							Str("path", entry.RelativePath).
							Str("project", project).
							Msg("Project already exists in file contexts, skipping")
						break
					}
				}
				if !exists {
					// Create or get Project entity
					var proj *entity.Project
					if s.projectService != nil && s.projectRepo != nil {
						s.logger.Info().
							Str("path", entry.RelativePath).
							Str("project", project).
							Str("workspace_id", wsInfo.ID.String()).
							Msg("Looking up existing project by name")
						proj, err = s.projectService.GetProjectByName(ctx, wsInfo.ID, project)
						if err != nil {
							// sql.ErrNoRows is expected when project doesn't exist yet
							if err != sql.ErrNoRows {
								s.logger.Warn().
									Err(err).
									Str("path", entry.RelativePath).
									Str("project", project).
									Msg("Error looking up project, will create new one")
							} else {
								s.logger.Debug().
									Str("path", entry.RelativePath).
									Str("project", project).
									Msg("Project not found, will create new one")
							}
						}
						if err != nil || proj == nil {
							// Create new project
							s.logger.Info().
								Str("path", entry.RelativePath).
								Str("project", project).
								Str("workspace_id", wsInfo.ID.String()).
								Msg("Creating new project entity")
							proj, err = s.projectService.CreateProject(ctx, wsInfo.ID, project, "", nil)
							if err != nil {
								s.logger.Error().
									Err(err).
									Str("path", entry.RelativePath).
									Str("project", project).
									Str("workspace_id", wsInfo.ID.String()).
									Msg("CRITICAL: Failed to create project entity")
								// Fallback: guardar como context
								if s.config.UseSuggestedContexts {
									s.logger.Info().
										Str("path", entry.RelativePath).
										Str("project", project).
										Msg("Falling back to suggested context")
									_ = s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, project)
								} else {
									s.logger.Info().
										Str("path", entry.RelativePath).
										Str("project", project).
										Msg("Falling back to context")
									_ = s.metaRepo.AddContext(ctx, wsInfo.ID, entry.ID, project)
								}
							} else {
								s.logger.Info().
									Str("path", entry.RelativePath).
									Str("project", project).
									Str("project_id", proj.ID.String()).
									Str("workspace_id", wsInfo.ID.String()).
									Msg("Successfully created new project entity")
							}
						} else {
							s.logger.Info().
								Str("path", entry.RelativePath).
								Str("project", project).
								Str("project_id", proj.ID.String()).
								Str("workspace_id", wsInfo.ID.String()).
								Msg("Found existing project entity")
						}

						// Associate document with project if we have a project and document
						if proj != nil && s.docRepo != nil {
							doc, docErr := s.docRepo.GetDocumentByPath(ctx, wsInfo.ID, entry.RelativePath)
							if docErr == nil && doc != nil {
								if addErr := s.projectRepo.AddDocument(ctx, wsInfo.ID, proj.ID, doc.ID, entity.ProjectDocumentRolePrimary); addErr != nil {
									s.logger.Debug().
										Err(addErr).
										Str("path", entry.RelativePath).
										Str("project_id", proj.ID.String()).
										Str("doc_id", doc.ID.String()).
										Msg("Failed to associate document with project (may already be associated)")
								} else {
									s.logger.Info().
										Str("path", entry.RelativePath).
										Str("project", project).
										Str("project_id", proj.ID.String()).
										Str("doc_id", doc.ID.String()).
										Msg("Successfully associated document with project")
								}
							} else {
								s.logger.Debug().
									Err(docErr).
									Str("path", entry.RelativePath).
									Str("project_id", proj.ID.String()).
									Msg("Document not found for project association (may not have been parsed yet)")
							}
						} else {
							if proj == nil {
								s.logger.Debug().
									Str("path", entry.RelativePath).
									Str("project", project).
									Msg("Project entity is nil, cannot associate document")
							}
							if s.docRepo == nil {
								s.logger.Debug().
									Str("path", entry.RelativePath).
									Msg("Document repository is nil, cannot associate document with project")
							}
						}
					} else {
						s.logger.Warn().
							Str("path", entry.RelativePath).
							Str("project", project).
							Bool("has_project_service", s.projectService != nil).
							Bool("has_project_repo", s.projectRepo != nil).
							Msg("CRITICAL: Cannot create project entity - projectService or projectRepo is nil")
						// Fallback: guardar como context
						if s.config.UseSuggestedContexts {
							s.logger.Info().
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Falling back to suggested context (projectService/repo not available)")
							_ = s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, project)
						} else {
							s.logger.Info().
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Falling back to context (projectService/repo not available)")
							_ = s.metaRepo.AddContext(ctx, wsInfo.ID, entry.ID, project)
						}
					}

					// Also maintain in contexts for compatibility during migration
					if s.config.UseSuggestedContexts {
						if err := s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, project); err != nil {
							s.logger.Warn().
								Err(err).
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Failed to store suggested project in metadata")
						} else {
							s.logger.Debug().
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Stored suggested project in metadata")
						}
					} else {
						if err := s.metaRepo.AddContext(ctx, wsInfo.ID, entry.ID, project); err != nil {
							s.logger.Warn().
								Err(err).
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Failed to store project in metadata")
						} else {
							s.logger.Debug().
								Str("path", entry.RelativePath).
								Str("project", project).
								Msg("Stored project in metadata")
						}
					}

					if s.assignRepo != nil {
						status := entity.ProjectAssignmentAuto
						if existing := existingAssignments[project]; existing != nil {
							if existing.Status == entity.ProjectAssignmentManual || existing.Status == entity.ProjectAssignmentRejected {
								status = existing.Status
							}
						}
						var projectID entity.ProjectID
						if proj != nil {
							projectID = proj.ID
						}
						assignment := &entity.ProjectAssignment{
							WorkspaceID: wsInfo.ID,
							FileID:      entry.ID,
							ProjectID:   projectID,
							ProjectName: project,
							Score:       0.7,
							Sources:     []string{"llm_fallback"},
							Status:      status,
						}
						if err := s.assignRepo.Upsert(ctx, assignment); err != nil {
							s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store LLM project assignment")
						}
					}
				}
			}
		}
	} else if s.config.AutoIndexEnabled && s.config.UseSuggestedContexts {
		// Generate project suggestions without applying them (UseSuggestedContexts = true)
		s.logger.Info().
			Str("path", entry.RelativePath).
			Bool("apply_projects", s.config.ApplyProjects).
			Bool("auto_index_enabled", s.config.AutoIndexEnabled).
			Bool("use_suggested_contexts", s.config.UseSuggestedContexts).
			Msg("Generating project suggestions (not applying, storing as suggestions)")

		// Get existing projects for context
		// Use a separate context with longer timeout for database operations
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		projects, err := s.metaRepo.GetAllContexts(dbCtx, wsInfo.ID)
		dbCancel()
		if err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to load contexts")
			projects = []string{}
		}

		// Use current summary/description
		summary := currentSummary
		description := currentDescription
		if summary == "" && meta.AISummary != nil && meta.AISummary.Summary != "" {
			summary = meta.AISummary.Summary
		}
		if description == "" && meta.AISummary != nil && len(meta.AISummary.KeyTerms) > 0 {
			description = strings.Join(meta.AISummary.KeyTerms, ", ")
		}

		// Get detected language
		detectedLanguage := ""
		if meta.DetectedLanguage != nil {
			detectedLanguage = *meta.DetectedLanguage
		}

		var project string
		if s.config.UseRAGForProjects {
			ragAvailable, err := s.canUseRAG()
			if err != nil {
				s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("RAG not available for project suggestion")
			} else if ragAvailable {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Int("existing_projects", len(projects)).
					Msg("Using RAG for project suggestion (suggestions only)")
				project, err = s.suggestProjectWithRAG(ctx, wsInfo, entry, content, summary, description, projects, traceCtx)
			} else {
				if len(projects) > 5 {
					projects = projects[:5]
				}
				if summary != "" {
					project, err = s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, projects, detectedLanguage)
				} else {
					project, err = s.llmRouter.SuggestProject(traceCtx("project"), content, projects)
				}
			}
		} else {
			if len(projects) > 5 {
				projects = projects[:5]
			}
			if summary != "" {
				project, err = s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, projects, detectedLanguage)
			} else {
				project, err = s.llmRouter.SuggestProject(traceCtx("project"), content, projects)
			}
		}

		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("path", entry.RelativePath).
				Msg("AI project suggestion failed (suggestions only)")
		} else if project != "" {
			// Validate and normalize project name length
			project = s.normalizeProjectName(project)
			s.logger.Info().
				Str("path", entry.RelativePath).
				Str("suggested_project", project).
				Msg("AI suggested project (storing as suggestion)")
			// Store as suggested context only (not applying)
			if err := s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, project); err != nil {
				s.logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Str("project", project).
					Msg("Failed to store suggested project")
			} else {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Str("project", project).
					Msg("Stored suggested project (not applied)")
			}

			if s.assignRepo != nil {
				var projectID entity.ProjectID
				if s.projectService != nil {
					if proj, err := s.projectService.GetProjectByName(ctx, wsInfo.ID, project); err == nil && proj != nil {
						projectID = proj.ID
					}
				}
				assignment := &entity.ProjectAssignment{
					WorkspaceID: wsInfo.ID,
					FileID:      entry.ID,
					ProjectID:   projectID,
					ProjectName: project,
					Score:       0.6,
					Sources:     []string{"llm_suggestion"},
					Status:      entity.ProjectAssignmentSuggested,
				}
				if err := s.assignRepo.Upsert(ctx, assignment); err != nil {
					s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store suggested project assignment")
				}
			}
		}
	} else {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Bool("apply_projects", s.config.ApplyProjects).
			Bool("auto_index_enabled", s.config.AutoIndexEnabled).
			Bool("use_suggested_contexts", s.config.UseSuggestedContexts).
			Msg("Skipping project suggestions (ApplyProjects disabled and AutoIndexEnabled/UseSuggestedContexts false)")
	}

	if s.config.CategoryEnabled {
		var category string
		var confidence float32

		// Use current summary/description (from regeneration if available, otherwise from meta)
		summary := currentSummary
		description := currentDescription
		if summary == "" && meta.AISummary != nil && meta.AISummary.Summary != "" {
			summary = meta.AISummary.Summary
		}
		if description == "" && meta.AISummary != nil && len(meta.AISummary.KeyTerms) > 0 {
			description = strings.Join(meta.AISummary.KeyTerms, ", ")
		}

		if s.config.UseRAGForCategories {
			var err error
			ragAvailable, err := s.canUseRAG()
			if err != nil {
				return err
			}
			if ragAvailable {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Msg("Using RAG for category classification")
				category, confidence, err = s.classifyCategoryWithRAG(ctx, wsInfo, entry, content, summary, description, traceCtx)
			} else {
				// Use summary + description if available, fallback to content
				if summary != "" {
					category, err = s.llmRouter.ClassifyCategoryWithSummary(traceCtx("category"), summary, description, content, defaultCategoryList())
				} else {
					category, err = s.llmRouter.ClassifyCategory(traceCtx("category"), content, defaultCategoryList())
				}
				confidence = 0.5 // Default confidence without RAG
			}
			if err != nil {
				// Check if this is a timeout error - mark as indexing error
				if isTimeoutError(err) {
					errMsg := fmt.Sprintf("AI category classification timed out: %v", err)
					details := fmt.Sprintf("LLM request for category classification timed out for %s. This may indicate LLM service is overloaded or unavailable.", entry.RelativePath)

					if entry.Enhanced == nil {
						entry.Enhanced = &entity.EnhancedMetadata{}
					}
					entry.Enhanced.AddIndexingError(
						"ai",
						"classify_category",
						errMsg,
						details,
						"llm_timeout",
					)
				}
				s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("AI category classification failed")
			} else if category != "" {
				// Post-process category: validate against tags/resumen for religious documents
				category = s.validateCategoryAgainstContent(ctx, wsInfo, entry, meta, category, summary, description)

				_ = s.metaRepo.UpdateAICategory(ctx, wsInfo.ID, entry.ID, entity.AICategory{
					Category:   category,
					Confidence: float64(confidence), // Convert float32 to float64
					UpdatedAt:  time.Now(),
				})
			}
		}
	}

	if s.config.RelatedEnabled && s.fileRepo != nil {
		var relatedPaths []string
		var err error

		if s.config.UseRAGForRelated {
			ragAvailable, err := s.canUseRAG()
			if err != nil {
				return err
			}
			if ragAvailable {
				s.logger.Info().
					Str("path", entry.RelativePath).
					Msg("Using RAG for related files search")
				relatedPaths, err = s.findRelatedFilesWithRAG(ctx, wsInfo, entry, content)
			} else {
				candidates := s.collectCandidates(ctx, wsInfo.ID, entry.RelativePath)
				if len(candidates) > 0 {
					relatedPaths, err = s.llmRouter.FindRelatedFiles(traceCtx("related"), content, candidates, s.config.RelatedMaxResults)
				}
			}
		} else {
			candidates := s.collectCandidates(ctx, wsInfo.ID, entry.RelativePath)
			if len(candidates) > 0 {
				relatedPaths, err = s.llmRouter.FindRelatedFiles(traceCtx("related"), content, candidates, s.config.RelatedMaxResults)
			}
		}

		if err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("AI related files failed")
		} else if len(relatedPaths) > 0 {
			related := make([]entity.RelatedFile, 0, len(relatedPaths))
			for _, path := range relatedPaths {
				if path == "" {
					continue
				}
				related = append(related, entity.RelatedFile{RelativePath: path})
			}
			if len(related) > 0 {
				_ = s.metaRepo.UpdateAIRelated(ctx, wsInfo.ID, entry.ID, related)
			}
		}
	}

	s.logger.Debug().
		Str("path", entry.RelativePath).
		Str("source", source).
		Msg("AI stage completed")

	// Regenerate embeddings with enriched metadata if significant changes occurred
	// This ensures embeddings reflect the latest metadata for better RAG search
	// Only regenerate if we have the necessary components and significant metadata
	if s.embedder != nil && s.docRepo != nil && s.vectorStore != nil {
		if err := s.regenerateEmbeddingsIfNeeded(ctx, wsInfo, entry, meta); err != nil {
			s.logger.Warn().
				Err(err).
				Str("path", entry.RelativePath).
				Msg("Failed to regenerate embeddings with metadata, continuing anyway")
			// Don't fail the stage if embedding regeneration fails
		}
	}

	return nil
}

// normalizeProjectName validates and normalizes project name length and format.
// Ensures project names are concise (max 50 chars) and removes problematic characters.
func (s *AIStage) normalizeProjectName(name string) string {
	// Trim whitespace
	name = strings.TrimSpace(name)

	// Remove quotes if present
	name = strings.Trim(name, `"'`)

	// Remove trailing punctuation (more comprehensive)
	// Remove in reverse order to handle multiple punctuation marks
	for {
		original := name
		name = strings.TrimSuffix(name, ".")
		name = strings.TrimSuffix(name, ",")
		name = strings.TrimSuffix(name, ";")
		name = strings.TrimSuffix(name, ":")
		if name == original {
			break // No more punctuation to remove
		}
	}
	name = strings.TrimSpace(name)

	// Validate length (max 50 characters, prefer 30-40)
	if len(name) > 50 {
		originalName := name
		// Try to truncate at word boundary
		truncated := name[:50]
		lastSpace := strings.LastIndex(truncated, " ")
		if lastSpace > 20 { // Only truncate at word if we keep at least 20 chars
			name = truncated[:lastSpace]
			s.logger.Warn().
				Str("original", originalName).
				Str("truncated", name).
				Int("original_length", len(originalName)).
				Int("truncated_length", len(name)).
				Msg("Project name truncated to 50 characters at word boundary")
		} else {
			name = truncated
			s.logger.Warn().
				Str("original", originalName).
				Str("truncated", name).
				Int("original_length", len(originalName)).
				Int("truncated_length", len(name)).
				Msg("Project name truncated to 50 characters (no word boundary found)")
		}
	}

	// Remove colons (problematic for project names)
	name = strings.ReplaceAll(name, ":", " - ")
	name = strings.TrimSpace(name)

	return name
}

func isTagSimilarToAny(tag string, existing map[string]struct{}) bool {
	for existingTag := range existing {
		if entity.AreTagsSimilar(tag, existingTag) {
			return true
		}
	}
	return false
}

// regenerateEmbeddingsIfNeeded checks if embeddings should be regenerated with enriched metadata.
// This is called after AIStage completes to ensure embeddings reflect the latest metadata.
func (s *AIStage) regenerateEmbeddingsIfNeeded(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	meta *entity.FileMetadata,
) error {
	// Check if document exists
	doc, err := s.docRepo.GetDocumentByPath(ctx, wsInfo.ID, entry.RelativePath)
	if err != nil || doc == nil {
		// Document doesn't exist yet, embeddings will be created in DocumentStage
		return nil
	}

	// Get chunks for this document
	chunks, err := s.docRepo.GetChunksByDocument(ctx, wsInfo.ID, doc.ID)
	if err != nil || len(chunks) == 0 {
		// No chunks yet, embeddings will be created in DocumentStage
		return nil
	}

	// Check if we have significant metadata that would improve embeddings
	hasSignificantMetadata := false
	metadataCount := 0

	if len(meta.Tags) > 0 {
		hasSignificantMetadata = true
		metadataCount += len(meta.Tags)
	}
	if meta.AICategory != nil && meta.AICategory.Category != "" {
		hasSignificantMetadata = true
		metadataCount++
	}
	if len(meta.Contexts) > 0 {
		hasSignificantMetadata = true
		metadataCount += len(meta.Contexts)
	}
	if meta.AISummary != nil && (len(meta.AISummary.KeyTerms) > 0 || meta.AISummary.Summary != "") {
		hasSignificantMetadata = true
		if len(meta.AISummary.KeyTerms) > 0 {
			metadataCount += len(meta.AISummary.KeyTerms)
		}
		if meta.AISummary.Summary != "" {
			metadataCount++
		}
	}

	if !hasSignificantMetadata {
		// No significant metadata to enrich with, skip regeneration
		s.logger.Debug().
			Str("path", entry.RelativePath).
			Msg("No significant metadata to enrich embeddings, skipping regeneration")
		return nil
	}

	// Regenerate embeddings: metadata was just added/updated in this pass
	// This ensures embeddings include the latest metadata for better RAG search
	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("chunks", len(chunks)).
		Int("metadata_items", metadataCount).
		Int("tags", len(meta.Tags)).
		Bool("has_category", meta.AICategory != nil && meta.AICategory.Category != "").
		Int("contexts", len(meta.Contexts)).
		Msg("Regenerating embeddings with enriched metadata")

	// Create a context with a very long timeout for embedding regeneration
	// Effectively unlimited for analysis purposes
	// Calculate timeout based on number of chunks: ~10 seconds per chunk with 5 concurrent workers
	// Minimum 5 minutes, maximum 60 minutes
	estimatedSeconds := (len(chunks) / 5) * 10
	if estimatedSeconds < 300 {
		estimatedSeconds = 300 // 5 minutes minimum
	}
	if estimatedSeconds > 3600 {
		estimatedSeconds = 3600 // 60 minutes maximum
	}
	embedCtx, embedCancel := context.WithTimeout(context.Background(), time.Duration(estimatedSeconds)*time.Second)
	defer embedCancel()

	// Process chunks concurrently with a semaphore to limit concurrency
	// Use 5 concurrent workers to avoid overwhelming the embedding service
	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)

	newEmbeddings := make([]entity.ChunkEmbedding, 0, len(chunks))
	regenerationErrors := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, ch *entity.Chunk) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check if context is cancelled
			select {
			case <-embedCtx.Done():
				mu.Lock()
				regenerationErrors++
				mu.Unlock()
				s.logger.Warn().
					Err(embedCtx.Err()).
					Str("path", entry.RelativePath).
					Str("chunk_id", string(ch.ID)).
					Int("chunk_index", idx+1).
					Msg("Embedding regeneration cancelled due to timeout")
				return
			default:
			}

			// Enrich chunk text with metadata
			enrichedText := s.enrichChunkTextForEmbedding(ch.Text, meta)

			// Truncate if needed (embedding models have token limits)
			if len(enrichedText) > 4000 {
				truncated := enrichedText[:4000]
				lastSpace := strings.LastIndex(truncated, " ")
				if lastSpace > 3600 {
					enrichedText = truncated[:lastSpace] + "..."
				} else {
					enrichedText = truncated + "..."
				}
			}

			vector, err := s.embedder.Embed(embedCtx, enrichedText)
			if err != nil {
				mu.Lock()
				regenerationErrors++
				mu.Unlock()
				s.logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Str("chunk_id", string(ch.ID)).
					Int("chunk_index", idx+1).
					Msg("Failed to regenerate embedding for chunk")
				return
			}

			mu.Lock()
			newEmbeddings = append(newEmbeddings, entity.ChunkEmbedding{
				ChunkID:    ch.ID,
				Vector:     vector,
				Dimensions: len(vector),
				UpdatedAt:  time.Now(),
			})
			mu.Unlock()
		}(i, chunk)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	if len(newEmbeddings) == 0 {
		return fmt.Errorf("failed to regenerate any embeddings (%d errors)", regenerationErrors)
	}

	// Update embeddings in vector store
	// Create a new context with very long timeout for database operations
	// Effectively unlimited for analysis purposes
	// Use a timeout based on the number of embeddings: ~500ms per embedding, minimum 5 minutes, maximum 60 minutes
	dbTimeoutSeconds := (len(newEmbeddings) * 500) / 1000
	if dbTimeoutSeconds < 300 {
		dbTimeoutSeconds = 300 // 5 minutes minimum
	}
	if dbTimeoutSeconds > 3600 {
		dbTimeoutSeconds = 3600 // 60 minutes maximum
	}
	dbCtx, dbCancel := context.WithTimeout(context.Background(), time.Duration(dbTimeoutSeconds)*time.Second)
	defer dbCancel()

	if err := s.vectorStore.BulkUpsert(dbCtx, wsInfo.ID, newEmbeddings); err != nil {
		// Log warning but don't fail completely - embeddings were generated successfully
		// The embeddings will be updated on the next regeneration pass
		s.logger.Warn().
			Err(err).
			Str("path", entry.RelativePath).
			Int("embeddings", len(newEmbeddings)).
			Msg("Failed to regenerate embeddings with metadata, continuing anyway")
		// Return nil to allow pipeline to continue - embeddings will be updated on next regeneration
		return nil
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("chunks", len(chunks)).
		Int("embeddings_regenerated", len(newEmbeddings)).
		Int("errors", regenerationErrors).
		Int("metadata_items", metadataCount).
		Msg("Successfully regenerated embeddings with enriched metadata")

	return nil
}

// enrichChunkTextForEmbedding enriches chunk text with metadata (same logic as DocumentStage).
func (s *AIStage) enrichChunkTextForEmbedding(chunkText string, fileMeta *entity.FileMetadata) string {
	if fileMeta == nil {
		return chunkText
	}

	var metadataParts []string

	// Add tags
	if len(fileMeta.Tags) > 0 {
		metadataParts = append(metadataParts, "Tags: "+strings.Join(fileMeta.Tags, ", "))
	}

	// Add category
	if fileMeta.AICategory != nil && fileMeta.AICategory.Category != "" {
		metadataParts = append(metadataParts, "Categoría: "+fileMeta.AICategory.Category)
	}

	// Add project/context
	if len(fileMeta.Contexts) > 0 {
		metadataParts = append(metadataParts, "Proyecto: "+strings.Join(fileMeta.Contexts, ", "))
	}

	// Add key terms from AI summary
	if fileMeta.AISummary != nil && len(fileMeta.AISummary.KeyTerms) > 0 {
		keyTerms := fileMeta.AISummary.KeyTerms
		if len(keyTerms) > 5 {
			keyTerms = keyTerms[:5]
		}
		metadataParts = append(metadataParts, "Términos clave: "+strings.Join(keyTerms, ", "))
	}

	// Add summary (truncated) if available
	if fileMeta.AISummary != nil && fileMeta.AISummary.Summary != "" {
		summary := fileMeta.AISummary.Summary
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		metadataParts = append(metadataParts, "Resumen: "+summary)
	}

	if len(metadataParts) == 0 {
		return chunkText
	}

	metadataSection := strings.Join(metadataParts, ". ")
	return metadataSection + "\n\n---\n\n" + chunkText
}

var mirrorFormats = map[string]string{
	".pdf":  "md",
	".docx": "md",
	".doc":  "md",
	".pptx": "md",
	".ppt":  "md",
	".odt":  "md",
	".xlsx": "csv",
	".xls":  "csv",
	".ods":  "csv",
}

var errNoContent = errors.New("no ai content")

func resolveAIContent(workspaceRoot string, entry *entity.FileEntry) (string, string, error) {
	ext := strings.ToLower(entry.Extension)
	if format, ok := mirrorFormats[ext]; ok {
		mirrorPath := filepath.Join(workspaceRoot, ".cortex", "mirror", filepath.FromSlash(entry.RelativePath)+"."+format)
		content, err := os.ReadFile(mirrorPath)
		if err == nil {
			text := strings.TrimSpace(string(content))
			if text != "" {
				return text, "mirror", nil
			}
			// Mirror file exists but is empty - this is a problem
			return "", "", fmt.Errorf("mirror file exists but is empty: %s", mirrorPath)
		} else {
			// Mirror file doesn't exist - this is expected if MirrorStage hasn't run yet
			// but we should log it at info level so we can see what's happening
			return "", "", fmt.Errorf("mirror file not found (MirrorStage may not have run yet): %s", mirrorPath)
		}
	}

	if ext != ".md" && !isLikelyTextCategory(entry) {
		return "", "", errNoContent
	}

	content, err := os.ReadFile(entry.AbsolutePath)
	if err != nil {
		return "", "", err
	}
	text := strings.TrimSpace(string(content))
	if text == "" {
		return "", "", errNoContent
	}
	return text, "file", nil
}

func isLikelyTextCategory(entry *entity.FileEntry) bool {
	if entry.Enhanced == nil || entry.Enhanced.MimeType == nil {
		return false
	}
	switch strings.ToLower(entry.Enhanced.MimeType.Category) {
	case "text", "code":
		return true
	default:
		return false
	}
}

func defaultCategoryList() []string {
	return []string{
		"Ciencia y Tecnología",
		"Arte y Diseño",
		"Negocios y Finanzas",
		"Educación y Referencia",
		"Literatura y Escritura",
		"Documentación Técnica",
		"Recursos Humanos",
		"Marketing y Comunicación",
		"Legal y Regulatorio",
		"Salud y Medicina",
		"Religión y Teología",
		"Ingeniería y Construcción",
		"Investigación y Análisis",
		"Configuración y Administración",
		"Pruebas y Calidad",
		"Sin Clasificar",
	}
}

// collectAllProjectCandidates collects project candidates from all metadata sources.
func (s *AIStage) collectAllProjectCandidates(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	meta *entity.FileMetadata,
	_ string, // summary - reserved for future use
	_ string, // description - reserved for future use
) []*ProjectCandidate {
	candidates := make(map[string]*ProjectCandidate)

	// 1. From confirmed contexts
	for _, context := range meta.Contexts {
		candidates[context] = &ProjectCandidate{
			ProjectName: context,
			Score:       1.0,
			Sources:     []string{"confirmed_context"},
		}
	}

	// 2. From suggested contexts
	if meta.SuggestedContexts != nil {
		for _, context := range meta.SuggestedContexts {
			if existing, ok := candidates[context]; ok {
				existing.Score += 0.7
				existing.Sources = append(existing.Sources, "suggested_context")
			} else {
				candidates[context] = &ProjectCandidate{
					ProjectName: context,
					Score:       0.7,
					Sources:     []string{"suggested_context"},
				}
			}
		}
	}

	// 3. From suggested metadata (SuggestionStage)
	if s.suggestedRepo != nil {
		suggestedMeta, err := s.suggestedRepo.Get(ctx, wsInfo.ID, entry.ID)
		if err == nil && suggestedMeta != nil {
			for _, suggestedProject := range suggestedMeta.SuggestedProjects {
				projectName := suggestedProject.ProjectName
				if existing, ok := candidates[projectName]; ok {
					existing.Score += suggestedProject.Confidence * 0.8
					if suggestedProject.ProjectID != nil {
						existing.ProjectID = *suggestedProject.ProjectID
					}
					existing.Sources = append(existing.Sources, fmt.Sprintf("suggested_metadata_%.2f", suggestedProject.Confidence))
				} else {
					candidate := &ProjectCandidate{
						ProjectName: projectName,
						Score:       suggestedProject.Confidence * 0.8,
						Sources:     []string{fmt.Sprintf("suggested_metadata_%.2f", suggestedProject.Confidence)},
					}
					if suggestedProject.ProjectID != nil {
						candidate.ProjectID = *suggestedProject.ProjectID
					}
					candidates[projectName] = candidate
				}
			}
		}
	}

	// 4. From document metadata (title, author, etc.)
	if entry.Enhanced != nil && entry.Enhanced.DocumentMetrics != nil {
		dm := entry.Enhanced.DocumentMetrics
		// Extract project hints from metadata
		if dm.Company != nil {
			company := strings.ToLower(*dm.Company)
			if existing, ok := candidates[company]; ok {
				existing.Score += 0.3
				existing.Sources = append(existing.Sources, "metadata_company")
			} else {
				candidates[company] = &ProjectCandidate{
					ProjectName: company,
					Score:       0.3,
					Sources:     []string{"metadata_company"},
				}
			}
		}
		if dm.Category != nil {
			category := strings.ToLower(*dm.Category)
			if existing, ok := candidates[category]; ok {
				existing.Score += 0.2
				existing.Sources = append(existing.Sources, "metadata_category")
			} else {
				candidates[category] = &ProjectCandidate{
					ProjectName: category,
					Score:       0.2,
					Sources:     []string{"metadata_category"},
				}
			}
		}
	}

	// 5. From tags (tags can hint at projects)
	for _, tag := range meta.Tags {
		// Simple heuristic: if tag matches a project name pattern
		tagLower := strings.ToLower(tag)
		for name, candidate := range candidates {
			if strings.Contains(strings.ToLower(name), tagLower) || strings.Contains(tagLower, strings.ToLower(name)) {
				candidate.Score += 0.15
				candidate.Sources = append(candidate.Sources, fmt.Sprintf("tag_match:%s", tag))
			}
		}
	}

	// Convert to slice and sort by score
	result := make([]*ProjectCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate)
	}

	// Sort by score (descending)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Score < result[j].Score {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// ProjectCandidate represents a potential project assignment.
type ProjectCandidate struct {
	ProjectID   entity.ProjectID
	ProjectName string
	Score       float64
	Sources     []string
}

// assignProjectToFile assigns a project to a file.
func (s *AIStage) assignProjectToFile(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	meta *entity.FileMetadata,
	projectName string,
	projectID entity.ProjectID,
) (entity.ProjectID, error) {
	// Check if already assigned
	for _, existing := range meta.Contexts {
		if strings.EqualFold(existing, projectName) {
			return projectID, nil // Already assigned
		}
	}

	// Get or create project
	var proj *entity.Project
	var err error

	if s.projectService == nil || s.projectRepo == nil {
		if s.config.UseSuggestedContexts {
			return projectID, s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, projectName)
		}
		return projectID, s.metaRepo.AddContext(ctx, wsInfo.ID, entry.ID, projectName)
	}

	if projectID != "" {
		proj, err = s.projectService.GetProject(ctx, wsInfo.ID, projectID)
		if err != nil {
			s.logger.Debug().Err(err).Str("project_id", projectID.String()).Msg("Project ID not found, will create by name")
		}
	}

	if proj == nil {
		proj, err = s.projectService.GetProjectByName(ctx, wsInfo.ID, projectName)
		if err != nil || proj == nil {
			// Create new project
			proj, err = s.projectService.CreateProject(ctx, wsInfo.ID, projectName, "", nil)
			if err != nil {
				return "", fmt.Errorf("failed to create project: %w", err)
			}
		}
	}

	// Associate document with project
	if s.docRepo != nil {
		doc, err := s.docRepo.GetDocumentByPath(ctx, wsInfo.ID, entry.RelativePath)
		if err == nil && doc != nil {
			if err := s.projectRepo.AddDocument(ctx, wsInfo.ID, proj.ID, doc.ID, entity.ProjectDocumentRolePrimary); err != nil {
				s.logger.Debug().Err(err).Msg("Failed to associate document (may already be associated)")
			}
		}
	}

	// Also add to contexts for compatibility
	if s.config.UseSuggestedContexts {
		return proj.ID, s.metaRepo.AddSuggestedContext(ctx, wsInfo.ID, entry.ID, projectName)
	}
	return proj.ID, s.metaRepo.AddContext(ctx, wsInfo.ID, entry.ID, projectName)
}

// validateCategoryAgainstContent validates category against tags, summary, and description
// to correct misclassifications, especially for religious/theological documents.
func (s *AIStage) validateCategoryAgainstContent(
	_ context.Context, // reserved for future use
	_ contextinfo.WorkspaceInfo, // reserved for future use
	entry *entity.FileEntry,
	meta *entity.FileMetadata,
	category string,
	summary string,
	description string,
) string {
	// Check if document contains religious/theological terms
	hasReligiousTerms := false

	// Check tags
	if meta != nil {
		for _, tag := range meta.Tags {
			tagLower := strings.ToLower(tag)
			religiousKeywords := []string{
				"teología", "teologia", "religión", "religion", "religioso", "religiosa",
				"santo", "santa", "cristo", "dios", "evangelio", "evangelios",
				"biblia", "bíblico", "biblico", "iglesia", "católico", "catolico",
				"espiritual", "místico", "mistico", "fe", "creencia", "creyente",
				"oración", "oracion", "rezar", "rezar", "sagrado", "sagrada",
			}
			for _, keyword := range religiousKeywords {
				if strings.Contains(tagLower, keyword) {
					hasReligiousTerms = true
					break
				}
			}
			if hasReligiousTerms {
				break
			}
		}
	}

	// Check summary and description
	if !hasReligiousTerms {
		combinedText := strings.ToLower(summary + " " + description)
		religiousKeywords := []string{
			"teología", "teologia", "religión", "religion", "religioso", "religiosa",
			"conferencia teológica", "conferencia teologica", "teológico", "teologico",
			"santo", "santa", "cristo", "dios", "evangelio", "evangelios",
			"biblia", "bíblico", "biblico", "iglesia", "católico", "catolico",
			"espiritual", "místico", "mistico", "fe", "creencia", "creyente",
			"sábana santa", "sabana santa", "divinidad", "autenticidad",
		}
		for _, keyword := range religiousKeywords {
			if strings.Contains(combinedText, keyword) {
				hasReligiousTerms = true
				break
			}
		}
	}

	// If document has religious terms but category is not "Religión y Teología", override it
	if hasReligiousTerms && category != "Religión y Teología" {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Str("original_category", category).
			Str("corrected_category", "Religión y Teología").
			Msg("Category corrected: document contains religious/theological terms, overriding to 'Religión y Teología'")
		return "Religión y Teología"
	}

	return category
}

var keyTermStopwords = map[string]struct{}{
	// English stopwords
	"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {}, "from": {}, "into": {}, "than": {}, "then": {},
	"else": {}, "when": {}, "while": {}, "where": {}, "what": {}, "which": {}, "who": {}, "whom": {}, "whose": {},
	"also": {}, "been": {}, "are": {}, "was": {}, "were": {}, "will": {}, "would": {}, "could": {}, "should": {},
	"have": {}, "has": {}, "had": {}, "not": {}, "but": {}, "can": {}, "may": {}, "might": {}, "our": {}, "your": {},
	"their": {}, "them": {}, "they": {}, "you": {}, "we": {}, "its": {}, "it's": {}, "true": {}, "false": {}, "null": {},
	"return": {}, "const": {}, "let": {}, "var": {}, "function": {}, "class": {}, "interface": {}, "type": {},
	"import": {}, "export": {}, "default": {}, "public": {}, "private": {}, "protected": {}, "static": {},
	"async": {}, "await": {}, "new": {}, "try": {}, "catch": {}, "throw": {}, "case": {}, "break": {}, "if": {},
	"switch": {}, "do": {}, "in": {}, "of": {}, "to": {}, "as": {}, "is": {},
	// Spanish stopwords
	"que": {}, "los": {}, "del": {}, "por": {}, "una": {}, "con": {}, "las": {}, "porque": {}, "pero": {}, "para": {},
	"como": {}, "esta": {}, "este": {}, "estos": {}, "estas": {}, "ese": {}, "eso": {}, "esos": {}, "esas": {},
	"donde": {}, "cuando": {}, "quien": {}, "quienes": {}, "cual": {}, "cuales": {}, "cuanto": {}, "cuantos": {},
	"tambien": {}, "tampoco": {}, "si": {}, "no": {}, "sino": {}, "aunque": {}, "mientras": {}, "desde": {},
	"hasta": {}, "hacia": {}, "sobre": {}, "bajo": {}, "entre": {}, "durante": {}, "mediante": {}, "según": {},
	"ante": {}, "contra": {}, "sin": {}, "tras": {},
	"ser": {}, "estar": {}, "haber": {}, "tener": {}, "hacer": {}, "decir": {}, "ir": {}, "ver": {}, "dar": {},
	"saber": {}, "querer": {}, "llegar": {}, "pasar": {}, "deber": {}, "poner": {}, "parecer": {}, "quedar": {},
	"hablar": {}, "llevar": {}, "dejar": {}, "seguir": {}, "encontrar": {}, "llamar": {}, "venir": {}, "pensar": {},
	"salir": {}, "volver": {}, "tomar": {}, "conocer": {}, "vivir": {}, "sentir": {}, "tratar": {}, "mirar": {},
	"contar": {}, "empezar": {}, "esperar": {}, "buscar": {}, "existir": {}, "entrar": {}, "trabajar": {}, "escribir": {},
	"perder": {}, "producir": {}, "ocurrir": {}, "entender": {}, "pedir": {}, "recibir": {}, "recordar": {}, "terminar": {},
	"permitir": {}, "aparecer": {}, "conseguir": {}, "comenzar": {}, "servir": {}, "sacar": {}, "necesitar": {},
	"mantener": {}, "resultar": {}, "leer": {}, "caer": {}, "cambiar": {}, "presentar": {}, "crear": {}, "abrir": {},
	"considerar": {}, "oír": {}, "acabar": {}, "convertir": {}, "ganar": {}, "formar": {}, "traer": {}, "partir": {},
	"morir": {}, "aceptar": {}, "realizar": {}, "suponer": {}, "comprender": {}, "lograr": {}, "explicar": {},
	"preguntar": {}, "tocar": {}, "reconocer": {}, "estudiar": {}, "alcanzar": {}, "nacer": {}, "dirigir": {},
	"correr": {}, "utilizar": {}, "pagar": {}, "ayudar": {}, "gustar": {}, "jugar": {}, "escuchar": {},
	"sentar": {}, "incluir": {}, "continuar": {}, "sufrir": {}, "visualizar": {}, "describir": {}, "ofrecer": {},
	"mostrar": {}, "indicar": {}, "definir": {}, "referir": {}, "establecer": {}, "demostrar": {},
	"obtener": {}, "proporcionar": {}, "desarrollar": {}, "proponer": {}, "sugerir": {}, "recomendar": {},
}

// extractKeyTerms extracts key terms from content, filtering out stopwords.
// It prioritizes longer words and words that appear multiple times.
// Supports both English and Spanish content.
func extractKeyTerms(content string, maxTerms int) []string {
	if maxTerms <= 0 {
		maxTerms = 12
	}

	// Match words including Spanish characters (á, é, í, ó, ú, ñ, ü)
	re := regexp.MustCompile(`[A-Za-zÁÉÍÓÚÑÜáéíóúñü_][A-Za-zÁÉÍÓÚÑÜáéíóúñü0-9_]{2,}`)
	tokens := re.FindAllString(content, -1)
	counts := make(map[string]int)

	for _, raw := range tokens {
		token := strings.ToLower(raw)
		// Skip stopwords
		if _, stop := keyTermStopwords[token]; stop {
			continue
		}
		// Skip very short words (less than 3 chars after normalization)
		if len(token) < 3 {
			continue
		}
		// Skip common years (4-digit numbers starting with 19 or 20)
		if matched, _ := regexp.MatchString(`^(19|20)\d{2}$`, token); matched {
			continue
		}
		counts[token]++
	}

	type pair struct {
		term   string
		count  int
		length int
	}
	terms := make([]pair, 0, len(counts))
	for term, count := range counts {
		terms = append(terms, pair{
			term:   term,
			count:  count,
			length: len(term),
		})
	}

	// Sort by: 1) frequency (higher first), 2) length (longer first), 3) alphabetical
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].count != terms[j].count {
			return terms[i].count > terms[j].count
		}
		if terms[i].length != terms[j].length {
			return terms[i].length > terms[j].length
		}
		return terms[i].term < terms[j].term
	})

	if len(terms) > maxTerms {
		terms = terms[:maxTerms]
	}

	results := make([]string, 0, len(terms))
	for _, term := range terms {
		results = append(results, term.term)
	}
	return results
}

// generateKeyTermsFromSummary uses LLM to extract key terms from a summary.
// This produces more relevant terms than simple frequency-based extraction.
func (s *AIStage) generateKeyTermsFromSummary(
	ctx context.Context,
	summary string,
	_ func(string) context.Context, // traceCtx - reserved for future use
) ([]string, error) {
	if s.llmRouter == nil {
		return nil, fmt.Errorf("LLM router not available")
	}

	// Use prompt from router if available, otherwise use default
	prompt := s.llmRouter.GetExtractKeyTermsPrompt(summary)

	resp, err := s.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   100,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	// Parse comma-separated terms
	text := strings.TrimSpace(resp.Text)
	terms := strings.Split(text, ",")
	result := make([]string, 0, len(terms))
	for _, term := range terms {
		cleaned := strings.TrimSpace(term)
		// Remove common prefixes/suffixes that LLMs sometimes add
		cleaned = strings.TrimPrefix(cleaned, "-")
		cleaned = strings.TrimPrefix(cleaned, "•")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned != "" && len(cleaned) >= 2 {
			result = append(result, cleaned)
		}
	}

	// Limit to 12 terms
	if len(result) > 12 {
		result = result[:12]
	}

	return result, nil
}

// canUseRAG checks if RAG components are available.
// Returns error if RAG is required but not available.
func (s *AIStage) canUseRAG() (bool, error) {
	available := s.vectorStore != nil && s.embedder != nil && s.docRepo != nil
	if !available {
		missing := []string{}
		if s.vectorStore == nil {
			missing = append(missing, "vectorStore")
		}
		if s.embedder == nil {
			missing = append(missing, "embedder")
		}
		if s.docRepo == nil {
			missing = append(missing, "docRepo")
		}
		err := fmt.Errorf("CRITICAL: RAG not available - missing components: %s", strings.Join(missing, ", "))
		s.logger.Error().
			Err(err).
			Bool("vector_store", s.vectorStore != nil).
			Bool("embedder", s.embedder != nil).
			Bool("doc_repo", s.docRepo != nil).
			Msg("CRITICAL: RAG not available - missing components")
		return false, err
	}
	return true, nil
}

// truncateForEmbedding truncates text to a safe length for embedding models.
// nomic-embed-text has a limit around 8192 tokens, which is roughly 4000-5000 characters.
// We use 4000 as a conservative limit to ensure compatibility.
func truncateForEmbedding(text string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 4000 // Conservative default for nomic-embed-text
	}
	if len(text) <= maxChars {
		return text
	}
	// Truncate at word boundary if possible
	truncated := text[:maxChars]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars*9/10 { // If space is near the end, use it
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

// classifyCategoryWithRAG uses RAG to find similar files and improve categorization.
func (s *AIStage) classifyCategoryWithRAG(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	content string,
	summary string,
	description string,
	traceCtx func(string) context.Context,
) (string, float32, error) {
	// Use summary for embedding if available, otherwise use truncated content
	embeddingText := content
	if summary != "" {
		// Use summary + description for embedding (more representative than truncated content)
		if description != "" {
			embeddingText = summary + "\n\n" + description
		} else {
			embeddingText = summary
		}
	}
	embeddingText = truncateForEmbedding(embeddingText, 4000)

	// Create embedding of current content
	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("embedding_text_length", len(embeddingText)).
		Msg("Creating embedding for RAG category classification")

	vector, err := s.embedder.Embed(ctx, embeddingText)
	if err != nil {
		// Check if this is a timeout error - mark as indexing error
		if isTimeoutError(err) {
			errMsg := fmt.Sprintf("Embedding generation timed out for category classification: %v", err)
			details := fmt.Sprintf("Embedding request timed out for %s during category classification. This may indicate LLM service is overloaded or unavailable.", entry.RelativePath)

			if entry.Enhanced == nil {
				entry.Enhanced = &entity.EnhancedMetadata{}
			}
			entry.Enhanced.AddIndexingError(
				"ai",
				"create_embedding_category",
				errMsg,
				details,
				"llm_timeout",
			)
		}
		s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to create embedding, falling back to non-RAG classification")
		if summary != "" {
			category, err2 := s.llmRouter.ClassifyCategoryWithSummary(traceCtx("category"), summary, description, content, defaultCategoryList())
			return category, 0.5, err2
		}
		category, err2 := s.llmRouter.ClassifyCategory(traceCtx("category"), content, defaultCategoryList())
		return category, 0.5, err2
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("vector_dimensions", len(vector)).
		Msg("Embedding created, searching vector store for similar files")

	// Search for similar files (top 5)
	matches, err := s.vectorStore.Search(ctx, wsInfo.ID, vector, 5)
	if err != nil || len(matches) == 0 {
		s.logger.Warn().
			Err(err).
			Str("path", entry.RelativePath).
			Int("matches_found", len(matches)).
			Msg("Vector search failed or no matches, falling back to non-RAG classification")
		// Fallback to non-RAG classification
		if summary != "" {
			category, err2 := s.llmRouter.ClassifyCategoryWithSummary(traceCtx("category"), summary, description, content, defaultCategoryList())
			return category, 0.5, err2
		}
		category, err2 := s.llmRouter.ClassifyCategory(traceCtx("category"), content, defaultCategoryList())
		return category, 0.5, err2
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("matches_found", len(matches)).
		Msg("Vector search completed, analyzing similar files for category")

	// Filter matches by similarity threshold
	validMatches := make([]repository.VectorMatch, 0)
	for _, match := range matches {
		if match.Similarity >= s.config.RAGSimilarityThreshold {
			validMatches = append(validMatches, match)
		}
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("total_matches", len(matches)).
		Int("valid_matches", len(validMatches)).
		Float32("similarity_threshold", s.config.RAGSimilarityThreshold).
		Msg("Filtered matches by similarity threshold")

	if len(validMatches) == 0 {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Msg("No similar files above threshold, using standard classification")
		// No similar files above threshold, use standard classification
		if summary != "" {
			category, err2 := s.llmRouter.ClassifyCategoryWithSummary(traceCtx("category"), summary, description, content, defaultCategoryList())
			return category, 0.5, err2
		}
		category, err2 := s.llmRouter.ClassifyCategory(traceCtx("category"), content, defaultCategoryList())
		return category, 0.5, err2
	}

	// Get categories from similar files
	similarCategories := s.getCategoriesFromSimilarFiles(ctx, wsInfo.ID, validMatches)

	// If we have strong consensus from similar files, use it directly
	if len(similarCategories) > 0 {
		// Count category occurrences
		categoryCounts := make(map[string]int)
		for _, cat := range similarCategories {
			if cat != "" && cat != "Sin Clasificar" {
				categoryCounts[cat]++
			}
		}

		// Find most common category
		maxCount := 0
		mostCommon := ""
		for cat, count := range categoryCounts {
			if count > maxCount {
				maxCount = count
				mostCommon = cat
			}
		}

		// If we have strong consensus (>= 3 files with same category, or >= 50% of matches), use it
		totalValidCategories := len(similarCategories)
		if maxCount >= 3 || (totalValidCategories >= 2 && float64(maxCount)/float64(totalValidCategories) >= 0.5) {
			s.logger.Info().
				Str("path", entry.RelativePath).
				Str("consensus_category", mostCommon).
				Int("count", maxCount).
				Int("total", totalValidCategories).
				Msg("Strong category consensus from similar files, using consensus category")
			// Still use LLM but with high confidence
			confidence := s.calculateConfidence(validMatches)
			// Boost confidence if consensus is strong
			if maxCount >= 3 {
				boosted := confidence * 1.2
				if boosted > 0.95 {
					confidence = 0.95
				} else {
					confidence = boosted
				}
			}
			// Use LLM to validate, but consensus is strong signal
			var category string
			if summary != "" {
				category, err = s.llmRouter.ClassifyCategoryWithContextAndSummary(
					traceCtx("category"),
					summary,
					description,
					content,
					defaultCategoryList(),
					similarCategories,
				)
			} else {
				category, err = s.llmRouter.ClassifyCategoryWithContext(
					traceCtx("category"),
					content,
					defaultCategoryList(),
					similarCategories,
				)
			}
			// If LLM result matches consensus, use it; otherwise prefer consensus for strong cases
			if err == nil && category != "" {
				if strings.EqualFold(category, mostCommon) {
					return category, confidence, nil
				}
				// If consensus is very strong (>= 4 files), prefer consensus
				if maxCount >= 4 {
					s.logger.Info().
						Str("path", entry.RelativePath).
						Str("llm_category", category).
						Str("consensus_category", mostCommon).
						Msg("Very strong consensus, preferring consensus over LLM result")
					return mostCommon, confidence, nil
				}
			}
			// Fall through to use LLM result if consensus not strong enough
		}
	}

	// Use LLM with context from similar files, using summary + description if available
	var category string
	if summary != "" {
		category, err = s.llmRouter.ClassifyCategoryWithContextAndSummary(
			traceCtx("category"),
			summary,
			description,
			content,
			defaultCategoryList(),
			similarCategories,
		)
	} else {
		category, err = s.llmRouter.ClassifyCategoryWithContext(
			traceCtx("category"),
			content,
			defaultCategoryList(),
			similarCategories,
		)
	}

	// Calculate confidence based on similarity scores
	confidence := s.calculateConfidence(validMatches)

	return category, confidence, err
}

// getCategoriesFromSimilarFiles extracts categories from similar files.
func (s *AIStage) getCategoriesFromSimilarFiles(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	matches []repository.VectorMatch,
) []string {
	categories := make([]string, 0)
	seen := make(map[string]struct{})

	for _, match := range matches {
		// Get chunk to find document
		chunks, err := s.docRepo.GetChunksByIDs(ctx, workspaceID, []entity.ChunkID{match.ChunkID})
		if err != nil || len(chunks) == 0 {
			continue
		}

		doc, err := s.docRepo.GetDocument(ctx, workspaceID, chunks[0].DocumentID)
		if err != nil || doc == nil {
			continue
		}

		// Get metadata for this document
		meta, err := s.metaRepo.GetByPath(ctx, workspaceID, doc.RelativePath)
		if err != nil || meta == nil {
			continue
		}

		if meta.AICategory != nil && meta.AICategory.Category != "" {
			category := meta.AICategory.Category
			if _, exists := seen[category]; !exists {
				categories = append(categories, category)
				seen[category] = struct{}{}
			}
		}
	}

	return categories
}

// calculateConfidence calculates confidence based on similarity scores.
func (s *AIStage) calculateConfidence(matches []repository.VectorMatch) float32 {
	if len(matches) == 0 {
		return 0.5
	}

	// Average similarity, capped at 0.95
	var sum float32
	for _, match := range matches {
		sum += match.Similarity
	}
	avg := sum / float32(len(matches))
	if avg > 0.95 {
		return 0.95
	}
	return avg
}

// suggestTagsWithRAG uses RAG to find similar files and suggest tags based on their tags.
func (s *AIStage) suggestTagsWithRAG(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	content string,
	summary string,
	description string,
	maxTags int,
	traceCtx func(string) context.Context,
) ([]string, error) {
	var contextTags []string

	// Use summary for embedding if available, otherwise use truncated content
	embeddingText := content
	if summary != "" {
		if description != "" {
			embeddingText = summary + "\n\n" + description
		} else {
			embeddingText = summary
		}
	}
	embeddingText = truncateForEmbedding(embeddingText, 4000)

	s.logger.Info().
		Bool("has_summary", summary != "").
		Bool("has_description", description != "").
		Int("embedding_text_length", len(embeddingText)).
		Msg("Creating embedding for RAG tag suggestion")

	// Create embedding and search for similar files
	vector, err := s.embedder.Embed(ctx, embeddingText)
	if err != nil {
		// Note: Timeout errors are handled at the caller level where entry is available
		s.logger.Warn().Err(err).Msg("Failed to create embedding for tag suggestion")
	}
	if err == nil {
		s.logger.Info().
			Int("vector_dimensions", len(vector)).
			Msg("Searching vector store for similar files for tag suggestion")

		matches, err := s.vectorStore.Search(ctx, wsInfo.ID, vector, 10)
		if err == nil && len(matches) > 0 {
			s.logger.Info().
				Int("matches_found", len(matches)).
				Msg("Vector search completed, extracting tags from similar files")
			// Get tags from similar files
			contextTags = s.getTagsFromSimilarFiles(ctx, wsInfo.ID, matches)
			s.logger.Info().
				Int("context_tags_found", len(contextTags)).
				Msg("Extracted tags from similar files")
		} else if err != nil {
			s.logger.Warn().Err(err).Msg("Vector search failed for tag suggestion")
		}
	} else {
		s.logger.Warn().Err(err).Msg("Failed to create embedding for tag suggestion")
	}

	// Use LLM with context tags, using summary + description if available
	if summary != "" {
		return s.llmRouter.SuggestTagsWithContextAndSummary(traceCtx("tags"), summary, description, content, maxTags, contextTags)
	}
	return s.llmRouter.SuggestTagsWithContext(traceCtx("tags"), content, maxTags, contextTags)
}

// getTagsFromSimilarFiles extracts tags from similar files.
func (s *AIStage) getTagsFromSimilarFiles(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	matches []repository.VectorMatch,
) []string {
	tags := make([]string, 0)
	tagCounts := make(map[string]int)

	for _, match := range matches {
		chunks, err := s.docRepo.GetChunksByIDs(ctx, workspaceID, []entity.ChunkID{match.ChunkID})
		if err != nil || len(chunks) == 0 {
			continue
		}

		doc, err := s.docRepo.GetDocument(ctx, workspaceID, chunks[0].DocumentID)
		if err != nil || doc == nil {
			continue
		}

		meta, err := s.metaRepo.GetByPath(ctx, workspaceID, doc.RelativePath)
		if err != nil || meta == nil {
			continue
		}

		for _, tag := range meta.Tags {
			tagCounts[tag]++
		}
	}

	// Sort by frequency and return top tags
	type tagPair struct {
		tag   string
		count int
	}
	pairs := make([]tagPair, 0, len(tagCounts))
	for tag, count := range tagCounts {
		pairs = append(pairs, tagPair{tag: tag, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	for _, pair := range pairs {
		if len(tags) >= 10 { // Limit context tags
			break
		}
		tags = append(tags, pair.tag)
	}

	return tags
}

// suggestProjectWithRAG uses RAG to find projects with similar content.
func (s *AIStage) suggestProjectWithRAG(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	content string,
	summary string,
	description string,
	existingProjects []string,
	traceCtx func(string) context.Context,
) (string, error) {
	// Get detected language from metadata if available
	meta, _ := s.metaRepo.GetByPath(ctx, wsInfo.ID, entry.RelativePath)
	langCode := ""
	if meta != nil && meta.DetectedLanguage != nil {
		langCode = *meta.DetectedLanguage
	}

	if len(existingProjects) == 0 {
		// No existing projects, use standard method
		if summary != "" {
			return s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, existingProjects, langCode)
		}
		return s.llmRouter.SuggestProject(traceCtx("project"), content, existingProjects)
	}

	// Use summary for embedding if available, otherwise use truncated content
	embeddingText := content
	if summary != "" {
		if description != "" {
			embeddingText = summary + "\n\n" + description
		} else {
			embeddingText = summary
		}
	}
	embeddingText = truncateForEmbedding(embeddingText, 4000)

	s.logger.Info().
		Bool("has_summary", summary != "").
		Bool("has_description", description != "").
		Int("embedding_text_length", len(embeddingText)).
		Int("existing_projects", len(existingProjects)).
		Msg("Creating embedding for RAG project suggestion")

	// Create embedding of current content
	vector, err := s.embedder.Embed(ctx, embeddingText)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to create embedding, falling back to standard project suggestion")
		// Fallback to standard method
		if summary != "" {
			// Get detected language from metadata if available
			meta, _ := s.metaRepo.GetByPath(ctx, wsInfo.ID, entry.RelativePath)
			langCode := ""
			if meta != nil && meta.DetectedLanguage != nil {
				langCode = *meta.DetectedLanguage
			}
			return s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, existingProjects, langCode)
		}
		return s.llmRouter.SuggestProject(traceCtx("project"), content, existingProjects)
	}

	s.logger.Info().
		Int("vector_dimensions", len(vector)).
		Msg("Embedding created, building project vectors for comparison")

	// For each project, find representative files and create project vector
	projectVectors := make(map[string][]float32)
	projectFiles := make(map[string][]string)

	for _, project := range existingProjects {
		// Get files in this project
		fileMetas, err := s.metaRepo.ListByContext(ctx, wsInfo.ID, project, repository.DefaultFileListOptions())
		if err != nil || len(fileMetas) == 0 {
			continue
		}

		filePaths := make([]string, 0, len(fileMetas))
		for _, meta := range fileMetas {
			filePaths = append(filePaths, meta.RelativePath)
		}
		projectFiles[project] = filePaths

		// Get embeddings for files in this project (sample up to 5 files)
		sampleSize := 5
		if len(filePaths) < sampleSize {
			sampleSize = len(filePaths)
		}

		var projectVectorsList [][]float32
		for i := 0; i < sampleSize; i++ {
			// Get document for this file
			doc, err := s.docRepo.GetDocumentByPath(ctx, wsInfo.ID, filePaths[i])
			if err != nil || doc == nil {
				continue
			}

			// Get first chunk as representative
			chunks, err := s.docRepo.GetChunksByDocument(ctx, wsInfo.ID, doc.ID)
			if err != nil || len(chunks) == 0 {
				continue
			}

			// Create embedding from first chunk (truncate if needed)
			chunkText := truncateForEmbedding(chunks[0].Text, 4000)
			chunkVector, err := s.embedder.Embed(ctx, chunkText)
			if err == nil {
				projectVectorsList = append(projectVectorsList, chunkVector)
			}
		}

		// Average the vectors to create project representation
		if len(projectVectorsList) > 0 {
			s.logger.Info().
				Str("project", project).
				Int("project_vectors", len(projectVectorsList)).
				Msg("Averaging project vectors for RAG project suggestion")

			avgVector := averageVectors(projectVectorsList)
			projectVectors[project] = avgVector
		}
	}

	s.logger.Info().
		Int("project_vectors_available", len(projectVectors)).
		Float32("similarity_threshold", s.config.RAGSimilarityThreshold).
		Msg("Comparing content vector with project vectors")

	// Find most similar project
	bestProject := ""
	bestSimilarity := float32(0)

	for project, projVector := range projectVectors {
		similarity := cosineSimilarity(vector, projVector)
		s.logger.Debug().
			Str("project", project).
			Float32("similarity", similarity).
			Msg("Project similarity calculated")
		if similarity > bestSimilarity && similarity >= s.config.RAGSimilarityThreshold {
			bestSimilarity = similarity
			bestProject = project
		}
	}

	if bestProject != "" {
		s.logger.Info().
			Str("best_project", bestProject).
			Float32("best_similarity", bestSimilarity).
			Msg("RAG project suggestion found")

		// Verify that the suggested project name matches the content language
		// If content is Spanish but project name is in English, regenerate with LLM
		embeddingTextForDetection := content
		if summary != "" {
			embeddingTextForDetection = summary
		} else if description != "" {
			embeddingTextForDetection = description
		}

		// Check: if content has Spanish or Portuguese characters but project name doesn't, regenerate
		hasSpanishChars := strings.ContainsAny(embeddingTextForDetection, "áéíóúñü¿¡ÁÉÍÓÚÑÜ")
		hasPortugueseChars := strings.ContainsAny(embeddingTextForDetection, "áéíóúãõâêôçàüÁÉÍÓÚÃÕÂÊÔÇÀÜ")
		projectHasSpanishChars := strings.ContainsAny(bestProject, "áéíóúñü¿¡ÁÉÍÓÚÑÜ")
		projectHasPortugueseChars := strings.ContainsAny(bestProject, "áéíóúãõâêôçàüÁÉÍÓÚÃÕÂÊÔÇÀÜ")

		// Also check for common Spanish and Portuguese words in content vs English words in project
		contentLower := strings.ToLower(embeddingTextForDetection)
		projectLower := strings.ToLower(bestProject)
		hasSpanishWords := strings.Contains(contentLower, " el ") || strings.Contains(contentLower, " la ") ||
			strings.Contains(contentLower, " de ") || strings.Contains(contentLower, " que ") ||
			strings.Contains(contentLower, " y ") || strings.Contains(contentLower, " en ") ||
			strings.Contains(contentLower, " del ") || strings.Contains(contentLower, " con ") ||
			strings.Contains(contentLower, " las ") || strings.Contains(contentLower, " los ")
		hasPortugueseWords := strings.Contains(contentLower, " o ") || strings.Contains(contentLower, " a ") ||
			strings.Contains(contentLower, " os ") || strings.Contains(contentLower, " as ") ||
			strings.Contains(contentLower, " de ") || strings.Contains(contentLower, " do ") ||
			strings.Contains(contentLower, " da ") || strings.Contains(contentLower, " dos ") ||
			strings.Contains(contentLower, " das ") || strings.Contains(contentLower, " que ") ||
			strings.Contains(contentLower, " e ") || strings.Contains(contentLower, " em ") ||
			strings.Contains(contentLower, " com ") || strings.Contains(contentLower, " para ") ||
			strings.Contains(contentLower, " não ") || strings.Contains(contentLower, " são ")
		hasEnglishWords := strings.Contains(projectLower, " the ") || strings.Contains(projectLower, " of ") ||
			strings.Contains(projectLower, " and ") || strings.Contains(projectLower, " in ") ||
			strings.Contains(projectLower, " for ") || strings.Contains(projectLower, " with ") ||
			strings.Contains(projectLower, " project") || strings.Contains(projectLower, " documents")

		// Check if content is Spanish/Portuguese but project is English
		contentIsSpanish := (hasSpanishChars || hasSpanishWords) && !hasPortugueseChars && !hasPortugueseWords
		contentIsPortuguese := (hasPortugueseChars || hasPortugueseWords) && !hasSpanishChars && !hasSpanishWords
		projectIsEnglish := hasEnglishWords && !projectHasSpanishChars && !projectHasPortugueseChars

		if (contentIsSpanish || contentIsPortuguese) && projectIsEnglish {
			langType := "Spanish"
			if contentIsPortuguese {
				langType = "Portuguese"
			}
			s.logger.Info().
				Str("best_project", bestProject).
				Str("content_language", langType).
				Bool("content_has_spanish", hasSpanishChars || hasSpanishWords).
				Bool("content_has_portuguese", hasPortugueseChars || hasPortugueseWords).
				Bool("project_has_spanish", projectHasSpanishChars).
				Bool("project_has_portuguese", projectHasPortugueseChars).
				Bool("project_has_english", hasEnglishWords).
				Msgf("RAG found English project for %s content, regenerating with LLM to ensure %s name", langType, langType)
			// Regenerate with LLM to ensure correct language name
			if len(existingProjects) > 5 {
				existingProjects = existingProjects[:5]
			}
			if summary != "" {
				return s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, existingProjects)
			}
			return s.llmRouter.SuggestProject(traceCtx("project"), content, existingProjects)
		}

		return bestProject, nil
	}

	s.logger.Info().
		Float32("best_similarity", bestSimilarity).
		Msg("No project above threshold, falling back to LLM suggestion")

	// Fallback to standard LLM suggestion
	if len(existingProjects) > 5 {
		existingProjects = existingProjects[:5]
	}
	if summary != "" {
		return s.llmRouter.SuggestProjectWithSummary(traceCtx("project"), summary, description, content, entry.RelativePath, existingProjects)
	}
	return s.llmRouter.SuggestProject(traceCtx("project"), content, existingProjects)
}

// findRelatedFilesWithRAG uses semantic search instead of candidate list.
func (s *AIStage) findRelatedFilesWithRAG(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	entry *entity.FileEntry,
	content string,
) ([]string, error) {
	// Truncate content for embedding
	embeddingText := truncateForEmbedding(content, 4000)

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("content_length", len(content)).
		Int("embedding_text_length", len(embeddingText)).
		Msg("Creating embedding for RAG related files search")

	// Create embedding of current content
	vector, err := s.embedder.Embed(ctx, embeddingText)
	if err != nil {
		// Check if this is a timeout error - mark as indexing error
		if isTimeoutError(err) {
			errMsg := fmt.Sprintf("Embedding generation timed out for related files search: %v", err)
			details := fmt.Sprintf("Embedding request timed out for %s during related files search. This may indicate LLM service is overloaded or unavailable.", entry.RelativePath)

			if entry.Enhanced == nil {
				entry.Enhanced = &entity.EnhancedMetadata{}
			}
			entry.Enhanced.AddIndexingError(
				"ai",
				"create_embedding_related",
				errMsg,
				details,
				"llm_timeout",
			)
		}
		// Log but don't fail - return empty list instead
		s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to create embedding for related files")
		return nil, nil
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("vector_dimensions", len(vector)).
		Int("max_results", s.config.RelatedMaxResults+1).
		Msg("Searching vector store for related files")

	// Search for similar files (exclude current file by filtering results)
	matches, err := s.vectorStore.Search(ctx, wsInfo.ID, vector, s.config.RelatedMaxResults+1)
	if err != nil {
		s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Vector search failed for related files")
		return nil, err
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("matches_found", len(matches)).
		Msg("Vector search completed for related files")

	// Get chunks and documents, filter out current file
	relatedPaths := make([]string, 0)
	seenPaths := make(map[string]struct{})
	seenPaths[entry.RelativePath] = struct{}{}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("matches_to_process", len(matches)).
		Msg("Processing matches to find related files")

	for _, match := range matches {
		chunks, err := s.docRepo.GetChunksByIDs(ctx, wsInfo.ID, []entity.ChunkID{match.ChunkID})
		if err != nil || len(chunks) == 0 {
			continue
		}

		doc, err := s.docRepo.GetDocument(ctx, wsInfo.ID, chunks[0].DocumentID)
		if err != nil || doc == nil {
			continue
		}

		if doc.RelativePath == entry.RelativePath {
			continue // Skip current file
		}

		if _, exists := seenPaths[doc.RelativePath]; !exists {
			relatedPaths = append(relatedPaths, doc.RelativePath)
			seenPaths[doc.RelativePath] = struct{}{}
			if len(relatedPaths) >= s.config.RelatedMaxResults {
				break
			}
		}
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("related_files_found", len(relatedPaths)).
		Msg("RAG related files search completed")

	return relatedPaths, nil
}

// generateSummaryWithRAG includes context from related files.
func (s *AIStage) generateSummaryWithRAG(
	ctx context.Context,
	wsInfo contextinfo.WorkspaceInfo,
	content string,
	maxLength int,
	traceCtx func(string) context.Context,
) (string, error) {
	var contextSnippets []string

	// Truncate content for embedding
	embeddingText := truncateForEmbedding(content, 4000)

	s.logger.Info().
		Int("content_length", len(content)).
		Int("embedding_text_length", len(embeddingText)).
		Msg("Creating embedding for RAG-enhanced summary")

	// Find related content using RAG
	vector, err := s.embedder.Embed(ctx, embeddingText)
	if err == nil {
		s.logger.Info().
			Int("vector_dimensions", len(vector)).
			Msg("Searching vector store for context for RAG summary")

		matches, err := s.vectorStore.Search(ctx, wsInfo.ID, vector, 3)
		if err == nil && len(matches) > 0 {
			s.logger.Info().
				Int("context_matches", len(matches)).
				Msg("Found context matches for RAG summary")

			for _, match := range matches {
				chunks, err := s.docRepo.GetChunksByIDs(ctx, wsInfo.ID, []entity.ChunkID{match.ChunkID})
				if err == nil && len(chunks) > 0 {
					chunkText := chunks[0].Text
					if len(chunkText) > 200 {
						chunkText = chunkText[:200] + "..."
					}
					contextSnippets = append(contextSnippets, chunkText)
				}
			}
		} else if err != nil {
			s.logger.Warn().Err(err).Msg("Vector search failed for RAG summary")
		}
	} else {
		s.logger.Warn().Err(err).Msg("Failed to create embedding for RAG summary")
	}

	s.logger.Info().
		Int("context_snippets", len(contextSnippets)).
		Msg("Generating summary with RAG context")

	// Generate summary with context
	return s.llmRouter.GenerateSummaryWithContext(traceCtx("summary"), content, maxLength, contextSnippets)
}

// Helper functions for vector operations

func averageVectors(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	if len(vectors) == 1 {
		return vectors[0]
	}

	dim := len(vectors[0])
	result := make([]float32, dim)

	for _, vec := range vectors {
		if len(vec) != dim {
			continue
		}
		for i := range vec {
			result[i] += vec[i]
		}
	}

	for i := range result {
		result[i] /= float32(len(vectors))
	}

	return result
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
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

	return dotProduct / (float32(sqrt(float64(normA))) * float32(sqrt(float64(normB))))
}

func sqrt(x float64) float64 {
	return math.Sqrt(x)
}

// isTimeoutError checks if an error is a timeout or deadline exceeded error.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

func (s *AIStage) collectCandidates(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) []string {
	if s.fileRepo == nil {
		return nil
	}
	limit := s.config.RelatedCandidates
	if limit <= 0 {
		limit = 100
	}
	opts := repository.DefaultFileListOptions()
	opts.Limit = limit
	files, err := s.fileRepo.List(ctx, workspaceID, opts)
	if err != nil {
		return nil
	}
	candidates := make([]string, 0, len(files))
	for _, file := range files {
		if file.RelativePath == relativePath {
			continue
		}
		candidates = append(candidates, file.RelativePath)
	}
	return candidates
}
