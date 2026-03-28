// Package providers contains LLM provider implementations.
package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs (OpenAI, LM Studio, etc.).
type OpenAIProvider struct {
	id       string
	name     string
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
func NewOpenAIProvider(id, name, endpoint, apiKey string) *OpenAIProvider {
	if name == "" {
		name = "OpenAI"
	}
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		id:       id,
		name:     name,
		endpoint: endpoint,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ID returns the provider ID.
func (p *OpenAIProvider) ID() string {
	return p.id
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return p.name
}

// Type returns the provider type.
func (p *OpenAIProvider) Type() string {
	return "openai"
}

// IsAvailable checks if the OpenAI-compatible API is available.
func (p *OpenAIProvider) IsAvailable(ctx context.Context) (bool, error) {
	// Try to list models as a health check
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/models", nil)
	if err != nil {
		return false, err
	}

	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// ListModels returns available models.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/models", nil)
	if err != nil {
		return nil, err
	}

	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list models: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]llm.ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, llm.ModelInfo{
			Name: m.ID,
		})
	}

	return models, nil
}

// Generate generates a completion using OpenAI-compatible API.
func (p *OpenAIProvider) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required for OpenAI provider")
	}

	// Build request body
	requestBody := map[string]interface{}{
		"model":       req.Model,
		"prompt":      req.Prompt,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		requestBody["stop"] = req.Stop
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Set timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
	}

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("completion request failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Text         string `json:"text"`
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &llm.GenerateResponse{
		Text:             result.Choices[0].Text,
		TokensUsed:       result.Usage.TotalTokens,
		Provider:         p.id,
		Model:            result.Model,
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

// StreamGenerate generates a streaming completion.
func (p *OpenAIProvider) StreamGenerate(ctx context.Context, req llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	// OpenAI-compatible streaming implementation
	if req.Model == "" {
		return nil, fmt.Errorf("model is required for OpenAI provider")
	}

	// Build request body with stream=true
	requestBody := map[string]interface{}{
		"model":       req.Model,
		"prompt":      req.Prompt,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"stream":      true,
	}

	if req.TopP > 0 {
		requestBody["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		requestBody["stop"] = req.Stop
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Set timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("streaming request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("streaming request failed: status %d", resp.StatusCode)
	}

	chunks := make(chan llm.GenerateChunk, 100)

	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// OpenAI streaming format: "data: {...}\n\n"
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}

			data := bytes.TrimPrefix(line, []byte("data: "))
			if string(data) == "[DONE]" {
				chunks <- llm.GenerateChunk{Done: true}
				return
			}

			var result struct {
				Choices []struct {
					Text         string `json:"text"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal(data, &result); err != nil {
				errStr := err.Error()
				chunks <- llm.GenerateChunk{Error: &errStr}
				return
			}

			if len(result.Choices) > 0 {
				done := result.Choices[0].FinishReason != ""
				chunks <- llm.GenerateChunk{
					Text: result.Choices[0].Text,
					Done: done,
				}
				if done {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errStr := err.Error()
			chunks <- llm.GenerateChunk{Error: &errStr}
		}
	}()

	return chunks, nil
}

// Ensure OpenAIProvider implements Provider
var _ llm.Provider = (*OpenAIProvider)(nil)

