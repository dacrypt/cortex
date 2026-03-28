package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"embedding": [0.1, 0.2, 0.3]}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	vector, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := []float32{0.1, 0.2, 0.3}
	if len(vector) != len(expected) {
		t.Fatalf("expected vector length %d, got %d", len(expected), len(vector))
	}
	for i, v := range vector {
		if v != expected[i] {
			t.Errorf("vector[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestOllamaEmbedder_Embed_EmptyText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for empty text")
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	_, err := embedder.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

func TestOllamaEmbedder_Embed_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	_, err := embedder.Embed(context.Background(), "hello world")
	if err == nil {
		t.Fatal("expected error for server error response, got nil")
	}
}

func TestOllamaEmbedder_IsAvailable_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	if !embedder.IsAvailable(context.Background()) {
		t.Fatal("expected IsAvailable to return true")
	}
}

func TestOllamaEmbedder_IsAvailable_False(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	if embedder.IsAvailable(context.Background()) {
		t.Fatal("expected IsAvailable to return false")
	}
}

func TestOllamaEmbedder_Dimensions(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"nomic-embed-text", 768},
		{"mxbai-embed-large", 1024},
		{"all-minilm", 384},
		{"unknown-model", 768},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			embedder := NewOllamaEmbedder("http://localhost:11434", tc.model)
			if got := embedder.Dimensions(); got != tc.expected {
				t.Errorf("Dimensions() for model %q = %d, want %d", tc.model, got, tc.expected)
			}
		})
	}
}
