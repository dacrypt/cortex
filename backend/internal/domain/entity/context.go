package entity

import (
	"time"
)

// AIContext contains AI-extracted contextual information about a document.
// This includes authors, publication details, temporal information, and other
// contextual metadata extracted using LLM with RAG.
type AIContext struct {
	// Authors and Contributors
	Authors          []AuthorInfo    // Primary and secondary authors
	Editors          []string        // Editors
	Translators      []string        // Translators
	Contributors     []string        // Other contributors
	
	// Publication Information
	Publisher        *string         // Publishing house/editorial
	PublicationYear  *int            // Year of publication
	PublicationPlace *string        // City/country of publication
	Edition          *string         // Edition number/info
	ISBN             *string         // ISBN number
	ISSN             *string         // ISSN number
	DOI              *string         // Digital Object Identifier
	
	// Temporal Context
	DocumentDate     *time.Time      // Date mentioned in document
	HistoricalPeriod *string         // Historical period (e.g., "Medieval", "Renaissance")
	TimeRange        *TimeRange      // Start and end dates if document covers a period
	
	// Geographic Context
	Locations        []LocationInfo  // Places mentioned or relevant
	Regions          []string        // Geographic regions
	
	// People and Organizations
	PeopleMentioned  []PersonInfo    // Important people mentioned
	Organizations    []OrgInfo       // Organizations/institutions mentioned
	
	// Events and References
	HistoricalEvents []EventInfo     // Historical events mentioned
	References       []ReferenceInfo // Bibliographic references
	
	// Language and Translation
	OriginalLanguage *string         // Original language if translated
	TranslationInfo  *TranslationInfo // Translation details
	
	// Additional Context
	Genre            *string         // Literary genre (novel, essay, etc.)
	Subject          *string         // Subject matter
	Audience         *string         // Target audience
	
	// Enriched Metadata (from external APIs)
	EnrichedMetadata *EnrichedMetadata // Metadata enriched from external APIs (Amazon, Open Library, etc.)
	
	// Metadata
	Confidence       float64         // Overall confidence in extraction
	ExtractedAt      time.Time       // When this context was extracted
	Source           string          // Source of extraction (e.g., "llm_rag")
}

// EnrichedMetadata contains metadata enriched from external APIs.
type EnrichedMetadata struct {
	// Basic Information
	Title           string   // Full title
	Subtitle        string   // Subtitle if available
	Authors         []string // All authors
	Publisher       string   // Publisher name
	PublicationDate string   // Publication date (YYYY-MM-DD)
	ISBN10          string   // ISBN-10
	ISBN13          string   // ISBN-13
	Pages           int      // Number of pages
	Language        string   // Language code
	
	// Descriptions
	Description      string  // Full description
	ShortDescription string  // Short description (first 200 chars)
	
	// Categorization
	Categories      []string // Categories/subjects
	Genre           string   // Genre
	Subject         string   // Subject
	
	// Ratings and Reviews
	Rating          float64  // Average rating
	RatingCount     int      // Number of ratings
	ReviewCount     int      // Number of reviews
	
	// Images
	CoverImageURL   string   // URL to cover image
	ThumbnailURL    string   // URL to thumbnail
	
	// Links
	AmazonURL       string   // Amazon product URL
	GoodreadsURL    string   // Goodreads URL
	
	// Additional Metadata
	Edition         string   // Edition information
	Format          string   // Format (Hardcover, Paperback, eBook, etc.)
	Dimensions      string   // Physical dimensions
	Weight          string   // Weight
	
	// Source information
	Source          string   // "amazon", "open_library", "google_books"
	EnrichedAt      time.Time // When metadata was enriched
}

// AuthorInfo contains information about an author.
type AuthorInfo struct {
	Name         string   // Full name
	Role         string   // "author", "co-author", "contributor", etc.
	Affiliation  *string  // Institution/organization
	Confidence   float64  // Confidence in this extraction
}

// LocationInfo contains geographic location information.
type LocationInfo struct {
	Name         string   // Location name
	Type         string   // "city", "country", "region", "continent"
	Coordinates  *Coordinates // Optional GPS coordinates
	Context      string   // How this location is relevant
}

// Coordinates represents geographic coordinates.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// TimeRange represents a period of time.
type TimeRange struct {
	Start *time.Time
	End   *time.Time
}

// PersonInfo contains information about a person mentioned in the document.
type PersonInfo struct {
	Name         string   // Person's name
	Role         string   // Their role (e.g., "saint", "scientist", "politician")
	Context      string   // How they're mentioned
	Confidence   float64  // Confidence in extraction
}

// OrgInfo contains information about an organization.
type OrgInfo struct {
	Name         string   // Organization name
	Type         string   // "church", "university", "government", etc.
	Context      string   // How it's relevant
	Confidence   float64  // Confidence in extraction
}

// EventInfo contains information about a historical event.
type EventInfo struct {
	Name         string   // Event name
	Date         *time.Time // When it occurred
	Location     *string  // Where it occurred
	Context      string   // How it's mentioned
	Confidence   float64  // Confidence in extraction
}

// ReferenceInfo contains bibliographic reference information.
type ReferenceInfo struct {
	Title        string   // Title of referenced work
	Author       *string  // Author of referenced work
	Year         *int     // Publication year
	Type         string   // "book", "article", "website", etc.
	URL          *string  // If available
}

// TranslationInfo contains translation metadata.
type TranslationInfo struct {
	OriginalLanguage string   // Source language
	TargetLanguage   string   // Target language
	Translator        *string  // Translator name
	TranslationYear   *int     // Year of translation
}

// NewAIContext creates a new AIContext with default values.
func NewAIContext() *AIContext {
	return &AIContext{
		Authors:          []AuthorInfo{},
		Editors:          []string{},
		Translators:      []string{},
		Contributors:     []string{},
		Locations:        []LocationInfo{},
		Regions:          []string{},
		PeopleMentioned:  []PersonInfo{},
		Organizations:    []OrgInfo{},
		HistoricalEvents: []EventInfo{},
		References:       []ReferenceInfo{},
		ExtractedAt:      time.Now(),
		Source:           "llm_rag",
	}
}

// HasAnyData returns true if any contextual information has been extracted.
func (c *AIContext) HasAnyData() bool {
	return len(c.Authors) > 0 ||
		len(c.Editors) > 0 ||
		len(c.Translators) > 0 ||
		len(c.Contributors) > 0 ||
		c.Publisher != nil ||
		c.PublicationYear != nil ||
		c.PublicationPlace != nil ||
		c.ISBN != nil ||
		c.ISSN != nil ||
		c.DOI != nil ||
		c.DocumentDate != nil ||
		c.HistoricalPeriod != nil ||
		len(c.Locations) > 0 ||
		len(c.Regions) > 0 ||
		len(c.PeopleMentioned) > 0 ||
		len(c.Organizations) > 0 ||
		len(c.HistoricalEvents) > 0 ||
		len(c.References) > 0 ||
		c.OriginalLanguage != nil ||
		c.Genre != nil ||
		c.Subject != nil ||
		c.Audience != nil
}

