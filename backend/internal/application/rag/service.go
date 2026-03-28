package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/chains"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// Service provides retrieval-augmented queries over documents.
type Service struct {
	docRepo     repository.DocumentRepository
	vectorStore repository.VectorStore
	embedder    embedding.Embedder
	llmRouter   *llm.Router
	logger      zerolog.Logger
}

// NewService creates a new RAG service.
func NewService(
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	llmRouter *llm.Router,
	logger zerolog.Logger,
) *Service {
	return &Service{
		docRepo:     docRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		llmRouter:   llmRouter,
		logger:      logger.With().Str("component", "rag").Logger(),
	}
}

// QueryRequest represents a RAG query.
type QueryRequest struct {
	WorkspaceID    entity.WorkspaceID
	Query          string
	TopK           int
	GenerateAnswer bool
}

// Source represents a retrieved chunk for a query.
type Source struct {
	DocumentID   entity.DocumentID
	ChunkID      entity.ChunkID
	RelativePath string
	HeadingPath  string
	Snippet      string
	Score        float32
}

// toRAGSource converts rag.Source to llm.RAGSource
func (s Source) toRAGSource() llm.RAGSource {
	return llm.RAGSource{
		DocumentID:   s.DocumentID.String(),
		ChunkID:      s.ChunkID.String(),
		RelativePath: s.RelativePath,
		HeadingPath:  s.HeadingPath,
		Snippet:      s.Snippet,
		Score:        s.Score,
	}
}

// QueryResponse contains an answer and sources.
type QueryResponse struct {
	Answer  string
	Sources []Source
}

// Query performs a semantic search and returns context with citations.
func (s *Service) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	if s.docRepo == nil || s.vectorStore == nil || s.embedder == nil {
		return nil, fmt.Errorf("rag service is not configured")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	s.logger.Info().
		Str("workspace_id", req.WorkspaceID.String()).
		Str("query", req.Query).
		Int("top_k", req.TopK).
		Msg("RAG: Creating embedding for query")

	vector, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		s.logger.Error().Err(err).Str("query", req.Query).Msg("RAG: Failed to create embedding")
		return nil, err
	}

	s.logger.Info().
		Str("workspace_id", req.WorkspaceID.String()).
		Int("vector_dimensions", len(vector)).
		Msg("RAG: Searching vector store")

	matches, err := s.vectorStore.Search(ctx, req.WorkspaceID, vector, req.TopK)
	if err != nil {
		s.logger.Error().Err(err).Str("query", req.Query).Msg("RAG: Vector search failed")
		return nil, err
	}

	s.logger.Info().
		Str("workspace_id", req.WorkspaceID.String()).
		Int("matches_found", len(matches)).
		Msg("RAG: Vector search completed")

	chunkIDs := make([]entity.ChunkID, 0, len(matches))
	for _, match := range matches {
		chunkIDs = append(chunkIDs, match.ChunkID)
	}

	chunks, err := s.docRepo.GetChunksByIDs(ctx, req.WorkspaceID, chunkIDs)
	if err != nil {
		return nil, err
	}

	chunkByID := make(map[entity.ChunkID]*entity.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkByID[chunk.ID] = chunk
	}

	docCache := make(map[entity.DocumentID]*entity.Document)
	sources := make([]Source, 0, len(matches))
	for _, match := range matches {
		chunk := chunkByID[match.ChunkID]
		if chunk == nil {
			continue
		}
		doc := docCache[chunk.DocumentID]
		if doc == nil {
			doc, _ = s.docRepo.GetDocument(ctx, req.WorkspaceID, chunk.DocumentID)
			if doc != nil {
				docCache[chunk.DocumentID] = doc
			}
		}

		relativePath := ""
		if doc != nil {
			relativePath = doc.RelativePath
		}

		sources = append(sources, Source{
			DocumentID:   chunk.DocumentID,
			ChunkID:      chunk.ID,
			RelativePath: relativePath,
			HeadingPath:  chunk.HeadingPath,
			Snippet:      summarizeSnippet(chunk.Text, 400),
			Score:        match.Similarity,
		})
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Score > sources[j].Score
	})

	var answer string
	if req.GenerateAnswer && s.llmRouter != nil {
		// Try using langchaingo's RetrievalQA chain first
		answer, err = s.generateAnswerWithLangchain(ctx, req.Query, req.WorkspaceID, req.TopK)
		if err != nil {
			s.logger.Debug().Err(err).Msg("RAG: langchaingo chain failed, trying our RAG Chain")
			// Fallback to our RAG Chain
			ragChain := llm.NewRAGChain(s.llmRouter, s.logger)
			ragSources := make([]llm.RAGSource, 0, len(sources))
			for _, source := range sources {
				ragSources = append(ragSources, source.toRAGSource())
			}
			result, err := ragChain.ExecuteWithSources(ctx, req.Query, ragSources)
			if err != nil {
				s.logger.Warn().Err(err).Msg("RAG: RAG Chain failed, falling back to basic assembly")
				answer = buildAnswer(req.Query, sources)
			} else {
				s.logger.Info().
					Str("workspace_id", req.WorkspaceID.String()).
					Int("answer_length", len(result.Answer)).
					Msg("RAG: RAG Chain generated answer successfully")
				answer = result.Answer
			}
		} else {
			s.logger.Info().
				Str("workspace_id", req.WorkspaceID.String()).
				Int("answer_length", len(answer)).
				Msg("RAG: langchaingo RetrievalQA chain generated answer successfully")
		}
	} else {
		s.logger.Info().
			Str("workspace_id", req.WorkspaceID.String()).
			Msg("RAG: Generating answer without LLM (basic assembly)")
		answer = buildAnswer(req.Query, sources)
	}

	s.logger.Info().
		Str("workspace_id", req.WorkspaceID.String()).
		Int("answer_length", len(answer)).
		Int("sources_count", len(sources)).
		Msg("RAG: Query completed successfully")

	return &QueryResponse{
		Answer:  answer,
		Sources: sources,
	}, nil
}

// generateAnswerWithLangchain uses langchaingo's RetrievalQA chain to generate answers.
// This provides a standard, well-tested abstraction for RAG workflows.
func (s *Service) generateAnswerWithLangchain(ctx context.Context, query string, workspaceID entity.WorkspaceID, topK int) (string, error) {
	// Create retriever
	retriever := NewCortexRetriever(
		s.docRepo,
		s.vectorStore,
		s.embedder,
		workspaceID,
		topK,
	)

	// Create LLM wrapper
	llmModel := llm.NewLangchainModelWrapper(s.llmRouter)

	// Create RetrievalQA chain
	qaChain := chains.NewRetrievalQAFromLLM(llmModel, retriever)

	// Execute chain
	result, err := chains.Call(ctx, qaChain, map[string]any{
		"query": query,
	})
	if err != nil {
		return "", err
	}

	// Extract answer from result
	// langchaingo chains typically return the answer in a "text" key
	if text, ok := result["text"].(string); ok {
		return text, nil
	}

	// Fallback: try to find answer in any string value
	for _, v := range result {
		if str, ok := v.(string); ok && str != "" {
			return str, nil
		}
	}

	return "", fmt.Errorf("no answer found in chain result")
}

func (s *Service) generateLLMAnswer(ctx context.Context, query string, sources []Source) (string, error) {
	if len(sources) == 0 {
		return "No se encontró información relevante en los documentos.", nil
	}

	var contextBuilder strings.Builder
	for i, src := range sources {
		contextBuilder.WriteString(fmt.Sprintf("[%d] Source: %s\nContent: %s\n\n", i+1, src.RelativePath, src.Snippet))
	}

	// Use prompt from router if available, otherwise use default
	prompt := s.llmRouter.GetRAGAnswerPrompt(contextBuilder.String(), query)

	resp, err := s.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   800,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text), nil
}

func summarizeSnippet(text string, maxLen int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen] + "..."
}

func buildAnswer(query string, sources []Source) string {
	if len(sources) == 0 {
		return "No matching context found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Query: %s", strings.TrimSpace(query)))
	parts = append(parts, "Context:")
	for i, src := range sources {
		citation := fmt.Sprintf("[%d] %s — %s", i+1, src.RelativePath, src.HeadingPath)
		parts = append(parts, citation)
		parts = append(parts, src.Snippet)
	}
	return strings.Join(parts, "\n\n")
}
