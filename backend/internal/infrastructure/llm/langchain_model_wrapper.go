package llm

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

// LangchainModelWrapper wraps our Router to implement langchaingo's llms.Model interface.
// This allows us to use langchaingo chains with our existing LLM infrastructure.
type LangchainModelWrapper struct {
	router *Router
}

// NewLangchainModelWrapper creates a new wrapper for our Router.
func NewLangchainModelWrapper(router *Router) *LangchainModelWrapper {
	return &LangchainModelWrapper{
		router: router,
	}
}

// Call implements llms.Model interface for simple text generation.
func (w *LangchainModelWrapper) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	// Convert langchaingo options to our GenerateRequest
	maxTokens := 800
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}

	temperature := 0.7
	if opts.Temperature > 0 {
		temperature = opts.Temperature
	}

	req := GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	resp, err := w.router.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

// GenerateContent implements llms.Model interface for structured content generation.
func (w *LangchainModelWrapper) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// Convert messages to prompt
	prompt := ""
	for _, msg := range messages {
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case llms.TextContent:
				if prompt != "" {
					prompt += "\n\n"
				}
				prompt += p.Text
			}
		}
	}

	// Use Call for simple text generation
	text, err := w.Call(ctx, prompt, options...)
	if err != nil {
		return nil, err
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: text,
			},
		},
	}, nil
}

// GetNumTokens is a simple token counter (approximate).
func (w *LangchainModelWrapper) GetNumTokens(text string) int {
	// Simple approximation: ~4 characters per token
	return len(text) / 4
}

// Static assertions to ensure we implement the interface
var _ llms.Model = (*LangchainModelWrapper)(nil)

