// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/gabriel-vasile/mimetype"
)

// MimeStage detects MIME types and categorizes files.
type MimeStage struct{}

// NewMimeStage creates a new MIME detection stage.
func NewMimeStage() *MimeStage {
	return &MimeStage{}
}

// Name returns the stage name.
func (s *MimeStage) Name() string {
	return "mime"
}

// Process detects MIME type and categorizes the file.
func (s *MimeStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}

	// Use mimetype library for accurate MIME type detection
	// This library uses magic bytes (file signatures) to detect the actual file type
	// It automatically falls back to extension-based detection if magic bytes don't match
	var detectedMime *mimetype.MIME

	// Try to detect from file directly (reads magic bytes, automatically handles extension fallback)
	if entry.FileSize > 0 {
		var err error
		detectedMime, err = mimetype.DetectFile(entry.AbsolutePath)
		if err != nil {
			// If file detection fails, try reading first bytes manually
			file, fileErr := os.Open(entry.AbsolutePath)
			if fileErr == nil {
				defer file.Close()
				header := make([]byte, 3072) // mimetype library recommends 3072 bytes for accurate detection
				n, _ := file.Read(header)
				if n > 0 {
					detectedMime = mimetype.Detect(header[:n])
				}
			}
		}
	}

	// If still no result, use extension as last resort
	if detectedMime == nil {
		// Read file to detect by content + extension
		if entry.FileSize > 0 {
			file, err := os.Open(entry.AbsolutePath)
			if err == nil {
				defer file.Close()
				header := make([]byte, 512) // Minimum for detection
				n, _ := file.Read(header)
				if n > 0 {
					detectedMime = mimetype.Detect(header[:n])
				}
			}
		}
		
		// If still nil, create default
		if detectedMime == nil {
			detectedMime = mimetype.Detect(nil)
		}
	}

	// Convert mimetype library result to our internal structure
	mimeTypeStr := detectedMime.String()
	category := s.mimeTypeToCategory(mimeTypeStr)
	encoding := s.detectEncodingFromMimeType(mimeTypeStr)

	// Store MIME info
	entry.Enhanced.MimeType = &entity.MimeTypeInfo{
		MimeType: mimeTypeStr,
		Category: category,
		Encoding: encoding,
	}
	
	// Detect content encoding for text files
	if category == "text" || category == "code" {
		detectedEncoding := s.detectContentEncoding(entry.AbsolutePath)
		if detectedEncoding != "" {
			entry.Enhanced.ContentEncoding = &detectedEncoding
		}
	}
	
	entry.Enhanced.IndexedState.Mime = true

	return nil
}

// detectContentEncoding attempts to detect the character encoding of a text file.
func (s *MimeStage) detectContentEncoding(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read first 8KB to analyze encoding
	buffer := make([]byte, 8192)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return ""
	}
	
	data := buffer[:n]

	// Check for BOM (Byte Order Mark) which indicates encoding
	if len(data) >= 3 {
		// UTF-8 BOM: EF BB BF
		if data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
			return "UTF-8"
		}
		// UTF-16 LE BOM: FF FE
		if data[0] == 0xFF && data[1] == 0xFE {
			return "UTF-16LE"
		}
		// UTF-16 BE BOM: FE FF
		if data[0] == 0xFE && data[1] == 0xFF {
			return "UTF-16BE"
		}
	}

	// Check if valid UTF-8
	if utf8.Valid(data) {
		return "UTF-8"
	}

	// Try to detect common encodings by attempting to decode
	// Latin-1 (ISO-8859-1) - most common fallback
	if s.isLikelyLatin1(data) {
		return "ISO-8859-1"
	}

	// Windows-1252 (common on Windows systems)
	if s.isLikelyWindows1252(data) {
		return "Windows-1252"
	}

	// Default to UTF-8 if we can't determine
	return "UTF-8"
}

// isLikelyLatin1 checks if data is likely ISO-8859-1 encoded.
func (s *MimeStage) isLikelyLatin1(data []byte) bool {
	// ISO-8859-1 is a single-byte encoding where all bytes are valid
	// We can't definitively detect it, but if it's not UTF-8 and contains
	// bytes in the 0x80-0xFF range, it might be Latin-1
	hasHighBytes := false
	for _, b := range data {
		if b >= 0x80 && b <= 0xFF {
			hasHighBytes = true
			break
		}
	}
	return hasHighBytes && !utf8.Valid(data)
}

// isLikelyWindows1252 checks if data is likely Windows-1252 encoded.
func (s *MimeStage) isLikelyWindows1252(data []byte) bool {
	// Windows-1252 is similar to Latin-1 but has some differences
	// This is a heuristic - Windows-1252 is common for Windows text files
	// For now, we'll use the same heuristic as Latin-1
	// A more sophisticated implementation would try to decode with Windows-1252
	return s.isLikelyLatin1(data)
}

// categorize determines the file type from extension and MIME (unused but kept for reference).
func (s *MimeStage) categorize(ext string, mimeType *entity.MimeTypeInfo) string {
	// Check extension first (more reliable for source files)
	switch ext {
	// Code files
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".ps1":
		return "powershell"
	case ".sql":
		return "sql"
	case ".r":
		return "r"
	case ".lua":
		return "lua"
	case ".pl", ".pm":
		return "perl"
	case ".ex", ".exs":
		return "elixir"
	case ".erl", ".hrl":
		return "erlang"
	case ".clj", ".cljs", ".cljc":
		return "clojure"
	case ".hs":
		return "haskell"
	case ".ml", ".mli":
		return "ocaml"
	case ".fs", ".fsi", ".fsx":
		return "fsharp"
	case ".dart":
		return "dart"
	case ".v":
		return "vlang"
	case ".zig":
		return "zig"
	case ".nim":
		return "nim"

	// Markup and data
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "css"
	case ".xml":
		return "xml"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md", ".markdown":
		return "markdown"
	case ".rst":
		return "restructuredtext"
	case ".tex":
		return "latex"

	// Documents
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "word"
	case ".xls", ".xlsx":
		return "excel"
	case ".ppt", ".pptx":
		return "powerpoint"
	case ".odt", ".ods", ".odp":
		return "opendocument"
	case ".txt":
		return "text"
	case ".rtf":
		return "rtf"
	case ".csv":
		return "csv"

	// Images
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".png":
		return "png"
	case ".gif":
		return "gif"
	case ".svg":
		return "svg"
	case ".webp":
		return "webp"
	case ".bmp":
		return "bmp"
	case ".ico":
		return "icon"
	case ".psd":
		return "photoshop"
	case ".ai":
		return "illustrator"
	case ".sketch":
		return "sketch"
	case ".fig":
		return "figma"

	// Audio
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	case ".ogg":
		return "ogg"
	case ".flac":
		return "flac"
	case ".m4a", ".aac":
		return "aac"

	// Video
	case ".mp4", ".m4v":
		return "mp4"
	case ".mkv":
		return "mkv"
	case ".avi":
		return "avi"
	case ".mov":
		return "quicktime"
	case ".webm":
		return "webm"

	// Archives
	case ".zip":
		return "zip"
	case ".tar":
		return "tar"
	case ".gz", ".gzip":
		return "gzip"
	case ".rar":
		return "rar"
	case ".7z":
		return "7zip"

	// Config files
	case ".env":
		return "dotenv"
	case ".gitignore", ".dockerignore":
		return "ignore"
	case ".editorconfig":
		return "editorconfig"
	case ".prettierrc", ".eslintrc":
		return "config"

	// Build files
	case ".make", ".makefile":
		return "makefile"
	case ".dockerfile":
		return "dockerfile"
	case ".gradle":
		return "gradle"
	case ".cmake":
		return "cmake"
	}

	// Fall back to MIME-based categorization
	if mimeType != nil && mimeType.MimeType != "" {
		return s.mimeToType(mimeType.MimeType)
	}

	return "unknown"
}

// mimeToType converts a MIME type to a file type.
func (s *MimeStage) mimeToType(mimeType string) string {
	mimeMap := map[string]string{
		"application/pdf":        "pdf",
		"application/json":       "json",
		"application/xml":        "xml",
		"application/zip":        "zip",
		"application/x-tar":      "tar",
		"application/gzip":       "gzip",
		"application/javascript": "javascript",
		"application/typescript": "typescript",
		"application/x-python":   "python",
		"application/msword":     "word",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "word",
		"application/vnd.ms-excel": "excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "excel",
		"application/vnd.ms-powerpoint":                                             "powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": "powerpoint",
		"text/plain":    "text",
		"text/html":     "html",
		"text/css":      "css",
		"text/markdown": "markdown",
		"text/csv":      "csv",
		"image/jpeg":    "jpeg",
		"image/png":     "png",
		"image/gif":     "gif",
		"image/svg+xml": "svg",
		"image/webp":    "webp",
		"audio/mpeg":    "mp3",
		"audio/wav":     "wav",
		"audio/ogg":     "ogg",
		"video/mp4":     "mp4",
		"video/webm":    "webm",
	}

	if t, ok := mimeMap[mimeType]; ok {
		return t
	}
	return "unknown"
}

// getCategory returns the high-level category for a MIME type.
func (s *MimeStage) getCategory(mimeType string) string {
	if mimeType == "" {
		return "unknown"
	}

	// Check prefix
	switch {
	case hasPrefix(mimeType, "text/"):
		return "text"
	case hasPrefix(mimeType, "image/"):
		return "image"
	case hasPrefix(mimeType, "audio/"):
		return "audio"
	case hasPrefix(mimeType, "video/"):
		return "video"
	case hasPrefix(mimeType, "application/"):
		return s.applicationCategory(mimeType)
	}

	return "other"
}

// applicationCategory categorizes application/* MIME types.
func (s *MimeStage) applicationCategory(mimeType string) string {
	codeTypes := []string{
		"application/javascript",
		"application/typescript",
		"application/x-python",
		"application/x-ruby",
		"application/x-perl",
	}

	docTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument",
		"application/vnd.ms-",
		"application/vnd.oasis.opendocument",
	}

	archiveTypes := []string{
		"application/zip",
		"application/x-tar",
		"application/gzip",
		"application/x-rar",
		"application/x-7z",
	}

	for _, t := range codeTypes {
		if mimeType == t {
			return "code"
		}
	}

	for _, t := range docTypes {
		if hasPrefix(mimeType, t) {
			return "document"
		}
	}

	for _, t := range archiveTypes {
		if mimeType == t {
			return "archive"
		}
	}

	if mimeType == "application/json" || mimeType == "application/xml" {
		return "data"
	}

	return "application"
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// mimeTypeToCategory converts a MIME type string to our category format.
func (s *MimeStage) mimeTypeToCategory(mimeType string) string {
	return s.getCategory(mimeType)
}

// detectEncodingFromMimeType extracts encoding information from MIME type.
func (s *MimeStage) detectEncodingFromMimeType(mimeType string) string {
	// Check for charset in MIME type (e.g., "text/plain; charset=utf-8")
	if strings.Contains(mimeType, "charset=") {
		parts := strings.Split(mimeType, "charset=")
		if len(parts) > 1 {
			encoding := strings.TrimSpace(strings.Split(parts[1], ";")[0])
			return strings.ToUpper(encoding)
		}
	}

	// Default encoding based on MIME type category
	if strings.HasPrefix(mimeType, "text/") || strings.HasPrefix(mimeType, "application/json") || 
		strings.HasPrefix(mimeType, "application/xml") || strings.HasPrefix(mimeType, "application/javascript") {
		return "utf-8"
	}

	return ""
}
