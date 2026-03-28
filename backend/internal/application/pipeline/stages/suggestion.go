// Package stages provides pipeline processing stages.
package stages

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/metadata"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// SuggestionStage generates metadata suggestions using RAG and LLM.
type SuggestionStage struct {
	suggestionService *metadata.SuggestionService
	metaRepo          repository.MetadataRepository
	suggestedRepo     repository.SuggestedMetadataRepository
	logger            zerolog.Logger
	enabled           bool
}

// NewSuggestionStage creates a new suggestion generation stage.
func NewSuggestionStage(
	suggestionService *metadata.SuggestionService,
	metaRepo repository.MetadataRepository,
	suggestedRepo repository.SuggestedMetadataRepository,
	logger zerolog.Logger,
	enabled bool,
) *SuggestionStage {
	return &SuggestionStage{
		suggestionService: suggestionService,
		metaRepo:          metaRepo,
		suggestedRepo:     suggestedRepo,
		logger:            logger.With().Str("component", "suggestion_stage").Logger(),
		enabled:           enabled,
	}
}

// Name returns the stage name.
func (s *SuggestionStage) Name() string {
	return "suggestion"
}

// CanProcess returns true if suggestions are enabled and file has been indexed.
func (s *SuggestionStage) CanProcess(entry *entity.FileEntry) bool {
	if !s.enabled || s.suggestionService == nil {
		return false
	}
	// Only process files that have been indexed (have metadata)
	return entry.Enhanced != nil && entry.Enhanced.IndexedState.Document
}

// Process generates metadata suggestions for the file.
func (s *SuggestionStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return nil
	}

	// Get file metadata
	fileMeta, err := s.metaRepo.GetByPath(ctx, wsInfo.ID, entry.RelativePath)
	if err != nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("File metadata not found, skipping suggestions")
		return nil
	}

	// Generate suggestions
	suggested, err := s.suggestionService.GenerateSuggestions(ctx, wsInfo.ID, entry, fileMeta)
	if err != nil {
		s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to generate suggestions")
		return nil // Don't fail pipeline on suggestion errors
	}

	if suggested == nil || !suggested.HasSuggestions() {
		s.logger.Debug().Str("path", entry.RelativePath).Msg("No suggestions generated")
		return nil
	}

	// Store suggestions in database
	if s.suggestedRepo != nil {
		if err := s.suggestedRepo.Upsert(ctx, wsInfo.ID, suggested); err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store suggestions")
			// Don't fail the pipeline on storage errors
		} else {
			s.logger.Info().
				Str("path", entry.RelativePath).
				Int("tags", len(suggested.SuggestedTags)).
				Int("projects", len(suggested.SuggestedProjects)).
				Float64("confidence", suggested.Confidence).
				Msg("Stored metadata suggestions")
		}
	}

	return nil
}

