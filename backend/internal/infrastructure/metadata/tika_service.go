package metadata

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// TikaService orchestrates Tika extraction with fallback to existing extractors.
type TikaService struct {
	client   *TikaClient
	enabled  bool
	logger   zerolog.Logger
	fallback Extractor // Fallback to existing extractors
}

// NewTikaService creates a new Tika service.
func NewTikaService(client *TikaClient, enabled bool, fallback Extractor, logger zerolog.Logger) *TikaService {
	return &TikaService{
		client:   client,
		enabled:  enabled,
		logger:   logger.With().Str("component", "tika_service").Logger(),
		fallback: fallback,
	}
}

// HealthCheck verifies that Tika Server is available.
func (s *TikaService) HealthCheck(ctx context.Context) error {
	if !s.enabled {
		return fmt.Errorf("tika service is disabled")
	}
	return s.client.HealthCheck(ctx)
}

// ExtractMetadata extracts metadata using Tika with fallback.
func (s *TikaService) ExtractMetadata(ctx context.Context, entry *entity.FileEntry) (*entity.TikaMetadata, error) {
	if !s.enabled {
		s.logger.Debug().Str("file", entry.RelativePath).Msg("Tika service is disabled, skipping")
		return nil, nil
	}

	// Check file size limit (if configured)
	// For now, we'll extract regardless of size, but Tika Server may have its own limits

	// Try Tika extraction
	tikaMeta, err := s.extractWithTika(ctx, entry)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("file", entry.RelativePath).
			Msg("Tika extraction failed, will use fallback if available")
		
		// If fallback is available, try it
		if s.fallback != nil && s.fallback.CanExtract(entry.Extension) {
			enhanced, fallbackErr := s.fallback.Extract(ctx, entry)
			if fallbackErr == nil && enhanced != nil {
				// Convert EnhancedMetadata to TikaMetadata
				return s.convertEnhancedToTika(enhanced), nil
			}
		}
		return nil, err
	}

	return tikaMeta, nil
}

// extractWithTika performs the actual Tika extraction.
func (s *TikaService) extractWithTika(ctx context.Context, entry *entity.FileEntry) (*entity.TikaMetadata, error) {
	// Extract metadata from Tika
	rawMetadata, err := s.client.ExtractMetadata(ctx, entry.AbsolutePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Parse and map to TikaMetadata
	tikaMeta := s.parseTikaMetadata(rawMetadata)

	// Optionally extract text and language
	// (These are separate calls and can be expensive, so we do them optionally)
	if text, err := s.client.ExtractText(ctx, entry.AbsolutePath); err == nil {
		// Store text length as character count if not already set
		if tikaMeta.CharacterCount == nil {
			count := len(text)
			tikaMeta.CharacterCount = &count
		}
	}

	if lang, err := s.client.DetectLanguage(ctx, entry.AbsolutePath); err == nil {
		tikaMeta.LanguageCode = &lang
	}

	return tikaMeta, nil
}

// parseTikaMetadata maps Tika's JSON metadata to TikaMetadata struct.
func (s *TikaService) parseTikaMetadata(raw map[string]interface{}) *entity.TikaMetadata {
	meta := entity.NewTikaMetadata()

	// Map standard Tika fields
	for key, value := range raw {
		valueStr := fmt.Sprintf("%v", value)
		
		switch strings.ToLower(key) {
		case "title", "dc:title", "dcterms:title":
			if meta.Title == nil {
				meta.Title = &valueStr
			}
		case "author", "dc:creator", "dcterms:creator", "meta:author":
			if arr, ok := value.([]interface{}); ok {
				for _, v := range arr {
					meta.Author = append(meta.Author, fmt.Sprintf("%v", v))
				}
			} else {
				meta.Author = append(meta.Author, valueStr)
			}
		case "creator", "xmp:creator", "xmp:creatortool":
			if arr, ok := value.([]interface{}); ok {
				for _, v := range arr {
					meta.Creator = append(meta.Creator, fmt.Sprintf("%v", v))
				}
			} else {
				meta.Creator = append(meta.Creator, valueStr)
			}
		case "subject", "dc:subject", "dcterms:subject":
			if meta.Subject == nil {
				meta.Subject = &valueStr
			}
		case "description", "dc:description", "dcterms:description", "xmp:description":
			if meta.Description == nil {
				meta.Description = &valueStr
			}
		case "keywords", "meta:keyword", "xmp:subject":
			if arr, ok := value.([]interface{}); ok {
				for _, v := range arr {
					meta.Keywords = append(meta.Keywords, fmt.Sprintf("%v", v))
				}
			} else {
				// Split comma-separated keywords
				keywords := strings.Split(valueStr, ",")
				for _, kw := range keywords {
					kw = strings.TrimSpace(kw)
					if kw != "" {
						meta.Keywords = append(meta.Keywords, kw)
					}
				}
			}
		case "language", "dc:language", "dcterms:language", "xmp:language":
			if meta.Language == nil {
				meta.Language = &valueStr
			}
		case "content-type", "contenttype":
			if meta.ContentType == nil {
				meta.ContentType = &valueStr
			}
		case "content-encoding", "contentencoding":
			if meta.ContentEncoding == nil {
				meta.ContentEncoding = &valueStr
			}
		case "creation-date", "dcterms:created", "xmp:createdate", "date", "meta:creation-date":
			if t := s.parseDate(valueStr); t != nil {
				meta.Created = t
			}
		case "last-modified", "dcterms:modified", "xmp:modifydate", "meta:last-modified":
			if t := s.parseDate(valueStr); t != nil {
				meta.Modified = t
			}
		case "last-saved", "meta:last-saved":
			if t := s.parseDate(valueStr); t != nil {
				meta.LastSaved = t
			}
		case "page-count", "xmp:pagecount", "meta:page-count":
			if count := s.parseInt(valueStr); count != nil {
				meta.PageCount = count
			}
		case "word-count", "meta:word-count":
			if count := s.parseInt(valueStr); count != nil {
				meta.WordCount = count
			}
		case "character-count", "meta:character-count":
			if count := s.parseInt(valueStr); count != nil {
				meta.CharacterCount = count
			}
		}

		// Store all fields in RawMetadata
		meta.RawMetadata[key] = value
	}

	return meta
}

// parseDate attempts to parse various date formats from Tika.
func (s *TikaService) parseDate(dateStr string) *time.Time {
	// Tika returns dates in various formats, try common ones
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}

	return nil
}

// parseInt attempts to parse an integer from a string.
func (s *TikaService) parseInt(str string) *int {
	if i, err := strconv.Atoi(strings.TrimSpace(str)); err == nil {
		return &i
	}
	return nil
}

// convertEnhancedToTika converts EnhancedMetadata to TikaMetadata (for fallback).
func (s *TikaService) convertEnhancedToTika(enhanced *entity.EnhancedMetadata) *entity.TikaMetadata {
	tika := entity.NewTikaMetadata()
	tika.ExtractedBy = "fallback"

	if enhanced.DocumentMetrics != nil {
		dm := enhanced.DocumentMetrics
		tika.Title = dm.Title
		if dm.Author != nil {
			tika.Author = []string{*dm.Author}
		}
		tika.Subject = dm.Subject
		tika.Creator = dm.XMPCreator
		if len(dm.Keywords) > 0 {
			tika.Keywords = dm.Keywords
		}
		if dm.PageCount > 0 {
			tika.PageCount = &dm.PageCount
		}
		if dm.WordCount > 0 {
			tika.WordCount = &dm.WordCount
		}
		if dm.CharacterCount > 0 {
			tika.CharacterCount = &dm.CharacterCount
		}
		tika.Created = dm.CreatedDate
		tika.Modified = dm.ModifiedDate
	}

	if enhanced.MimeType != nil {
		tika.ContentType = &enhanced.MimeType.MimeType
		tika.ContentEncoding = &enhanced.MimeType.Encoding
	}

	if enhanced.Language != nil {
		tika.LanguageCode = enhanced.Language
	}

	return tika
}

// CanExtract returns true if Tika can handle this file type (always true when enabled).
func (s *TikaService) CanExtract(extension string) bool {
	return s.enabled
}

// Extract implements the Extractor interface.
func (s *TikaService) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	tikaMeta, err := s.ExtractMetadata(ctx, entry)
	if err != nil {
		return nil, err
	}

	if tikaMeta == nil {
		return nil, nil
	}

	// Convert TikaMetadata to EnhancedMetadata
	return s.convertTikaToEnhanced(tikaMeta), nil
}

// convertTikaToEnhanced converts TikaMetadata to EnhancedMetadata.
func (s *TikaService) convertTikaToEnhanced(tika *entity.TikaMetadata) *entity.EnhancedMetadata {
	enhanced := &entity.EnhancedMetadata{
		DocumentMetrics: &entity.DocumentMetrics{},
	}

	dm := enhanced.DocumentMetrics

	// Map basic fields
	dm.Title = tika.Title
	if len(tika.Author) > 0 {
		author := tika.Author[0]
		dm.Author = &author
	}
	dm.Subject = tika.Subject
	if len(tika.Creator) > 0 {
		creator := tika.Creator[0]
		dm.Creator = &creator
	}
	dm.XMPCreator = tika.Creator
	dm.Keywords = tika.Keywords
	dm.CreatedDate = tika.Created
	dm.ModifiedDate = tika.Modified

	if tika.PageCount != nil {
		dm.PageCount = *tika.PageCount
	}
	if tika.WordCount != nil {
		dm.WordCount = *tika.WordCount
	}
	if tika.CharacterCount != nil {
		dm.CharacterCount = *tika.CharacterCount
	}

	// MIME type
	if tika.ContentType != nil {
		enhanced.MimeType = &entity.MimeTypeInfo{
			MimeType: *tika.ContentType,
		}
		if tika.ContentEncoding != nil {
			enhanced.MimeType.Encoding = *tika.ContentEncoding
		}
	}

	// Language
	if tika.LanguageCode != nil {
		enhanced.Language = tika.LanguageCode
	}

	// Store raw Tika metadata in CustomData
	if enhanced.CustomData == nil {
		enhanced.CustomData = make(map[string]interface{})
	}
	enhanced.CustomData["tika_metadata"] = tika.RawMetadata
	enhanced.CustomData["tika_extracted_by"] = tika.ExtractedBy
	enhanced.CustomData["tika_extraction_date"] = tika.ExtractionDate

	return enhanced
}

