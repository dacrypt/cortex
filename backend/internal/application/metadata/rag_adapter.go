package metadata

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/application/rag"
)

// RAGServiceAdapter adapts the RAG service to the SuggestionService interface.
type RAGServiceAdapter struct {
	ragService *rag.Service
}

// NewRAGServiceAdapter creates a new RAG service adapter.
func NewRAGServiceAdapter(ragService *rag.Service) *RAGServiceAdapter {
	return &RAGServiceAdapter{
		ragService: ragService,
	}
}

// Query performs a RAG query and adapts the response.
func (a *RAGServiceAdapter) Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error) {
	if a.ragService == nil {
		return nil, nil
	}

	result, err := a.ragService.Query(ctx, rag.QueryRequest{
		WorkspaceID:    req.WorkspaceID,
		Query:          req.Query,
		TopK:           req.TopK,
		GenerateAnswer: req.GenerateAnswer,
	})
	if err != nil {
		return nil, err
	}

	sources := make([]RAGSource, len(result.Sources))
	for i, src := range result.Sources {
		sources[i] = RAGSource{
			DocumentID:   src.DocumentID,
			ChunkID:      src.ChunkID,
			RelativePath: src.RelativePath,
			HeadingPath:  src.HeadingPath,
			Snippet:      src.Snippet,
			Score:        src.Score,
		}
	}

	return &RAGQueryResponse{
		Answer:  result.Answer,
		Sources: sources,
	}, nil
}

