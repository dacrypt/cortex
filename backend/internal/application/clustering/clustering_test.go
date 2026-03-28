package clustering

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TestCommunityDetector_Louvain tests the Louvain community detection algorithm.
func TestCommunityDetector_Louvain(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	// Create a simple graph with two obvious communities:
	// Community 1: doc1, doc2, doc3 (tightly connected)
	// Community 2: doc4, doc5, doc6 (tightly connected)
	// One weak edge between communities

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")
	doc4 := entity.DocumentID("doc4")
	doc5 := entity.DocumentID("doc5")
	doc6 := entity.DocumentID("doc6")

	// Community 1 edges (strong connections)
	addEdge(graph, doc1, doc2, workspaceID, 0.9)
	addEdge(graph, doc1, doc3, workspaceID, 0.85)
	addEdge(graph, doc2, doc3, workspaceID, 0.8)

	// Community 2 edges (strong connections)
	addEdge(graph, doc4, doc5, workspaceID, 0.95)
	addEdge(graph, doc4, doc6, workspaceID, 0.88)
	addEdge(graph, doc5, doc6, workspaceID, 0.82)

	// Weak inter-community edge
	addEdge(graph, doc3, doc4, workspaceID, 0.25)

	ctx := context.Background()
	communities, err := detector.DetectCommunities(ctx, graph)
	if err != nil {
		t.Fatalf("DetectCommunities failed: %v", err)
	}

	// Should detect at least 2 communities
	if len(communities) < 2 {
		t.Errorf("Expected at least 2 communities, got %d", len(communities))
	}

	// Verify that the communities are coherent
	for _, comm := range communities {
		if len(comm.Members) < 2 {
			t.Errorf("Expected community to have at least 2 members, got %d", len(comm.Members))
		}
	}

	t.Logf("Detected %d communities", len(communities))
	for i, comm := range communities {
		t.Logf("Community %d: %d members, modularity: %.3f", i, len(comm.Members), comm.Modularity)
	}
}

// TestCommunityDetector_SingletonNodes tests handling of isolated nodes.
func TestCommunityDetector_SingletonNodes(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	config.MinCommunitySize = 2
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	// Create a graph with some connected nodes and one isolated
	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3") // isolated

	addEdge(graph, doc1, doc2, workspaceID, 0.8)

	// Add doc3 as a node without edges
	graph.AddNode(doc3)

	ctx := context.Background()
	communities, err := detector.DetectCommunities(ctx, graph)
	if err != nil {
		t.Fatalf("DetectCommunities failed: %v", err)
	}

	// Isolated node should not form its own community (min size = 2)
	for _, comm := range communities {
		for _, member := range comm.Members {
			if member == doc3 {
				t.Errorf("Isolated node doc3 should not be in any community")
			}
		}
	}
}

// TestCommunityDetector_EmptyGraph tests handling of empty graphs.
func TestCommunityDetector_EmptyGraph(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	ctx := context.Background()
	communities, err := detector.DetectCommunities(ctx, graph)
	if err != nil {
		t.Fatalf("DetectCommunities failed: %v", err)
	}

	if len(communities) != 0 {
		t.Errorf("Expected 0 communities for empty graph, got %d", len(communities))
	}
}

// TestGraphBuilder_CosineSimilarity tests the cosine similarity function.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		delta    float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "similar vectors",
			a:        []float32{1.0, 1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.707, // 1/sqrt(2)
			delta:    0.01,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if diff := abs(result - tt.expected); diff > tt.delta {
				t.Errorf("cosineSimilarity(%v, %v) = %f, expected %f (diff: %f)", tt.a, tt.b, result, tt.expected, diff)
			}
		})
	}
}

// TestDocumentGraph_EdgeOperations tests graph edge operations.
func TestDocumentGraph_EdgeOperations(t *testing.T) {
	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")

	// Add edges
	edge1 := entity.NewDocumentEdge(doc1, doc2, workspaceID)
	edge1.AddSource(entity.EdgeSource{
		Type:   entity.EdgeSourceSemantic,
		Weight: 0.8,
	})
	graph.AddEdge(edge1)

	edge2 := entity.NewDocumentEdge(doc2, doc3, workspaceID)
	edge2.AddSource(entity.EdgeSource{
		Type:   entity.EdgeSourceStructural,
		Weight: 0.5,
	})
	graph.AddEdge(edge2)

	// Test node count
	if graph.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", graph.NodeCount())
	}

	// Test edge count
	if graph.EdgeCount() != 2 {
		t.Errorf("Expected 2 edges, got %d", graph.EdgeCount())
	}

	// Test neighbors
	neighbors := graph.GetNeighbors(doc2)
	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors for doc2, got %d", len(neighbors))
	}

	// Test GetEdge
	edge := graph.GetEdge(doc1, doc2)
	if edge == nil {
		t.Error("Expected to find edge between doc1 and doc2")
	}
	if edge != nil && edge.Weight != 0.8 {
		t.Errorf("Expected edge weight 0.8, got %f", edge.Weight)
	}

	// Test edge directionality (should be bidirectional)
	edgeReverse := graph.GetEdge(doc2, doc1)
	if edgeReverse == nil {
		t.Error("Expected to find edge between doc2 and doc1 (reverse)")
	}
}

// TestCommunityDetector_CompleteGraph tests community detection on a complete graph.
func TestCommunityDetector_CompleteGraph(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	// Create a simple complete graph (clique) of 3 nodes
	doc1 := entity.DocumentID("doc1")
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")

	addEdge(graph, doc1, doc2, workspaceID, 1.0)
	addEdge(graph, doc1, doc3, workspaceID, 1.0)
	addEdge(graph, doc2, doc3, workspaceID, 1.0)

	ctx := context.Background()
	communities, err := detector.DetectCommunities(ctx, graph)
	if err != nil {
		t.Fatalf("DetectCommunities failed: %v", err)
	}

	// For a small complete graph, Louvain may or may not merge all nodes.
	// Nodes in communities smaller than MinCommunitySize are filtered out.
	t.Logf("Complete graph of 3 nodes produced %d communities", len(communities))

	// Verify communities are valid
	totalNodes := 0
	for _, comm := range communities {
		totalNodes += len(comm.Members)
		// Each community should have at least MinCommunitySize members
		if len(comm.Members) < config.MinCommunitySize {
			t.Errorf("Community has %d members, less than minimum %d", len(comm.Members), config.MinCommunitySize)
		}
	}
	t.Logf("Total nodes in communities: %d (some may be orphaned due to MinCommunitySize filter)", totalNodes)

	// At least one valid community should exist
	if len(communities) == 0 {
		t.Error("Expected at least one community")
	}
}

// TestCommunityDetector_ConvertToDocumentClusters tests cluster conversion.
func TestCommunityDetector_ConvertToDocumentClusters(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()

	communities := []*Community{
		{
			ID:         0,
			Members:    []entity.DocumentID{"doc1", "doc2", "doc3"},
			Modularity: 0.45,
		},
		{
			ID:         1,
			Members:    []entity.DocumentID{"doc4", "doc5"},
			Modularity: 0.35,
		},
	}

	ctx := context.Background()
	clusters := detector.ConvertToDocumentClusters(ctx, workspaceID, communities)

	if len(clusters) != 2 {
		t.Fatalf("Expected 2 clusters, got %d", len(clusters))
	}

	// Check first cluster
	if clusters[0].MemberCount != 3 {
		t.Errorf("Expected first cluster to have 3 members, got %d", clusters[0].MemberCount)
	}
	if clusters[0].WorkspaceID != workspaceID {
		t.Error("Cluster workspace ID mismatch")
	}
	if clusters[0].Status != entity.ClusterStatusPending {
		t.Errorf("Expected pending status, got %s", clusters[0].Status)
	}

	// Check second cluster
	if clusters[1].MemberCount != 2 {
		t.Errorf("Expected second cluster to have 2 members, got %d", clusters[1].MemberCount)
	}
}

func TestCommunityDetector_ConvertToDocumentClusters_DeterministicID(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	communityA := []*Community{
		{
			ID:         0,
			Members:    []entity.DocumentID{"doc3", "doc1", "doc2"},
			Modularity: 0.4,
		},
	}
	communityB := []*Community{
		{
			ID:         0,
			Members:    []entity.DocumentID{"doc2", "doc3", "doc1"},
			Modularity: 0.4,
		},
	}

	ctx := context.Background()
	clustersA := detector.ConvertToDocumentClusters(ctx, workspaceID, communityA)
	clustersB := detector.ConvertToDocumentClusters(ctx, workspaceID, communityB)

	if len(clustersA) != 1 || len(clustersB) != 1 {
		t.Fatalf("Expected 1 cluster in each conversion, got %d and %d", len(clustersA), len(clustersB))
	}
	if clustersA[0].ID != clustersB[0].ID {
		t.Errorf("Expected deterministic cluster ID for same members, got %s and %s", clustersA[0].ID, clustersB[0].ID)
	}
}

// TestCommunityDetector_FindCentralNodes tests central node identification.
func TestCommunityDetector_FindCentralNodes(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultCommunityDetectorConfig()
	detector := NewCommunityDetector(config, logger)

	workspaceID := entity.NewWorkspaceID()
	graph := entity.NewDocumentGraph(workspaceID)

	// Create a star graph: doc1 is the hub connected to all others
	doc1 := entity.DocumentID("doc1") // hub
	doc2 := entity.DocumentID("doc2")
	doc3 := entity.DocumentID("doc3")
	doc4 := entity.DocumentID("doc4")

	addEdge(graph, doc1, doc2, workspaceID, 0.8)
	addEdge(graph, doc1, doc3, workspaceID, 0.8)
	addEdge(graph, doc1, doc4, workspaceID, 0.8)

	members := []entity.DocumentID{doc1, doc2, doc3, doc4}
	centralNodes := detector.FindCentralNodes(graph, members, 2)

	if len(centralNodes) != 2 {
		t.Fatalf("Expected 2 central nodes, got %d", len(centralNodes))
	}

	// doc1 should be the most central (highest degree)
	if centralNodes[0] != doc1 {
		t.Errorf("Expected doc1 to be most central, got %s", centralNodes[0])
	}
}

// Helper functions

func addEdge(graph *entity.DocumentGraph, from, to entity.DocumentID, workspaceID entity.WorkspaceID, weight float64) {
	edge := entity.NewDocumentEdge(from, to, workspaceID)
	edge.AddSource(entity.EdgeSource{
		Type:   entity.EdgeSourceSemantic,
		Weight: weight,
	})
	graph.AddEdge(edge)
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
