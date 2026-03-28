package sqlite

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// randVector generates a random float32 vector of the given dimension.
func randVector(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rng.Float32()*2 - 1
	}
	return v
}

// bruteForceTopK computes exact top-K by cosine similarity.
func bruteForceTopK(query []float32, vectors map[string][]float32, k int) []repository.VectorMatch {
	type scored struct {
		id  string
		sim float32
	}
	all := make([]scored, 0, len(vectors))
	for id, vec := range vectors {
		all = append(all, scored{id: id, sim: hnswCosineSimilarity(query, vec)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].sim > all[j].sim })
	if len(all) > k {
		all = all[:k]
	}
	results := make([]repository.VectorMatch, len(all))
	for i, s := range all {
		results[i] = repository.VectorMatch{
			ChunkID:    entity.ChunkID(s.id),
			Similarity: s.sim,
		}
	}
	return results
}

func TestHNSW_InsertAndSearch(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(123))
	dim := 64
	idx := NewHNSWIndex(dim)

	vectors := make(map[string][]float32, 100)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("vec-%d", i)
		v := randVector(rng, dim)
		vectors[id] = v
		idx.Insert(id, v)
	}

	if idx.Size() != 100 {
		t.Fatalf("expected size 100, got %d", idx.Size())
	}

	// Search for vec-42 using its own vector as query.
	query := vectors["vec-42"]
	results := idx.Search(query, 5)

	if len(results) == 0 {
		t.Fatal("expected at least one result, got none")
	}

	if string(results[0].ChunkID) != "vec-42" {
		t.Errorf("expected top result to be vec-42, got %s", results[0].ChunkID)
	}

	if results[0].Similarity < 0.999 {
		t.Errorf("expected similarity ~1.0 for self-search, got %f", results[0].Similarity)
	}
}

func TestHNSW_RecallAt10(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(456))
	dim := 128
	n := 500
	idx := NewHNSWIndex(dim)

	vectors := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("v-%d", i)
		v := randVector(rng, dim)
		vectors[id] = v
		idx.Insert(id, v)
	}

	totalRecall := 0.0
	numQueries := 10
	topK := 10

	for q := 0; q < numQueries; q++ {
		query := randVector(rng, dim)
		hnswResults := idx.Search(query, topK)
		bfResults := bruteForceTopK(query, vectors, topK)

		// Build set of brute-force top-K IDs.
		bfSet := make(map[entity.ChunkID]bool, topK)
		for _, m := range bfResults {
			bfSet[m.ChunkID] = true
		}

		hits := 0
		for _, m := range hnswResults {
			if bfSet[m.ChunkID] {
				hits++
			}
		}
		totalRecall += float64(hits) / float64(topK)
	}

	avgRecall := totalRecall / float64(numQueries)
	if avgRecall < 0.8 {
		t.Errorf("average recall@10 = %.2f, expected > 0.80", avgRecall)
	}
}

func TestHNSW_Delete(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(789))
	dim := 32
	idx := NewHNSWIndex(dim)

	vectors := make(map[string][]float32, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("d-%d", i)
		v := randVector(rng, dim)
		vectors[id] = v
		idx.Insert(id, v)
	}

	if idx.Size() != 5 {
		t.Fatalf("expected size 5, got %d", idx.Size())
	}

	// Delete d-2.
	idx.Delete("d-2")

	if idx.Size() != 4 {
		t.Fatalf("expected size 4 after delete, got %d", idx.Size())
	}

	// Search for d-2's vector; it should not appear in results.
	results := idx.Search(vectors["d-2"], 5)
	for _, r := range results {
		if string(r.ChunkID) == "d-2" {
			t.Errorf("deleted vector d-2 should not appear in results")
		}
	}
}

func TestHNSW_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	dim := 32
	idx := NewHNSWIndex(dim)

	// Pre-insert some vectors so searches have data.
	baseRng := rand.New(rand.NewSource(100))
	for i := 0; i < 50; i++ {
		idx.Insert(fmt.Sprintf("base-%d", i), randVector(baseRng, dim))
	}

	var wg sync.WaitGroup
	goroutines := 10
	opsPerGoroutine := 50

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(int64(gID * 1000)))
			for i := 0; i < opsPerGoroutine; i++ {
				id := fmt.Sprintf("g%d-%d", gID, i)
				vec := randVector(localRng, dim)
				if i%2 == 0 {
					idx.Insert(id, vec)
				} else {
					idx.Search(vec, 5)
				}
			}
		}(g)
	}

	wg.Wait()
	// If we reach here without panic or race detector failure, the test passes.
	if idx.Size() < 50 {
		t.Errorf("expected at least 50 vectors, got %d", idx.Size())
	}
}

func TestHNSW_Load(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(321))
	dim := 64
	n := 200
	idx := NewHNSWIndex(dim)

	vectors := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vectors[fmt.Sprintf("load-%d", i)] = randVector(rng, dim)
	}

	idx.Load(vectors)

	if idx.Size() != n {
		t.Fatalf("expected size %d after Load, got %d", n, idx.Size())
	}

	// Verify search works: query with a known vector.
	query := vectors["load-50"]
	results := idx.Search(query, 5)

	if len(results) == 0 {
		t.Fatal("expected results after Load, got none")
	}

	// The vector itself should be top result or very close.
	found := false
	for _, r := range results {
		if string(r.ChunkID) == "load-50" {
			found = true
			if r.Similarity < 0.99 {
				t.Errorf("expected similarity ~1.0, got %f", r.Similarity)
			}
			break
		}
	}
	if !found {
		t.Error("expected load-50 in top-5 results when querying its own vector")
	}
}

func TestHNSW_Empty(t *testing.T) {
	t.Parallel()

	idx := NewHNSWIndex(64)
	query := make([]float32, 64)
	for i := range query {
		query[i] = 1.0
	}

	results := idx.Search(query, 10)
	if len(results) != 0 {
		t.Errorf("expected empty results on empty index, got %d", len(results))
	}
}

// Verify hnswCosineSimilarity helper correctness.
func TestCosineSimilarity(t *testing.T) {
	t.Parallel()

	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := hnswCosineSimilarity(a, b)
	if math.Abs(float64(sim)-1.0) > 1e-6 {
		t.Errorf("identical vectors: expected 1.0, got %f", sim)
	}

	c := []float32{-1, 0, 0}
	sim2 := hnswCosineSimilarity(a, c)
	if math.Abs(float64(sim2)+1.0) > 1e-6 {
		t.Errorf("opposite vectors: expected -1.0, got %f", sim2)
	}

	d := []float32{0, 1, 0}
	sim3 := hnswCosineSimilarity(a, d)
	if math.Abs(float64(sim3)) > 1e-6 {
		t.Errorf("orthogonal vectors: expected 0.0, got %f", sim3)
	}
}
