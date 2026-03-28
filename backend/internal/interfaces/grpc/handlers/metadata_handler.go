// Package handlers provides gRPC service implementations.
package handlers

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// MetadataHandler handles metadata-related gRPC requests.
type MetadataHandler struct {
	metaRepo      repository.MetadataRepository
	traceRepo     repository.TraceRepository
	suggestedRepo repository.SuggestedMetadataRepository
	publisher     event.Publisher
	logger        zerolog.Logger
}

// NewMetadataHandler creates a new metadata handler.
func NewMetadataHandler(
	metaRepo repository.MetadataRepository,
	traceRepo repository.TraceRepository,
	publisher event.Publisher,
	logger zerolog.Logger,
) *MetadataHandler {
	return &MetadataHandler{
		metaRepo:  metaRepo,
		traceRepo: traceRepo,
		publisher: publisher,
		logger:    logger.With().Str("handler", "metadata").Logger(),
	}
}

// SetSuggestedMetadataRepository sets the suggested metadata repository.
func (h *MetadataHandler) SetSuggestedMetadataRepository(repo repository.SuggestedMetadataRepository) {
	h.suggestedRepo = repo
}

// GetMetadata retrieves metadata for a file.
func (h *MetadataHandler) GetMetadata(ctx context.Context, workspaceID, path string) (*entity.FileMetadata, error) {
	fileID := entity.NewFileID(path)
	return h.metaRepo.Get(ctx, entity.WorkspaceID(workspaceID), fileID)
}

// ListProcessingTraces returns processing traces for a file or workspace.
func (h *MetadataHandler) ListProcessingTraces(ctx context.Context, workspaceID, relativePath string, limit int) ([]entity.ProcessingTrace, error) {
	if h.traceRepo == nil {
		return nil, nil
	}
	wsID := entity.WorkspaceID(workspaceID)
	if relativePath != "" {
		return h.traceRepo.ListTracesByFile(ctx, wsID, relativePath, limit)
	}
	return h.traceRepo.ListRecentTraces(ctx, wsID, limit)
}

// AddTag adds a tag to a file.
func (h *MetadataHandler) AddTag(ctx context.Context, workspaceID, path, tag string) error {
	fileID := entity.NewFileID(path)

	if err := h.metaRepo.AddTag(ctx, entity.WorkspaceID(workspaceID), fileID, tag); err != nil {
		return err
	}

	// Publish event
	if h.publisher != nil {
		_ = h.publisher.Publish(ctx, &event.Event{
			Type: event.EventTagAdded,
			Data: event.MetadataEventData{
				FileID:   fileID.String(),
				FilePath: path,
				Tag:      tag,
			},
		})
	}

	h.logger.Debug().
		Str("path", path).
		Str("tag", tag).
		Msg("Tag added")

	return nil
}

// RemoveTag removes a tag from a file.
func (h *MetadataHandler) RemoveTag(ctx context.Context, workspaceID, path, tag string) error {
	fileID := entity.NewFileID(path)

	if err := h.metaRepo.RemoveTag(ctx, entity.WorkspaceID(workspaceID), fileID, tag); err != nil {
		return err
	}

	// Publish event
	if h.publisher != nil {
		_ = h.publisher.Publish(ctx, &event.Event{
			Type: event.EventTagRemoved,
			Data: event.MetadataEventData{
				FileID:   fileID.String(),
				FilePath: path,
				Tag:      tag,
			},
		})
	}

	h.logger.Debug().
		Str("path", path).
		Str("tag", tag).
		Msg("Tag removed")

	return nil
}

// ListByTag lists files with a specific tag.
func (h *MetadataHandler) ListByTag(ctx context.Context, workspaceID, tag string, opts ListFilesOptions) ([]*entity.FileMetadata, error) {
	repoOpts := repository.DefaultFileListOptions()
	repoOpts.Offset = opts.Offset
	if opts.Limit > 0 {
		repoOpts.Limit = opts.Limit
	}
	return h.metaRepo.ListByTag(ctx, entity.WorkspaceID(workspaceID), tag, repoOpts)
}

// GetAllTags returns all tags with counts.
func (h *MetadataHandler) GetAllTags(ctx context.Context, workspaceID string) ([]string, error) {
	return h.metaRepo.GetAllTags(ctx, entity.WorkspaceID(workspaceID))
}

// GetTagCounts returns all tags with counts.
func (h *MetadataHandler) GetTagCounts(ctx context.Context, workspaceID string) (map[string]int, error) {
	return h.metaRepo.GetTagCounts(ctx, entity.WorkspaceID(workspaceID))
}

// AddContext adds a context/project to a file.
func (h *MetadataHandler) AddContext(ctx context.Context, workspaceID, path, context string) error {
	fileID := entity.NewFileID(path)

	if err := h.metaRepo.AddContext(ctx, entity.WorkspaceID(workspaceID), fileID, context); err != nil {
		return err
	}

	// Publish event
	if h.publisher != nil {
		_ = h.publisher.Publish(ctx, &event.Event{
			Type: event.EventContextAdded,
			Data: event.MetadataEventData{
				FileID:   fileID.String(),
				FilePath: path,
				Context:  context,
			},
		})
	}

	h.logger.Debug().
		Str("path", path).
		Str("context", context).
		Msg("Context added")

	return nil
}

// RemoveContext removes a context/project from a file.
func (h *MetadataHandler) RemoveContext(ctx context.Context, workspaceID, path, context string) error {
	fileID := entity.NewFileID(path)

	if err := h.metaRepo.RemoveContext(ctx, entity.WorkspaceID(workspaceID), fileID, context); err != nil {
		return err
	}

	// Publish event
	if h.publisher != nil {
		_ = h.publisher.Publish(ctx, &event.Event{
			Type: event.EventContextRemoved,
			Data: event.MetadataEventData{
				FileID:   fileID.String(),
				FilePath: path,
				Context:  context,
			},
		})
	}

	h.logger.Debug().
		Str("path", path).
		Str("context", context).
		Msg("Context removed")

	return nil
}

// ListByContext lists files with a specific context.
func (h *MetadataHandler) ListByContext(ctx context.Context, workspaceID, context string, opts ListFilesOptions) ([]*entity.FileMetadata, error) {
	repoOpts := repository.DefaultFileListOptions()
	repoOpts.Offset = opts.Offset
	if opts.Limit > 0 {
		repoOpts.Limit = opts.Limit
	}
	return h.metaRepo.ListByContext(ctx, entity.WorkspaceID(workspaceID), context, repoOpts)
}

// GetAllContexts returns all contexts with counts.
func (h *MetadataHandler) GetAllContexts(ctx context.Context, workspaceID string) ([]string, error) {
	return h.metaRepo.GetAllContexts(ctx, entity.WorkspaceID(workspaceID))
}

// GetContextCounts returns all contexts with counts.
func (h *MetadataHandler) GetContextCounts(ctx context.Context, workspaceID string) (map[string]int, error) {
	return h.metaRepo.GetContextCounts(ctx, entity.WorkspaceID(workspaceID))
}

// AddSuggestedContext adds a suggested context to a file.
func (h *MetadataHandler) AddSuggestedContext(ctx context.Context, workspaceID, path, context string) error {
	fileID := entity.NewFileID(path)
	return h.metaRepo.AddSuggestedContext(ctx, entity.WorkspaceID(workspaceID), fileID, context)
}

// AcceptSuggestion accepts a suggested context.
func (h *MetadataHandler) AcceptSuggestion(ctx context.Context, workspaceID, path, context string) error {
	fileID := entity.NewFileID(path)
	workspaceIDValue := entity.WorkspaceID(workspaceID)

	// Add to contexts
	if err := h.metaRepo.AddContext(ctx, workspaceIDValue, fileID, context); err != nil {
		return err
	}

	// Remove from suggestions
	return h.metaRepo.RemoveSuggestedContext(ctx, workspaceIDValue, fileID, context)
}

// DismissSuggestion dismisses a suggested context.
func (h *MetadataHandler) DismissSuggestion(ctx context.Context, workspaceID, path, context string) error {
	fileID := entity.NewFileID(path)
	return h.metaRepo.RemoveSuggestedContext(ctx, entity.WorkspaceID(workspaceID), fileID, context)
}

// UpdateNotes updates notes for a file.
func (h *MetadataHandler) UpdateNotes(ctx context.Context, workspaceID, path string, notes *string) error {
	fileID := entity.NewFileID(path)
	if notes == nil {
		return nil
	}
	return h.metaRepo.UpdateNotes(ctx, entity.WorkspaceID(workspaceID), fileID, *notes)
}

// UpdateAISummary updates AI summary metadata for a file.
func (h *MetadataHandler) UpdateAISummary(ctx context.Context, workspaceID, path string, summary entity.AISummary) error {
	fileID := entity.NewFileID(path)
	h.logger.Debug().
		Str("path", path).
		Int("summary_chars", len(summary.Summary)).
		Int("key_terms", len(summary.KeyTerms)).
		Msg("Updating AI summary")
	return h.metaRepo.UpdateAISummary(ctx, entity.WorkspaceID(workspaceID), fileID, summary)
}

// GetSuggestions returns files with suggested contexts.
func (h *MetadataHandler) GetSuggestions(ctx context.Context, workspaceID string, relativePath *string) ([]*entity.FileMetadata, error) {
	workspaceIDValue := entity.WorkspaceID(workspaceID)
	if relativePath != nil && *relativePath != "" {
		meta, err := h.metaRepo.GetByPath(ctx, workspaceIDValue, *relativePath)
		if err != nil {
			return nil, err
		}
		if meta == nil {
			return []*entity.FileMetadata{}, nil
		}
		return []*entity.FileMetadata{meta}, nil
	}
	opts := repository.DefaultFileListOptions()
	return h.metaRepo.GetFilesWithSuggestions(ctx, workspaceIDValue, opts)
}

// GetSuggestedMetadata retrieves suggested metadata for a file.
func (h *MetadataHandler) GetSuggestedMetadata(ctx context.Context, workspaceID, relativePath string) (*entity.SuggestedMetadata, error) {
	if h.suggestedRepo == nil {
		return nil, nil // No suggestions available if repo is not set
	}
	workspaceIDValue := entity.WorkspaceID(workspaceID)
	return h.suggestedRepo.GetByPath(ctx, workspaceIDValue, relativePath)
}

// BatchAddTags adds tags to multiple files.
func (h *MetadataHandler) BatchAddTags(ctx context.Context, workspaceID string, paths []string, tags []string) error {
	workspaceIDValue := entity.WorkspaceID(workspaceID)
	for _, path := range paths {
		fileID := entity.NewFileID(path)
		for _, tag := range tags {
			if err := h.metaRepo.AddTag(ctx, workspaceIDValue, fileID, tag); err != nil {
				h.logger.Warn().
					Err(err).
					Str("path", path).
					Str("tag", tag).
					Msg("Failed to add tag in batch")
			}
		}
	}
	return nil
}

// BatchAddContexts adds contexts to multiple files.
func (h *MetadataHandler) BatchAddContexts(ctx context.Context, workspaceID string, paths []string, contexts []string) error {
	workspaceIDValue := entity.WorkspaceID(workspaceID)
	for _, path := range paths {
		fileID := entity.NewFileID(path)
		for _, context := range contexts {
			if err := h.metaRepo.AddContext(ctx, workspaceIDValue, fileID, context); err != nil {
				h.logger.Warn().
					Err(err).
					Str("path", path).
					Str("context", context).
					Msg("Failed to add context in batch")
			}
		}
	}
	return nil
}
