// Package entity contains domain entities for the Cortex system.
package entity

import (
	"path/filepath"
	"time"
)

// EntityType represents the type of entity
type EntityType string

const (
	EntityTypeFile    EntityType = "file"
	EntityTypeFolder  EntityType = "folder"
	EntityTypeProject EntityType = "project"
)

// EntityID is a unified identifier for any entity
type EntityID struct {
	Type EntityType
	ID   string // FileID, FolderID, or ProjectID
}

// String returns the string representation of EntityID
func (id EntityID) String() string {
	return string(id.Type) + ":" + id.ID
}

// NewEntityID creates a new EntityID
func NewEntityID(entityType EntityType, id string) EntityID {
	return EntityID{
		Type: entityType,
		ID:   id,
	}
}

// Entity represents a unified view of files, folders, and projects
// This allows facets to filter any type of entity uniformly
type Entity struct {
	// Identity
	ID          EntityID
	Type        EntityType
	WorkspaceID WorkspaceID

	// Basic information
	Name        string
	Path        string // Relative path for files/folders, hierarchical path for projects
	Description *string

	// Timestamps
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ModifiedAt *time.Time // For files

	// Size (for files and folders)
	Size *int64

	// Semantic metadata (unified - can be filtered by facets)
	Tags            []string
	Projects        []string // Assigned project names/IDs
	Language        *string
	Category        *string // AI category, folder nature, or project nature
	Author          *string
	Owner           *string
	Location        *string
	PublicationYear *int

	// Quality metrics
	Complexity    *float64
	LinesOfCode   *int
	QualityScore  *float64

	// Status
	Status     *string // indexing status, project status, etc.
	Priority   *string
	Visibility *string

	// AI metadata
	AISummary  *string
	AIKeywords []string

	// Type-specific data (preserved for compatibility and type-specific operations)
	FileData    *FileEntityData
	FolderData  *FolderEntityData
	ProjectData *ProjectEntityData
}

// FileEntityData contains file-specific data
type FileEntityData struct {
	Extension        string
	MimeType         *string
	ContentType      *string
	CodeMetrics      *CodeMetrics
	DocumentMetrics  *DocumentMetrics
	ImageMetadata    *ImageMetadata
	AudioMetadata    *AudioMetadata
	VideoMetadata    *VideoMetadata
	IndexedState     IndexedState
	RelativePath     string
	AbsolutePath     string
}

// FolderEntityData contains folder-specific data
type FolderEntityData struct {
	Depth            int
	TotalFiles       int
	DirectFiles      int
	Subfolders       int
	FolderMetrics    *FolderMetrics
	DominantFileType *string
	RelativePath     string
	AbsolutePath     string
}

// ProjectEntityData contains project-specific data
type ProjectEntityData struct {
	Nature        ProjectNature
	Attributes    *ProjectAttributes
	ParentID      *ProjectID
	DocumentCount int
}

// ToFileEntry converts Entity back to FileEntry (if it's a file)
func (e *Entity) ToFileEntry() *FileEntry {
	if e.Type != EntityTypeFile || e.FileData == nil {
		return nil
	}

	modifiedAt := e.CreatedAt
	if e.ModifiedAt != nil {
		modifiedAt = *e.ModifiedAt
	}

	size := int64(0)
	if e.Size != nil {
		size = *e.Size
	}

	return &FileEntry{
		ID:           FileID(e.ID.ID),
		RelativePath: e.FileData.RelativePath,
		AbsolutePath: e.FileData.AbsolutePath,
		Filename:     e.Name,
		Extension:    e.FileData.Extension,
		FileSize:     size,
		LastModified: modifiedAt,
		CreatedAt:    e.CreatedAt,
		Enhanced: &EnhancedMetadata{
			Language:        e.Language,
			CodeMetrics:     e.FileData.CodeMetrics,
			DocumentMetrics: e.FileData.DocumentMetrics,
			ImageMetadata:   e.FileData.ImageMetadata,
			AudioMetadata:   e.FileData.AudioMetadata,
			VideoMetadata:   e.FileData.VideoMetadata,
			IndexedState:    e.FileData.IndexedState,
			MimeType: &MimeTypeInfo{
				MimeType: getStringValue(e.FileData.MimeType),
				Category: getStringValue(e.FileData.ContentType),
			},
		},
	}
}

// ToFolderEntry converts Entity back to FolderEntry (if it's a folder)
func (e *Entity) ToFolderEntry() *FolderEntry {
	if e.Type != EntityTypeFolder || e.FolderData == nil {
		return nil
	}

	return &FolderEntry{
		ID:           FolderID(e.ID.ID),
		RelativePath: e.FolderData.RelativePath,
		AbsolutePath: e.FolderData.AbsolutePath,
		Name:         e.Name,
		ParentPath:   getParentPath(e.Path),
		Depth:        e.FolderData.Depth,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		Metrics:      e.FolderData.FolderMetrics,
		Metadata: &FolderMetadata{
			InferredTags:       e.Tags,
			UserProject:        getFirstString(e.Projects),
			DominantLanguage:   e.Language,
			ProjectNature:      e.Category,
			ContentDescription: e.Description,
			AISummary:          e.AISummary,
			AIKeywords:         e.AIKeywords,
		},
	}
}

// ToProject converts Entity back to Project (if it's a project)
func (e *Entity) ToProject() *Project {
	if e.Type != EntityTypeProject || e.ProjectData == nil {
		return nil
	}

	parentID := e.ProjectData.ParentID
	if parentID == nil && len(e.Projects) > 0 {
		// Try to infer parent from projects list
		// This is a simplification - actual implementation may need more logic
	}

	return &Project{
		ID:          ProjectID(e.ID.ID),
		WorkspaceID: e.WorkspaceID,
		Name:        e.Name,
		Description: getStringValue(e.Description),
		Nature:      e.ProjectData.Nature,
		Attributes:  e.ProjectData.Attributes,
		ParentID:    parentID,
		Path:        e.Path,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// FromFileEntry converts FileEntry to Entity
func FromFileEntry(workspaceID WorkspaceID, file *FileEntry, metadata *FileMetadata) *Entity {
	if file == nil {
		return nil
	}

	entity := &Entity{
		ID:          NewEntityID(EntityTypeFile, string(file.ID)),
		Type:        EntityTypeFile,
		WorkspaceID: workspaceID,
		Name:        file.Filename,
		Path:        file.RelativePath,
		CreatedAt:   file.CreatedAt,
		ModifiedAt:  &file.LastModified,
		UpdatedAt:   file.CreatedAt, // Will be updated from metadata if available
		Size:        &file.FileSize,
	}

	// Extract semantic metadata from FileMetadata
	if metadata != nil {
		entity.Tags = metadata.Tags
		entity.Projects = metadata.Contexts
		entity.Language = metadata.DetectedLanguage
		if metadata.AICategory != nil {
			entity.Category = &metadata.AICategory.Category
		}
		if metadata.AISummary != nil {
			entity.AISummary = &metadata.AISummary.Summary
			entity.AIKeywords = metadata.AISummary.KeyTerms
		}
		entity.UpdatedAt = metadata.UpdatedAt
	}

	// Extract from EnhancedMetadata
	if file.Enhanced != nil {
		if entity.Language == nil {
			entity.Language = file.Enhanced.Language
		}

		// Extract author from DocumentMetrics
		if file.Enhanced.DocumentMetrics != nil {
			if file.Enhanced.DocumentMetrics.Author != nil {
				entity.Author = file.Enhanced.DocumentMetrics.Author
			}
			if file.Enhanced.DocumentMetrics.CreatedDate != nil {
				year := file.Enhanced.DocumentMetrics.CreatedDate.Year()
				entity.PublicationYear = &year
			}
		}

		// Extract location from ImageMetadata
		if file.Enhanced.ImageMetadata != nil && file.Enhanced.ImageMetadata.GPSLocation != nil {
			entity.Location = file.Enhanced.ImageMetadata.GPSLocation
		}

		// Extract owner from OSMetadata
		if file.Enhanced.OSMetadata != nil && file.Enhanced.OSMetadata.Owner != nil {
			entity.Owner = &file.Enhanced.OSMetadata.Owner.Username
		}

		// Extract quality metrics
		if file.Enhanced.CodeMetrics != nil {
			entity.Complexity = &file.Enhanced.CodeMetrics.Complexity
			entity.LinesOfCode = &file.Enhanced.CodeMetrics.LinesOfCode
		}
		if file.Enhanced.ContentQuality != nil {
			entity.QualityScore = &file.Enhanced.ContentQuality.QualityScore
		}

		// Extract status from IndexedState
		if !file.Enhanced.IndexedState.IsFullyIndexed() {
			status := "indexing"
			entity.Status = &status
		}
	}

	// Set file-specific data
	entity.FileData = &FileEntityData{
		Extension:        file.Extension,
		RelativePath:     file.RelativePath,
		AbsolutePath:     file.AbsolutePath,
		CodeMetrics:      file.Enhanced.CodeMetrics,
		DocumentMetrics:  file.Enhanced.DocumentMetrics,
		ImageMetadata:    file.Enhanced.ImageMetadata,
		AudioMetadata:    file.Enhanced.AudioMetadata,
		VideoMetadata:    file.Enhanced.VideoMetadata,
		IndexedState:     file.Enhanced.IndexedState,
	}

	if file.Enhanced != nil && file.Enhanced.MimeType != nil {
		entity.FileData.MimeType = &file.Enhanced.MimeType.MimeType
		entity.FileData.ContentType = &file.Enhanced.MimeType.Category
	}

	return entity
}

// FromFolderEntry converts FolderEntry to Entity
func FromFolderEntry(workspaceID WorkspaceID, folder *FolderEntry) *Entity {
	if folder == nil {
		return nil
	}

	entity := &Entity{
		ID:          NewEntityID(EntityTypeFolder, string(folder.ID)),
		Type:        EntityTypeFolder,
		WorkspaceID: workspaceID,
		Name:        folder.Name,
		Path:        folder.RelativePath,
		CreatedAt:   folder.CreatedAt,
		UpdatedAt:   folder.UpdatedAt,
	}

	// Extract size from metrics
	if folder.Metrics != nil {
		size := folder.Metrics.TotalSize
		entity.Size = &size
	}

	// Extract semantic metadata from FolderMetadata
	if folder.Metadata != nil {
		entity.Tags = folder.Metadata.UserTags
		if folder.Metadata.InferredTags != nil {
			// Merge inferred tags
			entity.Tags = append(entity.Tags, folder.Metadata.InferredTags...)
		}
		if folder.Metadata.UserProject != nil {
			entity.Projects = []string{*folder.Metadata.UserProject}
		}
		entity.Language = folder.Metadata.DominantLanguage
		if folder.Metadata.ProjectNature != nil {
			entity.Category = folder.Metadata.ProjectNature
		}
		entity.Description = folder.Metadata.ContentDescription
		entity.AISummary = folder.Metadata.AISummary
		entity.AIKeywords = folder.Metadata.AIKeywords
	}

	// Extract quality metrics from FolderMetrics
	if folder.Metrics != nil {
		complexity := folder.Metrics.AverageComplexity
		if complexity > 0 {
			entity.Complexity = &complexity
		}
		loc := folder.Metrics.TotalLinesOfCode
		if loc > 0 {
			entity.LinesOfCode = &loc
		}
	}

	// Set folder-specific data
	entity.FolderData = &FolderEntityData{
		Depth:            folder.Depth,
		TotalFiles:       getIntValue(folder.Metrics, func(m *FolderMetrics) int { return m.TotalFiles }),
		DirectFiles:      getIntValue(folder.Metrics, func(m *FolderMetrics) int { return m.DirectFiles }),
		Subfolders:       getIntValue(folder.Metrics, func(m *FolderMetrics) int { return m.TotalSubfolders }),
		FolderMetrics:    folder.Metrics,
		DominantFileType: getStringPtr(folder.Metadata, func(m *FolderMetadata) *string { return m.DominantFileType }),
		RelativePath:     folder.RelativePath,
		AbsolutePath:     folder.AbsolutePath,
	}

	return entity
}

// FromProject converts Project to Entity
func FromProject(workspaceID WorkspaceID, project *Project, documentCount int) *Entity {
	if project == nil {
		return nil
	}

	entity := &Entity{
		ID:          NewEntityID(EntityTypeProject, string(project.ID)),
		Type:        EntityTypeProject,
		WorkspaceID: workspaceID,
		Name:        project.Name,
		Path:        project.Path,
		Description: &project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
		Category:    stringPtr(string(project.Nature)),
	}

	// Extract from ProjectAttributes
	if project.Attributes != nil {
		entity.Status = &project.Attributes.Status
		entity.Priority = &project.Attributes.Priority
		entity.Visibility = &project.Attributes.Visibility
		// Extract unified entity metadata
		entity.Tags = project.Attributes.Tags
		entity.Language = project.Attributes.Language
		entity.Author = project.Attributes.Author
		entity.Owner = project.Attributes.Owner
		entity.Location = project.Attributes.Location
		entity.PublicationYear = project.Attributes.PublicationYear
		entity.AISummary = project.Attributes.AISummary
		entity.AIKeywords = project.Attributes.AIKeywords
	}

	// Extract projects from parent relationship
	if project.ParentID != nil {
		// Parent project name would need to be fetched from repository
		// For now, we'll use the parent ID
		entity.Projects = []string{string(*project.ParentID)}
	}

	// Set project-specific data
	entity.ProjectData = &ProjectEntityData{
		Nature:        project.Nature,
		Attributes:    project.Attributes,
		ParentID:      project.ParentID,
		DocumentCount: documentCount,
	}

	return entity
}

// Helper functions
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getStringPtr[T any](v *T, fn func(*T) *string) *string {
	if v == nil {
		return nil
	}
	return fn(v)
}

func getIntValue[T any](v *T, fn func(*T) int) int {
	if v == nil {
		return 0
	}
	return fn(v)
}

func getFirstString(slice []string) *string {
	if len(slice) == 0 {
		return nil
	}
	return &slice[0]
}

func getParentPath(path string) string {
	if path == "" || path == "." {
		return ""
	}
	parent := filepath.Dir(path)
	if parent == "." {
		return ""
	}
	return parent
}

func stringPtr(s string) *string {
	return &s
}

