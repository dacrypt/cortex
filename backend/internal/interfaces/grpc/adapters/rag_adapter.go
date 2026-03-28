package adapters

import (
	"context"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RAGServiceAdapter implements cortexv1.RAGServiceServer.
type RAGServiceAdapter struct {
	cortexv1.UnimplementedRAGServiceServer
	handler *handlers.RAGHandler
}

// NewRAGServiceAdapter creates a new RAG service adapter.
func NewRAGServiceAdapter(handler *handlers.RAGHandler) *RAGServiceAdapter {
	return &RAGServiceAdapter{handler: handler}
}

// Query performs a RAG query.
func (a *RAGServiceAdapter) Query(ctx context.Context, req *cortexv1.RAGQueryRequest) (*cortexv1.RAGQueryResponse, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	result, err := a.handler.Query(ctx, handlers.RAGQueryRequest{
		WorkspaceID:   req.WorkspaceId,
		Query:         req.Query,
		TopK:          int(req.TopK),
		MinSimilarity: req.MinSimilarity,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query failed: %v", err)
	}

	sources := make([]*cortexv1.RAGSource, len(result.Sources))
	for i, src := range result.Sources {
		sources[i] = &cortexv1.RAGSource{
			DocumentId:   sanitizeUTF8(src.DocumentID),
			ChunkId:      sanitizeUTF8(src.ChunkID),
			RelativePath: sanitizeUTF8(src.RelativePath),
			HeadingPath:  sanitizeUTF8(src.HeadingPath),
			Snippet:      sanitizeUTF8(src.Snippet),
			Score:        src.Score,
		}
	}

	return &cortexv1.RAGQueryResponse{
		Answer:  sanitizeUTF8(result.Answer),
		Sources: sources,
	}, nil
}

// SemanticSearch performs a semantic search.
func (a *RAGServiceAdapter) SemanticSearch(ctx context.Context, req *cortexv1.SemanticSearchRequest) (*cortexv1.SemanticSearchResponse, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	result, err := a.handler.SemanticSearch(ctx, handlers.SemanticSearchRequest{
		WorkspaceID:   req.WorkspaceId,
		Query:         req.Query,
		TopK:          int(req.TopK),
		MinSimilarity: req.MinSimilarity,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	results := make([]*cortexv1.RAGSource, len(result.Results))
	for i, src := range result.Results {
		results[i] = &cortexv1.RAGSource{
			DocumentId:   sanitizeUTF8(src.DocumentID),
			ChunkId:      sanitizeUTF8(src.ChunkID),
			RelativePath: sanitizeUTF8(src.RelativePath),
			HeadingPath:  sanitizeUTF8(src.HeadingPath),
			Snippet:      sanitizeUTF8(src.Snippet),
			Score:        src.Score,
		}
	}

	return &cortexv1.SemanticSearchResponse{
		Results: results,
	}, nil
}

// GetIndexStats returns index statistics.
func (a *RAGServiceAdapter) GetIndexStats(ctx context.Context, req *cortexv1.GetIndexStatsRequest) (*cortexv1.GetIndexStatsResponse, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	result, err := a.handler.GetIndexStats(ctx, handlers.IndexStatsRequest{
		WorkspaceID: req.WorkspaceId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stats: %v", err)
	}

	return &cortexv1.GetIndexStatsResponse{
		TotalDocuments:  result.TotalDocuments,
		TotalChunks:     result.TotalChunks,
		TotalEmbeddings: result.TotalEmbeddings,
		EmbeddingModel:  sanitizeUTF8(result.EmbeddingModel),
	}, nil
}
