// Package entity contains domain entities for the Cortex system.
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
)

// FileID is a stable identifier for a file (SHA-256 hash of relative path).
type FileID string

// NewFileID creates a FileID from a relative path.
func NewFileID(relativePath string) FileID {
	// Normalize path separators
	normalized := filepath.ToSlash(relativePath)
	hash := sha256.Sum256([]byte(normalized))
	return FileID(hex.EncodeToString(hash[:]))
}

// String returns the string representation of the FileID.
func (id FileID) String() string {
	return string(id)
}

// FileEntry represents a file in the workspace index.
type FileEntry struct {
	ID           FileID
	RelativePath string
	AbsolutePath string
	Filename     string
	Extension    string
	FileSize     int64
	LastModified time.Time
	CreatedAt    time.Time
	Enhanced     *EnhancedMetadata
}

// NewFileEntry creates a new FileEntry from path information.
func NewFileEntry(workspaceRoot, relativePath string, size int64, modTime time.Time) *FileEntry {
	absPath := filepath.Join(workspaceRoot, relativePath)
	filename := filepath.Base(relativePath)
	ext := strings.ToLower(filepath.Ext(filename))

	return &FileEntry{
		ID:           NewFileID(relativePath),
		RelativePath: relativePath,
		AbsolutePath: absPath,
		Filename:     filename,
		Extension:    ext,
		FileSize:     size,
		LastModified: modTime,
		CreatedAt:    time.Now(),
	}
}

// EnhancedMetadata contains rich metadata extracted from files.
type EnhancedMetadata struct {
	Stats            *FileStats
	Folder           string
	Depth            int
	PathComponents   []string // Extracted path components for semantic analysis
	PathPattern      string   // Normalized path pattern for matching
	Language         *string
	LanguageConfidence *float64 // Confidence score for language detection (0.0-1.0)
	ContentEncoding  *string    // Detected content encoding (UTF-8, Latin-1, etc.)
	ContentStructure *ContentStructure // Document structure analysis
	CodeImports      []CodeImport      // Import/export relationships extracted from code
	FileHash         *FileHash         // File hashes for duplicate detection
	MetadataConsistency *MetadataConsistency // Metadata consistency validation results
	TemporalMetrics *TemporalMetrics  // Temporal analysis metrics
	ContentQuality  *ContentQuality   // Content quality metrics
	MimeType         *MimeTypeInfo
	CodeMetrics      *CodeMetrics
	DocumentMetrics  *DocumentMetrics
	ImageMetadata    *ImageMetadata
	AudioMetadata    *AudioMetadata
	VideoMetadata    *VideoMetadata
	OSMetadata       *OSMetadata
	OSContextTaxonomy *OSContextTaxonomy
	IndexedState     IndexedState
	IndexingErrors   []IndexingError // Errors encountered during indexing
	CustomData       map[string]interface{}
	TikaMetadata     *TikaMetadata // Metadata extracted by Apache Tika
}

// FileStats contains basic file statistics.
type FileStats struct {
	Size       int64
	Created    time.Time
	Modified   time.Time
	Accessed   time.Time
	Changed    *time.Time // Metadata changed time (ctime on Unix) - nil if not available
	Backup     *time.Time // Backup time - nil if not available
	IsReadOnly bool
	IsHidden   bool
}

// FileHash contains file hash information for duplicate detection and integrity checking.
type FileHash struct {
	MD5    string // MD5 hash
	SHA256 string // SHA-256 hash
	SHA512 string // SHA-512 hash
}

// MimeTypeInfo contains MIME type information.
type MimeTypeInfo struct {
	MimeType string
	Category string // text, code, image, document, binary, archive, audio, video
	Encoding string
}

// CodeMetrics contains code analysis metrics.
type CodeMetrics struct {
	LinesOfCode       int
	CommentLines      int
	BlankLines        int
	CommentPercentage float64
	FunctionCount     int
	ClassCount        int
	Complexity        float64
	Imports           []string
	Exports           []string
}

// DocumentMetrics contains document analysis metrics.
type DocumentMetrics struct {
	// Basic metrics
	PageCount      int
	WordCount      int
	CharacterCount int
	
	// PDF Info Dictionary (standard PDF metadata)
	Author         *string
	Title          *string
	Subject        *string
	Keywords       []string
	Creator        *string // Application that created the original document
	Producer       *string // Software that produced the PDF
	CreatedDate    *time.Time
	ModifiedDate   *time.Time
	Trapped        *string // PDF trapping information
	
	// XMP Metadata (Extensible Metadata Platform - richer metadata)
	XMPTitle            *string
	XMPDescription      *string
	XMPCreator          []string // Can be multiple creators
	XMPContributor      []string
	XMPRights           *string
	XMPRightsOwner       []string
	XMPCopyright        *string
	XMPCopyrightURL     *string
	XMPIdentifier       []string
	XMPLanguage         []string
	XMPRating           *int
	XMPMetadataDate     *time.Time
	XMPModifyDate       *time.Time
	XMPCreateDate       *time.Time
	XMPNickname         *string
	XMPLabel            []string
	XMPMarked            *bool
	XMPUsageTerms        *string
	XMPWebStatement     *string
	
	// PDF-specific technical metadata
	PDFVersion          *string // e.g., "1.4", "1.7"
	PDFEncrypted        *bool
	PDFLinearized       *bool
	PDFTagged           *bool
	PDFPageLayout       *string // e.g., "SinglePage", "TwoColumnLeft"
	PDFPageMode         *string // e.g., "UseNone", "UseOutlines"
	
	// Additional document properties
	Company             *string
	Category            *string
	Comments            *string
	Hyperlinks          []string // URLs found in document
	Fonts               []string // Fonts used in document
	ColorSpace          []string // Color spaces used
	ImageCount          *int
	FormFields          *int
	Annotations         *int
	
	// Custom/arbitrary metadata (key-value pairs)
	CustomProperties    map[string]string
}

// ImageMetadata contains image-specific metadata (EXIF, IPTC, XMP).
type ImageMetadata struct {
	// Basic image properties
	Width       int
	Height      int
	ColorDepth  int // Bits per pixel
	ColorSpace  *string // RGB, CMYK, Grayscale, etc.
	Format      *string // JPEG, PNG, TIFF, etc.
	Orientation *int // EXIF orientation (1-8)
	
	// EXIF (Exchangeable Image File Format) metadata
	EXIFCameraMake         *string
	EXIFCameraModel        *string
	EXIFSoftware           *string
	EXIFDateTimeOriginal   *time.Time
	EXIFDateTimeDigitized   *time.Time
	EXIFDateTimeModified    *time.Time
	EXIFArtist              *string
	EXIFCopyright           *string
	EXIFImageDescription    *string
	EXIFUserComment         *string
	
	// Camera settings
	EXIFFNumber             *float64 // Aperture (f-stop)
	EXIFExposureTime        *string // Shutter speed (e.g., "1/125")
	EXIFISO                 *int
	EXIFFocalLength         *float64 // mm
	EXIFFocalLength35mm     *int // 35mm equivalent
	EXIFExposureMode        *string
	EXIFWhiteBalance        *string
	EXIFFlash               *string
	EXIFMeteringMode        *string
	EXIFExposureProgram     *string
	
	// GPS/Location data
	GPSLatitude             *float64
	GPSLongitude            *float64
	GPSAltitude             *float64
	GPSLatitudeRef          *string // N or S
	GPSLongitudeRef         *string // E or W
	GPSAltitudeRef          *string // Above or Below sea level
	GPSLocation             *string // Human-readable location
	
	// IPTC (International Press Telecommunications Council) metadata
	IPTCObjectName          *string
	IPTCCaption              *string
	IPTCKeywords             []string
	IPTCCopyrightNotice      *string
	IPTCByline               *string // Author/Photographer
	IPTCBylineTitle          *string
	IPTCHeadline             *string
	IPTCContact              *string
	IPTCContactCity          *string
	IPTCContactCountry       *string
	IPTCContactEmail         *string
	IPTCContactPhone          *string
	IPTCContactWebsite       *string
	IPTCSource               *string
	IPTCUsageTerms           *string
	
	// XMP (Extensible Metadata Platform) metadata
	XMPTitle                 *string
	XMPDescription           *string
	XMPCreator               []string
	XMPRights                *string
	XMPRating                *int
	XMPLabel                 []string
	XMPSubject               []string // Keywords/tags
	
	// Image analysis
	DominantColors           []string // Main colors in image
	HasTransparency          *bool
	IsAnimated               *bool
	FrameCount               *int // For animated GIFs
}

// AudioMetadata contains audio file metadata (ID3, Vorbis Comments, etc.).
type AudioMetadata struct {
	// Basic audio properties
	Duration        *float64 // Duration in seconds
	Bitrate         *int // kbps
	SampleRate      *int // Hz
	Channels        *int // Mono (1), Stereo (2), etc.
	BitDepth        *int // Bits per sample
	Codec           *string // MP3, FLAC, AAC, etc.
	Format          *string // Container format
	
	// ID3 tags (MP3)
	ID3Title       *string
	ID3Artist      *string
	ID3Album       *string
	ID3Year        *int
	ID3Genre       *string
	ID3Track       *int
	ID3Disc        *int
	ID3Composer    *string
	ID3Conductor   *string
	ID3Performer   *string
	ID3Publisher   *string
	ID3Comment     *string
	ID3Lyrics      *string
	ID3BPM         *int // Beats per minute
	ID3ISRC        *string // International Standard Recording Code
	ID3Copyright   *string
	ID3EncodedBy   *string
	ID3AlbumArtist *string
	
	// Vorbis Comments (FLAC, OGG)
	VorbisTitle    *string
	VorbisArtist   *string
	VorbisAlbum    *string
	VorbisDate     *string
	VorbisGenre    *string
	VorbisTrack    *string
	VorbisComment  *string
	
	// Album art
	HasAlbumArt    *bool
	AlbumArtFormat *string // JPEG, PNG, etc.
	AlbumArtSize   *int // Size in bytes
	
	// Technical metadata
	ReplayGain     *float64 // dB
	Normalized     *bool
	Lossless       *bool
}

// VideoMetadata contains video file metadata.
type VideoMetadata struct {
	// Basic video properties
	Duration        *float64 // Duration in seconds
	Width           int
	Height          int
	FrameRate       *float64 // fps
	Bitrate         *int // kbps
	Codec           *string // H.264, H.265, VP9, etc.
	Container       *string // MP4, AVI, MKV, etc.
	
	// Video stream metadata
	VideoCodec      *string
	VideoBitrate     *int
	VideoPixelFormat *string
	VideoColorSpace  *string
	VideoAspectRatio *string // e.g., "16:9"
	
	// Audio stream metadata
	AudioCodec      *string
	AudioBitrate    *int
	AudioSampleRate *int
	AudioChannels   *int
	AudioLanguage   *string
	
	// Metadata tags
	Title           *string
	Artist          *string
	Album           *string
	Genre           *string
	Year            *int
	Director        *string
	Producer        *string
	Copyright       *string
	Description     *string
	Comment         *string
	
	// Technical metadata
	HasSubtitles    *bool
	SubtitleTracks  []string // Languages
	HasChapters     *bool
	ChapterCount    *int
	Is3D            *bool
	IsHD            *bool
	Is4K            *bool
}

// ContentStructure contains analysis of document structure.
type ContentStructure struct {
	// Headings hierarchy
	Headings        []HeadingInfo // All headings with their levels
	HeadingDepth    int           // Maximum heading depth (H1=1, H2=2, etc.)
	
	// Lists
	HasLists        bool
	ListCount       int
	MaxListDepth    int // Maximum nesting depth of lists
	
	// Table of contents
	HasTOC          bool
	TOCEntries      []string // Table of contents entries if detected
	
	// Cross-references
	HasCrossRefs    bool
	CrossRefCount   int
	
	// Footnotes/endnotes
	HasFootnotes    bool
	HasEndnotes     bool
	FootnoteCount   int
	EndnoteCount    int
	
	// Document sections
	SectionCount    int
	Sections        []SectionInfo
}

// HeadingInfo represents a heading in the document.
type HeadingInfo struct {
	Level    int    // Heading level (1-6 for H1-H6)
	Text     string // Heading text
	Line     int    // Line number where heading appears
	Path     string // Heading path (e.g., "Introduction/Overview")
}

// SectionInfo represents a document section.
type SectionInfo struct {
	Title    string
	Level    int
	StartLine int
	EndLine   int
	HeadingPath string
}

// CodeImport represents an import/export relationship in source code.
type CodeImport struct {
	Path        string   // Import path/module name
	Type        string   // "import", "require", "include", "use", etc.
	Language    string   // Programming language
	Line        int      // Line number where import occurs
	Confidence  float64  // Confidence in extraction (0.0-1.0)
	ResolvedPath *string // Resolved file path (if successfully resolved)
}

// MetadataConsistency contains results of metadata consistency validation.
type MetadataConsistency struct {
	Score   float64  // Overall consistency score (0.0-1.0)
	Issues  []string // List of consistency issues found
	Warnings []string // Warnings about potential inconsistencies
}

// TemporalMetrics contains temporal analysis metrics.
type TemporalMetrics struct {
	Age                  time.Duration // Time since creation
	Staleness            time.Duration // Time since last modification
	AccessFrequency      float64       // Accesses per time period (e.g., per day)
	ModificationFrequency float64      // Modifications per time period
	LastAccessPattern    string        // "recent", "occasional", "rare", "never"
}

// ContentQuality contains content quality analysis metrics.
type ContentQuality struct {
	ReadabilityScore  float64 // Flesch-Kincaid, SMOG, etc.
	ComplexityScore   float64 // Text/code complexity (0.0-1.0)
	QualityScore      float64 // Overall quality score (0.0-1.0)
	ReadabilityLevel   string  // "elementary", "high_school", "college", "graduate", "professional"
}

// IndexedState tracks which indexing phases have been completed.
type IndexedState struct {
	Basic    bool
	Mime     bool
	Code     bool
	Document bool
	Mirror   bool
}

// IsFullyIndexed returns true if all indexing phases are complete.
func (s IndexedState) IsFullyIndexed() bool {
	return s.Basic && s.Mime
}

// IndexingError represents an error encountered during file indexing.
type IndexingError struct {
	Stage       string    // Stage where error occurred (e.g., "document", "mirror", "code")
	Operation   string    // Operation that failed (e.g., "read_mirror", "parse_content")
	Error       string    // Error message
	Details     string    // Additional details for debugging
	Requirement string    // What requirement was missing (e.g., "mirror_file", "metadata")
	Timestamp   time.Time // When the error occurred
}

// HasIndexingErrors returns true if there are any indexing errors.
func (e *EnhancedMetadata) HasIndexingErrors() bool {
	return e != nil && len(e.IndexingErrors) > 0
}

// AddIndexingError adds an indexing error to the metadata.
func (e *EnhancedMetadata) AddIndexingError(stage, operation, errMsg, details, requirement string) {
	if e == nil {
		return
	}
	if e.IndexingErrors == nil {
		e.IndexingErrors = []IndexingError{}
	}
	e.IndexingErrors = append(e.IndexingErrors, IndexingError{
		Stage:       stage,
		Operation:   operation,
		Error:       errMsg,
		Details:     details,
		Requirement: requirement,
		Timestamp:   time.Now(),
	})
}

// FileEvent represents a file system event.
type FileEvent struct {
	Type      FileEventType
	Path      string
	OldPath   *string // For rename events
	Timestamp time.Time
}

// FileEventType enumerates file event types.
type FileEventType int

const (
	FileEventCreated FileEventType = iota + 1
	FileEventModified
	FileEventDeleted
	FileEventRenamed
)

// String returns the string representation of FileEventType.
func (t FileEventType) String() string {
	switch t {
	case FileEventCreated:
		return "created"
	case FileEventModified:
		return "modified"
	case FileEventDeleted:
		return "deleted"
	case FileEventRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}
