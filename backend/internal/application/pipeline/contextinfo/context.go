package contextinfo

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

type contextKey string

const workspaceInfoKey contextKey = "workspace_info"

// WorkspaceInfo carries workspace metadata for pipeline stages.
type WorkspaceInfo struct {
	ID            entity.WorkspaceID
	Root          string
	Config        entity.WorkspaceConfig
	ForceFullScan bool // When true, forces regeneration of summaries and AI metadata
}

// WithWorkspaceInfo attaches workspace info to a context.
func WithWorkspaceInfo(ctx context.Context, info WorkspaceInfo) context.Context {
	return context.WithValue(ctx, workspaceInfoKey, info)
}

// GetWorkspaceInfo extracts workspace info from a context.
func GetWorkspaceInfo(ctx context.Context) (WorkspaceInfo, bool) {
	info, ok := ctx.Value(workspaceInfoKey).(WorkspaceInfo)
	return info, ok
}
