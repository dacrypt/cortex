package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// VectorStore implements repository.VectorStore using SQLite.
// It uses an optional in-memory HNSW index per workspace for fast approximate
// nearest neighbor search, falling back to brute-force when the index is not loaded.
type VectorStore struct {
	conn    *Connection
	indices sync.Map // map[string]*HNSWIndex keyed by workspace ID
}

// NewVectorStore creates a new SQLite vector store.
func NewVectorStore(conn *Connection) *VectorStore {
	return &VectorStore{conn: conn}
}

// getOrLoadIndex returns the HNSW index for a workspace, loading it from
// SQLite on the first call. Returns nil if loading fails or there are no vectors.
func (s *VectorStore) getOrLoadIndex(ctx context.Context, workspaceID entity.WorkspaceID) *HNSWIndex {
	wsKey := workspaceID.String()
	if val, ok := s.indices.Load(wsKey); ok {
		return val.(*HNSWIndex)
	}

	// Load all vectors from SQLite
	rows, err := s.conn.Query(ctx, `
		SELECT chunk_id, dimensions, vector
		FROM chunk_embeddings
		WHERE workspace_id = ?
	`, wsKey)
	if err != nil {
		return nil
	}
	defer rows.Close()

	vectors := make(map[string][]float32)
	dim := 0
	for rows.Next() {
		var (
			chunkID string
			dims    int
			data    []byte
		)
		if err := rows.Scan(&chunkID, &dims, &data); err != nil {
			continue
		}
		vec, err := decodeVector(data, dims)
		if err != nil || len(vec) == 0 {
			continue
		}
		vectors[chunkID] = vec
		if dim == 0 {
			dim = dims
		}
	}

	if len(vectors) == 0 {
		return nil
	}

	idx := NewHNSWIndex(dim)
	idx.Load(vectors)
	s.indices.Store(wsKey, idx)
	return idx
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

	err := s.conn.Transaction(ctx, func(tx *sql.Tx) error {
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
	if err != nil {
		return err
	}

	// Sync in-memory HNSW index
	if val, ok := s.indices.Load(workspaceID.String()); ok {
		idx := val.(*HNSWIndex)
		for _, emb := range embeddings {
			idx.Insert(emb.ChunkID.String(), emb.Vector)
		}
	}

	return nil
}

// DeleteByDocument removes embeddings for a document's chunks.
func (s *VectorStore) DeleteByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) error {
	// Collect chunk IDs before deletion (needed to sync the HNSW index).
	rows, err := s.conn.Query(ctx,
		`SELECT id FROM chunks WHERE workspace_id = ? AND document_id = ?`,
		workspaceID.String(), documentID.String())
	if err != nil {
		return err
	}
	var chunkIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		chunkIDs = append(chunkIDs, id)
	}
	rows.Close()

	// Delete from SQLite.
	query := `
		DELETE FROM chunk_embeddings
		WHERE workspace_id = ? AND chunk_id IN (
			SELECT id FROM chunks WHERE workspace_id = ? AND document_id = ?
		)
	`
	if _, err := s.conn.Exec(ctx, query, workspaceID.String(), workspaceID.String(), documentID.String()); err != nil {
		return err
	}

	// Sync in-memory HNSW index.
	if val, ok := s.indices.Load(workspaceID.String()); ok {
		idx := val.(*HNSWIndex)
		for _, id := range chunkIDs {
			idx.Delete(id)
		}
	}

	return nil
}

// Search performs a cosine similarity search. Uses the in-memory HNSW index
// for O(log n) approximate search when available, falling back to brute-force.
func (s *VectorStore) Search(ctx context.Context, workspaceID entity.WorkspaceID, query []float32, topK int) ([]repository.VectorMatch, error) {
	if len(query) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}

	// Try HNSW index first
	if idx := s.getOrLoadIndex(ctx, workspaceID); idx != nil && idx.Size() > 0 {
		return idx.Search(query, topK), nil
	}

	// Fallback: brute-force scan
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
