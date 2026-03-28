package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// EntityRepository implements repository.EntityRepository using SQLite.
// It unifies queries across files, folders, and projects.
type EntityRepository struct {
	conn        *Connection
	fileRepo    *FileRepository
	folderRepo  *FolderRepository
	projectRepo *ProjectRepository
	metaRepo    *MetadataRepository
}

// NewEntityRepository creates a new SQLite entity repository.
func NewEntityRepository(
	conn *Connection,
	fileRepo *FileRepository,
	folderRepo *FolderRepository,
	projectRepo *ProjectRepository,
	metaRepo *MetadataRepository,
) *EntityRepository {
	return &EntityRepository{
		conn:        conn,
		fileRepo:    fileRepo,
		folderRepo:  folderRepo,
		projectRepo: projectRepo,
		metaRepo:    metaRepo,
	}
}

// GetEntity retrieves an entity by ID.
func (r *EntityRepository) GetEntity(ctx context.Context, workspaceID entity.WorkspaceID, id entity.EntityID) (*entity.Entity, error) {
	switch id.Type {
	case entity.EntityTypeFile:
		file, err := r.fileRepo.GetByID(ctx, workspaceID, entity.FileID(id.ID))
		if err != nil {
			return nil, err
		}
		metadata, _ := r.metaRepo.GetByPath(ctx, workspaceID, file.RelativePath)
		return entity.FromFileEntry(workspaceID, file, metadata), nil

	case entity.EntityTypeFolder:
		folder, err := r.folderRepo.GetByID(ctx, workspaceID, entity.FolderID(id.ID))
		if err != nil {
			return nil, err
		}
		return entity.FromFolderEntry(workspaceID, folder), nil

	case entity.EntityTypeProject:
		project, err := r.projectRepo.Get(ctx, workspaceID, entity.ProjectID(id.ID))
		if err != nil {
			return nil, err
		}
		// Get document count for project
		docIDs, _ := r.projectRepo.GetDocuments(ctx, workspaceID, project.ID, false)
		return entity.FromProject(workspaceID, project, len(docIDs)), nil

	default:
		return nil, fmt.Errorf("unknown entity type: %s", id.Type)
	}
}

// ListEntities lists entities with filters.
func (r *EntityRepository) ListEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters repository.EntityFilters) ([]*entity.Entity, error) {
	var entities []*entity.Entity

	// Determine which types to query
	types := filters.Types
	if len(types) == 0 {
		types = []entity.EntityType{entity.EntityTypeFile, entity.EntityTypeFolder, entity.EntityTypeProject}
	}

	// Query each type
	for _, entityType := range types {
		typeEntities, err := r.listEntitiesByType(ctx, workspaceID, entityType, filters)
		if err != nil {
			continue // Continue with other types if one fails
		}
		entities = append(entities, typeEntities...)
	}

	// Apply limit and offset
	if filters.Limit > 0 {
		start := filters.Offset
		end := start + filters.Limit
		if end > len(entities) {
			end = len(entities)
		}
		if start < len(entities) {
			entities = entities[start:end]
		} else {
			entities = []*entity.Entity{}
		}
	}

	return entities, nil
}

// listEntitiesByType lists entities of a specific type with filters.
func (r *EntityRepository) listEntitiesByType(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entityType entity.EntityType,
	filters repository.EntityFilters,
) ([]*entity.Entity, error) {
	switch entityType {
	case entity.EntityTypeFile:
		return r.listFileEntities(ctx, workspaceID, filters)
	case entity.EntityTypeFolder:
		return r.listFolderEntities(ctx, workspaceID, filters)
	case entity.EntityTypeProject:
		return r.listProjectEntities(ctx, workspaceID, filters)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// listFileEntities lists file entities with filters.
func (r *EntityRepository) listFileEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters repository.EntityFilters) ([]*entity.Entity, error) {
	// Build query with filters
	query := `
		SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
		       f.last_modified, f.created_at, f.enhanced,
		       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
		FROM files f
		WHERE f.workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	// Add filters
	if len(filters.Tags) > 0 {
		query += ` AND EXISTS (
			SELECT 1 FROM file_tags ft
			WHERE ft.workspace_id = ? AND ft.file_id = f.id AND ft.tag IN (` + r.placeholders(len(filters.Tags)) + `)
		)`
		args = append(args, workspaceID.String())
		for _, tag := range filters.Tags {
			args = append(args, tag)
		}
	}

	if len(filters.Projects) > 0 {
		query += ` AND EXISTS (
			SELECT 1 FROM file_contexts fc
			WHERE fc.workspace_id = ? AND fc.file_id = f.id AND fc.context IN (` + r.placeholders(len(filters.Projects)) + `)
		)`
		args = append(args, workspaceID.String())
		for _, project := range filters.Projects {
			args = append(args, project)
		}
	}

	if filters.Language != nil {
		query += ` AND EXISTS (
			SELECT 1 FROM file_metadata fm
			WHERE fm.workspace_id = ? AND fm.file_id = f.id AND fm.detected_language = ?
		)`
		args = append(args, workspaceID.String(), *filters.Language)
	}

	// Execute query
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		var id, relativePath, absolutePath, filename, extension string
		var fileSize int64
		var lastModified, createdAt int64
		var enhancedJSON []byte
		var indexedBasic, indexedMime, indexedCode, indexedDocument, indexedMirror bool

		if err := rows.Scan(&id, &relativePath, &absolutePath, &filename, &extension, &fileSize,
			&lastModified, &createdAt, &enhancedJSON,
			&indexedBasic, &indexedMime, &indexedCode, &indexedDocument, &indexedMirror); err != nil {
			continue
		}

		// Reconstruct FileEntry
		file := &entity.FileEntry{
			ID:           entity.FileID(id),
			RelativePath: relativePath,
			AbsolutePath: absolutePath,
			Filename:     filename,
			Extension:    extension,
			FileSize:     fileSize,
			LastModified: time.UnixMilli(lastModified),
			CreatedAt:    time.UnixMilli(createdAt),
		}

		// Parse enhanced metadata
		if len(enhancedJSON) > 0 {
			var enhanced entity.EnhancedMetadata
			if err := json.Unmarshal(enhancedJSON, &enhanced); err == nil {
				file.Enhanced = &enhanced
			}
		}

		// Get metadata
		metadata, _ := r.metaRepo.GetByPath(ctx, workspaceID, relativePath)

		// Convert to entity
		ent := entity.FromFileEntry(workspaceID, file, metadata)
		entities = append(entities, ent)
	}

	return entities, nil
}

// listFolderEntities lists folder entities with filters.
func (r *EntityRepository) listFolderEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters repository.EntityFilters) ([]*entity.Entity, error) {
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	// Add filters based on metadata JSON
	if len(filters.Tags) > 0 || filters.Language != nil || filters.Category != nil {
		// For folders, we need to check metadata JSON
		// This is a simplified version - full implementation would parse JSON
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		var id, relativePath, name, parentPath string
		var depth int
		var metricsJSON, metadataJSON []byte
		var createdAt, updatedAt int64

		if err := rows.Scan(&id, &relativePath, &name, &parentPath, &depth, &metricsJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
			continue
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
			var metrics entity.FolderMetrics
			if err := json.Unmarshal(metricsJSON, &metrics); err == nil {
				folder.Metrics = &metrics
			}
		}

		if len(metadataJSON) > 0 {
			var metadata entity.FolderMetadata
			if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
				folder.Metadata = &metadata
			}
		}

		ent := entity.FromFolderEntry(workspaceID, folder)
		entities = append(entities, ent)
	}

	return entities, nil
}

// listProjectEntities lists project entities with filters.
func (r *EntityRepository) listProjectEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters repository.EntityFilters) ([]*entity.Entity, error) {
	query := `
		SELECT id, workspace_id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID.String()}

	// Add filters
	if filters.Category != nil {
		query += ` AND nature = ?`
		args = append(args, *filters.Category)
	}

	if filters.Status != nil {
		query += ` AND JSON_EXTRACT(attributes, '$.status') = ?`
		args = append(args, *filters.Status)
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		var id, name, description, nature, attributesJSON, path string
		var createdAt, updatedAt int64
		var parentIDPtr *string
		if err := rows.Scan(&id, &name, &description, &nature, &attributesJSON, &parentIDPtr, &path, &createdAt, &updatedAt); err != nil {
			continue
		}

		project := &entity.Project{
			ID:          entity.ProjectID(id),
			WorkspaceID: workspaceID,
			Name:        name,
			Description: description,
			Nature:      entity.ProjectNature(nature),
			Path:        path,
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		}

		if parentIDPtr != nil && *parentIDPtr != "" {
			parentID := entity.ProjectID(*parentIDPtr)
			project.ParentID = &parentID
		}

		if len(attributesJSON) > 0 {
			var attrs entity.ProjectAttributes
			if err := attrs.FromJSON(attributesJSON); err == nil {
				project.Attributes = &attrs
			}
		}

		docIDs, _ := r.projectRepo.GetDocuments(ctx, workspaceID, project.ID, false)
		ent := entity.FromProject(workspaceID, project, len(docIDs))
		entities = append(entities, ent)
	}

	return entities, nil
}

// GetEntitiesByFacet retrieves entities matching a facet value.
func (r *EntityRepository) GetEntitiesByFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	facet string,
	value string,
	entityTypes []entity.EntityType,
) ([]*entity.Entity, error) {
	if len(entityTypes) == 0 {
		entityTypes = []entity.EntityType{entity.EntityTypeFile, entity.EntityTypeFolder, entity.EntityTypeProject}
	}

	var allEntities []*entity.Entity

	for _, entityType := range entityTypes {
		entities, err := r.getEntitiesByFacetAndType(ctx, workspaceID, facet, value, entityType)
		if err != nil {
			continue
		}
		allEntities = append(allEntities, entities...)
	}

	return allEntities, nil
}

// getEntitiesByFacetAndType gets entities of a specific type matching a facet value.
func (r *EntityRepository) getEntitiesByFacetAndType(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	facet string,
	value string,
	entityType entity.EntityType,
) ([]*entity.Entity, error) {
	switch entityType {
	case entity.EntityTypeFile:
		return r.getFileEntitiesByFacet(ctx, workspaceID, facet, value)
	case entity.EntityTypeFolder:
		return r.getFolderEntitiesByFacet(ctx, workspaceID, facet, value)
	case entity.EntityTypeProject:
		return r.getProjectEntitiesByFacet(ctx, workspaceID, facet, value)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// getFileEntitiesByFacet gets file entities matching a facet value.
func (r *EntityRepository) getFileEntitiesByFacet(ctx context.Context, workspaceID entity.WorkspaceID, facet string, value string) ([]*entity.Entity, error) {
	var query string
	var args []interface{}

	switch facet {
	case "tag":
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_tags ft ON f.workspace_id = ft.workspace_id AND f.id = ft.file_id
			WHERE f.workspace_id = ? AND ft.tag = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "project":
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_contexts fc ON f.workspace_id = fc.workspace_id AND f.id = fc.file_id
			WHERE f.workspace_id = ? AND fc.context = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "language":
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_metadata fm ON f.workspace_id = fm.workspace_id AND f.id = fm.file_id
			WHERE f.workspace_id = ? AND fm.detected_language = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "extension":
		// Normalize extension value: ensure it has a leading dot and is lowercase
		// The database stores extensions with leading dot (e.g., ".csv")
		normalizedExt := strings.ToLower(strings.TrimSpace(value))
		if normalizedExt != "" && !strings.HasPrefix(normalizedExt, ".") {
			normalizedExt = "." + normalizedExt
		}
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ? AND f.extension = ?
		`
		args = []interface{}{workspaceID.String(), normalizedExt}

	case "type":
		// Type facet: match by MIME category or inferred type from extension
		// The value is a semantic type like "image", "document", "code", etc.
		// Optimized approach: Get files with MIME category match, then get files without
		// MIME category and filter by extension-based inference in Go
		normalizedType := strings.ToLower(strings.TrimSpace(value))
		
		// Step 1: Get files with MIME category matching the type (most efficient - uses index)
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ?
				AND LOWER(json_extract(f.enhanced, '$.MimeType.Category')) = ?
		`
		args = []interface{}{workspaceID.String(), normalizedType}
		
		rows, err := r.conn.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		
		var matchingEntities []*entity.Entity
		for rows.Next() {
			// Check for cancellation during processing
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			
			var id, relativePath, absolutePath, filename, extension string
			var fileSize int64
			var lastModified, createdAt int64
			var enhancedJSON []byte
			var indexedBasic, indexedMime, indexedCode, indexedDocument, indexedMirror bool
			
			if err := rows.Scan(&id, &relativePath, &absolutePath, &filename, &extension, &fileSize,
				&lastModified, &createdAt, &enhancedJSON,
				&indexedBasic, &indexedMime, &indexedCode, &indexedDocument, &indexedMirror); err != nil {
				continue
			}
			
			file := &entity.FileEntry{
				ID:           entity.FileID(id),
				RelativePath: relativePath,
				AbsolutePath: absolutePath,
				Filename:     filename,
				Extension:    extension,
				FileSize:     fileSize,
				LastModified: time.UnixMilli(lastModified),
				CreatedAt:    time.UnixMilli(createdAt),
			}
			
			if len(enhancedJSON) > 0 {
				var enhanced entity.EnhancedMetadata
				if err := json.Unmarshal(enhancedJSON, &enhanced); err == nil {
					file.Enhanced = &enhanced
				}
			}
			
			ent := entity.FromFileEntry(workspaceID, file, nil)
			matchingEntities = append(matchingEntities, ent)
		}
		
		// Step 2: Get files without MIME category and filter by extension-based inference
		query2 := `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ?
				AND (json_extract(f.enhanced, '$.MimeType.Category') IS NULL
					OR json_extract(f.enhanced, '$.MimeType.Category') = '')
		`
		rows2, err := r.conn.Query(ctx, query2, workspaceID.String())
		if err != nil {
			// If this query fails, return what we have from step 1
			return matchingEntities, nil
		}
		defer rows2.Close()
		
		for rows2.Next() {
			// Check for cancellation during processing
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			
			var id, relativePath, absolutePath, filename, extension string
			var fileSize int64
			var lastModified, createdAt int64
			var enhancedJSON []byte
			var indexedBasic, indexedMime, indexedCode, indexedDocument, indexedMirror bool
			
			if err := rows2.Scan(&id, &relativePath, &absolutePath, &filename, &extension, &fileSize,
				&lastModified, &createdAt, &enhancedJSON,
				&indexedBasic, &indexedMime, &indexedCode, &indexedDocument, &indexedMirror); err != nil {
				continue
			}
			
			// Infer type from extension using the same logic as GetTypeFacet
			fileType := inferFileTypeFromExtension(extension)
			
			// Check for special extensions that should always use extension-based inference
			extLower := strings.ToLower(strings.TrimPrefix(extension, "."))
			alwaysInferFromExtension := map[string]bool{
				"csv": true, "tsv": true,
				"db": true, "sqlite": true, "sqlite3": true, "thumbs": true,
				"ds_store": true,
			}
			
			// Only add if type matches (for special extensions or if inferred type matches)
			if alwaysInferFromExtension[extLower] || strings.ToLower(fileType) == normalizedType {
				file := &entity.FileEntry{
					ID:           entity.FileID(id),
					RelativePath: relativePath,
					AbsolutePath: absolutePath,
					Filename:     filename,
					Extension:    extension,
					FileSize:     fileSize,
					LastModified: time.UnixMilli(lastModified),
					CreatedAt:    time.UnixMilli(createdAt),
				}
				
				if len(enhancedJSON) > 0 {
					var enhanced entity.EnhancedMetadata
					if err := json.Unmarshal(enhancedJSON, &enhanced); err == nil {
						file.Enhanced = &enhanced
					}
				}
				
				ent := entity.FromFileEntry(workspaceID, file, nil)
				matchingEntities = append(matchingEntities, ent)
			}
		}
		
		return matchingEntities, nil

	case "document_type":
		// Document type facet: match by suggested_taxonomy.content_type (AI-suggested semantic classification)
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy st ON f.workspace_id = st.workspace_id AND f.id = st.file_id
			WHERE f.workspace_id = ? AND st.content_type = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "mime_type":
		// MIME type facet: match by MIME type from enhanced metadata (e.g., image/jpeg, application/pdf)
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ? AND json_extract(f.enhanced, '$.MimeType.MimeType') = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "mime_category":
		// MIME category facet: match by MIME category from enhanced metadata (e.g., image, document, text)
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ? AND json_extract(f.enhanced, '$.MimeType.Category') = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "indexing_status":
		// Indexing status facet: match by indexing status flags
		// Status values: complete, document_complete, code_complete, mime_complete, basic_only, not_indexed
		normalizedStatus := strings.ToLower(strings.TrimSpace(value))
		
		var statusCondition string
		switch normalizedStatus {
		case "complete":
			statusCondition = "indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 AND indexed_document = 1 AND indexed_mirror = 1"
		case "document_complete":
			statusCondition = "indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 AND indexed_document = 1 AND indexed_mirror = 0"
		case "code_complete":
			statusCondition = "indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 1 AND indexed_document = 0"
		case "mime_complete":
			statusCondition = "indexed_basic = 1 AND indexed_mime = 1 AND indexed_code = 0"
		case "basic_only":
			statusCondition = "indexed_basic = 1 AND indexed_mime = 0"
		case "not_indexed":
			statusCondition = "indexed_basic = 0"
		default:
			// Unknown status - return empty
			return []*entity.Entity{}, nil
		}
		
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ? AND ` + statusCondition
		args = []interface{}{workspaceID.String()}

	case "indexing_error", "index_error", "indexing_errors":
		// Indexing error facet: match by indexing error stage from enhanced metadata
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ?
				AND json_extract(f.enhanced, '$.IndexingErrors.Stage') = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "category", "ai_category":
		// AI category facet: match by ai_category from file_metadata
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_metadata fm ON f.workspace_id = fm.workspace_id AND f.id = fm.file_id
			WHERE f.workspace_id = ? AND COALESCE(fm.ai_category, 'uncategorized') = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "author", "authors":
		// Author facet: match by author name from file_authors
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_authors fa ON f.workspace_id = fa.workspace_id AND f.id = fa.file_id
			WHERE f.workspace_id = ? AND fa.name = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "owner":
		// Owner facet: match by owner username from file_ownership joined with system_users
		// The value may have "owner:" prefix (from GetOwnerFacet) or be just the username
		ownerValue := strings.TrimPrefix(value, "owner:")
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_ownership fo ON f.workspace_id = fo.workspace_id AND f.id = fo.file_id
			LEFT JOIN system_users su ON su.id = fo.user_id AND su.workspace_id = fo.workspace_id
			WHERE f.workspace_id = ? 
				AND fo.ownership_type = 'owner'
				AND COALESCE(su.username, 'unknown') = ?
		`
		args = []interface{}{workspaceID.String(), ownerValue}

	case "sentiment":
		// Sentiment facet: match by overall_sentiment from file_sentiment
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_sentiment fs ON f.workspace_id = fs.workspace_id AND f.id = fs.file_id
			WHERE f.workspace_id = ? AND COALESCE(fs.overall_sentiment, 'unknown') = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "duplicate_type":
		// Duplicate type facet: match by duplicate type from file_duplicates
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_duplicates fd ON f.workspace_id = fd.workspace_id AND f.id = fd.file_id
			WHERE f.workspace_id = ? AND fd.type = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "publication_year", "year":
		// Publication year facet: match by publication_year from file_publication_info
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN file_publication_info fpi ON f.workspace_id = fpi.workspace_id AND f.id = fpi.file_id
			WHERE f.workspace_id = ? AND fpi.publication_year = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "purpose":
		// Purpose facet: match by purpose from suggested_taxonomy
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy st ON f.workspace_id = st.workspace_id AND f.id = st.file_id
			WHERE f.workspace_id = ? AND st.purpose = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "audience":
		// Audience facet: match by audience from suggested_taxonomy
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy st ON f.workspace_id = st.workspace_id AND f.id = st.file_id
			WHERE f.workspace_id = ? AND st.audience = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "domain":
		// Domain facet: match by domain from suggested_taxonomy
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy st ON f.workspace_id = st.workspace_id AND f.id = st.file_id
			WHERE f.workspace_id = ? AND st.domain = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "subdomain":
		// Subdomain facet: match by subdomain from suggested_taxonomy
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy st ON f.workspace_id = st.workspace_id AND f.id = st.file_id
			WHERE f.workspace_id = ? AND st.subdomain = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "topic", "topics":
		// Topic facet: match by topic from suggested_taxonomy_topics
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN suggested_taxonomy_topics stt ON f.workspace_id = stt.workspace_id AND f.id = stt.file_id
			WHERE f.workspace_id = ? AND stt.topic = ?
		`
		args = []interface{}{workspaceID.String(), value}

	case "folder":
		// Folder facet: match by folder name (extract first-level folder from relative_path)
		// Extract first-level folder name: if path contains '/', get the part before the first '/'
		// For example: "Libros/file.txt" -> folder name is "Libros"
		// For "Libros/subfolder/file.txt" -> folder name is also "Libros" (first level only)
		// For root files (no '/'), match if value is "." or "root" or empty
		normalizedValue := strings.TrimSpace(value)
		if normalizedValue == "" || normalizedValue == "root" {
			normalizedValue = "."
		}
		
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			WHERE f.workspace_id = ?
			AND (
				-- If path contains '/', extract first-level folder name and match
				(CASE 
					WHEN INSTR(f.relative_path, '/') > 0 THEN
						-- Extract first-level folder: get the part before the first '/'
						SUBSTR(f.relative_path, 1, INSTR(f.relative_path, '/') - 1)
					ELSE '.'
				END) = ?
			)
		`
		args = []interface{}{workspaceID.String(), normalizedValue}

	case "cluster":
		// Cluster facet: match files by cluster membership
		// The value can be either a cluster name or cluster ID
		// First, find the cluster by name or ID, then get its members
		query = `
			SELECT f.id, f.relative_path, f.absolute_path, f.filename, f.extension, f.file_size,
			       f.last_modified, f.created_at, f.enhanced,
			       f.indexed_basic, f.indexed_mime, f.indexed_code, f.indexed_document, f.indexed_mirror
			FROM files f
			INNER JOIN documents d ON f.workspace_id = d.workspace_id AND f.relative_path = d.relative_path
			INNER JOIN cluster_memberships cm ON d.workspace_id = cm.workspace_id AND d.id = cm.document_id
			INNER JOIN document_clusters dc ON cm.workspace_id = dc.workspace_id AND cm.cluster_id = dc.id
			WHERE f.workspace_id = ?
				AND dc.status = 'active'
				AND (dc.name = ? OR dc.id = ?)
		`
		args = []interface{}{workspaceID.String(), value, value}

	default:
		// For other facets, use generic approach
		// Note: This will get all files and filter in memory, which may be slow for large workspaces
		// Consider adding specific cases for frequently used facets
		return r.listFileEntities(ctx, workspaceID, repository.EntityFilters{})
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		// Check for cancellation during row processing (critical for large result sets)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		var id, relativePath, absolutePath, filename, extension string
		var fileSize int64
		var lastModified, createdAt int64
		var enhancedJSON []byte
		var indexedBasic, indexedMime, indexedCode, indexedDocument, indexedMirror bool

		if err := rows.Scan(&id, &relativePath, &absolutePath, &filename, &extension, &fileSize,
			&lastModified, &createdAt, &enhancedJSON,
			&indexedBasic, &indexedMime, &indexedCode, &indexedDocument, &indexedMirror); err != nil {
			continue
		}

		file := &entity.FileEntry{
			ID:           entity.FileID(id),
			RelativePath: relativePath,
			AbsolutePath: absolutePath,
			Filename:     filename,
			Extension:    extension,
			FileSize:     fileSize,
			LastModified: time.UnixMilli(lastModified),
			CreatedAt:    time.UnixMilli(createdAt),
		}

		if len(enhancedJSON) > 0 {
			var enhanced entity.EnhancedMetadata
			if err := json.Unmarshal(enhancedJSON, &enhanced); err == nil {
				file.Enhanced = &enhanced
			}
		}

		// Skip metadata fetch for performance - not critical for facet queries
		// This avoids N+1 query problem and significantly improves response time
		ent := entity.FromFileEntry(workspaceID, file, nil)
		entities = append(entities, ent)
	}

	return entities, nil
}

// getFolderEntitiesByFacet gets folder entities matching a facet value.
func (r *EntityRepository) getFolderEntitiesByFacet(ctx context.Context, workspaceID entity.WorkspaceID, facet string, value string) ([]*entity.Entity, error) {
	// For folders, we need to query metadata JSON
	// This is a simplified version
	query := `
		SELECT id, relative_path, name, parent_path, depth, metrics, metadata, created_at, updated_at
		FROM folders
		WHERE workspace_id = ?
	`

	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		var id, relativePath, name, parentPath string
		var depth int
		var metricsJSON, metadataJSON []byte
		var createdAt, updatedAt int64

		if err := rows.Scan(&id, &relativePath, &name, &parentPath, &depth, &metricsJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
			continue
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
			var metrics entity.FolderMetrics
			if err := json.Unmarshal(metricsJSON, &metrics); err == nil {
				folder.Metrics = &metrics
			}
		}

		if len(metadataJSON) > 0 {
			var metadata entity.FolderMetadata
			if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
				folder.Metadata = &metadata
			}
		}

		ent := entity.FromFolderEntry(workspaceID, folder)

		// Filter by facet value
		if r.entityMatchesFacet(ent, facet, value) {
			entities = append(entities, ent)
		}
	}

	return entities, nil
}

// getProjectEntitiesByFacet gets project entities matching a facet value.
func (r *EntityRepository) getProjectEntitiesByFacet(ctx context.Context, workspaceID entity.WorkspaceID, facet string, value string) ([]*entity.Entity, error) {
	query := `
		SELECT id, workspace_id, name, description, nature, attributes, parent_id, path, created_at, updated_at
		FROM projects
		WHERE workspace_id = ?
	`

	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*entity.Entity
	for rows.Next() {
		var id, name, description, nature, attributesJSON, path string
		var createdAt, updatedAt int64
		var parentIDPtr *string
		if err := rows.Scan(&id, &name, &description, &nature, &attributesJSON, &parentIDPtr, &path, &createdAt, &updatedAt); err != nil {
			continue
		}

		project := &entity.Project{
			ID:          entity.ProjectID(id),
			WorkspaceID: workspaceID,
			Name:        name,
			Description: description,
			Nature:      entity.ProjectNature(nature),
			Path:        path,
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		}

		if parentIDPtr != nil && *parentIDPtr != "" {
			parentID := entity.ProjectID(*parentIDPtr)
			project.ParentID = &parentID
		}

		if len(attributesJSON) > 0 {
			var attrs entity.ProjectAttributes
			if err := attrs.FromJSON(attributesJSON); err == nil {
				project.Attributes = &attrs
			}
		}

		docIDs, _ := r.projectRepo.GetDocuments(ctx, workspaceID, project.ID, false)
		ent := entity.FromProject(workspaceID, project, len(docIDs))

		// Filter by facet value
		if r.entityMatchesFacet(ent, facet, value) {
			entities = append(entities, ent)
		}
	}

	return entities, nil
}

// entityMatchesFacet checks if an entity matches a facet value.
func (r *EntityRepository) entityMatchesFacet(ent *entity.Entity, facet string, value string) bool {
	switch facet {
	case "tag":
		for _, tag := range ent.Tags {
			if tag == value {
				return true
			}
		}
	case "project":
		for _, project := range ent.Projects {
			if project == value {
				return true
			}
		}
	case "language":
		if ent.Language != nil && *ent.Language == value {
			return true
		}
	case "category":
		if ent.Category != nil && *ent.Category == value {
			return true
		}
	case "author":
		if ent.Author != nil && *ent.Author == value {
			return true
		}
	case "owner":
		if ent.Owner != nil && *ent.Owner == value {
			return true
		}
	case "status":
		if ent.Status != nil && *ent.Status == value {
			return true
		}
	case "priority":
		if ent.Priority != nil && *ent.Priority == value {
			return true
		}
	case "visibility":
		if ent.Visibility != nil && *ent.Visibility == value {
			return true
		}
	}
	return false
}

// UpdateEntityMetadata updates semantic metadata for an entity.
func (r *EntityRepository) UpdateEntityMetadata(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	id entity.EntityID,
	metadata repository.EntityMetadata,
) error {
	switch id.Type {
	case entity.EntityTypeFile:
		return r.updateFileMetadata(ctx, workspaceID, entity.FileID(id.ID), metadata)
	case entity.EntityTypeFolder:
		return r.updateFolderMetadata(ctx, workspaceID, entity.FolderID(id.ID), metadata)
	case entity.EntityTypeProject:
		return r.updateProjectMetadata(ctx, workspaceID, entity.ProjectID(id.ID), metadata)
	default:
		return fmt.Errorf("unknown entity type: %s", id.Type)
	}
}

// updateFileMetadata updates file metadata.
func (r *EntityRepository) updateFileMetadata(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, metadata repository.EntityMetadata) error {
	// Update file_metadata table
	if len(metadata.Tags) > 0 {
		// Delete existing tags
		_, _ = r.conn.Exec(ctx, `DELETE FROM file_tags WHERE workspace_id = ? AND file_id = ?`, workspaceID.String(), fileID.String())
		// Insert new tags
		for _, tag := range metadata.Tags {
			_, _ = r.conn.Exec(ctx, `INSERT INTO file_tags (workspace_id, file_id, tag) VALUES (?, ?, ?)`, workspaceID.String(), fileID.String(), tag)
		}
	}

	if len(metadata.Projects) > 0 {
		// Delete existing contexts
		_, _ = r.conn.Exec(ctx, `DELETE FROM file_contexts WHERE workspace_id = ? AND file_id = ?`, workspaceID.String(), fileID.String())
		// Insert new contexts
		for _, project := range metadata.Projects {
			_, _ = r.conn.Exec(ctx, `INSERT INTO file_contexts (workspace_id, file_id, context) VALUES (?, ?, ?)`, workspaceID.String(), fileID.String(), project)
		}
	}

	// Update file_metadata table
	updateFields := []string{}
	args := []interface{}{workspaceID.String(), fileID.String()}

	if metadata.Language != nil {
		updateFields = append(updateFields, "detected_language = ?")
		args = append(args, *metadata.Language)
	}

	if len(updateFields) > 0 {
		query := `UPDATE file_metadata SET ` + strings.Join(updateFields, ", ") + ` WHERE workspace_id = ? AND file_id = ?`
		_, err := r.conn.Exec(ctx, query, args...)
		return err
	}

	return nil
}

// updateFolderMetadata updates folder metadata.
func (r *EntityRepository) updateFolderMetadata(ctx context.Context, workspaceID entity.WorkspaceID, folderID entity.FolderID, metadata repository.EntityMetadata) error {
	// Get existing folder
	folder, err := r.folderRepo.GetByID(ctx, workspaceID, folderID)
	if err != nil {
		return err
	}

	// Update metadata
	if folder.Metadata == nil {
		folder.Metadata = &entity.FolderMetadata{}
	}

	if len(metadata.Tags) > 0 {
		folder.Metadata.UserTags = metadata.Tags
	}
	if metadata.Language != nil {
		folder.Metadata.DominantLanguage = metadata.Language
	}
	if metadata.Category != nil {
		folder.Metadata.ProjectNature = metadata.Category
	}
	if len(metadata.Projects) > 0 && len(metadata.Projects) > 0 {
		projectName := metadata.Projects[0]
		folder.Metadata.UserProject = &projectName
	}
	if metadata.Status != nil {
		folder.Metadata.Status = metadata.Status
	}
	if metadata.Priority != nil {
		folder.Metadata.Priority = metadata.Priority
	}
	if metadata.Visibility != nil {
		folder.Metadata.Visibility = metadata.Visibility
	}

	// Save back
	return r.folderRepo.Upsert(ctx, workspaceID, folder)
}

// updateProjectMetadata updates project metadata.
func (r *EntityRepository) updateProjectMetadata(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, metadata repository.EntityMetadata) error {
	// Get existing project
	project, err := r.projectRepo.Get(ctx, workspaceID, projectID)
	if err != nil {
		return err
	}

	// Update attributes
	if project.Attributes == nil {
		project.Attributes = &entity.ProjectAttributes{}
	}

	if len(metadata.Tags) > 0 {
		project.Attributes.Tags = metadata.Tags
	}
	if metadata.Language != nil {
		project.Attributes.Language = metadata.Language
	}
	if metadata.Author != nil {
		project.Attributes.Author = metadata.Author
	}
	if metadata.Owner != nil {
		project.Attributes.Owner = metadata.Owner
	}
	if metadata.Status != nil {
		project.Attributes.Status = *metadata.Status
	}
	if metadata.Priority != nil {
		project.Attributes.Priority = *metadata.Priority
	}
	if metadata.Visibility != nil {
		project.Attributes.Visibility = *metadata.Visibility
	}

	// Save back
	return r.projectRepo.Update(ctx, workspaceID, project)
}

// CountEntitiesByFacet counts entities matching a facet value.
func (r *EntityRepository) CountEntitiesByFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	facet string,
	value string,
	entityTypes []entity.EntityType,
) (int, error) {
	entities, err := r.GetEntitiesByFacet(ctx, workspaceID, facet, value, entityTypes)
	if err != nil {
		return 0, err
	}
	return len(entities), nil
}

// placeholders generates SQL placeholders for IN clause.
func (r *EntityRepository) placeholders(count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}

