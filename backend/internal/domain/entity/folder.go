// Package entity contains domain entities for the Cortex system.
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"time"
)

// FolderID is a stable identifier for a folder (SHA-256 hash of relative path).
type FolderID string

// NewFolderID creates a FolderID from a relative path.
func NewFolderID(relativePath string) FolderID {
	normalized := filepath.ToSlash(relativePath)
	hash := sha256.Sum256([]byte("folder:" + normalized))
	return FolderID(hex.EncodeToString(hash[:]))
}

// String returns the string representation of the FolderID.
func (id FolderID) String() string {
	return string(id)
}

// FolderEntry represents a folder in the workspace index.
type FolderEntry struct {
	ID           FolderID
	RelativePath string
	AbsolutePath string
	Name         string
	ParentPath   string
	Depth        int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Metrics      *FolderMetrics
	Metadata     *FolderMetadata
}

// NewFolderEntry creates a new FolderEntry from path information.
func NewFolderEntry(workspaceRoot, relativePath string) *FolderEntry {
	absPath := filepath.Join(workspaceRoot, relativePath)
	name := filepath.Base(relativePath)
	parentPath := filepath.Dir(relativePath)
	if parentPath == "." {
		parentPath = ""
	}

	// Calculate depth
	depth := 0
	if relativePath != "" && relativePath != "." {
		for _, c := range relativePath {
			if c == '/' || c == filepath.Separator {
				depth++
			}
		}
		depth++ // Add one for the folder itself
	}

	return &FolderEntry{
		ID:           NewFolderID(relativePath),
		RelativePath: relativePath,
		AbsolutePath: absPath,
		Name:         name,
		ParentPath:   parentPath,
		Depth:        depth,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// FolderMetrics contains aggregated metrics for a folder.
type FolderMetrics struct {
	// File counts
	TotalFiles       int
	DirectFiles      int // Files directly in this folder
	RecursiveFiles   int // Files in subfolders

	// Size metrics
	TotalSize        int64
	DirectSize       int64
	RecursiveSize    int64

	// Subfolder counts
	DirectSubfolders int
	TotalSubfolders  int

	// Type distribution
	FileTypeCounts   map[string]int // Extension -> count
	MimeTypeCounts   map[string]int // MIME category -> count

	// Temporal metrics
	OldestFile       *time.Time
	NewestFile       *time.Time
	LastAccessed     *time.Time

	// Code metrics (aggregated from files)
	TotalLinesOfCode    int
	TotalCommentLines   int
	TotalFunctions      int
	TotalClasses        int
	AverageComplexity   float64

	// Document metrics (aggregated)
	TotalPages          int
	TotalWords          int
	TotalDocuments      int
}

// FolderMetadata contains semantic metadata for a folder.
type FolderMetadata struct {
	// Inferred project information
	InferredProject     *string
	ProjectConfidence   float64
	ProjectNature       *string // development, documentation, collection, etc.

	// Tags and categories
	InferredTags        []string
	InferredCategories  []string

	// Content analysis
	DominantLanguage    *string
	DominantFileType    *string
	ContentDescription  *string

	// Relationship to other folders
	RelatedFolders      []string // Folder IDs of related folders

	// User-assigned metadata
	UserTags            []string
	UserProject         *string
	UserNotes           *string

	// AI-generated metadata
	AISummary           *string
	AIKeywords          []string

	// Unified entity metadata (for facet filtering)
	Author          *string  // Inferred from files
	Owner           *string  // Inferred from files
	Location        *string  // Inferred from files
	PublicationYear *int     // Inferred from files
	Status          *string  // Folder status (e.g., "active", "archived")
	Priority        *string  // Folder priority
	Visibility      *string  // Folder visibility
}

// FolderNature represents the inferred nature of a folder's contents.
type FolderNature string

const (
	FolderNatureUnknown       FolderNature = "unknown"
	FolderNatureDevelopment   FolderNature = "development"
	FolderNatureDocumentation FolderNature = "documentation"
	FolderNatureMedia         FolderNature = "media"
	FolderNatureData          FolderNature = "data"
	FolderNatureConfiguration FolderNature = "configuration"
	FolderNatureTests         FolderNature = "tests"
	FolderNatureResources     FolderNature = "resources"
	FolderNatureVendor        FolderNature = "vendor"
	FolderNatureBuild         FolderNature = "build"
	FolderNatureArchive       FolderNature = "archive"
	FolderNatureMixed         FolderNature = "mixed"
)
