package metadata

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/rs/zerolog"
)

// DuplicateDetector detects duplicate and similar documents.
type DuplicateDetector struct {
	vectorStore repository.VectorStore
	docRepo     repository.DocumentRepository
	embedder    interface { // Embedder interface - we'll use embedding.Embedder
		Embed(ctx context.Context, text string) ([]float32, error)
	}
	logger    zerolog.Logger
	threshold float64 // Similarity threshold (0.0 to 1.0)
}

// NewDuplicateDetector creates a new duplicate detector.
func NewDuplicateDetector(
	vectorStore repository.VectorStore,
	docRepo repository.DocumentRepository,
	embedder interface {
		Embed(ctx context.Context, text string) ([]float32, error)
	},
	logger zerolog.Logger,
	threshold float64,
) *DuplicateDetector {
	if threshold <= 0 {
		threshold = 0.85 // Default threshold
	}
	return &DuplicateDetector{
		vectorStore: vectorStore,
		docRepo:     docRepo,
		embedder:    embedder,
		logger:      logger.With().Str("component", "duplicate_detector").Logger(),
		threshold:   threshold,
	}
}

// FindDuplicates finds duplicate or similar documents for a given document.
func (d *DuplicateDetector) FindDuplicates(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	relativePath string,
) ([]entity.DuplicateInfo, error) {
	// Get document chunks
	chunks, err := d.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	// Get chunk IDs
	chunkIDs := make([]entity.ChunkID, len(chunks))
	for i, chunk := range chunks {
		chunkIDs[i] = chunk.ID
	}

	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// Get first chunk to use as query
	queryChunks, err := d.docRepo.GetChunksByIDs(ctx, workspaceID, chunkIDs[:1])
	if err != nil || len(queryChunks) == 0 {
		return nil, fmt.Errorf("failed to get query chunk: %w", err)
	}

	// Generate embedding for query chunk
	if d.embedder == nil {
		return nil, fmt.Errorf("embedder not available")
	}

	queryVector, err := d.embedder.Embed(ctx, queryChunks[0].Text)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query chunk: %w", err)
	}

	// Search for similar documents
	similarDocs, err := d.vectorStore.Search(ctx, workspaceID, queryVector, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar documents: %w", err)
	}

	// Get all chunk IDs from results
	resultChunkIDs := make([]entity.ChunkID, len(similarDocs))
	for i, result := range similarDocs {
		resultChunkIDs[i] = result.ChunkID
	}

	// Get chunks to find their documents
	resultChunks, err := d.docRepo.GetChunksByIDs(ctx, workspaceID, resultChunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get result chunks: %w", err)
	}

	// Filter out the document itself and group by document
	docSimilarities := make(map[entity.DocumentID]float32)
	docPaths := make(map[entity.DocumentID]string)

	for i, result := range similarDocs {
		if i >= len(resultChunks) {
			continue
		}
		chunk := resultChunks[i]

		// Skip if it's the same document
		if chunk.DocumentID == docID {
			continue
		}

		// Get document to get path
		doc, err := d.docRepo.GetDocument(ctx, workspaceID, chunk.DocumentID)
		if err != nil {
			continue
		}

		// Track maximum similarity for each document
		if currentSim, exists := docSimilarities[chunk.DocumentID]; !exists || result.Similarity > currentSim {
			docSimilarities[chunk.DocumentID] = result.Similarity
			docPaths[chunk.DocumentID] = doc.RelativePath
		}
	}

	// Convert to DuplicateInfo
	var duplicates []entity.DuplicateInfo
	for docID, similarity := range docSimilarities {
		similarityFloat := float64(similarity)
		if similarityFloat >= float64(d.threshold) {
			duplicateType := "near"
			if similarityFloat >= 0.95 {
				duplicateType = "exact"
			} else if similarityFloat >= 0.70 && similarityFloat < 0.85 {
				duplicateType = "version"
			}

			duplicates = append(duplicates, entity.DuplicateInfo{
				DocumentID:   docID,
				RelativePath: docPaths[docID],
				Similarity:   similarityFloat,
				Type:         duplicateType,
				Reason:       fmt.Sprintf("Similarity score: %.2f", similarityFloat),
			})
		}
	}

	return duplicates, nil
}


