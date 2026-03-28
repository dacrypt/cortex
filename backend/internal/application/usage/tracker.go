package usage

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Tracker records and manages document usage events.
type Tracker struct {
	usageRepo repository.UsageRepository
}

// NewTracker creates a new usage tracker.
func NewTracker(usageRepo repository.UsageRepository) *Tracker {
	return &Tracker{
		usageRepo: usageRepo,
	}
}

// RecordOpen records that a document was opened/viewed.
func (t *Tracker) RecordOpen(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	context string,
) error {
	event := entity.NewDocumentUsageEvent(workspaceID, docID, entity.UsageEventOpened)
	if context != "" {
		event.WithContext(context)
	}
	return t.usageRepo.RecordEvent(ctx, workspaceID, event)
}

// RecordEdit records that a document was edited.
func (t *Tracker) RecordEdit(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	context string,
) error {
	event := entity.NewDocumentUsageEvent(workspaceID, docID, entity.UsageEventEdited)
	if context != "" {
		event.WithContext(context)
	}
	return t.usageRepo.RecordEvent(ctx, workspaceID, event)
}

// RecordSearch records that a document was found via search.
func (t *Tracker) RecordSearch(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	query string,
) error {
	event := entity.NewDocumentUsageEvent(workspaceID, docID, entity.UsageEventSearched)
	event.WithContext(fmt.Sprintf("search:%s", query))
	return t.usageRepo.RecordEvent(ctx, workspaceID, event)
}

// RecordReference records that a document was referenced (linked to).
func (t *Tracker) RecordReference(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	fromDocID entity.DocumentID,
) error {
	event := entity.NewDocumentUsageEvent(workspaceID, docID, entity.UsageEventReferenced)
	event.WithContext(fmt.Sprintf("from:%s", fromDocID.String()))
	return t.usageRepo.RecordEvent(ctx, workspaceID, event)
}

// RecordIndexed records that a document was indexed/processed.
func (t *Tracker) RecordIndexed(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) error {
	event := entity.NewDocumentUsageEvent(workspaceID, docID, entity.UsageEventIndexed)
	return t.usageRepo.RecordEvent(ctx, workspaceID, event)
}

// RecordBatch records multiple usage events efficiently.
func (t *Tracker) RecordBatch(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	events []*entity.DocumentUsageEvent,
) error {
	return t.usageRepo.RecordEvents(ctx, workspaceID, events)
}

