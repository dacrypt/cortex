package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// MetadataRepository implements repository.MetadataRepository using SQLite.
type MetadataRepository struct {
	conn              *Connection
	aiContextRepo     *AIContextRepository
	enrichmentRepo    *EnrichmentRepository
}

// NewMetadataRepository creates a new SQLite metadata repository.
func NewMetadataRepository(conn *Connection) *MetadataRepository {
	return &MetadataRepository{
		conn:           conn,
		aiContextRepo:  NewAIContextRepository(conn),
		enrichmentRepo: NewEnrichmentRepository(conn),
	}
}

// GetOrCreate retrieves or creates file metadata.
func (r *MetadataRepository) GetOrCreate(ctx context.Context, workspaceID entity.WorkspaceID, relativePath, extension string) (*entity.FileMetadata, error) {
	// Try to get existing
	existing, err := r.GetByPath(ctx, workspaceID, relativePath)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// Create new metadata
	meta := entity.NewFileMetadata(relativePath, extension)
	now := time.Now().UnixMilli()

	query := `
		INSERT INTO file_metadata (
			file_id, workspace_id, relative_path, type, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = r.conn.Exec(ctx, query,
		meta.FileID.String(),
		workspaceID.String(),
		relativePath,
		meta.Type,
		now,
		now,
	)
	if err != nil {
		// Handle race condition - try to get again
		existing, _ := r.GetByPath(ctx, workspaceID, relativePath)
		if existing != nil {
			return existing, nil
		}
		return nil, err
	}

	return meta, nil
}

// Get retrieves file metadata by ID.
func (r *MetadataRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.FileMetadata, error) {
	return r.getMetadata(ctx, workspaceID, "file_id = ?", fileID.String())
}

// GetByPath retrieves file metadata by path.
func (r *MetadataRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.FileMetadata, error) {
	return r.getMetadata(ctx, workspaceID, "relative_path = ?", relativePath)
}

func (r *MetadataRepository) getMetadata(ctx context.Context, workspaceID entity.WorkspaceID, where string, arg interface{}) (*entity.FileMetadata, error) {
	query := `
		SELECT file_id, relative_path, type, notes, detected_language,
		       ai_summary, ai_summary_hash, ai_key_terms, mirror_format, mirror_path,
		       mirror_source_mtime, mirror_updated_at, ai_category, ai_category_confidence,
		       ai_category_updated_at, ai_related, ai_context, enrichment_data, created_at, updated_at
		FROM file_metadata
		WHERE workspace_id = ? AND ` + where

	row := r.conn.QueryRow(ctx, query, workspaceID.String(), arg)

	var (
		fileID, relPath, fileType                       string
		notes, detectedLanguage, aiSummary, aiSummaryHash, aiKeyTermsJSON sql.NullString
		mirrorFormat, mirrorPath                        sql.NullString
		mirrorSourceMtime, mirrorUpdatedAt              sql.NullInt64
		aiCategory                                      sql.NullString
		aiCategoryConfidence                            sql.NullFloat64
		aiCategoryUpdatedAt                             sql.NullInt64
		aiRelatedJSON, aiContextJSON, enrichmentDataJSON sql.NullString
		createdAt, updatedAt                            int64
	)

	err := row.Scan(
		&fileID, &relPath, &fileType, &notes, &detectedLanguage,
		&aiSummary, &aiSummaryHash, &aiKeyTermsJSON, &mirrorFormat, &mirrorPath,
		&mirrorSourceMtime, &mirrorUpdatedAt, &aiCategory, &aiCategoryConfidence, &aiCategoryUpdatedAt,
		&aiRelatedJSON, &aiContextJSON, &enrichmentDataJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	meta := &entity.FileMetadata{
		FileID:       entity.FileID(fileID),
		RelativePath: relPath,
		Type:         fileType,
		CreatedAt:    time.UnixMilli(createdAt),
		UpdatedAt:    time.UnixMilli(updatedAt),
	}

	if notes.Valid {
		meta.Notes = &notes.String
	}

	if detectedLanguage.Valid {
		meta.DetectedLanguage = &detectedLanguage.String
	}

	if aiSummary.Valid {
		meta.AISummary = &entity.AISummary{
			Summary:     aiSummary.String,
			ContentHash: aiSummaryHash.String,
		}
		if aiKeyTermsJSON.Valid {
			_ = json.Unmarshal([]byte(aiKeyTermsJSON.String), &meta.AISummary.KeyTerms)
		}
	}

	if mirrorFormat.Valid {
		meta.Mirror = &entity.MirrorMetadata{
			Format:      entity.MirrorFormat(mirrorFormat.String),
			Path:        mirrorPath.String,
			SourceMtime: time.UnixMilli(mirrorSourceMtime.Int64),
			UpdatedAt:   time.UnixMilli(mirrorUpdatedAt.Int64),
		}
	}

	if aiCategory.Valid {
		category := entity.AICategory{
			Category: aiCategory.String,
		}
		if aiCategoryConfidence.Valid {
			category.Confidence = aiCategoryConfidence.Float64
		}
		if aiCategoryUpdatedAt.Valid {
			category.UpdatedAt = time.UnixMilli(aiCategoryUpdatedAt.Int64)
		}
		meta.AICategory = &category
	}

	if aiRelatedJSON.Valid {
		var related []entity.RelatedFile
		if err := json.Unmarshal([]byte(aiRelatedJSON.String), &related); err == nil {
			meta.AIRelated = related
		}
	}

	// Load AIContext
	if aiContextJSON.Valid && aiContextJSON.String != "" {
		var aiContext entity.AIContext
		if err := json.Unmarshal([]byte(aiContextJSON.String), &aiContext); err == nil {
			meta.AIContext = &aiContext
		}
		// Note: silently ignore unmarshal errors - AIContext is optional
	}

	// Load EnrichmentData
	if enrichmentDataJSON.Valid && enrichmentDataJSON.String != "" {
		var enrichmentData entity.EnrichmentData
		if err := json.Unmarshal([]byte(enrichmentDataJSON.String), &enrichmentData); err == nil {
			meta.EnrichmentData = &enrichmentData
		} else {
			// Log error but don't fail - EnrichmentData is optional
			// Note: logger may not be available in all contexts
		}
	}

	// Load tags
	tags, _ := r.loadTags(ctx, workspaceID, entity.FileID(fileID))
	meta.Tags = tags

	// Load contexts
	contexts, _ := r.loadContexts(ctx, workspaceID, entity.FileID(fileID))
	meta.Contexts = contexts

	// Load suggested contexts
	suggestions, _ := r.loadSuggestedContexts(ctx, workspaceID, entity.FileID(fileID))
	meta.SuggestedContexts = suggestions

	return meta, nil
}

// Delete removes file metadata.
func (r *MetadataRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `DELETE FROM file_metadata WHERE workspace_id = ? AND file_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String())
	return err
}

// AddTag adds a tag to a file.
func (r *MetadataRepository) AddTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error {
	tag = entity.NormalizeTag(tag)
	if tag == "" {
		return nil
	}

	query := `
		INSERT OR IGNORE INTO file_tags (workspace_id, file_id, tag)
		VALUES (?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), tag)
	if err != nil {
		return err
	}

	return r.touchUpdatedAt(ctx, workspaceID, fileID)
}

// RemoveTag removes a tag from a file.
func (r *MetadataRepository) RemoveTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error {
	query := `DELETE FROM file_tags WHERE workspace_id = ? AND file_id = ? AND tag = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), tag)
	if err != nil {
		return err
	}

	return r.touchUpdatedAt(ctx, workspaceID, fileID)
}

// GetAllTags returns all unique tags.
func (r *MetadataRepository) GetAllTags(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error) {
	query := `SELECT DISTINCT tag FROM file_tags WHERE workspace_id = ? ORDER BY tag`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetTagCounts returns tag counts.
func (r *MetadataRepository) GetTagCounts(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `SELECT tag, COUNT(*) FROM file_tags WHERE workspace_id = ? GROUP BY tag`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		counts[tag] = count
	}

	return counts, nil
}

// ListByTag returns files with a specific tag.
func (r *MetadataRepository) ListByTag(ctx context.Context, workspaceID entity.WorkspaceID, tag string, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	query := `
		SELECT m.file_id FROM file_metadata m
		JOIN file_tags t ON m.workspace_id = t.workspace_id AND m.file_id = t.file_id
		WHERE m.workspace_id = ? AND t.tag = ?
		LIMIT ? OFFSET ?
	`

	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), tag, opts.Limit, opts.Offset)
}

// AddContext adds a context to a file.
func (r *MetadataRepository) AddContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error {
	context = strings.TrimSpace(context)
	if context == "" {
		return nil
	}

	query := `
		INSERT OR IGNORE INTO file_contexts (workspace_id, file_id, context)
		VALUES (?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), context)
	if err != nil {
		return err
	}

	return r.touchUpdatedAt(ctx, workspaceID, fileID)
}

// RemoveContext removes a context from a file.
func (r *MetadataRepository) RemoveContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error {
	query := `DELETE FROM file_contexts WHERE workspace_id = ? AND file_id = ? AND context = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), context)
	if err != nil {
		return err
	}

	return r.touchUpdatedAt(ctx, workspaceID, fileID)
}

// GetAllContexts returns all unique contexts.
func (r *MetadataRepository) GetAllContexts(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error) {
	query := `SELECT DISTINCT context FROM file_contexts WHERE workspace_id = ? ORDER BY context`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		contexts = append(contexts, c)
	}

	return contexts, nil
}

// GetContextCounts returns context counts.
func (r *MetadataRepository) GetContextCounts(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `SELECT context, COUNT(*) FROM file_contexts WHERE workspace_id = ? GROUP BY context`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var context string
		var count int
		if err := rows.Scan(&context, &count); err != nil {
			return nil, err
		}
		counts[context] = count
	}

	return counts, nil
}

// ListByContext returns files with a specific context.
func (r *MetadataRepository) ListByContext(ctx context.Context, workspaceID entity.WorkspaceID, context string, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	query := `
		SELECT m.file_id FROM file_metadata m
		JOIN file_contexts c ON m.workspace_id = c.workspace_id AND m.file_id = c.file_id
		WHERE m.workspace_id = ? AND c.context = ?
		LIMIT ? OFFSET ?
	`

	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), context, opts.Limit, opts.Offset)
}

// AddSuggestedContext adds a suggested context.
func (r *MetadataRepository) AddSuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error {
	context = strings.TrimSpace(context)
	if context == "" {
		return nil
	}

	query := `
		INSERT OR IGNORE INTO file_context_suggestions (workspace_id, file_id, context)
		VALUES (?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), context)
	return err
}

// RemoveSuggestedContext removes a suggested context.
func (r *MetadataRepository) RemoveSuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, context string) error {
	query := `DELETE FROM file_context_suggestions WHERE workspace_id = ? AND file_id = ? AND context = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), context)
	return err
}

// ClearSuggestedContexts removes all suggested contexts for a file.
func (r *MetadataRepository) ClearSuggestedContexts(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `DELETE FROM file_context_suggestions WHERE workspace_id = ? AND file_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String())
	return err
}

// GetAllSuggestedContexts returns all unique suggested contexts.
func (r *MetadataRepository) GetAllSuggestedContexts(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error) {
	query := `SELECT DISTINCT context FROM file_context_suggestions WHERE workspace_id = ? ORDER BY context`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		contexts = append(contexts, c)
	}

	return contexts, nil
}

// ListBySuggestedContext returns files with a suggested context.
func (r *MetadataRepository) ListBySuggestedContext(ctx context.Context, workspaceID entity.WorkspaceID, context string, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	query := `
		SELECT m.file_id FROM file_metadata m
		JOIN file_context_suggestions s ON m.workspace_id = s.workspace_id AND m.file_id = s.file_id
		WHERE m.workspace_id = ? AND s.context = ?
		LIMIT ? OFFSET ?
	`

	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), context, opts.Limit, opts.Offset)
}

// GetFilesWithSuggestions returns files that have suggestions.
func (r *MetadataRepository) GetFilesWithSuggestions(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	query := `
		SELECT DISTINCT m.file_id FROM file_metadata m
		JOIN file_context_suggestions s ON m.workspace_id = s.workspace_id AND m.file_id = s.file_id
		WHERE m.workspace_id = ?
		LIMIT ? OFFSET ?
	`

	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), opts.Limit, opts.Offset)
}

// UpdateNotes updates the notes for a file.
func (r *MetadataRepository) UpdateNotes(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, notes string) error {
	query := `
		UPDATE file_metadata SET notes = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, notes, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// UpdateDetectedLanguage updates the detected language for a file.
func (r *MetadataRepository) UpdateDetectedLanguage(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, languageCode string) error {
	query := `
		UPDATE file_metadata SET detected_language = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, languageCode, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// UpdateAISummary updates the AI summary for a file.
func (r *MetadataRepository) UpdateAISummary(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, summary entity.AISummary) error {
	var keyTermsJSON []byte
	if len(summary.KeyTerms) > 0 {
		keyTermsJSON, _ = json.Marshal(summary.KeyTerms)
	}

	query := `
		UPDATE file_metadata SET
			ai_summary = ?, ai_summary_hash = ?, ai_key_terms = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query,
		summary.Summary, summary.ContentHash, string(keyTermsJSON),
		time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// ClearAISummary clears the AI summary for a file.
func (r *MetadataRepository) ClearAISummary(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		UPDATE file_metadata SET
			ai_summary = NULL, ai_summary_hash = NULL, ai_key_terms = NULL, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// UpdateAICategory updates the AI category for a file.
func (r *MetadataRepository) UpdateAICategory(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, category entity.AICategory) error {
	query := `
		UPDATE file_metadata SET
			ai_category = ?, ai_category_confidence = ?, ai_category_updated_at = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query,
		category.Category, category.Confidence, category.UpdatedAt.UnixMilli(), time.Now().UnixMilli(),
		workspaceID.String(), fileID.String())
	return err
}

// ClearAICategory clears AI category metadata.
func (r *MetadataRepository) ClearAICategory(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		UPDATE file_metadata SET
			ai_category = NULL, ai_category_confidence = NULL, ai_category_updated_at = NULL, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// UpdateAIRelated updates AI related file suggestions.
func (r *MetadataRepository) UpdateAIRelated(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, related []entity.RelatedFile) error {
	relatedJSON, _ := json.Marshal(related)
	query := `
		UPDATE file_metadata SET
			ai_related = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, string(relatedJSON), time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// UpdateAIContext updates the AI context for a file.
func (r *MetadataRepository) UpdateAIContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, aiContext *entity.AIContext) error {
	if aiContext == nil {
		return r.ClearAIContext(ctx, workspaceID, fileID)
	}

	aiContextJSON, err := json.Marshal(aiContext)
	if err != nil {
		return fmt.Errorf("failed to marshal AIContext: %w", err)
	}

	query := `
		UPDATE file_metadata SET
			ai_context = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err = r.conn.Exec(ctx, query, string(aiContextJSON), time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	if err != nil {
		return err
	}

	// Sync to normalized tables
	if err := r.aiContextRepo.SyncAIContext(ctx, workspaceID, fileID, aiContext); err != nil {
		// Log error but don't fail the update - JSON is still stored
		// This allows graceful degradation if normalized tables have issues
		return fmt.Errorf("failed to sync AI context to normalized tables (JSON stored): %w", err)
	}

	return nil
}

// ClearAIContext clears AI context metadata.
func (r *MetadataRepository) ClearAIContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		UPDATE file_metadata SET
			ai_context = NULL, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	if err != nil {
		return err
	}

	// Also clear normalized tables
	if err := r.aiContextRepo.DeleteByFile(ctx, workspaceID, fileID); err != nil {
		return fmt.Errorf("failed to clear normalized AI context tables: %w", err)
	}

	return nil
}

// UpdateEnrichmentData updates the enrichment data for a file.
func (r *MetadataRepository) UpdateEnrichmentData(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, enrichmentData *entity.EnrichmentData) error {
	if enrichmentData == nil {
		return r.ClearEnrichmentData(ctx, workspaceID, fileID)
	}

	enrichmentDataJSON, err := json.Marshal(enrichmentData)
	if err != nil {
		return fmt.Errorf("failed to marshal EnrichmentData: %w", err)
	}

	query := `
		UPDATE file_metadata SET
			enrichment_data = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err = r.conn.Exec(ctx, query, string(enrichmentDataJSON), time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	if err != nil {
		return err
	}

	// Sync to normalized tables
	if err := r.enrichmentRepo.SyncEnrichmentData(ctx, workspaceID, fileID, enrichmentData); err != nil {
		// Log error but don't fail the update - JSON is still stored
		return fmt.Errorf("failed to sync enrichment data to normalized tables (JSON stored): %w", err)
	}

	return nil
}

// ClearEnrichmentData clears enrichment data metadata.
func (r *MetadataRepository) ClearEnrichmentData(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		UPDATE file_metadata SET
			enrichment_data = NULL, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	if err != nil {
		return err
	}

	// Also clear normalized tables
	if err := r.enrichmentRepo.DeleteByFile(ctx, workspaceID, fileID); err != nil {
		return fmt.Errorf("failed to clear normalized enrichment tables: %w", err)
	}

	return nil
}

// UpdateMirror updates the mirror metadata for a file.
func (r *MetadataRepository) UpdateMirror(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, mirror entity.MirrorMetadata) error {
	query := `
		UPDATE file_metadata SET
			mirror_format = ?, mirror_path = ?, mirror_source_mtime = ?,
			mirror_updated_at = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query,
		string(mirror.Format), mirror.Path, mirror.SourceMtime.UnixMilli(),
		mirror.UpdatedAt.UnixMilli(), time.Now().UnixMilli(),
		workspaceID.String(), fileID.String())
	return err
}

// ClearMirror clears the mirror metadata for a file.
func (r *MetadataRepository) ClearMirror(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		UPDATE file_metadata SET
			mirror_format = NULL, mirror_path = NULL, mirror_source_mtime = NULL,
			mirror_updated_at = NULL, updated_at = ?
		WHERE workspace_id = ? AND file_id = ?
	`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

// EnsureMetadataForFiles ensures metadata exists for files.
func (r *MetadataRepository) EnsureMetadataForFiles(ctx context.Context, workspaceID entity.WorkspaceID, files []repository.FileInfo) (int, error) {
	count := 0

	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO file_metadata (
				file_id, workspace_id, relative_path, type, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		now := time.Now().UnixMilli()

		for _, file := range files {
			meta := entity.NewFileMetadata(file.RelativePath, file.Extension)
			result, err := stmt.ExecContext(ctx,
				meta.FileID.String(),
				workspaceID.String(),
				file.RelativePath,
				meta.Type,
				now,
				now,
			)
			if err != nil {
				return err
			}

			affected, _ := result.RowsAffected()
			count += int(affected)
		}

		return nil
	})

	return count, err
}

// BatchAddTag adds a tag to multiple files.
func (r *MetadataRepository) BatchAddTag(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, tag string) (int, error) {
	count := 0

	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO file_tags (workspace_id, file_id, tag) VALUES (?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, fileID := range fileIDs {
			result, err := stmt.ExecContext(ctx, workspaceID.String(), fileID.String(), tag)
			if err != nil {
				return err
			}

			affected, _ := result.RowsAffected()
			count += int(affected)
		}

		return nil
	})

	return count, err
}

// BatchAddContext adds a context to multiple files.
func (r *MetadataRepository) BatchAddContext(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID, context string) (int, error) {
	count := 0

	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO file_contexts (workspace_id, file_id, context) VALUES (?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, fileID := range fileIDs {
			result, err := stmt.ExecContext(ctx, workspaceID.String(), fileID.String(), context)
			if err != nil {
				return err
			}

			affected, _ := result.RowsAffected()
			count += int(affected)
		}

		return nil
	})

	return count, err
}

// ListByType returns files of a specific type.
func (r *MetadataRepository) ListByType(ctx context.Context, workspaceID entity.WorkspaceID, fileType string, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	query := `
		SELECT file_id FROM file_metadata
		WHERE workspace_id = ? AND type = ?
		LIMIT ? OFFSET ?
	`

	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), fileType, opts.Limit, opts.Offset)
}

// GetAllTypes returns all unique types.
func (r *MetadataRepository) GetAllTypes(ctx context.Context, workspaceID entity.WorkspaceID) ([]string, error) {
	query := `SELECT DISTINCT type FROM file_metadata WHERE workspace_id = ? ORDER BY type`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}

	return types, nil
}

// ListByFolderPrefix returns all files with paths starting with the given folder prefix.
// Used for propagating project/context assignments to child files.
func (r *MetadataRepository) ListByFolderPrefix(ctx context.Context, workspaceID entity.WorkspaceID, folderPrefix string, opts repository.FileListOptions) ([]*entity.FileMetadata, error) {
	// Ensure folder prefix ends with / for proper prefix matching
	if folderPrefix != "" && !strings.HasSuffix(folderPrefix, "/") {
		folderPrefix += "/"
	}

	query := `
		SELECT file_id FROM file_metadata
		WHERE workspace_id = ? AND relative_path LIKE ?
		LIMIT ? OFFSET ?
	`

	pattern := folderPrefix + "%"
	return r.queryMetadataByIDs(ctx, workspaceID, query, workspaceID.String(), pattern, opts.Limit, opts.Offset)
}

// Helper functions

func (r *MetadataRepository) loadTags(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]string, error) {
	query := `SELECT tag FROM file_tags WHERE workspace_id = ? AND file_id = ?`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (r *MetadataRepository) loadContexts(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]string, error) {
	query := `SELECT context FROM file_contexts WHERE workspace_id = ? AND file_id = ?`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		contexts = append(contexts, c)
	}

	return contexts, nil
}

func (r *MetadataRepository) loadSuggestedContexts(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]string, error) {
	query := `SELECT context FROM file_context_suggestions WHERE workspace_id = ? AND file_id = ?`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		contexts = append(contexts, c)
	}

	return contexts, nil
}

func (r *MetadataRepository) touchUpdatedAt(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `UPDATE file_metadata SET updated_at = ? WHERE workspace_id = ? AND file_id = ?`
	_, err := r.conn.Exec(ctx, query, time.Now().UnixMilli(), workspaceID.String(), fileID.String())
	return err
}

func (r *MetadataRepository) queryMetadataByIDs(ctx context.Context, workspaceID entity.WorkspaceID, query string, args ...interface{}) ([]*entity.FileMetadata, error) {
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var ids []entity.FileID
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, entity.FileID(fileID))
	}
	rows.Close()

	var metadata []*entity.FileMetadata
	for _, fileID := range ids {
		meta, err := r.Get(ctx, workspaceID, fileID)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			metadata = append(metadata, meta)
		}
	}

	return metadata, nil
}

// GetLanguageFacet returns detected language facet counts.
func (r *MetadataRepository) GetLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(detected_language, 'unknown') as language, COUNT(*) as count
		FROM file_metadata
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY language ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var language string
		var count int
		if err := rows.Scan(&language, &count); err != nil {
			return nil, err
		}
		counts[language] = count
	}

	return counts, nil
}

// GetAICategoryFacet returns AI category facet counts.
func (r *MetadataRepository) GetAICategoryFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(ai_category, 'uncategorized') as category, COUNT(*) as count
		FROM file_metadata
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY category ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		counts[category] = count
	}

	return counts, nil
}

// GetAuthorFacet returns author facet counts from file_authors table.
func (r *MetadataRepository) GetAuthorFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT fa.name, COUNT(DISTINCT fa.file_id) as count
		FROM file_authors fa
		INNER JOIN files f ON f.workspace_id = fa.workspace_id AND f.id = fa.file_id
		WHERE fa.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fa.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY fa.name ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var author string
		var count int
		if err := rows.Scan(&author, &count); err != nil {
			return nil, err
		}
		counts[author] = count
	}

	return counts, nil
}

// GetPublicationYearFacet returns publication year facet counts from file_publication_info table.
func (r *MetadataRepository) GetPublicationYearFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(fpi.publication_year, 'unknown') as year, COUNT(DISTINCT fpi.file_id) as count
		FROM file_publication_info fpi
		INNER JOIN files f ON f.workspace_id = fpi.workspace_id AND f.id = fpi.file_id
		WHERE fpi.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fpi.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY year ORDER BY year DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var year string
		var count int
		if err := rows.Scan(&year, &count); err != nil {
			return nil, err
		}
		counts[year] = count
	}

	return counts, nil
}

// GetSentimentFacet returns sentiment facet counts from file_sentiment table.
func (r *MetadataRepository) GetSentimentFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(fs.overall_sentiment, 'unknown') as sentiment, COUNT(DISTINCT fs.file_id) as count
		FROM file_sentiment fs
		INNER JOIN files f ON f.workspace_id = fs.workspace_id AND f.id = fs.file_id
		WHERE fs.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fs.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY sentiment ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var sentiment string
		var count int
		if err := rows.Scan(&sentiment, &count); err != nil {
			return nil, err
		}
		counts[sentiment] = count
	}

	return counts, nil
}

// GetDuplicateTypeFacet returns duplicate type facet counts from file_duplicates table.
func (r *MetadataRepository) GetDuplicateTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(fd.type, 'unknown') as duplicate_type, COUNT(DISTINCT fd.file_id) as count
		FROM file_duplicates fd
		INNER JOIN files f ON f.workspace_id = fd.workspace_id AND f.id = fd.file_id
		WHERE fd.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fd.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY duplicate_type ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var dupType string
		var count int
		if err := rows.Scan(&dupType, &count); err != nil {
			return nil, err
		}
		counts[dupType] = count
	}

	return counts, nil
}

// GetLocationFacet returns location facet counts from file_locations table.
func (r *MetadataRepository) GetLocationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT fl.name, COUNT(DISTINCT fl.file_id) as count
		FROM file_locations fl
		INNER JOIN files f ON f.workspace_id = fl.workspace_id AND f.id = fl.file_id
		WHERE fl.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fl.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY fl.name ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var location string
		var count int
		if err := rows.Scan(&location, &count); err != nil {
			return nil, err
		}
		counts[location] = count
	}

	return counts, nil
}

// GetOrganizationFacet returns organization facet counts from file_organizations table.
func (r *MetadataRepository) GetOrganizationFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT fo.name, COUNT(DISTINCT fo.file_id) as count
		FROM file_organizations fo
		INNER JOIN files f ON f.workspace_id = fo.workspace_id AND f.id = fo.file_id
		WHERE fo.workspace_id = ?
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

	query += ` GROUP BY fo.name ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var org string
		var count int
		if err := rows.Scan(&org, &count); err != nil {
			return nil, err
		}
		counts[org] = count
	}

	return counts, nil
}

// GetContentTypeFacet returns content type facet counts from suggested_taxonomy table.
// Optimized: removed unnecessary JOIN with files table since we only need counts from suggested_taxonomy.
func (r *MetadataRepository) GetContentTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(content_type, 'unknown') as content_type, COUNT(DISTINCT file_id) as count
		FROM suggested_taxonomy
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY content_type ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		// Check for cancellation during row processing (critical for large datasets)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		var contentType string
		var count int
		if err := rows.Scan(&contentType, &count); err != nil {
			return nil, err
		}
		counts[contentType] = count
	}

	return counts, nil
}

// GetPurposeFacet returns purpose facet counts from suggested_taxonomy table.
func (r *MetadataRepository) GetPurposeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(st.purpose, 'unknown') as purpose, COUNT(DISTINCT st.file_id) as count
		FROM suggested_taxonomy st
		INNER JOIN files f ON f.workspace_id = st.workspace_id AND f.id = st.file_id
		WHERE st.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND st.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY purpose ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var purpose string
		var count int
		if err := rows.Scan(&purpose, &count); err != nil {
			return nil, err
		}
		counts[purpose] = count
	}

	return counts, nil
}

// GetAudienceFacet returns audience facet counts from suggested_taxonomy table.
func (r *MetadataRepository) GetAudienceFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(st.audience, 'unknown') as audience, COUNT(DISTINCT st.file_id) as count
		FROM suggested_taxonomy st
		INNER JOIN files f ON f.workspace_id = st.workspace_id AND f.id = st.file_id
		WHERE st.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND st.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY audience ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var audience string
		var count int
		if err := rows.Scan(&audience, &count); err != nil {
			return nil, err
		}
		counts[audience] = count
	}

	return counts, nil
}

// GetDomainFacet returns domain facet counts from suggested_taxonomy table.
func (r *MetadataRepository) GetDomainFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(st.domain, 'unknown') as domain, COUNT(DISTINCT st.file_id) as count
		FROM suggested_taxonomy st
		INNER JOIN files f ON f.workspace_id = st.workspace_id AND f.id = st.file_id
		WHERE st.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND st.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY domain ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var domain string
		var count int
		if err := rows.Scan(&domain, &count); err != nil {
			return nil, err
		}
		counts[domain] = count
	}

	return counts, nil
}

// GetSubdomainFacet returns subdomain facet counts from suggested_taxonomy table.
func (r *MetadataRepository) GetSubdomainFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(st.subdomain, 'unknown') as subdomain, COUNT(DISTINCT st.file_id) as count
		FROM suggested_taxonomy st
		INNER JOIN files f ON f.workspace_id = st.workspace_id AND f.id = st.file_id
		WHERE st.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND st.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY subdomain ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var subdomain string
		var count int
		if err := rows.Scan(&subdomain, &count); err != nil {
			return nil, err
		}
		counts[subdomain] = count
	}

	return counts, nil
}

// GetTopicFacet returns topic facet counts from suggested_taxonomy_topics table.
func (r *MetadataRepository) GetTopicFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT stt.topic, COUNT(DISTINCT stt.file_id) as count
		FROM suggested_taxonomy_topics stt
		INNER JOIN files f ON f.workspace_id = stt.workspace_id AND f.id = stt.file_id
		WHERE stt.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND stt.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY stt.topic ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var topic string
		var count int
		if err := rows.Scan(&topic, &count); err != nil {
			return nil, err
		}
		counts[topic] = count
	}

	return counts, nil
}

// GetFolderNameFacet returns folder name facet counts by grouping files by folder name.
// Extracts folder name from relative_path: for "compras/file.txt", folder name is "compras".
// Root files (no '/') are grouped as "." (current directory).
func (r *MetadataRepository) GetFolderNameFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT 
			CASE 
				WHEN INSTR(f.relative_path, '/') > 0 THEN
					-- Extract first-level folder name: get the part before the first '/'
					-- For "Libros/file.txt", this gives "Libros"
					-- For "Libros/subfolder/file.txt", this also gives "Libros" (first level only)
					SUBSTR(f.relative_path, 1, INSTR(f.relative_path, '/') - 1)
				ELSE '.'
			END as folder_name,
			COUNT(DISTINCT f.id) as count
		FROM files f
		WHERE f.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND f.id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY folder_name ORDER BY count DESC`

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

		var folderName string
		var count int
		if err := rows.Scan(&folderName, &count); err != nil {
			continue
		}
		counts[folderName] = count
	}

	return counts, nil
}

// GetEventFacet returns historical event facet counts from file_events table.
func (r *MetadataRepository) GetEventFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT fe.name, COUNT(DISTINCT fe.file_id) as count
		FROM file_events fe
		INNER JOIN files f ON f.workspace_id = fe.workspace_id AND f.id = fe.file_id
		WHERE fe.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fe.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY fe.name ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		counts[name] = count
	}

	return counts, nil
}

// GetCitationTypeFacet returns citation type facet counts from file_citations table.
func (r *MetadataRepository) GetCitationTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(fc.type, 'unknown') as citation_type, COUNT(DISTINCT fc.file_id) as count
		FROM file_citations fc
		INNER JOIN files f ON f.workspace_id = fc.workspace_id AND f.id = fc.file_id
		WHERE fc.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		query += ` AND fc.file_id IN (` + joinStrings(placeholders, ",") + `)`
	}

	query += ` GROUP BY citation_type ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var citationType string
		var count int
		if err := rows.Scan(&citationType, &count); err != nil {
			return nil, err
		}
		counts[citationType] = count
	}

	return counts, nil
}

// GetRelationshipTypeFacet returns document relationship type facet counts.
func (r *MetadataRepository) GetRelationshipTypeFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	query := `
		SELECT COALESCE(dr.type, 'unknown') as rel_type, COUNT(*) as count
		FROM document_relationships dr
		LEFT JOIN documents df ON df.workspace_id = dr.workspace_id AND df.id = dr.from_document_id
		LEFT JOIN documents dt ON dt.workspace_id = dr.workspace_id AND dt.id = dr.to_document_id
		WHERE dr.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	if len(fileIDs) > 0 {
		placeholders := make([]string, len(fileIDs))
		for i := range fileIDs {
			placeholders[i] = "?"
			args = append(args, fileIDs[i].String())
		}
		for i := range fileIDs {
			args = append(args, fileIDs[i].String())
		}
		filter := ` AND (df.file_id IN (` + joinStrings(placeholders, ",") + `) OR dt.file_id IN (` + joinStrings(placeholders, ",") + `))`
		query += filter
	}

	query += ` GROUP BY rel_type ORDER BY count DESC`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var relType string
		var count int
		if err := rows.Scan(&relType, &count); err != nil {
			return nil, err
		}
		counts[relType] = count
	}

	return counts, nil
}
