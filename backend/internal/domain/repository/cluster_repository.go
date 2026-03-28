package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ClusterMemberInfo contains document membership info with file details for display.
type ClusterMemberInfo struct {
	DocumentID      entity.DocumentID
	RelativePath    string
	Filename        string
	MembershipScore float64
	IsCentral       bool
}

// ClusterRepository defines storage for document clusters and graph edges.
type ClusterRepository interface {
	// Cluster operations
	UpsertCluster(ctx context.Context, cluster *entity.DocumentCluster) error
	GetCluster(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID) (*entity.DocumentCluster, error)
	GetClustersByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error)
	GetActiveClustersByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error)
	DeleteCluster(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID) error
	UpdateClusterStatus(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID, status entity.ClusterStatus) error

	// Membership operations
	AddMembership(ctx context.Context, membership *entity.ClusterMembership) error
	RemoveMembership(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID) error
	GetMembershipsByCluster(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]*entity.ClusterMembership, error)
	GetMembershipsByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.ClusterMembership, error)
	GetClusterMembers(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]entity.DocumentID, error)
	GetClusterMembersWithInfo(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]*ClusterMemberInfo, error)
	UpdateMembershipScore(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, score float64) error
	SetCentralNode(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, isCentral bool) error

	// Edge operations (for graph persistence)
	UpsertEdge(ctx context.Context, edge *entity.DocumentEdge) error
	GetEdge(ctx context.Context, workspaceID entity.WorkspaceID, fromDoc, toDoc entity.DocumentID) (*entity.DocumentEdge, error)
	GetEdgesByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.DocumentEdge, error)
	GetAllEdges(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentEdge, error)
	GetEdgesAboveThreshold(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.DocumentEdge, error)
	DeleteEdge(ctx context.Context, workspaceID entity.WorkspaceID, fromDoc, toDoc entity.DocumentID) error
	DeleteEdgesByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) error

	// Graph loading
	LoadGraph(ctx context.Context, workspaceID entity.WorkspaceID, minEdgeWeight float64) (*entity.DocumentGraph, error)

	// Batch operations
	UpsertEdgesBatch(ctx context.Context, edges []*entity.DocumentEdge) error
	ClearClusterMemberships(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) error
	ClearAllClusters(ctx context.Context, workspaceID entity.WorkspaceID) error

	// Statistics
	GetClusterStats(ctx context.Context, workspaceID entity.WorkspaceID) (*ClusterStats, error)

	// Facet support
	GetClusterFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)
}

// ClusterStats contains aggregate statistics about clusters.
type ClusterStats struct {
	TotalClusters     int
	ActiveClusters    int
	TotalMemberships  int
	TotalEdges        int
	AvgClusterSize    float64
	AvgEdgeWeight     float64
	MaxClusterSize    int
	MinClusterSize    int
	IsolatedDocuments int // Documents not in any cluster
}
