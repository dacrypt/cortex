// Package sqlite provides SQLite implementations of repositories.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// WorkspaceRepository implements repository.WorkspaceRepository using SQLite.
type WorkspaceRepository struct {
	conn *Connection
}

// NewWorkspaceRepository creates a new SQLite workspace repository.
func NewWorkspaceRepository(conn *Connection) *WorkspaceRepository {
	return &WorkspaceRepository{conn: conn}
}

// Create creates a new workspace.
func (r *WorkspaceRepository) Create(ctx context.Context, ws *entity.Workspace) error {
	configJSON, err := json.Marshal(ws.Config)
	if err != nil {
		return err
	}

	createdAt := ws.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := ws.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	var lastIndexed *int64
	if ws.LastIndexed != nil {
		ts := ws.LastIndexed.UnixMilli()
		lastIndexed = &ts
	}

	query := `
		INSERT INTO workspaces (
			id, path, name, active, last_indexed, file_count, config, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.conn.db.ExecContext(ctx, query,
		ws.ID.String(),
		ws.Path,
		ws.Name,
		ws.Active,
		lastIndexed,
		ws.FileCount,
		configJSON,
		createdAt.UnixMilli(),
		updatedAt.UnixMilli(),
	)
	return err
}

// Get retrieves a workspace by ID.
func (r *WorkspaceRepository) Get(ctx context.Context, id entity.WorkspaceID) (*entity.Workspace, error) {
	query := `
		SELECT id, path, name, active, last_indexed, file_count, config, created_at, updated_at
		FROM workspaces WHERE id = ?`
	row := r.conn.db.QueryRowContext(ctx, query, id.String())
	return r.scanWorkspace(row)
}

// GetByPath retrieves a workspace by path.
func (r *WorkspaceRepository) GetByPath(ctx context.Context, path string) (*entity.Workspace, error) {
	query := `
		SELECT id, path, name, active, last_indexed, file_count, config, created_at, updated_at
		FROM workspaces WHERE path = ?`
	row := r.conn.db.QueryRowContext(ctx, query, path)
	ws, err := r.scanWorkspace(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ws, err
}

// Update updates an existing workspace.
func (r *WorkspaceRepository) Update(ctx context.Context, ws *entity.Workspace) error {
	configJSON, err := json.Marshal(ws.Config)
	if err != nil {
		return err
	}

	var lastIndexed *int64
	if ws.LastIndexed != nil {
		ts := ws.LastIndexed.UnixMilli()
		lastIndexed = &ts
	}

	query := `
		UPDATE workspaces
		SET path = ?, name = ?, active = ?, last_indexed = ?, file_count = ?, config = ?, updated_at = ?
		WHERE id = ?`

	_, err = r.conn.db.ExecContext(ctx, query,
		ws.Path,
		ws.Name,
		ws.Active,
		lastIndexed,
		ws.FileCount,
		configJSON,
		time.Now().UnixMilli(),
		ws.ID.String(),
	)
	return err
}

// Delete deletes a workspace.
func (r *WorkspaceRepository) Delete(ctx context.Context, id entity.WorkspaceID) error {
	_, err := r.conn.db.ExecContext(ctx, "DELETE FROM workspaces WHERE id = ?", id.String())
	return err
}

// List lists workspaces with options.
func (r *WorkspaceRepository) List(ctx context.Context, opts repository.WorkspaceListOptions) ([]*entity.Workspace, error) {
	query := `
		SELECT id, path, name, active, last_indexed, file_count, config, created_at, updated_at
		FROM workspaces`
	args := []interface{}{}
	if opts.ActiveOnly {
		query += " WHERE active = 1"
	}
	query += " ORDER BY name LIMIT ? OFFSET ?"

	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	args = append(args, limit, opts.Offset)

	rows, err := r.conn.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []*entity.Workspace
	for rows.Next() {
		ws, err := r.scanWorkspaceRow(rows)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}

	return workspaces, nil
}

// ListActive lists only active workspaces.
func (r *WorkspaceRepository) ListActive(ctx context.Context) ([]*entity.Workspace, error) {
	opts := repository.DefaultWorkspaceListOptions()
	opts.ActiveOnly = true
	return r.List(ctx, opts)
}

// SetActive sets the active status of a workspace.
func (r *WorkspaceRepository) SetActive(ctx context.Context, id entity.WorkspaceID, active bool) error {
	_, err := r.conn.db.ExecContext(ctx,
		"UPDATE workspaces SET active = ?, updated_at = ? WHERE id = ?",
		active, time.Now().UnixMilli(), id.String())
	return err
}

// UpdateLastIndexed updates the last indexed time for a workspace.
func (r *WorkspaceRepository) UpdateLastIndexed(ctx context.Context, id entity.WorkspaceID) error {
	_, err := r.conn.db.ExecContext(ctx,
		"UPDATE workspaces SET last_indexed = ?, updated_at = ? WHERE id = ?",
		time.Now().UnixMilli(), time.Now().UnixMilli(), id.String())
	return err
}

// UpdateFileCount updates the file count for a workspace.
func (r *WorkspaceRepository) UpdateFileCount(ctx context.Context, id entity.WorkspaceID, count int) error {
	_, err := r.conn.db.ExecContext(ctx,
		"UPDATE workspaces SET file_count = ?, updated_at = ? WHERE id = ?",
		count, time.Now().UnixMilli(), id.String())
	return err
}

// UpdateConfig updates the config for a workspace.
func (r *WorkspaceRepository) UpdateConfig(ctx context.Context, id entity.WorkspaceID, config entity.WorkspaceConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = r.conn.db.ExecContext(ctx,
		"UPDATE workspaces SET config = ?, updated_at = ? WHERE id = ?",
		configJSON, time.Now().UnixMilli(), id.String())
	return err
}

func (r *WorkspaceRepository) scanWorkspace(row *sql.Row) (*entity.Workspace, error) {
	var (
		ws          entity.Workspace
		lastIndexed *int64
		fileCount   int
		configJSON  []byte
		createdAt   int64
		updatedAt   int64
	)

	err := row.Scan(&ws.ID, &ws.Path, &ws.Name, &ws.Active, &lastIndexed, &fileCount, &configJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if lastIndexed != nil {
		t := time.UnixMilli(*lastIndexed)
		ws.LastIndexed = &t
	}

	ws.FileCount = fileCount
	ws.CreatedAt = time.UnixMilli(createdAt)
	ws.UpdatedAt = time.UnixMilli(updatedAt)

	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &ws.Config); err != nil {
			return nil, err
		}
	}

	return &ws, nil
}

func (r *WorkspaceRepository) scanWorkspaceRow(rows *sql.Rows) (*entity.Workspace, error) {
	var (
		ws          entity.Workspace
		lastIndexed *int64
		fileCount   int
		configJSON  []byte
		createdAt   int64
		updatedAt   int64
	)

	err := rows.Scan(&ws.ID, &ws.Path, &ws.Name, &ws.Active, &lastIndexed, &fileCount, &configJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if lastIndexed != nil {
		t := time.UnixMilli(*lastIndexed)
		ws.LastIndexed = &t
	}

	ws.FileCount = fileCount
	ws.CreatedAt = time.UnixMilli(createdAt)
	ws.UpdatedAt = time.UnixMilli(updatedAt)

	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &ws.Config); err != nil {
			return nil, err
		}
	}

	return &ws, nil
}

// Count returns the number of workspaces.
func (r *WorkspaceRepository) Count(ctx context.Context) (int, error) {
	row := r.conn.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces`)
	var count int
	err := row.Scan(&count)
	return count, err
}

// Ensure WorkspaceRepository implements repository.WorkspaceRepository
var _ repository.WorkspaceRepository = (*WorkspaceRepository)(nil)
