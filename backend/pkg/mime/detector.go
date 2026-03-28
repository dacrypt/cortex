// Package mime provides MIME type detection utilities.
package mime

import (
	"os"
	"path/filepath"
	"strings"
)

// TypeInfo contains MIME type information.
type TypeInfo struct {
	MimeType string
	Category string // text, code, image, document, binary, archive, audio, video
	Encoding string
}

// DetectByExtension detects MIME type by file extension.
func DetectByExtension(filename string) *TypeInfo {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return &TypeInfo{
			MimeType: "application/octet-stream",
			Category: "binary",
		}
	}

	ext = strings.TrimPrefix(ext, ".")

	if info, ok := extensionMap[ext]; ok {
		return info
	}

	return &TypeInfo{
		MimeType: "application/octet-stream",
		Category: "binary",
	}
}

// DetectByMagic detects MIME type by file magic bytes.
func DetectByMagic(filePath string) (*TypeInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read first 512 bytes for magic detection
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	return DetectByBytes(buf), nil
}

// DetectByBytes detects MIME type from raw bytes.
func DetectByBytes(data []byte) *TypeInfo {
	if len(data) == 0 {
		return &TypeInfo{
			MimeType: "application/octet-stream",
			Category: "binary",
		}
	}

	// First, check if it's text - text files should be detected before generic binary signatures
	// This prevents CSV and other text files from being misidentified as binary formats
	if isText(data) {
		// For text files, we'll return text/plain here
		// The extension-based detection will refine this to text/csv, text/markdown, etc.
		return &TypeInfo{
			MimeType: "text/plain",
			Category: "text",
			Encoding: "utf-8",
		}
	}

	// Check magic signatures (only for binary files)
	// Order matters: more specific signatures first
	for _, sig := range magicSignatures {
		if len(data) >= len(sig.magic) {
			match := true
			for i, b := range sig.magic {
				if sig.mask != nil && i < len(sig.mask) {
					if data[i]&sig.mask[i] != b&sig.mask[i] {
						match = false
						break
					}
				} else if data[i] != b {
					match = false
					break
				}
			}
			if match {
				return sig.info
			}
		}
	}

	return &TypeInfo{
		MimeType: "application/octet-stream",
		Category: "binary",
	}
}

// isText checks if data appears to be text.
func isText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return true
	}

	// Check for null bytes (binary indicator)
	// Text files should have very few or no null bytes
	nullCount := 0
	for _, b := range data {
		if b == 0 {
			nullCount++
			// If we find null bytes in the first 32 bytes, it's likely binary
			// This prevents CSV files that start with nulls from being misidentified
			if nullCount > 0 && len(data) <= 32 {
				return false
			}
			// For larger files, allow up to 1% null bytes
			if nullCount > len(data)/100 {
				return false
			}
		}
	}

	// Check for control characters (except common ones: Tab, LF, CR)
	// Text files should have mostly printable characters or common whitespace
	nonPrintableCount := 0
	for _, b := range data {
		if b < 32 && b != 9 && b != 10 && b != 13 { // Tab (9), LF (10), CR (13) are OK
			nonPrintableCount++
			// If we find non-printable characters in the first 32 bytes, it's likely binary
			if nonPrintableCount > 0 && len(data) <= 32 {
				return false
			}
			// For larger files, allow some non-printable but not too many
			if nonPrintableCount > len(data)/50 { // 2% threshold
				return false
			}
		}
	}

	// Additional check: CSV files typically start with printable characters
	// If the first few bytes are all printable or whitespace, it's likely text
	if len(data) >= 4 {
		allPrintable := true
		for i := 0; i < 4 && i < len(data); i++ {
			b := data[i]
			if b != 0 && (b < 32 && b != 9 && b != 10 && b != 13) {
				allPrintable = false
				break
			}
		}
		if allPrintable && nullCount == 0 {
			return true
		}
	}

	// If we have mostly printable characters, it's text
	printableCount := 0
	for _, b := range data {
		if b >= 32 || b == 9 || b == 10 || b == 13 {
			printableCount++
		}
	}
	
	// If more than 80% of bytes are printable, it's likely text
	return float64(printableCount)/float64(len(data)) > 0.8
}

type magicSignature struct {
	magic []byte
	mask  []byte
	info  *TypeInfo
}

var magicSignatures = []magicSignature{
	// Images
	{[]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, nil, &TypeInfo{"image/png", "image", ""}},
	{[]byte{0xFF, 0xD8, 0xFF}, nil, &TypeInfo{"image/jpeg", "image", ""}},
	{[]byte{0x47, 0x49, 0x46, 0x38}, nil, &TypeInfo{"image/gif", "image", ""}},
	{[]byte{0x52, 0x49, 0x46, 0x46}, nil, &TypeInfo{"image/webp", "image", ""}}, // Partial, need to check for WEBP
	{[]byte{0x42, 0x4D}, nil, &TypeInfo{"image/bmp", "image", ""}},

	// Documents
	{[]byte{0x25, 0x50, 0x44, 0x46}, nil, &TypeInfo{"application/pdf", "document", ""}},
	{[]byte{0x50, 0x4B, 0x03, 0x04}, nil, &TypeInfo{"application/zip", "archive", ""}}, // Also DOCX, XLSX, etc.

	// Audio
	{[]byte{0x49, 0x44, 0x33}, nil, &TypeInfo{"audio/mpeg", "audio", ""}},             // MP3 with ID3
	{[]byte{0xFF, 0xFB}, nil, &TypeInfo{"audio/mpeg", "audio", ""}},                   // MP3 without ID3
	{[]byte{0x52, 0x49, 0x46, 0x46}, nil, &TypeInfo{"audio/wav", "audio", ""}},        // Partial
	{[]byte{0x4F, 0x67, 0x67, 0x53}, nil, &TypeInfo{"audio/ogg", "audio", ""}},
	{[]byte{0x66, 0x4C, 0x61, 0x43}, nil, &TypeInfo{"audio/flac", "audio", ""}},

	// Video
	// MP4 files start with a box structure: 4-byte size + 4-byte type
	// The first box is usually "ftyp" (file type) at offset 4
	// We check for "ftyp" (0x66 0x74 0x79 0x70) at offset 4 with various common sizes
	{[]byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}, nil, &TypeInfo{"video/mp4", "video", ""}}, // MP4: size=0x20, type=ftyp
	{[]byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70}, nil, &TypeInfo{"video/mp4", "video", ""}}, // MP4: size=0x18, type=ftyp
	{[]byte{0x00, 0x00, 0x00, 0x1C, 0x66, 0x74, 0x79, 0x70}, nil, &TypeInfo{"video/mp4", "video", ""}}, // MP4: size=0x1C, type=ftyp
	{[]byte{0x00, 0x00, 0x00, 0x14, 0x66, 0x74, 0x79, 0x70}, nil, &TypeInfo{"video/mp4", "video", ""}}, // MP4: size=0x14, type=ftyp
	// Check for "ftyp" at offset 4 with variable size (mask ignores size bytes 0-3, checks "ftyp" at 4-7)
	{[]byte{0x00, 0x00, 0x00, 0x00, 0x66, 0x74, 0x79, 0x70}, []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}, &TypeInfo{"video/mp4", "video", ""}}, // MP4: any size, type=ftyp
	{[]byte{0x1A, 0x45, 0xDF, 0xA3}, nil, &TypeInfo{"video/webm", "video", ""}},

	// Archives
	{[]byte{0x1F, 0x8B}, nil, &TypeInfo{"application/gzip", "archive", ""}},
	{[]byte{0x42, 0x5A, 0x68}, nil, &TypeInfo{"application/x-bzip2", "archive", ""}},
	{[]byte{0xFD, 0x37, 0x7A, 0x58, 0x5A}, nil, &TypeInfo{"application/x-xz", "archive", ""}},
	{[]byte{0x52, 0x61, 0x72, 0x21}, nil, &TypeInfo{"application/x-rar-compressed", "archive", ""}},
	{[]byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}, nil, &TypeInfo{"application/x-7z-compressed", "archive", ""}},

	// Executables
	{[]byte{0x7F, 0x45, 0x4C, 0x46}, nil, &TypeInfo{"application/x-executable", "binary", ""}}, // ELF
	{[]byte{0x4D, 0x5A}, nil, &TypeInfo{"application/x-msdownload", "binary", ""}},             // PE/EXE

	// Database
	{[]byte{0x53, 0x51, 0x4C, 0x69, 0x74, 0x65}, nil, &TypeInfo{"application/x-sqlite3", "binary", ""}},
}

var extensionMap = map[string]*TypeInfo{
	// Text
	"txt":  {"text/plain", "text", "utf-8"},
	"md":   {"text/markdown", "text", "utf-8"},
	"csv":  {"text/csv", "text", "utf-8"},
	"tsv":  {"text/tab-separated-values", "text", "utf-8"},
	"log":  {"text/plain", "text", "utf-8"},

	// Code
	"go":     {"text/x-go", "code", "utf-8"},
	"rs":     {"text/x-rust", "code", "utf-8"},
	"py":     {"text/x-python", "code", "utf-8"},
	"js":     {"text/javascript", "code", "utf-8"},
	"ts":     {"text/typescript", "code", "utf-8"},
	"jsx":    {"text/javascript", "code", "utf-8"},
	"tsx":    {"text/typescript", "code", "utf-8"},
	"java":   {"text/x-java", "code", "utf-8"},
	"kt":     {"text/x-kotlin", "code", "utf-8"},
	"swift":  {"text/x-swift", "code", "utf-8"},
	"c":      {"text/x-c", "code", "utf-8"},
	"cpp":    {"text/x-c++", "code", "utf-8"},
	"h":      {"text/x-c", "code", "utf-8"},
	"hpp":    {"text/x-c++", "code", "utf-8"},
	"cs":     {"text/x-csharp", "code", "utf-8"},
	"rb":     {"text/x-ruby", "code", "utf-8"},
	"php":    {"text/x-php", "code", "utf-8"},
	"sh":     {"text/x-shellscript", "code", "utf-8"},
	"bash":   {"text/x-shellscript", "code", "utf-8"},
	"zsh":    {"text/x-shellscript", "code", "utf-8"},
	"sql":    {"text/x-sql", "code", "utf-8"},
	"r":      {"text/x-r", "code", "utf-8"},
	"scala":  {"text/x-scala", "code", "utf-8"},
	"lua":    {"text/x-lua", "code", "utf-8"},
	"pl":     {"text/x-perl", "code", "utf-8"},
	"ex":     {"text/x-elixir", "code", "utf-8"},
	"exs":    {"text/x-elixir", "code", "utf-8"},
	"erl":    {"text/x-erlang", "code", "utf-8"},
	"hs":     {"text/x-haskell", "code", "utf-8"},
	"ml":     {"text/x-ocaml", "code", "utf-8"},
	"fs":     {"text/x-fsharp", "code", "utf-8"},
	"clj":    {"text/x-clojure", "code", "utf-8"},
	"jl":     {"text/x-julia", "code", "utf-8"},

	// Markup/Data
	"html":   {"text/html", "code", "utf-8"},
	"htm":    {"text/html", "code", "utf-8"},
	"xml":    {"text/xml", "code", "utf-8"},
	"xsl":    {"text/xml", "code", "utf-8"},
	"xslt":   {"text/xml", "code", "utf-8"},
	"css":    {"text/css", "code", "utf-8"},
	"scss":   {"text/x-scss", "code", "utf-8"},
	"sass":   {"text/x-sass", "code", "utf-8"},
	"less":   {"text/x-less", "code", "utf-8"},
	"json":   {"application/json", "code", "utf-8"},
	"yaml":   {"text/yaml", "code", "utf-8"},
	"yml":    {"text/yaml", "code", "utf-8"},
	"toml":   {"text/toml", "code", "utf-8"},
	"ini":    {"text/plain", "code", "utf-8"},
	"env":    {"text/plain", "code", "utf-8"},
	"proto":  {"text/x-protobuf", "code", "utf-8"},
	"graphql": {"text/x-graphql", "code", "utf-8"},

	// Images
	"png":    {"image/png", "image", ""},
	"jpg":    {"image/jpeg", "image", ""},
	"jpeg":   {"image/jpeg", "image", ""},
	"gif":    {"image/gif", "image", ""},
	"bmp":    {"image/bmp", "image", ""},
	"svg":    {"image/svg+xml", "image", ""},
	"webp":   {"image/webp", "image", ""},
	"ico":    {"image/x-icon", "image", ""},
	"tiff":   {"image/tiff", "image", ""},
	"tif":    {"image/tiff", "image", ""},

	// Documents
	"pdf":    {"application/pdf", "document", ""},
	"doc":    {"application/msword", "document", ""},
	"docx":   {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document", ""},
	"xls":    {"application/vnd.ms-excel", "document", ""},
	"xlsx":   {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "document", ""},
	"ppt":    {"application/vnd.ms-powerpoint", "document", ""},
	"pptx":   {"application/vnd.openxmlformats-officedocument.presentationml.presentation", "document", ""},
	"odt":    {"application/vnd.oasis.opendocument.text", "document", ""},
	"ods":    {"application/vnd.oasis.opendocument.spreadsheet", "document", ""},
	"odp":    {"application/vnd.oasis.opendocument.presentation", "document", ""},
	"rtf":    {"application/rtf", "document", ""},
	"tex":    {"text/x-tex", "document", "utf-8"},

	// Audio
	"mp3":    {"audio/mpeg", "audio", ""},
	"wav":    {"audio/wav", "audio", ""},
	"ogg":    {"audio/ogg", "audio", ""},
	"flac":   {"audio/flac", "audio", ""},
	"aac":    {"audio/aac", "audio", ""},
	"m4a":    {"audio/mp4", "audio", ""},
	"wma":    {"audio/x-ms-wma", "audio", ""},

	// Video
	"mp4":    {"video/mp4", "video", ""},
	"avi":    {"video/x-msvideo", "video", ""},
	"mkv":    {"video/x-matroska", "video", ""},
	"mov":    {"video/quicktime", "video", ""},
	"wmv":    {"video/x-ms-wmv", "video", ""},
	"webm":   {"video/webm", "video", ""},
	"flv":    {"video/x-flv", "video", ""},

	// Archives
	"zip":    {"application/zip", "archive", ""},
	"tar":    {"application/x-tar", "archive", ""},
	"gz":     {"application/gzip", "archive", ""},
	"bz2":    {"application/x-bzip2", "archive", ""},
	"xz":     {"application/x-xz", "archive", ""},
	"7z":     {"application/x-7z-compressed", "archive", ""},
	"rar":    {"application/x-rar-compressed", "archive", ""},

	// Fonts
	"ttf":    {"font/ttf", "binary", ""},
	"otf":    {"font/otf", "binary", ""},
	"woff":   {"font/woff", "binary", ""},
	"woff2":  {"font/woff2", "binary", ""},
	"eot":    {"application/vnd.ms-fontobject", "binary", ""},

	// Binary
	"exe":    {"application/x-msdownload", "binary", ""},
	"dll":    {"application/x-msdownload", "binary", ""},
	"so":     {"application/x-sharedlib", "binary", ""},
	"dylib":  {"application/x-mach-binary", "binary", ""},
	"a":      {"application/x-archive", "binary", ""},
	"o":      {"application/x-object", "binary", ""},
	"class":  {"application/java-vm", "binary", ""},
	"jar":    {"application/java-archive", "archive", ""},
	"wasm":   {"application/wasm", "binary", ""},
}

// GetCategory returns the category for a given MIME type.
func GetCategory(mimeType string) string {
	if strings.HasPrefix(mimeType, "text/") {
		if strings.Contains(mimeType, "x-") {
			return "code"
		}
		return "text"
	}
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "video"
	}
	if strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "tar") ||
		strings.Contains(mimeType, "compressed") || strings.Contains(mimeType, "archive") {
		return "archive"
	}
	if strings.Contains(mimeType, "document") || strings.Contains(mimeType, "pdf") ||
		strings.Contains(mimeType, "word") || strings.Contains(mimeType, "excel") ||
		strings.Contains(mimeType, "powerpoint") || strings.Contains(mimeType, "opendocument") {
		return "document"
	}
	return "binary"
}
