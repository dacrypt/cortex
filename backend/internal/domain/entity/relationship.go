package entity

import (
	"time"

	"github.com/google/uuid"
)

// RelationshipID uniquely identifies a relationship.
type RelationshipID string

// NewRelationshipID creates a new unique RelationshipID.
func NewRelationshipID() RelationshipID {
	return RelationshipID(uuid.New().String())
}

// String returns the string representation of RelationshipID.
func (id RelationshipID) String() string {
	return string(id)
}

// RelationshipType represents the type of relationship between documents.
type RelationshipType string

const (
	// RelationshipReplaces indicates one document replaces another (versioning).
	RelationshipReplaces RelationshipType = "replaces"
	// RelationshipDependsOn indicates one document depends on another.
	RelationshipDependsOn RelationshipType = "depends_on"
	// RelationshipBelongsTo indicates one document belongs to another (composition).
	RelationshipBelongsTo RelationshipType = "belongs_to"
	// RelationshipReferences indicates one document references another.
	RelationshipReferences RelationshipType = "references"
	// RelationshipParentOf indicates one project is a parent of another (for project hierarchy).
	RelationshipParentOf RelationshipType = "parent_of"
)

// IsValid returns true if the relationship type is valid.
func (rt RelationshipType) IsValid() bool {
	return rt == RelationshipReplaces ||
		rt == RelationshipDependsOn ||
		rt == RelationshipBelongsTo ||
		rt == RelationshipReferences ||
		rt == RelationshipParentOf
}

// String returns the string representation of RelationshipType.
func (rt RelationshipType) String() string {
	return string(rt)
}

// DocumentRelationship represents a typed relationship between two documents.
type DocumentRelationship struct {
	ID           RelationshipID
	WorkspaceID  WorkspaceID
	FromDocument DocumentID
	ToDocument   DocumentID
	Type         RelationshipType
	Strength     float64 // 0.0-1.0, optional confidence/weight
	Metadata     map[string]interface{} // Optional context
	CreatedAt    time.Time
}

// NewDocumentRelationship creates a new document relationship.
func NewDocumentRelationship(
	workspaceID WorkspaceID,
	fromDocID DocumentID,
	toDocID DocumentID,
	relType RelationshipType,
) *DocumentRelationship {
	return &DocumentRelationship{
		ID:           NewRelationshipID(),
		WorkspaceID:  workspaceID,
		FromDocument: fromDocID,
		ToDocument:   toDocID,
		Type:         relType,
		Strength:     1.0, // Default to full strength
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}
}

// WithStrength sets the relationship strength.
func (r *DocumentRelationship) WithStrength(strength float64) *DocumentRelationship {
	if strength < 0.0 {
		strength = 0.0
	}
	if strength > 1.0 {
		strength = 1.0
	}
	r.Strength = strength
	return r
}

// WithMetadata sets relationship metadata.
func (r *DocumentRelationship) WithMetadata(key string, value interface{}) *DocumentRelationship {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// ProjectRelationship represents a typed relationship between two projects.
type ProjectRelationship struct {
	FromProjectID ProjectID
	ToProjectID   ProjectID
	Type          RelationshipType
	Description   string
	CreatedAt     time.Time
}

// NewProjectRelationship creates a new project relationship.
func NewProjectRelationship(
	fromProjectID ProjectID,
	toProjectID ProjectID,
	relType RelationshipType,
) *ProjectRelationship {
	return &ProjectRelationship{
		FromProjectID: fromProjectID,
		ToProjectID:   toProjectID,
		Type:          relType,
		CreatedAt:     time.Now(),
	}
}

// WithDescription sets the relationship description.
func (r *ProjectRelationship) WithDescription(description string) *ProjectRelationship {
	r.Description = description
	return r
}

