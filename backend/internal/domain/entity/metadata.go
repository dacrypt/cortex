package entity

import (
	"path/filepath"
	"strings"
	"time"
)

// FileMetadata contains user-assigned semantic metadata.
type FileMetadata struct {
	FileID            FileID
	RelativePath      string
	Tags              []string
	Contexts          []string
	SuggestedContexts []string
	Type              string
	Notes             *string
	DetectedLanguage  *string // Language detected by LLM (e.g., "es", "en", "fr")
	AISummary         *AISummary
	AICategory        *AICategory
	AIContext         *AIContext // AI-extracted contextual information (authors, dates, etc.)
	EnrichmentData    *EnrichmentData // Enrichment data from various techniques (NER, citations, OCR, etc.)
	AIRelated         []RelatedFile
	Mirror            *MirrorMetadata
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewFileMetadata creates a new FileMetadata for a file.
func NewFileMetadata(relativePath, extension string) *FileMetadata {
	fileType := inferFileType(extension)
	now := time.Now()

	return &FileMetadata{
		FileID:            NewFileID(relativePath),
		RelativePath:      relativePath,
		Tags:              []string{},
		Contexts:          []string{},
		SuggestedContexts: []string{},
		Type:              fileType,
		AIRelated:         []RelatedFile{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// HasTag returns true if the file has the specified tag.
func (m *FileMetadata) HasTag(tag string) bool {
	normalizedTag := strings.ToLower(strings.TrimSpace(tag))
	for _, t := range m.Tags {
		if strings.ToLower(t) == normalizedTag {
			return true
		}
	}
	return false
}

// HasContext returns true if the file has the specified context.
func (m *FileMetadata) HasContext(context string) bool {
	normalizedContext := strings.ToLower(strings.TrimSpace(context))
	for _, c := range m.Contexts {
		if strings.ToLower(c) == normalizedContext {
			return true
		}
	}
	return false
}

// AddTag adds a tag if not already present.
func (m *FileMetadata) AddTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" || m.HasTag(tag) {
		return false
	}
	m.Tags = append(m.Tags, tag)
	m.UpdatedAt = time.Now()
	return true
}

// RemoveTag removes a tag if present.
func (m *FileMetadata) RemoveTag(tag string) bool {
	normalizedTag := strings.ToLower(strings.TrimSpace(tag))
	for i, t := range m.Tags {
		if strings.ToLower(t) == normalizedTag {
			m.Tags = append(m.Tags[:i], m.Tags[i+1:]...)
			m.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// AddContext adds a context if not already present.
func (m *FileMetadata) AddContext(context string) bool {
	context = strings.TrimSpace(context)
	if context == "" || m.HasContext(context) {
		return false
	}
	m.Contexts = append(m.Contexts, context)
	m.UpdatedAt = time.Now()
	return true
}

// RemoveContext removes a context if present.
func (m *FileMetadata) RemoveContext(context string) bool {
	normalizedContext := strings.ToLower(strings.TrimSpace(context))
	for i, c := range m.Contexts {
		if strings.ToLower(c) == normalizedContext {
			m.Contexts = append(m.Contexts[:i], m.Contexts[i+1:]...)
			m.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// AISummary contains AI-generated content analysis.
type AISummary struct {
	// AI processing metadata
	ProcessedAt      *time.Time          // When AI processing occurred
	ModelVersion     string              // LLM model version used
	TokensUsed       int                 // Number of tokens consumed
	Cost             float64             // Cost in currency units (if applicable)
	ConfidenceScores map[string]float64  // Per-field confidence scores
	
	// Existing fields
	Summary     string
	ContentHash string
	KeyTerms    []string
	GeneratedAt time.Time
}

// AICategory contains AI-generated category classification.
type AICategory struct {
	Category   string
	Confidence float64
	UpdatedAt  time.Time
}

// RelatedFile contains AI-suggested relationships between files.
type RelatedFile struct {
	RelativePath string
	Similarity   float64
	Reason       string
}

// MirrorMetadata contains document content extraction info.
type MirrorMetadata struct {
	// Extraction metadata
	ExtractionMethod   string   // "pandoc", "pdftotext", "pdf_library", "office_extractor", etc.
	ExtractionConfidence float64 // Confidence in extraction quality (0.0-1.0)
	ExtractionQuality  float64  // Quality score of extracted content (0.0-1.0)
	ExtractionErrors   []string // Errors encountered during extraction
	ExtractionWarnings []string // Warnings during extraction
	
	// Existing fields
	Format      MirrorFormat
	Path        string
	SourceMtime time.Time
	UpdatedAt   time.Time
}

// MirrorFormat represents the format of mirrored content.
type MirrorFormat string

const (
	MirrorFormatMarkdown MirrorFormat = "md"
	MirrorFormatCSV      MirrorFormat = "csv"
)

// inferFileType infers the file type from extension.
func inferFileType(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	typeMap := map[string]string{
		// Code
		"go":    "go",
		"ts":    "typescript",
		"tsx":   "typescript",
		"js":    "javascript",
		"jsx":   "javascript",
		"py":    "python",
		"rb":    "ruby",
		"java":  "java",
		"kt":    "kotlin",
		"swift": "swift",
		"rs":    "rust",
		"cpp":   "cpp",
		"c":     "c",
		"h":     "c",
		"hpp":   "cpp",
		"cs":    "csharp",
		"php":   "php",
		"scala": "scala",
		"clj":   "clojure",
		"ex":    "elixir",
		"exs":   "elixir",
		"erl":   "erlang",
		"hs":    "haskell",
		"ml":    "ocaml",
		"fs":    "fsharp",
		"r":     "r",
		"jl":    "julia",
		"lua":   "lua",
		"pl":    "perl",
		"sh":    "shell",
		"bash":  "shell",
		"zsh":   "shell",
		"fish":  "shell",
		"ps1":   "powershell",
		"sql":   "sql",
		"v":     "verilog",
		"vhd":   "vhdl",
		"asm":   "assembly",
		"s":     "assembly",

		// Markup & Data
		"html": "html",
		"htm":  "html",
		"xml":  "xml",
		"xsl":  "xml",
		"xslt": "xml",
		"css":  "css",
		"scss": "scss",
		"sass": "sass",
		"less": "less",
		"json": "json",
		"yaml": "yaml",
		"yml":  "yaml",
		"toml": "toml",
		"ini":  "ini",
		"conf": "config",
		"cfg":  "config",
		"env":  "env",

		// Documents
		"md":       "markdown",
		"markdown": "markdown",
		"txt":      "text",
		"rtf":      "rtf",
		"pdf":      "pdf",
		"doc":      "word",
		"docx":     "word",
		"odt":      "opendocument",
		"xls":      "excel",
		"xlsx":     "excel",
		"ods":      "opendocument",
		"ppt":      "powerpoint",
		"pptx":     "powerpoint",
		"odp":      "opendocument",
		"csv":      "csv",
		"tsv":      "tsv",
		"tex":      "latex",
		"bib":      "bibtex",

		// Images
		"png":    "image",
		"jpg":    "image",
		"jpeg":   "image",
		"gif":    "image",
		"bmp":    "image",
		"svg":    "svg",
		"ico":    "image",
		"webp":   "image",
		"tiff":   "image",
		"tif":    "image",
		"psd":    "photoshop",
		"ai":     "illustrator",
		"sketch": "sketch",
		"fig":    "figma",
		"xd":     "xd",

		// Media
		"mp3":  "audio",
		"wav":  "audio",
		"flac": "audio",
		"aac":  "audio",
		"ogg":  "audio",
		"m4a":  "audio",
		"mp4":  "video",
		"avi":  "video",
		"mkv":  "video",
		"mov":  "video",
		"wmv":  "video",
		"webm": "video",
		"flv":  "video",

		// Archives
		"zip": "archive",
		"tar": "archive",
		"gz":  "archive",
		"bz2": "archive",
		"xz":  "archive",
		"7z":  "archive",
		"rar": "archive",

		// Special
		"proto":   "protobuf",
		"graphql": "graphql",
		"gql":     "graphql",
		"lock":    "lockfile",
		"sum":     "checksum",
	}

	if t, ok := typeMap[ext]; ok {
		return t
	}

	// Check for dotfiles
	if ext == "" {
		return "unknown"
	}

	return ext
}

// GetMirrorFormat returns the appropriate mirror format for an extension.
func GetMirrorFormat(ext string) *MirrorFormat {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	mdFormats := map[string]bool{
		"pdf": true, "doc": true, "docx": true, "odt": true,
		"ppt": true, "pptx": true, "odp": true,
	}

	csvFormats := map[string]bool{
		"xls": true, "xlsx": true, "ods": true,
	}

	if mdFormats[ext] {
		f := MirrorFormatMarkdown
		return &f
	}
	if csvFormats[ext] {
		f := MirrorFormatCSV
		return &f
	}
	return nil
}

// GetMirrorPath returns the mirror file path for a source file.
func GetMirrorPath(relativePath string, format MirrorFormat) string {
	return filepath.Join(".cortex", "mirror", relativePath+"."+string(format))
}
