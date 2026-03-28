// Package stages provides extractor implementations based on pipeline stages.
package stages

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// BasicMetadataExtractor extracts basic file metadata.
// This is a wrapper around BasicStage that implements MetadataExtractor.
type BasicMetadataExtractor struct {
	stage *BasicStage
}

// NewBasicMetadataExtractor creates a new basic metadata extractor.
func NewBasicMetadataExtractor() service.MetadataExtractor {
	return &BasicMetadataExtractor{
		stage: NewBasicStage(),
	}
}

// Extract extracts basic file information.
func (e *BasicMetadataExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
	return e.stage.Process(ctx, entry)
}

// CanExtract returns true for all files (basic stage processes all files).
func (e *BasicMetadataExtractor) CanExtract(entry *entity.FileEntry) bool {
	return true
}

// GetPriority returns the priority (100 = highest, runs first).
func (e *BasicMetadataExtractor) GetPriority() int {
	return 100
}

// MimeMetadataExtractor extracts MIME type information.
type MimeMetadataExtractor struct {
	stage *MimeStage
}

// NewMimeMetadataExtractor creates a new MIME metadata extractor.
func NewMimeMetadataExtractor() service.MetadataExtractor {
	return &MimeMetadataExtractor{
		stage: NewMimeStage(),
	}
}

// Extract extracts MIME type information.
func (e *MimeMetadataExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
	return e.stage.Process(ctx, entry)
}

// CanExtract returns true for all files.
func (e *MimeMetadataExtractor) CanExtract(entry *entity.FileEntry) bool {
	return true
}

// GetPriority returns the priority (90 = runs after basic).
func (e *MimeMetadataExtractor) GetPriority() int {
	return 90
}

// CodeMetadataExtractor extracts code metrics from source files.
type CodeMetadataExtractor struct {
	stage *CodeStage
}

// NewCodeMetadataExtractor creates a new code metadata extractor.
func NewCodeMetadataExtractor() service.MetadataExtractor {
	return &CodeMetadataExtractor{
		stage: NewCodeStage(),
	}
}

// Extract extracts code metrics.
func (e *CodeMetadataExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
	return e.stage.Process(ctx, entry)
}

// CanExtract returns true only for code files.
func (e *CodeMetadataExtractor) CanExtract(entry *entity.FileEntry) bool {
	return e.stage.CanProcess(entry)
}

// GetPriority returns the priority (80 = runs after MIME).
func (e *CodeMetadataExtractor) GetPriority() int {
	return 80
}

// OSMetadataExtractor extracts OS-level metadata.
// Note: This extractor requires a logger, so it's not included in CreateDefaultExtractors.
// Use NewOSMetadataExtractorWithLogger() to create it with proper logging.
type OSMetadataExtractor struct {
	stage *OSMetadataStage
}

// NewOSMetadataExtractorWithLogger creates a new OS metadata extractor with a logger and file repository.
func NewOSMetadataExtractorWithLogger(fileRepo repository.FileRepository, logger zerolog.Logger) service.MetadataExtractor {
	return &OSMetadataExtractor{
		stage: NewOSMetadataStage(fileRepo, logger),
	}
}

// Extract extracts OS metadata.
func (e *OSMetadataExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
	return e.stage.Process(ctx, entry)
}

// CanExtract returns true for all files.
func (e *OSMetadataExtractor) CanExtract(entry *entity.FileEntry) bool {
	return true
}

// GetPriority returns the priority (70 = runs after code).
func (e *OSMetadataExtractor) GetPriority() int {
	return 70
}

// CreateDefaultExtractors creates a default set of extractors with proper priorities.
// This is a convenience function for setting up extraction pipelines.
func CreateDefaultExtractors() []service.MetadataExtractor {
	return []service.MetadataExtractor{
		NewBasicMetadataExtractor(), // Priority: 100
		NewMimeMetadataExtractor(),  // Priority: 90
		NewCodeMetadataExtractor(),  // Priority: 80
		// OSMetadataExtractor would need proper logger setup
	}
}
