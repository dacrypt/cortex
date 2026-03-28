package clustering

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

type inMemoryClusterRepo struct {
	clusters     map[entity.ClusterID]*entity.DocumentCluster
	memberships  map[entity.ClusterID]map[entity.DocumentID]*entity.ClusterMembership
	clusterEdges map[string]*entity.DocumentEdge
}

func newInMemoryClusterRepo() *inMemoryClusterRepo {
	return &inMemoryClusterRepo{
		clusters:     make(map[entity.ClusterID]*entity.DocumentCluster),
		memberships:  make(map[entity.ClusterID]map[entity.DocumentID]*entity.ClusterMembership),
		clusterEdges: make(map[string]*entity.DocumentEdge),
	}
}

func (r *inMemoryClusterRepo) UpsertCluster(_ context.Context, cluster *entity.DocumentCluster) error {
	r.clusters[cluster.ID] = cluster
	return nil
}
func (r *inMemoryClusterRepo) GetCluster(_ context.Context, _ entity.WorkspaceID, id entity.ClusterID) (*entity.DocumentCluster, error) {
	return r.clusters[id], nil
}
func (r *inMemoryClusterRepo) GetClustersByWorkspace(_ context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error) {
	var clusters []*entity.DocumentCluster
	for _, cluster := range r.clusters {
		if cluster.WorkspaceID == workspaceID {
			clusters = append(clusters, cluster)
		}
	}
	return clusters, nil
}
func (r *inMemoryClusterRepo) GetActiveClustersByWorkspace(_ context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error) {
	var clusters []*entity.DocumentCluster
	for _, cluster := range r.clusters {
		if cluster.WorkspaceID == workspaceID && cluster.Status == entity.ClusterStatusActive {
			clusters = append(clusters, cluster)
		}
	}
	return clusters, nil
}
func (r *inMemoryClusterRepo) DeleteCluster(_ context.Context, _ entity.WorkspaceID, id entity.ClusterID) error {
	delete(r.clusters, id)
	delete(r.memberships, id)
	return nil
}
func (r *inMemoryClusterRepo) UpdateClusterStatus(_ context.Context, _ entity.WorkspaceID, id entity.ClusterID, status entity.ClusterStatus) error {
	if cluster, ok := r.clusters[id]; ok {
		cluster.Status = status
	}
	return nil
}
func (r *inMemoryClusterRepo) AddMembership(_ context.Context, membership *entity.ClusterMembership) error {
	if r.memberships[membership.ClusterID] == nil {
		r.memberships[membership.ClusterID] = make(map[entity.DocumentID]*entity.ClusterMembership)
	}
	r.memberships[membership.ClusterID][membership.DocumentID] = membership
	return nil
}
func (r *inMemoryClusterRepo) RemoveMembership(_ context.Context, clusterID entity.ClusterID, documentID entity.DocumentID) error {
	if r.memberships[clusterID] != nil {
		delete(r.memberships[clusterID], documentID)
	}
	return nil
}
func (r *inMemoryClusterRepo) GetMembershipsByCluster(_ context.Context, _ entity.WorkspaceID, clusterID entity.ClusterID) ([]*entity.ClusterMembership, error) {
	var memberships []*entity.ClusterMembership
	for _, membership := range r.memberships[clusterID] {
		memberships = append(memberships, membership)
	}
	return memberships, nil
}
func (r *inMemoryClusterRepo) GetMembershipsByDocument(_ context.Context, _ entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.ClusterMembership, error) {
	var memberships []*entity.ClusterMembership
	for _, clusterMembers := range r.memberships {
		if membership, ok := clusterMembers[documentID]; ok {
			memberships = append(memberships, membership)
		}
	}
	return memberships, nil
}
func (r *inMemoryClusterRepo) GetClusterMembers(_ context.Context, _ entity.WorkspaceID, clusterID entity.ClusterID) ([]entity.DocumentID, error) {
	var members []entity.DocumentID
	for docID := range r.memberships[clusterID] {
		members = append(members, docID)
	}
	return members, nil
}
func (r *inMemoryClusterRepo) GetClusterMembersWithInfo(_ context.Context, _ entity.WorkspaceID, clusterID entity.ClusterID) ([]*repository.ClusterMemberInfo, error) {
	members := r.memberships[clusterID]
	out := make([]*repository.ClusterMemberInfo, 0, len(members))
	for _, membership := range members {
		out = append(out, &repository.ClusterMemberInfo{
			DocumentID:      membership.DocumentID,
			MembershipScore: membership.Score,
			IsCentral:       membership.IsCentral,
		})
	}
	return out, nil
}
func (r *inMemoryClusterRepo) UpdateMembershipScore(_ context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, score float64) error {
	if membership, ok := r.memberships[clusterID][documentID]; ok {
		membership.Score = score
	}
	return nil
}
func (r *inMemoryClusterRepo) SetCentralNode(_ context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, isCentral bool) error {
	if membership, ok := r.memberships[clusterID][documentID]; ok {
		membership.IsCentral = isCentral
	}
	return nil
}
func (r *inMemoryClusterRepo) UpsertEdge(_ context.Context, edge *entity.DocumentEdge) error {
	r.clusterEdges[edgeKey(edge.FromDoc, edge.ToDoc)] = edge
	return nil
}
func (r *inMemoryClusterRepo) GetEdge(_ context.Context, _ entity.WorkspaceID, fromDoc, toDoc entity.DocumentID) (*entity.DocumentEdge, error) {
	key := edgeKey(fromDoc, toDoc)
	return r.clusterEdges[key], nil
}
func (r *inMemoryClusterRepo) GetEdgesByDocument(_ context.Context, _ entity.WorkspaceID, _ entity.DocumentID) ([]*entity.DocumentEdge, error) {
	return nil, nil
}
func (r *inMemoryClusterRepo) GetAllEdges(_ context.Context, _ entity.WorkspaceID) ([]*entity.DocumentEdge, error) {
	return nil, nil
}
func (r *inMemoryClusterRepo) GetEdgesAboveThreshold(_ context.Context, _ entity.WorkspaceID, _ float64) ([]*entity.DocumentEdge, error) {
	return nil, nil
}
func (r *inMemoryClusterRepo) DeleteEdge(_ context.Context, _ entity.WorkspaceID, _ entity.DocumentID, _ entity.DocumentID) error {
	return nil
}
func (r *inMemoryClusterRepo) DeleteEdgesByDocument(_ context.Context, _ entity.WorkspaceID, _ entity.DocumentID) error {
	return nil
}
func (r *inMemoryClusterRepo) LoadGraph(_ context.Context, workspaceID entity.WorkspaceID, _ float64) (*entity.DocumentGraph, error) {
	return entity.NewDocumentGraph(workspaceID), nil
}
func (r *inMemoryClusterRepo) UpsertEdgesBatch(_ context.Context, _ []*entity.DocumentEdge) error {
	return nil
}
func (r *inMemoryClusterRepo) ClearClusterMemberships(_ context.Context, _ entity.WorkspaceID, clusterID entity.ClusterID) error {
	r.memberships[clusterID] = make(map[entity.DocumentID]*entity.ClusterMembership)
	return nil
}
func (r *inMemoryClusterRepo) ClearAllClusters(_ context.Context, _ entity.WorkspaceID) error {
	r.clusters = make(map[entity.ClusterID]*entity.DocumentCluster)
	r.memberships = make(map[entity.ClusterID]map[entity.DocumentID]*entity.ClusterMembership)
	return nil
}
func (r *inMemoryClusterRepo) GetClusterStats(_ context.Context, _ entity.WorkspaceID) (*repository.ClusterStats, error) {
	return &repository.ClusterStats{}, nil
}
func (r *inMemoryClusterRepo) GetClusterFacet(_ context.Context, _ entity.WorkspaceID, _ []entity.FileID) (map[string]int, error) {
	return map[string]int{}, nil
}

type stubEmbeddingConn struct {
	embeddings []DocumentEmbedding
}

func (s *stubEmbeddingConn) GetAllEmbeddings(_ context.Context, _ entity.WorkspaceID) ([]DocumentEmbedding, error) {
	return s.embeddings, nil
}

type sequentialMockProvider struct {
	id        string
	name      string
	responses []string
	index     int
}

func (m *sequentialMockProvider) ID() string   { return m.id }
func (m *sequentialMockProvider) Name() string { return m.name }
func (m *sequentialMockProvider) Type() string { return "mock" }
func (m *sequentialMockProvider) IsAvailable(_ context.Context) (bool, error) {
	return true, nil
}
func (m *sequentialMockProvider) ListModels(_ context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{{Name: "mock"}}, nil
}
func (m *sequentialMockProvider) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if m.index >= len(m.responses) {
		return &llm.GenerateResponse{Text: ""}, nil
	}
	resp := m.responses[m.index]
	m.index++
	return &llm.GenerateResponse{Text: resp}, nil
}
func (m *sequentialMockProvider) StreamGenerate(_ context.Context, _ llm.GenerateRequest) (<-chan llm.GenerateChunk, error) {
	ch := make(chan llm.GenerateChunk)
	close(ch)
	return ch, nil
}

func TestService_RunClustering_RenamesWhenMembershipChanges(t *testing.T) {
	logger := zerolog.Nop()
	workspaceID := entity.NewWorkspaceID()

	clusterRepo := newInMemoryClusterRepo()
	conn := &stubEmbeddingConn{}
	graphConfig := DefaultGraphBuilderConfig()
	graphConfig.MinSemanticSimilarity = 0.0
	graphConfig.MinEdgeWeight = 0.0
	graphBuilder := NewGraphBuilder(nil, nil, nil, nil, clusterRepo, conn, graphConfig, logger)
	detector := NewCommunityDetector(DefaultCommunityDetectorConfig(), logger)

	provider := &sequentialMockProvider{
		id:   "mock",
		name: "Mock",
		responses: []string{
			`{"is_valid": true, "confidence": 1.0, "reason": "ok"}`,
			`{"name":"Alpha Cluster","summary":"First summary","keywords":["alpha"],"entities":["Alpha"]}`,
			`{"is_valid": true, "confidence": 1.0, "reason": "ok"}`,
			`{"name":"Alpha Cluster Updated","summary":"Updated summary","keywords":["alpha","beta"],"entities":["Alpha"]}`,
		},
	}
	router := llm.NewRouter(logger)
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")
	doc4 := entity.DocumentID("doc4")
	docProvider := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Doc One", RelativePath: "one.txt", Summary: "First"},
			doc2: {ID: doc2, Title: "Doc Two", RelativePath: "two.txt", Summary: "Second"},
			doc3: {ID: doc3, Title: "Doc Three", RelativePath: "three.txt", Summary: "Third"},
			doc4: {ID: doc4, Title: "Doc Four", RelativePath: "four.txt", Summary: "Fourth"},
		},
	}
	validator := NewLLMValidator(router, docProvider, DefaultLLMValidatorConfig(), logger)

	serviceConfig := DefaultServiceConfig()
	serviceConfig.CreateProjectsAuto = false
	service := NewService(graphBuilder, detector, validator, clusterRepo, nil, serviceConfig, logger)

	conn.embeddings = []DocumentEmbedding{
		{DocumentID: doc1, Vector: []float32{1, 0}},
		{DocumentID: doc2, Vector: []float32{1, 0}},
		{DocumentID: doc3, Vector: []float32{1, 0}},
	}
	if _, err := service.RunClustering(context.Background(), workspaceID, false); err != nil {
		t.Fatalf("RunClustering failed: %v", err)
	}

	clusters, _ := clusterRepo.GetActiveClustersByWorkspace(context.Background(), workspaceID)
	if len(clusters) != 1 {
		t.Fatalf("Expected 1 cluster after first run, got %d", len(clusters))
	}
	if clusters[0].Name != "Alpha Cluster" {
		t.Fatalf("Expected initial cluster name to be set, got %q", clusters[0].Name)
	}

	// Simulate membership drift so the service should not preserve the old name.
	clusterRepo.RemoveMembership(context.Background(), clusters[0].ID, doc3)

	// Keep embeddings the same to ensure communities are consistent across runs.
	if _, err := service.RunClustering(context.Background(), workspaceID, false); err != nil {
		t.Fatalf("RunClustering failed: %v", err)
	}

	updated, _ := clusterRepo.GetActiveClustersByWorkspace(context.Background(), workspaceID)
	if len(updated) != 1 {
		t.Fatalf("Expected 1 cluster after update, got %d", len(updated))
	}
	if updated[0].Name != "Alpha Cluster Updated" {
		t.Fatalf("Expected cluster name to update after membership change, got %q", updated[0].Name)
	}
}

func TestService_RunClustering_PreservesNameWhenMembershipStable(t *testing.T) {
	logger := zerolog.Nop()
	workspaceID := entity.NewWorkspaceID()

	clusterRepo := newInMemoryClusterRepo()
	conn := &stubEmbeddingConn{}
	graphConfig := DefaultGraphBuilderConfig()
	graphConfig.MinSemanticSimilarity = 0.0
	graphConfig.MinEdgeWeight = 0.0
	graphBuilder := NewGraphBuilder(nil, nil, nil, nil, clusterRepo, conn, graphConfig, logger)
	detector := NewCommunityDetector(DefaultCommunityDetectorConfig(), logger)

	provider := &sequentialMockProvider{
		id:   "mock",
		name: "Mock",
		responses: []string{
			`{"is_valid": true, "confidence": 1.0, "reason": "ok"}`,
			`{"name":"Stable Name","summary":"First summary","keywords":["alpha"],"entities":["Alpha"]}`,
			`{"is_valid": true, "confidence": 1.0, "reason": "ok"}`,
			`{"name":"New Name","summary":"Second summary","keywords":["beta"],"entities":["Beta"]}`,
		},
	}
	router := llm.NewRouter(logger)
	router.RegisterProvider(provider)
	if err := router.SetActiveProvider("mock", "mock"); err != nil {
		t.Fatalf("SetActiveProvider failed: %v", err)
	}

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")
	docProvider := &stubDocInfoProvider{
		infos: map[entity.DocumentID]*DocumentInfo{
			doc1: {ID: doc1, Title: "Doc One", RelativePath: "one.txt", Summary: "First"},
			doc2: {ID: doc2, Title: "Doc Two", RelativePath: "two.txt", Summary: "Second"},
			doc3: {ID: doc3, Title: "Doc Three", RelativePath: "three.txt", Summary: "Third"},
		},
	}
	validator := NewLLMValidator(router, docProvider, DefaultLLMValidatorConfig(), logger)

	serviceConfig := DefaultServiceConfig()
	serviceConfig.CreateProjectsAuto = false
	service := NewService(graphBuilder, detector, validator, clusterRepo, nil, serviceConfig, logger)

	conn.embeddings = []DocumentEmbedding{
		{DocumentID: doc1, Vector: []float32{1, 0}},
		{DocumentID: doc2, Vector: []float32{1, 0}},
		{DocumentID: doc3, Vector: []float32{1, 0}},
	}
	if _, err := service.RunClustering(context.Background(), workspaceID, false); err != nil {
		t.Fatalf("RunClustering failed: %v", err)
	}

	first, _ := clusterRepo.GetActiveClustersByWorkspace(context.Background(), workspaceID)
	if len(first) != 1 {
		t.Fatalf("Expected 1 cluster after first run, got %d", len(first))
	}
	if first[0].Name != "Stable Name" {
		t.Fatalf("Expected initial cluster name to be set, got %q", first[0].Name)
	}

	if _, err := service.RunClustering(context.Background(), workspaceID, false); err != nil {
		t.Fatalf("RunClustering failed: %v", err)
	}

	second, _ := clusterRepo.GetActiveClustersByWorkspace(context.Background(), workspaceID)
	if len(second) != 1 {
		t.Fatalf("Expected 1 cluster after second run, got %d", len(second))
	}
	if second[0].Name != "Stable Name" {
		t.Fatalf("Expected cluster name to remain stable, got %q", second[0].Name)
	}
}
