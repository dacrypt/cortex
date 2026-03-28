package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// ForensicToolRunner provides access to various forensic tools for deep metadata extraction
type ForensicToolRunner struct {
	logger zerolog.Logger
}

// NewForensicToolRunner creates a new forensic tool runner
func NewForensicToolRunner(logger zerolog.Logger) *ForensicToolRunner {
	return &ForensicToolRunner{
		logger: logger.With().Str("component", "forensic_tools").Logger(),
	}
}

// ToolAvailability checks which forensic tools are available
type ToolAvailability struct {
	File       bool // file command
	Strings    bool // strings command
	Hexdump    bool // hexdump command
	PDFInfo    bool // pdfinfo
	PDFFonts   bool // pdffonts (poppler)
	PDFImages  bool // pdfimages (poppler)
	PDFTK      bool // pdftk
	MuTool     bool // mutool (MuPDF)
	ExifTool   bool // exiftool
	FFProbe    bool // ffprobe (for media files)
	Identify   bool // ImageMagick identify
	Md5Sum     bool // md5sum / md5
	Sha256Sum  bool // sha256sum / shasum -a 256
	Stat       bool // stat command
}

// CheckAvailability checks which tools are available in the system
func (ft *ForensicToolRunner) CheckAvailability() ToolAvailability {
	return ToolAvailability{
		File:       ft.hasTool("file"),
		Strings:    ft.hasTool("strings"),
		Hexdump:    ft.hasTool("hexdump"),
		PDFInfo:    ft.hasTool("pdfinfo"),
		PDFFonts:   ft.hasTool("pdffonts"),
		PDFImages:  ft.hasTool("pdfimages"),
		PDFTK:      ft.hasTool("pdftk"),
		MuTool:     ft.hasTool("mutool"),
		ExifTool:   ft.hasTool("exiftool"),
		FFProbe:    ft.hasTool("ffprobe"),
		Identify:   ft.hasTool("identify"),
		Md5Sum:     ft.hasTool("md5sum") || ft.hasTool("md5"),
		Sha256Sum:  ft.hasTool("sha256sum") || ft.hasTool("shasum"),
		Stat:       ft.hasTool("stat"),
	}
}

func (ft *ForensicToolRunner) hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RunFileCommand runs the 'file' command for MIME type detection
func (ft *ForensicToolRunner) RunFileCommand(ctx context.Context, filePath string) (map[string]string, error) {
	if !ft.hasTool("file") {
		return nil, fmt.Errorf("file command not available")
	}

	// Use -b for brief output, -i for MIME type, --mime-type for just MIME
	cmd := exec.CommandContext(ctx, "file", "-b", "--mime-type", "--mime-encoding", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("file command failed: %w", err)
	}

	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	// Parse output: "mime/type; charset=encoding" or just "mime/type"
	if len(lines) > 0 {
		parts := strings.Split(lines[0], ";")
		result["mime_type"] = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			encoding := strings.TrimSpace(parts[1])
			if strings.HasPrefix(encoding, "charset=") {
				result["charset"] = strings.TrimPrefix(encoding, "charset=")
			}
		}
	}

	// Also get detailed file type
	cmd = exec.CommandContext(ctx, "file", "-b", filePath)
	output, err = cmd.Output()
	if err == nil {
		result["file_type"] = strings.TrimSpace(string(output))
	}

	return result, nil
}

// RunStringsCommand extracts printable strings from binary files
func (ft *ForensicToolRunner) RunStringsCommand(ctx context.Context, filePath string, minLength int) ([]string, error) {
	if !ft.hasTool("strings") {
		return nil, fmt.Errorf("strings command not available")
	}

	// -n sets minimum string length, -a scans entire file
	cmd := exec.CommandContext(ctx, "strings", "-n", strconv.Itoa(minLength), "-a", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("strings command failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result, nil
}

// RunExifTool runs exiftool with comprehensive extraction
func (ft *ForensicToolRunner) RunExifTool(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !ft.hasTool("exiftool") {
		return nil, fmt.Errorf("exiftool not available")
	}

	// Use JSON output for structured data, -all to get everything
	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-all", "-G", "-a", "-u", "-U", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("exiftool failed: %w", err)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse exiftool JSON: %w", err)
	}

	if len(results) == 0 {
		return make(map[string]interface{}), nil
	}

	// Flatten the nested structure
	flattened := make(map[string]interface{})
	for key, value := range results[0] {
		flattened[key] = value
	}

	return flattened, nil
}

// RunPDFInfo runs pdfinfo for comprehensive PDF metadata
func (ft *ForensicToolRunner) RunPDFInfo(ctx context.Context, filePath string) (map[string]string, error) {
	if !ft.hasTool("pdfinfo") {
		return nil, fmt.Errorf("pdfinfo not available")
	}

	cmd := exec.CommandContext(ctx, "pdfinfo", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdfinfo failed: %w", err)
	}

	result := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		} else {
			// Some lines don't have colons, store as-is
			result["_raw_"+strconv.Itoa(len(result))] = line
		}
	}

	return result, nil
}

// RunFFProbe runs ffprobe for media file analysis
func (ft *ForensicToolRunner) RunFFProbe(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !ft.hasTool("ffprobe") {
		return nil, fmt.Errorf("ffprobe not available")
	}

	// Get comprehensive media information in JSON format
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		"-show_programs",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe JSON: %w", err)
	}

	return result, nil
}

// RunIdentify runs ImageMagick identify for image analysis
func (ft *ForensicToolRunner) RunIdentify(ctx context.Context, filePath string) (map[string]string, error) {
	if !ft.hasTool("identify") {
		return nil, fmt.Errorf("identify (ImageMagick) not available")
	}

	// Get verbose information in JSON format
	cmd := exec.CommandContext(ctx, "identify", "-verbose", "-format", "%[json]", filePath)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to regular verbose output
		cmd = exec.CommandContext(ctx, "identify", "-verbose", filePath)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("identify failed: %w", err)
		}
		return ft.parseIdentifyOutput(string(output)), nil
	}

	// Try to parse JSON output
	var jsonData map[string]interface{}
	if err := json.Unmarshal(output, &jsonData); err == nil {
		result := make(map[string]string)
		for k, v := range jsonData {
			result[k] = fmt.Sprintf("%v", v)
		}
		return result, nil
	}

	return ft.parseIdentifyOutput(string(output)), nil
}

func (ft *ForensicToolRunner) parseIdentifyOutput(output string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse "Key: value" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}

	return result
}

// RunStat runs stat command for file system metadata
func (ft *ForensicToolRunner) RunStat(ctx context.Context, filePath string) (map[string]string, error) {
	if !ft.hasTool("stat") {
		return nil, fmt.Errorf("stat command not available")
	}

	// Use format string for comprehensive output
	format := "%a|%A|%b|%B|%C|%d|%D|%f|%F|%g|%G|%h|%i|%m|%n|%N|%o|%s|%t|%T|%u|%U|%w|%W|%x|%X|%y|%Y|%z|%Z"
	cmd := exec.CommandContext(ctx, "stat", "-f", format, filePath)
	output, err := cmd.Output()
	if err != nil {
		// Try Linux format
		cmd = exec.CommandContext(ctx, "stat", "--format=%a|%A|%b|%B|%C|%d|%D|%f|%F|%g|%G|%h|%i|%m|%n|%N|%o|%s|%t|%T|%u|%U|%w|%W|%x|%X|%y|%Y|%z|%Z", filePath)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("stat failed: %w", err)
		}
	}

	result := make(map[string]string)
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	fieldNames := []string{"access_rights", "access_rights_human", "blocks", "block_size", "selinux_context",
		"device_number", "device_id", "raw_mode", "file_type", "group_id", "group_name",
		"hard_links", "inode", "mount_point", "file_name", "quoted_name", "optimal_io_size",
		"size", "major_device_type", "minor_device_type", "user_id", "user_name",
		"time_birth", "time_birth_epoch", "time_access", "time_access_epoch",
		"time_modify", "time_modify_epoch", "time_change", "time_change_epoch"}

	for i, field := range fields {
		if i < len(fieldNames) {
			result[fieldNames[i]] = strings.TrimSpace(field)
		}
	}

	return result, nil
}

// ExtractMagicBytes extracts magic bytes (file signature) from file
func (ft *ForensicToolRunner) ExtractMagicBytes(filePath string, length int) ([]byte, error) {
	if length <= 0 || length > 512 {
		length = 512 // Default to 512 bytes
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	magic := make([]byte, length)
	n, err := file.Read(magic)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("failed to read magic bytes: %w", err)
	}

	return magic[:n], nil
}

// ExtractFileHash calculates MD5 and SHA256 hashes
func (ft *ForensicToolRunner) ExtractFileHash(ctx context.Context, filePath string) (map[string]string, error) {
	result := make(map[string]string)

	// MD5
	if ft.hasTool("md5sum") {
		cmd := exec.CommandContext(ctx, "md5sum", filePath)
		output, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(string(output))
			if len(parts) > 0 {
				result["md5"] = parts[0]
			}
		}
	} else if ft.hasTool("md5") {
		cmd := exec.CommandContext(ctx, "md5", "-q", filePath)
		output, err := cmd.Output()
		if err == nil {
			result["md5"] = strings.TrimSpace(string(output))
		}
	}

	// SHA256
	if ft.hasTool("sha256sum") {
		cmd := exec.CommandContext(ctx, "sha256sum", filePath)
		output, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(string(output))
			if len(parts) > 0 {
				result["sha256"] = parts[0]
			}
		}
	} else if ft.hasTool("shasum") {
		cmd := exec.CommandContext(ctx, "shasum", "-a", "256", filePath)
		output, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(string(output))
			if len(parts) > 0 {
				result["sha256"] = parts[0]
			}
		}
	}

	return result, nil
}

// ExtractEmbeddedMetadata extracts all possible metadata using all available tools
func (ft *ForensicToolRunner) ExtractEmbeddedMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	availability := ft.CheckAvailability()

	// File command
	if availability.File {
		if fileInfo, err := ft.RunFileCommand(ctx, filePath); err == nil {
			result["file_command"] = fileInfo
		}
	}

	// Stat command
	if availability.Stat {
		if statInfo, err := ft.RunStat(ctx, filePath); err == nil {
			result["stat"] = statInfo
		}
	}

	// ExifTool (works for many file types)
	if availability.ExifTool {
		if exifData, err := ft.RunExifTool(ctx, filePath); err == nil {
			result["exiftool"] = exifData
		}
	}

	// File hashes
	if hashes, err := ft.ExtractFileHash(ctx, filePath); err == nil {
		result["hashes"] = hashes
	}

	// Magic bytes
	if magic, err := ft.ExtractMagicBytes(filePath, 512); err == nil {
		result["magic_bytes"] = fmt.Sprintf("%x", magic)
		result["magic_bytes_ascii"] = ft.extractPrintableMagic(magic)
	}

	// Strings extraction (for binary files)
	if availability.Strings {
		if strings, err := ft.RunStringsCommand(ctx, filePath, 4); err == nil {
			// Filter and extract interesting strings
			result["strings"] = ft.filterInterestingStrings(strings)
		}
	}

	return result, nil
}

func (ft *ForensicToolRunner) extractPrintableMagic(magic []byte) string {
	var builder strings.Builder
	for _, b := range magic {
		if b >= 32 && b < 127 {
			builder.WriteByte(b)
		} else {
			builder.WriteString(".")
		}
	}
	return builder.String()
}

func (ft *ForensicToolRunner) filterInterestingStrings(strs []string) map[string][]string {
	result := make(map[string][]string)
	
	// Patterns to look for
	patterns := map[string]*regexp.Regexp{
		"urls":        regexp.MustCompile(`https?://[^\s]+`),
		"emails":      regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		"ip_addresses": regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		"version_numbers": regexp.MustCompile(`\b\d+\.\d+(?:\.\d+)?\b`),
		"copyright":   regexp.MustCompile(`(?i)copyright|©|\(c\)`),
		"author":      regexp.MustCompile(`(?i)author|created by|written by`),
	}

	for _, str := range strs {
		for category, pattern := range patterns {
			if pattern.MatchString(str) {
				result[category] = append(result[category], str)
			}
		}
	}

	// Also keep some raw strings (limited)
	if len(strs) > 0 {
		maxRaw := 100
		if len(strs) < maxRaw {
			maxRaw = len(strs)
		}
		result["raw"] = strs[:maxRaw]
	}

	return result
}

// RunPDFFontsAnalysis extracts font information from PDF
func (ft *ForensicToolRunner) RunPDFFontsAnalysis(ctx context.Context, filePath string) ([]map[string]string, error) {
	if !ft.hasTool("pdffonts") {
		return nil, fmt.Errorf("pdffonts not available")
	}

	cmd := exec.CommandContext(ctx, "pdffonts", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdffonts failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 3 {
		return []map[string]string{}, nil // No fonts or header only
	}

	// Parse header: "name                                 type              encoding         emb sub uni object ID"
	// Skip first 2 lines (header and separator)
	var fonts []map[string]string
	for i := 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse fixed-width columns (pdffonts uses fixed width)
		font := make(map[string]string)
		if len(line) > 0 {
			// Name (first 40 chars)
			if len(line) > 40 {
				font["name"] = strings.TrimSpace(line[0:40])
				line = line[40:]
			}
			// Type (next 18 chars)
			if len(line) > 18 {
				font["type"] = strings.TrimSpace(line[0:18])
				line = line[18:]
			}
			// Encoding (next 18 chars)
			if len(line) > 18 {
				font["encoding"] = strings.TrimSpace(line[0:18])
				line = line[18:]
			}
			// Emb (next 4 chars)
			if len(line) > 4 {
				font["embedded"] = strings.TrimSpace(line[0:4])
				line = line[4:]
			}
			// Sub (next 4 chars)
			if len(line) > 4 {
				font["subset"] = strings.TrimSpace(line[0:4])
				line = line[4:]
			}
			// Uni (next 4 chars)
			if len(line) > 4 {
				font["unicode"] = strings.TrimSpace(line[0:4])
				line = line[4:]
			}
			// Object ID (rest)
			if len(line) > 0 {
				font["object_id"] = strings.TrimSpace(line)
			}
		}

		if len(font) > 0 {
			fonts = append(fonts, font)
		}
	}

	return fonts, nil
}

// RunPDFImagesAnalysis extracts image information from PDF
func (ft *ForensicToolRunner) RunPDFImagesAnalysis(ctx context.Context, filePath string) ([]map[string]string, error) {
	if !ft.hasTool("pdfimages") {
		return nil, fmt.Errorf("pdfimages not available")
	}

	// Use -list to get image information without extracting
	cmd := exec.CommandContext(ctx, "pdfimages", "-list", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdfimages failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 3 {
		return []map[string]string{}, nil // No images or header only
	}

	// Parse header: "page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio"
	// Skip first 2 lines (header and separator)
	var images []map[string]string
	for i := 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse fixed-width columns
		fields := strings.Fields(line)
		if len(fields) >= 13 {
			image := map[string]string{
				"page":      fields[0],
				"num":       fields[1],
				"type":      fields[2],
				"width":     fields[3],
				"height":    fields[4],
				"color":     fields[5],
				"comp":      fields[6],
				"bpc":       fields[7],
				"enc":       fields[8],
				"interp":    fields[9],
				"object_id": fields[10],
				"x_ppi":     fields[11],
				"y_ppi":     fields[12],
			}
			if len(fields) > 13 {
				image["size"] = fields[13]
			}
			if len(fields) > 14 {
				image["ratio"] = fields[14]
			}
			images = append(images, image)
		}
	}

	return images, nil
}

// RunPDFLinksAnalysis extracts link information from PDF using pdfinfo -dests
func (ft *ForensicToolRunner) RunPDFLinksAnalysis(ctx context.Context, filePath string) ([]map[string]string, error) {
	// pdfinfo doesn't have a direct links option, so we'll use strings + regex
	// or we can use mutool if available
	if ft.hasTool("mutool") {
		return ft.runPDFLinksWithMuTool(ctx, filePath)
	}

	// Fallback: extract from strings
	return ft.runPDFLinksFromStrings(ctx, filePath)
}

func (ft *ForensicToolRunner) runPDFLinksWithMuTool(ctx context.Context, filePath string) ([]map[string]string, error) {
	// mutool show file.pdf grep -a "/URI" or similar
	// This is a simplified version - mutool parsing is complex
	cmd := exec.CommandContext(ctx, "mutool", "show", filePath, "trailer/Root/Pages")
	_, err := cmd.Output()
	if err != nil {
		return ft.runPDFLinksFromStrings(ctx, filePath)
	}

	// For now, fallback to strings method
	return ft.runPDFLinksFromStrings(ctx, filePath)
}

func (ft *ForensicToolRunner) runPDFLinksFromStrings(ctx context.Context, filePath string) ([]map[string]string, error) {
	// Extract strings and look for URL patterns
	strings, err := ft.RunStringsCommand(ctx, filePath, 4)
	if err != nil {
		return nil, err
	}

	var links []map[string]string
	urlPattern := regexp.MustCompile(`https?://[^\s<>"']+`)
	seen := make(map[string]bool)

	for _, str := range strings {
		matches := urlPattern.FindAllString(str, -1)
		for _, url := range matches {
			if !seen[url] {
				seen[url] = true
				links = append(links, map[string]string{
					"url":    url,
					"type":   "external",
					"source": "strings",
				})
			}
		}
	}

	return links, nil
}

// RunPDFOutlineAnalysis extracts outline/bookmarks structure from PDF
func (ft *ForensicToolRunner) RunPDFOutlineAnalysis(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// Use pdftk or mutool to extract outline
	if ft.hasTool("pdftk") {
		return ft.runPDFOutlineWithPDFTK(ctx, filePath)
	}

	if ft.hasTool("mutool") {
		return ft.runPDFOutlineWithMuTool(ctx, filePath)
	}

	// Fallback: return empty structure
	return map[string]interface{}{
		"outline_available": false,
		"reason":            "no tools available (pdftk or mutool required)",
	}, nil
}

func (ft *ForensicToolRunner) runPDFOutlineWithPDFTK(ctx context.Context, filePath string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "pdftk", filePath, "dump_data_utf8")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftk failed: %w", err)
	}

	result := map[string]interface{}{
		"outline_available": true,
		"source":            "pdftk",
	}

	lines := strings.Split(string(output), "\n")
	var bookmarks []map[string]string
	currentBookmark := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BookmarkTitle:") {
			if len(currentBookmark) > 0 {
				bookmarks = append(bookmarks, currentBookmark)
			}
			currentBookmark = make(map[string]string)
			currentBookmark["title"] = strings.TrimPrefix(line, "BookmarkTitle:")
		} else if strings.HasPrefix(line, "BookmarkLevel:") {
			currentBookmark["level"] = strings.TrimPrefix(line, "BookmarkLevel:")
		} else if strings.HasPrefix(line, "BookmarkPageNumber:") {
			currentBookmark["page"] = strings.TrimPrefix(line, "BookmarkPageNumber:")
		}
	}

	if len(currentBookmark) > 0 {
		bookmarks = append(bookmarks, currentBookmark)
	}

	result["bookmarks"] = bookmarks
	result["bookmark_count"] = len(bookmarks)

	return result, nil
}

func (ft *ForensicToolRunner) runPDFOutlineWithMuTool(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// mutool show file.pdf outline
	cmd := exec.CommandContext(ctx, "mutool", "show", filePath, "outline")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mutool outline failed: %w", err)
	}

	result := map[string]interface{}{
		"outline_available": true,
		"source":            "mutool",
		"raw_output":        string(output),
	}

	// Parse mutool outline output (format varies)
	// For now, store raw output - can be parsed later if needed
	return result, nil
}

