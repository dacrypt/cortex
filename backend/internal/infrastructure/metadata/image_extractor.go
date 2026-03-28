package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// ImageExtractor extracts comprehensive metadata from image files (EXIF, IPTC, XMP).
type ImageExtractor struct {
	logger zerolog.Logger
}

// NewImageExtractor creates a new image metadata extractor.
func NewImageExtractor(logger zerolog.Logger) *ImageExtractor {
	return &ImageExtractor{
		logger: logger.With().Str("component", "image_metadata_extractor").Logger(),
	}
}

// CanExtract returns true for image files.
func (e *ImageExtractor) CanExtract(extension string) bool {
	ext := strings.ToLower(extension)
	imageExts := []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".gif", ".bmp", ".webp", ".heic", ".heif", ".raw", ".cr2", ".nef", ".arw"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// Extract extracts all possible metadata from an image file.
func (e *ImageExtractor) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	if !e.CanExtract(entry.Extension) {
		return nil, nil
	}

	enhanced := &entity.EnhancedMetadata{
		ImageMetadata: &entity.ImageMetadata{},
	}

	// Use exiftool for comprehensive extraction
	if err := e.extractWithExifTool(ctx, entry.AbsolutePath, enhanced); err != nil {
		e.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract image metadata with exiftool")
		// Try basic extraction as fallback
		return e.extractBasic(ctx, entry.AbsolutePath, enhanced)
	}

	return enhanced, nil
}

// extractWithExifTool uses exiftool to extract comprehensive image metadata.
func (e *ImageExtractor) extractWithExifTool(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found")
	}

	im := enhanced.ImageMetadata
	if im == nil {
		im = &entity.ImageMetadata{}
		enhanced.ImageMetadata = im
	}

	// Try JSON output first
	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-all", filePath)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		if err := e.parseExifToolJSON(output, im); err == nil {
			return nil
		}
		e.logger.Debug().Err(err).Msg("Failed to parse exiftool JSON, falling back to text")
	}

	// Fallback to text output
	cmd = exec.CommandContext(ctx, "exiftool", filePath)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w", err)
	}

	return e.parseExifToolText(output, im)
}

// parseExifToolJSON parses exiftool JSON output for images.
func (e *ImageExtractor) parseExifToolJSON(jsonData []byte, im *entity.ImageMetadata) error {
	var results []map[string]interface{}
	if err := json.Unmarshal(jsonData, &results); err != nil {
		return fmt.Errorf("failed to unmarshal exiftool JSON: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no data in exiftool output")
	}

	data := results[0]

	for key, value := range data {
		if value == nil {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		// Parse EXIF metadata
		if strings.HasPrefix(key, "EXIF:") {
			exifKey := strings.TrimPrefix(key, "EXIF:")
			e.setEXIFField(im, exifKey, value, valueStr)
		} else if strings.HasPrefix(key, "IPTC:") {
			iptcKey := strings.TrimPrefix(key, "IPTC:")
			e.setIPTCField(im, iptcKey, value, valueStr)
		} else if strings.HasPrefix(key, "XMP:") {
			xmpKey := strings.TrimPrefix(key, "XMP:")
			e.setXMPField(im, xmpKey, value, valueStr)
		} else if strings.HasPrefix(key, "GPS:") {
			gpsKey := strings.TrimPrefix(key, "GPS:")
			e.setGPSField(im, gpsKey, value, valueStr)
		} else {
			// Standard image properties
			e.setStandardField(im, key, value, valueStr)
		}
	}

	return nil
}

// parseExifToolText parses exiftool text output (fallback).
func (e *ImageExtractor) parseExifToolText(output []byte, im *entity.ImageMetadata) error {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=====") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.HasPrefix(key, "EXIF:") {
			exifKey := strings.TrimPrefix(key, "EXIF:")
			e.setEXIFField(im, exifKey, value, value)
		} else if strings.HasPrefix(key, "IPTC:") {
			iptcKey := strings.TrimPrefix(key, "IPTC:")
			e.setIPTCField(im, iptcKey, value, value)
		} else if strings.HasPrefix(key, "XMP:") {
			xmpKey := strings.TrimPrefix(key, "XMP:")
			e.setXMPField(im, xmpKey, value, value)
		} else if strings.HasPrefix(key, "GPS:") {
			gpsKey := strings.TrimPrefix(key, "GPS:")
			e.setGPSField(im, gpsKey, value, value)
		} else {
			e.setStandardField(im, key, value, value)
		}
	}

	return nil
}

// setEXIFField sets EXIF metadata fields.
func (e *ImageExtractor) setEXIFField(im *entity.ImageMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "Make":
		im.EXIFCameraMake = &valueStr
	case "Model":
		im.EXIFCameraModel = &valueStr
	case "Software":
		im.EXIFSoftware = &valueStr
	case "DateTimeOriginal":
		if t := parseExifDate(valueStr); t != nil {
			im.EXIFDateTimeOriginal = t
		}
	case "DateTimeDigitized":
		if t := parseExifDate(valueStr); t != nil {
			im.EXIFDateTimeDigitized = t
		}
	case "DateTime":
		if t := parseExifDate(valueStr); t != nil {
			im.EXIFDateTimeModified = t
		}
	case "Artist":
		im.EXIFArtist = &valueStr
	case "Copyright":
		im.EXIFCopyright = &valueStr
	case "ImageDescription":
		im.EXIFImageDescription = &valueStr
	case "UserComment":
		im.EXIFUserComment = &valueStr
	case "FNumber":
		if f, err := parseFloat(valueStr); err == nil {
			im.EXIFFNumber = &f
		}
	case "ExposureTime":
		im.EXIFExposureTime = &valueStr
	case "ISO":
		if iso, err := strconv.Atoi(valueStr); err == nil {
			im.EXIFISO = &iso
		}
	case "FocalLength":
		if f, err := parseFloat(valueStr); err == nil {
			im.EXIFFocalLength = &f
		}
	case "FocalLengthIn35mmFormat":
		if f, err := strconv.Atoi(valueStr); err == nil {
			im.EXIFFocalLength35mm = &f
		}
	case "ExposureMode":
		im.EXIFExposureMode = &valueStr
	case "WhiteBalance":
		im.EXIFWhiteBalance = &valueStr
	case "Flash":
		im.EXIFFlash = &valueStr
	case "MeteringMode":
		im.EXIFMeteringMode = &valueStr
	case "ExposureProgram":
		im.EXIFExposureProgram = &valueStr
	case "Orientation":
		if o, err := strconv.Atoi(valueStr); err == nil {
			im.Orientation = &o
		}
	}
}

// setIPTCField sets IPTC metadata fields.
func (e *ImageExtractor) setIPTCField(im *entity.ImageMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "ObjectName":
		im.IPTCObjectName = &valueStr
	case "Caption-Abstract", "Caption":
		im.IPTCCaption = &valueStr
	case "Keywords":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				im.IPTCKeywords = append(im.IPTCKeywords, fmt.Sprintf("%v", v))
			}
		} else {
			im.IPTCKeywords = append(im.IPTCKeywords, valueStr)
		}
	case "CopyrightNotice":
		im.IPTCCopyrightNotice = &valueStr
	case "By-line", "Byline":
		im.IPTCByline = &valueStr
	case "By-lineTitle":
		im.IPTCBylineTitle = &valueStr
	case "Headline":
		im.IPTCHeadline = &valueStr
	case "Contact":
		im.IPTCContact = &valueStr
	case "City":
		im.IPTCContactCity = &valueStr
	case "Country-PrimaryLocationName", "Country":
		im.IPTCContactCountry = &valueStr
	case "ContactEmail":
		im.IPTCContactEmail = &valueStr
	case "ContactPhone":
		im.IPTCContactPhone = &valueStr
	case "ContactWebsite":
		im.IPTCContactWebsite = &valueStr
	case "Source":
		im.IPTCSource = &valueStr
	case "UsageTerms":
		im.IPTCUsageTerms = &valueStr
	}
}

// setXMPField sets XMP metadata fields for images.
func (e *ImageExtractor) setXMPField(im *entity.ImageMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "Title", "dc:title":
		im.XMPTitle = &valueStr
	case "Description", "dc:description":
		im.XMPDescription = &valueStr
	case "Creator", "dc:creator":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				im.XMPCreator = append(im.XMPCreator, fmt.Sprintf("%v", v))
			}
		} else {
			im.XMPCreator = append(im.XMPCreator, valueStr)
		}
	case "Rights", "dc:rights":
		im.XMPRights = &valueStr
	case "Rating", "xmp:Rating":
		if rating, err := strconv.Atoi(valueStr); err == nil {
			im.XMPRating = &rating
		}
	case "Label", "xmp:Label":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				im.XMPLabel = append(im.XMPLabel, fmt.Sprintf("%v", v))
			}
		} else {
			im.XMPLabel = append(im.XMPLabel, valueStr)
		}
	case "Subject", "dc:subject":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				im.XMPSubject = append(im.XMPSubject, fmt.Sprintf("%v", v))
			}
		} else {
			im.XMPSubject = append(im.XMPSubject, valueStr)
		}
	}
}

// setGPSField sets GPS metadata fields.
func (e *ImageExtractor) setGPSField(im *entity.ImageMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "GPSLatitude":
		if lat, err := parseFloat(valueStr); err == nil {
			im.GPSLatitude = &lat
		}
	case "GPSLongitude":
		if lon, err := parseFloat(valueStr); err == nil {
			im.GPSLongitude = &lon
		}
	case "GPSAltitude":
		if alt, err := parseFloat(valueStr); err == nil {
			im.GPSAltitude = &alt
		}
	case "GPSLatitudeRef":
		im.GPSLatitudeRef = &valueStr
	case "GPSLongitudeRef":
		im.GPSLongitudeRef = &valueStr
	case "GPSAltitudeRef":
		im.GPSAltitudeRef = &valueStr
	}
}

// setStandardField sets standard image properties.
func (e *ImageExtractor) setStandardField(im *entity.ImageMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "ImageWidth", "Width":
		if w, err := strconv.Atoi(valueStr); err == nil {
			im.Width = w
		}
	case "ImageHeight", "Height":
		if h, err := strconv.Atoi(valueStr); err == nil {
			im.Height = h
		}
	case "BitsPerSample", "BitDepth":
		if depth, err := strconv.Atoi(valueStr); err == nil {
			im.ColorDepth = depth
		}
	case "ColorSpace":
		im.ColorSpace = &valueStr
	case "FileType", "MIMEType":
		im.Format = &valueStr
	}
}

// extractBasic extracts basic image properties without exiftool.
func (e *ImageExtractor) extractBasic(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) (*entity.EnhancedMetadata, error) {
	// Basic extraction would require image libraries
	// For now, return what we have
	return enhanced, nil
}

// parseFloat parses a float from a string.
func parseFloat(s string) (float64, error) {
	// Remove common units
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " mm")
	s = strings.TrimSuffix(s, " m")
	return strconv.ParseFloat(s, 64)
}

