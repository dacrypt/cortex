package repository

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// UsageRepository defines storage for document usage events and analytics.
type UsageRepository interface {
	// Event recording
	RecordEvent(ctx context.Context, workspaceID entity.WorkspaceID, event *entity.DocumentUsageEvent) error
	RecordEvents(ctx context.Context, workspaceID entity.WorkspaceID, events []*entity.DocumentUsageEvent) error

	// Statistics
	GetUsageStats(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, since time.Time) (*entity.DocumentUsageStats, error)
	GetCoOccurrences(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, limit int, since time.Time) (map[entity.DocumentID]int, error)

	// Queries
	GetFrequentlyUsed(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, limit int) ([]entity.DocumentID, error)
	GetRecentlyUsed(ctx context.Context, workspaceID entity.WorkspaceID, limit int) ([]entity.DocumentID, error)
	GetEventsByType(ctx context.Context, workspaceID entity.WorkspaceID, eventType entity.UsageEventType, since time.Time, limit int) ([]*entity.DocumentUsageEvent, error)
	GetEventsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, since time.Time, limit int) ([]*entity.DocumentUsageEvent, error)
}

