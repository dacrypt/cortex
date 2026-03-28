// Package llm provides LLM router and provider implementations.
// This file contains RAG chain implementation inspired by langchaingo patterns.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// RAGChain encapsulates the RAG workflow for generating answers from sources
// This chain handles: context building → LLM generation → answer cleaning
type RAGChain struct {
	llmRouter *Router
	logger    zerolog.Logger
}

// RAGSource represents a retrieved document chunk (compatible with rag.Source)
type RAGSource struct {
	DocumentID   string
	ChunkID      string
	RelativePath string
	HeadingPath  string
	Snippet      string
	Score        float32
}

// RAGResult contains the result of a RAG query
type RAGResult struct {
	Answer  string
	Sources []RAGSource
}

// NewRAGChain creates a new RAG chain
func NewRAGChain(llmRouter *Router, logger zerolog.Logger) *RAGChain {
	return &RAGChain{
		llmRouter: llmRouter,
		logger:    logger.With().Str("component", "rag_chain").Logger(),
	}
}

// ExecuteWithSources runs RAG workflow with pre-processed sources
// This encapsulates: context building → LLM generation → answer cleaning
func (c *RAGChain) ExecuteWithSources(ctx context.Context, query string, sources []RAGSource) (*RAGResult, error) {
	if len(sources) == 0 {
		return &RAGResult{
			Answer:  "No se encontró información relevante en los documentos.",
			Sources: sources,
		}, nil
	}

	c.logger.Debug().
		Str("query", query).
		Int("sources_count", len(sources)).
		Msg("RAG Chain: Executing with pre-processed sources")

	// Step 1: Build context from sources
	contextStr := c.buildContext(sources)

	// Step 2: Generate answer with LLM (if available)
	if c.llmRouter == nil || !c.llmRouter.IsAvailable(ctx) {
		c.logger.Warn().Msg("RAG Chain: LLM not available, using basic assembly")
		return &RAGResult{
			Answer:  c.buildBasicAnswer(query, sources),
			Sources: sources,
		}, nil
	}

	// Step 3: Generate answer using LLM
	prompt := c.llmRouter.GetRAGAnswerPrompt(contextStr, query)
	response, err := c.llmRouter.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   800,
		Temperature: 0.2,
	})
	if err != nil {
		c.logger.Warn().Err(err).Msg("RAG Chain: LLM generation failed, using basic assembly")
		return &RAGResult{
			Answer:  c.buildBasicAnswer(query, sources),
			Sources: sources,
		}, nil
	}

	// Step 4: Clean and return answer
	stringParser := NewStringParser(c.logger)
	cleanedAnswer := stringParser.ParseString(response.Text)

	c.logger.Info().
		Int("answer_length", len(cleanedAnswer)).
		Int("sources_count", len(sources)).
		Msg("RAG Chain: Execution completed successfully")

	return &RAGResult{
		Answer:  cleanedAnswer,
		Sources: sources,
	}, nil
}

// buildContext builds context string from sources
func (c *RAGChain) buildContext(sources []RAGSource) string {
	var contextBuilder strings.Builder
	for i, source := range sources {
		contextBuilder.WriteString(fmt.Sprintf("[%d] Source: %s\nContent: %s\n\n", i+1, source.RelativePath, source.Snippet))
	}
	return contextBuilder.String()
}

// buildBasicAnswer builds a basic answer without LLM
func (c *RAGChain) buildBasicAnswer(query string, sources []RAGSource) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Query: %s", strings.TrimSpace(query)))
	parts = append(parts, "Context:")
	for i, source := range sources {
		citation := fmt.Sprintf("[%d] %s — %s", i+1, source.RelativePath, source.HeadingPath)
		parts = append(parts, citation)
		parts = append(parts, source.Snippet)
	}
	return strings.Join(parts, "\n\n")
}
