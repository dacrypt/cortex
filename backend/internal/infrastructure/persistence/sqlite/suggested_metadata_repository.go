package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// SuggestedMetadataRepository implements repository.SuggestedMetadataRepository using SQLite.
type SuggestedMetadataRepository struct {
	conn *Connection
}

// NewSuggestedMetadataRepository creates a new SQLite suggested metadata repository.
func NewSuggestedMetadataRepository(conn *Connection) *SuggestedMetadataRepository {
	return &SuggestedMetadataRepository{conn: conn}
}

// Upsert stores or updates suggested metadata for a file.
func (r *SuggestedMetadataRepository) Upsert(ctx context.Context, workspaceID entity.WorkspaceID, suggested *entity.SuggestedMetadata) error {
	if suggested == nil {
		return fmt.Errorf("suggested metadata is nil")
	}

	if suggested.GeneratedAt.IsZero() {
		suggested.GeneratedAt = time.Now()
	}
	if suggested.UpdatedAt.IsZero() {
		suggested.UpdatedAt = time.Now()
	}

	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Upsert main suggested_metadata record
		query := `
			INSERT INTO suggested_metadata (
				file_id, workspace_id, relative_path, confidence, source, generated_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(workspace_id, file_id) DO UPDATE SET
				confidence = excluded.confidence,
				source = excluded.source,
				updated_at = excluded.updated_at
		`
		_, err := tx.ExecContext(ctx, query,
			suggested.FileID.String(),
			workspaceID.String(),
			suggested.RelativePath,
			suggested.Confidence,
			suggested.Source,
			suggested.GeneratedAt.UnixMilli(),
			suggested.UpdatedAt.UnixMilli(),
		)
		if err != nil {
			return fmt.Errorf("failed to upsert suggested metadata: %w", err)
		}

		// Delete existing suggestions
		if err := r.deleteSuggestions(ctx, tx, workspaceID, suggested.FileID); err != nil {
			return err
		}

		// Insert suggested tags
		for _, tag := range suggested.SuggestedTags {
			if err := r.insertTag(ctx, tx, workspaceID, suggested.FileID, tag); err != nil {
				return err
			}
		}

		// Insert suggested projects
		for _, project := range suggested.SuggestedProjects {
			if err := r.insertProject(ctx, tx, workspaceID, suggested.FileID, project); err != nil {
				return err
			}
		}

		// Insert suggested taxonomy
		if suggested.SuggestedTaxonomy != nil {
			if err := r.insertTaxonomy(ctx, tx, workspaceID, suggested.FileID, suggested.SuggestedTaxonomy); err != nil {
				return err
			}
		}

		// Insert suggested fields
		for _, field := range suggested.SuggestedFields {
			if err := r.insertField(ctx, tx, workspaceID, suggested.FileID, field); err != nil {
				return err
			}
		}

		return nil
	})
}

// Get retrieves suggested metadata for a file.
func (r *SuggestedMetadataRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.SuggestedMetadata, error) {
	query := `
		SELECT file_id, relative_path, confidence, source, generated_at, updated_at
		FROM suggested_metadata
		WHERE workspace_id = ? AND file_id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), fileID.String())

	var sm entity.SuggestedMetadata
	var fileIDStr, relativePath, source string
	var confidence float64
	var generatedAt, updatedAt int64

	err := row.Scan(&fileIDStr, &relativePath, &confidence, &source, &generatedAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get suggested metadata: %w", err)
	}

	sm.FileID = entity.FileID(fileIDStr)
	sm.WorkspaceID = workspaceID
	sm.RelativePath = relativePath
	sm.Confidence = confidence
	sm.Source = source
	sm.GeneratedAt = time.UnixMilli(generatedAt)
	sm.UpdatedAt = time.UnixMilli(updatedAt)
	sm.SuggestedFields = make(map[string]entity.SuggestedField)

	// Load tags
	tags, err := r.getTags(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	sm.SuggestedTags = tags

	// Load projects
	projects, err := r.getProjects(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	sm.SuggestedProjects = projects

	// Load taxonomy
	taxonomy, err := r.getTaxonomy(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	sm.SuggestedTaxonomy = taxonomy

	// Load fields
	fields, err := r.getFields(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		sm.SuggestedFields[field.FieldName] = field
	}

	return &sm, nil
}

// GetByPath retrieves suggested metadata by relative path.
func (r *SuggestedMetadataRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.SuggestedMetadata, error) {
	fileID := entity.NewFileID(relativePath)
	return r.Get(ctx, workspaceID, fileID)
}

// Delete removes suggested metadata for a file.
func (r *SuggestedMetadataRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		if err := r.deleteSuggestions(ctx, tx, workspaceID, fileID); err != nil {
			return err
		}

		query := `DELETE FROM suggested_metadata WHERE workspace_id = ? AND file_id = ?`
		_, err := tx.ExecContext(ctx, query, workspaceID.String(), fileID.String())
		return err
	})
}

// AcceptTag accepts a suggested tag and removes it from suggestions.
func (r *SuggestedMetadataRepository) AcceptTag(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, tag string) error {
	query := `DELETE FROM suggested_tags WHERE workspace_id = ? AND file_id = ? AND tag = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), tag)
	return err
}

// AcceptProject accepts a suggested project and removes it from suggestions.
func (r *SuggestedMetadataRepository) AcceptProject(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, projectName string) error {
	query := `DELETE FROM suggested_projects WHERE workspace_id = ? AND file_id = ? AND project_name = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), projectName)
	return err
}

// Helper methods

func (r *SuggestedMetadataRepository) deleteSuggestions(ctx context.Context, tx *sql.Tx, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	fileIDStr := fileID.String()
	wsIDStr := workspaceID.String()

	queries := []string{
		`DELETE FROM suggested_fields WHERE workspace_id = ? AND file_id = ?`,
		`DELETE FROM suggested_taxonomy_topics WHERE workspace_id = ? AND file_id = ?`,
		`DELETE FROM suggested_taxonomy WHERE workspace_id = ? AND file_id = ?`,
		`DELETE FROM suggested_projects WHERE workspace_id = ? AND file_id = ?`,
		`DELETE FROM suggested_tags WHERE workspace_id = ? AND file_id = ?`,
	}

	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q, wsIDStr, fileIDStr); err != nil {
			return fmt.Errorf("failed to delete suggestions: %w", err)
		}
	}

	return nil
}

func (r *SuggestedMetadataRepository) insertTag(ctx context.Context, tx *sql.Tx, workspaceID entity.WorkspaceID, fileID entity.FileID, tag entity.SuggestedTag) error {
	query := `
		INSERT INTO suggested_tags (
			workspace_id, file_id, tag, confidence, reason, source, category
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, file_id, tag) DO UPDATE SET
			confidence = excluded.confidence,
			reason = excluded.reason,
			source = excluded.source,
			category = excluded.category
	`
	_, err := tx.ExecContext(ctx, query,
		workspaceID.String(),
		fileID.String(),
		tag.Tag,
		tag.Confidence,
		tag.Reason,
		tag.Source,
		tag.Category,
	)
	return err
}

func (r *SuggestedMetadataRepository) insertProject(ctx context.Context, tx *sql.Tx, workspaceID entity.WorkspaceID, fileID entity.FileID, project entity.SuggestedProject) error {
	query := `
		INSERT INTO suggested_projects (
			workspace_id, file_id, project_id, project_name, confidence, reason, source, is_new
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, file_id, project_name) DO UPDATE SET
			project_id = excluded.project_id,
			confidence = excluded.confidence,
			reason = excluded.reason,
			source = excluded.source,
			is_new = excluded.is_new
	`
	var projectID *string
	if project.ProjectID != nil {
		id := project.ProjectID.String()
		projectID = &id
	}

	isNew := 0
	if project.IsNew {
		isNew = 1
	}

	_, err := tx.ExecContext(ctx, query,
		workspaceID.String(),
		fileID.String(),
		projectID,
		project.ProjectName,
		project.Confidence,
		project.Reason,
		project.Source,
		isNew,
	)
	return err
}

func (r *SuggestedMetadataRepository) insertTaxonomy(ctx context.Context, tx *sql.Tx, workspaceID entity.WorkspaceID, fileID entity.FileID, taxonomy *entity.SuggestedTaxonomy) error {
	query := `
		INSERT INTO suggested_taxonomy (
			workspace_id, file_id, category, subcategory, domain, subdomain,
			content_type, purpose, audience, language,
			category_confidence, domain_confidence, content_type_confidence,
			reasoning, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, file_id) DO UPDATE SET
			category = excluded.category,
			subcategory = excluded.subcategory,
			domain = excluded.domain,
			subdomain = excluded.subdomain,
			content_type = excluded.content_type,
			purpose = excluded.purpose,
			audience = excluded.audience,
			language = excluded.language,
			category_confidence = excluded.category_confidence,
			domain_confidence = excluded.domain_confidence,
			content_type_confidence = excluded.content_type_confidence,
			reasoning = excluded.reasoning,
			source = excluded.source
	`
	_, err := tx.ExecContext(ctx, query,
		workspaceID.String(),
		fileID.String(),
		taxonomy.Category,
		taxonomy.Subcategory,
		taxonomy.Domain,
		taxonomy.Subdomain,
		taxonomy.ContentType,
		taxonomy.Purpose,
		taxonomy.Audience,
		taxonomy.Language,
		taxonomy.CategoryConfidence,
		taxonomy.DomainConfidence,
		taxonomy.ContentTypeConfidence,
		taxonomy.Reasoning,
		taxonomy.Source,
	)
	if err != nil {
		return err
	}

	// Insert topics
	for _, topic := range taxonomy.Topic {
		topicQuery := `
			INSERT INTO suggested_taxonomy_topics (workspace_id, file_id, topic)
			VALUES (?, ?, ?)
			ON CONFLICT(workspace_id, file_id, topic) DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, topicQuery, workspaceID.String(), fileID.String(), topic); err != nil {
			return err
		}
	}

	return nil
}

func (r *SuggestedMetadataRepository) insertField(ctx context.Context, tx *sql.Tx, workspaceID entity.WorkspaceID, fileID entity.FileID, field entity.SuggestedField) error {
	// Convert value to JSON string
	valueJSON, err := json.Marshal(field.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal field value: %w", err)
	}

	query := `
		INSERT INTO suggested_fields (
			workspace_id, file_id, field_name, field_value, field_type, confidence, reason, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, file_id, field_name) DO UPDATE SET
			field_value = excluded.field_value,
			field_type = excluded.field_type,
			confidence = excluded.confidence,
			reason = excluded.reason,
			source = excluded.source
	`
	_, err = tx.ExecContext(ctx, query,
		workspaceID.String(),
		fileID.String(),
		field.FieldName,
		string(valueJSON),
		field.FieldType,
		field.Confidence,
		field.Reason,
		field.Source,
	)
	return err
}

func (r *SuggestedMetadataRepository) getTags(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]entity.SuggestedTag, error) {
	query := `
		SELECT tag, confidence, reason, source, category
		FROM suggested_tags
		WHERE workspace_id = ? AND file_id = ?
		ORDER BY confidence DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []entity.SuggestedTag
	for rows.Next() {
		var tag entity.SuggestedTag
		err := rows.Scan(&tag.Tag, &tag.Confidence, &tag.Reason, &tag.Source, &tag.Category)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

func (r *SuggestedMetadataRepository) getProjects(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]entity.SuggestedProject, error) {
	query := `
		SELECT project_id, project_name, confidence, reason, source, is_new
		FROM suggested_projects
		WHERE workspace_id = ? AND file_id = ?
		ORDER BY confidence DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []entity.SuggestedProject
	for rows.Next() {
		var project entity.SuggestedProject
		var projectIDStr sql.NullString
		var isNew int

		err := rows.Scan(&projectIDStr, &project.ProjectName, &project.Confidence, &project.Reason, &project.Source, &isNew)
		if err != nil {
			return nil, err
		}

		if projectIDStr.Valid {
			projectID := entity.ProjectID(projectIDStr.String)
			project.ProjectID = &projectID
		}
		project.IsNew = isNew == 1

		projects = append(projects, project)
	}

	return projects, rows.Err()
}

func (r *SuggestedMetadataRepository) getTaxonomy(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) (*entity.SuggestedTaxonomy, error) {
	query := `
		SELECT category, subcategory, domain, subdomain, content_type, purpose,
		       audience, language, category_confidence, domain_confidence,
		       content_type_confidence, reasoning, source
		FROM suggested_taxonomy
		WHERE workspace_id = ? AND file_id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), fileID.String())

	var taxonomy entity.SuggestedTaxonomy
	err := row.Scan(
		&taxonomy.Category,
		&taxonomy.Subcategory,
		&taxonomy.Domain,
		&taxonomy.Subdomain,
		&taxonomy.ContentType,
		&taxonomy.Purpose,
		&taxonomy.Audience,
		&taxonomy.Language,
		&taxonomy.CategoryConfidence,
		&taxonomy.DomainConfidence,
		&taxonomy.ContentTypeConfidence,
		&taxonomy.Reasoning,
		&taxonomy.Source,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Load topics
	topicsQuery := `
		SELECT topic FROM suggested_taxonomy_topics
		WHERE workspace_id = ? AND file_id = ?
		ORDER BY topic
	`
	rows, err := r.conn.Query(ctx, topicsQuery, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		taxonomy.Topic = append(taxonomy.Topic, topic)
	}

	return &taxonomy, rows.Err()
}

func (r *SuggestedMetadataRepository) getFields(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]entity.SuggestedField, error) {
	query := `
		SELECT field_name, field_value, field_type, confidence, reason, source
		FROM suggested_fields
		WHERE workspace_id = ? AND file_id = ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []entity.SuggestedField
	for rows.Next() {
		var field entity.SuggestedField
		var valueJSON string

		err := rows.Scan(&field.FieldName, &valueJSON, &field.FieldType, &field.Confidence, &field.Reason, &field.Source)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(valueJSON), &field.Value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal field value: %w", err)
		}

		fields = append(fields, field)
	}

	return fields, rows.Err()
}

