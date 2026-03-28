package metadata

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// Extractor is the interface for extracting metadata from files.
// Each file type should have its own implementation.
type Extractor interface {
	// CanExtract returns true if this extractor can handle the given file extension.
	CanExtract(extension string) bool
	
	// Extract extracts metadata from the file and populates the EnhancedMetadata.
	// It should be safe to call on files that CanExtract returns false for (should return nil).
	Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error)
}

// Registry manages multiple extractors and routes files to the appropriate one.
type Registry struct {
	extractors []Extractor
}

// NewRegistry creates a new metadata extractor registry.
func NewRegistry() *Registry {
	return &Registry{
		extractors: make([]Extractor, 0),
	}
}

// Register adds an extractor to the registry.
func (r *Registry) Register(extractor Extractor) {
	r.extractors = append(r.extractors, extractor)
}

// Extract finds the appropriate extractor and extracts metadata from the file.
// Returns the extracted metadata or nil if no extractor can handle the file.
func (r *Registry) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	for _, extractor := range r.extractors {
		if extractor.CanExtract(entry.Extension) {
			return extractor.Extract(ctx, entry)
		}
	}
	return nil, nil // No extractor found, not an error
}







