package query

import (
	"context"
	"sort"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Executor executes queries against repositories.
type Executor struct {
	docRepo    repository.DocumentRepository
	projectRepo repository.ProjectRepository
	stateRepo  repository.DocumentStateRepository
	relRepo    repository.RelationshipRepository
	usageRepo  repository.UsageRepository
}

// NewExecutor creates a new query executor.
func NewExecutor(
	docRepo repository.DocumentRepository,
	projectRepo repository.ProjectRepository,
	stateRepo repository.DocumentStateRepository,
	relRepo repository.RelationshipRepository,
	usageRepo repository.UsageRepository,
) *Executor {
	return &Executor{
		docRepo:    docRepo,
		projectRepo: projectRepo,
		stateRepo:  stateRepo,
		relRepo:    relRepo,
		usageRepo:  usageRepo,
	}
}

// Execute executes a query and returns results.
func (e *Executor) Execute(ctx context.Context, qb *QueryBuilder) (*QueryResult, error) {
	var docIDs []entity.DocumentID

	// Apply all filters
	if len(qb.filters) == 0 {
		// No filters - return empty (or all documents if needed)
		return &QueryResult{
			DocumentIDs: []entity.DocumentID{},
			Total:       0,
			HasMore:     false,
		}, nil
	}

	// If multiple filters, combine with AND logic by default
	if len(qb.filters) == 1 {
		var err error
		docIDs, err = qb.filters[0].Apply(ctx, e, qb.workspaceID)
		if err != nil {
			return nil, err
		}
	} else {
		// Use AndFilter to combine
		andFilter := &AndFilter{Filters: qb.filters}
		var err error
		docIDs, err = andFilter.Apply(ctx, e, qb.workspaceID)
		if err != nil {
			return nil, err
		}
	}

	// Apply ordering
	if qb.ordering != nil {
		if err := e.applyOrdering(ctx, qb.workspaceID, docIDs, qb.ordering); err != nil {
			return nil, err
		}
	}

	// Apply pagination
	total := len(docIDs)
	start := qb.offset
	end := start + qb.limit
	if end > len(docIDs) {
		end = len(docIDs)
	}
	if start > len(docIDs) {
		start = len(docIDs)
	}

	result := docIDs[start:end]
	hasMore := end < total

	return &QueryResult{
		DocumentIDs: result,
		Total:       total,
		HasMore:     hasMore,
	}, nil
}

// applyOrdering sorts document IDs based on the ordering specification.
func (e *Executor) applyOrdering(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docIDs []entity.DocumentID,
	ordering *Ordering,
) error {
	// Fetch documents to get ordering fields
	type docWithField struct {
		docID entity.DocumentID
		field interface{}
	}

	docsWithFields := make([]docWithField, 0, len(docIDs))
	for _, docID := range docIDs {
		doc, err := e.docRepo.GetDocument(ctx, workspaceID, docID)
		if err != nil {
			continue
		}

		var field interface{}
		switch ordering.Field {
		case "updated_at", "updatedAt":
			field = doc.UpdatedAt
		case "created_at", "createdAt":
			field = doc.CreatedAt
		case "title":
			field = doc.Title
		default:
			field = doc.UpdatedAt // Default to updated_at
		}

		docsWithFields = append(docsWithFields, docWithField{
			docID: docID,
			field: field,
		})
	}

	// Sort
	sort.Slice(docsWithFields, func(i, j int) bool {
		if ordering.Descending {
			return e.compareFields(docsWithFields[i].field, docsWithFields[j].field) > 0
		}
		return e.compareFields(docsWithFields[i].field, docsWithFields[j].field) < 0
	})

	// Update docIDs with sorted order
	for i, dwf := range docsWithFields {
		docIDs[i] = dwf.docID
	}

	return nil
}

// compareFields compares two field values for sorting.
func (e *Executor) compareFields(a, b interface{}) int {
	switch aVal := a.(type) {
	case time.Time:
		if bVal, ok := b.(time.Time); ok {
			if aVal.Before(bVal) {
				return -1
			}
			if aVal.After(bVal) {
				return 1
			}
			return 0
		}
	case string:
		if bVal, ok := b.(string); ok {
			if aVal < bVal {
				return -1
			}
			if aVal > bVal {
				return 1
			}
			return 0
		}
	}
	return 0
}

