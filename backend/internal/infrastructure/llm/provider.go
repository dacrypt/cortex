package llm

import (
	"context"
)

// Provider defines the interface for LLM providers.
type Provider interface {
	// ID returns the provider's unique identifier.
	ID() string

	// Name returns the provider's display name.
	Name() string

	// Type returns the provider type (ollama, openai, etc.).
	Type() string

	// IsAvailable checks if the provider is available.
	IsAvailable(ctx context.Context) (bool, error)

	// ListModels returns available models.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Generate generates a completion.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// StreamGenerate generates a streaming completion.
	StreamGenerate(ctx context.Context, req GenerateRequest) (<-chan GenerateChunk, error)
}

// ProviderInfo contains provider metadata.
type ProviderInfo struct {
	ID   string
	Name string
	Type string
}

// ProviderStatus contains provider health status.
type ProviderStatus struct {
	Info      ProviderInfo
	Available bool
	Models    []ModelInfo
	Error     *string
}

// ModelInfo contains model metadata.
type ModelInfo struct {
	Name          string
	Size          int64
	ContextLength int64
	Capabilities  []string
}

// GenerateRequest contains parameters for generation.
type GenerateRequest struct {
	Prompt      string
	Model       string
	MaxTokens   int
	Temperature float64
	TopP        float64
	Stop        []string
	TimeoutMs   int
	Images      [][]byte // Raw image bytes for vision models (nil = text-only)
}

// GenerateResponse contains the generation result.
type GenerateResponse struct {
	Text             string
	TokensUsed       int
	Provider         string
	Model            string
	ProcessingTimeMs int64
}

// GenerateChunk is a chunk from streaming generation.
type GenerateChunk struct {
	Text  string
	Done  bool
	Error *string
}
