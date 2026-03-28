package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// ClusterID uniquely identifies a document cluster.
type ClusterID string

// NewClusterID creates a new unique ClusterID.
func NewClusterID() ClusterID {
	return ClusterID(uuid.New().String())
}

// NewClusterIDFromSeed creates a deterministic ClusterID from a seed string.
func NewClusterIDFromSeed(seed string) ClusterID {
	hash := sha256.Sum256([]byte("cluster:" + seed))
	return ClusterID(hex.EncodeToString(hash[:]))
}

// String returns the string representation of ClusterID.
func (id ClusterID) String() string {
	return string(id)
}

// ClusterStatus represents the lifecycle state of a cluster.
type ClusterStatus string

const (
	// ClusterStatusPending indicates the cluster is being formed.
	ClusterStatusPending ClusterStatus = "pending"
	// ClusterStatusActive indicates the cluster is active and validated.
	ClusterStatusActive ClusterStatus = "active"
	// ClusterStatusMerged indicates the cluster was merged into another.
	ClusterStatusMerged ClusterStatus = "merged"
	// ClusterStatusDisbanded indicates the cluster was disbanded.
	ClusterStatusDisbanded ClusterStatus = "disbanded"
)

// String returns the string representation of ClusterStatus.
func (s ClusterStatus) String() string {
	return string(s)
}

// IsValid checks if the status is valid.
func (s ClusterStatus) IsValid() bool {
	switch s {
	case ClusterStatusPending, ClusterStatusActive, ClusterStatusMerged, ClusterStatusDisbanded:
		return true
	default:
		return false
	}
}

// DocumentCluster represents a group of semantically related documents.
// Clusters are formed through graph-based community detection and LLM validation.
type DocumentCluster struct {
	ID           ClusterID
	WorkspaceID  WorkspaceID
	Name         string        // AI-generated cluster name
	Summary      string        // Community summary (GraphRAG style)
	Status       ClusterStatus // Lifecycle state
	Confidence   float64       // Cluster cohesion score (0-1)
	MemberCount  int           // Number of documents in cluster
	CentralNodes []DocumentID  // Representative documents (centroids)
	TopEntities  []string      // Shared entities across cluster
	TopKeywords  []string      // Shared keywords
	MergedInto   *ClusterID    // If merged, target cluster ID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewDocumentCluster creates a new document cluster.
func NewDocumentCluster(workspaceID WorkspaceID, name string) *DocumentCluster {
	now := time.Now()
	return &DocumentCluster{
		ID:           NewClusterID(),
		WorkspaceID:  workspaceID,
		Name:         name,
		Status:       ClusterStatusPending,
		Confidence:   0.0,
		MemberCount:  0,
		CentralNodes: []DocumentID{},
		TopEntities:  []string{},
		TopKeywords:  []string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Activate marks the cluster as active after validation.
func (c *DocumentCluster) Activate() {
	c.Status = ClusterStatusActive
	c.UpdatedAt = time.Now()
}

// MergeInto marks this cluster as merged into another.
func (c *DocumentCluster) MergeInto(targetID ClusterID) {
	c.Status = ClusterStatusMerged
	c.MergedInto = &targetID
	c.UpdatedAt = time.Now()
}

// Disband marks the cluster as disbanded.
func (c *DocumentCluster) Disband() {
	c.Status = ClusterStatusDisbanded
	c.UpdatedAt = time.Now()
}

// ClusterMembership represents the membership of a document in a cluster.
type ClusterMembership struct {
	ClusterID   ClusterID
	DocumentID  DocumentID
	WorkspaceID WorkspaceID
	Score       float64   // Membership strength (0-1)
	IsCentral   bool      // Whether this document is a cluster centroid
	JoinedAt    time.Time
}

// NewClusterMembership creates a new cluster membership.
func NewClusterMembership(clusterID ClusterID, documentID DocumentID, workspaceID WorkspaceID, score float64) *ClusterMembership {
	return &ClusterMembership{
		ClusterID:   clusterID,
		DocumentID:  documentID,
		WorkspaceID: workspaceID,
		Score:       score,
		IsCentral:   false,
		JoinedAt:    time.Now(),
	}
}

// EdgeSourceType represents the type of connection between documents.
type EdgeSourceType string

const (
	// EdgeSourceSemantic indicates connection via embedding similarity.
	EdgeSourceSemantic EdgeSourceType = "semantic"
	// EdgeSourceEntity indicates connection via shared entities.
	EdgeSourceEntity EdgeSourceType = "entity"
	// EdgeSourceTemporal indicates connection via co-occurrence in time.
	EdgeSourceTemporal EdgeSourceType = "temporal"
	// EdgeSourceStructural indicates connection via folder/path proximity.
	EdgeSourceStructural EdgeSourceType = "structural"
	// EdgeSourceReference indicates connection via explicit references.
	EdgeSourceReference EdgeSourceType = "reference"
)

// String returns the string representation of EdgeSourceType.
func (t EdgeSourceType) String() string {
	return string(t)
}

// EdgeSource represents a contributing factor to a document edge weight.
type EdgeSource struct {
	Type   EdgeSourceType `json:"type"`
	Weight float64        `json:"weight"` // Contribution to total weight (0-1)
	Detail string         `json:"detail"` // Additional context (e.g., entity name, time window)
}

// DocumentEdge represents a weighted connection between two documents in the graph.
type DocumentEdge struct {
	FromDoc     DocumentID
	ToDoc       DocumentID
	WorkspaceID WorkspaceID
	Weight      float64      // Combined similarity score (0-1)
	Sources     []EdgeSource // Contributing factors
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewDocumentEdge creates a new document edge.
func NewDocumentEdge(fromDoc, toDoc DocumentID, workspaceID WorkspaceID) *DocumentEdge {
	now := time.Now()
	return &DocumentEdge{
		FromDoc:     fromDoc,
		ToDoc:       toDoc,
		WorkspaceID: workspaceID,
		Weight:      0.0,
		Sources:     []EdgeSource{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddSource adds a source contribution to the edge.
func (e *DocumentEdge) AddSource(source EdgeSource) {
	e.Sources = append(e.Sources, source)
	e.recalculateWeight()
	e.UpdatedAt = time.Now()
}

// recalculateWeight updates the total weight based on sources.
func (e *DocumentEdge) recalculateWeight() {
	total := 0.0
	for _, src := range e.Sources {
		total += src.Weight
	}
	// Normalize to [0, 1]
	if total > 1.0 {
		total = 1.0
	}
	e.Weight = total
}

// HasSource checks if the edge has a source of the given type.
func (e *DocumentEdge) HasSource(sourceType EdgeSourceType) bool {
	for _, src := range e.Sources {
		if src.Type == sourceType {
			return true
		}
	}
	return false
}

// GetSourceWeight returns the weight contribution from a specific source type.
func (e *DocumentEdge) GetSourceWeight(sourceType EdgeSourceType) float64 {
	for _, src := range e.Sources {
		if src.Type == sourceType {
			return src.Weight
		}
	}
	return 0.0
}

// DocumentGraph represents a graph of documents connected by weighted edges.
// Used for community detection and clustering.
type DocumentGraph struct {
	WorkspaceID WorkspaceID
	Nodes       []DocumentID
	Edges       map[string]*DocumentEdge // key: "fromDoc|toDoc"
	NodeIndex   map[DocumentID]int       // Maps DocumentID to node index
}

// NewDocumentGraph creates a new document graph.
func NewDocumentGraph(workspaceID WorkspaceID) *DocumentGraph {
	return &DocumentGraph{
		WorkspaceID: workspaceID,
		Nodes:       []DocumentID{},
		Edges:       make(map[string]*DocumentEdge),
		NodeIndex:   make(map[DocumentID]int),
	}
}

// AddNode adds a document node to the graph.
func (g *DocumentGraph) AddNode(docID DocumentID) {
	if _, exists := g.NodeIndex[docID]; !exists {
		g.NodeIndex[docID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, docID)
	}
}

// AddEdge adds or updates an edge between two documents.
func (g *DocumentGraph) AddEdge(edge *DocumentEdge) {
	g.AddNode(edge.FromDoc)
	g.AddNode(edge.ToDoc)

	key := edgeKey(edge.FromDoc, edge.ToDoc)
	if existing, exists := g.Edges[key]; exists {
		// Merge sources
		for _, src := range edge.Sources {
			existing.AddSource(src)
		}
	} else {
		g.Edges[key] = edge
	}
}

// GetEdge returns the edge between two documents if it exists.
func (g *DocumentGraph) GetEdge(fromDoc, toDoc DocumentID) *DocumentEdge {
	key := edgeKey(fromDoc, toDoc)
	if edge, exists := g.Edges[key]; exists {
		return edge
	}
	// Try reverse direction (undirected graph)
	key = edgeKey(toDoc, fromDoc)
	return g.Edges[key]
}

// GetNeighbors returns all documents connected to the given document.
func (g *DocumentGraph) GetNeighbors(docID DocumentID) []DocumentID {
	neighbors := []DocumentID{}
	for _, edge := range g.Edges {
		if edge.FromDoc == docID {
			neighbors = append(neighbors, edge.ToDoc)
		} else if edge.ToDoc == docID {
			neighbors = append(neighbors, edge.FromDoc)
		}
	}
	return neighbors
}

// GetNeighborsWithWeight returns neighbors with their edge weights.
func (g *DocumentGraph) GetNeighborsWithWeight(docID DocumentID) map[DocumentID]float64 {
	neighbors := make(map[DocumentID]float64)
	for _, edge := range g.Edges {
		if edge.FromDoc == docID {
			neighbors[edge.ToDoc] = edge.Weight
		} else if edge.ToDoc == docID {
			neighbors[edge.FromDoc] = edge.Weight
		}
	}
	return neighbors
}

// NodeCount returns the number of nodes in the graph.
func (g *DocumentGraph) NodeCount() int {
	return len(g.Nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *DocumentGraph) EdgeCount() int {
	return len(g.Edges)
}

// TotalWeight returns the sum of all edge weights.
func (g *DocumentGraph) TotalWeight() float64 {
	total := 0.0
	for _, edge := range g.Edges {
		total += edge.Weight
	}
	return total
}

// edgeKey creates a canonical key for an edge (order-independent for undirected graph).
func edgeKey(from, to DocumentID) string {
	if string(from) < string(to) {
		return string(from) + "|" + string(to)
	}
	return string(to) + "|" + string(from)
}
