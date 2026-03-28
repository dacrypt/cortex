package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ClearFileData clears only file-related data (files, metadata, tags, contexts)
// but preserves documents, chunks, embeddings, and clusters for incremental clustering.
// This is used for "force full scan" to reindex files while maintaining clustering state.
func (r *WorkspaceRepository) ClearFileData(ctx context.Context, workspaceID entity.WorkspaceID, workspaceRoot string) error {
	// Get workspace to verify it exists
	ws, err := r.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}
	if ws == nil {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	// Use workspace root from parameter if provided, otherwise use workspace path
	root := workspaceRoot
	if root == "" {
		root = ws.Path
	}

	// Delete only file-related data from database in a transaction
	err = r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Delete only file-related tables, preserving documents, chunks, embeddings, clusters
		// Delete in order to respect foreign key constraints
		fileTables := []struct {
			name string
			key  string
		}{
			{"file_traces", "workspace_id"},
			{"file_context_suggestions", "workspace_id"},
			{"file_contexts", "workspace_id"},
			{"file_tags", "workspace_id"},
			{"file_metadata", "workspace_id"},
			{"files", "workspace_id"},
		}

		for _, table := range fileTables {
			// Check if table exists before trying to delete
			var exists int
			checkQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM sqlite_master 
				WHERE type='table' AND name='%s'
			`, table.name)
			if err := tx.QueryRowContext(ctx, checkQuery).Scan(&exists); err != nil {
				continue
			} else if exists == 0 {
				continue
			}

			query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table.name, table.key)
			if _, err := tx.ExecContext(ctx, query, workspaceID.String()); err != nil {
				// Check if error is "no such table" - if so, skip it
				if sqlErr, ok := err.(interface{ Error() string }); ok {
					errMsg := sqlErr.Error()
					if contains(errMsg, "no such table") {
						continue
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to clear file data: %w", err)
	}

	// Delete .cortex directory contents (mirrors and traces)
	cortexDir := filepath.Join(root, ".cortex")
	if err := r.clearCortexDirectory(cortexDir); err != nil {
		// Log but don't fail - file deletion is best effort
		return fmt.Errorf("failed to clear .cortex directory: %w", err)
	}

	return nil
}

// ClearWorkspaceData deletes all data associated with a workspace from the database
// and removes all .cortex files (mirrors, traces) from the workspace directory.
// This is a complete wipe - use ClearFileData for incremental reindexing.
func (r *WorkspaceRepository) ClearWorkspaceData(ctx context.Context, workspaceID entity.WorkspaceID, workspaceRoot string) error {
	// Get workspace to verify it exists and get the root path
	ws, err := r.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}
	if ws == nil {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	// Use workspace root from parameter if provided, otherwise use workspace path
	root := workspaceRoot
	if root == "" {
		root = ws.Path
	}

	// Delete all workspace data from database in a transaction
	err = r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Delete in order to respect foreign key constraints
		// Delete child tables first (those that reference other workspace tables)
		// Note: Some tables may not exist if migrations haven't run yet, so we check first
		tables := []struct {
			name string
			key  string
		}{
			{"chunk_embeddings", "workspace_id"},
			{"chunks", "workspace_id"},
			{"document_relationships", "workspace_id"},
			{"document_state_history", "workspace_id"}, // Fixed: was document_states
			{"document_usage_events", "workspace_id"},
			{"document_usage_stats", "workspace_id"},
			{"project_documents", "workspace_id"},
			{"project_relationships", "workspace_id"},
			{"projects", "workspace_id"},
			{"documents", "workspace_id"}, // Delete documents after relationships
			{"file_traces", "workspace_id"},
			{"file_context_suggestions", "workspace_id"},
			{"file_contexts", "workspace_id"},
			{"file_tags", "workspace_id"},
			{"file_metadata", "workspace_id"},
			{"files", "workspace_id"},
		}

		for _, table := range tables {
			// Check if table exists before trying to delete
			var exists int
			checkQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM sqlite_master 
				WHERE type='table' AND name='%s'
			`, table.name)
			if err := tx.QueryRowContext(ctx, checkQuery).Scan(&exists); err != nil {
				// If we can't check, try to delete anyway (might fail gracefully)
				// Use a simple logger if conn.Logger is not available
				_ = err // Log would require logger access
			} else if exists == 0 {
				// Table doesn't exist, skip it
				continue
			}

			query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table.name, table.key)
			if _, err := tx.ExecContext(ctx, query, workspaceID.String()); err != nil {
				// Check if error is "no such table" - if so, skip it
				if sqlErr, ok := err.(interface{ Error() string }); ok {
					errMsg := sqlErr.Error()
					if contains(errMsg, "no such table") {
						// Table doesn't exist, skip it
						continue
					}
				}
				// For other errors, log but continue - some tables may not exist in older schemas
				// Don't return error - continue with other tables
			}
		}

		// Delete tasks associated with this workspace
		if _, err := tx.ExecContext(ctx, "DELETE FROM tasks WHERE workspace_id = ?", workspaceID.String()); err != nil {
			return fmt.Errorf("failed to delete tasks: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to clear workspace data: %w", err)
	}

	// Delete .cortex directory contents (mirrors and traces)
	cortexDir := filepath.Join(root, ".cortex")
	if err := r.clearCortexDirectory(cortexDir); err != nil {
		// Log but don't fail - file deletion is best effort
		return fmt.Errorf("failed to clear .cortex directory: %w", err)
	}

	return nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// clearCortexDirectory removes all files and subdirectories in .cortex except the database
func (r *WorkspaceRepository) clearCortexDirectory(cortexDir string) error {
	// Check if .cortex directory exists
	if _, err := os.Stat(cortexDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Remove mirror directory
	mirrorDir := filepath.Join(cortexDir, "mirror")
	if err := os.RemoveAll(mirrorDir); err != nil {
		return fmt.Errorf("failed to remove mirror directory: %w", err)
	}

	// Remove traces directory
	tracesDir := filepath.Join(cortexDir, "traces")
	if err := os.RemoveAll(tracesDir); err != nil {
		return fmt.Errorf("failed to remove traces directory: %w", err)
	}

	return nil
}

