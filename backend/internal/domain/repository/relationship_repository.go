package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// RelationshipRepository defines storage for document relationships.
type RelationshipRepository interface {
	// CRUD operations
	Create(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.DocumentRelationship) error
	Get(ctx context.Context, workspaceID entity.WorkspaceID, id entity.RelationshipID) (*entity.DocumentRelationship, error)
	Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.RelationshipID) error
	DeleteByDocuments(ctx context.Context, workspaceID entity.WorkspaceID, fromDocID, toDocID entity.DocumentID, relType entity.RelationshipType) error

	// Query operations
	GetOutgoing(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]*entity.DocumentRelationship, error)
	GetIncoming(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]*entity.DocumentRelationship, error)
	GetAllOutgoing(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentRelationship, error)
	GetAllIncoming(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentRelationship, error)
	GetRelated(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]entity.DocumentID, error)

	// Graph traversal
	Traverse(ctx context.Context, workspaceID entity.WorkspaceID, startDocID entity.DocumentID, relType entity.RelationshipType, maxDepth int) ([]entity.DocumentID, error)
	GetReplacementChain(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]entity.DocumentID, error) // For "replaces" relationships

	// Project relationship operations
	AddProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.ProjectRelationship) error
	RemoveProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, fromProjectID, toProjectID entity.ProjectID, relType entity.RelationshipType) error
	GetProjectRelationships(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, relType *entity.RelationshipType) ([]*entity.ProjectRelationship, error)
	GetRelatedProjects(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, relType *entity.RelationshipType) ([]entity.ProjectID, error)
}

