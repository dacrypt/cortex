package embedding

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
)

// HashEmbedder is a deterministic local embedder for offline use.
type HashEmbedder struct {
	dimensions int
}

// NewHashEmbedder creates a new hash-based embedder.
func NewHashEmbedder(dimensions int) *HashEmbedder {
	if dimensions <= 0 {
		dimensions = 384
	}
	return &HashEmbedder{dimensions: dimensions}
}

// Embed returns a normalized vector based on token hashes.
func (e *HashEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	tokens := tokenize(text)
	vector := make([]float32, e.dimensions)

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		idx := int(h.Sum32()) % e.dimensions
		vector[idx] += 1
	}

	normalize(vector)
	return vector, nil
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_'
	})
	return fields
}

func normalize(vector []float32) {
	var sum float32
	for _, v := range vector {
		sum += v * v
	}
	if sum == 0 {
		return
	}
	norm := float32(1.0 / math.Sqrt(float64(sum)))
	for i := range vector {
		vector[i] *= norm
	}
}
