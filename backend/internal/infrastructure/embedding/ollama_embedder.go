// Package embedding provides text-to-vector embedding implementations.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	embedRetryMaxAttempts = 100                // Effectively unlimited retries
	embedRetryBaseTimeout = 30 * time.Minute   // Very long base timeout for analysis
	embedRetryMaxTimeout  = 60 * time.Minute   // Effectively unlimited
	embedRetryBaseBackoff = 5 * time.Second
	embedRetryMaxBackoff  = 60 * time.Second
)

// OllamaEmbedder uses Ollama's /api/embeddings endpoint for embeddings.
type OllamaEmbedder struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewOllamaEmbedder creates a new Ollama-based embedder.
func NewOllamaEmbedder(endpoint, model string) *OllamaEmbedder {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	return &OllamaEmbedder{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 30 * time.Minute, // Effectively unlimited for analysis
		},
	}
}

// embeddingRequest is the request body for Ollama embeddings API.
type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embeddingResponse is the response from Ollama embeddings API.
type embeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Embed generates an embedding vector for the given text.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	startTotal := time.Now()
	defer func() {
		log.Printf("[EMBED_TIMING] Total embedding time: %v, text_len=%d", time.Since(startTotal), len(text))
	}()

	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	baseTimeout := e.client.Timeout
	if baseTimeout <= 0 {
		baseTimeout = embedRetryBaseTimeout
	}

	reqBody := embeddingRequest{
		Model:  e.model,
		Prompt: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= embedRetryMaxAttempts; attempt++ {
		attemptTimeout := baseTimeout * time.Duration(1<<(attempt-1))
		if attemptTimeout > embedRetryMaxTimeout {
			attemptTimeout = embedRetryMaxTimeout
		}

		attemptCtx := ctx
		cancel := func() {}
		if attempt > 1 {
			attemptCtx, cancel = context.WithTimeout(context.Background(), attemptTimeout)
		}

		req, err := http.NewRequestWithContext(attemptCtx, "POST", e.endpoint+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := e.client
		if attemptTimeout != e.client.Timeout {
			client = &http.Client{Timeout: attemptTimeout}
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("embedding request failed: %w", err)
		} else {
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				lastErr = fmt.Errorf("embedding request failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
			} else {
				var result embeddingResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					resp.Body.Close()
					lastErr = fmt.Errorf("failed to decode response: %w", err)
				} else if len(result.Embedding) == 0 {
					resp.Body.Close()
					lastErr = fmt.Errorf("empty embedding returned")
				} else {
					resp.Body.Close()
					vector := make([]float32, len(result.Embedding))
					for i, v := range result.Embedding {
						vector[i] = float32(v)
					}
					cancel()
					return vector, nil
				}
			}
		}
		cancel()

		if !isTimeoutError(lastErr) || attempt == embedRetryMaxAttempts {
			break
		}

		backoff := embedRetryBaseBackoff * time.Duration(1<<(attempt-1))
		if backoff > embedRetryMaxBackoff {
			backoff = embedRetryMaxBackoff
		}
		time.Sleep(backoff)
	}

	return nil, lastErr
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

// EmbedBatch generates embeddings for multiple texts.
// Note: Ollama doesn't support batch embeddings, so we process sequentially.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))

	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		embedding, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		results[i] = embedding
	}

	return results, nil
}

// IsAvailable checks if the Ollama embedding service is available.
func (e *OllamaEmbedder) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", e.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Model returns the configured model name.
func (e *OllamaEmbedder) Model() string {
	return e.model
}

// Dimensions returns the expected embedding dimensions.
// nomic-embed-text produces 768-dimensional vectors.
func (e *OllamaEmbedder) Dimensions() int {
	switch e.model {
	case "nomic-embed-text":
		return 768
	case "mxbai-embed-large":
		return 1024
	case "all-minilm":
		return 384
	default:
		return 768 // default
	}
}
