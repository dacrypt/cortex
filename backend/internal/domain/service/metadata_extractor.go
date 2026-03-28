// Package service defines domain service interfaces.
package service

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// MetadataExtractor defines the interface for extracting metadata from files.
// This abstraction allows for different extraction strategies and implementations.
type MetadataExtractor interface {
	// Extract extracts metadata from a file entry.
	// The entry may be modified in-place with extracted metadata.
	Extract(ctx context.Context, entry *entity.FileEntry) error

	// CanExtract returns true if this extractor can process the given file.
	CanExtract(entry *entity.FileEntry) bool

	// GetPriority returns the priority for this extractor (higher = earlier execution).
	// Used when multiple extractors can process the same file.
	GetPriority() int
}

// ContentExtractor defines the interface for extracting text content from files.
// Used for PDF, Office documents, etc.
type ContentExtractor interface {
	// ExtractContent extracts text content from a file.
	ExtractContent(ctx context.Context, entry *entity.FileEntry) (string, error)

	// CanExtract returns true if this extractor can process the given file.
	CanExtract(entry *entity.FileEntry) bool

	// GetSupportedMimeTypes returns the MIME types this extractor supports.
	GetSupportedMimeTypes() []string

	// GetSupportedExtensions returns the file extensions this extractor supports.
	GetSupportedExtensions() []string
}

// DocumentClassifier defines the interface for AI-powered document classification.
// Used for project assignment, tag suggestions, etc.
type DocumentClassifier interface {
	// Classify classifies a document and returns suggestions.
	Classify(ctx context.Context, entry *entity.FileEntry, content string) (*ClassificationResult, error)

	// SuggestProjects suggests projects for a document.
	SuggestProjects(ctx context.Context, entry *entity.FileEntry, content string) ([]string, error)

	// SuggestTags suggests tags for a document.
	SuggestTags(ctx context.Context, entry *entity.FileEntry, content string) ([]string, error)

	// GenerateSummary generates a summary for a document.
	GenerateSummary(ctx context.Context, entry *entity.FileEntry, content string) (string, error)
}

// ClassificationResult contains classification results.
type ClassificationResult struct {
	SuggestedProjects []string
	SuggestedTags     []string
	Summary           *string
	Confidence        float64
	Reasoning         *string
}






