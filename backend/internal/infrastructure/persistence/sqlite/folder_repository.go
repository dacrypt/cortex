package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// FolderRepository implements repository.FolderRepository using SQLite.
type FolderRepository struct {
	conn *Connection
}

// NewFolderRepository creates a new SQLite folder repository.
func NewFolderRepository(conn *Connection) *FolderRepository {
	return &FolderRepository{conn: conn}
}

// GetByID retrieves a folder by ID.
func (r *FolderRepository) GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FolderID) (*entity.FolderEntry, error) {
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return r.scanFolderEntry(row)
}

// GetByPath retrieves a folder by relative path.
func (r *FolderRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FolderEntry, error) {
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND relative_path = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), relativePath)
	return r.scanFolderEntry(row)
}

// Upsert inserts or updates a folder entry.
func (r *FolderRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, folder *entity.FolderEntry) error {
	var metricsJSON, metadataJSON []byte
	var err error

	if folder.Metrics != nil {
		metricsJSON, err = json.Marshal(folder.Metrics)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}
	}

	if folder.Metadata != nil {
		metadataJSON, err = json.Marshal(folder.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO folders (id, workspace_id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, id) DO UPDATE SET
			relative_path = excluded.relative_path,
			name = excluded.name,
			parent_path = excluded.parent_path,
			depth = excluded.depth,
			metrics = excluded.metrics,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at
	`

	_, err = r.conn.Exec(ctx, query,
		folder.ID.String(),
		workspaceID.String(),
		folder.RelativePath,
		folder.Name,
		folder.ParentPath,
		folder.Depth,
		metricsJSON,
		metadataJSON,
		folder.CreatedAt.UnixMilli(),
		folder.UpdatedAt.UnixMilli(),
	)
	return err
}

// Delete removes a folder by ID.
func (r *FolderRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FolderID) error {
	query := `DELETE FROM folders WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), id.String())
	return err
}

// BulkUpsert inserts or updates multiple folder entries.
func (r *FolderRepository) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, folders []*entity.FolderEntry) (int, error) {
	if len(folders) == 0 {
		return 0, nil
	}

	count := 0
	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO folders (id, workspace_id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, id) DO UPDATE SET
				relative_path = excluded.relative_path,
				name = excluded.name,
				parent_path = excluded.parent_path,
				depth = excluded.depth,
				metrics = excluded.metrics,
				metadata = excluded.metadata,
				updated_at = excluded.updated_at
		`

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, folder := range folders {
			var metricsJSON, metadataJSON []byte

			if folder.Metrics != nil {
				metricsJSON, _ = json.Marshal(folder.Metrics)
			}
			if folder.Metadata != nil {
				metadataJSON, _ = json.Marshal(folder.Metadata)
			}

			_, err := stmt.ExecContext(ctx,
				folder.ID.String(),
				workspaceID.String(),
				folder.RelativePath,
				folder.Name,
				folder.ParentPath,
				folder.Depth,
				metricsJSON,
				metadataJSON,
				folder.CreatedAt.UnixMilli(),
				folder.UpdatedAt.UnixMilli(),
			)
			if err != nil {
				return fmt.Errorf("failed to upsert folder %s: %w", folder.RelativePath, err)
			}
			count++
		}
		return nil
	})

	return count, err
}

// BulkDelete removes multiple folders by ID.
func (r *FolderRepository) BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FolderID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	count := 0
	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		query := `DELETE FROM folders WHERE workspace_id = ? AND id = ?`
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, id := range ids {
			result, err := stmt.ExecContext(ctx, workspaceID.String(), id.String())
			if err != nil {
				return fmt.Errorf("failed to delete folder %s: %w", id, err)
			}
			affected, _ := result.RowsAffected()
			count += int(affected)
		}
		return nil
	})

	return count, err
}

// List lists all folders with pagination.
func (r *FolderRepository) List(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.FolderListOptions) ([]*entity.FolderEntry, error) {
	query := fmt.Sprintf(`
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeFolderSortBy(opts.SortBy, "relative_path"), folderSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// ListByParent lists folders by parent path.
func (r *FolderRepository) ListByParent(ctx context.Context, workspaceID entity.WorkspaceID, parentPath string, opts repository.FolderListOptions) ([]*entity.FolderEntry, error) {
	query := fmt.Sprintf(`
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND parent_path = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeFolderSortBy(opts.SortBy, "relative_path"), folderSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), parentPath, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// ListByDepth lists folders at a specific depth.
func (r *FolderRepository) ListByDepth(ctx context.Context, workspaceID entity.WorkspaceID, depth int, opts repository.FolderListOptions) ([]*entity.FolderEntry, error) {
	query := fmt.Sprintf(`
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND depth = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeFolderSortBy(opts.SortBy, "relative_path"), folderSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), depth, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// ListByNature lists folders by nature.
func (r *FolderRepository) ListByNature(ctx context.Context, workspaceID entity.WorkspaceID, nature entity.FolderNature, opts repository.FolderListOptions) ([]*entity.FolderEntry, error) {
	query := fmt.Sprintf(`
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND json_extract(metadata, '$.ProjectNature') = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeFolderSortBy(opts.SortBy, "relative_path"), folderSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), string(nature), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// GetChildren returns direct child folders.
func (r *FolderRepository) GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentPath string) ([]*entity.FolderEntry, error) {
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND parent_path = ?
		ORDER BY name
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), parentPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// GetDescendants returns all descendant folders.
func (r *FolderRepository) GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, path string) ([]*entity.FolderEntry, error) {
	pattern := path + "/%"
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND (relative_path LIKE ? OR relative_path = ?)
		ORDER BY depth, relative_path
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), pattern, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// GetAncestors returns all ancestor folders.
func (r *FolderRepository) GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, path string) ([]*entity.FolderEntry, error) {
	// Build list of ancestor paths
	ancestors := []string{}
	current := path
	for current != "" && current != "." {
		parent := getParentPath(current)
		if parent != "" {
			ancestors = append(ancestors, parent)
		}
		current = parent
	}

	if len(ancestors) == 0 {
		return []*entity.FolderEntry{}, nil
	}

	// Build IN clause
	placeholders := ""
	args := []interface{}{workspaceID.String()}
	for i, anc := range ancestors {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args = append(args, anc)
	}

	query := fmt.Sprintf(`
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ? AND relative_path IN (%s)
		ORDER BY depth
	`, placeholders)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFolderEntries(rows)
}

// GetStats returns folder statistics.
func (r *FolderRepository) GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.FolderStats, error) {
	stats := &repository.FolderStats{
		NatureCounts: make(map[string]int),
	}

	// Total count and max depth
	query := `SELECT COUNT(*), COALESCE(MAX(depth), 0) FROM folders WHERE workspace_id = ?`
	row := r.conn.QueryRow(ctx, query, workspaceID.String())
	if err := row.Scan(&stats.TotalFolders, &stats.MaxDepth); err != nil {
		return nil, err
	}

	// Aggregated metrics
	query = `
		SELECT
			COALESCE(SUM(json_extract(metrics, '$.TotalFiles')), 0),
			COALESCE(SUM(json_extract(metrics, '$.TotalSize')), 0)
		FROM folders
		WHERE workspace_id = ? AND depth = 1
	`
	row = r.conn.QueryRow(ctx, query, workspaceID.String())
	if err := row.Scan(&stats.TotalFilesInAll, &stats.TotalSizeInAll); err != nil {
		return nil, err
	}

	if stats.TotalFolders > 0 {
		stats.AverageFilesPerDir = float64(stats.TotalFilesInAll) / float64(stats.TotalFolders)
	}

	// Nature counts
	query = `
		SELECT json_extract(metadata, '$.ProjectNature') as nature, COUNT(*)
		FROM folders
		WHERE workspace_id = ? AND nature IS NOT NULL
		GROUP BY nature
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nature string
		var count int
		if err := rows.Scan(&nature, &count); err != nil {
			return nil, err
		}
		stats.NatureCounts[nature] = count
	}

	return stats, nil
}

// Count returns total folder count.
func (r *FolderRepository) Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	query := `SELECT COUNT(*) FROM folders WHERE workspace_id = ?`
	row := r.conn.QueryRow(ctx, query, workspaceID.String())
	var count int
	err := row.Scan(&count)
	return count, err
}

// GetNatureFacet returns folder count by nature.
func (r *FolderRepository) GetNatureFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `
		SELECT COALESCE(json_extract(metadata, '$.ProjectNature'), 'unknown') as nature, COUNT(*)
		FROM folders
		WHERE workspace_id = ?
		GROUP BY nature
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var nature string
		var count int
		if err := rows.Scan(&nature, &count); err != nil {
			return nil, err
		}
		result[nature] = count
	}
	return result, nil
}

// GetDepthFacet returns folder count by depth.
func (r *FolderRepository) GetDepthFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[int]int, error) {
	query := `
		SELECT depth, COUNT(*)
		FROM folders
		WHERE workspace_id = ?
		GROUP BY depth
		ORDER BY depth
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]int)
	for rows.Next() {
		var depth, count int
		if err := rows.Scan(&depth, &count); err != nil {
			return nil, err
		}
		result[depth] = count
	}
	return result, nil
}

// GetDominantTypeFacet returns folder count by dominant file type.
func (r *FolderRepository) GetDominantTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `
		SELECT COALESCE(json_extract(metadata, '$.DominantFileType'), 'unknown') as dtype, COUNT(*)
		FROM folders
		WHERE workspace_id = ?
		GROUP BY dtype
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var dtype string
		var count int
		if err := rows.Scan(&dtype, &count); err != nil {
			return nil, err
		}
		result[dtype] = count
	}
	return result, nil
}

// GetFileSizeRangeFacet returns folder count by total size ranges.
func (r *FolderRepository) GetFileSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]repository.NumericRangeCount, error) {
	query := `
		SELECT
			CASE
				WHEN json_extract(metrics, '$.TotalSize') < 1024 THEN '< 1 KB'
				WHEN json_extract(metrics, '$.TotalSize') < 1048576 THEN '1 KB - 1 MB'
				WHEN json_extract(metrics, '$.TotalSize') < 10485760 THEN '1 MB - 10 MB'
				WHEN json_extract(metrics, '$.TotalSize') < 104857600 THEN '10 MB - 100 MB'
				WHEN json_extract(metrics, '$.TotalSize') < 1073741824 THEN '100 MB - 1 GB'
				ELSE '> 1 GB'
			END as range_label,
			COUNT(*)
		FROM folders
		WHERE workspace_id = ?
		GROUP BY range_label
		ORDER BY
			CASE range_label
				WHEN '< 1 KB' THEN 1
				WHEN '1 KB - 1 MB' THEN 2
				WHEN '1 MB - 10 MB' THEN 3
				WHEN '10 MB - 100 MB' THEN 4
				WHEN '100 MB - 1 GB' THEN 5
				ELSE 6
			END
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []repository.NumericRangeCount
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, err
		}
		result = append(result, repository.NumericRangeCount{Label: label, Count: count})
	}
	return result, nil
}

// GetFileCountRangeFacet returns folder count by file count ranges.
func (r *FolderRepository) GetFileCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]repository.NumericRangeCount, error) {
	query := `
		SELECT
			CASE
				WHEN json_extract(metrics, '$.TotalFiles') = 0 THEN 'Empty'
				WHEN json_extract(metrics, '$.TotalFiles') <= 5 THEN '1-5 files'
				WHEN json_extract(metrics, '$.TotalFiles') <= 20 THEN '6-20 files'
				WHEN json_extract(metrics, '$.TotalFiles') <= 50 THEN '21-50 files'
				WHEN json_extract(metrics, '$.TotalFiles') <= 100 THEN '51-100 files'
				ELSE '100+ files'
			END as range_label,
			COUNT(*)
		FROM folders
		WHERE workspace_id = ?
		GROUP BY range_label
		ORDER BY
			CASE range_label
				WHEN 'Empty' THEN 1
				WHEN '1-5 files' THEN 2
				WHEN '6-20 files' THEN 3
				WHEN '21-50 files' THEN 4
				WHEN '51-100 files' THEN 5
				ELSE 6
			END
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []repository.NumericRangeCount
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, err
		}
		result = append(result, repository.NumericRangeCount{Label: label, Count: count})
	}
	return result, nil
}

// DeleteAll removes all folders for a workspace.
func (r *FolderRepository) DeleteAll(ctx context.Context, workspaceID entity.WorkspaceID) error {
	query := `DELETE FROM folders WHERE workspace_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String())
	return err
}

// scanFolderEntry scans a single folder entry from a row.
func (r *FolderRepository) scanFolderEntry(row *sql.Row) (*entity.FolderEntry, error) {
	var id, relativePath, name, parentPath string
	var depth int
	var metricsJSON, metadataJSON []byte
	var createdAt, updatedAt int64

	err := row.Scan(&id, &relativePath, &name, &parentPath, &depth, &metricsJSON, &metadataJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	folder := &entity.FolderEntry{
		ID:           entity.FolderID(id),
		RelativePath: relativePath,
		Name:         name,
		ParentPath:   parentPath,
		Depth:        depth,
		CreatedAt:    time.UnixMilli(createdAt),
		UpdatedAt:    time.UnixMilli(updatedAt),
	}

	if len(metricsJSON) > 0 {
		folder.Metrics = &entity.FolderMetrics{}
		if err := json.Unmarshal(metricsJSON, folder.Metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		folder.Metadata = &entity.FolderMetadata{}
		if err := json.Unmarshal(metadataJSON, folder.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return folder, nil
}

// scanFolderEntries scans multiple folder entries from rows.
func (r *FolderRepository) scanFolderEntries(rows *sql.Rows) ([]*entity.FolderEntry, error) {
	var folders []*entity.FolderEntry

	for rows.Next() {
		var id, relativePath, name, parentPath string
		var depth int
		var metricsJSON, metadataJSON []byte
		var createdAt, updatedAt int64

		err := rows.Scan(&id, &relativePath, &name, &parentPath, &depth, &metricsJSON, &metadataJSON, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		folder := &entity.FolderEntry{
			ID:           entity.FolderID(id),
			RelativePath: relativePath,
			Name:         name,
			ParentPath:   parentPath,
			Depth:        depth,
			CreatedAt:    time.UnixMilli(createdAt),
			UpdatedAt:    time.UnixMilli(updatedAt),
		}

		if len(metricsJSON) > 0 {
			folder.Metrics = &entity.FolderMetrics{}
			if err := json.Unmarshal(metricsJSON, folder.Metrics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
			}
		}

		if len(metadataJSON) > 0 {
			folder.Metadata = &entity.FolderMetadata{}
			if err := json.Unmarshal(metadataJSON, folder.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		folders = append(folders, folder)
	}

	return folders, rows.Err()
}

// getParentPath returns the parent path of a given path.
func getParentPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}

// sanitizeFolderSortBy returns a safe column name for sorting folders.
func sanitizeFolderSortBy(sortBy, defaultCol string) string {
	allowed := map[string]bool{
		"relative_path": true,
		"name":          true,
		"depth":         true,
		"created_at":    true,
		"updated_at":    true,
	}
	if allowed[sortBy] {
		return sortBy
	}
	return defaultCol
}

// folderSortDirection returns ASC or DESC based on the boolean.
func folderSortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}
