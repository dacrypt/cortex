// Package clustering provides document clustering and community detection services.
package clustering

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// VectorStoreEmbeddingConnection implements EmbeddingConnection using VectorStore and DocumentRepository.
type VectorStoreEmbeddingConnection struct {
	vectorStore repository.VectorStore
	docRepo     repository.DocumentRepository
	conn        SQLConnection
	logger      zerolog.Logger
}

// SQLConnection provides direct SQL access for querying embeddings.
type SQLConnection interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// NewVectorStoreEmbeddingConnection creates a new embedding connection.
func NewVectorStoreEmbeddingConnection(
	vectorStore repository.VectorStore,
	docRepo repository.DocumentRepository,
	conn SQLConnection,
	logger zerolog.Logger,
) *VectorStoreEmbeddingConnection {
	return &VectorStoreEmbeddingConnection{
		vectorStore: vectorStore,
		docRepo:     docRepo,
		conn:        conn,
		logger:      logger.With().Str("component", "embedding_connection").Logger(),
	}
}

// GetAllEmbeddings retrieves all embeddings for a workspace.
func (c *VectorStoreEmbeddingConnection) GetAllEmbeddings(ctx context.Context, workspaceID entity.WorkspaceID) ([]DocumentEmbedding, error) {
	if c.conn == nil {
		return nil, nil
	}

	// Query chunk_embeddings joined with chunks to get document_id
	query := `
		SELECT 
			ce.chunk_id,
			c.document_id,
			ce.dimensions,
			ce.vector
		FROM chunk_embeddings ce
		INNER JOIN chunks c ON ce.workspace_id = c.workspace_id AND ce.chunk_id = c.id
		WHERE ce.workspace_id = ?
	`

	rows, err := c.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	var embeddings []DocumentEmbedding
	for rows.Next() {
		var (
			chunkID    string
			documentID string
			dims       int
			vectorBlob []byte
		)

		if err := rows.Scan(&chunkID, &documentID, &dims, &vectorBlob); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to scan embedding row")
			continue
		}

		// Decode vector
		vector, err := decodeVector(vectorBlob, dims)
		if err != nil {
			c.logger.Warn().Err(err).Str("chunk_id", chunkID).Msg("Failed to decode vector")
			continue
		}

		embeddings = append(embeddings, DocumentEmbedding{
			DocumentID: entity.DocumentID(documentID),
			ChunkID:    entity.ChunkID(chunkID),
			Vector:     vector,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating embeddings: %w", err)
	}

	c.logger.Debug().
		Int("count", len(embeddings)).
		Msg("Retrieved embeddings from database")

	return embeddings, nil
}

// decodeVector decodes a binary vector blob to float32 slice.
func decodeVector(data []byte, dims int) ([]float32, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("vector data is empty")
	}

	expectedSize := dims * 4 // 4 bytes per float32
	if len(data) < expectedSize {
		return nil, fmt.Errorf("vector data too short: expected %d bytes, got %d", expectedSize, len(data))
	}

	vector := make([]float32, dims)
	for i := 0; i < dims; i++ {
		if i*4+4 > len(data) {
			break
		}
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		vector[i] = math.Float32frombits(bits)
	}

	return vector, nil
}


