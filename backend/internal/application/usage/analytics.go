package usage

import (
	"context"
	"sort"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Analytics provides usage analytics and insights.
type Analytics struct {
	usageRepo repository.UsageRepository
}

// NewAnalytics creates a new usage analytics service.
func NewAnalytics(usageRepo repository.UsageRepository) *Analytics {
	return &Analytics{
		usageRepo: usageRepo,
	}
}

// GetUsageStats retrieves usage statistics for a document.
func (a *Analytics) GetUsageStats(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	since time.Time,
) (*entity.DocumentUsageStats, error) {
	return a.usageRepo.GetUsageStats(ctx, workspaceID, docID, since)
}

// GetCoOccurringDocuments finds documents that are frequently used together.
func (a *Analytics) GetCoOccurringDocuments(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	limit int,
	since time.Time,
) ([]entity.DocumentID, error) {
	coOccurrences, err := a.usageRepo.GetCoOccurrences(ctx, workspaceID, docID, limit, since)
	if err != nil {
		return nil, err
	}

	// Sort by count (descending) and return document IDs
	type docCount struct {
		docID entity.DocumentID
		count int
	}
	var sorted []docCount
	for docID, count := range coOccurrences {
		sorted = append(sorted, docCount{docID: docID, count: count})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	result := make([]entity.DocumentID, 0, len(sorted))
	for _, item := range sorted {
		result = append(result, item.docID)
	}

	return result, nil
}

// GetFrequentlyUsedDocuments returns the most frequently accessed documents.
func (a *Analytics) GetFrequentlyUsedDocuments(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	since time.Time,
	limit int,
) ([]entity.DocumentID, error) {
	return a.usageRepo.GetFrequentlyUsed(ctx, workspaceID, since, limit)
}

// GetRecentlyUsedDocuments returns the most recently accessed documents.
func (a *Analytics) GetRecentlyUsedDocuments(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	limit int,
) ([]entity.DocumentID, error) {
	return a.usageRepo.GetRecentlyUsed(ctx, workspaceID, limit)
}

// GetUsageFrequency calculates the access frequency (per day) for a document.
func (a *Analytics) GetUsageFrequency(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	since time.Time,
) (float64, error) {
	stats, err := a.GetUsageStats(ctx, workspaceID, docID, since)
	if err != nil {
		return 0, err
	}
	return stats.Frequency, nil
}

// GetKnowledgeClusters identifies clusters of documents that are used together.
func (a *Analytics) GetKnowledgeClusters(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	minClusterSize int,
	since time.Time,
) (map[entity.DocumentID][]entity.DocumentID, error) {
	// Get all frequently used documents
	frequentDocs, err := a.GetFrequentlyUsedDocuments(ctx, workspaceID, since, 100)
	if err != nil {
		return nil, err
	}

	clusters := make(map[entity.DocumentID][]entity.DocumentID)

	for _, docID := range frequentDocs {
		coOccurring, err := a.GetCoOccurringDocuments(ctx, workspaceID, docID, 10, since)
		if err != nil {
			continue
		}

		if len(coOccurring) >= minClusterSize {
			clusters[docID] = coOccurring
		}
	}

	return clusters, nil
}

// GetDocumentsUsedTogether returns documents that are typically used together with the given document.
func (a *Analytics) GetDocumentsUsedTogether(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	limit int,
) ([]entity.DocumentID, error) {
	since := time.Now().AddDate(0, -1, 0) // Last month
	return a.GetCoOccurringDocuments(ctx, workspaceID, docID, limit, since)
}

