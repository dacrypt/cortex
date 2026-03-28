package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// TikaClient is an HTTP client for Apache Tika Server.
type TikaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewTikaClient creates a new Tika client.
func NewTikaClient(baseURL string, timeout time.Duration, logger zerolog.Logger) *TikaClient {
	return &TikaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.With().Str("component", "tika_client").Logger(),
	}
}

// DetectContentType detects the MIME type of a file using Tika.
// POST /tika endpoint returns the detected content type.
func (c *TikaClient) DetectContentType(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/tika", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to detect content type: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tika server returned status %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Read response body as content type
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %w", err)
		}
		contentType = string(body)
	}

	return contentType, nil
}

// ExtractMetadata extracts metadata from a file in JSON format.
// GET /meta endpoint returns metadata as JSON.
func (c *TikaClient) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/meta", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tika server returned status %d: %s", resp.StatusCode, string(body))
	}

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata JSON: %w", err)
	}

	return metadata, nil
}

// ExtractText extracts plain text from a document.
// GET /tika endpoint returns the extracted text.
func (c *TikaClient) ExtractText(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/tika", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to extract text: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tika server returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// DetectLanguage detects the language of the content.
// GET /language/stream endpoint returns the detected language code.
func (c *TikaClient) DetectLanguage(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/language/stream", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tika server returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	langCode := string(bytes.TrimSpace(body))
	return langCode, nil
}

// HealthCheck verifies that Tika Server is available.
// Uses GET /tika which returns 405 (Method Not Allowed) if server is up but expects PUT,
// or 200 if server is fully operational.
func (c *TikaClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/tika", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tika server not available: %w", err)
	}
	defer resp.Body.Close()

	// Tika Server returns 405 (Method Not Allowed) for GET /tika when it's running
	// This is expected - it means the server is up but expects PUT with file content
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return nil // Server is available
	}

	// 200 OK also means server is available
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return fmt.Errorf("tika server returned unexpected status %d", resp.StatusCode)
}

