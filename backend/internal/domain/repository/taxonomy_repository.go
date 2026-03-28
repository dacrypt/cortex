// Package repository defines repository interfaces for the domain layer.
package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TaxonomyRepository defines storage for taxonomy nodes and file mappings.
type TaxonomyRepository interface {
	// Node CRUD operations
	CreateNode(ctx context.Context, workspaceID entity.WorkspaceID, node *entity.TaxonomyNode) error
	GetNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) (*entity.TaxonomyNode, error)
	GetNodeByPath(ctx context.Context, workspaceID entity.WorkspaceID, path string) (*entity.TaxonomyNode, error)
	GetNodeByName(ctx context.Context, workspaceID entity.WorkspaceID, name string, parentID *entity.TaxonomyNodeID) (*entity.TaxonomyNode, error)
	UpdateNode(ctx context.Context, workspaceID entity.WorkspaceID, node *entity.TaxonomyNode) error
	DeleteNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) error

	// Hierarchy operations
	GetRootNodes(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error)
	GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error)
	GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error)
	GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error)
	ListAll(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error)

	// File mapping operations
	AddFileMapping(ctx context.Context, mapping *entity.FileTaxonomyMapping) error
	RemoveFileMapping(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, nodeID entity.TaxonomyNodeID) error
	GetFileMappings(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.FileTaxonomyMapping, error)
	GetNodeFiles(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID, includeDescendants bool) ([]entity.FileID, error)
	UpdateFileMapping(ctx context.Context, mapping *entity.FileTaxonomyMapping) error

	// Bulk operations
	BulkCreateNodes(ctx context.Context, workspaceID entity.WorkspaceID, nodes []*entity.TaxonomyNode) error
	BulkAddFileMappings(ctx context.Context, mappings []*entity.FileTaxonomyMapping) error

	// Statistics
	GetNodeStats(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) (*TaxonomyNodeStats, error)
	GetTaxonomyStats(ctx context.Context, workspaceID entity.WorkspaceID) (*TaxonomyStats, error)

	// Search and filter
	SearchNodes(ctx context.Context, workspaceID entity.WorkspaceID, query string, limit int) ([]*entity.TaxonomyNode, error)
	GetNodesBySource(ctx context.Context, workspaceID entity.WorkspaceID, source entity.TaxonomyNodeSource) ([]*entity.TaxonomyNode, error)
	GetLowConfidenceNodes(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.TaxonomyNode, error)

	// Merge operations
	MergeNodes(ctx context.Context, workspaceID entity.WorkspaceID, sourceID, targetID entity.TaxonomyNodeID) error
}

// TaxonomyNodeStats contains statistics for a single taxonomy node.
type TaxonomyNodeStats struct {
	NodeID           entity.TaxonomyNodeID
	DirectFileCount  int
	TotalFileCount   int // Including descendants
	ChildCount       int
	DescendantCount  int
	AverageScore     float64
	LastMappingAt    *int64 // Unix timestamp
}

// TaxonomyStats contains overall taxonomy statistics.
type TaxonomyStats struct {
	TotalNodes       int
	RootNodes        int
	MaxDepth         int
	TotalMappings    int
	NodesBySource    map[entity.TaxonomyNodeSource]int
	AverageConfidence float64
	OrphanedNodes    int // Nodes with no files and no children
}
