package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// VectorMatch represents a vector search result.
type VectorMatch struct {
	ChunkID    entity.ChunkID
	Similarity float32
}

// VectorStore defines vector storage and search.
type VectorStore interface {
	Upsert(ctx context.Context, workspaceID entity.WorkspaceID, embedding entity.ChunkEmbedding) error
	BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, embeddings []entity.ChunkEmbedding) error
	DeleteByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) error
	Search(ctx context.Context, workspaceID entity.WorkspaceID, query []float32, topK int) ([]VectorMatch, error)
}
