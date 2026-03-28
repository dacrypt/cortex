// Package entity contains domain entities for the Cortex system.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// TaxonomyNodeID uniquely identifies a taxonomy node.
type TaxonomyNodeID string

// NewTaxonomyNodeID creates a new unique taxonomy node ID.
func NewTaxonomyNodeID() TaxonomyNodeID {
	return TaxonomyNodeID(uuid.New().String())
}

// String returns the string representation of the taxonomy node ID.
func (id TaxonomyNodeID) String() string {
	return string(id)
}

// TaxonomyNodeSource indicates how the taxonomy node was created.
type TaxonomyNodeSource string

const (
	// TaxonomyNodeSourceInferred indicates the node was inferred by the LLM.
	TaxonomyNodeSourceInferred TaxonomyNodeSource = "inferred"
	// TaxonomyNodeSourceUser indicates the node was created by the user.
	TaxonomyNodeSourceUser TaxonomyNodeSource = "user"
	// TaxonomyNodeSourceMerged indicates the node was created by merging others.
	TaxonomyNodeSourceMerged TaxonomyNodeSource = "merged"
	// TaxonomyNodeSourceSystem indicates the node is a system default.
	TaxonomyNodeSourceSystem TaxonomyNodeSource = "system"
)

// TaxonomyNode represents a node in the dynamic taxonomy hierarchy.
// The taxonomy is a hierarchical categorization system that evolves
// based on content analysis and user feedback.
type TaxonomyNode struct {
	ID          TaxonomyNodeID     `json:"id"`
	WorkspaceID WorkspaceID        `json:"workspace_id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	ParentID    *TaxonomyNodeID    `json:"parent_id,omitempty"`
	Path        string             `json:"path"` // Full path like "root/category/subcategory"
	Level       int                `json:"level"`
	Source      TaxonomyNodeSource `json:"source"`
	Confidence  float64            `json:"confidence"` // How confident we are in this node
	Keywords    []string           `json:"keywords,omitempty"`
	ExampleDocs []DocumentID       `json:"example_docs,omitempty"`
	ChildCount  int                `json:"child_count"`
	DocCount    int                `json:"doc_count"` // Number of documents in this node
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// NewTaxonomyNode creates a new taxonomy node.
func NewTaxonomyNode(workspaceID WorkspaceID, name string, parentID *TaxonomyNodeID) *TaxonomyNode {
	now := time.Now()
	return &TaxonomyNode{
		ID:          NewTaxonomyNodeID(),
		WorkspaceID: workspaceID,
		Name:        name,
		ParentID:    parentID,
		Level:       0,
		Source:      TaxonomyNodeSourceInferred,
		Confidence:  1.0,
		Keywords:    []string{},
		ExampleDocs: []DocumentID{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// SetPath sets the full path and updates the level.
func (n *TaxonomyNode) SetPath(path string) {
	n.Path = path
	// Count path segments to determine level
	if path == "" {
		n.Level = 0
		return
	}
	level := 0
	for _, c := range path {
		if c == '/' {
			level++
		}
	}
	n.Level = level
}

// IsRoot returns true if this is a root node (no parent).
func (n *TaxonomyNode) IsRoot() bool {
	return n.ParentID == nil
}

// FileTaxonomyMapping represents a document's mapping to a taxonomy node.
type FileTaxonomyMapping struct {
	FileID      FileID             `json:"file_id"`
	NodeID      TaxonomyNodeID     `json:"node_id"`
	WorkspaceID WorkspaceID        `json:"workspace_id"`
	Score       float64            `json:"score"`
	Source      TaxonomyNodeSource `json:"source"` // "manual", "auto", "suggested"
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// NewFileTaxonomyMapping creates a new file-to-taxonomy mapping.
func NewFileTaxonomyMapping(workspaceID WorkspaceID, fileID FileID, nodeID TaxonomyNodeID, source TaxonomyNodeSource) *FileTaxonomyMapping {
	now := time.Now()
	return &FileTaxonomyMapping{
		FileID:      fileID,
		NodeID:      nodeID,
		WorkspaceID: workspaceID,
		Score:       1.0,
		Source:      source,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TaxonomyInductionRequest represents a request to induce taxonomy from content.
type TaxonomyInductionRequest struct {
	WorkspaceID     WorkspaceID
	MaxLevels       int      // Maximum depth of taxonomy
	MaxNodesPerLevel int     // Maximum nodes per level
	SeedCategories  []string // Optional initial categories to consider
	IncludeExisting bool     // Whether to include existing taxonomy nodes
}

// TaxonomyInductionResult represents the result of taxonomy induction.
type TaxonomyInductionResult struct {
	NodesCreated  int
	NodesMerged   int
	NodesUpdated  int
	MappingsAdded int
	Errors        []string
}

// TaxonomySuggestion represents a suggested taxonomy categorization for a document.
type TaxonomySuggestion struct {
	DocumentID   DocumentID       `json:"document_id"`
	SuggestedPath string          `json:"suggested_path"` // Full path suggestion
	NodeID       *TaxonomyNodeID  `json:"node_id,omitempty"` // If matching existing node
	NewNodeName  string           `json:"new_node_name,omitempty"` // If suggesting new node
	Confidence   float64          `json:"confidence"`
	Reasoning    string           `json:"reasoning,omitempty"`
}
