package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// FileRepository implements repository.FileRepository using SQLite.
type FileRepository struct {
	conn *Connection
}

// NewFileRepository creates a new SQLite file repository.
func NewFileRepository(conn *Connection) *FileRepository {
	return &FileRepository{conn: conn}
}

// GetByID retrieves a file by ID.
func (r *FileRepository) GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) (*entity.FileEntry, error) {
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND id = ?
	`

	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return r.scanFileEntry(row)
}

// GetByPath retrieves a file by relative path.
// Normalizes the path to handle different path separator formats (Windows vs Unix)
// and URL-encoded paths (e.g., %20 for spaces).
func (r *FileRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileEntry, error) {
	// Decode URL encoding if present (e.g., %20 -> space)
	// This handles cases where the frontend sends URL-encoded paths
	decodedPath, err := url.PathUnescape(relativePath)
	if err != nil {
		// If decoding fails, use original path
		decodedPath = relativePath
	}
	
	// Normalize path separators to forward slashes for consistent matching
	// This handles cases where documents have paths with different separators
	normalizedPath := filepath.ToSlash(decodedPath)
	
	// Try exact match first (most common case)
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND relative_path = ?
	`

	// Try with normalized decoded path first
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), normalizedPath)
	entry, err := r.scanFileEntry(row)
	if err == nil && entry != nil {
		return entry, nil
	}
	
	// If not found, try with original decoded path (without normalization)
	if normalizedPath != decodedPath {
		row = r.conn.QueryRow(ctx, query, workspaceID.String(), decodedPath)
		entry, err = r.scanFileEntry(row)
		if err == nil && entry != nil {
			return entry, nil
		}
	}
	
	// If still not found, try with original path (in case it wasn't URL-encoded)
	if decodedPath != relativePath {
		normalizedOriginal := filepath.ToSlash(relativePath)
		row = r.conn.QueryRow(ctx, query, workspaceID.String(), normalizedOriginal)
		entry, err = r.scanFileEntry(row)
		if err == nil && entry != nil {
			return entry, nil
		}
		if normalizedOriginal != relativePath {
			row = r.conn.QueryRow(ctx, query, workspaceID.String(), relativePath)
			entry, err = r.scanFileEntry(row)
			if err == nil && entry != nil {
				return entry, nil
			}
		}
	}
	
	return entry, err
}

// Upsert inserts or updates a file entry.
func (r *FileRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, file *entity.FileEntry) error {
	var enhancedJSON []byte
	if file.Enhanced != nil {
		var err error
		enhancedJSON, err = json.Marshal(file.Enhanced)
		if err != nil {
			return fmt.Errorf("failed to marshal enhanced metadata: %w", err)
		}
	}

	// Serialize path components to JSON
	var pathComponentsJSON []byte
	if file.Enhanced != nil && len(file.Enhanced.PathComponents) > 0 {
		pathComponentsJSON, _ = json.Marshal(file.Enhanced.PathComponents)
	}

	query := `
		INSERT INTO files (
			id, workspace_id, relative_path, absolute_path, filename, extension,
			file_size, last_modified, created_at, enhanced,
			indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
			accessed_at, changed_at, backup_at,
			path_components, path_pattern,
			file_hash_md5, file_hash_sha256, file_hash_sha512
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, id) DO UPDATE SET
			relative_path = excluded.relative_path,
			absolute_path = excluded.absolute_path,
			filename = excluded.filename,
			extension = excluded.extension,
			file_size = excluded.file_size,
			last_modified = excluded.last_modified,
			enhanced = excluded.enhanced,
			indexed_basic = excluded.indexed_basic,
			indexed_mime = excluded.indexed_mime,
			indexed_code = excluded.indexed_code,
			indexed_document = excluded.indexed_document,
			indexed_mirror = excluded.indexed_mirror,
			accessed_at = excluded.accessed_at,
			changed_at = excluded.changed_at,
			backup_at = excluded.backup_at,
			path_components = excluded.path_components,
			path_pattern = excluded.path_pattern,
			file_hash_md5 = excluded.file_hash_md5,
			file_hash_sha256 = excluded.file_hash_sha256,
			file_hash_sha512 = excluded.file_hash_sha512
	`

	var indexedBasic, indexedMime, indexedCode, indexedDoc, indexedMirror int
	if file.Enhanced != nil {
		if file.Enhanced.IndexedState.Basic {
			indexedBasic = 1
		}
		if file.Enhanced.IndexedState.Mime {
			indexedMime = 1
		}
		if file.Enhanced.IndexedState.Code {
			indexedCode = 1
		}
		if file.Enhanced.IndexedState.Document {
			indexedDoc = 1
		}
		if file.Enhanced.IndexedState.Mirror {
			indexedMirror = 1
		}
	}

	// Extract timestamps from EnhancedMetadata.Stats if available
	var accessedAt, changedAt, backupAt *int64
	if file.Enhanced != nil && file.Enhanced.Stats != nil {
		if !file.Enhanced.Stats.Accessed.IsZero() {
			accessed := file.Enhanced.Stats.Accessed.UnixMilli()
			accessedAt = &accessed
		}
		if file.Enhanced.Stats.Changed != nil {
			changed := file.Enhanced.Stats.Changed.UnixMilli()
			changedAt = &changed
		}
		if file.Enhanced.Stats.Backup != nil {
			backup := file.Enhanced.Stats.Backup.UnixMilli()
			backupAt = &backup
		}
	}

	_, err := r.conn.Exec(ctx, query,
		file.ID.String(),
		workspaceID.String(),
		file.RelativePath,
		file.AbsolutePath,
		file.Filename,
		file.Extension,
		file.FileSize,
		file.LastModified.UnixMilli(),
		file.CreatedAt.UnixMilli(),
		enhancedJSON,
		indexedBasic,
		indexedMime,
		indexedCode,
		indexedDoc,
		indexedMirror,
		accessedAt,
		changedAt,
		backupAt,
		pathComponentsJSON,
		func() string {
			if file.Enhanced != nil {
				return file.Enhanced.PathPattern
			}
			return ""
		}(),
		func() *string {
			if file.Enhanced != nil && file.Enhanced.FileHash != nil {
				return &file.Enhanced.FileHash.MD5
			}
			return nil
		}(),
		func() *string {
			if file.Enhanced != nil && file.Enhanced.FileHash != nil {
				return &file.Enhanced.FileHash.SHA256
			}
			return nil
		}(),
		func() *string {
			if file.Enhanced != nil && file.Enhanced.FileHash != nil {
				return &file.Enhanced.FileHash.SHA512
			}
			return nil
		}(),
	)

	return err
}

// Delete removes a file entry.
func (r *FileRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID) error {
	query := `DELETE FROM files WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), id.String())
	return err
}

// BulkUpsert inserts or updates multiple file entries.
func (r *FileRepository) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, files []*entity.FileEntry) (int, error) {
	count := 0

	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO files (
				id, workspace_id, relative_path, absolute_path, filename, extension,
				file_size, last_modified, created_at, enhanced,
				indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
				accessed_at, changed_at, backup_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, id) DO UPDATE SET
				relative_path = excluded.relative_path,
				absolute_path = excluded.absolute_path,
				filename = excluded.filename,
				extension = excluded.extension,
				file_size = excluded.file_size,
				last_modified = excluded.last_modified,
				enhanced = COALESCE(excluded.enhanced, files.enhanced),
				accessed_at = COALESCE(excluded.accessed_at, files.accessed_at),
				changed_at = COALESCE(excluded.changed_at, files.changed_at),
				backup_at = COALESCE(excluded.backup_at, files.backup_at)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, file := range files {
			var enhancedJSON []byte
			if file.Enhanced != nil {
				enhancedJSON, _ = json.Marshal(file.Enhanced)
			}

			// Extract timestamps from EnhancedMetadata.Stats if available
			var accessedAt, changedAt, backupAt interface{}
			if file.Enhanced != nil && file.Enhanced.Stats != nil {
				if !file.Enhanced.Stats.Accessed.IsZero() {
					accessedAt = file.Enhanced.Stats.Accessed.UnixMilli()
				}
				if file.Enhanced.Stats.Changed != nil {
					changedAt = file.Enhanced.Stats.Changed.UnixMilli()
				}
				if file.Enhanced.Stats.Backup != nil {
					backupAt = file.Enhanced.Stats.Backup.UnixMilli()
				}
			}

			_, err := stmt.ExecContext(ctx,
				file.ID.String(),
				workspaceID.String(),
				file.RelativePath,
				file.AbsolutePath,
				file.Filename,
				file.Extension,
				file.FileSize,
				file.LastModified.UnixMilli(),
				file.CreatedAt.UnixMilli(),
				enhancedJSON,
				0, 0, 0, 0, 0,
				accessedAt,
				changedAt,
				backupAt,
			)
			if err != nil {
				return err
			}
			count++
		}

		return nil
	})

	return count, err
}

// BulkDelete removes multiple file entries.
func (r *FileRepository) BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.FileID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = workspaceID.String()

	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id.String()
	}

	query := fmt.Sprintf(`DELETE FROM files WHERE workspace_id = ? AND id IN (%s)`,
		joinStrings(placeholders, ","))

	result, err := r.conn.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// List lists files with pagination.
func (r *FileRepository) List(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ?
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	return r.queryFiles(ctx, query, workspaceID.String(), opts.Limit, opts.Offset)
}

// ListByExtension lists files by extension.
func (r *FileRepository) ListByExtension(ctx context.Context, workspaceID entity.WorkspaceID, ext string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND extension = ?
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	return r.queryFiles(ctx, query, workspaceID.String(), ext, opts.Limit, opts.Offset)
}

// ListByFolder lists files in a folder.
func (r *FileRepository) ListByFolder(ctx context.Context, workspaceID entity.WorkspaceID, folder string, recursive bool, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	var query string
	var pattern string

	if recursive {
		pattern = folder + "/%"
		query = `
			SELECT id, relative_path, absolute_path, filename, extension, file_size,
			       last_modified, created_at, enhanced,
			       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
			       accessed_at, changed_at, backup_at,
			       path_components, path_pattern,
			       file_hash_md5, file_hash_sha256, file_hash_sha512
			FROM files
			WHERE workspace_id = ? AND (relative_path LIKE ? OR relative_path LIKE ?)
			ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
			LIMIT ? OFFSET ?
		`
		return r.queryFiles(ctx, query, workspaceID.String(), folder+"/%", folder, opts.Limit, opts.Offset)
	}

	pattern = folder + "/%"
	query = `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND relative_path LIKE ?
		      AND relative_path NOT LIKE ?
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	return r.queryFiles(ctx, query, workspaceID.String(), pattern, folder+"/%/%", opts.Limit, opts.Offset)
}

// ListByDateRange lists files modified within a date range.
func (r *FileRepository) ListByDateRange(ctx context.Context, workspaceID entity.WorkspaceID, start, end time.Time, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND last_modified BETWEEN ? AND ?
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	return r.queryFiles(ctx, query, workspaceID.String(), start.UnixMilli(), end.UnixMilli(), opts.Limit, opts.Offset)
}

// ListBySizeRange lists files within a size range.
func (r *FileRepository) ListBySizeRange(ctx context.Context, workspaceID entity.WorkspaceID, minSize, maxSize int64, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	query := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND file_size BETWEEN ? AND ?
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	return r.queryFiles(ctx, query, workspaceID.String(), minSize, maxSize, opts.Limit, opts.Offset)
}

// ListByContentType lists files by content type category.
func (r *FileRepository) ListByContentType(ctx context.Context, workspaceID entity.WorkspaceID, category string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	// This would require querying enhanced metadata JSON
	// For now, return empty list
	return []*entity.FileEntry{}, nil
}

// Search searches files by pattern.
func (r *FileRepository) Search(ctx context.Context, workspaceID entity.WorkspaceID, query string, opts repository.FileListOptions) ([]*entity.FileEntry, error) {
	sqlQuery := `
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND (
			relative_path LIKE ? OR filename LIKE ?
		)
		ORDER BY ` + sanitizeSortColumn(opts.SortBy) + ` ` + sortDirection(opts.SortDesc) + `
		LIMIT ? OFFSET ?
	`

	pattern := "%" + query + "%"
	return r.queryFiles(ctx, sqlQuery, workspaceID.String(), pattern, pattern, opts.Limit, opts.Offset)
}

// GetStats returns file statistics.
func (r *FileRepository) GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.FileStats, error) {
	stats := &repository.FileStats{
		ExtensionCounts: make(map[string]int),
		FolderCounts:    make(map[string]int),
	}

	// Get totals
	row := r.conn.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(file_size), 0) FROM files WHERE workspace_id = ?`,
		workspaceID.String())
	if err := row.Scan(&stats.TotalFiles, &stats.TotalSize); err != nil {
		return nil, err
	}

	// Get extension counts
	rows, err := r.conn.Query(ctx,
		`SELECT extension, COUNT(*) FROM files WHERE workspace_id = ? GROUP BY extension`,
		workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ext string
		var count int
		if err := rows.Scan(&ext, &count); err != nil {
			return nil, err
		}
		stats.ExtensionCounts[ext] = count
	}

	return stats, nil
}

// Count returns the number of files.
func (r *FileRepository) Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	row := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM files WHERE workspace_id = ?`,
		workspaceID.String())

	var count int
	err := row.Scan(&count)
	return count, err
}

// GetUnindexedFiles returns files missing a specific indexing phase.
func (r *FileRepository) GetUnindexedFiles(ctx context.Context, workspaceID entity.WorkspaceID, phase string, limit int) ([]*entity.FileEntry, error) {
	column := "indexed_" + phase
	query := fmt.Sprintf(`
		SELECT id, relative_path, absolute_path, filename, extension, file_size,
		       last_modified, created_at, enhanced,
		       indexed_basic, indexed_mime, indexed_code, indexed_document, indexed_mirror,
		       accessed_at, changed_at, backup_at,
		       path_components, path_pattern,
		       file_hash_md5, file_hash_sha256, file_hash_sha512
		FROM files
		WHERE workspace_id = ? AND %s = 0
		LIMIT ?
	`, column)

	return r.queryFiles(ctx, query, workspaceID.String(), limit)
}

// UpdateIndexedState updates the indexing state for a file.
func (r *FileRepository) UpdateIndexedState(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, state entity.IndexedState) error {
	query := `
		UPDATE files SET
			indexed_basic = ?,
			indexed_mime = ?,
			indexed_code = ?,
			indexed_document = ?,
			indexed_mirror = ?
		WHERE workspace_id = ? AND id = ?
	`

	_, err := r.conn.Exec(ctx, query,
		boolToInt(state.Basic),
		boolToInt(state.Mime),
		boolToInt(state.Code),
		boolToInt(state.Document),
		boolToInt(state.Mirror),
		workspaceID.String(),
		id.String(),
	)

	return err
}

// UpdateEnhancedMetadata updates the enhanced metadata for a file.
func (r *FileRepository) UpdateEnhancedMetadata(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FileID, enhanced *entity.EnhancedMetadata) error {
	var enhancedJSON []byte
	if enhanced != nil {
		var err error
		enhancedJSON, err = json.Marshal(enhanced)
		if err != nil {
			return err
		}
	}

	query := `
		UPDATE files SET
			enhanced = ?,
			indexed_basic = ?,
			indexed_mime = ?,
			indexed_code = ?,
			indexed_document = ?,
			indexed_mirror = ?
		WHERE workspace_id = ? AND id = ?
	`

	var state entity.IndexedState
	if enhanced != nil {
		state = enhanced.IndexedState
	}

	_, err := r.conn.Exec(ctx, query,
		enhancedJSON,
		boolToInt(state.Basic),
		boolToInt(state.Mime),
		boolToInt(state.Code),
		boolToInt(state.Document),
		boolToInt(state.Mirror),
		workspaceID.String(),
		id.String(),
	)

	return err
}

// Helper functions

func (r *FileRepository) scanFileEntry(row *sql.Row) (*entity.FileEntry, error) {
	var (
		id                                                                string
		relPath, absPath, filename, ext                                   string
		fileSize, lastModified, createdAt                                 int64
		enhancedJSON                                                      []byte
		indexedBasic, indexedMime, indexedCode, indexedDoc, indexedMirror int
		accessedAt, changedAt, backupAt                                   sql.NullInt64
		pathComponentsJSON                                                sql.NullString
		pathPattern                                                       sql.NullString
		hashMD5, hashSHA256, hashSHA512                                   sql.NullString
	)

	err := row.Scan(
		&id, &relPath, &absPath, &filename, &ext, &fileSize,
		&lastModified, &createdAt, &enhancedJSON,
		&indexedBasic, &indexedMime, &indexedCode, &indexedDoc, &indexedMirror,
		&accessedAt, &changedAt, &backupAt,
		&pathComponentsJSON, &pathPattern,
		&hashMD5, &hashSHA256, &hashSHA512,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	entry := &entity.FileEntry{
		ID:           entity.FileID(id),
		RelativePath: relPath,
		AbsolutePath: absPath,
		Filename:     filename,
		Extension:    ext,
		FileSize:     fileSize,
		LastModified: time.UnixMilli(lastModified),
		CreatedAt:    time.UnixMilli(createdAt),
	}

	if len(enhancedJSON) > 0 {
		entry.Enhanced = &entity.EnhancedMetadata{}
		_ = json.Unmarshal(enhancedJSON, entry.Enhanced)
	}

	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}

	// Ensure Stats is initialized
	if entry.Enhanced.Stats == nil {
		entry.Enhanced.Stats = &entity.FileStats{}
	}

	// Populate timestamps from database columns if available
	if accessedAt.Valid {
		entry.Enhanced.Stats.Accessed = time.UnixMilli(accessedAt.Int64)
	}
	if changedAt.Valid {
		changed := time.UnixMilli(changedAt.Int64)
		entry.Enhanced.Stats.Changed = &changed
	}
	if backupAt.Valid {
		backup := time.UnixMilli(backupAt.Int64)
		entry.Enhanced.Stats.Backup = &backup
	}

	// Populate path components and pattern from database columns
	if pathComponentsJSON.Valid && len(pathComponentsJSON.String) > 0 {
		var components []string
		if err := json.Unmarshal([]byte(pathComponentsJSON.String), &components); err == nil {
			entry.Enhanced.PathComponents = components
		}
	}
	if pathPattern.Valid {
		entry.Enhanced.PathPattern = pathPattern.String
	}

	// Populate file hashes from database columns
	if hashMD5.Valid || hashSHA256.Valid || hashSHA512.Valid {
		if entry.Enhanced.FileHash == nil {
			entry.Enhanced.FileHash = &entity.FileHash{}
		}
		if hashMD5.Valid {
			entry.Enhanced.FileHash.MD5 = hashMD5.String
		}
		if hashSHA256.Valid {
			entry.Enhanced.FileHash.SHA256 = hashSHA256.String
		}
		if hashSHA512.Valid {
			entry.Enhanced.FileHash.SHA512 = hashSHA512.String
		}
	}

	entry.Enhanced.IndexedState = entity.IndexedState{
		Basic:    indexedBasic == 1,
		Mime:     indexedMime == 1,
		Code:     indexedCode == 1,
		Document: indexedDoc == 1,
		Mirror:   indexedMirror == 1,
	}

	return entry, nil
}

func (r *FileRepository) queryFiles(ctx context.Context, query string, args ...interface{}) ([]*entity.FileEntry, error) {
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*entity.FileEntry
	for rows.Next() {
		var (
			id                                                                string
			relPath, absPath, filename, ext                                   string
			fileSize, lastModified, createdAt                                 int64
			enhancedJSON                                                      []byte
			indexedBasic, indexedMime, indexedCode, indexedDoc, indexedMirror int
			accessedAt, changedAt, backupAt                                   sql.NullInt64
			pathComponentsJSON                                                sql.NullString
			pathPattern                                                       sql.NullString
			hashMD5, hashSHA256, hashSHA512                                   sql.NullString
		)

		err := rows.Scan(
			&id, &relPath, &absPath, &filename, &ext, &fileSize,
			&lastModified, &createdAt, &enhancedJSON,
			&indexedBasic, &indexedMime, &indexedCode, &indexedDoc, &indexedMirror,
			&accessedAt, &changedAt, &backupAt,
			&pathComponentsJSON, &pathPattern,
			&hashMD5, &hashSHA256, &hashSHA512,
		)
		if err != nil {
			return nil, err
		}

		entry := &entity.FileEntry{
			ID:           entity.FileID(id),
			RelativePath: relPath,
			AbsolutePath: absPath,
			Filename:     filename,
			Extension:    ext,
			FileSize:     fileSize,
			LastModified: time.UnixMilli(lastModified),
			CreatedAt:    time.UnixMilli(createdAt),
		}

		if len(enhancedJSON) > 0 {
			entry.Enhanced = &entity.EnhancedMetadata{}
			_ = json.Unmarshal(enhancedJSON, entry.Enhanced)
		}

		if entry.Enhanced == nil {
			entry.Enhanced = &entity.EnhancedMetadata{}
		}

		// Ensure Stats is initialized
		if entry.Enhanced.Stats == nil {
			entry.Enhanced.Stats = &entity.FileStats{}
		}

		// Populate timestamps from database columns if available
		if accessedAt.Valid {
			entry.Enhanced.Stats.Accessed = time.UnixMilli(accessedAt.Int64)
		}
		if changedAt.Valid {
			changed := time.UnixMilli(changedAt.Int64)
			entry.Enhanced.Stats.Changed = &changed
		}
		if backupAt.Valid {
			backup := time.UnixMilli(backupAt.Int64)
			entry.Enhanced.Stats.Backup = &backup
		}

		// Populate path components and pattern from database columns
		if pathComponentsJSON.Valid && len(pathComponentsJSON.String) > 0 {
			var components []string
			if err := json.Unmarshal([]byte(pathComponentsJSON.String), &components); err == nil {
				entry.Enhanced.PathComponents = components
			}
		}
		if pathPattern.Valid {
			entry.Enhanced.PathPattern = pathPattern.String
		}

		// Populate file hashes from database columns
		if hashMD5.Valid || hashSHA256.Valid || hashSHA512.Valid {
			if entry.Enhanced.FileHash == nil {
				entry.Enhanced.FileHash = &entity.FileHash{}
			}
			if hashMD5.Valid {
				entry.Enhanced.FileHash.MD5 = hashMD5.String
			}
			if hashSHA256.Valid {
				entry.Enhanced.FileHash.SHA256 = hashSHA256.String
			}
			if hashSHA512.Valid {
				entry.Enhanced.FileHash.SHA512 = hashSHA512.String
			}
		}

		entry.Enhanced.IndexedState = entity.IndexedState{
			Basic:    indexedBasic == 1,
			Mime:     indexedMime == 1,
			Code:     indexedCode == 1,
			Document: indexedDoc == 1,
			Mirror:   indexedMirror == 1,
		}

		files = append(files, entry)
	}

	return files, nil
}

func sanitizeSortColumn(col string) string {
	allowed := map[string]bool{
		"relative_path": true,
		"filename":      true,
		"extension":     true,
		"file_size":     true,
		"last_modified": true,
		"created_at":    true,
	}
	if allowed[col] {
		return col
	}
	return "relative_path"
}

func sortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetExtensionFacet returns extension facet counts.
func (r *FileRepository) GetExtensionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `SELECT extension, COUNT(*) FROM files WHERE workspace_id = ?`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY extension ORDER BY COUNT(*) DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		// Check for cancellation during row processing
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		var ext string
		var count int
		if err := rows.Scan(&ext, &count); err != nil {
			return nil, err
		}
		counts[ext] = count
	}

	return counts, nil
}

// GetTypeFacet returns file type facet counts.
// Uses MIME category from enhanced metadata if available, otherwise infers from extension.
func (r *FileRepository) GetTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	// Try to get type from MIME category first, fall back to extension-based inference
	query := `
		SELECT 
			CASE 
				WHEN json_extract(enhanced, '$.MimeType.Category') IS NOT NULL 
					AND json_extract(enhanced, '$.MimeType.Category') != ''
				THEN json_extract(enhanced, '$.MimeType.Category')
				ELSE NULL
			END as mime_category,
			extension,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY mime_category, extension ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		// Check for cancellation during row processing
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		var mimeCategory sql.NullString
		var extension string
		var count int
		if err := rows.Scan(&mimeCategory, &extension, &count); err != nil {
			return nil, err
		}

		var fileType string
		
		// List of extensions that should always use extension-based inference
		// (ignore potentially incorrect MIME categories)
		extLower := strings.ToLower(strings.TrimPrefix(extension, "."))
		alwaysInferFromExtension := map[string]bool{
			"csv": true, "tsv": true,
			"db": true, "sqlite": true, "sqlite3": true, "thumbs": true,
			"ds_store": true,
		}
		
		if alwaysInferFromExtension[extLower] {
			// Always infer from extension for these files (ignore MIME category)
			fileType = inferFileTypeFromExtension(extension)
		} else if mimeCategory.Valid && mimeCategory.String != "" {
			// Use MIME category for other files
			fileType = mimeCategory.String
		} else {
			// Infer from extension if no MIME category
			fileType = inferFileTypeFromExtension(extension)
		}

		if fileType != "" {
			counts[fileType] += count
		}
	}

	return counts, nil
}

// inferFileTypeFromExtension infers semantic file type from extension.
// This matches the frontend normalizeType function logic.
func inferFileTypeFromExtension(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "" {
		return "unknown"
	}

	typeMap := map[string]string{
		// Code languages
		"ts": "typescript", "tsx": "typescript-react",
		"js": "javascript", "jsx": "javascript-react",
		"py": "python", "go": "go", "rs": "rust",
		"java": "java", "cpp": "cpp", "c": "c",
		"cs": "csharp", "php": "php", "rb": "ruby",
		"swift": "swift", "kt": "kotlin", "scala": "scala",
		"sh": "shell", "bash": "bash", "zsh": "zsh",
		"fish": "fish", "ps1": "powershell", "bat": "batch",
		"sql": "sql",
		// Markup & Config
		"md": "markdown", "txt": "text", "json": "json",
		"xml": "xml", "yaml": "yaml", "yml": "yaml",
		"html": "html", "css": "css", "scss": "scss",
		"sass": "sass", "less": "less",
		// Documents
		"pdf": "pdf", "doc": "word", "docx": "word",
		"xls": "excel", "xlsx": "excel",
		"ppt": "powerpoint", "pptx": "powerpoint",
		// Data files
		"csv": "text", "tsv": "text",
		// Database files
		"db": "database", "sqlite": "database", "sqlite3": "database",
		"thumbs": "database", // Windows thumbnail cache
		// System files
		"ds_store": "system",
		// Images
		"png": "image", "jpg": "image", "jpeg": "image",
		"gif": "image", "svg": "image", "webp": "image",
		// Media
		"mp4": "video", "avi": "video", "mov": "video",
		"mp3": "audio", "wav": "audio",
		// Archives
		"zip": "archive", "tar": "archive", "gz": "archive",
		"7z": "archive",
	}

	if t, ok := typeMap[ext]; ok {
		return t
	}
	return ext // Return extension as-is if not in map
}

// GetSizeRangeFacet returns size range facet counts.
func (r *FileRepository) GetSizeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	const (
		KB = 1024
		MB = 1024 * KB
	)

	ranges := []struct {
		label string
		min   int64
		max   int64
	}{
		{"Tiny (< 1 KB)", 0, KB},
		{"Small (1 KB - 100 KB)", KB, 100 * KB},
		{"Medium (100 KB - 1 MB)", 100 * KB, MB},
		{"Large (1 MB - 10 MB)", MB, 10 * MB},
		{"Huge (> 10 MB)", 10 * MB, 0}, // 0 means unbounded
	}

	query := `SELECT file_size FROM files WHERE workspace_id = ?`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var size int64
		if err := rows.Scan(&size); err != nil {
			return nil, err
		}

		for i, rng := range ranges {
			if rng.max == 0 {
				// Unbounded upper limit
				if size >= rng.min {
					counts[i]++
					break
				}
			} else {
				if size >= rng.min && size < rng.max {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   float64(rng.min),
			Max:   float64(rng.max),
			Count: counts[i],
		}
	}

	return result, nil
}

// GetDateRangeFacet returns last modified date range facet counts.
func (r *FileRepository) GetDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)
	yearAgo := today.AddDate(-1, 0, 0)

	ranges := []struct {
		label string
		start time.Time
		end   time.Time
	}{
		{"Today", today, time.Time{}}, // Zero time means unbounded
		{"Yesterday", yesterday, today},
		{"This Week", weekAgo, yesterday},
		{"This Month", monthAgo, weekAgo},
		{"This Year", yearAgo, monthAgo},
		{"Older", time.Time{}, yearAgo}, // Zero time means unbounded
	}

	query := `SELECT last_modified FROM files WHERE workspace_id = ?`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var lastModified int64
		if err := rows.Scan(&lastModified); err != nil {
			return nil, err
		}

		fileTime := time.UnixMilli(lastModified)
		fileDate := time.Date(fileTime.Year(), fileTime.Month(), fileTime.Day(), 0, 0, 0, 0, fileTime.Location())

		for i, rng := range ranges {
			if rng.start.IsZero() {
				// Unbounded start - check end only
				if rng.end.IsZero() {
					// Both unbounded (shouldn't happen, but handle it)
					counts[i]++
					break
				} else {
					if fileDate.Before(rng.end) {
						counts[i]++
						break
					}
				}
			} else if rng.end.IsZero() {
				// Unbounded end - check start only
				if !fileDate.Before(rng.start) {
					counts[i]++
					break
				}
			} else {
				// Both bounded
				if !fileDate.Before(rng.start) && fileDate.Before(rng.end) {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.DateRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.DateRangeCount{
			Label: rng.label,
			Start: rng.start,
			End:   rng.end,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetCreatedDateRangeFacet returns created date range facet counts.
func (r *FileRepository) GetCreatedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	// Similar to GetDateRangeFacet but uses created_at instead of last_modified
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)
	yearAgo := today.AddDate(-1, 0, 0)

	ranges := []struct {
		label string
		start time.Time
		end   time.Time
	}{
		{"Today", today, time.Time{}},
		{"Yesterday", yesterday, today},
		{"This Week", weekAgo, yesterday},
		{"This Month", monthAgo, weekAgo},
		{"This Year", yearAgo, monthAgo},
		{"Older", time.Time{}, yearAgo},
	}

	query := `SELECT created_at FROM files WHERE workspace_id = ?`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var createdAt int64
		if err := rows.Scan(&createdAt); err != nil {
			return nil, err
		}

		fileTime := time.UnixMilli(createdAt)
		fileDate := time.Date(fileTime.Year(), fileTime.Month(), fileTime.Day(), 0, 0, 0, 0, fileTime.Location())

		for i, rng := range ranges {
			if rng.start.IsZero() {
				if rng.end.IsZero() {
					counts[i]++
					break
				} else {
					if fileDate.Before(rng.end) {
						counts[i]++
						break
					}
				}
			} else if rng.end.IsZero() {
				if !fileDate.Before(rng.start) {
					counts[i]++
					break
				}
			} else {
				if !fileDate.Before(rng.start) && fileDate.Before(rng.end) {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.DateRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.DateRangeCount{
			Label: rng.label,
			Start: rng.start,
			End:   rng.end,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetAccessedDateRangeFacet returns accessed date range facet counts.
func (r *FileRepository) GetAccessedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return r.getDateRangeFacetForColumn(ctx, workspaceID, fileIDs, "accessed_at")
}

// GetChangedDateRangeFacet returns changed date range facet counts.
func (r *FileRepository) GetChangedDateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error) {
	return r.getDateRangeFacetForColumn(ctx, workspaceID, fileIDs, "changed_at")
}

// getDateRangeFacetForColumn is a helper that implements date range faceting for any timestamp column.
func (r *FileRepository) getDateRangeFacetForColumn(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, column string) ([]repository.DateRangeCount, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)
	yearAgo := today.AddDate(-1, 0, 0)

	ranges := []struct {
		label string
		start time.Time
		end   time.Time
	}{
		{"Today", today, time.Time{}},
		{"Yesterday", yesterday, today},
		{"This Week", weekAgo, yesterday},
		{"This Month", monthAgo, weekAgo},
		{"This Year", yearAgo, monthAgo},
		{"Older", time.Time{}, yearAgo},
	}

	query := fmt.Sprintf(`SELECT %s FROM files WHERE workspace_id = ? AND %s IS NOT NULL`, column, column)
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var timestamp sql.NullInt64
		if err := rows.Scan(&timestamp); err != nil {
			return nil, err
		}
		if !timestamp.Valid {
			continue
		}

		fileTime := time.UnixMilli(timestamp.Int64)
		fileDate := time.Date(fileTime.Year(), fileTime.Month(), fileTime.Day(), 0, 0, 0, 0, fileTime.Location())

		for i, rng := range ranges {
			if rng.start.IsZero() {
				if rng.end.IsZero() {
					counts[i]++
					break
				} else {
					if fileDate.Before(rng.end) {
						counts[i]++
						break
					}
				}
			} else if rng.end.IsZero() {
				if !fileDate.Before(rng.start) {
					counts[i]++
					break
				}
			} else {
				if !fileDate.Before(rng.start) && fileDate.Before(rng.end) {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.DateRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.DateRangeCount{
			Label: rng.label,
			Start: rng.start,
			End:   rng.end,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetOwnerFacet returns owner facet counts from OS metadata.
// Returns a map of "owner:username" -> count, where username is the actual owner username.
func (r *FileRepository) GetOwnerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	counts := make(map[string]int)

	// Get owner usernames from file_ownership joined with system_users
	// Only return actual usernames, not the ownership_type value ("owner" is not a username)
	query := `
		SELECT COALESCE(su.username, 'unknown') as owner, COUNT(DISTINCT fo.file_id) as count
		FROM file_ownership fo
		INNER JOIN files f ON f.workspace_id = fo.workspace_id AND f.id = fo.file_id
		LEFT JOIN system_users su ON su.id = fo.user_id
		WHERE fo.workspace_id = ? AND fo.ownership_type = 'owner'
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fo.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY su.username ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var owner string
		var count int
		if err := rows.Scan(&owner, &count); err != nil {
			return nil, err
		}
		// Only add non-empty usernames (skip empty strings)
		// The "unknown" value is acceptable if there are files with unknown owners
		if owner != "" {
			counts["owner:"+owner] = count
		}
	}

	return counts, nil
}

// UpsertSystemUser inserts or updates a system user.
func (r *FileRepository) UpsertSystemUser(ctx context.Context, workspaceID entity.WorkspaceID, user *entity.SystemUser) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if user.Username == "" {
		return fmt.Errorf("username is required")
	}

	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	// Generate ID if not set
	if user.ID == "" {
		user.ID = fmt.Sprintf("%s:%s:%d", workspaceID.String(), user.Username, user.UID)
	}

	var personID interface{}
	if user.PersonID != nil {
		personID = *user.PersonID
	}

	isSystem := 0
	if user.System {
		isSystem = 1
	}

	query := `
		INSERT INTO system_users (
			id, workspace_id, person_id, username, uid, full_name, home_dir, shell, is_system, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, username, uid) DO UPDATE SET
			person_id = excluded.person_id,
			full_name = excluded.full_name,
			home_dir = excluded.home_dir,
			shell = excluded.shell,
			is_system = excluded.is_system,
			updated_at = excluded.updated_at
	`

	_, err := r.conn.Exec(ctx, query,
		user.ID,
		workspaceID.String(),
		personID,
		user.Username,
		user.UID,
		user.FullName,
		user.HomeDir,
		user.Shell,
		isSystem,
		user.CreatedAt.UnixMilli(),
		user.UpdatedAt.UnixMilli(),
	)

	return err
}

// UpsertFileOwnership inserts or updates a file ownership relationship.
func (r *FileRepository) UpsertFileOwnership(ctx context.Context, workspaceID entity.WorkspaceID, ownership *entity.FileOwnership) error {
	if ownership == nil {
		return fmt.Errorf("ownership is nil")
	}
	if ownership.FileID == "" {
		return fmt.Errorf("file_id is required")
	}
	if ownership.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if ownership.OwnershipType == "" {
		return fmt.Errorf("ownership_type is required")
	}

	if ownership.DetectedAt.IsZero() {
		ownership.DetectedAt = time.Now()
	}

	query := `
		INSERT INTO file_ownership (
			file_id, workspace_id, user_id, ownership_type, permissions, detected_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, user_id) DO UPDATE SET
			ownership_type = excluded.ownership_type,
			permissions = excluded.permissions,
			detected_at = excluded.detected_at
	`

	_, err := r.conn.Exec(ctx, query,
		ownership.FileID,
		workspaceID.String(),
		ownership.UserID,
		ownership.OwnershipType,
		ownership.Permissions,
		ownership.DetectedAt.UnixMilli(),
	)

	return err
}

// GetMimeTypeFacet returns MIME type facet counts from enhanced metadata.
func (r *FileRepository) GetMimeTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(json_extract(enhanced, '$.MimeType.MimeType'), 'unknown') as mime_type,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY mime_type ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var mimeType string
		var count int
		if err := rows.Scan(&mimeType, &count); err != nil {
			return nil, err
		}
		if mimeType != "" && mimeType != "unknown" {
			counts[mimeType] = count
		}
	}

	return counts, nil
}

// GetMimeCategoryFacet returns MIME category facet counts from enhanced metadata.
func (r *FileRepository) GetMimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(json_extract(enhanced, '$.MimeType.Category'), 'unknown') as mime_category,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY mime_category ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var mimeCategory string
		var count int
		if err := rows.Scan(&mimeCategory, &count); err != nil {
			return nil, err
		}
		if mimeCategory != "" && mimeCategory != "unknown" {
			counts[mimeCategory] = count
		}
	}

	return counts, nil
}

// GetIndexingStatusFacet returns indexing status facet counts.
func (r *FileRepository) GetIndexingStatusFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			CASE 
				WHEN indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 AND indexed_document = 1 AND indexed_mirror = 1 THEN 'complete'
				WHEN indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 AND indexed_document = 1 THEN 'document_complete'
				WHEN indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 THEN 'code_complete'
				WHEN indexed_basic = 1 AND indexed_mime = 1 THEN 'mime_complete'
				WHEN indexed_basic = 1 THEN 'basic_only'
				ELSE 'not_indexed'
			END as status,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY status ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}

	return counts, nil
}

// GetIndexingErrorFacet returns indexing error facet counts.
// Groups files by the stage where the error occurred.
func (r *FileRepository) GetIndexingErrorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			CASE 
				WHEN json_extract(enhanced, '$.IndexingErrors') IS NOT NULL 
					AND json_array_length(json_extract(enhanced, '$.IndexingErrors')) > 0 THEN
					json_extract(json_extract(enhanced, '$.IndexingErrors[0]'), '$.Stage')
				ELSE 'none'
			END as error_stage,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY error_stage ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var stage sql.NullString
		var count int
		if err := rows.Scan(&stage, &count); err != nil {
			return nil, err
		}
		stageStr := "none"
		if stage.Valid && stage.String != "" {
			stageStr = stage.String
		}
		counts[stageStr] = count
	}

	return counts, nil
}

// GetComplexityRangeFacet returns code complexity range facet counts.
// Extracts complexity from CodeMetrics stored in enhanced JSON column.
func (r *FileRepository) GetComplexityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Very Low (0-5)", 0, 5},
		{"Low (5-10)", 5, 10},
		{"Medium (10-20)", 10, 20},
		{"High (20-50)", 20, 50},
		{"Very High (> 50)", 50, 0}, // 0 means unbounded
	}

	// Extract complexity from JSON: enhanced->CodeMetrics->Complexity
	query := `
		SELECT 
			json_extract(enhanced, '$.CodeMetrics.Complexity') as complexity
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.CodeMetrics.Complexity') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var complexity sql.NullFloat64
		if err := rows.Scan(&complexity); err != nil {
			return nil, err
		}

		if !complexity.Valid {
			continue
		}

		val := complexity.Float64
		for i, rng := range ranges {
			if rng.max == 0 {
				// Unbounded upper limit
				if val >= rng.min {
					counts[i]++
					break
				}
			} else {
				if val >= rng.min && val < rng.max {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetProjectScoreRangeFacet returns project assignment score range facet counts.
func (r *FileRepository) GetProjectScoreRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Very Low (0-0.2)", 0, 0.2},
		{"Low (0.2-0.4)", 0.2, 0.4},
		{"Medium (0.4-0.6)", 0.4, 0.6},
		{"High (0.6-0.8)", 0.6, 0.8},
		{"Very High (0.8-1.0)", 0.8, 1.0},
	}

	query := `
		SELECT DISTINCT pa.score
		FROM project_assignments pa
		INNER JOIN files f ON f.workspace_id = pa.workspace_id AND f.id = pa.file_id
		WHERE pa.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND pa.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var score float64
		if err := rows.Scan(&score); err != nil {
			return nil, err
		}

		for i, rng := range ranges {
			if score >= rng.min && score < rng.max {
				counts[i]++
				break
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetTemporalPatternFacet returns temporal pattern facet counts.
// Extracts LastAccessPattern from TemporalMetrics stored in enhanced JSON column.
func (r *FileRepository) GetTemporalPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(json_extract(enhanced, '$.TemporalMetrics.LastAccessPattern'), 'unknown') as pattern,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.TemporalMetrics.LastAccessPattern') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY pattern ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var pattern string
		var count int
		if err := rows.Scan(&pattern, &count); err != nil {
			return nil, err
		}
		counts[pattern] = count
	}

	return counts, nil
}

// GetReadabilityLevelFacet returns readability level facet counts.
// Extracts ReadabilityLevel from ContentQuality stored in enhanced JSON column.
func (r *FileRepository) GetReadabilityLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(json_extract(enhanced, '$.ContentQuality.ReadabilityLevel'), 'unknown') as level,
			COUNT(*) as count
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.ContentQuality.ReadabilityLevel') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY level ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		counts[level] = count
	}

	return counts, nil
}

// GetFunctionCountRangeFacet returns function count range facet counts.
// Extracts FunctionCount from CodeMetrics stored in enhanced JSON column.
func (r *FileRepository) GetFunctionCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   int
		max   int
	}{
		{"None (0)", 0, 0},
		{"Small (1-10)", 1, 10},
		{"Medium (11-50)", 11, 50},
		{"Large (51-200)", 51, 200},
		{"Very Large (> 200)", 200, 0}, // 0 means unbounded
	}

	query := `
		SELECT 
			json_extract(enhanced, '$.CodeMetrics.FunctionCount') as function_count
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.CodeMetrics.FunctionCount') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var functionCount sql.NullInt64
		if err := rows.Scan(&functionCount); err != nil {
			return nil, err
		}

		if !functionCount.Valid {
			continue
		}

		val := int(functionCount.Int64)
		for i, rng := range ranges {
			if rng.max == 0 {
				// Unbounded upper limit
				if val >= rng.min {
					counts[i]++
					break
				}
			} else {
				if val >= rng.min && val < rng.max {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   float64(rng.min),
			Max:   float64(rng.max),
			Count: counts[i],
		}
	}

	return result, nil
}

// GetLinesOfCodeRangeFacet returns lines of code range facet counts.
// Extracts LinesOfCode from CodeMetrics stored in enhanced JSON column.
func (r *FileRepository) GetLinesOfCodeRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   int
		max   int
	}{
		{"Tiny (< 100)", 0, 100},
		{"Small (100-500)", 100, 500},
		{"Medium (500-2000)", 500, 2000},
		{"Large (2000-10000)", 2000, 10000},
		{"Very Large (> 10000)", 10000, 0}, // 0 means unbounded
	}

	query := `
		SELECT 
			json_extract(enhanced, '$.CodeMetrics.LinesOfCode') as loc
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.CodeMetrics.LinesOfCode') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var loc sql.NullInt64
		if err := rows.Scan(&loc); err != nil {
			return nil, err
		}

		if !loc.Valid {
			continue
		}

		val := int(loc.Int64)
		for i, rng := range ranges {
			if rng.max == 0 {
				// Unbounded upper limit
				if val >= rng.min {
					counts[i]++
					break
				}
			} else {
				if val >= rng.min && val < rng.max {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   float64(rng.min),
			Max:   float64(rng.max),
			Count: counts[i],
		}
	}

	return result, nil
}

// GetCommentPercentageRangeFacet returns comment percentage range facet counts.
// Extracts CommentPercentage from CodeMetrics stored in enhanced JSON column.
func (r *FileRepository) GetCommentPercentageRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"None (0%)", 0, 0},
		{"Low (0-10%)", 0, 10},
		{"Medium (10-25%)", 10, 25},
		{"High (25-50%)", 25, 50},
		{"Very High (> 50%)", 50, 0}, // 0 means unbounded
	}

	query := `
		SELECT 
			json_extract(enhanced, '$.CodeMetrics.CommentPercentage') as comment_pct
		FROM files
		WHERE workspace_id = ? 
			AND enhanced IS NOT NULL 
			AND json_extract(enhanced, '$.CodeMetrics.CommentPercentage') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count files in each range
	counts := make([]int, len(ranges))
	for rows.Next() {
		var commentPct sql.NullFloat64
		if err := rows.Scan(&commentPct); err != nil {
			return nil, err
		}

		if !commentPct.Valid {
			continue
		}

		val := commentPct.Float64
		for i, rng := range ranges {
			if rng.max == 0 {
				// Unbounded upper limit
				if val >= rng.min {
					counts[i]++
					break
				}
			} else {
				if val >= rng.min && val < rng.max {
					counts[i]++
					break
				}
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetImageFormatFacet returns image format facet counts.
func (r *FileRepository) GetImageFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.Format", "unknown")
}

// GetImageColorSpaceFacet returns image color space facet counts.
func (r *FileRepository) GetImageColorSpaceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.ColorSpace", "unknown")
}

// GetCameraMakeFacet returns camera make facet counts.
func (r *FileRepository) GetCameraMakeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.EXIFCameraMake", "unknown")
}

// GetCameraModelFacet returns camera model facet counts.
func (r *FileRepository) GetCameraModelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.EXIFCameraModel", "unknown")
}

// GetImageGPSLocationFacet returns GPS location facet counts for images.
func (r *FileRepository) GetImageGPSLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.GPSLocation", "unknown")
}

// GetImageOrientationFacet returns image orientation facet counts.
func (r *FileRepository) GetImageOrientationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.Orientation", "unknown")
}

// GetImageTransparencyFacet returns image transparency facet counts.
func (r *FileRepository) GetImageTransparencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.HasTransparency")
}

// GetImageAnimatedFacet returns image animation facet counts.
func (r *FileRepository) GetImageAnimatedFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.IsAnimated")
}

// GetImageColorDepthRangeFacet returns image color depth range facet counts.
func (r *FileRepository) GetImageColorDepthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (<= 8 bpp)", 0, 9},
		{"Medium (9 - 16 bpp)", 9, 17},
		{"High (17 - 32 bpp)", 17, 33},
		{"Ultra (> 32 bpp)", 33, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.ColorDepth", ranges)
}

// GetImageISORangeFacet returns image ISO range facet counts.
func (r *FileRepository) GetImageISORangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 200)", 0, 200},
		{"Medium (200 - 800)", 200, 800},
		{"High (800 - 3200)", 800, 3200},
		{"Extreme (> 3200)", 3200, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.EXIFISO", ranges)
}

// GetImageApertureRangeFacet returns image aperture range facet counts.
func (r *FileRepository) GetImageApertureRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Wide (<= 2.8)", 0, 2.8},
		{"Standard (2.8 - 5.6)", 2.8, 5.6},
		{"Narrow (5.6 - 11)", 5.6, 11},
		{"Very Narrow (> 11)", 11, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.EXIFFNumber", ranges)
}

// GetImageFocalLengthRangeFacet returns image focal length range facet counts.
func (r *FileRepository) GetImageFocalLengthRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Wide (< 35mm)", 0, 35},
		{"Standard (35 - 70mm)", 35, 70},
		{"Telephoto (70 - 200mm)", 70, 200},
		{"Super Telephoto (> 200mm)", 200, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.ImageMetadata.EXIFFocalLength", ranges)
}

// GetVideoResolutionFacet returns video resolution facet counts.
func (r *FileRepository) GetVideoResolutionFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT json_extract(enhanced, '$.VideoMetadata.Width') as width,
		       json_extract(enhanced, '$.VideoMetadata.Height') as height
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND json_extract(enhanced, '$.VideoMetadata.Width') IS NOT NULL
		  AND json_extract(enhanced, '$.VideoMetadata.Height') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var width, height float64
		if err := rows.Scan(&width, &height); err != nil {
			return nil, err
		}
		label := classifyVideoResolution(width, height)
		if label == "" {
			continue
		}
		counts[label] = counts[label] + 1
	}

	return counts, nil
}

// GetPermissionLevelFacet returns permission level facet counts.
func (r *FileRepository) GetPermissionLevelFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(json_extract(enhanced, '$.OSContextTaxonomy.Security.PermissionLevel'), 'unknown') as level,
		       COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY level ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		counts[level] = count
	}

	return counts, nil
}

// GetContentEncodingFacet returns content encoding facet counts.
func (r *FileRepository) GetContentEncodingFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.ContentEncoding", "unknown")
}

// GetLanguageConfidenceRangeFacet returns language confidence range facet counts.
func (r *FileRepository) GetLanguageConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 0.5)", 0, 0.5},
		{"Medium (0.5 - 0.8)", 0.5, 0.8},
		{"High (0.8 - 1.0)", 0.8, 1.0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.LanguageConfidence", ranges)
}

// GetFilesystemTypeFacet returns filesystem type facet counts.
func (r *FileRepository) GetFilesystemTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSMetadata.FileSystem.FileSystemType", "unknown")
}

// GetMountPointFacet returns mount point facet counts.
func (r *FileRepository) GetMountPointFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSMetadata.FileSystem.MountPoint", "unknown")
}

// GetSecurityCategoryFacet returns security category facet counts.
func (r *FileRepository) GetSecurityCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Security.SecurityCategory")
}

// GetSecurityAttributesFacet returns security attributes facet counts.
func (r *FileRepository) GetSecurityAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Security.SecurityAttributes")
}

// GetHasACLsFacet returns ACL presence facet counts.
func (r *FileRepository) GetHasACLsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Security.HasACLs")
}

// GetACLComplexityFacet returns ACL complexity facet counts.
func (r *FileRepository) GetACLComplexityFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Security.ACLComplexity", "unknown")
}

// GetOwnerTypeFacet returns owner type facet counts.
func (r *FileRepository) GetOwnerTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Ownership.OwnerType", "unknown")
}

// GetGroupCategoryFacet returns group category facet counts.
func (r *FileRepository) GetGroupCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Ownership.GroupCategory", "unknown")
}

// GetAccessRelationFacet returns access relation facet counts.
func (r *FileRepository) GetAccessRelationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Ownership.AccessRelations")
}

// GetOwnershipPatternFacet returns ownership pattern facet counts.
func (r *FileRepository) GetOwnershipPatternFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Ownership.OwnershipPattern", "unknown")
}

// GetAccessFrequencyFacet returns access frequency facet counts.
func (r *FileRepository) GetAccessFrequencyFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Temporal.AccessFrequency", "unknown")
}

// GetTimeCategoryFacet returns time category facet counts.
func (r *FileRepository) GetTimeCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.Temporal.TimeCategory")
}

// GetSystemFileTypeFacet returns system file type facet counts.
func (r *FileRepository) GetSystemFileTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.System.SystemFileType", "unknown")
}

// GetFileSystemCategoryFacet returns file system category facet counts.
func (r *FileRepository) GetFileSystemCategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.System.FileSystemCategory", "unknown")
}

// GetSystemAttributesFacet returns system attributes facet counts.
func (r *FileRepository) GetSystemAttributesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.System.SystemAttributes")
}

// GetSystemFeaturesFacet returns system features facet counts.
func (r *FileRepository) GetSystemFeaturesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedArrayFacet(ctx, workspaceID, fileIDs, "$.OSContextTaxonomy.System.SystemFeatures")
}

// GetContentQualityRangeFacet returns content quality range facet counts.
func (r *FileRepository) GetContentQualityRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 0.3)", 0, 0.3},
		{"Medium (0.3 - 0.7)", 0.3, 0.7},
		{"High (0.7 - 1.0)", 0.7, 1.0},
	}

	query := `
		SELECT json_extract(enhanced, '$.ContentQuality.QualityScore') as quality
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND json_extract(enhanced, '$.ContentQuality.QualityScore') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var quality float64
		if err := rows.Scan(&quality); err != nil {
			return nil, err
		}
		for i, rng := range ranges {
			if rng.max == 0 {
				if quality >= rng.min {
					counts[i]++
					break
				}
			} else if quality >= rng.min && quality < rng.max {
				counts[i]++
				break
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetImageDimensionsRangeFacet returns image dimension range facet counts.
func (r *FileRepository) GetImageDimensionsRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Tiny (< 0.5 MP)", 0, 0.5e6},
		{"Small (0.5 - 2 MP)", 0.5e6, 2e6},
		{"Medium (2 - 8 MP)", 2e6, 8e6},
		{"Large (8 - 20 MP)", 8e6, 20e6},
		{"Huge (> 20 MP)", 20e6, 0},
	}

	query := `
		SELECT json_extract(enhanced, '$.ImageMetadata.Width') as width,
		       json_extract(enhanced, '$.ImageMetadata.Height') as height
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND json_extract(enhanced, '$.ImageMetadata.Width') IS NOT NULL
		  AND json_extract(enhanced, '$.ImageMetadata.Height') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var width, height float64
		if err := rows.Scan(&width, &height); err != nil {
			return nil, err
		}
		if width <= 0 || height <= 0 {
			continue
		}
		pixels := width * height
		for i, rng := range ranges {
			if rng.max == 0 {
				if pixels >= rng.min {
					counts[i]++
					break
				}
			} else if pixels >= rng.min && pixels < rng.max {
				counts[i]++
				break
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetAudioDurationRangeFacet returns audio duration range facet counts.
func (r *FileRepository) GetAudioDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Very Short (< 30s)", 0, 30},
		{"Short (30s - 2m)", 30, 120},
		{"Medium (2m - 10m)", 120, 600},
		{"Long (10m - 1h)", 600, 3600},
		{"Very Long (> 1h)", 3600, 0},
	}

	query := `
		SELECT json_extract(enhanced, '$.AudioMetadata.Duration') as duration
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND json_extract(enhanced, '$.AudioMetadata.Duration') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var duration float64
		if err := rows.Scan(&duration); err != nil {
			return nil, err
		}
		if duration <= 0 {
			continue
		}
		for i, rng := range ranges {
			if rng.max == 0 {
				if duration >= rng.min {
					counts[i]++
					break
				}
			} else if duration >= rng.min && duration < rng.max {
				counts[i]++
				break
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

// GetAudioBitrateRangeFacet returns audio bitrate range facet counts.
func (r *FileRepository) GetAudioBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 128 kbps)", 0, 128},
		{"Standard (128 - 256 kbps)", 128, 256},
		{"High (256 - 320 kbps)", 256, 320},
		{"Lossless (> 320 kbps)", 320, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.AudioMetadata.Bitrate", ranges)
}

// GetAudioSampleRateRangeFacet returns audio sample rate range facet counts.
func (r *FileRepository) GetAudioSampleRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 22.05 kHz)", 0, 22050},
		{"Standard (22.05 - 44.1 kHz)", 22050, 44100},
		{"High (44.1 - 96 kHz)", 44100, 96000},
		{"Ultra (> 96 kHz)", 96000, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.AudioMetadata.SampleRate", ranges)
}

// GetAudioCodecFacet returns audio codec facet counts.
func (r *FileRepository) GetAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.AudioMetadata.Codec", "unknown")
}

// GetAudioFormatFacet returns audio format facet counts.
func (r *FileRepository) GetAudioFormatFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.AudioMetadata.Format", "unknown")
}

// GetAudioGenreFacet returns audio genre facet counts.
func (r *FileRepository) GetAudioGenreFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacetCoalesce(ctx, workspaceID, fileIDs, []string{
		"$.AudioMetadata.ID3Genre",
		"$.AudioMetadata.VorbisGenre",
	}, "unknown")
}

// GetAudioArtistFacet returns audio artist facet counts.
func (r *FileRepository) GetAudioArtistFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacetCoalesce(ctx, workspaceID, fileIDs, []string{
		"$.AudioMetadata.ID3Artist",
		"$.AudioMetadata.ID3AlbumArtist",
		"$.AudioMetadata.VorbisArtist",
	}, "unknown")
}

// GetAudioAlbumFacet returns audio album facet counts.
func (r *FileRepository) GetAudioAlbumFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacetCoalesce(ctx, workspaceID, fileIDs, []string{
		"$.AudioMetadata.ID3Album",
		"$.AudioMetadata.VorbisAlbum",
	}, "unknown")
}

// GetAudioYearFacet returns audio year facet counts.
func (r *FileRepository) GetAudioYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacetCoalesce(ctx, workspaceID, fileIDs, []string{
		"$.AudioMetadata.ID3Year",
		"$.AudioMetadata.VorbisDate",
	}, "unknown")
}

// GetAudioChannelsFacet returns audio channel count facet counts.
func (r *FileRepository) GetAudioChannelsFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT json_extract(enhanced, '$.AudioMetadata.Channels') as channels
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var channels sql.NullFloat64
		if err := rows.Scan(&channels); err != nil {
			return nil, err
		}
		label := "unknown"
		if channels.Valid {
			label = classifyAudioChannels(int(channels.Float64))
			if label == "" {
				label = "unknown"
			}
		}
		counts[label] = counts[label] + 1
	}

	return counts, nil
}

// GetAudioHasAlbumArtFacet returns audio album art facet counts.
func (r *FileRepository) GetAudioHasAlbumArtFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.AudioMetadata.HasAlbumArt")
}

// GetVideoDurationRangeFacet returns video duration range facet counts.
func (r *FileRepository) GetVideoDurationRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Short (< 5m)", 0, 300},
		{"Medium (5m - 30m)", 300, 1800},
		{"Long (30m - 2h)", 1800, 7200},
		{"Feature (> 2h)", 7200, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.Duration", ranges)
}

// GetVideoBitrateRangeFacet returns video bitrate range facet counts.
func (r *FileRepository) GetVideoBitrateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 1000 kbps)", 0, 1000},
		{"Standard (1000 - 3000 kbps)", 1000, 3000},
		{"High (3000 - 8000 kbps)", 3000, 8000},
		{"Ultra (> 8000 kbps)", 8000, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.Bitrate", ranges)
}

// GetVideoFrameRateRangeFacet returns video frame rate range facet counts.
func (r *FileRepository) GetVideoFrameRateRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error) {
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"Low (< 24 fps)", 0, 24},
		{"Standard (24 - 30 fps)", 24, 30},
		{"High (30 - 60 fps)", 30, 60},
		{"Ultra (> 60 fps)", 60, 0},
	}

	return r.getEnhancedNumericRangeFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.FrameRate", ranges)
}

// GetVideoCodecFacet returns video codec facet counts.
func (r *FileRepository) GetVideoCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacetCoalesce(ctx, workspaceID, fileIDs, []string{
		"$.VideoMetadata.VideoCodec",
		"$.VideoMetadata.Codec",
	}, "unknown")
}

// GetVideoAudioCodecFacet returns video audio codec facet counts.
func (r *FileRepository) GetVideoAudioCodecFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.AudioCodec", "unknown")
}

// GetVideoContainerFacet returns video container facet counts.
func (r *FileRepository) GetVideoContainerFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.Container", "unknown")
}

// GetVideoAspectRatioFacet returns video aspect ratio facet counts.
func (r *FileRepository) GetVideoAspectRatioFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedTextFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.VideoAspectRatio", "unknown")
}

// GetVideoHasSubtitlesFacet returns video subtitles facet counts.
func (r *FileRepository) GetVideoHasSubtitlesFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.HasSubtitles")
}

// GetVideoSubtitleLanguageFacet returns video subtitle language facet counts.
func (r *FileRepository) GetVideoSubtitleLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(CAST(json_each.value AS TEXT), 'unknown') as value,
		       COUNT(*) as count
		FROM files
		JOIN json_each(COALESCE(json_extract(enhanced, '$.VideoMetadata.SubtitleTracks'), '[]'))
		WHERE files.workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND files.id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY value ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		counts[value] = count
	}

	return counts, nil
}

// GetVideoHasChaptersFacet returns video chapters facet counts.
func (r *FileRepository) GetVideoHasChaptersFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.HasChapters")
}

// GetVideoIs3DFacet returns video 3D facet counts.
func (r *FileRepository) GetVideoIs3DFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	return r.getEnhancedBoolFacet(ctx, workspaceID, fileIDs, "$.VideoMetadata.Is3D")
}

// GetVideoQualityTierFacet returns video quality tier facet counts.
func (r *FileRepository) GetVideoQualityTierFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(json_extract(enhanced, '$.VideoMetadata.Width'), 0) as width,
		       COALESCE(json_extract(enhanced, '$.VideoMetadata.Height'), 0) as height,
		       COALESCE(json_extract(enhanced, '$.VideoMetadata.Is4K'), 0) as is4k,
		       COALESCE(json_extract(enhanced, '$.VideoMetadata.IsHD'), 0) as ishd
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND (
		    json_extract(enhanced, '$.VideoMetadata.Width') IS NOT NULL
		    OR json_extract(enhanced, '$.VideoMetadata.Height') IS NOT NULL
		    OR json_extract(enhanced, '$.VideoMetadata.Is4K') IS NOT NULL
		    OR json_extract(enhanced, '$.VideoMetadata.IsHD') IS NOT NULL
		  )
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var width, height float64
		var is4k, ishd float64
		if err := rows.Scan(&width, &height, &is4k, &ishd); err != nil {
			return nil, err
		}
		label := classifyVideoQualityTier(width, height, ishd > 0.5, is4k > 0.5)
		if label == "" {
			continue
		}
		counts[label] = counts[label] + 1
	}

	return counts, nil
}

func classifyVideoResolution(width, height float64) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	maxDim := height
	if width > maxDim {
		maxDim = width
	}
	switch {
	case maxDim >= 2160:
		return "4K+"
	case maxDim >= 1440:
		return "1440p"
	case maxDim >= 1080:
		return "1080p"
	case maxDim >= 720:
		return "720p"
	case maxDim >= 480:
		return "480p"
	default:
		return "SD (< 480p)"
	}
}

func classifyAudioChannels(channels int) string {
	switch channels {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	case 6:
		return "5.1"
	case 7:
		return "6.1"
	case 8:
		return "7.1"
	default:
		if channels > 0 {
			return fmt.Sprintf("%d channels", channels)
		}
		return ""
	}
}

func classifyVideoQualityTier(width, height float64, isHD, is4K bool) string {
	if is4K {
		return "4K"
	}
	maxDim := height
	if width > maxDim {
		maxDim = width
	}
	if maxDim >= 2160 {
		return "4K"
	}
	if isHD || maxDim >= 720 {
		return "HD"
	}
	if maxDim > 0 {
		return "SD"
	}
	return ""
}

func (r *FileRepository) getEnhancedTextFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, jsonPath, defaultValue string) (map[string]int, error) {
	query := `
		SELECT COALESCE(CAST(json_extract(enhanced, '` + jsonPath + `') AS TEXT), ?) as value,
		       COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{defaultValue, workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY value ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		counts[value] = count
	}

	return counts, nil
}

func (r *FileRepository) getEnhancedArrayFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, jsonPath string) (map[string]int, error) {
	query := `
		SELECT COALESCE(CAST(json_each.value AS TEXT), 'unknown') as value,
		       COUNT(*) as count
		FROM files
		JOIN json_each(COALESCE(json_extract(enhanced, '` + jsonPath + `'), '[]'))
		WHERE files.workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND files.id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY value ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		counts[value] = count
	}

	return counts, nil
}

func (r *FileRepository) getEnhancedTextFacetCoalesce(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, jsonPaths []string, defaultValue string) (map[string]int, error) {
	if len(jsonPaths) == 0 {
		return map[string]int{defaultValue: 0}, nil
	}

	valueExprs := make([]string, len(jsonPaths))
	for i, path := range jsonPaths {
		valueExprs[i] = "CAST(json_extract(enhanced, '" + path + "') AS TEXT)"
	}

	query := `
		SELECT COALESCE(` + joinStrings(valueExprs, ",") + `, ?) as value,
		       COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{defaultValue, workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY value ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		counts[value] = count
	}

	return counts, nil
}

func (r *FileRepository) getEnhancedBoolFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, jsonPath string) (map[string]int, error) {
	query := `
		SELECT CASE json_extract(enhanced, '` + jsonPath + `')
			WHEN 1 THEN 'true'
			WHEN 0 THEN 'false'
			ELSE 'unknown'
		END as value,
		COUNT(*) as count
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY value ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		counts[value] = count
	}

	return counts, nil
}

func (r *FileRepository) getEnhancedNumericRangeFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	fileIDs []entity.FileID,
	jsonPath string,
	ranges []struct {
		label string
		min   float64
		max   float64
	},
) ([]repository.NumericRangeCount, error) {
	query := `
		SELECT json_extract(enhanced, '` + jsonPath + `') as value
		FROM files
		WHERE workspace_id = ?
		  AND enhanced IS NOT NULL
		  AND json_extract(enhanced, '` + jsonPath + `') IS NOT NULL
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND id IN (` + joinStrings(placeholders, ",") + `)`
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]int, len(ranges))
	for rows.Next() {
		var value float64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		for i, rng := range ranges {
			if rng.max == 0 {
				if value >= rng.min {
					counts[i]++
					break
				}
			} else if value >= rng.min && value < rng.max {
				counts[i]++
				break
			}
		}
	}

	result := make([]repository.NumericRangeCount, len(ranges))
	for i, rng := range ranges {
		result[i] = repository.NumericRangeCount{
			Label: rng.label,
			Min:   rng.min,
			Max:   rng.max,
			Count: counts[i],
		}
	}

	return result, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
