// Package grpc provides gRPC handlers for the Cortex API.
package grpc

import (
	"context"

	"github.com/rs/zerolog"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/clustering"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ClusteringHandler implements the ClusteringService gRPC interface.
type ClusteringHandler struct {
	cortexv1.UnimplementedClusteringServiceServer
	clusteringService *clustering.Service
	logger            zerolog.Logger
}

// NewClusteringHandler creates a new Clustering gRPC handler.
func NewClusteringHandler(clusteringService *clustering.Service, logger zerolog.Logger) *ClusteringHandler {
	return &ClusteringHandler{
		clusteringService: clusteringService,
		logger:            logger.With().Str("handler", "clustering").Logger(),
	}
}

// GetClusters returns all clusters for a workspace.
func (h *ClusteringHandler) GetClusters(req *cortexv1.GetClustersRequest, stream cortexv1.ClusteringService_GetClustersServer) error {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("[CLUSTER_HANDLER] GetClusters request received")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	clusters, err := h.clusteringService.GetClusters(ctx, workspaceID)
	if err != nil {
		h.logger.Error().Err(err).Str("workspace_id", req.GetWorkspaceId()).Msg("[CLUSTER_HANDLER] Error getting clusters")
		return err
	}

	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Int("cluster_count", len(clusters)).
		Msg("[CLUSTER_HANDLER] Returning clusters")

	for _, c := range clusters {
		h.logger.Info().
			Str("cluster_id", c.ID.String()).
			Str("cluster_name", c.Name).
			Str("cluster_status", string(c.Status)).
			Int("member_count", c.MemberCount).
			Float64("confidence", c.Confidence).
			Msg("[CLUSTER_HANDLER] Sending cluster details")
		if err := stream.Send(clusterToProto(c)); err != nil {
			return err
		}
	}

	return nil
}

// GetCluster returns a specific cluster by ID.
func (h *ClusteringHandler) GetCluster(ctx context.Context, req *cortexv1.GetClusterRequest) (*cortexv1.DocumentCluster, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("cluster_id", req.GetClusterId()).
		Msg("GetCluster request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	clusterID := entity.ClusterID(req.GetClusterId())

	// Get all clusters and find the matching one
	clusters, err := h.clusteringService.GetClusters(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	for _, c := range clusters {
		if c.ID == clusterID {
			return clusterToProto(c), nil
		}
	}

	return nil, nil
}

// GetClusterMembers returns all members of a cluster.
func (h *ClusteringHandler) GetClusterMembers(req *cortexv1.GetClusterMembersRequest, stream cortexv1.ClusteringService_GetClusterMembersServer) error {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("cluster_id", req.GetClusterId()).
		Msg("[CLUSTER_HANDLER] GetClusterMembers request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	clusterID := entity.ClusterID(req.GetClusterId())

	// GetClusterMembersWithInfo returns members with file information
	members, err := h.clusteringService.GetClusterMembersWithInfo(ctx, workspaceID, clusterID)
	if err != nil {
		h.logger.Error().Err(err).Msg("[CLUSTER_HANDLER] Error getting cluster members")
		return err
	}

	h.logger.Info().
		Int("member_count", len(members)).
		Msg("[CLUSTER_HANDLER] Returning cluster members")

	for _, member := range members {
		h.logger.Debug().
			Str("document_id", member.DocumentID.String()).
			Str("relative_path", member.RelativePath).
			Str("filename", member.Filename).
			Float64("score", member.MembershipScore).
			Bool("is_central", member.IsCentral).
			Msg("[CLUSTER_HANDLER] Sending cluster member")

		if err := stream.Send(&cortexv1.ClusterMember{
			DocumentId:      member.DocumentID.String(),
			RelativePath:    member.RelativePath,
			Filename:        member.Filename,
			MembershipScore: member.MembershipScore,
			IsCentral:       member.IsCentral,
			AddedAt:         0,
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetDocumentGraph returns the document graph for visualization.
func (h *ClusteringHandler) GetDocumentGraph(ctx context.Context, req *cortexv1.GetDocumentGraphRequest) (*cortexv1.DocumentGraphData, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Float64("min_edge_weight", req.GetMinEdgeWeight()).
		Msg("GetDocumentGraph request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	graph, err := h.clusteringService.LoadGraph(ctx, workspaceID, req.GetMinEdgeWeight())
	if err != nil {
		return nil, err
	}

	if graph == nil {
		return &cortexv1.DocumentGraphData{
			Nodes:      []*cortexv1.ClusterNode{},
			Edges:      []*cortexv1.ClusterEdge{},
			TotalNodes: 0,
			TotalEdges: 0,
		}, nil
	}

	// Convert graph nodes to proto
	nodes := make([]*cortexv1.ClusterNode, 0, len(graph.Nodes))
	for _, nodeID := range graph.Nodes {
		nodes = append(nodes, &cortexv1.ClusterNode{
			Id:       string(nodeID),
			Label:    string(nodeID), // Would need file lookup for better label
			NodeType: "document",
		})
	}

	// Convert graph edges to proto
	edges := make([]*cortexv1.ClusterEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		if edge.Weight >= req.GetMinEdgeWeight() {
			edgeType := "semantic"
			detail := ""
			if len(edge.Sources) > 0 {
				edgeType = string(edge.Sources[0].Type)
				detail = edge.Sources[0].Detail
			}
			edges = append(edges, &cortexv1.ClusterEdge{
				FromId:   string(edge.FromDoc),
				ToId:     string(edge.ToDoc),
				EdgeType: edgeType,
				Weight:   edge.Weight,
				Detail:   detail,
			})
		}
	}

	return &cortexv1.DocumentGraphData{
		Nodes:      nodes,
		Edges:      edges,
		TotalNodes: int32(len(nodes)),
		TotalEdges: int32(len(edges)),
	}, nil
}

// GetDocumentClusters returns all clusters a document belongs to.
func (h *ClusteringHandler) GetDocumentClusters(req *cortexv1.GetDocumentClustersRequest, stream cortexv1.ClusteringService_GetDocumentClustersServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("document_id", req.GetDocumentId()).
		Msg("GetDocumentClusters request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	documentID := entity.DocumentID(req.GetDocumentId())

	// GetDocumentClusters returns []*entity.DocumentCluster
	clusters, err := h.clusteringService.GetDocumentClusters(ctx, workspaceID, documentID)
	if err != nil {
		return err
	}

	for _, c := range clusters {
		if err := stream.Send(&cortexv1.ClusterMembership{
			ClusterId:   c.ID.String(),
			ClusterName: c.Name,
			Score:       c.Confidence,
			IsCentral:   false, // Would need membership lookup
		}); err != nil {
			return err
		}
	}

	return nil
}

// RunClustering triggers clustering analysis for a workspace.
func (h *ClusteringHandler) RunClustering(ctx context.Context, req *cortexv1.RunClusteringRequest) (*cortexv1.ClusteringResult, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Bool("force_rebuild", req.GetForceRebuild()).
		Msg("RunClustering request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	forceRebuild := req.GetForceRebuild()

	result, err := h.clusteringService.RunClustering(ctx, workspaceID, forceRebuild)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ClusteringResult{
		Success:           true,
		Message:           "Clustering completed",
		ClustersCreated:   int32(len(result.Clusters)),
		ClustersUpdated:   0,
		DocumentsAssigned: int32(result.AssignmentsMade),
		ProjectsCreated:   int32(result.ProjectsCreated),
		Errors:            result.ValidationErrors,
		DurationMs:        result.Duration.Milliseconds(),
	}, nil
}

// MergeClusters combines two clusters into one.
func (h *ClusteringHandler) MergeClusters(ctx context.Context, req *cortexv1.MergeClustersRequest) (*cortexv1.ClusteringResult, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("target", req.GetTargetClusterId()).
		Str("source", req.GetSourceClusterId()).
		Msg("MergeClusters request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	targetID := entity.ClusterID(req.GetTargetClusterId())
	sourceID := entity.ClusterID(req.GetSourceClusterId())

	err := h.clusteringService.MergeClusters(ctx, workspaceID, targetID, sourceID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ClusteringResult{
		Success:         true,
		Message:         "Clusters merged successfully",
		ClustersUpdated: 1,
	}, nil
}

// DisbandCluster removes a cluster.
func (h *ClusteringHandler) DisbandCluster(ctx context.Context, req *cortexv1.DisbandClusterRequest) (*cortexv1.ClusteringResult, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("cluster_id", req.GetClusterId()).
		Msg("DisbandCluster request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	clusterID := entity.ClusterID(req.GetClusterId())

	err := h.clusteringService.DisbandCluster(ctx, workspaceID, clusterID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ClusteringResult{
		Success: true,
		Message: "Cluster disbanded",
	}, nil
}

// GetClusterStats returns clustering statistics for a workspace.
func (h *ClusteringHandler) GetClusterStats(ctx context.Context, req *cortexv1.GetClusterStatsRequest) (*cortexv1.ClusterStats, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetClusterStats request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	stats, err := h.clusteringService.GetClusterStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ClusterStats{
		TotalClusters:        int32(stats.TotalClusters),
		ActiveClusters:       int32(stats.ActiveClusters),
		DocumentsClustered:   int32(stats.TotalMemberships),
		DocumentsUnclustered: int32(stats.IsolatedDocuments),
		AvgClusterSize:       stats.AvgClusterSize,
		AvgClusterConfidence: stats.AvgEdgeWeight, // Using edge weight as proxy for confidence
		LastClusteringAt:     0,                   // Not tracked in repository
	}, nil
}

// clusterToProto converts a domain cluster to proto.
func clusterToProto(c *entity.DocumentCluster) *cortexv1.DocumentCluster {
	centralNodes := make([]string, 0, len(c.CentralNodes))
	for _, n := range c.CentralNodes {
		centralNodes = append(centralNodes, n.String())
	}

	return &cortexv1.DocumentCluster{
		Id:           c.ID.String(),
		WorkspaceId:  c.WorkspaceID.String(),
		Name:         c.Name,
		Summary:      c.Summary,
		Status:       clusterStatusToProto(c.Status),
		Confidence:   c.Confidence,
		MemberCount:  int32(c.MemberCount),
		CentralNodes: centralNodes,
		TopEntities:  c.TopEntities,
		TopKeywords:  c.TopKeywords,
		CreatedAt:    c.CreatedAt.UnixMilli(),
		UpdatedAt:    c.UpdatedAt.UnixMilli(),
	}
}

// clusterStatusToProto converts cluster status to proto enum.
func clusterStatusToProto(status entity.ClusterStatus) cortexv1.ClusterStatus {
	switch status {
	case entity.ClusterStatusPending:
		return cortexv1.ClusterStatus_CLUSTER_STATUS_PENDING
	case entity.ClusterStatusActive:
		return cortexv1.ClusterStatus_CLUSTER_STATUS_ACTIVE
	case entity.ClusterStatusMerged:
		return cortexv1.ClusterStatus_CLUSTER_STATUS_MERGED
	case entity.ClusterStatusDisbanded:
		return cortexv1.ClusterStatus_CLUSTER_STATUS_DISBANDED
	default:
		return cortexv1.ClusterStatus_CLUSTER_STATUS_UNKNOWN
	}
}
