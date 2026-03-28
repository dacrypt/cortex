// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
)

// MetadataStage extracts comprehensive metadata from files using specialized extractors.
type MetadataStage struct {
	registry *metadata.Registry
	logger   zerolog.Logger
}

// NewMetadataStage creates a new metadata extraction stage.
func NewMetadataStage(registry *metadata.Registry, logger zerolog.Logger) *MetadataStage {
	return &MetadataStage{
		registry: registry,
		logger:   logger.With().Str("component", "metadata_stage").Logger(),
	}
}

// Name returns the stage name.
func (s *MetadataStage) Name() string {
	return "metadata"
}

// CanProcess returns true if any extractor can handle this file.
func (s *MetadataStage) CanProcess(entry *entity.FileEntry) bool {
	if entry == nil || s.registry == nil {
		return false
	}
	// Check if any extractor can handle this file
	// We'll let Extract handle the actual check
	return true
}

// Process extracts metadata from the file using the appropriate extractor.
func (s *MetadataStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if s.registry == nil {
		return nil // No extractors registered, skip silently
	}

	// Extract metadata using the registry
	extracted, err := s.registry.Extract(ctx, entry)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("file", entry.RelativePath).
			Msg("Failed to extract metadata")
		return nil // Don't fail the pipeline on metadata extraction errors
	}

	if extracted == nil {
		return nil // No extractor found for this file type, not an error
	}

	// Merge extracted metadata into entry.Enhanced
	s.mergeMetadata(entry, extracted)

	s.logger.Debug().
		Str("file", entry.RelativePath).
		Msg("Extracted metadata")

	return nil
}

// mergeMetadata merges extracted metadata into entry.Enhanced.
func (s *MetadataStage) mergeMetadata(entry *entity.FileEntry, extracted *entity.EnhancedMetadata) {
	if entry.Enhanced == nil {
		entry.Enhanced = extracted
		return
	}

	s.mergeDocumentMetrics(entry.Enhanced, extracted)
	s.mergeMediaMetadata(entry.Enhanced, extracted)
	s.mergeCustomData(entry.Enhanced, extracted)
	s.mergeTikaMetadata(entry.Enhanced, extracted)
}

// mergeDocumentMetrics merges document metrics if present.
func (s *MetadataStage) mergeDocumentMetrics(target, source *entity.EnhancedMetadata) {
	if source.DocumentMetrics == nil {
		return
	}

	if target.DocumentMetrics == nil {
		target.DocumentMetrics = source.DocumentMetrics
	} else {
		mergeDocumentMetrics(target.DocumentMetrics, source.DocumentMetrics)
	}
}

// mergeMediaMetadata merges image, audio, and video metadata.
func (s *MetadataStage) mergeMediaMetadata(target, source *entity.EnhancedMetadata) {
	if source.ImageMetadata != nil {
		target.ImageMetadata = source.ImageMetadata
	}
	if source.AudioMetadata != nil {
		target.AudioMetadata = source.AudioMetadata
	}
	if source.VideoMetadata != nil {
		target.VideoMetadata = source.VideoMetadata
	}
}

// mergeCustomData merges custom data maps.
func (s *MetadataStage) mergeCustomData(target, source *entity.EnhancedMetadata) {
	if source.CustomData == nil {
		return
	}

	if target.CustomData == nil {
		target.CustomData = make(map[string]interface{})
	}
	for k, v := range source.CustomData {
		target.CustomData[k] = v
	}
}

// mergeDocumentMetrics merges source into target, preserving existing values.
func mergeDocumentMetrics(target, source *entity.DocumentMetrics) {
	if source == nil {
		return
	}

	mergeBasicFields(target, source)
	mergeXMPFields(target, source)
	mergeDocumentFields(target, source)
	mergeCustomProperties(target, source)
}

// mergeBasicFields merges basic document metadata fields.
func mergeBasicFields(target, source *entity.DocumentMetrics) {
	mergeStringPtr(&target.Title, source.Title)
	mergeStringPtr(&target.Author, source.Author)
	mergeStringPtr(&target.Subject, source.Subject)
	mergeStringPtr(&target.Creator, source.Creator)
	mergeStringPtr(&target.Producer, source.Producer)
	mergeTimePtr(&target.CreatedDate, source.CreatedDate)
	mergeTimePtr(&target.ModifiedDate, source.ModifiedDate)
	mergeStringSlice(&target.Keywords, source.Keywords)
	mergeIntValue(&target.PageCount, source.PageCount)
	mergeIntValue(&target.WordCount, source.WordCount)
	mergeIntValue(&target.CharacterCount, source.CharacterCount)
}

// mergeXMPFields merges XMP metadata fields.
func mergeXMPFields(target, source *entity.DocumentMetrics) {
	mergeStringPtr(&target.XMPTitle, source.XMPTitle)
	mergeStringPtr(&target.XMPDescription, source.XMPDescription)
	mergeStringSlice(&target.XMPCreator, source.XMPCreator)
	mergeStringPtr(&target.XMPRights, source.XMPRights)
	mergeStringPtr(&target.XMPCopyright, source.XMPCopyright)
}

// mergeDocumentFields merges document structure fields.
func mergeDocumentFields(target, source *entity.DocumentMetrics) {
	mergeStringSlice(&target.Fonts, source.Fonts)
	mergeStringSlice(&target.Hyperlinks, source.Hyperlinks)
	mergeIntPtr(&target.ImageCount, source.ImageCount)
	mergeFormFields(target, source)
	mergeAnnotations(target, source)
	mergeStringSlice(&target.ColorSpace, source.ColorSpace)
}

// mergeFormFields merges form fields if present.
func mergeFormFields(target, source *entity.DocumentMetrics) {
	if source.FormFields != nil && target.FormFields == nil {
		target.FormFields = source.FormFields
	}
}

// mergeAnnotations merges annotations if present.
func mergeAnnotations(target, source *entity.DocumentMetrics) {
	if source.Annotations != nil && target.Annotations == nil {
		target.Annotations = source.Annotations
	}
}

// mergeCustomProperties merges custom properties (always merge, don't skip existing).
func mergeCustomProperties(target, source *entity.DocumentMetrics) {
	if source.CustomProperties == nil {
		return
	}

	if target.CustomProperties == nil {
		target.CustomProperties = make(map[string]string)
	}
	for k, v := range source.CustomProperties {
		target.CustomProperties[k] = v
	}
}

// Helper functions for merging different field types

func mergeStringPtr(target **string, source *string) {
	if source != nil && *target == nil {
		*target = source
	}
}

func mergeTimePtr(target **time.Time, source *time.Time) {
	if source != nil && *target == nil {
		*target = source
	}
}

func mergeStringSlice(target *[]string, source []string) {
	if len(source) > 0 && len(*target) == 0 {
		*target = source
	}
}

func mergeIntValue(target *int, source int) {
	if source > 0 && *target == 0 {
		*target = source
	}
}

func mergeIntPtr(target **int, source *int) {
	if source != nil && *target == nil {
		*target = source
	}
}

// mergeTikaMetadata merges Tika metadata if present.
func (s *MetadataStage) mergeTikaMetadata(target, source *entity.EnhancedMetadata) {
	if source.TikaMetadata != nil {
		target.TikaMetadata = source.TikaMetadata
	}
}
