package rag

import (
	"context"

	"github.com/tmc/langchaingo/schema"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// CortexRetriever implements langchaingo's schema.Retriever interface
// to enable use of langchaingo's RetrievalQA chain with our vector store.
type CortexRetriever struct {
	docRepo     repository.DocumentRepository
	vectorStore repository.VectorStore
	embedder    embedding.Embedder
	workspaceID entity.WorkspaceID
	topK        int
}

// NewCortexRetriever creates a new retriever that uses our vector store.
func NewCortexRetriever(
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	workspaceID entity.WorkspaceID,
	topK int,
) *CortexRetriever {
	if topK <= 0 {
		topK = 5
	}
	return &CortexRetriever{
		docRepo:     docRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		workspaceID: workspaceID,
		topK:        topK,
	}
}

// GetRelevantDocuments implements schema.Retriever interface.
// It performs semantic search using our vector store and returns langchaingo Documents.
func (r *CortexRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]schema.Document, error) {
	// Create embedding for query
	vector, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Search vector store
	matches, err := r.vectorStore.Search(ctx, r.workspaceID, vector, r.topK)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return []schema.Document{}, nil
	}

	// Get chunk IDs
	chunkIDs := make([]entity.ChunkID, 0, len(matches))
	for _, match := range matches {
		chunkIDs = append(chunkIDs, match.ChunkID)
	}

	// Get chunks from repository
	chunks, err := r.docRepo.GetChunksByIDs(ctx, r.workspaceID, chunkIDs)
	if err != nil {
		return nil, err
	}

	// Create a map for quick lookup
	chunkByID := make(map[entity.ChunkID]*entity.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkByID[chunk.ID] = chunk
	}

	// Get documents for relative paths
	docCache := make(map[entity.DocumentID]*entity.Document)
	
	// Convert to langchaingo Documents
	docs := make([]schema.Document, 0, len(matches))
	for _, match := range matches {
		chunk := chunkByID[match.ChunkID]
		if chunk == nil {
			continue
		}

		// Get document if not cached
		doc := docCache[chunk.DocumentID]
		if doc == nil {
			doc, err = r.docRepo.GetDocument(ctx, r.workspaceID, chunk.DocumentID)
			if err != nil || doc == nil {
				continue
			}
			docCache[chunk.DocumentID] = doc
		}

		// Create snippet (truncate if needed)
		snippet := chunk.Text
		if len(snippet) > 400 {
			snippet = snippet[:400] + "..."
		}

		// Convert to langchaingo Document
		docs = append(docs, schema.Document{
			PageContent: snippet,
			Metadata: map[string]interface{}{
				"document_id":   chunk.DocumentID.String(),
				"chunk_id":      chunk.ID.String(),
				"relative_path": doc.RelativePath,
				"heading_path":  chunk.HeadingPath,
				"score":         match.Similarity,
			},
		})
	}

	return docs, nil
}






