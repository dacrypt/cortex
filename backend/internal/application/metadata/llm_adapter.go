package metadata

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// LLMServiceAdapter adapts the LLM router to the SuggestionService interface.
type LLMServiceAdapter struct {
	llmRouter *llm.Router
}

// NewLLMServiceAdapter creates a new LLM service adapter.
func NewLLMServiceAdapter(llmRouter *llm.Router) *LLMServiceAdapter {
	return &LLMServiceAdapter{
		llmRouter: llmRouter,
	}
}

// Generate generates text using the LLM router.
func (a *LLMServiceAdapter) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if a.llmRouter == nil || !a.llmRouter.IsAvailable(ctx) {
		return "", nil
	}

	req := llm.GenerateRequest{
		Prompt:    prompt,
		MaxTokens: maxTokens,
	}

	response, err := a.llmRouter.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	if response == nil {
		return "", nil
	}

	return response.Text, nil
}

