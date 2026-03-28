// Package providers contains LLM provider implementations.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	id       string
	name     string
	endpoint string
	client   *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(id, name, endpoint string) *OllamaProvider {
	if name == "" {
		name = "Ollama"
	}
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	return &OllamaProvider{
		id:       id,
		name:     name,
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ID returns the provider ID.
func (p *OllamaProvider) ID() string {
	return p.id
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return p.name
}

// Type returns the provider type.
func (p *OllamaProvider) Type() string {
	return "ollama"
}

// IsAvailable checks if Ollama is available.
func (p *OllamaProvider) IsAvailable(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/api/tags", nil)
	if err != nil {
		return false, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// ListModels returns available models.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []llm.ModelInfo
	for _, m := range result.Models {
		models = append(models, llm.ModelInfo{
			Name: m.Name,
			Size: m.Size,
		})
	}

	return models, nil
}

// Generate generates a completion.
func (p *OllamaProvider) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	started := time.Now()
	payload := map[string]interface{}{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		payload["options"].(map[string]interface{})["num_predict"] = req.MaxTokens
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Use custom timeout if specified
	client := p.client
	if req.TimeoutMs > 0 {
		client = &http.Client{
			Timeout: time.Duration(req.TimeoutMs) * time.Millisecond,
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("generation failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Response           string `json:"response"`
		PromptEvalCount    int    `json:"prompt_eval_count"`
		EvalCount          int    `json:"eval_count"`
		TotalDuration      int64  `json:"total_duration"`
		LoadDuration       int64  `json:"load_duration"`
		PromptEvalDuration int64  `json:"prompt_eval_duration"`
		EvalDuration       int64  `json:"eval_duration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	elapsed := time.Since(started).Milliseconds()
	return &llm.GenerateResponse{
		Text:             result.Response,
		TokensUsed:       result.PromptEvalCount + result.EvalCount,
		ProcessingTimeMs: elapsed,
	}, nil
}

// StreamGenerate generates a streaming completion.
func (p *OllamaProvider) StreamGenerate(ctx context.Context, req llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	payload := map[string]interface{}{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": true,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if req.MaxTokens > 0 {
		payload["options"].(map[string]interface{})["num_predict"] = req.MaxTokens
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("generation failed: status %d", resp.StatusCode)
	}

	chunks := make(chan llm.GenerateChunk, 100)

	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var result struct {
				Response string `json:"response"`
				Done     bool   `json:"done"`
			}

			if err := decoder.Decode(&result); err != nil {
				if err != io.EOF {
					errStr := err.Error()
					chunks <- llm.GenerateChunk{Error: &errStr}
				}
				return
			}

			chunks <- llm.GenerateChunk{
				Text: result.Response,
				Done: result.Done,
			}

			if result.Done {
				return
			}
		}
	}()

	return chunks, nil
}

// Ensure OllamaProvider implements Provider
var _ llm.Provider = (*OllamaProvider)(nil)
