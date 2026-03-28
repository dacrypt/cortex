package entity

import (
	"time"
)

// TikaMetadata represents metadata extracted by Apache Tika.
// This structure maps to Tika's standard metadata format.
type TikaMetadata struct {
	// Basic document metadata (mapped from Tika JSON)
	Title           *string
	Author          []string
	Creator         []string
	Subject         *string
	Description     *string
	Keywords        []string
	Language        *string
	LanguageCode    *string // ISO 639-1
	ContentType     *string // MIME type
	ContentEncoding *string

	// Dates (parsed from strings)
	Created   *time.Time
	Modified  *time.Time
	LastSaved *time.Time

	// Technical metadata
	PageCount      *int
	WordCount      *int
	CharacterCount *int

	// Extended metadata (all fields from Tika)
	RawMetadata map[string]interface{}

	// Extraction metadata
	ExtractedBy    string
	ExtractionDate time.Time
}

// NewTikaMetadata creates a new TikaMetadata instance.
func NewTikaMetadata() *TikaMetadata {
	return &TikaMetadata{
		Author:         []string{},
		Creator:        []string{},
		Keywords:       []string{},
		RawMetadata:    make(map[string]interface{}),
		ExtractionDate: time.Now(),
		ExtractedBy:    "tika",
	}
}

