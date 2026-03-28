package entity

import (
	"time"
)

// EnrichmentData contains all enrichment data extracted from various techniques.
type EnrichmentData struct {
	// Named Entity Recognition
	NamedEntities []NamedEntity `json:"named_entities,omitempty"`
	
	// Citations and References
	Citations []Citation `json:"citations,omitempty"`
	
	// Sentiment Analysis
	Sentiment *SentimentAnalysis `json:"sentiment,omitempty"`
	
	// Tables extracted
	Tables []Table `json:"tables,omitempty"`
	
	// Mathematical Formulas
	Formulas []Formula `json:"formulas,omitempty"`
	
	// Code Dependencies
	Dependencies []Dependency `json:"dependencies,omitempty"`
	
	// Duplicate Detection
	Duplicates []DuplicateInfo `json:"duplicates,omitempty"`
	
	// OCR Results
	OCRText *OCRResult `json:"ocr_text,omitempty"`
	
	// Audio/Video Transcription
	Transcription *TranscriptionResult `json:"transcription,omitempty"`
	
	// Processing metadata
	ProcessedAt  time.Time   `json:"processed_at"`  // When enrichment was performed
	MethodsUsed  []string    `json:"methods_used"`  // ["ner", "citations", "sentiment", "ocr", etc.]
	ToolsUsed    []string    `json:"tools_used"`    // Tool names/versions used
	Errors       []string    `json:"errors,omitempty"` // Errors encountered
	Warnings     []string    `json:"warnings,omitempty"` // Warnings during processing
	
	// Legacy metadata (kept for backward compatibility)
	ExtractedAt time.Time `json:"extracted_at"`
	Source      string    `json:"source"` // "ner", "citations", "ocr", etc.
}

// NamedEntity represents an entity extracted through NER.
type NamedEntity struct {
	Text        string  `json:"text"`
	Type        string  `json:"type"` // "PERSON", "LOCATION", "ORGANIZATION", "DATE", "MONEY", "PERCENT", etc.
	StartPos    int     `json:"start_pos"`
	EndPos      int     `json:"end_pos"`
	Confidence  float64 `json:"confidence"`
	Context     string  `json:"context,omitempty"` // Surrounding text
}

// Citation represents a bibliographic citation.
type Citation struct {
	Text        string   `json:"text"`        // Full citation text
	Authors     []string `json:"authors,omitempty"`
	Title       string   `json:"title,omitempty"`
	Year        *int     `json:"year,omitempty"`
	DOI         *string  `json:"doi,omitempty"`
	URL         *string  `json:"url,omitempty"`
	Type        string   `json:"type"`        // "book", "article", "website", "conference", etc.
	Context     string   `json:"context,omitempty"` // Text around citation
	Confidence  float64  `json:"confidence"`
	Page        *int     `json:"page,omitempty"`
}

// SentimentAnalysis contains sentiment analysis results.
type SentimentAnalysis struct {
	OverallSentiment string  `json:"overall_sentiment"` // "positive", "negative", "neutral", "mixed"
	Score            float64 `json:"score"`             // -1.0 to 1.0
	Confidence       float64 `json:"confidence"`
	Emotions         map[string]float64 `json:"emotions,omitempty"` // "joy", "sadness", "anger", etc.
}

// Table represents an extracted table.
type Table struct {
	Rows       [][]string `json:"rows"`
	Headers    []string   `json:"headers,omitempty"`
	Caption    string     `json:"caption,omitempty"`
	Page       int        `json:"page,omitempty"`
	RowCount   int        `json:"row_count"`
	ColumnCount int       `json:"column_count"`
}

// Formula represents a mathematical formula.
type Formula struct {
	Text        string  `json:"text"`        // Formula in LaTeX or plain text
	LaTeX       string  `json:"latex,omitempty"` // LaTeX representation
	Context     string  `json:"context,omitempty"` // Surrounding text
	Page        int     `json:"page,omitempty"`
	Type        string  `json:"type,omitempty"` // "equation", "inequality", "expression", etc.
}

// Dependency represents a code dependency.
type Dependency struct {
	Name        string   `json:"name"`        // Package/module name
	Version     string   `json:"version,omitempty"`
	Type        string   `json:"type"`        // "import", "require", "include", etc.
	Language    string   `json:"language"`   // "go", "python", "javascript", etc.
	Path        string   `json:"path,omitempty"` // File path where used
}

// DuplicateInfo contains information about duplicate documents.
type DuplicateInfo struct {
	DocumentID    DocumentID `json:"document_id"`
	RelativePath  string     `json:"relative_path"`
	Similarity    float64    `json:"similarity"`    // 0.0 to 1.0
	Type          string     `json:"type"`         // "exact", "near", "version"
	Reason        string     `json:"reason,omitempty"` // Why it's considered duplicate
}

// OCRResult contains OCR extraction results.
type OCRResult struct {
	Text        string  `json:"text"`        // Extracted text
	Confidence  float64 `json:"confidence"` // Average confidence
	Language    string  `json:"language,omitempty"`
	PageCount   int     `json:"page_count"`
	ExtractedAt time.Time `json:"extracted_at"`
}

// TranscriptionResult contains audio/video transcription results.
type TranscriptionResult struct {
	Text        string    `json:"text"`        // Transcribed text
	Language    string    `json:"language"`
	Duration    float64   `json:"duration"`     // Duration in seconds
	Confidence  float64   `json:"confidence"`
	Segments    []Segment `json:"segments,omitempty"` // Timestamped segments
	ExtractedAt time.Time `json:"extracted_at"`
}

// Segment represents a timestamped segment of transcription.
type Segment struct {
	Start   float64 `json:"start"`   // Start time in seconds
	End     float64 `json:"end"`      // End time in seconds
	Text    string  `json:"text"`     // Text for this segment
}




