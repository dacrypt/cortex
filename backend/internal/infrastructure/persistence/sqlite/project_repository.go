package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// ProjectRepository implements repository.ProjectRepository using SQLite.
type ProjectRepository struct {
	conn *Connection
}

// NewProjectRepository creates a new SQLite project repository.
func NewProjectRepository(conn *Connection) *ProjectRepository {
	return &ProjectRepository{conn: conn}
}

// Create inserts a new project.
func (r *ProjectRepository) Create(ctx context.Context, workspaceID entity.WorkspaceID, project *entity.Project) error {
	// Calculate full path if parent exists
	if project.ParentID != nil {
		parent, err := r.Get(ctx, workspaceID, *project.ParentID)
		if err != nil {
			return fmt.Errorf("failed to get parent project: %w", err)
		}
		project.UpdatePath(parent.Path)
	} else {
		project.Path = project.Name
	}

	// Serialize attributes to JSON
	attributesJSON := "{}"
	if project.Attributes != nil {
		attrsJSON, err := project.Attributes.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize attributes: %w", err)
		}
		attributesJSON = attrsJSON
	}

	query := `
		INSERT INTO projects (
			id, workspace_id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		project.ID.String(),
		workspaceID.String(),
		project.Name,
		project.Description,
		project.Nature.String(),
		attributesJSON,
		nilIfEmpty(project.ParentID),
		project.Path,
		project.CreatedAt.UnixMilli(),
		project.UpdatedAt.UnixMilli(),
	)
	return err
}

// Get retrieves a project by ID.
func (r *ProjectRepository) Get(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) (*entity.Project, error) {
	query := `
		SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return r.scanProject(row, workspaceID)
}

// GetByPath retrieves a project by hierarchical path.
func (r *ProjectRepository) GetByPath(ctx context.Context, workspaceID entity.WorkspaceID, path string) (*entity.Project, error) {
	query := `
		SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ? AND path = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), path)
	return r.scanProject(row, workspaceID)
}

// GetByName retrieves a project by name and optional parent.
func (r *ProjectRepository) GetByName(ctx context.Context, workspaceID entity.WorkspaceID, name string, parentID *entity.ProjectID) (*entity.Project, error) {
	var query string
	var args []interface{}

	if parentID == nil {
		query = `
			SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
			FROM projects
			WHERE workspace_id = ? AND name = ? AND parent_id IS NULL
		`
		args = []interface{}{workspaceID.String(), name}
	} else {
		query = `
			SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
			FROM projects
			WHERE workspace_id = ? AND name = ? AND parent_id = ?
		`
		args = []interface{}{workspaceID.String(), name, parentID.String()}
	}

	row := r.conn.QueryRow(ctx, query, args...)
	return r.scanProject(row, workspaceID)
}

// Update updates an existing project.
func (r *ProjectRepository) Update(ctx context.Context, workspaceID entity.WorkspaceID, project *entity.Project) error {
	// Recalculate path if parent changed
	if project.ParentID != nil {
		parent, err := r.Get(ctx, workspaceID, *project.ParentID)
		if err != nil {
			return fmt.Errorf("failed to get parent project: %w", err)
		}
		project.UpdatePath(parent.Path)
	} else {
		project.Path = project.Name
	}

	project.UpdatedAt = time.Now()

	// Serialize attributes to JSON
	attributesJSON := "{}"
	if project.Attributes != nil {
		attrsJSON, err := project.Attributes.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize attributes: %w", err)
		}
		attributesJSON = attrsJSON
	}

	query := `
		UPDATE projects
		SET name = ?, description = ?, nature = ?, attributes = ?, parent_id = ?, path = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`
	_, err := r.conn.Exec(ctx, query,
		project.Name,
		project.Description,
		project.Nature.String(),
		attributesJSON,
		nilIfEmpty(project.ParentID),
		project.Path,
		project.UpdatedAt.UnixMilli(),
		workspaceID.String(),
		project.ID.String(),
	)
	return err
}

// Delete deletes a project.
func (r *ProjectRepository) Delete(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) error {
	// Check if project has children
	children, err := r.GetChildren(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		return fmt.Errorf("cannot delete project with children")
	}

	query := `DELETE FROM projects WHERE workspace_id = ? AND id = ?`
	_, err = r.conn.Exec(ctx, query, workspaceID.String(), id.String())
	return err
}

// List returns all projects in a workspace.
func (r *ProjectRepository) List(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.Project, error) {
	query := `
		SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ?
		ORDER BY path
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*entity.Project
	for rows.Next() {
		project, err := r.scanProject(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// GetChildren returns all direct children of a project.
func (r *ProjectRepository) GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.ProjectID) ([]*entity.Project, error) {
	query := `
		SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ? AND parent_id = ?
		ORDER BY name
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), parentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*entity.Project
	for rows.Next() {
		project, err := r.scanProject(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// GetAncestors returns all ancestors of a project (parent, grandparent, etc.).
func (r *ProjectRepository) GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) ([]*entity.Project, error) {
	var ancestors []*entity.Project
	currentID := &id

	for currentID != nil {
		project, err := r.Get(ctx, workspaceID, *currentID)
		if err != nil {
			return nil, err
		}
		if project.ParentID == nil {
			break
		}
		parent, err := r.Get(ctx, workspaceID, *project.ParentID)
		if err != nil {
			return nil, err
		}
		ancestors = append([]*entity.Project{parent}, ancestors...)
		currentID = project.ParentID
	}

	return ancestors, nil
}

// GetDescendants returns all descendants of a project (children, grandchildren, etc.).
func (r *ProjectRepository) GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ProjectID) ([]*entity.Project, error) {
	var descendants []*entity.Project
	children, err := r.GetChildren(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		descendants = append(descendants, child)
		grandchildren, err := r.GetDescendants(ctx, workspaceID, child.ID)
		if err != nil {
			return nil, err
		}
		descendants = append(descendants, grandchildren...)
	}

	return descendants, nil
}

// GetRootProjects returns all root projects (no parent).
func (r *ProjectRepository) GetRootProjects(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.Project, error) {
	query := `
		SELECT id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ? AND parent_id IS NULL
		ORDER BY name
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*entity.Project
	for rows.Next() {
		project, err := r.scanProject(rows, workspaceID)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// AddDocument adds a document to a project.
func (r *ProjectRepository) AddDocument(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID, role entity.ProjectDocumentRole) error {
	query := `
		INSERT INTO project_documents (workspace_id, project_id, document_id, role, added_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, project_id, document_id) DO UPDATE SET
			role = excluded.role,
			added_at = excluded.added_at
	`
	_, err := r.conn.Exec(ctx, query,
		workspaceID.String(),
		projectID.String(),
		docID.String(),
		string(role),
		time.Now().UnixMilli(),
	)
	return err
}

// RemoveDocument removes a document from a project.
func (r *ProjectRepository) RemoveDocument(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID) error {
	query := `
		DELETE FROM project_documents
		WHERE workspace_id = ? AND project_id = ? AND document_id = ?
	`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), projectID.String(), docID.String())
	return err
}

// GetDocuments returns all documents in a project, optionally including subprojects.
func (r *ProjectRepository) GetDocuments(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, includeSubprojects bool) ([]entity.DocumentID, error) {
	var projectIDs []string
	projectIDs = append(projectIDs, projectID.String())

	if includeSubprojects {
		descendants, err := r.GetDescendants(ctx, workspaceID, projectID)
		if err != nil {
			return nil, err
		}
		for _, desc := range descendants {
			projectIDs = append(projectIDs, desc.ID.String())
		}
	}

	placeholders := strings.Repeat("?,", len(projectIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT DISTINCT document_id
		FROM project_documents
		WHERE workspace_id = ? AND project_id IN (%s)
	`, placeholders)

	args := make([]interface{}, len(projectIDs)+1)
	args[0] = workspaceID.String()
	for i, pid := range projectIDs {
		args[i+1] = pid
	}

	rows, err := r.conn.Query(ctx, query, args...)
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

// GetProjectsForDocument returns all projects that contain a document.
func (r *ProjectRepository) GetProjectsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]entity.ProjectID, error) {
	query := `
		SELECT project_id
		FROM project_documents
		WHERE workspace_id = ? AND document_id = ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projectIDs []entity.ProjectID
	for rows.Next() {
		var projectIDStr string
		if err := rows.Scan(&projectIDStr); err != nil {
			return nil, err
		}
		projectIDs = append(projectIDs, entity.ProjectID(projectIDStr))
	}
	return projectIDs, rows.Err()
}

// scanProject scans a project from a database row.
func (r *ProjectRepository) scanProject(scanner interface {
	Scan(dest ...interface{}) error
}, workspaceID entity.WorkspaceID) (*entity.Project, error) {
	var id, name, description, path string
	var natureStr sql.NullString
	var attributesJSON sql.NullString
	var parentID sql.NullString
	var createdAt, updatedAt int64

	err := scanner.Scan(&id, &name, &description, &natureStr, &attributesJSON, &parentID, &path, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	// Parse nature (default to generic if not set)
	nature := entity.NatureGeneric
	if natureStr.Valid && natureStr.String != "" {
		nature = entity.ProjectNature(natureStr.String)
		if !nature.IsValid() {
			nature = entity.NatureGeneric
		}
	}

	// Parse attributes JSON
	var attributes *entity.ProjectAttributes
	if attributesJSON.Valid && attributesJSON.String != "" && attributesJSON.String != "{}" {
		attributes = &entity.ProjectAttributes{}
		if err := attributes.FromJSON(attributesJSON.String); err != nil {
			// If parsing fails, use empty attributes
			attributes = &entity.ProjectAttributes{}
		}
	} else {
		attributes = &entity.ProjectAttributes{}
	}

	project := &entity.Project{
		ID:          entity.ProjectID(id),
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		Nature:      nature,
		Attributes:  attributes,
		Path:        path,
		CreatedAt:   time.UnixMilli(createdAt),
		UpdatedAt:   time.UnixMilli(updatedAt),
	}

	if parentID.Valid {
		pid := entity.ProjectID(parentID.String)
		project.ParentID = &pid
	}

	return project, nil
}

// nilIfEmpty returns nil if the pointer is nil, otherwise returns the value as string.
func nilIfEmpty(ptr *entity.ProjectID) interface{} {
	if ptr == nil {
		return nil
	}
	return ptr.String()
}

var _ repository.ProjectRepository = (*ProjectRepository)(nil)
