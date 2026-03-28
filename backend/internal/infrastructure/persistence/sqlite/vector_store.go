package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// VectorStore implements repository.VectorStore using SQLite.
type VectorStore struct {
	conn *Connection
}

// NewVectorStore creates a new SQLite vector store.
func NewVectorStore(conn *Connection) *VectorStore {
	return &VectorStore{conn: conn}
}

// Upsert inserts or updates a chunk embedding.
func (s *VectorStore) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, embedding entity.ChunkEmbedding) error {
	return s.BulkUpsert(ctx, workspaceID, []entity.ChunkEmbedding{embedding})
}

// BulkUpsert inserts or updates multiple embeddings.
func (s *VectorStore) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, embeddings []entity.ChunkEmbedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	return s.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO chunk_embeddings (workspace_id, chunk_id, dimensions, vector, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, chunk_id) DO UPDATE SET
				dimensions = excluded.dimensions,
				vector = excluded.vector,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, embedding := range embeddings {
			vectorBlob, err := encodeVector(embedding.Vector)
			if err != nil {
				return err
			}
			dims := embedding.Dimensions
			if dims == 0 {
				dims = len(embedding.Vector)
			}
			updatedAt := embedding.UpdatedAt
			if updatedAt.IsZero() {
				updatedAt = time.Now()
			}
			if _, err := stmt.ExecContext(ctx,
				workspaceID.String(),
				embedding.ChunkID.String(),
				dims,
				vectorBlob,
				updatedAt.UnixMilli(),
			); err != nil {
				return err
			}
		}

		return nil
	})
}

// DeleteByDocument removes embeddings for a document's chunks.
func (s *VectorStore) DeleteByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) error {
	query := `
		DELETE FROM chunk_embeddings
		WHERE workspace_id = ? AND chunk_id IN (
			SELECT id FROM chunks WHERE workspace_id = ? AND document_id = ?
		)
	`
	_, err := s.conn.Exec(ctx, query, workspaceID.String(), workspaceID.String(), documentID.String())
	return err
}

// Search performs a brute-force cosine similarity search.
func (s *VectorStore) Search(ctx context.Context, workspaceID entity.WorkspaceID, query []float32, topK int) ([]repository.VectorMatch, error) {
	if len(query) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}

	rows, err := s.conn.Query(ctx, `
		SELECT chunk_id, dimensions, vector
		FROM chunk_embeddings
		WHERE workspace_id = ?
	`, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]repository.VectorMatch, 0, topK)
	for rows.Next() {
		var (
			chunkID string
			dims    int
			vector  []byte
		)
		if err := rows.Scan(&chunkID, &dims, &vector); err != nil {
			return nil, err
		}

		decoded, err := decodeVector(vector, dims)
		if err != nil || len(decoded) == 0 {
			continue
		}

		if len(decoded) != len(query) {
			continue
		}

		score := cosineSimilarity(query, decoded)
		matches = append(matches, repository.VectorMatch{
			ChunkID:    entity.ChunkID(chunkID),
			Similarity: score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Similarity > matches[j].Similarity
	})
	if len(matches) > topK {
		matches = matches[:topK]
	}

	return matches, nil
}

func encodeVector(vector []float32) ([]byte, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector is empty")
	}
	buf := make([]byte, 4*len(vector))
	for i, v := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf, nil
}

func decodeVector(data []byte, dims int) ([]float32, error) {
	if dims <= 0 {
		return nil, fmt.Errorf("invalid dimensions")
	}
	expected := dims * 4
	if len(data) < expected {
		return nil, fmt.Errorf("vector size mismatch")
	}
	out := make([]float32, dims)
	for i := 0; i < dims; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return out, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(normA*normB)))
}

var _ repository.VectorStore = (*VectorStore)(nil)
