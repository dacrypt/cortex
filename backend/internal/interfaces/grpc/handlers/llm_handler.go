// Package handlers provides gRPC service implementations.
package handlers

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// LLMHandler handles LLM-related gRPC requests.
type LLMHandler struct {
	router *llm.Router
	logger zerolog.Logger
}

// NewLLMHandler creates a new LLM handler.
func NewLLMHandler(router *llm.Router, logger zerolog.Logger) *LLMHandler {
	return &LLMHandler{
		router: router,
		logger: logger.With().Str("handler", "llm").Logger(),
	}
}

// ListProviders lists all registered providers.
func (h *LLMHandler) ListProviders() []llm.ProviderInfo {
	return h.router.ListProviders()
}

// GetProviderStatus returns the status of a provider.
func (h *LLMHandler) GetProviderStatus(ctx context.Context, providerID string) (*llm.ProviderStatus, error) {
	return h.router.GetProviderStatus(ctx, providerID)
}

// SetActiveProvider sets the active provider and model.
func (h *LLMHandler) SetActiveProvider(providerID, model string) error {
	return h.router.SetActiveProvider(providerID, model)
}

// ListModels lists models for a provider.
func (h *LLMHandler) ListModels(ctx context.Context, providerID string) ([]llm.ModelInfo, error) {
	if providerID == "" {
		provider, _, err := h.router.GetActiveProvider()
		if err != nil {
			return nil, err
		}
		providerID = provider.ID()
	}
	status, err := h.router.GetProviderStatus(ctx, providerID)
	if err != nil {
		return nil, err
	}
	return status.Models, nil
}

// SuggestTags suggests tags for content.
func (h *LLMHandler) SuggestTags(ctx context.Context, content string, maxTags int) ([]string, error) {
	if maxTags <= 0 {
		maxTags = 5
	}

	h.logger.Debug().
		Int("content_chars", len(content)).
		Int("max_tags", maxTags).
		Msg("Suggesting tags")

	tags, err := h.router.SuggestTags(ctx, content, maxTags)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Int("max_tags", maxTags).
			Msg("Failed to suggest tags")
		return nil, err
	}

	h.logger.Debug().
		Int("tags_count", len(tags)).
		Msg("Tags suggested")

	return tags, nil
}

// SuggestProject suggests a project for content.
func (h *LLMHandler) SuggestProject(ctx context.Context, content string, existingProjects []string) (string, error) {
	h.logger.Debug().
		Int("content_chars", len(content)).
		Int("existing_projects", len(existingProjects)).
		Msg("Suggesting project")

	project, err := h.router.SuggestProject(ctx, content, existingProjects)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Msg("Failed to suggest project")
		return "", err
	}

	h.logger.Debug().
		Str("project", project).
		Msg("Project suggested")

	return project, nil
}

// GenerateSummary generates a summary for content.
func (h *LLMHandler) GenerateSummary(ctx context.Context, content string, maxLength int) (string, error) {
	if maxLength <= 0 {
		maxLength = 200
	}

	h.logger.Debug().
		Int("content_chars", len(content)).
		Int("max_length", maxLength).
		Msg("Generating summary")

	summary, err := h.router.GenerateSummary(ctx, content, maxLength)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Int("max_length", maxLength).
			Msg("Failed to generate summary")
		return "", err
	}

	h.logger.Debug().
		Int("summary_length", len(summary)).
		Msg("Summary generated")

	return summary, nil
}

// ClassifyCategory classifies content into a category.
func (h *LLMHandler) ClassifyCategory(ctx context.Context, content string, categories []string) (string, error) {
	h.logger.Debug().
		Int("content_chars", len(content)).
		Int("categories", len(categories)).
		Msg("Classifying category")

	category, err := h.router.ClassifyCategory(ctx, content, categories)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Msg("Failed to classify category")
		return "", err
	}

	h.logger.Debug().
		Str("category", category).
		Msg("Category classified")

	return category, nil
}

// FindRelatedFiles returns related file paths.
func (h *LLMHandler) FindRelatedFiles(ctx context.Context, content string, candidates []string, maxResults int) ([]string, error) {
	h.logger.Debug().
		Int("content_chars", len(content)).
		Int("candidates", len(candidates)).
		Msg("Finding related files")

	related, err := h.router.FindRelatedFiles(ctx, content, candidates, maxResults)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Msg("Failed to find related files")
		return nil, err
	}

	h.logger.Debug().
		Int("related", len(related)).
		Msg("Related files found")

	return related, nil
}

// GenerateCompletion generates a raw completion.
func (h *LLMHandler) GenerateCompletion(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	h.logger.Debug().
		Str("model", req.Model).
		Int("prompt_chars", len(req.Prompt)).
		Int("max_tokens", req.MaxTokens).
		Float64("temperature", req.Temperature).
		Int("timeout_ms", req.TimeoutMs).
		Msg("Generating completion")

	resp, err := h.router.Generate(ctx, req)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Str("model", req.Model).
			Msg("Failed to generate completion")
		return nil, err
	}

	h.logger.Debug().
		Str("provider", resp.Provider).
		Str("model", resp.Model).
		Int("tokens", resp.TokensUsed).
		Int64("duration_ms", resp.ProcessingTimeMs).
		Msg("Completion generated")

	return resp, nil
}

// StreamCompletion streams a completion.
func (h *LLMHandler) StreamCompletion(ctx context.Context, req llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	return h.router.StreamGenerate(ctx, req)
}

// IsAvailable checks if LLM is available.
func (h *LLMHandler) IsAvailable(ctx context.Context) bool {
	return h.router.IsAvailable(ctx)
}
