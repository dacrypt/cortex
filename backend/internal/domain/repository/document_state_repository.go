package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// DocumentStateRepository defines storage for document state and transitions.
type DocumentStateRepository interface {
	// State management
	SetState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, state entity.DocumentState, reason string) error
	GetState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) (entity.DocumentState, error)
	GetStateHistory(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentStateTransition, error)

	// Queries
	GetDocumentsByState(ctx context.Context, workspaceID entity.WorkspaceID, state entity.DocumentState) ([]entity.DocumentID, error)
	GetDocumentsByStates(ctx context.Context, workspaceID entity.WorkspaceID, states []entity.DocumentState) ([]entity.DocumentID, error)
}

