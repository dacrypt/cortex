package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TraceRepository provides trace persistence in SQLite.
type TraceRepository struct {
	conn *Connection
}

// NewTraceRepository creates a new trace repository.
func NewTraceRepository(conn *Connection) *TraceRepository {
	return &TraceRepository{conn: conn}
}

// AddTrace inserts a new processing trace.
func (r *TraceRepository) AddTrace(ctx context.Context, trace entity.ProcessingTrace) error {
	_, err := r.conn.Exec(ctx, `
		INSERT INTO file_traces (
			workspace_id,
			file_id,
			relative_path,
			stage,
			operation,
			prompt_path,
			output_path,
			prompt_preview,
			output_preview,
			model,
			tokens_used,
			duration_ms,
			error,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		trace.WorkspaceID,
		trace.FileID,
		trace.RelativePath,
		trace.Stage,
		trace.Operation,
		trace.PromptPath,
		trace.OutputPath,
		trace.PromptPreview,
		trace.OutputPreview,
		trace.Model,
		trace.TokensUsed,
		trace.DurationMs,
		trace.Error,
		trace.CreatedAt.Unix(),
	)
	return err
}

// ListTracesByFile lists traces for a specific file.
func (r *TraceRepository) ListTracesByFile(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string, limit int) ([]entity.ProcessingTrace, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.conn.Query(ctx, `
		SELECT workspace_id,
		       file_id,
		       relative_path,
		       stage,
		       operation,
		       prompt_path,
		       output_path,
		       prompt_preview,
		       output_preview,
		       model,
		       tokens_used,
		       duration_ms,
		       error,
		       created_at
		  FROM file_traces
		 WHERE workspace_id = ?
		   AND relative_path = ?
		 ORDER BY created_at DESC
		 LIMIT ?
	`, workspaceID, relativePath, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []entity.ProcessingTrace
	for rows.Next() {
		trace, err := scanTrace(rows)
		if err != nil {
			return nil, err
		}
		traces = append(traces, trace)
	}
	return traces, rows.Err()
}

// ListRecentTraces lists recent traces for a workspace.
func (r *TraceRepository) ListRecentTraces(ctx context.Context, workspaceID entity.WorkspaceID, limit int) ([]entity.ProcessingTrace, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.conn.Query(ctx, `
		SELECT workspace_id,
		       file_id,
		       relative_path,
		       stage,
		       operation,
		       prompt_path,
		       output_path,
		       prompt_preview,
		       output_preview,
		       model,
		       tokens_used,
		       duration_ms,
		       error,
		       created_at
		  FROM file_traces
		 WHERE workspace_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?
	`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []entity.ProcessingTrace
	for rows.Next() {
		trace, err := scanTrace(rows)
		if err != nil {
			return nil, err
		}
		traces = append(traces, trace)
	}
	return traces, rows.Err()
}

func scanTrace(rows *sql.Rows) (entity.ProcessingTrace, error) {
	var trace entity.ProcessingTrace
	var createdAt int64
	if err := rows.Scan(
		&trace.WorkspaceID,
		&trace.FileID,
		&trace.RelativePath,
		&trace.Stage,
		&trace.Operation,
		&trace.PromptPath,
		&trace.OutputPath,
		&trace.PromptPreview,
		&trace.OutputPreview,
		&trace.Model,
		&trace.TokensUsed,
		&trace.DurationMs,
		&trace.Error,
		&createdAt,
	); err != nil {
		return trace, err
	}
	trace.CreatedAt = time.Unix(createdAt, 0)
	return trace, nil
}
