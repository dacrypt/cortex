package rag

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	embedding "github.com/dacrypt/cortex/backend/internal/infrastructure/embedding"
)

// --- Mock VectorStore ---

type mockVectorStore struct {
	searchResults []repository.VectorMatch
	searchErr     error
}

func (m *mockVectorStore) Upsert(_ context.Context, _ entity.WorkspaceID, _ entity.ChunkEmbedding) error {
	return nil
}

func (m *mockVectorStore) BulkUpsert(_ context.Context, _ entity.WorkspaceID, _ []entity.ChunkEmbedding) error {
	return nil
}

func (m *mockVectorStore) DeleteByDocument(_ context.Context, _ entity.WorkspaceID, _ entity.DocumentID) error {
	return nil
}

func (m *mockVectorStore) Search(_ context.Context, _ entity.WorkspaceID, _ []float32, _ int) ([]repository.VectorMatch, error) {
	return m.searchResults, m.searchErr
}

// --- Mock DocumentRepository ---

type mockDocumentRepository struct {
	documents map[entity.DocumentID]*entity.Document
	chunks    map[entity.ChunkID]*entity.Chunk
}

func newMockDocumentRepository() *mockDocumentRepository {
	return &mockDocumentRepository{
		documents: make(map[entity.DocumentID]*entity.Document),
		chunks:    make(map[entity.ChunkID]*entity.Chunk),
	}
}

func (m *mockDocumentRepository) UpsertDocument(_ context.Context, _ entity.WorkspaceID, doc *entity.Document) error {
	m.documents[doc.ID] = doc
	return nil
}

func (m *mockDocumentRepository) GetDocument(_ context.Context, _ entity.WorkspaceID, id entity.DocumentID) (*entity.Document, error) {
	doc, ok := m.documents[id]
	if !ok {
		return nil, nil
	}
	return doc, nil
}

func (m *mockDocumentRepository) GetDocumentByPath(_ context.Context, _ entity.WorkspaceID, relativePath string) (*entity.Document, error) {
	for _, doc := range m.documents {
		if doc.RelativePath == relativePath {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *mockDocumentRepository) ReplaceChunks(_ context.Context, _ entity.WorkspaceID, _ entity.DocumentID, chunks []*entity.Chunk) error {
	for _, c := range chunks {
		m.chunks[c.ID] = c
	}
	return nil
}

func (m *mockDocumentRepository) GetChunksByDocument(_ context.Context, _ entity.WorkspaceID, docID entity.DocumentID) ([]*entity.Chunk, error) {
	var result []*entity.Chunk
	for _, c := range m.chunks {
		if c.DocumentID == docID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockDocumentRepository) GetChunksByIDs(_ context.Context, _ entity.WorkspaceID, ids []entity.ChunkID) ([]*entity.Chunk, error) {
	var result []*entity.Chunk
	for _, id := range ids {
		if c, ok := m.chunks[id]; ok {
			result = append(result, c)
		}
	}
	return result, nil
}

// --- Tests ---

func TestRAGService_Query_WithSources(t *testing.T) {
	docID := entity.NewDocumentID("docs/readme.md")
	chunkID := entity.NewChunkID(docID, 0, "Introduction")

	docRepo := newMockDocumentRepository()
	docRepo.documents[docID] = &entity.Document{
		ID:           docID,
		RelativePath: "docs/readme.md",
		Title:        "README",
	}
	docRepo.chunks[chunkID] = &entity.Chunk{
		ID:          chunkID,
		DocumentID:  docID,
		Ordinal:     0,
		Heading:     "Introduction",
		HeadingPath: "Introduction",
		Text:        "This is the introduction to the project.",
	}

	vectorStore := &mockVectorStore{
		searchResults: []repository.VectorMatch{
			{ChunkID: chunkID, Similarity: 0.95},
		},
	}

	hashEmbedder := embedding.NewHashEmbedder(384)
	logger := zerolog.Nop()

	svc := NewService(docRepo, vectorStore, hashEmbedder, nil, logger)

	resp, err := svc.Query(context.Background(), QueryRequest{
		WorkspaceID:    entity.WorkspaceID("ws-test"),
		Query:          "introduction",
		TopK:           5,
		GenerateAnswer: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(resp.Sources))
	}

	src := resp.Sources[0]
	if src.DocumentID != docID {
		t.Errorf("expected document ID %s, got %s", docID, src.DocumentID)
	}
	if src.ChunkID != chunkID {
		t.Errorf("expected chunk ID %s, got %s", chunkID, src.ChunkID)
	}
	if src.RelativePath != "docs/readme.md" {
		t.Errorf("expected relative path 'docs/readme.md', got %q", src.RelativePath)
	}
	if src.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", src.Score)
	}
	if resp.Answer == "" {
		t.Error("expected non-empty answer from buildAnswer fallback")
	}
}

func TestRAGService_Query_EmptyResults(t *testing.T) {
	docRepo := newMockDocumentRepository()
	vectorStore := &mockVectorStore{
		searchResults: []repository.VectorMatch{},
	}

	hashEmbedder := embedding.NewHashEmbedder(384)
	logger := zerolog.Nop()

	svc := NewService(docRepo, vectorStore, hashEmbedder, nil, logger)

	resp, err := svc.Query(context.Background(), QueryRequest{
		WorkspaceID:    entity.WorkspaceID("ws-test"),
		Query:          "nonexistent topic",
		TopK:           5,
		GenerateAnswer: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(resp.Sources))
	}
}

func TestRAGService_Query_EmptyQuery(t *testing.T) {
	docRepo := newMockDocumentRepository()
	vectorStore := &mockVectorStore{}

	hashEmbedder := embedding.NewHashEmbedder(384)
	logger := zerolog.Nop()

	svc := NewService(docRepo, vectorStore, hashEmbedder, nil, logger)

	_, err := svc.Query(context.Background(), QueryRequest{
		WorkspaceID:    entity.WorkspaceID("ws-test"),
		Query:          "",
		TopK:           5,
		GenerateAnswer: false,
	})
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}
