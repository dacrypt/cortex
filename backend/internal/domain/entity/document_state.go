package entity

import (
	"time"
)

// DocumentState represents the lifecycle state of a document.
type DocumentState string

const (
	// DocumentStateDraft indicates a document is being worked on.
	DocumentStateDraft DocumentState = "draft"
	// DocumentStateActive indicates a document is the current valid version.
	DocumentStateActive DocumentState = "active"
	// DocumentStateReplaced indicates a document has been superseded by another.
	DocumentStateReplaced DocumentState = "replaced"
	// DocumentStateArchived indicates a document is archived and no longer actively used.
	DocumentStateArchived DocumentState = "archived"
)

// IsValid returns true if the state is a valid document state.
func (s DocumentState) IsValid() bool {
	return s == DocumentStateDraft ||
		s == DocumentStateActive ||
		s == DocumentStateReplaced ||
		s == DocumentStateArchived
}

// String returns the string representation of DocumentState.
func (s DocumentState) String() string {
	return string(s)
}

// DocumentStateTransition represents a state change in a document's lifecycle.
type DocumentStateTransition struct {
	ID        string
	DocumentID DocumentID
	FromState  *DocumentState // Nil for initial state
	ToState    DocumentState
	Reason     string
	ChangedBy  string // Optional: user/system identifier
	ChangedAt  time.Time
}

// NewDocumentStateTransition creates a new state transition.
func NewDocumentStateTransition(
	docID DocumentID,
	fromState *DocumentState,
	toState DocumentState,
	reason string,
) *DocumentStateTransition {
	return &DocumentStateTransition{
		DocumentID: docID,
		FromState:  fromState,
		ToState:    toState,
		Reason:     reason,
		ChangedAt:  time.Now(),
	}
}

// WithChangedBy sets the entity that changed the state.
func (t *DocumentStateTransition) WithChangedBy(changedBy string) *DocumentStateTransition {
	t.ChangedBy = changedBy
	return t
}

