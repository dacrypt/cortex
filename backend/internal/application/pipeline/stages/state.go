package stages

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// StateStage infers and sets document states.
type StateStage struct {
	docRepo    repository.DocumentRepository
	stateRepo  repository.DocumentStateRepository
	relRepo    repository.RelationshipRepository
	logger     zerolog.Logger
}

// NewStateStage creates a new state inference stage.
func NewStateStage(
	docRepo repository.DocumentRepository,
	stateRepo repository.DocumentStateRepository,
	relRepo repository.RelationshipRepository,
	logger zerolog.Logger,
) *StateStage {
	return &StateStage{
		docRepo:   docRepo,
		stateRepo: stateRepo,
		relRepo:   relRepo,
		logger:    logger.With().Str("stage", "state").Logger(),
	}
}

// Name returns the stage name.
func (s *StateStage) Name() string {
	return "state"
}

// CanProcess returns true if this stage can process the file entry.
func (s *StateStage) CanProcess(entry *entity.FileEntry) bool {
	// Process all files that have been parsed as documents
	return entry.Enhanced != nil && entry.Enhanced.IndexedState.Document
}

// Process infers and sets the document state.
func (s *StateStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return fmt.Errorf("workspace info not found in context")
	}
	workspaceID := wsInfo.ID

	// Get document
	doc, err := s.docRepo.GetDocumentByPath(ctx, workspaceID, entry.RelativePath)
	if err != nil || doc == nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// Get current state
	currentState, err := s.stateRepo.GetState(ctx, workspaceID, doc.ID)
	if err != nil {
		// Document might not have state set yet - default to draft
		currentState = entity.DocumentStateDraft
	}

	// Infer new state based on relationships
	inferredState := s.inferState(ctx, workspaceID, doc.ID, currentState)

	// Update state if changed
	if inferredState != currentState {
		reason := s.getStateChangeReason(currentState, inferredState)
		if err := s.stateRepo.SetState(ctx, workspaceID, doc.ID, inferredState, reason); err != nil {
			return fmt.Errorf("failed to set document state: %w", err)
		}

		s.logger.Debug().
			Str("path", entry.RelativePath).
			Str("from", currentState.String()).
			Str("to", inferredState.String()).
			Msg("Document state updated")
	}

	return nil
}

// inferState infers the appropriate state for a document.
func (s *StateStage) inferState(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	currentState entity.DocumentState,
) entity.DocumentState {
	// If document is already archived, keep it archived
	if currentState == entity.DocumentStateArchived {
		return currentState
	}

	// Check if this document replaces another (incoming "replaces" relationship)
	incomingReplaces, err := s.relRepo.GetIncoming(ctx, workspaceID, docID, entity.RelationshipReplaces)
	if err == nil && len(incomingReplaces) > 0 {
		// This document has been replaced - mark old state as replaced
		// (This would be handled when the new document is processed)
	}

	// Check if another document replaces this one
	outgoingReplaces, err := s.relRepo.GetOutgoing(ctx, workspaceID, docID, entity.RelationshipReplaces)
	if err == nil && len(outgoingReplaces) > 0 {
		// This document replaces another - mark as active
		return entity.DocumentStateActive
	}

	// Check if this document is being replaced
	incomingReplaces, err = s.relRepo.GetIncoming(ctx, workspaceID, docID, entity.RelationshipReplaces)
	if err == nil && len(incomingReplaces) > 0 {
		// Another document replaces this one - mark as replaced
		return entity.DocumentStateReplaced
	}

	// Default: if new document, set to draft; otherwise keep current or set to active
	if currentState == "" || currentState == entity.DocumentStateDraft {
		// Check if document has been around for a while (has chunks, etc.)
		// For now, default to active for documents that have been processed
		return entity.DocumentStateActive
	}

	return currentState
}

// getStateChangeReason returns a human-readable reason for state change.
func (s *StateStage) getStateChangeReason(from, to entity.DocumentState) string {
	switch {
	case from == "" && to == entity.DocumentStateDraft:
		return "Initial state"
	case from == entity.DocumentStateDraft && to == entity.DocumentStateActive:
		return "Document processed and ready"
	case to == entity.DocumentStateReplaced:
		return "Document has been replaced by another version"
	case to == entity.DocumentStateArchived:
		return "Document archived"
	default:
		return fmt.Sprintf("State changed from %s to %s", from, to)
	}
}

