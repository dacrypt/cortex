// Package grpc provides gRPC handlers for the Cortex API.
package grpc

import (
	"context"

	"github.com/rs/zerolog"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/taxonomy"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TaxonomyHandler implements the TaxonomyService gRPC interface.
type TaxonomyHandler struct {
	cortexv1.UnimplementedTaxonomyServiceServer
	taxonomyService *taxonomy.Service
	logger          zerolog.Logger
}

// NewTaxonomyHandler creates a new Taxonomy gRPC handler.
func NewTaxonomyHandler(taxonomyService *taxonomy.Service, logger zerolog.Logger) *TaxonomyHandler {
	return &TaxonomyHandler{
		taxonomyService: taxonomyService,
		logger:          logger.With().Str("handler", "taxonomy").Logger(),
	}
}

// GetRootNodes returns top-level taxonomy categories.
// If no categories exist, it automatically creates default root categories.
func (h *TaxonomyHandler) GetRootNodes(req *cortexv1.GetRootNodesRequest, stream cortexv1.TaxonomyService_GetRootNodesServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetRootNodes request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	nodes, err := h.taxonomyService.GetRootNodes(ctx, workspaceID)
	if err != nil {
		return err
	}

	// Auto-initialize default categories if none exist
	if len(nodes) == 0 {
		h.logger.Info().
			Str("workspace_id", req.GetWorkspaceId()).
			Msg("No taxonomy categories found, creating default root categories")

		defaultCategories := []string{"Documents", "Code", "Media", "Data", "Other"}
		if err := h.taxonomyService.EnsureRootCategories(ctx, workspaceID, defaultCategories); err != nil {
			h.logger.Warn().Err(err).Msg("Failed to create default taxonomy categories")
		} else {
			// Fetch the newly created nodes
			nodes, err = h.taxonomyService.GetRootNodes(ctx, workspaceID)
			if err != nil {
				return err
			}
		}
	}

	for _, n := range nodes {
		if err := stream.Send(nodeToProto(n)); err != nil {
			return err
		}
	}

	return nil
}

// GetNode returns a specific taxonomy node.
func (h *TaxonomyHandler) GetNode(ctx context.Context, req *cortexv1.GetNodeRequest) (*cortexv1.TaxonomyNode, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("node_id", req.GetNodeId()).
		Msg("GetNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	node, err := h.taxonomyService.GetNode(ctx, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}

	return nodeToProto(node), nil
}

// GetNodeByPath returns a node by its path.
func (h *TaxonomyHandler) GetNodeByPath(ctx context.Context, req *cortexv1.GetNodeByPathRequest) (*cortexv1.TaxonomyNode, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("path", req.GetPath()).
		Msg("GetNodeByPath request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	node, err := h.taxonomyService.GetNodeByPath(ctx, workspaceID, req.GetPath())
	if err != nil {
		return nil, err
	}

	return nodeToProto(node), nil
}

// GetChildren returns child nodes of a parent.
func (h *TaxonomyHandler) GetChildren(req *cortexv1.GetChildrenRequest, stream cortexv1.TaxonomyService_GetChildrenServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("parent_id", req.GetParentId()).
		Msg("GetChildren request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	parentID := entity.TaxonomyNodeID(req.GetParentId())

	children, err := h.taxonomyService.GetChildren(ctx, workspaceID, parentID)
	if err != nil {
		return err
	}

	for _, n := range children {
		if err := stream.Send(nodeToProto(n)); err != nil {
			return err
		}
	}

	return nil
}

// GetAncestors returns ancestor nodes up to root.
func (h *TaxonomyHandler) GetAncestors(req *cortexv1.GetAncestorsRequest, stream cortexv1.TaxonomyService_GetAncestorsServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("node_id", req.GetNodeId()).
		Msg("GetAncestors request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	ancestors, err := h.taxonomyService.GetAncestors(ctx, workspaceID, nodeID)
	if err != nil {
		return err
	}

	for _, n := range ancestors {
		if err := stream.Send(nodeToProto(n)); err != nil {
			return err
		}
	}

	return nil
}

// GetNodeFiles returns files assigned to a node.
func (h *TaxonomyHandler) GetNodeFiles(req *cortexv1.GetNodeFilesRequest, stream cortexv1.TaxonomyService_GetNodeFilesServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("node_id", req.GetNodeId()).
		Bool("include_descendants", req.GetIncludeDescendants()).
		Msg("GetNodeFiles request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	fileIDs, err := h.taxonomyService.GetNodeFiles(ctx, workspaceID, nodeID, req.GetIncludeDescendants())
	if err != nil {
		return err
	}

	for _, fileID := range fileIDs {
		if err := stream.Send(&cortexv1.TaxonomyFileEntry{
			FileId:       fileID.String(),
			RelativePath: "", // Would need file lookup
			Filename:     "", // Would need file lookup
			Score:        1.0,
			Source:       cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_UNKNOWN,
			AssignedAt:   0,
		}); err != nil {
			return err
		}
	}

	return nil
}

// CreateNode creates a new taxonomy category.
func (h *TaxonomyHandler) CreateNode(ctx context.Context, req *cortexv1.CreateNodeRequest) (*cortexv1.TaxonomyNode, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("name", req.GetName()).
		Str("parent_id", req.GetParentId()).
		Msg("CreateNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	source := protoToNodeSource(req.GetSource())

	var parentID *entity.TaxonomyNodeID
	if req.GetParentId() != "" {
		pid := entity.TaxonomyNodeID(req.GetParentId())
		parentID = &pid
	}

	node, err := h.taxonomyService.CreateNode(ctx, workspaceID, req.GetName(), parentID, source)
	if err != nil {
		return nil, err
	}

	return nodeToProto(node), nil
}

// UpdateNode updates an existing node.
func (h *TaxonomyHandler) UpdateNode(ctx context.Context, req *cortexv1.UpdateNodeRequest) (*cortexv1.TaxonomyNode, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("node_id", req.GetNodeId()).
		Str("name", req.GetName()).
		Msg("UpdateNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	// Get existing node
	node, err := h.taxonomyService.GetNode(ctx, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.GetName() != "" {
		node.Name = req.GetName()
	}
	if req.GetDescription() != "" {
		node.Description = req.GetDescription()
	}
	node.Keywords = req.GetKeywords()

	err = h.taxonomyService.UpdateNode(ctx, workspaceID, node)
	if err != nil {
		return nil, err
	}

	return nodeToProto(node), nil
}

// DeleteNode removes a taxonomy node.
func (h *TaxonomyHandler) DeleteNode(ctx context.Context, req *cortexv1.DeleteNodeRequest) (*cortexv1.DeleteNodeResponse, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("node_id", req.GetNodeId()).
		Bool("delete_children", req.GetDeleteChildren()).
		Msg("DeleteNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	err := h.taxonomyService.DeleteNode(ctx, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.DeleteNodeResponse{
		Success:      true,
		NodesDeleted: 1,
		Message:      "Node deleted successfully",
	}, nil
}

// AddFileToNode assigns a file to a taxonomy category.
func (h *TaxonomyHandler) AddFileToNode(ctx context.Context, req *cortexv1.AddFileToNodeRequest) (*cortexv1.AddFileToNodeResponse, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("file_id", req.GetFileId()).
		Str("node_id", req.GetNodeId()).
		Msg("AddFileToNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	fileID := entity.FileID(req.GetFileId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())
	source := protoToMappingSource(req.GetSource())

	err := h.taxonomyService.AddFileToNode(ctx, workspaceID, fileID, nodeID, source)
	if err != nil {
		return nil, err
	}

	return &cortexv1.AddFileToNodeResponse{
		Success: true,
		Message: "File assigned to category",
	}, nil
}

// RemoveFileFromNode removes a file from a category.
func (h *TaxonomyHandler) RemoveFileFromNode(ctx context.Context, req *cortexv1.RemoveFileFromNodeRequest) (*cortexv1.RemoveFileFromNodeResponse, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("file_id", req.GetFileId()).
		Str("node_id", req.GetNodeId()).
		Msg("RemoveFileFromNode request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	fileID := entity.FileID(req.GetFileId())
	nodeID := entity.TaxonomyNodeID(req.GetNodeId())

	err := h.taxonomyService.RemoveFileFromNode(ctx, workspaceID, fileID, nodeID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.RemoveFileFromNodeResponse{
		Success: true,
		Message: "File removed from category",
	}, nil
}

// GetFileTaxonomies returns all taxonomy nodes for a file.
func (h *TaxonomyHandler) GetFileTaxonomies(req *cortexv1.GetFileTaxonomiesRequest, stream cortexv1.TaxonomyService_GetFileTaxonomiesServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("file_id", req.GetFileId()).
		Msg("GetFileTaxonomies request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	fileID := entity.FileID(req.GetFileId())

	nodes, err := h.taxonomyService.GetFileTaxonomies(ctx, workspaceID, fileID)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		if err := stream.Send(nodeToProto(n)); err != nil {
			return err
		}
	}

	return nil
}

// SearchNodes searches for taxonomy nodes by name.
func (h *TaxonomyHandler) SearchNodes(req *cortexv1.SearchNodesRequest, stream cortexv1.TaxonomyService_SearchNodesServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("query", req.GetQuery()).
		Msg("SearchNodes request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100 // Default limit
	}
	nodes, err := h.taxonomyService.SearchNodes(ctx, workspaceID, req.GetQuery(), limit)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		if err := stream.Send(nodeToProto(n)); err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns taxonomy statistics.
func (h *TaxonomyHandler) GetStats(ctx context.Context, req *cortexv1.GetTaxonomyStatsRequest) (*cortexv1.TaxonomyStats, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetStats request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	stats, err := h.taxonomyService.GetStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.TaxonomyStats{
		TotalNodes:           int32(stats.TotalNodes),
		RootNodes:            int32(stats.RootNodes),
		MaxDepth:             int32(stats.MaxDepth),
		FilesCategorized:     int32(stats.TotalMappings),
		FilesUncategorized:   int32(stats.OrphanedNodes),
		AvgFilesPerNode:      float64(stats.TotalMappings) / float64(max(stats.TotalNodes, 1)),
		AvgCategoriesPerFile: stats.AverageConfidence, // Using confidence as proxy
		LastInductionAt:      0,                       // Not tracked
	}, nil
}

// SuggestTaxonomy suggests categories for a file.
func (h *TaxonomyHandler) SuggestTaxonomy(req *cortexv1.SuggestTaxonomyRequest, stream cortexv1.TaxonomyService_SuggestTaxonomyServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("file_id", req.GetFileId()).
		Msg("SuggestTaxonomy request")

	// TODO: Implement taxonomy suggestion using LLM
	// For now, return empty suggestions

	return nil
}

// InduceTaxonomy triggers LLM-based taxonomy generation.
func (h *TaxonomyHandler) InduceTaxonomy(ctx context.Context, req *cortexv1.InduceTaxonomyRequest) (*cortexv1.TaxonomyInductionResult, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Int32("max_levels", req.GetMaxLevels()).
		Msg("InduceTaxonomy request")

	// TODO: Implement taxonomy induction using LLM
	// For now, just ensure root categories exist

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Use default root categories for now
	defaultCategories := []string{"Documents", "Code", "Media", "Data", "Other"}
	err := h.taxonomyService.EnsureRootCategories(ctx, workspaceID, defaultCategories)
	if err != nil {
		return nil, err
	}

	return &cortexv1.TaxonomyInductionResult{
		Success: true,
		Message: "Root categories initialized",
	}, nil
}

// nodeToProto converts a domain node to proto.
func nodeToProto(n *entity.TaxonomyNode) *cortexv1.TaxonomyNode {
	parentID := ""
	if n.ParentID != nil {
		parentID = n.ParentID.String()
	}

	return &cortexv1.TaxonomyNode{
		Id:            n.ID.String(),
		WorkspaceId:   n.WorkspaceID.String(),
		Name:          n.Name,
		Description:   n.Description,
		ParentId:      parentID,
		Path:          n.Path,
		Level:         int32(n.Level),
		Source:        nodeSourceToProto(n.Source),
		Confidence:    n.Confidence,
		Keywords:      n.Keywords,
		ChildCount:    int32(n.ChildCount),
		DocCount:      int32(n.DocCount),
		TotalDocCount: int32(n.DocCount), // Using DocCount for TotalDocCount
		CreatedAt:     n.CreatedAt.UnixMilli(),
		UpdatedAt:     n.UpdatedAt.UnixMilli(),
	}
}

// nodeSourceToProto converts node source to proto enum.
func nodeSourceToProto(source entity.TaxonomyNodeSource) cortexv1.TaxonomyNodeSource {
	switch source {
	case entity.TaxonomyNodeSourceUser:
		return cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_USER
	case entity.TaxonomyNodeSourceInferred:
		return cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_AI
	case entity.TaxonomyNodeSourceSystem:
		return cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_SYSTEM
	case entity.TaxonomyNodeSourceMerged:
		return cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_MERGED
	default:
		return cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_UNKNOWN
	}
}

// protoToNodeSource converts proto enum to node source.
func protoToNodeSource(source cortexv1.TaxonomyNodeSource) entity.TaxonomyNodeSource {
	switch source {
	case cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_USER:
		return entity.TaxonomyNodeSourceUser
	case cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_AI:
		return entity.TaxonomyNodeSourceInferred
	case cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_SYSTEM:
		return entity.TaxonomyNodeSourceSystem
	case cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_IMPORT:
		return entity.TaxonomyNodeSourceUser // Map import to user
	case cortexv1.TaxonomyNodeSource_TAXONOMY_SOURCE_MERGED:
		return entity.TaxonomyNodeSourceMerged
	default:
		return entity.TaxonomyNodeSourceUser
	}
}

// mappingSourceToProto converts mapping source to proto enum.
// Note: TaxonomyNodeSource is used for both node and mapping sources.
func mappingSourceToProto(source entity.TaxonomyNodeSource) cortexv1.TaxonomyMappingSource {
	switch source {
	case entity.TaxonomyNodeSourceUser:
		return cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_MANUAL
	case entity.TaxonomyNodeSourceInferred:
		return cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_AUTO
	case entity.TaxonomyNodeSourceSystem:
		return cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_SUGGESTED
	default:
		return cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_UNKNOWN
	}
}

// protoToMappingSource converts proto enum to mapping source.
func protoToMappingSource(source cortexv1.TaxonomyMappingSource) entity.TaxonomyNodeSource {
	switch source {
	case cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_MANUAL:
		return entity.TaxonomyNodeSourceUser
	case cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_AUTO:
		return entity.TaxonomyNodeSourceInferred
	case cortexv1.TaxonomyMappingSource_MAPPING_SOURCE_SUGGESTED:
		return entity.TaxonomyNodeSourceSystem
	default:
		return entity.TaxonomyNodeSourceUser
	}
}
