package sqlite

import (
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

const (
	hnswM              = 16  // max connections per node per layer
	hnswMmax0          = 32  // max connections at layer 0 (2*M)
	hnswEfConstruction = 200 // beam width during construction
	hnswEfSearch       = 50  // beam width during search
)

// hnswNode represents a single vector in the HNSW graph.
type hnswNode struct {
	id      string
	vector  []float32
	layer   int
	friends [][]string // friends[level] = list of neighbor IDs
}

// HNSWIndex is an in-memory Hierarchical Navigable Small World graph
// for approximate nearest neighbor search using cosine similarity.
type HNSWIndex struct {
	mu         sync.RWMutex
	dim        int
	nodes      map[string]*hnswNode
	entryPoint string
	maxLayer   int
	rng        *rand.Rand
}

// NewHNSWIndex creates a new HNSW index for vectors of the given dimensionality.
func NewHNSWIndex(dim int) *HNSWIndex {
	return &HNSWIndex{
		dim:      dim,
		nodes:    make(map[string]*hnswNode),
		maxLayer: -1,
		rng:      rand.New(rand.NewSource(42)),
	}
}

// Size returns the number of vectors in the index.
func (idx *HNSWIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.nodes)
}

// Insert adds a vector with the given ID to the index.
func (idx *HNSWIndex) Insert(id string, vector []float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.insertInternal(id, vector)
}

// insertInternal performs insertion without acquiring the lock (caller must hold it).
func (idx *HNSWIndex) insertInternal(id string, vector []float32) {
	if _, exists := idx.nodes[id]; exists {
		// Update vector in place.
		idx.nodes[id].vector = vector
		return
	}

	nodeLayer := idx.randomLevel()
	node := &hnswNode{
		id:      id,
		vector:  vector,
		layer:   nodeLayer,
		friends: make([][]string, nodeLayer+1),
	}
	for i := range node.friends {
		node.friends[i] = make([]string, 0)
	}
	idx.nodes[id] = node

	if len(idx.nodes) == 1 {
		idx.entryPoint = id
		idx.maxLayer = nodeLayer
		return
	}

	ep := idx.entryPoint

	// Phase 1: Greedily traverse from top layer down to nodeLayer+1.
	for level := idx.maxLayer; level > nodeLayer; level-- {
		ep = idx.greedyClosest(vector, ep, level)
	}

	// Phase 2: At each layer from min(nodeLayer, maxLayer) down to 0,
	// find efConstruction nearest neighbors and connect.
	for level := min(nodeLayer, idx.maxLayer); level >= 0; level-- {
		candidates := idx.searchLayer(vector, ep, hnswEfConstruction, level)

		mMax := hnswM
		if level == 0 {
			mMax = hnswMmax0
		}

		// Select the top mMax neighbors.
		neighbors := idx.selectNeighbors(candidates, mMax)
		node.friends[level] = neighbors

		// Add bidirectional connections.
		for _, neighborID := range neighbors {
			neighbor := idx.nodes[neighborID]
			if level < len(neighbor.friends) {
				neighbor.friends[level] = append(neighbor.friends[level], id)
				// Prune if over capacity.
				if len(neighbor.friends[level]) > mMax {
					neighbor.friends[level] = idx.selectNeighbors(
						idx.rankByDistance(vector, neighbor.friends[level]),
						mMax,
					)
				}
			}
		}

		if len(candidates) > 0 {
			ep = candidates[0].id
		}
	}

	if nodeLayer > idx.maxLayer {
		idx.entryPoint = id
		idx.maxLayer = nodeLayer
	}
}

// Delete removes a vector from the index. Neighbors' connections pointing
// to the deleted node are cleaned up.
func (idx *HNSWIndex) Delete(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, exists := idx.nodes[id]
	if !exists {
		return
	}

	// Remove references from all neighbors.
	for level := 0; level <= node.layer; level++ {
		if level >= len(node.friends) {
			break
		}
		for _, neighborID := range node.friends[level] {
			neighbor := idx.nodes[neighborID]
			if neighbor == nil || level >= len(neighbor.friends) {
				continue
			}
			filtered := make([]string, 0, len(neighbor.friends[level]))
			for _, fid := range neighbor.friends[level] {
				if fid != id {
					filtered = append(filtered, fid)
				}
			}
			neighbor.friends[level] = filtered
		}
	}

	delete(idx.nodes, id)

	// If the deleted node was the entry point, pick a new one.
	if idx.entryPoint == id {
		if len(idx.nodes) == 0 {
			idx.entryPoint = ""
			idx.maxLayer = -1
		} else {
			// Find the node with the highest layer.
			bestID := ""
			bestLayer := -1
			for nid, n := range idx.nodes {
				if n.layer > bestLayer {
					bestLayer = n.layer
					bestID = nid
				}
			}
			idx.entryPoint = bestID
			idx.maxLayer = bestLayer
		}
	}
}

// Search returns the topK most similar vectors to the query using cosine similarity.
func (idx *HNSWIndex) Search(query []float32, topK int) []repository.VectorMatch {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.nodes) == 0 || topK <= 0 {
		return nil
	}

	ep := idx.entryPoint

	// Greedy descent from top layer to layer 1.
	for level := idx.maxLayer; level > 0; level-- {
		ep = idx.greedyClosest(query, ep, level)
	}

	// Beam search at layer 0.
	candidates := idx.searchLayer(query, ep, max(hnswEfSearch, topK), 0)

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	results := make([]repository.VectorMatch, len(candidates))
	for i, c := range candidates {
		results[i] = repository.VectorMatch{
			ChunkID:    entity.ChunkID(c.id),
			Similarity: c.dist,
		}
	}
	return results
}

// Load performs a bulk load of vectors, replacing any existing index contents.
func (idx *HNSWIndex) Load(vectors map[string][]float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Reset the index.
	idx.nodes = make(map[string]*hnswNode, len(vectors))
	idx.entryPoint = ""
	idx.maxLayer = -1

	for id, vec := range vectors {
		idx.insertInternal(id, vec)
	}
}

// candidate is used during search to track nodes and their similarity scores.
type candidate struct {
	id   string
	dist float32 // cosine similarity (higher = closer)
}

// searchLayer performs a beam search at the given layer, starting from ep,
// returning up to ef candidates sorted by descending similarity.
func (idx *HNSWIndex) searchLayer(query []float32, ep string, ef int, level int) []candidate {
	epNode := idx.nodes[ep]
	if epNode == nil {
		return nil
	}

	epDist := hnswCosineSimilarity(query, epNode.vector)
	visited := map[string]bool{ep: true}
	candidateList := []candidate{{id: ep, dist: epDist}}
	results := []candidate{{id: ep, dist: epDist}}

	for len(candidateList) > 0 {
		// Pop the best (highest similarity) candidate.
		sort.Slice(candidateList, func(i, j int) bool {
			return candidateList[i].dist > candidateList[j].dist
		})
		current := candidateList[0]
		candidateList = candidateList[1:]

		// Worst in results.
		worst := results[len(results)-1].dist
		if len(results) >= ef && current.dist < worst {
			break
		}

		node := idx.nodes[current.id]
		if node == nil || level >= len(node.friends) {
			continue
		}

		for _, neighborID := range node.friends[level] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true

			neighbor := idx.nodes[neighborID]
			if neighbor == nil {
				continue
			}

			sim := hnswCosineSimilarity(query, neighbor.vector)
			worstResult := results[len(results)-1].dist

			if len(results) < ef || sim > worstResult {
				candidateList = append(candidateList, candidate{id: neighborID, dist: sim})
				results = append(results, candidate{id: neighborID, dist: sim})
				sort.Slice(results, func(i, j int) bool {
					return results[i].dist > results[j].dist
				})
				if len(results) > ef {
					results = results[:ef]
				}
			}
		}
	}

	return results
}

// greedyClosest walks from ep greedily at the given layer, returning the
// node ID closest (highest cosine similarity) to the query.
func (idx *HNSWIndex) greedyClosest(query []float32, ep string, level int) string {
	current := ep
	currentDist := hnswCosineSimilarity(query, idx.nodes[ep].vector)

	for {
		node := idx.nodes[current]
		if node == nil || level >= len(node.friends) {
			break
		}
		improved := false
		for _, neighborID := range node.friends[level] {
			neighbor := idx.nodes[neighborID]
			if neighbor == nil {
				continue
			}
			sim := hnswCosineSimilarity(query, neighbor.vector)
			if sim > currentDist {
				current = neighborID
				currentDist = sim
				improved = true
			}
		}
		if !improved {
			break
		}
	}
	return current
}

// selectNeighbors picks up to mMax neighbors from candidates (already sorted
// by descending similarity), returning their IDs.
func (idx *HNSWIndex) selectNeighbors(candidates []candidate, mMax int) []string {
	n := mMax
	if len(candidates) < n {
		n = len(candidates)
	}
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = candidates[i].id
	}
	return ids
}

// rankByDistance returns candidates ranked by cosine similarity to ref vector
// (descending). This is used when pruning neighbor lists.
func (idx *HNSWIndex) rankByDistance(ref []float32, ids []string) []candidate {
	candidates := make([]candidate, 0, len(ids))
	for _, id := range ids {
		n := idx.nodes[id]
		if n == nil {
			continue
		}
		candidates = append(candidates, candidate{id: id, dist: hnswCosineSimilarity(ref, n.vector)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist > candidates[j].dist
	})
	return candidates
}

// randomLevel returns a random layer for a new node. The probability of
// being assigned to layer l is (1/ln(M))^l, giving an exponential distribution.
func (idx *HNSWIndex) randomLevel() int {
	ml := 1.0 / math.Log(float64(hnswM))
	r := idx.rng.Float64()
	return int(math.Floor(-math.Log(r) * ml))
}

// hnswCosineSimilarity computes the cosine similarity between two vectors.
// Returns a value in [-1, 1] where 1 means identical direction.
// This is a local copy to avoid coupling with vector_store.go's cosineSimilarity.
func hnswCosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}
