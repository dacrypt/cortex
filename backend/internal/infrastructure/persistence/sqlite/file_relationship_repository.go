package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// FileRelationshipRepository handles file relationship storage.
type FileRelationshipRepository struct {
	conn *Connection
}

// NewFileRelationshipRepository creates a new file relationship repository.
func NewFileRelationshipRepository(conn *Connection) *FileRelationshipRepository {
	return &FileRelationshipRepository{conn: conn}
}

// FileRelationship represents a relationship between two files.
type FileRelationship struct {
	FromFileID entity.FileID
	ToFileID   entity.FileID
	Type       string // "import", "export", "include", "require", "reference"
	Language   string
	Confidence float64
}

// UpsertRelationship creates or updates a file relationship.
func (r *FileRelationshipRepository) UpsertRelationship(ctx context.Context, workspaceID entity.WorkspaceID, rel FileRelationship) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_relationships (id, workspace_id, from_file_id, to_file_id, type, language, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, from_file_id, to_file_id, type) DO UPDATE SET
			language = excluded.language,
			confidence = excluded.confidence
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		rel.FromFileID.String(),
		rel.ToFileID.String(),
		rel.Type,
		rel.Language,
		rel.Confidence,
		time.Now().UnixMilli(),
	)
	return err
}

// DeleteByFile deletes all relationships for a file (both incoming and outgoing).
func (r *FileRelationshipRepository) DeleteByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	query := `
		DELETE FROM file_relationships 
		WHERE workspace_id = ? AND (from_file_id = ? OR to_file_id = ?)
	`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String(), fileID.String())
	return err
}

// GetRelationshipsFrom returns all relationships from a file.
func (r *FileRelationshipRepository) GetRelationshipsFrom(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]FileRelationship, error) {
	query := `
		SELECT from_file_id, to_file_id, type, language, confidence
		FROM file_relationships
		WHERE workspace_id = ? AND from_file_id = ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []FileRelationship
	for rows.Next() {
		var rel FileRelationship
		var fromID, toID string
		if err := rows.Scan(&fromID, &toID, &rel.Type, &rel.Language, &rel.Confidence); err != nil {
			return nil, err
		}
		rel.FromFileID = entity.FileID(fromID)
		rel.ToFileID = entity.FileID(toID)
		relationships = append(relationships, rel)
	}

	return relationships, nil
}

// GetRelationshipsTo returns all relationships to a file.
func (r *FileRelationshipRepository) GetRelationshipsTo(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]FileRelationship, error) {
	query := `
		SELECT from_file_id, to_file_id, type, language, confidence
		FROM file_relationships
		WHERE workspace_id = ? AND to_file_id = ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []FileRelationship
	for rows.Next() {
		var rel FileRelationship
		var fromID, toID string
		if err := rows.Scan(&fromID, &toID, &rel.Type, &rel.Language, &rel.Confidence); err != nil {
			return nil, err
		}
		rel.FromFileID = entity.FileID(fromID)
		rel.ToFileID = entity.FileID(toID)
		relationships = append(relationships, rel)
	}

	return relationships, nil
}

// FindFileByImportPath attempts to find a file by its import path.
// This is a helper method that tries to match import paths to file paths.
func (r *FileRelationshipRepository) FindFileByImportPath(ctx context.Context, workspaceID entity.WorkspaceID, importPath string) (*entity.FileID, error) {
	// This is a simplified implementation
	// Full implementation would need to understand module resolution
	// For now, try to match by relative path patterns
	query := `
		SELECT id FROM files
		WHERE workspace_id = ? AND (
			relative_path LIKE ? OR
			relative_path LIKE ? OR
			filename = ?
		)
		LIMIT 1
	`
	
	// Try different patterns
	pattern1 := "%" + importPath + "%"
	pattern2 := importPath + ".%"
	filename := fmt.Sprintf("%s.%s", importPath, "go") // Default to .go, should be language-specific
	
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), pattern1, pattern2, filename)
	var fileIDStr string
	err := row.Scan(&fileIDStr)
	if err != nil {
		return nil, err
	}
	
	fileID := entity.FileID(fileIDStr)
	return &fileID, nil
}

