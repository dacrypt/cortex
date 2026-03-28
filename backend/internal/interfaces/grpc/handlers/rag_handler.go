package handlers

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/rag"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// RAGHandler handles RAG-related gRPC requests.
type RAGHandler struct {
	service *rag.Service
	logger  zerolog.Logger
}

// NewRAGHandler creates a new RAG handler.
func NewRAGHandler(service *rag.Service, logger zerolog.Logger) *RAGHandler {
	return &RAGHandler{
		service: service,
		logger:  logger.With().Str("handler", "rag").Logger(),
	}
}

// RAGQueryRequest is the handler request for RAG queries.
type RAGQueryRequest struct {
	WorkspaceID   string
	Query         string
	TopK          int
	MinSimilarity float32
}

// RAGQueryResponse is the handler response for RAG queries.
type RAGQueryResponse struct {
	Answer  string
	Sources []RAGSource
}

// RAGSource represents a source in the response.
type RAGSource struct {
	DocumentID   string
	ChunkID      string
	RelativePath string
	HeadingPath  string
	Snippet      string
	Score        float32
}

// Query performs a RAG query.
func (h *RAGHandler) Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error) {
	h.logger.Debug().
		Str("workspace_id", req.WorkspaceID).
		Str("query", req.Query).
		Int("top_k", req.TopK).
		Msg("Processing RAG query")

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	result, err := h.service.Query(ctx, rag.QueryRequest{
		WorkspaceID:    entity.WorkspaceID(req.WorkspaceID),
		Query:          req.Query,
		TopK:           topK,
		GenerateAnswer: true,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("RAG query failed")
		return nil, err
	}

	sources := make([]RAGSource, len(result.Sources))
	for i, src := range result.Sources {
		sources[i] = RAGSource{
			DocumentID:   string(src.DocumentID),
			ChunkID:      string(src.ChunkID),
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

// SemanticSearchRequest is the handler request for semantic search.
type SemanticSearchRequest struct {
	WorkspaceID   string
	Query         string
	TopK          int
	MinSimilarity float32
}

// SemanticSearchResponse is the handler response for semantic search.
type SemanticSearchResponse struct {
	Results []RAGSource
}

// SemanticSearch performs a semantic search without generating an answer.
func (h *RAGHandler) SemanticSearch(ctx context.Context, req SemanticSearchRequest) (*SemanticSearchResponse, error) {
	h.logger.Debug().
		Str("workspace_id", req.WorkspaceID).
		Str("query", req.Query).
		Int("top_k", req.TopK).
		Msg("Processing semantic search")

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	result, err := h.service.Query(ctx, rag.QueryRequest{
		WorkspaceID:    entity.WorkspaceID(req.WorkspaceID),
		Query:          req.Query,
		TopK:           topK,
		GenerateAnswer: false,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("Semantic search failed")
		return nil, err
	}

	results := make([]RAGSource, len(result.Sources))
	for i, src := range result.Sources {
		results[i] = RAGSource{
			DocumentID:   string(src.DocumentID),
			ChunkID:      string(src.ChunkID),
			RelativePath: src.RelativePath,
			HeadingPath:  src.HeadingPath,
			Snippet:      src.Snippet,
			Score:        src.Score,
		}
	}

	return &SemanticSearchResponse{
		Results: results,
	}, nil
}

// IndexStatsRequest is the handler request for index stats.
type IndexStatsRequest struct {
	WorkspaceID string
}

// IndexStatsResponse is the handler response for index stats.
type IndexStatsResponse struct {
	TotalDocuments  int64
	TotalChunks     int64
	TotalEmbeddings int64
	EmbeddingModel  string
}

// GetIndexStats returns statistics about the RAG index.
func (h *RAGHandler) GetIndexStats(ctx context.Context, req IndexStatsRequest) (*IndexStatsResponse, error) {
	h.logger.Debug().
		Str("workspace_id", req.WorkspaceID).
		Msg("Getting index stats")

	// TODO: Implement actual stats retrieval from repositories
	return &IndexStatsResponse{
		TotalDocuments:  0,
		TotalChunks:     0,
		TotalEmbeddings: 0,
		EmbeddingModel:  "nomic-embed-text",
	}, nil
}
