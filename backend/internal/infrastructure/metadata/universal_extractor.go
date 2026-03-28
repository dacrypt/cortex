package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// UniversalExtractor uses all available forensic tools to extract maximum metadata from any file
type UniversalExtractor struct {
	logger         zerolog.Logger
	forensicRunner *ForensicToolRunner
}

// NewUniversalExtractor creates a new universal metadata extractor
func NewUniversalExtractor(logger zerolog.Logger) *UniversalExtractor {
	return &UniversalExtractor{
		logger:         logger.With().Str("component", "universal_metadata_extractor").Logger(),
		forensicRunner: NewForensicToolRunner(logger),
	}
}

// CanExtract returns true for all files (universal extractor)
func (e *UniversalExtractor) CanExtract(extension string) bool {
	return true // Can handle any file type
}

// Extract extracts all possible metadata using all available forensic tools
func (e *UniversalExtractor) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	enhanced := &entity.EnhancedMetadata{
		DocumentMetrics: &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		},
	}

	// Use forensic tools to extract everything possible
	forensicData, err := e.forensicRunner.ExtractEmbeddedMetadata(ctx, entry.AbsolutePath)
	if err != nil {
		e.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract forensic metadata")
	} else {
		// Store all forensic data in custom properties
		e.storeForensicData(enhanced, forensicData)
	}

	// Extract file-specific metadata based on type
	switch {
	case e.isPDF(entry.Extension):
		e.extractPDFSpecific(ctx, entry, enhanced, forensicData)
	case e.isImage(entry.Extension):
		e.extractImageSpecific(ctx, entry, enhanced, forensicData)
	case e.isAudio(entry.Extension):
		e.extractAudioSpecific(ctx, entry, enhanced, forensicData)
	case e.isVideo(entry.Extension):
		e.extractVideoSpecific(ctx, entry, enhanced, forensicData)
	default:
		e.extractGenericSpecific(ctx, entry, enhanced, forensicData)
	}

	return enhanced, nil
}

func (e *UniversalExtractor) isPDF(ext string) bool {
	return ext == ".pdf"
}

func (e *UniversalExtractor) isImage(ext string) bool {
	imageExts := []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".gif", ".bmp", ".webp", ".heic", ".heif", ".raw", ".cr2", ".nef", ".arw"}
	ext = strings.ToLower(ext)
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

func (e *UniversalExtractor) isAudio(ext string) bool {
	audioExts := []string{".mp3", ".flac", ".ogg", ".wav", ".aac", ".m4a", ".opus", ".wma", ".ape", ".mpc"}
	ext = strings.ToLower(ext)
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

func (e *UniversalExtractor) isVideo(ext string) bool {
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".3gp", ".mpg", ".mpeg", ".ts", ".m2ts"}
	ext = strings.ToLower(ext)
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

func (e *UniversalExtractor) storeForensicData(enhanced *entity.EnhancedMetadata, data map[string]interface{}) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	// Store all forensic data in custom properties
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			// Nested map - serialize to JSON
			if jsonData, err := json.Marshal(v); err == nil {
				enhanced.DocumentMetrics.CustomProperties["forensic_"+key] = string(jsonData)
			}
		case map[string]string:
			// String map - serialize to JSON
			if jsonData, err := json.Marshal(v); err == nil {
				enhanced.DocumentMetrics.CustomProperties["forensic_"+key] = string(jsonData)
			}
		case []string:
			// String array - join with separator
			enhanced.DocumentMetrics.CustomProperties["forensic_"+key] = strings.Join(v, " | ")
		case string:
			enhanced.DocumentMetrics.CustomProperties["forensic_"+key] = v
		default:
			// Convert to string
			enhanced.DocumentMetrics.CustomProperties["forensic_"+key] = fmt.Sprintf("%v", v)
		}
	}
}

func (e *UniversalExtractor) extractPDFSpecific(ctx context.Context, entry *entity.FileEntry, enhanced *entity.EnhancedMetadata, forensicData map[string]interface{}) {
	// Use pdfinfo if available
	if pdfInfo, ok := forensicData["pdfinfo"].(map[string]string); ok {
		e.parsePDFInfoData(enhanced, pdfInfo)
	}

	// Use exiftool data if available
	if exifData, ok := forensicData["exiftool"].(map[string]interface{}); ok {
		e.parseExifToolPDFData(enhanced, exifData)
	}
}

func (e *UniversalExtractor) extractImageSpecific(ctx context.Context, entry *entity.FileEntry, enhanced *entity.EnhancedMetadata, forensicData map[string]interface{}) {
	if enhanced.ImageMetadata == nil {
		enhanced.ImageMetadata = &entity.ImageMetadata{}
	}

	// Use exiftool data if available
	if exifData, ok := forensicData["exiftool"].(map[string]interface{}); ok {
		e.parseExifToolImageData(enhanced, exifData)
	}

	// Use ImageMagick identify if available
	if identifyData, ok := forensicData["identify"].(map[string]string); ok {
		e.parseIdentifyData(enhanced, identifyData)
	}
}

func (e *UniversalExtractor) extractAudioSpecific(ctx context.Context, entry *entity.FileEntry, enhanced *entity.EnhancedMetadata, forensicData map[string]interface{}) {
	if enhanced.AudioMetadata == nil {
		enhanced.AudioMetadata = &entity.AudioMetadata{}
	}

	// Use exiftool data if available
	if exifData, ok := forensicData["exiftool"].(map[string]interface{}); ok {
		e.parseExifToolAudioData(enhanced, exifData)
	}

	// Use ffprobe if available
	if ffprobeData, ok := forensicData["ffprobe"].(map[string]interface{}); ok {
		e.parseFFProbeAudioData(enhanced, ffprobeData)
	}
}

func (e *UniversalExtractor) extractVideoSpecific(ctx context.Context, entry *entity.FileEntry, enhanced *entity.EnhancedMetadata, forensicData map[string]interface{}) {
	if enhanced.VideoMetadata == nil {
		enhanced.VideoMetadata = &entity.VideoMetadata{}
	}

	// Use ffprobe if available
	if ffprobeData, ok := forensicData["ffprobe"].(map[string]interface{}); ok {
		e.parseFFProbeVideoData(enhanced, ffprobeData)
	}

	// Use exiftool data if available
	if exifData, ok := forensicData["exiftool"].(map[string]interface{}); ok {
		e.parseExifToolVideoData(enhanced, exifData)
	}
}

func (e *UniversalExtractor) extractGenericSpecific(ctx context.Context, entry *entity.FileEntry, enhanced *entity.EnhancedMetadata, forensicData map[string]interface{}) {
	// For generic files, extract what we can from file command, stat, etc.
	if fileInfo, ok := forensicData["file_command"].(map[string]string); ok {
		if mimeType, ok := fileInfo["mime_type"]; ok {
			// Store MIME type in custom properties (MimeType field may need special handling)
			if enhanced.DocumentMetrics == nil {
				enhanced.DocumentMetrics = &entity.DocumentMetrics{
					CustomProperties: make(map[string]string),
				}
			}
			enhanced.DocumentMetrics.CustomProperties["mime_type"] = mimeType
		}
	}
}

// Helper methods to parse specific tool outputs
func (e *UniversalExtractor) parsePDFInfoData(enhanced *entity.EnhancedMetadata, pdfInfo map[string]string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	// Parse standard PDF info fields
	if title, ok := pdfInfo["Title"]; ok && title != "" {
		dm.Title = &title
	}
	if author, ok := pdfInfo["Author"]; ok && author != "" {
		dm.Author = &author
	}
	if subject, ok := pdfInfo["Subject"]; ok && subject != "" {
		dm.Subject = &subject
	}
	if creator, ok := pdfInfo["Creator"]; ok && creator != "" {
		dm.Creator = &creator
	}
	if producer, ok := pdfInfo["Producer"]; ok && producer != "" {
		dm.Producer = &producer
	}
	if pages, ok := pdfInfo["Pages"]; ok {
		if pageCount, err := strconv.Atoi(pages); err == nil {
			dm.PageCount = pageCount
		}
	}
	if pdfVersion, ok := pdfInfo["PDF version"]; ok {
		dm.PDFVersion = &pdfVersion
	}

	// Store all other fields in custom properties
	for key, value := range pdfInfo {
		if !e.isStandardPDFField(key) {
			dm.CustomProperties["pdfinfo_"+key] = value
		}
	}
}

func (e *UniversalExtractor) isStandardPDFField(key string) bool {
	standardFields := []string{"Title", "Author", "Subject", "Creator", "Producer", "Pages", "PDF version", "CreationDate", "ModDate", "Keywords"}
	for _, field := range standardFields {
		if key == field {
			return true
		}
	}
	return false
}

func (e *UniversalExtractor) parseExifToolPDFData(enhanced *entity.EnhancedMetadata, exifData map[string]interface{}) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	// Extract XMP and PDF-specific fields
	for key, value := range exifData {
		valueStr := fmt.Sprintf("%v", value)
		if strings.HasPrefix(key, "XMP:") {
			xmpKey := strings.TrimPrefix(key, "XMP:")
			dm.CustomProperties["xmp_"+xmpKey] = valueStr
		} else if strings.HasPrefix(key, "PDF:") {
			pdfKey := strings.TrimPrefix(key, "PDF:")
			dm.CustomProperties["pdf_"+pdfKey] = valueStr
		} else {
			dm.CustomProperties["exiftool_"+key] = valueStr
		}
	}
}

func (e *UniversalExtractor) parseExifToolImageData(enhanced *entity.EnhancedMetadata, exifData map[string]interface{}) {
	// Parse EXIF, IPTC, XMP data for images
	// This would be similar to ImageExtractor logic
	for key, value := range exifData {
		valueStr := fmt.Sprintf("%v", value)
		if enhanced.DocumentMetrics == nil {
			enhanced.DocumentMetrics = &entity.DocumentMetrics{
				CustomProperties: make(map[string]string),
			}
		}
		enhanced.DocumentMetrics.CustomProperties["exiftool_"+key] = valueStr
	}
}

func (e *UniversalExtractor) parseIdentifyData(enhanced *entity.EnhancedMetadata, identifyData map[string]string) {
	// Parse ImageMagick identify output
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	for key, value := range identifyData {
		enhanced.DocumentMetrics.CustomProperties["identify_"+key] = value
	}
}

func (e *UniversalExtractor) parseExifToolAudioData(enhanced *entity.EnhancedMetadata, exifData map[string]interface{}) {
	// Parse audio metadata from exiftool
	for key, value := range exifData {
		valueStr := fmt.Sprintf("%v", value)
		if enhanced.DocumentMetrics == nil {
			enhanced.DocumentMetrics = &entity.DocumentMetrics{
				CustomProperties: make(map[string]string),
			}
		}
		enhanced.DocumentMetrics.CustomProperties["exiftool_"+key] = valueStr
	}
}

func (e *UniversalExtractor) parseFFProbeAudioData(enhanced *entity.EnhancedMetadata, ffprobeData map[string]interface{}) {
	// Parse ffprobe audio data
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	// Extract format and stream information
	if format, ok := ffprobeData["format"].(map[string]interface{}); ok {
		for key, value := range format {
			enhanced.DocumentMetrics.CustomProperties["ffprobe_format_"+key] = fmt.Sprintf("%v", value)
		}
	}

	if streams, ok := ffprobeData["streams"].([]interface{}); ok {
		for i, stream := range streams {
			if streamMap, ok := stream.(map[string]interface{}); ok {
				for key, value := range streamMap {
					enhanced.DocumentMetrics.CustomProperties[fmt.Sprintf("ffprobe_stream_%d_%s", i, key)] = fmt.Sprintf("%v", value)
				}
			}
		}
	}
}

func (e *UniversalExtractor) parseExifToolVideoData(enhanced *entity.EnhancedMetadata, exifData map[string]interface{}) {
	// Parse video metadata from exiftool
	for key, value := range exifData {
		valueStr := fmt.Sprintf("%v", value)
		if enhanced.DocumentMetrics == nil {
			enhanced.DocumentMetrics = &entity.DocumentMetrics{
				CustomProperties: make(map[string]string),
			}
		}
		enhanced.DocumentMetrics.CustomProperties["exiftool_"+key] = valueStr
	}
}

func (e *UniversalExtractor) parseFFProbeVideoData(enhanced *entity.EnhancedMetadata, ffprobeData map[string]interface{}) {
	// Parse ffprobe video data (similar to audio)
	e.parseFFProbeAudioData(enhanced, ffprobeData)
}

