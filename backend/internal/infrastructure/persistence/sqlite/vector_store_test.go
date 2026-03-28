package sqlite

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func setupVectorStore(t *testing.T) (*VectorStore, *Connection) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewVectorStore(conn)
	return store, conn
}

func TestVectorStore_Upsert_And_Search(t *testing.T) {
	t.Parallel()

	store, _ := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-vector-1")
	now := time.Now()

	// Insert 3 vectors with known values for deterministic cosine similarity.
	// cos([1,0,0], [1,0,0])     = 1.0
	// cos([1,0,0], [0.9,0.1,0]) ≈ 0.994
	// cos([1,0,0], [0,1,0])     = 0.0
	embeddings := []entity.ChunkEmbedding{
		{ChunkID: "chunk-a", Vector: []float32{1, 0, 0}, Dimensions: 3, UpdatedAt: now},
		{ChunkID: "chunk-b", Vector: []float32{0, 1, 0}, Dimensions: 3, UpdatedAt: now},
		{ChunkID: "chunk-c", Vector: []float32{0.9, 0.1, 0}, Dimensions: 3, UpdatedAt: now},
	}

	for _, emb := range embeddings {
		if err := store.Upsert(ctx, wsID, emb); err != nil {
			t.Fatalf("upsert %s: %v", emb.ChunkID, err)
		}
	}

	results, err := store.Search(ctx, wsID, []float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// First result should be chunk-a (exact match, similarity=1.0)
	if results[0].ChunkID != "chunk-a" {
		t.Errorf("expected first result chunk-a, got %s", results[0].ChunkID)
	}
	if math.Abs(float64(results[0].Similarity-1.0)) > 0.001 {
		t.Errorf("expected similarity ~1.0 for chunk-a, got %f", results[0].Similarity)
	}

	// Second result should be chunk-c ([0.9,0.1,0])
	if results[1].ChunkID != "chunk-c" {
		t.Errorf("expected second result chunk-c, got %s", results[1].ChunkID)
	}
	if results[1].Similarity < 0.99 {
		t.Errorf("expected high similarity for chunk-c, got %f", results[1].Similarity)
	}

	// Third result should be chunk-b ([0,1,0]) with similarity=0.0
	if results[2].ChunkID != "chunk-b" {
		t.Errorf("expected third result chunk-b, got %s", results[2].ChunkID)
	}
	if math.Abs(float64(results[2].Similarity)) > 0.001 {
		t.Errorf("expected similarity ~0.0 for chunk-b, got %f", results[2].Similarity)
	}
}

func TestVectorStore_BulkUpsert(t *testing.T) {
	t.Parallel()

	store, _ := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-bulk-1")
	now := time.Now()

	embeddings := make([]entity.ChunkEmbedding, 5)
	for i := range embeddings {
		vec := make([]float32, 4)
		vec[i%4] = 1.0
		embeddings[i] = entity.ChunkEmbedding{
			ChunkID:    entity.ChunkID(fmt.Sprintf("bulk-chunk-%d", i)),
			Vector:     vec,
			Dimensions: 4,
			UpdatedAt:  now,
		}
	}

	if err := store.BulkUpsert(ctx, wsID, embeddings); err != nil {
		t.Fatalf("bulk upsert: %v", err)
	}

	// Search with a vector that matches all and verify we get 5 results.
	results, err := store.Search(ctx, wsID, []float32{1, 1, 1, 1}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// Verify all chunk IDs are present.
	found := make(map[entity.ChunkID]bool)
	for _, r := range results {
		found[r.ChunkID] = true
	}
	for _, emb := range embeddings {
		if !found[emb.ChunkID] {
			t.Errorf("chunk %s not found in results", emb.ChunkID)
		}
	}
}

func TestVectorStore_DeleteByDocument(t *testing.T) {
	t.Parallel()

	store, conn := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-delete-1")
	now := time.Now()
	nowMs := now.UnixMilli()

	docA := entity.DocumentID("doc-a")
	docB := entity.DocumentID("doc-b")

	// Insert chunks into the chunks table so the subquery in DeleteByDocument works.
	chunksSQL := `INSERT INTO chunks (workspace_id, id, document_id, ordinal, heading, heading_path, text, token_count, start_line, end_line, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for i, chunk := range []struct {
		id  string
		doc entity.DocumentID
	}{
		{"chunk-da-1", docA},
		{"chunk-da-2", docA},
		{"chunk-db-1", docB},
	} {
		if _, err := conn.Exec(ctx, chunksSQL, wsID.String(), chunk.id, chunk.doc.String(), i, "test", "test", "text", 10, 0, 1, nowMs, nowMs); err != nil {
			t.Fatalf("insert chunk %s: %v", chunk.id, err)
		}
	}

	// Insert embeddings for both documents.
	embeddings := []entity.ChunkEmbedding{
		{ChunkID: "chunk-da-1", Vector: []float32{1, 0, 0}, Dimensions: 3, UpdatedAt: now},
		{ChunkID: "chunk-da-2", Vector: []float32{0, 1, 0}, Dimensions: 3, UpdatedAt: now},
		{ChunkID: "chunk-db-1", Vector: []float32{0, 0, 1}, Dimensions: 3, UpdatedAt: now},
	}
	if err := store.BulkUpsert(ctx, wsID, embeddings); err != nil {
		t.Fatalf("bulk upsert: %v", err)
	}

	// Delete embeddings for docA.
	if err := store.DeleteByDocument(ctx, wsID, docA); err != nil {
		t.Fatalf("delete by document: %v", err)
	}

	// Search should only return chunk-db-1.
	results, err := store.Search(ctx, wsID, []float32{1, 1, 1}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after delete, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-db-1" {
		t.Errorf("expected chunk-db-1, got %s", results[0].ChunkID)
	}
}

func TestVectorStore_Search_Empty(t *testing.T) {
	t.Parallel()

	store, _ := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-empty-1")

	results, err := store.Search(ctx, wsID, []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results on empty store, got %d", len(results))
	}
}

func TestVectorStore_Search_TopK_Limit(t *testing.T) {
	t.Parallel()

	store, _ := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-topk-1")
	now := time.Now()

	embeddings := make([]entity.ChunkEmbedding, 10)
	for i := range embeddings {
		vec := make([]float32, 3)
		// Create distinct vectors with varying similarity to [1,0,0].
		vec[0] = float32(10-i) / 10.0
		vec[1] = float32(i) / 10.0
		vec[2] = 0
		embeddings[i] = entity.ChunkEmbedding{
			ChunkID:    entity.ChunkID(fmt.Sprintf("topk-chunk-%d", i)),
			Vector:     vec,
			Dimensions: 3,
			UpdatedAt:  now,
		}
	}

	if err := store.BulkUpsert(ctx, wsID, embeddings); err != nil {
		t.Fatalf("bulk upsert: %v", err)
	}

	results, err := store.Search(ctx, wsID, []float32{1, 0, 0}, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected exactly 3 results with topK=3, got %d", len(results))
	}

	// Results should be sorted by descending similarity.
	for i := 1; i < len(results); i++ {
		if results[i].Similarity > results[i-1].Similarity {
			t.Errorf("results not sorted: index %d similarity %f > index %d similarity %f",
				i, results[i].Similarity, i-1, results[i-1].Similarity)
		}
	}
}

func TestVectorStore_Upsert_Conflict(t *testing.T) {
	t.Parallel()

	store, _ := setupVectorStore(t)
	ctx := context.Background()
	wsID := entity.WorkspaceID("ws-conflict-1")
	now := time.Now()

	// First upsert: chunk points toward [1,0,0].
	emb1 := entity.ChunkEmbedding{
		ChunkID:    "chunk-conflict",
		Vector:     []float32{1, 0, 0},
		Dimensions: 3,
		UpdatedAt:  now,
	}
	if err := store.Upsert(ctx, wsID, emb1); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}

	// Second upsert: same chunkID, different vector pointing toward [0,1,0].
	emb2 := entity.ChunkEmbedding{
		ChunkID:    "chunk-conflict",
		Vector:     []float32{0, 1, 0},
		Dimensions: 3,
		UpdatedAt:  now.Add(time.Second),
	}
	if err := store.Upsert(ctx, wsID, emb2); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	// Search with [0,1,0] should return high similarity (the updated vector).
	results, err := store.Search(ctx, wsID, []float32{0, 1, 0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-conflict" {
		t.Errorf("expected chunk-conflict, got %s", results[0].ChunkID)
	}
	if math.Abs(float64(results[0].Similarity-1.0)) > 0.001 {
		t.Errorf("expected similarity ~1.0 after conflict update, got %f", results[0].Similarity)
	}

	// Also verify searching with the old vector [1,0,0] returns low similarity.
	results2, err := store.Search(ctx, wsID, []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search old vector: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results2))
	}
	if math.Abs(float64(results2[0].Similarity)) > 0.001 {
		t.Errorf("expected similarity ~0.0 for old vector, got %f", results2[0].Similarity)
	}
}

// Ensure VectorStore satisfies the repository interface.
var _ repository.VectorStore = (*VectorStore)(nil)
