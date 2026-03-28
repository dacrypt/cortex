// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/mirror"
)

// MirrorStage extracts mirror content (md/csv) for office/PDF files.
type MirrorStage struct {
	extractor  *mirror.Extractor
	metaRepo   repository.MetadataRepository
	ocrService *metadata.OCRService
	logger     zerolog.Logger
}

// NewMirrorStage creates a new mirror extraction stage.
func NewMirrorStage(extractor *mirror.Extractor, metaRepo repository.MetadataRepository, ocrService *metadata.OCRService, logger zerolog.Logger) *MirrorStage {
	return &MirrorStage{
		extractor:  extractor,
		metaRepo:   metaRepo,
		ocrService: ocrService,
		logger:     logger.With().Str("stage", "mirror").Logger(),
	}
}

// Name returns the stage name.
func (s *MirrorStage) Name() string {
	return "mirror"
}

// CanProcess returns true for mirrorable extensions.
func (s *MirrorStage) CanProcess(entry *entity.FileEntry) bool {
	if entry == nil {
		return false
	}
	ext := strings.ToLower(entry.Extension)
	if ext == ".md" || ext == ".markdown" {
		return false
	}
	return mirror.Mirrorable(entry.AbsolutePath)
}

// Process extracts and stores mirror content.
func (s *MirrorStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) || s.extractor == nil || s.metaRepo == nil {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return nil
	}

	_, err := s.metaRepo.GetOrCreate(ctx, wsInfo.ID, entry.RelativePath, entry.Extension)
	if err != nil {
		return err
	}

	mirrorMeta, content, err := s.extractor.EnsureMirror(ctx, wsInfo.Root, entry)
	if err != nil {
		// Check if this is an expected "no mirror" case (file too large, empty content, etc.)
		// vs an actual extraction failure
		if strings.Contains(err.Error(), "no mirror content") {
			if strings.EqualFold(entry.Extension, ".pdf") {
				ocrDir := filepath.Join(wsInfo.Root, ".cortex", "mirror")
				ocrContent, ocrErr := s.tryOCRPDF(ctx, entry, ocrDir)
				if ocrErr == nil && strings.TrimSpace(ocrContent) != "" {
					content = ocrContent
					sourceInfo, statErr := os.Stat(entry.AbsolutePath)
					sourceMtime := time.Now()
					if statErr == nil {
						sourceMtime = sourceInfo.ModTime()
					}
					mirrorMeta = &entity.MirrorMetadata{
						Format:      entity.MirrorFormatMarkdown,
						Path:        filepath.Join(wsInfo.Root, entity.GetMirrorPath(entry.RelativePath, entity.MirrorFormatMarkdown)),
						SourceMtime: sourceMtime,
						UpdatedAt:   time.Now(),
					}
				}
			}

			if mirrorMeta == nil {
				errMsg := "No mirror content available"
				details := fmt.Sprintf("Mirror extraction returned no content for %s. This file likely has no extractable text layer (image-only PDF) or exceeded mirror limits.", entry.RelativePath)
				if entry.Enhanced == nil {
					entry.Enhanced = &entity.EnhancedMetadata{}
				}
				entry.Enhanced.AddIndexingError(
					"mirror",
					"no_mirror_content",
					errMsg,
					details,
					"mirror_content",
				)
				s.logger.Warn().
					Str("path", entry.RelativePath).
					Str("extension", entry.Extension).
					Msg("Indexing error: No mirror content available - stopping pipeline")
				return fmt.Errorf("indexing error: no mirror content available for %s", entry.RelativePath)
			}
			err = nil
		}

		if err != nil {
			// This is an actual extraction failure - mark as indexing error and stop pipeline
			errMsg := fmt.Sprintf("Mirror extraction failed: %v", err)
			details := fmt.Sprintf("Failed to extract mirror content for %s. Error: %v. This file requires a mirror for document processing.", entry.RelativePath, err)

			// Ensure EnhancedMetadata exists
			if entry.Enhanced == nil {
				entry.Enhanced = &entity.EnhancedMetadata{}
			}
			entry.Enhanced.AddIndexingError(
				"mirror",
				"extract_mirror",
				errMsg,
				details,
				"mirror_extraction",
			)

			s.logger.Warn().
				Err(err).
				Str("path", entry.RelativePath).
				Str("extension", entry.Extension).
				Msg("Indexing error: Mirror extraction failed - stopping pipeline")

			// Return error to stop pipeline - this is a fatal error for files that need mirrors
			return fmt.Errorf("indexing error: mirror extraction failed for %s: %w", entry.RelativePath, err)
		}
	}

	if mirrorMeta != nil {
		if mirrorMeta.Path == "" {
			mirrorMeta.Path = filepath.Join(wsInfo.Root, entity.GetMirrorPath(entry.RelativePath, mirrorMeta.Format))
		}
		if strings.TrimSpace(content) != "" {
			if err := os.MkdirAll(filepath.Dir(mirrorMeta.Path), 0o755); err != nil {
				s.logger.Warn().Err(err).Str("dir", filepath.Dir(mirrorMeta.Path)).Msg("Failed to create mirror directory")
			} else if err := os.WriteFile(mirrorMeta.Path, []byte(content), 0o644); err != nil {
				s.logger.Warn().Err(err).Str("path", mirrorMeta.Path).Msg("Failed to write mirror file")
			}
		}
		if err := s.metaRepo.UpdateMirror(ctx, wsInfo.ID, entry.ID, *mirrorMeta); err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to update mirror metadata")
		}
		if entry.Enhanced == nil {
			entry.Enhanced = &entity.EnhancedMetadata{}
		}
		entry.Enhanced.IndexedState.Mirror = true
		if entry.Enhanced.DocumentMetrics == nil {
			entry.Enhanced.DocumentMetrics = &entity.DocumentMetrics{}
		}
		applyMirrorMetrics(entry, content)
	}

	if strings.TrimSpace(content) == "" {
		return nil
	}
	return nil
}

func (s *MirrorStage) tryOCRPDF(ctx context.Context, entry *entity.FileEntry, ocrDir string) (string, error) {
	if s.ocrService == nil || !s.ocrService.IsAvailable() {
		return "", fmt.Errorf("ocr not available")
	}
	ocrResult, err := s.ocrService.ExtractTextFromPDF(ctx, entry.AbsolutePath, "", ocrDir)
	if err != nil || ocrResult == nil || strings.TrimSpace(ocrResult.Text) == "" {
		if err != nil {
			s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("OCR failed for PDF mirror fallback")
		}
		return "", fmt.Errorf("ocr produced no content")
	}
	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("content_size", len([]byte(ocrResult.Text))).
		Msg("OCR produced mirror content for PDF")
	return ocrResult.Text, nil
}

func applyMirrorMetrics(entry *entity.FileEntry, content string) {
	words := countWords(content)
	entry.Enhanced.DocumentMetrics.WordCount = words
	entry.Enhanced.DocumentMetrics.CharacterCount = len([]rune(content))
	if entry.Enhanced.DocumentMetrics.Title == nil {
		title := entry.Filename
		entry.Enhanced.DocumentMetrics.Title = &title
	}
}
