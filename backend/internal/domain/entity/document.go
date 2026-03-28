package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DocumentID uniquely identifies a parsed document.
type DocumentID string

// ChunkID uniquely identifies a content chunk.
type ChunkID string

// NewDocumentID creates a document ID from a relative path.
func NewDocumentID(relativePath string) DocumentID {
	normalized := filepath.ToSlash(relativePath)
	hash := sha256.Sum256([]byte("doc:" + normalized))
	return DocumentID(hex.EncodeToString(hash[:]))
}

// NewChunkID creates a chunk ID from document ID and ordinal.
func NewChunkID(documentID DocumentID, ordinal int, headingPath string) ChunkID {
	payload := strings.Join([]string{string(documentID), headingPath, strconv.Itoa(ordinal)}, "|")
	hash := sha256.Sum256([]byte(payload))
	return ChunkID(hex.EncodeToString(hash[:]))
}

// String returns the string representation of DocumentID.
func (id DocumentID) String() string {
	return string(id)
}

// String returns the string representation of ChunkID.
func (id ChunkID) String() string {
	return string(id)
}

// Document represents a parsed Markdown document.
type Document struct {
	ID            DocumentID
	FileID        FileID
	RelativePath  string
	Title         string
	State         DocumentState // Lifecycle state
	Frontmatter   map[string]interface{}
	Checksum      string
	StateChangedAt *time.Time // When state was last changed
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Chunk represents a semantically grouped section of a document.
type Chunk struct {
	ID          ChunkID
	DocumentID  DocumentID
	Ordinal     int
	Heading     string
	HeadingPath string
	Text        string
	TokenCount  int
	StartLine   int
	EndLine     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ChunkEmbedding stores a vector representation for a chunk.
type ChunkEmbedding struct {
	ChunkID    ChunkID
	Vector     []float32
	Dimensions int
	UpdatedAt  time.Time
}
