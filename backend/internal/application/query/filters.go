package query

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ProjectFilter filters documents by project membership.
type ProjectFilter struct {
	ProjectID          entity.ProjectID
	IncludeSubprojects bool
}

// Apply implements Filter interface.
func (f *ProjectFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	return executor.projectRepo.GetDocuments(ctx, workspaceID, f.ProjectID, f.IncludeSubprojects)
}

// StateFilter filters documents by state.
type StateFilter struct {
	States []entity.DocumentState
}

// Apply implements Filter interface.
func (f *StateFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	if len(f.States) == 0 {
		return []entity.DocumentID{}, nil
	}
	if len(f.States) == 1 {
		return executor.stateRepo.GetDocumentsByState(ctx, workspaceID, f.States[0])
	}
	return executor.stateRepo.GetDocumentsByStates(ctx, workspaceID, f.States)
}

// RelationshipFilter filters documents by relationships.
type RelationshipFilter struct {
	FromDocument entity.DocumentID
	RelType      entity.RelationshipType
	MaxDepth     int
}

// Apply implements Filter interface.
func (f *RelationshipFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	if f.MaxDepth <= 0 {
		f.MaxDepth = 1
	}
	return executor.relRepo.Traverse(ctx, workspaceID, f.FromDocument, f.RelType, f.MaxDepth)
}

// TemporalFilter filters documents by usage events.
type TemporalFilter struct {
	Since     time.Time
	EventType entity.UsageEventType
	Limit     int
}

// Apply implements Filter interface.
func (f *TemporalFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}

	events, err := executor.usageRepo.GetEventsByType(ctx, workspaceID, f.EventType, f.Since, f.Limit)
	if err != nil {
		return nil, err
	}

	// Extract unique document IDs
	docSet := make(map[entity.DocumentID]bool)
	for _, event := range events {
		docSet[event.DocumentID] = true
	}

	result := make([]entity.DocumentID, 0, len(docSet))
	for docID := range docSet {
		result = append(result, docID)
	}

	return result, nil
}

// TypeFilter filters documents by file type.
type TypeFilter struct {
	FileType string
}

// Apply implements Filter interface.
func (f *TypeFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	// This would need to query file_metadata or files table
	// For now, return empty - would need metadata repo extension
	return []entity.DocumentID{}, nil
}

// TagFilter filters documents by tags.
type TagFilter struct {
	Tag string
}

// Apply implements Filter interface.
func (f *TagFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	// This would need metadata repo
	// For now, return empty
	return []entity.DocumentID{}, nil
}

// AndFilter combines multiple filters with AND logic.
type AndFilter struct {
	Filters []Filter
}

// Apply implements Filter interface.
func (f *AndFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	if len(f.Filters) == 0 {
		return []entity.DocumentID{}, nil
	}

	// Start with first filter
	result, err := f.Filters[0].Apply(ctx, executor, workspaceID)
	if err != nil {
		return nil, err
	}

	// Intersect with remaining filters
	for i := 1; i < len(f.Filters); i++ {
		other, err := f.Filters[i].Apply(ctx, executor, workspaceID)
		if err != nil {
			return nil, err
		}

		// Intersect
		otherSet := make(map[entity.DocumentID]bool)
		for _, id := range other {
			otherSet[id] = true
		}

		intersected := []entity.DocumentID{}
		for _, id := range result {
			if otherSet[id] {
				intersected = append(intersected, id)
			}
		}
		result = intersected
	}

	return result, nil
}

// OrFilter combines multiple filters with OR logic.
type OrFilter struct {
	Filters []Filter
}

// Apply implements Filter interface.
func (f *OrFilter) Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error) {
	if len(f.Filters) == 0 {
		return []entity.DocumentID{}, nil
	}

	docSet := make(map[entity.DocumentID]bool)

	for _, filter := range f.Filters {
		result, err := filter.Apply(ctx, executor, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, id := range result {
			docSet[id] = true
		}
	}

	result := make([]entity.DocumentID, 0, len(docSet))
	for id := range docSet {
		result = append(result, id)
	}

	return result, nil
}

