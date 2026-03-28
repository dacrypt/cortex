package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ProjectAssignmentRepository implements repository.ProjectAssignmentRepository using SQLite.
type ProjectAssignmentRepository struct {
	conn *Connection
}

// NewProjectAssignmentRepository creates a new SQLite project assignment repository.
func NewProjectAssignmentRepository(conn *Connection) *ProjectAssignmentRepository {
	return &ProjectAssignmentRepository{conn: conn}
}

// Upsert stores or updates a project assignment.
func (r *ProjectAssignmentRepository) Upsert(ctx context.Context, assignment *entity.ProjectAssignment) error {
	if assignment == nil {
		return fmt.Errorf("assignment is nil")
	}
	if assignment.ProjectName == "" {
		return fmt.Errorf("project name is required")
	}
	if assignment.Status != "" && !assignment.Status.IsValid() {
		return fmt.Errorf("invalid assignment status: %s", assignment.Status)
	}
	if assignment.CreatedAt.IsZero() {
		assignment.CreatedAt = time.Now()
	}
	if assignment.UpdatedAt.IsZero() {
		assignment.UpdatedAt = time.Now()
	}

	var sourcesJSON []byte
	if assignment.Sources != nil {
		data, err := json.Marshal(assignment.Sources)
		if err != nil {
			return fmt.Errorf("failed to marshal sources: %w", err)
		}
		sourcesJSON = data
	}

	query := `
		INSERT INTO project_assignments (
			workspace_id, file_id, project_id, project_name, score, sources, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, file_id, project_name) DO UPDATE SET
			project_id = excluded.project_id,
			score = excluded.score,
			sources = excluded.sources,
			status = excluded.status,
			updated_at = excluded.updated_at
	`
	_, err := r.conn.Exec(ctx, query,
		assignment.WorkspaceID.String(),
		assignment.FileID.String(),
		nullIfEmpty(assignment.ProjectID.String()),
		assignment.ProjectName,
		assignment.Score,
		sourcesJSON,
		string(assignment.Status),
		assignment.CreatedAt.UnixMilli(),
		assignment.UpdatedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert project assignment: %w", err)
	}

	return nil
}

// ListByFile retrieves assignments for a file.
func (r *ProjectAssignmentRepository) ListByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.ProjectAssignment, error) {
	query := `
		SELECT workspace_id, file_id, project_id, project_name, score, sources, status, created_at, updated_at
		FROM project_assignments
		WHERE workspace_id = ? AND file_id = ?
		ORDER BY score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments by file: %w", err)
	}
	defer rows.Close()

	return scanAssignments(rows)
}

// ListByProject retrieves assignments for a project.
func (r *ProjectAssignmentRepository) ListByProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) ([]*entity.ProjectAssignment, error) {
	query := `
		SELECT workspace_id, file_id, project_id, project_name, score, sources, status, created_at, updated_at
		FROM project_assignments
		WHERE workspace_id = ? AND project_id = ?
		ORDER BY score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), projectID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments by project: %w", err)
	}
	defer rows.Close()

	return scanAssignments(rows)
}

// DeleteByFile removes assignments for a file.
func (r *ProjectAssignmentRepository) DeleteByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `DELETE FROM project_assignments WHERE workspace_id = ? AND file_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String())
	return err
}

func scanAssignments(rows *sql.Rows) ([]*entity.ProjectAssignment, error) {
	assignments := []*entity.ProjectAssignment{}
	for rows.Next() {
		var assignment entity.ProjectAssignment
		var workspaceID, fileID, projectName string
		var projectID sql.NullString
		var sourcesJSON []byte
		var status string
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&workspaceID,
			&fileID,
			&projectID,
			&projectName,
			&assignment.Score,
			&sourcesJSON,
			&status,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		assignment.WorkspaceID = entity.WorkspaceID(workspaceID)
		assignment.FileID = entity.FileID(fileID)
		assignment.ProjectName = projectName
		if projectID.Valid {
			assignment.ProjectID = entity.ProjectID(projectID.String)
		}
		if len(sourcesJSON) > 0 {
			if err := json.Unmarshal(sourcesJSON, &assignment.Sources); err != nil {
				return nil, fmt.Errorf("failed to unmarshal sources: %w", err)
			}
		}
		if status != "" {
			assignment.Status = entity.ProjectAssignmentStatus(status)
		}
		assignment.CreatedAt = time.UnixMilli(createdAt)
		assignment.UpdatedAt = time.UnixMilli(updatedAt)

		assignments = append(assignments, &assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

