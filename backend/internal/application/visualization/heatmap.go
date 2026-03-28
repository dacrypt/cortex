package visualization

import (
	"context"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DocumentInfo contains basic information about a document for heatmap visualization.
type DocumentInfo struct {
	ID    entity.DocumentID
	Path  string
	Title string
}

// TimeRange represents a time range for heatmap data.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// HeatmapData contains co-occurrence matrix data for visualization.
type HeatmapData struct {
	Documents []DocumentInfo
	Matrix    [][]float64 // Co-occurrence matrix
	TimeRange TimeRange
}

// GenerateHeatmap generates heatmap data based on document usage co-occurrences.
func GenerateHeatmap(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docRepo repository.DocumentRepository,
	usageRepo repository.UsageRepository,
	since time.Time,
	eventType *entity.UsageEventType,
) (*HeatmapData, error) {
	// Get all documents that have usage events since the specified time
	// We'll use GetFrequentlyUsed as a starting point
	docIDs, err := usageRepo.GetFrequentlyUsed(ctx, workspaceID, since, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get frequently used documents: %w", err)
	}

	if len(docIDs) == 0 {
		return &HeatmapData{
			Documents: []DocumentInfo{},
			Matrix:    [][]float64{},
			TimeRange: TimeRange{
				Start: since,
				End:   time.Now(),
			},
		}, nil
	}

	// Build document info list
	docInfos := make([]DocumentInfo, 0, len(docIDs))
	docIndexMap := make(map[entity.DocumentID]int)

	for i, docID := range docIDs {
		doc, err := docRepo.GetDocument(ctx, workspaceID, docID)
		if err != nil {
			continue
		}

		docInfos = append(docInfos, DocumentInfo{
			ID:    docID,
			Path:  doc.RelativePath,
			Title: doc.Title,
		})
		docIndexMap[docID] = i
	}

	// Initialize co-occurrence matrix
	matrix := make([][]float64, len(docInfos))
	for i := range matrix {
		matrix[i] = make([]float64, len(docInfos))
	}

	// Calculate co-occurrences
	maxCoOccurrence := 0
	for _, docID := range docIDs {
		coOccurrences, err := usageRepo.GetCoOccurrences(ctx, workspaceID, docID, 100, since)
		if err != nil {
			continue
		}

		idx, exists := docIndexMap[docID]
		if !exists {
			continue
		}

		for otherDocID, count := range coOccurrences {
			otherIdx, exists := docIndexMap[otherDocID]
			if !exists {
				continue
			}

			// Matrix is symmetric
			matrix[idx][otherIdx] = float64(count)
			matrix[otherIdx][idx] = float64(count)

			if count > maxCoOccurrence {
				maxCoOccurrence = count
			}
		}
	}

	// Normalize matrix to 0.0-1.0 range
	if maxCoOccurrence > 0 {
		for i := range matrix {
			for j := range matrix[i] {
				matrix[i][j] = matrix[i][j] / float64(maxCoOccurrence)
			}
		}
	}

	return &HeatmapData{
		Documents: docInfos,
		Matrix:    matrix,
		TimeRange: TimeRange{
			Start: since,
			End:   time.Now(),
		},
	}, nil
}

