package clustering

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

type stubDocInfoProvider struct {
	infos map[entity.DocumentID]*DocumentInfo
}

func (s *stubDocInfoProvider) GetDocumentInfo(_ context.Context, _ entity.WorkspaceID, docID entity.DocumentID) (*DocumentInfo, error) {
	return s.infos[docID], nil
}

type mockLLMProvider struct {
	id           string
	name         string
	responseText string
	lastRequest  *llm.GenerateRequest
}

func (m *mockLLMProvider) ID() string   { return m.id }
func (m *mockLLMProvider) Name() string { return m.name }
func (m *mockLLMProvider) Type() string { return "mock" }
func (m *mockLLMProvider) IsAvailable(_ context.Context) (bool, error) {
	return true, nil
}
func (m *mockLLMProvider) ListModels(_ context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{{Name: "mock"}}, nil
}
func (m *mockLLMProvider) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	m.lastRequest = &req
	return &llm.GenerateResponse{Text: m.responseText}, nil
}
func (m *mockLLMProvider) StreamGenerate(_ context.Context, _ llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	ch := make(chan llm.GenerateChunk)
	close(ch)
	return ch, nil
}

func TestLLMValidator_GenerateClusterMetadata(t *testing.T) {
	logger := zerolog.Nop()
	router := llm.NewRouter(logger)
	provider := &mockLLMProvider{
		id:           "mock",
		name:         "Mock",
		responseText: `{"name":"Business Docs","summary":"Invoices and budgets","keywords":["invoice","budget"],"entities":["Finance"]}`,
	}
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	providerDoc := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Invoice Q1", RelativePath: "finance/invoice-q1.pdf", Summary: "Quarterly invoice summary"},
			doc2: {ID: doc2, Title: "Budget 2025", RelativePath: "finance/budget-2025.xlsx", Summary: "Budget plan for 2025"},
		},
	}

	validator := NewLLMValidator(router, providerDoc, DefaultLLMValidatorConfig(), logger)
	cluster := &entity.DocumentCluster{ID: entity.NewClusterID(), WorkspaceID: entity.NewWorkspaceID()}
	if err := validator.GenerateClusterMetadata(context.Background(), cluster, []entity.DocumentID{doc1, doc2}); err != nil {
		t.Fatalf("GenerateClusterMetadata failed: %v", err)
	}

	if cluster.Name != "Business Docs" {
		t.Errorf("Expected cluster name from LLM, got %q", cluster.Name)
	}
	if cluster.Summary != "Invoices and budgets" {
		t.Errorf("Expected cluster summary from LLM, got %q", cluster.Summary)
	}
	if len(cluster.TopKeywords) != 2 || cluster.TopKeywords[0] != "invoice" {
		t.Errorf("Expected keywords from LLM, got %v", cluster.TopKeywords)
	}
	if len(cluster.TopEntities) != 1 || cluster.TopEntities[0] != "Finance" {
		t.Errorf("Expected entities from LLM, got %v", cluster.TopEntities)
	}

	if provider.lastRequest == nil {
		t.Fatal("Expected LLM provider to receive a request")
	}
	prompt := provider.lastRequest.Prompt
	if !strings.Contains(prompt, "Invoice Q1") || !strings.Contains(prompt, "Budget 2025") {
		t.Errorf("Expected prompt to include document titles, got: %s", prompt)
	}
	if !strings.Contains(prompt, "finance/invoice-q1.pdf") || !strings.Contains(prompt, "finance/budget-2025.xlsx") {
		t.Errorf("Expected prompt to include document paths, got: %s", prompt)
	}
}

func TestLLMValidator_MaxDocsForContext(t *testing.T) {
	logger := zerolog.Nop()
	router := llm.NewRouter(logger)
	provider := &mockLLMProvider{
		id:           "mock",
		name:         "Mock",
		responseText: `{"name":"Mixed Docs","summary":"Summary","keywords":["a"],"entities":["b"]}`,
	}
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")
	providerDoc := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Doc One", RelativePath: "one.txt", Summary: "First"},
			doc2: {ID: doc2, Title: "Doc Two", RelativePath: "two.txt", Summary: "Second"},
			doc3: {ID: doc3, Title: "Doc Three", RelativePath: "three.txt", Summary: "Third"},
		},
	}

	config := DefaultLLMValidatorConfig()
	config.MaxDocsForContext = 2
	validator := NewLLMValidator(router, providerDoc, config, logger)

	cluster := &entity.DocumentCluster{ID: entity.NewClusterID(), WorkspaceID: entity.NewWorkspaceID()}
	if err := validator.GenerateClusterMetadata(context.Background(), cluster, []entity.DocumentID{doc1, doc2, doc3}); err != nil {
		t.Fatalf("GenerateClusterMetadata failed: %v", err)
	}

	if provider.lastRequest == nil {
		t.Fatal("Expected LLM provider to receive a request")
	}
	prompt := provider.lastRequest.Prompt
	if !strings.Contains(prompt, "Doc One") || !strings.Contains(prompt, "Doc Two") {
		t.Errorf("Expected prompt to include first two docs, got: %s", prompt)
	}
	if strings.Contains(prompt, "Doc Three") {
		t.Errorf("Expected prompt to exclude third doc due to MaxDocsForContext, got: %s", prompt)
	}
}

func TestLLMValidator_InvalidMetadataResponseDoesNotOverwrite(t *testing.T) {
	logger := zerolog.Nop()
	router := llm.NewRouter(logger)
	provider := &mockLLMProvider{
		id:           "mock",
		name:         "Mock",
		responseText: "not json",
	}
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	providerDoc := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Doc One", RelativePath: "one.txt", Summary: "First"},
		},
	}

	validator := NewLLMValidator(router, providerDoc, DefaultLLMValidatorConfig(), logger)
	cluster := &entity.DocumentCluster{
		ID:          entity.NewClusterID(),
		WorkspaceID: entity.NewWorkspaceID(),
		Name:        "Existing Name",
		Summary:     "Existing Summary",
	}
	if err := validator.GenerateClusterMetadata(context.Background(), cluster, []entity.DocumentID{doc1}); err != nil {
		t.Fatalf("GenerateClusterMetadata failed: %v", err)
	}

	if cluster.Name != "Existing Name" {
		t.Errorf("Expected name to remain unchanged, got %q", cluster.Name)
	}
	if cluster.Summary != "Existing Summary" {
		t.Errorf("Expected summary to remain unchanged, got %q", cluster.Summary)
	}
}

func TestLLMValidator_GenericNameUsesKeywords(t *testing.T) {
	logger := zerolog.Nop()
	router := llm.NewRouter(logger)
	provider := &mockLLMProvider{
		id:           "mock",
		name:         "Mock",
		responseText: `{"name":"Documents","summary":"Summary","keywords":["budget","invoices"],"entities":["Finance"]}`,
	}
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	providerDoc := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Invoice Q1", RelativePath: "finance/invoice-q1.pdf", Summary: "Quarterly invoice summary"},
		},
	}

	validator := NewLLMValidator(router, providerDoc, DefaultLLMValidatorConfig(), logger)
	cluster := &entity.DocumentCluster{ID: entity.NewClusterID(), WorkspaceID: entity.NewWorkspaceID()}
	if err := validator.GenerateClusterMetadata(context.Background(), cluster, []entity.DocumentID{doc1}); err != nil {
		t.Fatalf("GenerateClusterMetadata failed: %v", err)
	}

	if cluster.Name == "Documents" || cluster.Name == "" {
		t.Fatalf("Expected generic name to be replaced, got %q", cluster.Name)
	}
}
