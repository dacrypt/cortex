package embedding

import "context"

// Embedder provides text-to-vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
