package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// InferredProjectRepository implements repository.InferredProjectRepository using SQLite.
type InferredProjectRepository struct {
	conn *Connection
}

// NewInferredProjectRepository creates a new SQLite inferred project repository.
func NewInferredProjectRepository(conn *Connection) *InferredProjectRepository {
	return &InferredProjectRepository{conn: conn}
}

// GetByID retrieves an inferred project by ID.
func (r *InferredProjectRepository) GetByID(ctx context.Context, workspaceID entity.WorkspaceID, id string) (*repository.InferredProject, error) {
	query := `
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id)
	return r.scanInferredProject(row, workspaceID.String())
}

// GetByFolderPath retrieves an inferred project by folder path.
func (r *InferredProjectRepository) GetByFolderPath(ctx context.Context, workspaceID entity.WorkspaceID, folderPath string) (*repository.InferredProject, error) {
	query := `
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ? AND folder_path = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), folderPath)
	return r.scanInferredProject(row, workspaceID.String())
}

// Upsert inserts or updates an inferred project.
func (r *InferredProjectRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, project *repository.InferredProject) error {
	indicatorFilesJSON, err := json.Marshal(project.IndicatorFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal indicator files: %w", err)
	}

	query := `
		INSERT INTO inferred_projects (
			id, workspace_id, name, folder_path, nature, confidence, file_count,
			indicator_files, dominant_language, description, auto_created, linked_project_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, id) DO UPDATE SET
			name = excluded.name,
			folder_path = excluded.folder_path,
			nature = excluded.nature,
			confidence = excluded.confidence,
			file_count = excluded.file_count,
			indicator_files = excluded.indicator_files,
			dominant_language = excluded.dominant_language,
			description = excluded.description,
			auto_created = excluded.auto_created,
			linked_project_id = excluded.linked_project_id
	`

	autoCreated := 0
	if project.AutoCreated {
		autoCreated = 1
	}

	_, err = r.conn.Exec(ctx, query,
		project.ID,
		workspaceID.String(),
		project.Name,
		project.FolderPath,
		project.Nature,
		project.Confidence,
		project.FileCount,
		indicatorFilesJSON,
		project.DominantLanguage,
		project.Description,
		autoCreated,
		project.LinkedProjectID,
	)
	return err
}

// Delete removes an inferred project by ID.
func (r *InferredProjectRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, id string) error {
	query := `DELETE FROM inferred_projects WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), id)
	return err
}

// BulkUpsert inserts or updates multiple inferred projects.
func (r *InferredProjectRepository) BulkUpsert(ctx context.Context, workspaceID entity.WorkspaceID, projects []*repository.InferredProject) (int, error) {
	if len(projects) == 0 {
		return 0, nil
	}

	count := 0
	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO inferred_projects (
				id, workspace_id, name, folder_path, nature, confidence, file_count,
				indicator_files, dominant_language, description, auto_created, linked_project_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, id) DO UPDATE SET
				name = excluded.name,
				folder_path = excluded.folder_path,
				nature = excluded.nature,
				confidence = excluded.confidence,
				file_count = excluded.file_count,
				indicator_files = excluded.indicator_files,
				dominant_language = excluded.dominant_language,
				description = excluded.description,
				auto_created = excluded.auto_created,
				linked_project_id = excluded.linked_project_id
		`

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, project := range projects {
			indicatorFilesJSON, _ := json.Marshal(project.IndicatorFiles)
			autoCreated := 0
			if project.AutoCreated {
				autoCreated = 1
			}

			_, err := stmt.ExecContext(ctx,
				project.ID,
				workspaceID.String(),
				project.Name,
				project.FolderPath,
				project.Nature,
				project.Confidence,
				project.FileCount,
				indicatorFilesJSON,
				project.DominantLanguage,
				project.Description,
				autoCreated,
				project.LinkedProjectID,
			)
			if err != nil {
				return fmt.Errorf("failed to upsert project %s: %w", project.Name, err)
			}
			count++
		}
		return nil
	})

	return count, err
}

// BulkDelete removes multiple inferred projects by ID.
func (r *InferredProjectRepository) BulkDelete(ctx context.Context, workspaceID entity.WorkspaceID, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	count := 0
	err := r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		query := `DELETE FROM inferred_projects WHERE workspace_id = ? AND id = ?`
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, id := range ids {
			result, err := stmt.ExecContext(ctx, workspaceID.String(), id)
			if err != nil {
				return fmt.Errorf("failed to delete project %s: %w", id, err)
			}
			affected, _ := result.RowsAffected()
			count += int(affected)
		}
		return nil
	})

	return count, err
}

// List lists all inferred projects with pagination.
func (r *InferredProjectRepository) List(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.InferredProjectListOptions) ([]*repository.InferredProject, error) {
	query := fmt.Sprintf(`
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeProjectSortBy(opts.SortBy), inferredProjectSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInferredProjects(rows, workspaceID.String())
}

// ListByNature lists inferred projects by nature.
func (r *InferredProjectRepository) ListByNature(ctx context.Context, workspaceID entity.WorkspaceID, nature string, opts repository.InferredProjectListOptions) ([]*repository.InferredProject, error) {
	query := fmt.Sprintf(`
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ? AND nature = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeProjectSortBy(opts.SortBy), inferredProjectSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), nature, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInferredProjects(rows, workspaceID.String())
}

// ListByLanguage lists inferred projects by dominant language.
func (r *InferredProjectRepository) ListByLanguage(ctx context.Context, workspaceID entity.WorkspaceID, language string, opts repository.InferredProjectListOptions) ([]*repository.InferredProject, error) {
	query := fmt.Sprintf(`
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ? AND dominant_language = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeProjectSortBy(opts.SortBy), inferredProjectSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), language, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInferredProjects(rows, workspaceID.String())
}

// ListAboveConfidence lists inferred projects with confidence above threshold.
func (r *InferredProjectRepository) ListAboveConfidence(ctx context.Context, workspaceID entity.WorkspaceID, minConfidence float64, opts repository.InferredProjectListOptions) ([]*repository.InferredProject, error) {
	query := fmt.Sprintf(`
		SELECT id, name, folder_path, nature, confidence, file_count, indicator_files,
		       dominant_language, description, auto_created, linked_project_id
		FROM inferred_projects
		WHERE workspace_id = ? AND confidence >= ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sanitizeProjectSortBy(opts.SortBy), inferredProjectSortDirection(opts.SortDesc))

	rows, err := r.conn.Query(ctx, query, workspaceID.String(), minConfidence, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInferredProjects(rows, workspaceID.String())
}

// LinkToProject links an inferred project to a manually created project.
func (r *InferredProjectRepository) LinkToProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string, projectID string) error {
	query := `UPDATE inferred_projects SET linked_project_id = ? WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, projectID, workspaceID.String(), inferredID)
	return err
}

// UnlinkFromProject removes the link to a manually created project.
func (r *InferredProjectRepository) UnlinkFromProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string) error {
	query := `UPDATE inferred_projects SET linked_project_id = NULL WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), inferredID)
	return err
}

// GetLinkedProject returns the linked project ID if any.
func (r *InferredProjectRepository) GetLinkedProject(ctx context.Context, workspaceID entity.WorkspaceID, inferredID string) (*string, error) {
	query := `SELECT linked_project_id FROM inferred_projects WHERE workspace_id = ? AND id = ?`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), inferredID)
	var linkedID sql.NullString
	if err := row.Scan(&linkedID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if linkedID.Valid {
		return &linkedID.String, nil
	}
	return nil, nil
}

// GetStats returns inferred project statistics.
func (r *InferredProjectRepository) GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.InferredProjectStats, error) {
	stats := &repository.InferredProjectStats{
		NatureCounts:   make(map[string]int),
		LanguageCounts: make(map[string]int),
	}

	// Total counts
	query := `
		SELECT
			COUNT(*),
			COUNT(CASE WHEN linked_project_id IS NOT NULL THEN 1 END),
			COUNT(CASE WHEN linked_project_id IS NULL THEN 1 END),
			COALESCE(AVG(confidence), 0),
			COUNT(CASE WHEN confidence >= 0.8 THEN 1 END)
		FROM inferred_projects
		WHERE workspace_id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String())
	if err := row.Scan(
		&stats.TotalProjects,
		&stats.LinkedProjects,
		&stats.UnlinkedProjects,
		&stats.AverageConfidence,
		&stats.HighConfidenceCount,
	); err != nil {
		return nil, err
	}

	// Nature counts
	query = `SELECT nature, COUNT(*) FROM inferred_projects WHERE workspace_id = ? GROUP BY nature`
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

	// Language counts
	query = `SELECT dominant_language, COUNT(*) FROM inferred_projects WHERE workspace_id = ? AND dominant_language != '' GROUP BY dominant_language`
	rows, err = r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err != nil {
			return nil, err
		}
		stats.LanguageCounts[lang] = count
	}

	return stats, nil
}

// Count returns total inferred project count.
func (r *InferredProjectRepository) Count(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	query := `SELECT COUNT(*) FROM inferred_projects WHERE workspace_id = ?`
	row := r.conn.QueryRow(ctx, query, workspaceID.String())
	var count int
	err := row.Scan(&count)
	return count, err
}

// GetNatureFacet returns project count by nature.
func (r *InferredProjectRepository) GetNatureFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `SELECT nature, COUNT(*) FROM inferred_projects WHERE workspace_id = ? GROUP BY nature`
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

// GetLanguageFacet returns project count by language.
func (r *InferredProjectRepository) GetLanguageFacet(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]int, error) {
	query := `SELECT COALESCE(dominant_language, 'unknown'), COUNT(*) FROM inferred_projects WHERE workspace_id = ? GROUP BY dominant_language`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err != nil {
			return nil, err
		}
		result[lang] = count
	}
	return result, nil
}

// GetConfidenceRangeFacet returns project count by confidence ranges.
func (r *InferredProjectRepository) GetConfidenceRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]repository.NumericRangeCount, error) {
	query := `
		SELECT
			CASE
				WHEN confidence < 0.3 THEN 'Low (< 0.3)'
				WHEN confidence < 0.5 THEN 'Medium-Low (0.3-0.5)'
				WHEN confidence < 0.7 THEN 'Medium (0.5-0.7)'
				WHEN confidence < 0.9 THEN 'High (0.7-0.9)'
				ELSE 'Very High (0.9+)'
			END as range_label,
			COUNT(*)
		FROM inferred_projects
		WHERE workspace_id = ?
		GROUP BY range_label
		ORDER BY
			CASE range_label
				WHEN 'Low (< 0.3)' THEN 1
				WHEN 'Medium-Low (0.3-0.5)' THEN 2
				WHEN 'Medium (0.5-0.7)' THEN 3
				WHEN 'High (0.7-0.9)' THEN 4
				ELSE 5
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

// GetFileCountRangeFacet returns project count by file count ranges.
func (r *InferredProjectRepository) GetFileCountRangeFacet(ctx context.Context, workspaceID entity.WorkspaceID) ([]repository.NumericRangeCount, error) {
	query := `
		SELECT
			CASE
				WHEN file_count <= 5 THEN 'Tiny (1-5)'
				WHEN file_count <= 20 THEN 'Small (6-20)'
				WHEN file_count <= 50 THEN 'Medium (21-50)'
				WHEN file_count <= 100 THEN 'Large (51-100)'
				ELSE 'Very Large (100+)'
			END as range_label,
			COUNT(*)
		FROM inferred_projects
		WHERE workspace_id = ?
		GROUP BY range_label
		ORDER BY
			CASE range_label
				WHEN 'Tiny (1-5)' THEN 1
				WHEN 'Small (6-20)' THEN 2
				WHEN 'Medium (21-50)' THEN 3
				WHEN 'Large (51-100)' THEN 4
				ELSE 5
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

// DeleteAll removes all inferred projects for a workspace.
func (r *InferredProjectRepository) DeleteAll(ctx context.Context, workspaceID entity.WorkspaceID) error {
	query := `DELETE FROM inferred_projects WHERE workspace_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String())
	return err
}

// scanInferredProject scans a single inferred project from a row.
func (r *InferredProjectRepository) scanInferredProject(row *sql.Row, workspaceID string) (*repository.InferredProject, error) {
	var id, name, folderPath, nature, dominantLang, description string
	var confidence float64
	var fileCount int
	var indicatorFilesJSON []byte
	var autoCreated int
	var linkedProjectID sql.NullString

	err := row.Scan(&id, &name, &folderPath, &nature, &confidence, &fileCount,
		&indicatorFilesJSON, &dominantLang, &description, &autoCreated, &linkedProjectID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	project := &repository.InferredProject{
		ID:               id,
		WorkspaceID:      workspaceID,
		Name:             name,
		FolderPath:       folderPath,
		Nature:           nature,
		Confidence:       confidence,
		FileCount:        fileCount,
		DominantLanguage: dominantLang,
		Description:      description,
		AutoCreated:      autoCreated == 1,
	}

	if linkedProjectID.Valid {
		project.LinkedProjectID = &linkedProjectID.String
	}

	if len(indicatorFilesJSON) > 0 {
		if err := json.Unmarshal(indicatorFilesJSON, &project.IndicatorFiles); err != nil {
			return nil, fmt.Errorf("failed to unmarshal indicator files: %w", err)
		}
	}

	return project, nil
}

// scanInferredProjects scans multiple inferred projects from rows.
func (r *InferredProjectRepository) scanInferredProjects(rows *sql.Rows, workspaceID string) ([]*repository.InferredProject, error) {
	var projects []*repository.InferredProject

	for rows.Next() {
		var id, name, folderPath, nature, dominantLang, description string
		var confidence float64
		var fileCount int
		var indicatorFilesJSON []byte
		var autoCreated int
		var linkedProjectID sql.NullString

		err := rows.Scan(&id, &name, &folderPath, &nature, &confidence, &fileCount,
			&indicatorFilesJSON, &dominantLang, &description, &autoCreated, &linkedProjectID)
		if err != nil {
			return nil, err
		}

		project := &repository.InferredProject{
			ID:               id,
			WorkspaceID:      workspaceID,
			Name:             name,
			FolderPath:       folderPath,
			Nature:           nature,
			Confidence:       confidence,
			FileCount:        fileCount,
			DominantLanguage: dominantLang,
			Description:      description,
			AutoCreated:      autoCreated == 1,
		}

		if linkedProjectID.Valid {
			project.LinkedProjectID = &linkedProjectID.String
		}

		if len(indicatorFilesJSON) > 0 {
			if err := json.Unmarshal(indicatorFilesJSON, &project.IndicatorFiles); err != nil {
				return nil, fmt.Errorf("failed to unmarshal indicator files: %w", err)
			}
		}

		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// sanitizeProjectSortBy returns a safe column name for sorting.
func sanitizeProjectSortBy(sortBy string) string {
	allowed := map[string]bool{
		"name":              true,
		"folder_path":       true,
		"nature":            true,
		"confidence":        true,
		"file_count":        true,
		"dominant_language": true,
	}
	if allowed[sortBy] {
		return sortBy
	}
	return "confidence"
}

// inferredProjectSortDirection returns ASC or DESC based on the boolean.
func inferredProjectSortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}
