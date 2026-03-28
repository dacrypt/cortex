package query

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// QueryBuilder provides a fluent interface for building queries.
type QueryBuilder struct {
	workspaceID entity.WorkspaceID
	filters     []Filter
	ordering    *Ordering
	limit       int
	offset      int
}

// Query creates a new query builder.
func Query(workspaceID entity.WorkspaceID) *QueryBuilder {
	return &QueryBuilder{
		workspaceID: workspaceID,
		filters:     []Filter{},
		limit:       100, // Default limit
	}
}

// Filter adds a filter to the query.
func (qb *QueryBuilder) Filter(f Filter) *QueryBuilder {
	qb.filters = append(qb.filters, f)
	return qb
}

// OrderBy sets the ordering for the query.
func (qb *QueryBuilder) OrderBy(field string, descending bool) *QueryBuilder {
	qb.ordering = &Ordering{
		Field:      field,
		Descending: descending,
	}
	return qb
}

// Limit sets the maximum number of results.
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	if limit > 0 {
		qb.limit = limit
	}
	return qb
}

// Offset sets the offset for pagination.
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	if offset >= 0 {
		qb.offset = offset
	}
	return qb
}

// Execute executes the query and returns results.
func (qb *QueryBuilder) Execute(ctx context.Context, executor *Executor) (*QueryResult, error) {
	return executor.Execute(ctx, qb)
}

// Filter is an interface for query filters.
type Filter interface {
	Apply(ctx context.Context, executor *Executor, workspaceID entity.WorkspaceID) ([]entity.DocumentID, error)
}

// Ordering specifies how to order query results.
type Ordering struct {
	Field      string
	Descending bool
}

// QueryResult contains the results of a query execution.
type QueryResult struct {
	DocumentIDs []entity.DocumentID
	Total       int
	HasMore     bool
}

