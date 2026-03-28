// Package providers contains LLM provider implementations.
package providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude models.
type AnthropicProvider struct {
	id     string
	name   string
	apiKey string
	client anthropic.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(id, name, apiKey string) *AnthropicProvider {
	if name == "" {
		name = "Anthropic"
	}

	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	return &AnthropicProvider{
		id:     id,
		name:   name,
		apiKey: apiKey,
		client: anthropic.NewClient(opts...),
	}
}

// ID returns the provider ID.
func (p *AnthropicProvider) ID() string {
	return p.id
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return p.name
}

// Type returns the provider type.
func (p *AnthropicProvider) Type() string {
	return "anthropic"
}

// IsAvailable checks if Anthropic API is reachable.
func (p *AnthropicProvider) IsAvailable(ctx context.Context) (bool, error) {
	if p.apiKey == "" {
		return false, fmt.Errorf("no API key configured")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Make a minimal request to verify connectivity
	_, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude_3_Haiku_20240307,
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
		},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListModels returns available Claude models.
// Anthropic API does not have a models listing endpoint, so we return known models.
func (p *AnthropicProvider) ListModels(_ context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{
		{Name: "claude-sonnet-4-6", ContextLength: 200000, Capabilities: []string{"text", "vision"}},
		{Name: "claude-opus-4-6", ContextLength: 200000, Capabilities: []string{"text", "vision"}},
		{Name: "claude-sonnet-4-5", ContextLength: 200000, Capabilities: []string{"text", "vision"}},
		{Name: "claude-haiku-4-5", ContextLength: 200000, Capabilities: []string{"text", "vision"}},
		{Name: "claude-3-haiku-20240307", ContextLength: 200000, Capabilities: []string{"text", "vision"}},
	}, nil
}

// Generate generates a completion using the Anthropic Messages API.
func (p *AnthropicProvider) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	started := time.Now()

	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	model := req.Model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	// Build content blocks (text + optional images for vision)
	contentBlocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock(req.Prompt),
	}
	for _, img := range req.Images {
		mediaType := detectImageMediaType(img)
		b64 := base64.StdEncoding.EncodeToString(img)
		contentBlocks = append(contentBlocks, anthropic.NewImageBlockBase64(mediaType, b64))
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(contentBlocks...),
		},
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.Opt(req.Temperature)
	}
	if req.TopP > 0 {
		params.TopP = anthropic.Opt(req.TopP)
	}

	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic generation failed: %w", err)
	}

	// Extract text from response content blocks
	var text strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}

	elapsed := time.Since(started).Milliseconds()
	tokensUsed := int(resp.Usage.InputTokens + resp.Usage.OutputTokens)

	return &llm.GenerateResponse{
		Text:             text.String(),
		TokensUsed:       tokensUsed,
		Provider:         p.id,
		Model:            string(resp.Model),
		ProcessingTimeMs: elapsed,
	}, nil
}

// StreamGenerate generates a streaming completion.
func (p *AnthropicProvider) StreamGenerate(ctx context.Context, req llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	model := req.Model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.Opt(req.Temperature)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	chunks := make(chan llm.GenerateChunk, 100)

	go func() {
		defer close(chunks)
		defer stream.Close()

		for stream.Next() {
			event := stream.Current()
			if event.Type == "content_block_delta" {
				delta := event.Delta
				if delta.Type == "text_delta" {
					chunks <- llm.GenerateChunk{
						Text: delta.Text,
						Done: false,
					}
				}
			} else if event.Type == "message_stop" {
				chunks <- llm.GenerateChunk{
					Text: "",
					Done: true,
				}
			}
		}

		if err := stream.Err(); err != nil {
			errStr := err.Error()
			chunks <- llm.GenerateChunk{Error: &errStr}
		}
	}()

	return chunks, nil
}

// detectImageMediaType returns the MIME type for image bytes based on magic bytes.
func detectImageMediaType(data []byte) string {
	ct := http.DetectContentType(data)
	if strings.HasPrefix(ct, "image/") {
		return ct
	}
	return "image/jpeg" // safe default
}

// Ensure AnthropicProvider implements Provider
var _ llm.Provider = (*AnthropicProvider)(nil)
