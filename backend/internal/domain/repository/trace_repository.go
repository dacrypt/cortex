package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TraceRepository stores processing traces.
type TraceRepository interface {
	AddTrace(ctx context.Context, trace entity.ProcessingTrace) error
	ListTracesByFile(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string, limit int) ([]entity.ProcessingTrace, error)
	ListRecentTraces(ctx context.Context, workspaceID entity.WorkspaceID, limit int) ([]entity.ProcessingTrace, error)
}
