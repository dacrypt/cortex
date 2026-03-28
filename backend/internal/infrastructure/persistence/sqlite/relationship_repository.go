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

// RelationshipRepository implements repository.RelationshipRepository using SQLite.
type RelationshipRepository struct {
	conn *Connection
}

// NewRelationshipRepository creates a new SQLite relationship repository.
func NewRelationshipRepository(conn *Connection) *RelationshipRepository {
	return &RelationshipRepository{conn: conn}
}

// Create inserts a new document relationship.
func (r *RelationshipRepository) Create(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.DocumentRelationship) error {
	if !rel.Type.IsValid() {
		return fmt.Errorf("invalid relationship type: %s", rel.Type)
	}

	var metadataJSON []byte
	if rel.Metadata != nil {
		data, err := json.Marshal(rel.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = data
	}

	query := `
		INSERT INTO document_relationships (
			id, workspace_id, from_document_id, to_document_id, type, strength, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		rel.ID.String(),
		workspaceID.String(),
		rel.FromDocument.String(),
		rel.ToDocument.String(),
		rel.Type.String(),
		rel.Strength,
		metadataJSON,
		rel.CreatedAt.UnixMilli(),
	)
	return err
}

// Get retrieves a relationship by ID.
func (r *RelationshipRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID, id entity.RelationshipID) (*entity.DocumentRelationship, error) {
	query := `
		SELECT id, from_document_id, to_document_id, type, strength, metadata, created_at
		FROM document_relationships
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return r.scanRelationship(row, workspaceID)
}

// Delete removes a relationship by ID.
func (r *RelationshipRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.RelationshipID) error {
	query := `DELETE FROM document_relationships WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), id.String())
	return err
}

// DeleteByDocuments removes a relationship between two documents.
func (r *RelationshipRepository) DeleteByDocuments(ctx context.Context, workspaceID entity.WorkspaceID, fromDocID, toDocID entity.DocumentID, relType entity.RelationshipType) error {
	query := `
		DELETE FROM document_relationships
		WHERE workspace_id = ? AND from_document_id = ? AND to_document_id = ? AND type = ?
	`
	_, err := r.conn.Exec(ctx, query,
		workspaceID.String(),
		fromDocID.String(),
		toDocID.String(),
		relType.String(),
	)
	return err
}

// GetOutgoing returns all outgoing relationships of a specific type from a document.
func (r *RelationshipRepository) GetOutgoing(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]*entity.DocumentRelationship, error) {
	query := `
		SELECT id, from_document_id, to_document_id, type, strength, metadata, created_at
		FROM document_relationships
		WHERE workspace_id = ? AND from_document_id = ? AND type = ?
		ORDER BY created_at DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String(), relType.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*entity.DocumentRelationship
	for rows.Next() {
		rel, err := r.scanRelationship(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}
	return relationships, rows.Err()
}

// GetIncoming returns all incoming relationships of a specific type to a document.
func (r *RelationshipRepository) GetIncoming(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]*entity.DocumentRelationship, error) {
	query := `
		SELECT id, from_document_id, to_document_id, type, strength, metadata, created_at
		FROM document_relationships
		WHERE workspace_id = ? AND to_document_id = ? AND type = ?
		ORDER BY created_at DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String(), relType.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*entity.DocumentRelationship
	for rows.Next() {
		rel, err := r.scanRelationship(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}
	return relationships, rows.Err()
}

// GetAllOutgoing returns all outgoing relationships from a document.
func (r *RelationshipRepository) GetAllOutgoing(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentRelationship, error) {
	query := `
		SELECT id, from_document_id, to_document_id, type, strength, metadata, created_at
		FROM document_relationships
		WHERE workspace_id = ? AND from_document_id = ?
		ORDER BY type, created_at DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*entity.DocumentRelationship
	for rows.Next() {
		rel, err := r.scanRelationship(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}
	return relationships, rows.Err()
}

// GetAllIncoming returns all incoming relationships to a document.
func (r *RelationshipRepository) GetAllIncoming(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentRelationship, error) {
	query := `
		SELECT id, from_document_id, to_document_id, type, strength, metadata, created_at
		FROM document_relationships
		WHERE workspace_id = ? AND to_document_id = ?
		ORDER BY type, created_at DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*entity.DocumentRelationship
	for rows.Next() {
		rel, err := r.scanRelationship(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}
	return relationships, rows.Err()
}

// GetRelated returns document IDs related to a document via a specific relationship type.
func (r *RelationshipRepository) GetRelated(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType) ([]entity.DocumentID, error) {
	query := `
		SELECT to_document_id
		FROM document_relationships
		WHERE workspace_id = ? AND from_document_id = ? AND type = ?
		ORDER BY strength DESC, created_at DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String(), relType.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docIDs []entity.DocumentID
	for rows.Next() {
		var docIDStr string
		if err := rows.Scan(&docIDStr); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, entity.DocumentID(docIDStr))
	}
	return docIDs, rows.Err()
}

// Traverse performs graph traversal starting from a document.
func (r *RelationshipRepository) Traverse(ctx context.Context, workspaceID entity.WorkspaceID, startDocID entity.DocumentID, relType entity.RelationshipType, maxDepth int) ([]entity.DocumentID, error) {
	if maxDepth <= 0 {
		return []entity.DocumentID{}, nil
	}

	visited := make(map[entity.DocumentID]bool)
	var result []entity.DocumentID
	queue := []struct {
		docID entity.DocumentID
		depth int
	}{{startDocID, 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.docID] || current.depth >= maxDepth {
			continue
		}
		visited[current.docID] = true

		// Get outgoing relationships
		related, err := r.GetRelated(ctx, workspaceID, current.docID, relType)
		if err != nil {
			return nil, err
		}

		for _, docID := range related {
			if !visited[docID] {
				result = append(result, docID)
				queue = append(queue, struct {
					docID entity.DocumentID
					depth int
				}{docID, current.depth + 1})
			}
		}
	}

	return result, nil
}

// GetReplacementChain returns the chain of documents that replace each other.
func (r *RelationshipRepository) GetReplacementChain(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]entity.DocumentID, error) {
	var chain []entity.DocumentID
	currentID := docID
	visited := make(map[entity.DocumentID]bool)

	for {
		if visited[currentID] {
			break // Cycle detected
		}
		visited[currentID] = true
		chain = append(chain, currentID)

		// Find what replaces current document
		incoming, err := r.GetIncoming(ctx, workspaceID, currentID, entity.RelationshipReplaces)
		if err != nil {
			return nil, err
		}
		if len(incoming) == 0 {
			break
		}
		// Take the most recent replacement
		currentID = incoming[0].FromDocument
	}

	return chain, nil
}

// scanRelationship scans a relationship from a database row.
func (r *RelationshipRepository) scanRelationship(scanner interface {
	Scan(dest ...interface{}) error
}, workspaceID entity.WorkspaceID) (*entity.DocumentRelationship, error) {
	var id, fromDocID, toDocID, typeStr string
	var strength sql.NullFloat64
	var metadataJSON sql.NullString
	var createdAt int64

	err := scanner.Scan(&id, &fromDocID, &toDocID, &typeStr, &strength, &metadataJSON, &createdAt)
	if err != nil {
		return nil, err
	}

	rel := &entity.DocumentRelationship{
		ID:           entity.RelationshipID(id),
		WorkspaceID:  workspaceID,
		FromDocument: entity.DocumentID(fromDocID),
		ToDocument:   entity.DocumentID(toDocID),
		Type:         entity.RelationshipType(typeStr),
		CreatedAt:    time.UnixMilli(createdAt),
		Metadata:     make(map[string]interface{}),
	}

	if strength.Valid {
		rel.Strength = strength.Float64
	} else {
		rel.Strength = 1.0
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &rel.Metadata); err != nil {
			// Log error but don't fail - metadata is optional
			rel.Metadata = make(map[string]interface{})
		}
	}

	return rel, nil
}

// AddProjectRelationship adds a relationship between projects.
func (r *RelationshipRepository) AddProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.ProjectRelationship) error {
	if !rel.Type.IsValid() {
		return fmt.Errorf("invalid relationship type: %s", rel.Type)
	}

	query := `
		INSERT INTO project_relationships (
			workspace_id, from_project_id, to_project_id, type, description, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		workspaceID.String(),
		rel.FromProjectID.String(),
		rel.ToProjectID.String(),
		rel.Type.String(),
		rel.Description,
		rel.CreatedAt.UnixMilli(),
	)
	return err
}

// RemoveProjectRelationship removes a relationship between projects.
func (r *RelationshipRepository) RemoveProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, fromProjectID, toProjectID entity.ProjectID, relType entity.RelationshipType) error {
	query := `
		DELETE FROM project_relationships
		WHERE workspace_id = ? AND from_project_id = ? AND to_project_id = ? AND type = ?
	`
	_, err := r.conn.Exec(ctx, query,
		workspaceID.String(),
		fromProjectID.String(),
		toProjectID.String(),
		relType.String(),
	)
	return err
}

// GetProjectRelationships gets relationships for a project.
func (r *RelationshipRepository) GetProjectRelationships(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, relType *entity.RelationshipType) ([]*entity.ProjectRelationship, error) {
	var query string
	var args []interface{}

	if relType != nil {
		query = `
			SELECT from_project_id, to_project_id, type, description, created_at
			FROM project_relationships
			WHERE workspace_id = ? AND from_project_id = ? AND type = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{workspaceID.String(), projectID.String(), relType.String()}
	} else {
		query = `
			SELECT from_project_id, to_project_id, type, description, created_at
			FROM project_relationships
			WHERE workspace_id = ? AND from_project_id = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{workspaceID.String(), projectID.String()}
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rels []*entity.ProjectRelationship
	for rows.Next() {
		rel, err := r.scanProjectRelationship(rows)
		if err != nil {
			return nil, err
		}
		rels = append(rels, rel)
	}

	return rels, rows.Err()
}

// GetRelatedProjects gets related project IDs.
func (r *RelationshipRepository) GetRelatedProjects(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, relType *entity.RelationshipType) ([]entity.ProjectID, error) {
	var query string
	var args []interface{}

	if relType != nil {
		query = `
			SELECT to_project_id
			FROM project_relationships
			WHERE workspace_id = ? AND from_project_id = ? AND type = ?
		`
		args = []interface{}{workspaceID.String(), projectID.String(), relType.String()}
	} else {
		query = `
			SELECT to_project_id
			FROM project_relationships
			WHERE workspace_id = ? AND from_project_id = ?
		`
		args = []interface{}{workspaceID.String(), projectID.String()}
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projectIDs []entity.ProjectID
	for rows.Next() {
		var toProjectIDStr string
		if err := rows.Scan(&toProjectIDStr); err != nil {
			return nil, err
		}
		projectIDs = append(projectIDs, entity.ProjectID(toProjectIDStr))
	}

	return projectIDs, rows.Err()
}

// scanProjectRelationship scans a project relationship from a row.
func (r *RelationshipRepository) scanProjectRelationship(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ProjectRelationship, error) {
	var fromProjectIDStr, toProjectIDStr, typeStr, description string
	var createdAt int64

	err := scanner.Scan(&fromProjectIDStr, &toProjectIDStr, &typeStr, &description, &createdAt)
	if err != nil {
		return nil, err
	}

	rel := &entity.ProjectRelationship{
		FromProjectID: entity.ProjectID(fromProjectIDStr),
		ToProjectID:   entity.ProjectID(toProjectIDStr),
		Type:          entity.RelationshipType(typeStr),
		Description:   description,
		CreatedAt:     time.UnixMilli(createdAt),
	}

	return rel, nil
}

var _ repository.RelationshipRepository = (*RelationshipRepository)(nil)

