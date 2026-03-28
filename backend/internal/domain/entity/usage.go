package entity

import (
	"time"

	"github.com/google/uuid"
)

// UsageEventID uniquely identifies a usage event.
type UsageEventID string

// NewUsageEventID creates a new unique UsageEventID.
func NewUsageEventID() UsageEventID {
	return UsageEventID(uuid.New().String())
}

// String returns the string representation of UsageEventID.
func (id UsageEventID) String() string {
	return string(id)
}

// UsageEventType represents the type of usage event.
type UsageEventType string

const (
	// UsageEventOpened indicates a document was opened/viewed.
	UsageEventOpened UsageEventType = "opened"
	// UsageEventEdited indicates a document was edited.
	UsageEventEdited UsageEventType = "edited"
	// UsageEventSearched indicates a document was found via search.
	UsageEventSearched UsageEventType = "searched"
	// UsageEventReferenced indicates a document was referenced (linked to).
	UsageEventReferenced UsageEventType = "referenced"
	// UsageEventIndexed indicates a document was indexed/processed.
	UsageEventIndexed UsageEventType = "indexed"
)

// IsValid returns true if the event type is valid.
func (et UsageEventType) IsValid() bool {
	return et == UsageEventOpened ||
		et == UsageEventEdited ||
		et == UsageEventSearched ||
		et == UsageEventReferenced ||
		et == UsageEventIndexed
}

// String returns the string representation of UsageEventType.
func (et UsageEventType) String() string {
	return string(et)
}

// DocumentUsageEvent represents a single usage event for a document.
type DocumentUsageEvent struct {
	ID         UsageEventID
	WorkspaceID WorkspaceID
	DocumentID  DocumentID
	EventType   UsageEventType
	Context     string // e.g., "project:auth-refactor", "search:login"
	Metadata    map[string]interface{} // Optional event metadata
	Timestamp   time.Time
}

// NewDocumentUsageEvent creates a new usage event.
func NewDocumentUsageEvent(
	workspaceID WorkspaceID,
	docID DocumentID,
	eventType UsageEventType,
) *DocumentUsageEvent {
	return &DocumentUsageEvent{
		ID:          NewUsageEventID(),
		WorkspaceID: workspaceID,
		DocumentID:  docID,
		EventType:   eventType,
		Timestamp:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
}

// WithContext sets the event context.
func (e *DocumentUsageEvent) WithContext(context string) *DocumentUsageEvent {
	e.Context = context
	return e
}

// WithMetadata sets event metadata.
func (e *DocumentUsageEvent) WithMetadata(key string, value interface{}) *DocumentUsageEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// DocumentUsageStats aggregates usage statistics for a document.
type DocumentUsageStats struct {
	DocumentID    DocumentID
	AccessCount   int
	LastAccessed  time.Time
	FirstAccessed time.Time
	CoOccurrences map[DocumentID]int // Documents used together (count)
	Frequency     float64             // Accesses per day
}

// NewDocumentUsageStats creates new usage statistics.
func NewDocumentUsageStats(docID DocumentID) *DocumentUsageStats {
	return &DocumentUsageStats{
		DocumentID:    docID,
		CoOccurrences: make(map[DocumentID]int),
	}
}

