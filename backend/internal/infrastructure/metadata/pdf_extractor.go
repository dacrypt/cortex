package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/ledongthuc/pdf"
	"github.com/rs/zerolog"
)

// PDFExtractor extracts comprehensive metadata from PDF files.
type PDFExtractor struct {
	logger         zerolog.Logger
	forensicRunner *ForensicToolRunner
}

// NewPDFExtractor creates a new PDF metadata extractor.
func NewPDFExtractor(logger zerolog.Logger) *PDFExtractor {
	return &PDFExtractor{
		logger:         logger.With().Str("component", "pdf_metadata_extractor").Logger(),
		forensicRunner: NewForensicToolRunner(logger),
	}
}

// CanExtract returns true for PDF files.
func (e *PDFExtractor) CanExtract(extension string) bool {
	return strings.ToLower(extension) == ".pdf"
}

// Extract extracts all possible metadata from a PDF file.
func (e *PDFExtractor) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	if !e.CanExtract(entry.Extension) {
		return nil, nil
	}

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
		// Store forensic data in custom properties
		e.storeForensicData(enhanced, forensicData)
	}

	// Try multiple methods to extract metadata
	// 1. Try pdfinfo (most comprehensive)
	if pdfInfo, err := e.forensicRunner.RunPDFInfo(ctx, entry.AbsolutePath); err == nil {
		e.logger.Debug().Str("file", entry.RelativePath).Msg("Extracted PDF metadata using pdfinfo")
		e.parsePDFInfoData(enhanced, pdfInfo)
	} else {
		// Fallback to old method
		if err := e.extractWithPDFInfo(ctx, entry.AbsolutePath, enhanced); err == nil {
			e.logger.Debug().Str("file", entry.RelativePath).Msg("Extracted PDF metadata using pdfinfo (fallback)")
		}
	}

	// 2. Try using the pdf library for basic info
	if err := e.extractWithPDFLibrary(ctx, entry.AbsolutePath, enhanced); err != nil {
		e.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract PDF metadata with library")
	}

	// 3. Try exiftool (if not already extracted by forensic runner)
	if exifData, ok := forensicData["exiftool"].(map[string]interface{}); ok {
		e.parseExifToolData(enhanced, exifData)
	} else {
		// Fallback to old method
		if err := e.extractWithExifTool(ctx, entry.AbsolutePath, enhanced); err == nil {
			e.logger.Debug().Str("file", entry.RelativePath).Msg("Extracted PDF metadata using exiftool (fallback)")
		}
	}

	// 4. Extract strings from PDF for embedded metadata
	if strings, err := e.forensicRunner.RunStringsCommand(ctx, entry.AbsolutePath, 4); err == nil {
		e.extractEmbeddedStrings(enhanced, strings)
	}

	// 5. Extract font information
	if fonts, err := e.forensicRunner.RunPDFFontsAnalysis(ctx, entry.AbsolutePath); err == nil {
		e.logger.Debug().Int("font_count", len(fonts)).Str("file", entry.RelativePath).Msg("Extracted font information")
		e.extractFontInformation(enhanced, fonts)
	} else {
		e.logger.Debug().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract font information")
	}

	// 6. Extract image information
	if images, err := e.forensicRunner.RunPDFImagesAnalysis(ctx, entry.AbsolutePath); err == nil {
		e.logger.Debug().Int("image_count", len(images)).Str("file", entry.RelativePath).Msg("Extracted image information")
		e.extractImageInformation(enhanced, images)
	} else {
		e.logger.Debug().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract image information")
	}

	// 7. Extract link information
	if links, err := e.forensicRunner.RunPDFLinksAnalysis(ctx, entry.AbsolutePath); err == nil {
		e.logger.Debug().Int("link_count", len(links)).Str("file", entry.RelativePath).Msg("Extracted link information")
		e.extractLinkInformation(enhanced, links)
	} else {
		e.logger.Debug().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract link information")
	}

	// 8. Extract outline/bookmarks structure
	if outline, err := e.forensicRunner.RunPDFOutlineAnalysis(ctx, entry.AbsolutePath); err == nil {
		e.logger.Debug().Str("file", entry.RelativePath).Msg("Extracted outline information")
		e.extractOutlineInformation(enhanced, outline)
	} else {
		e.logger.Debug().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract outline information")
	}

	return enhanced, nil
}

// extractWithPDFInfo uses pdfinfo command to extract metadata.
func (e *PDFExtractor) extractWithPDFInfo(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		return fmt.Errorf("pdfinfo not found")
	}

	cmd := exec.CommandContext(ctx, "pdfinfo", filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("pdfinfo failed: %w", err)
	}

	dm := enhanced.DocumentMetrics
	if dm == nil {
		dm = &entity.DocumentMetrics{CustomProperties: make(map[string]string)}
		enhanced.DocumentMetrics = dm
	}

	// Parse pdfinfo output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Title":
			dm.Title = &value
		case "Subject":
			dm.Subject = &value
		case "Author":
			dm.Author = &value
		case "Creator":
			dm.Creator = &value
		case "Producer":
			dm.Producer = &value
		case "CreationDate":
			if t := parsePDFDate(value); t != nil {
				dm.CreatedDate = t
			}
		case "ModDate":
			if t := parsePDFDate(value); t != nil {
				dm.ModifiedDate = t
			}
		case "Keywords":
			if value != "" {
				dm.Keywords = strings.Split(value, ",")
				for i, k := range dm.Keywords {
					dm.Keywords[i] = strings.TrimSpace(k)
				}
			}
		case "Pages":
			if pages, err := strconv.Atoi(value); err == nil {
				dm.PageCount = pages
			}
		case "PDF version":
			dm.PDFVersion = &value
		case "Encrypted":
			encrypted := strings.Contains(strings.ToLower(value), "yes")
			dm.PDFEncrypted = &encrypted
		case "Tagged":
			tagged := strings.Contains(strings.ToLower(value), "yes")
			dm.PDFTagged = &tagged
		case "Page size":
			// Store in custom properties
			dm.CustomProperties["PageSize"] = value
		default:
			// Store unknown fields in custom properties
			dm.CustomProperties[key] = value
		}
	}

	return nil
}

// extractWithPDFLibrary uses the pdf library to extract basic metadata.
func (e *PDFExtractor) extractWithPDFLibrary(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	dm := enhanced.DocumentMetrics
	if dm == nil {
		dm = &entity.DocumentMetrics{CustomProperties: make(map[string]string)}
		enhanced.DocumentMetrics = dm
	}

	// Get page count (only if not already set by pdfinfo)
	if dm.PageCount == 0 {
		dm.PageCount = reader.NumPage()
	}

	// Note: The pdf library doesn't expose metadata directly, so we rely on pdfinfo/exiftool
	// for comprehensive extraction. This function mainly provides page count as fallback.

	return nil
}

// extractWithExifTool uses exiftool to extract XMP and other metadata.
func (e *PDFExtractor) extractWithExifTool(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found")
	}

	dm := enhanced.DocumentMetrics
	if dm == nil {
		dm = &entity.DocumentMetrics{CustomProperties: make(map[string]string)}
		enhanced.DocumentMetrics = dm
	}

	// Try JSON output first (more reliable)
	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-all", filePath)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		if err := e.parseExifToolJSON(output, dm); err == nil {
			return nil // Successfully parsed JSON
		}
		e.logger.Debug().Err(err).Msg("Failed to parse exiftool JSON, falling back to text")
	}

	// Fallback to text output
	cmd = exec.CommandContext(ctx, "exiftool", filePath)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("exiftool text output failed: %w", err)
	}

	return e.parseExifToolText(output, dm)
}

// parseExifToolJSON parses exiftool JSON output.
func (e *PDFExtractor) parseExifToolJSON(jsonData []byte, dm *entity.DocumentMetrics) error {
	var results []map[string]interface{}
	if err := json.Unmarshal(jsonData, &results); err != nil {
		return fmt.Errorf("failed to unmarshal exiftool JSON: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no data in exiftool output")
	}

	data := results[0] // exiftool returns array with one object

	for key, value := range data {
		if value == nil {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		// Parse XMP metadata
		if strings.HasPrefix(key, "XMP:") {
			xmpKey := strings.TrimPrefix(key, "XMP:")
			e.setXMPField(dm, xmpKey, value, valueStr)
		} else if strings.HasPrefix(key, "PDF:") {
			// PDF-specific fields
			pdfKey := strings.TrimPrefix(key, "PDF:")
			e.setPDFField(dm, pdfKey, value, valueStr)
		} else {
			// Standard fields
			e.setStandardField(dm, key, value, valueStr)
		}
	}

	return nil
}

// parseExifToolText parses exiftool text output (fallback).
func (e *PDFExtractor) parseExifToolText(output []byte, dm *entity.DocumentMetrics) error {
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

		if strings.HasPrefix(key, "XMP:") {
			xmpKey := strings.TrimPrefix(key, "XMP:")
			e.setXMPField(dm, xmpKey, value, value)
		} else {
			dm.CustomProperties[key] = value
		}
	}

	return nil
}

// setXMPField sets XMP metadata fields.
func (e *PDFExtractor) setXMPField(dm *entity.DocumentMetrics, key string, value interface{}, valueStr string) {
	switch key {
	case "Title", "dc:title":
		dm.XMPTitle = &valueStr
	case "Description", "dc:description":
		dm.XMPDescription = &valueStr
	case "Creator", "dc:creator":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPCreator = append(dm.XMPCreator, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPCreator = append(dm.XMPCreator, valueStr)
		}
	case "Contributor", "dc:contributor":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPContributor = append(dm.XMPContributor, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPContributor = append(dm.XMPContributor, valueStr)
		}
	case "Rights", "dc:rights":
		dm.XMPRights = &valueStr
	case "RightsOwner", "xmpRights:Owner":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPRightsOwner = append(dm.XMPRightsOwner, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPRightsOwner = append(dm.XMPRightsOwner, valueStr)
		}
	case "Copyright":
		dm.XMPCopyright = &valueStr
	case "CopyrightURL":
		dm.XMPCopyrightURL = &valueStr
	case "Identifier", "dc:identifier":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPIdentifier = append(dm.XMPIdentifier, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPIdentifier = append(dm.XMPIdentifier, valueStr)
		}
	case "Language", "dc:language":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPLanguage = append(dm.XMPLanguage, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPLanguage = append(dm.XMPLanguage, valueStr)
		}
	case "Rating", "xmp:Rating":
		if rating, err := strconv.Atoi(valueStr); err == nil {
			dm.XMPRating = &rating
		}
	case "MetadataDate", "xmp:MetadataDate":
		if t := parseExifDate(valueStr); t != nil {
			dm.XMPMetadataDate = t
		}
	case "ModifyDate", "xmp:ModifyDate":
		if t := parseExifDate(valueStr); t != nil {
			dm.XMPModifyDate = t
		}
	case "CreateDate", "xmp:CreateDate":
		if t := parseExifDate(valueStr); t != nil {
			dm.XMPCreateDate = t
		}
	case "Nickname", "xmp:Nickname":
		dm.XMPNickname = &valueStr
	case "Label", "xmp:Label":
		if arr, ok := value.([]interface{}); ok {
			for _, v := range arr {
				dm.XMPLabel = append(dm.XMPLabel, fmt.Sprintf("%v", v))
			}
		} else {
			dm.XMPLabel = append(dm.XMPLabel, valueStr)
		}
	case "Marked", "xmpRights:Marked":
		marked := strings.ToLower(valueStr) == "true" || valueStr == "1"
		dm.XMPMarked = &marked
	case "UsageTerms", "xmpRights:UsageTerms":
		dm.XMPUsageTerms = &valueStr
	case "WebStatement", "xmpRights:WebStatement":
		dm.XMPWebStatement = &valueStr
	default:
		dm.CustomProperties["XMP:"+key] = valueStr
	}
}

// setPDFField sets PDF-specific fields.
func (e *PDFExtractor) setPDFField(dm *entity.DocumentMetrics, key string, value interface{}, valueStr string) {
	switch key {
	case "Version":
		dm.PDFVersion = &valueStr
	case "Encrypted":
		encrypted := strings.Contains(strings.ToLower(valueStr), "yes") || valueStr == "true"
		dm.PDFEncrypted = &encrypted
	case "Linearized":
		linearized := strings.Contains(strings.ToLower(valueStr), "yes") || valueStr == "true"
		dm.PDFLinearized = &linearized
	case "Tagged":
		tagged := strings.Contains(strings.ToLower(valueStr), "yes") || valueStr == "true"
		dm.PDFTagged = &tagged
	case "PageLayout":
		dm.PDFPageLayout = &valueStr
	case "PageMode":
		dm.PDFPageMode = &valueStr
	default:
		dm.CustomProperties["PDF:"+key] = valueStr
	}
}

// setStandardField sets standard PDF Info Dictionary fields.
func (e *PDFExtractor) setStandardField(dm *entity.DocumentMetrics, key string, value interface{}, valueStr string) {
	switch key {
	case "Title":
		if dm.Title == nil {
			dm.Title = &valueStr
		}
	case "Author", "Creator":
		if dm.Author == nil {
			dm.Author = &valueStr
		}
		if key == "Creator" && dm.Creator == nil {
			dm.Creator = &valueStr
		}
	case "Subject":
		if dm.Subject == nil {
			dm.Subject = &valueStr
		}
	case "Keywords":
		if len(dm.Keywords) == 0 {
			keywords := strings.Split(valueStr, ",")
			for i, k := range keywords {
				keywords[i] = strings.TrimSpace(k)
			}
			dm.Keywords = keywords
		}
	case "Producer":
		if dm.Producer == nil {
			dm.Producer = &valueStr
		}
	case "CreateDate", "CreationDate":
		if dm.CreatedDate == nil {
			if t := parseExifDate(valueStr); t != nil {
				dm.CreatedDate = t
			}
		}
	case "ModifyDate", "ModDate":
		if dm.ModifiedDate == nil {
			if t := parseExifDate(valueStr); t != nil {
				dm.ModifiedDate = t
			}
		}
	case "Pages", "PageCount":
		if dm.PageCount == 0 {
			if pages, err := strconv.Atoi(valueStr); err == nil {
				dm.PageCount = pages
			}
		}
	default:
		dm.CustomProperties[key] = valueStr
	}
}

// parseExifDate parses various date formats from exiftool.
func parseExifDate(dateStr string) *time.Time {
	// Try PDF date format first
	if t := parsePDFDate(dateStr); t != nil {
		return t
	}

	// Try common exiftool date formats
	formats := []string{
		"2006:01:02 15:04:05",
		"2006:01:02 15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}

	return nil
}

// parsePDFDate parses PDF date format: "D:YYYYMMDDHHmmSSOHH'mm'"
// Example: "D:20240101120000-05'00'"
func parsePDFDate(dateStr string) *time.Time {
	// Remove "D:" prefix if present
	dateStr = strings.TrimPrefix(dateStr, "D:")

	// PDF date format: YYYYMMDDHHmmSSOHH'mm'
	// Try to parse it
	re := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})([+-])(\d{2})'(\d{2})'?$`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) == 10 {
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		hour, _ := strconv.Atoi(matches[4])
		min, _ := strconv.Atoi(matches[5])
		sec, _ := strconv.Atoi(matches[6])
		tzSign := matches[7]
		tzHour, _ := strconv.Atoi(matches[8])
		tzMin, _ := strconv.Atoi(matches[9])

		loc := time.FixedZone("", tzHour*3600+tzMin*60)
		if tzSign == "-" {
			loc = time.FixedZone("", -(tzHour*3600+tzMin*60))
		}

		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, loc)
		return &t
	}

	// Try simpler formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}

	return nil
}

// parsePDFInfoDate parses pdfinfo date format: "Sun Apr 27 16:24:15 2014 -05"
// This is different from PDF date format
func parsePDFInfoDate(dateStr string) *time.Time {
	// Try pdfinfo format: "Sun Apr 27 16:24:15 2014 -05"
	// Format: "Mon Jan 2 15:04:05 2006 -0700"
	formats := []string{
		"Mon Jan 2 15:04:05 2006 -0700",
		"Mon Jan 2 15:04:05 2006 -07",
		"Mon Jan 2 15:04:05 2006 MST",
		"Mon Jan 2 15:04:05 2006",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -07",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}

	return nil
}

// storeForensicData stores forensic metadata in custom properties
func (e *PDFExtractor) storeForensicData(enhanced *entity.EnhancedMetadata, data map[string]interface{}) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	// Store all forensic data in custom properties
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			if jsonData, err := json.Marshal(v); err == nil {
				dm.CustomProperties["forensic_"+key] = string(jsonData)
			}
		case map[string]string:
			if jsonData, err := json.Marshal(v); err == nil {
				dm.CustomProperties["forensic_"+key] = string(jsonData)
			}
		case []string:
			dm.CustomProperties["forensic_"+key] = strings.Join(v, " | ")
		case string:
			dm.CustomProperties["forensic_"+key] = v
		default:
			dm.CustomProperties["forensic_"+key] = fmt.Sprintf("%v", v)
		}
	}
}

// parsePDFInfoData parses pdfinfo output into DocumentMetrics
func (e *PDFExtractor) parsePDFInfoData(enhanced *entity.EnhancedMetadata, pdfInfo map[string]string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	// Parse standard fields
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
	// Parse dates (pdfinfo uses a different format than PDF date strings)
	if creationDate, ok := pdfInfo["CreationDate"]; ok && creationDate != "" {
		if t := parsePDFInfoDate(creationDate); t != nil {
			dm.CreatedDate = t
		} else if t := parsePDFDate(creationDate); t != nil {
			dm.CreatedDate = t
		}
	}
	if modDate, ok := pdfInfo["ModDate"]; ok && modDate != "" {
		if t := parsePDFInfoDate(modDate); t != nil {
			dm.ModifiedDate = t
		} else if t := parsePDFDate(modDate); t != nil {
			dm.ModifiedDate = t
		}
	}
	// Parse keywords
	if keywords, ok := pdfInfo["Keywords"]; ok && keywords != "" {
		keywordsList := strings.Split(keywords, ",")
		for i, k := range keywordsList {
			keywordsList[i] = strings.TrimSpace(k)
		}
		dm.Keywords = keywordsList
	}
	// Parse encrypted/tagged flags
	if encrypted, ok := pdfInfo["Encrypted"]; ok {
		encryptedBool := strings.Contains(strings.ToLower(encrypted), "yes")
		dm.PDFEncrypted = &encryptedBool
	}
	if tagged, ok := pdfInfo["Tagged"]; ok {
		taggedBool := strings.Contains(strings.ToLower(tagged), "yes")
		dm.PDFTagged = &taggedBool
	}

	// Store all other fields in custom properties
	for key, value := range pdfInfo {
		if !e.isStandardPDFField(key) {
			dm.CustomProperties["pdfinfo_"+key] = value
		}
	}
}

func (e *PDFExtractor) isStandardPDFField(key string) bool {
	standardFields := []string{"Title", "Author", "Subject", "Creator", "Producer", "Pages", "PDF version", "CreationDate", "ModDate", "Keywords"}
	for _, field := range standardFields {
		if key == field {
			return true
		}
	}
	return false
}

// parseExifToolData parses exiftool output into DocumentMetrics
func (e *PDFExtractor) parseExifToolData(enhanced *entity.EnhancedMetadata, exifData map[string]interface{}) {
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

// extractEmbeddedStrings extracts metadata from embedded strings
func (e *PDFExtractor) extractEmbeddedStrings(enhanced *entity.EnhancedMetadata, strList []string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	// Look for interesting patterns in strings
	urlPattern := regexp.MustCompile(`https?://[^\s]+`)
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	versionPattern := regexp.MustCompile(`\b\d+\.\d+(?:\.\d+)?\b`)

	var urls, emails, versions []string

	for _, str := range strList {
		if urlPattern.MatchString(str) {
			urls = append(urls, str)
		}
		if emailPattern.MatchString(str) {
			emails = append(emails, str)
		}
		if versionPattern.MatchString(str) {
			versions = append(versions, str)
		}
	}

	if len(urls) > 0 {
		dm.CustomProperties["embedded_urls"] = strings.Join(urls, " | ")
	}
	if len(emails) > 0 {
		dm.CustomProperties["embedded_emails"] = strings.Join(emails, " | ")
	}
	if len(versions) > 0 {
		dm.CustomProperties["embedded_versions"] = strings.Join(versions, " | ")
	}

	// Store first 100 strings as raw data
	if len(strList) > 0 {
		maxStrings := 100
		if len(strList) < maxStrings {
			maxStrings = len(strList)
		}
		dm.CustomProperties["embedded_strings_count"] = strconv.Itoa(len(strList))
		dm.CustomProperties["embedded_strings_sample"] = strings.Join(strList[:maxStrings], " | ")
	}
}

// extractFontInformation stores font information in DocumentMetrics
func (e *PDFExtractor) extractFontInformation(enhanced *entity.EnhancedMetadata, fonts []map[string]string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	if len(fonts) == 0 {
		return
	}

	// Store font count
	dm.CustomProperties["font_count"] = strconv.Itoa(len(fonts))

	// Extract font names
	var fontNames []string
	var embeddedCount, subsetCount, unicodeCount int

	for _, font := range fonts {
		if name, ok := font["name"]; ok && name != "" {
			fontNames = append(fontNames, name)
		}
		if embedded, ok := font["embedded"]; ok && embedded == "yes" {
			embeddedCount++
		}
		if subset, ok := font["subset"]; ok && subset == "yes" {
			subsetCount++
		}
		if unicode, ok := font["unicode"]; ok && unicode == "yes" {
			unicodeCount++
		}
	}

	if len(fontNames) > 0 {
		// Store all font names (limit to first 50 to avoid huge strings)
		maxFonts := 50
		if len(fontNames) < maxFonts {
			maxFonts = len(fontNames)
		}
		dm.Fonts = fontNames[:maxFonts]
		dm.CustomProperties["font_names"] = strings.Join(fontNames[:maxFonts], ", ")
		if len(fontNames) > maxFonts {
			dm.CustomProperties["font_names_truncated"] = "true"
		}
	}

	// Store statistics
	dm.CustomProperties["fonts_embedded"] = strconv.Itoa(embeddedCount)
	dm.CustomProperties["fonts_subset"] = strconv.Itoa(subsetCount)
	dm.CustomProperties["fonts_unicode"] = strconv.Itoa(unicodeCount)

	// Store detailed font information as JSON (first 20 fonts)
	if len(fonts) > 0 {
		maxDetail := 20
		if len(fonts) < maxDetail {
			maxDetail = len(fonts)
		}
		if jsonData, err := json.Marshal(fonts[:maxDetail]); err == nil {
			dm.CustomProperties["fonts_detail"] = string(jsonData)
		}
	}
}

// extractImageInformation stores image information in DocumentMetrics
func (e *PDFExtractor) extractImageInformation(enhanced *entity.EnhancedMetadata, images []map[string]string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	if len(images) == 0 {
		return
	}

	// Store image count
	imageCount := len(images)
	dm.ImageCount = &imageCount
	dm.CustomProperties["image_count"] = strconv.Itoa(imageCount)

	// Extract image types and calculate statistics
	imageTypes := make(map[string]int)
	var totalWidth, totalHeight int
	var widthCount, heightCount int

	for _, img := range images {
		if imgType, ok := img["type"]; ok && imgType != "" {
			imageTypes[imgType]++
		}
		if width, ok := img["width"]; ok && width != "" {
			if w, err := strconv.Atoi(width); err == nil {
				totalWidth += w
				widthCount++
			}
		}
		if height, ok := img["height"]; ok && height != "" {
			if h, err := strconv.Atoi(height); err == nil {
				totalHeight += h
				heightCount++
			}
		}
	}

	// Store image type distribution
	if len(imageTypes) > 0 {
		var typeList []string
		for imgType, count := range imageTypes {
			typeList = append(typeList, fmt.Sprintf("%s:%d", imgType, count))
		}
		dm.CustomProperties["image_types"] = strings.Join(typeList, ", ")
	}

	// Store average dimensions
	if widthCount > 0 {
		avgWidth := totalWidth / widthCount
		dm.CustomProperties["image_avg_width"] = strconv.Itoa(avgWidth)
	}
	if heightCount > 0 {
		avgHeight := totalHeight / heightCount
		dm.CustomProperties["image_avg_height"] = strconv.Itoa(avgHeight)
	}

	// Store detailed image information as JSON (first 20 images)
	if len(images) > 0 {
		maxDetail := 20
		if len(images) < maxDetail {
			maxDetail = len(images)
		}
		if jsonData, err := json.Marshal(images[:maxDetail]); err == nil {
			dm.CustomProperties["images_detail"] = string(jsonData)
		}
	}
}

// extractLinkInformation stores link information in DocumentMetrics
func (e *PDFExtractor) extractLinkInformation(enhanced *entity.EnhancedMetadata, links []map[string]string) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	if len(links) == 0 {
		return
	}

	// Extract URLs
	var urls []string
	externalCount := 0
	internalCount := 0

	for _, link := range links {
		if url, ok := link["url"]; ok && url != "" {
			urls = append(urls, url)
		}
		if linkType, ok := link["type"]; ok {
			if linkType == "external" {
				externalCount++
			} else {
				internalCount++
			}
		}
	}

	// Store in Hyperlinks field
	if len(urls) > 0 {
		dm.Hyperlinks = urls
		dm.CustomProperties["link_count"] = strconv.Itoa(len(urls))
		dm.CustomProperties["link_external_count"] = strconv.Itoa(externalCount)
		dm.CustomProperties["link_internal_count"] = strconv.Itoa(internalCount)
	}
}

// extractOutlineInformation stores outline/bookmarks information in DocumentMetrics
func (e *PDFExtractor) extractOutlineInformation(enhanced *entity.EnhancedMetadata, outline map[string]interface{}) {
	if enhanced.DocumentMetrics == nil {
		enhanced.DocumentMetrics = &entity.DocumentMetrics{
			CustomProperties: make(map[string]string),
		}
	}

	dm := enhanced.DocumentMetrics

	if outline == nil {
		return
	}

	// Store outline availability
	if available, ok := outline["outline_available"].(bool); ok {
		dm.CustomProperties["outline_available"] = strconv.FormatBool(available)
	}

	// Store bookmark count
	if count, ok := outline["bookmark_count"].(int); ok {
		dm.CustomProperties["bookmark_count"] = strconv.Itoa(count)
	}

	// Store bookmarks as JSON
	if bookmarks, ok := outline["bookmarks"].([]map[string]string); ok && len(bookmarks) > 0 {
		if jsonData, err := json.Marshal(bookmarks); err == nil {
			dm.CustomProperties["outline_bookmarks"] = string(jsonData)
		}
	}

	// Store source
	if source, ok := outline["source"].(string); ok {
		dm.CustomProperties["outline_source"] = source
	}
}
