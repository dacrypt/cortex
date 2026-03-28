package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// DocumentRepository defines storage for parsed documents and chunks.
type DocumentRepository interface {
	UpsertDocument(ctx context.Context, workspaceID entity.WorkspaceID, doc *entity.Document) error
	GetDocument(ctx context.Context, workspaceID entity.WorkspaceID, id entity.DocumentID) (*entity.Document, error)
	GetDocumentByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.Document, error)

	ReplaceChunks(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID, chunks []*entity.Chunk) error
	GetChunksByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.Chunk, error)
	GetChunksByIDs(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.ChunkID) ([]*entity.Chunk, error)
}
