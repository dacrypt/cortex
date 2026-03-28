// Package event contains domain event definitions.
package event

import (
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// EventType represents the type of domain event.
type EventType string

const (
	// File events
	EventFileCreated  EventType = "file.created"
	EventFileModified EventType = "file.modified"
	EventFileDeleted  EventType = "file.deleted"
	EventFileRenamed  EventType = "file.renamed"

	// Metadata events
	EventTagAdded          EventType = "metadata.tag.added"
	EventTagRemoved        EventType = "metadata.tag.removed"
	EventContextAdded      EventType = "metadata.context.added"
	EventContextRemoved    EventType = "metadata.context.removed"
	EventNotesUpdated      EventType = "metadata.notes.updated"
	EventSuggestionAdded   EventType = "metadata.suggestion.added"
	EventSuggestionRemoved EventType = "metadata.suggestion.removed"

	// Indexing events
	EventIndexingStarted   EventType = "indexing.started"
	EventIndexingProgress  EventType = "indexing.progress"
	EventIndexingCompleted EventType = "indexing.completed"
	EventIndexingFailed    EventType = "indexing.failed"

	// Scan events
	EventScanStarted   EventType = "scan.started"
	EventScanProgress  EventType = "scan.progress"
	EventScanCompleted EventType = "scan.completed"
	EventScanFailed    EventType = "scan.failed"

	// Pipeline events
	EventPipelineStarted   EventType = "pipeline.started"
	EventPipelineProgress  EventType = "pipeline.progress"
	EventPipelineCompleted EventType = "pipeline.completed"
	EventPipelineFailed    EventType = "pipeline.failed"

	// Task events
	EventTaskCreated   EventType = "task.created"
	EventTaskStarted   EventType = "task.started"
	EventTaskProgress  EventType = "task.progress"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskCancelled EventType = "task.cancelled"

	// LLM events
	EventLLMRequestStarted   EventType = "llm.request.started"
	EventLLMRequestCompleted EventType = "llm.request.completed"
	EventLLMRequestFailed    EventType = "llm.request.failed"

	// Workspace events
	EventWorkspaceRegistered   EventType = "workspace.registered"
	EventWorkspaceUnregistered EventType = "workspace.unregistered"
	EventWorkspaceActivated    EventType = "workspace.activated"
	EventWorkspaceDeactivated  EventType = "workspace.deactivated"

	// Plugin events
	EventPluginLoaded   EventType = "plugin.loaded"
	EventPluginUnloaded EventType = "plugin.unloaded"
	EventPluginError    EventType = "plugin.error"
)

// Event represents a domain event.
type Event struct {
	ID          string
	Type        EventType
	WorkspaceID *entity.WorkspaceID
	Timestamp   time.Time
	Data        interface{}
}

// NewEvent creates a new event.
func NewEvent(eventType EventType, data interface{}) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// WithWorkspace sets the workspace ID for the event.
func (e *Event) WithWorkspace(id entity.WorkspaceID) *Event {
	e.WorkspaceID = &id
	return e
}

// FileEventData contains data for file-related events.
type FileEventData struct {
	FileID       entity.FileID
	RelativePath string
	OldPath      *string // For rename events
}

// TagEventData contains data for tag-related events.
type TagEventData struct {
	FileID       entity.FileID
	RelativePath string
	Tag          string
}

// ContextEventData contains data for context-related events.
type ContextEventData struct {
	FileID       entity.FileID
	RelativePath string
	Context      string
}

// IndexingEventData contains data for indexing-related events.
type IndexingEventData struct {
	Phase      string
	Processed  int
	Total      int
	Percentage float64
	Message    string
	Error      *string
}

// ScanEventData contains data for scan-related events.
type ScanEventData struct {
	WorkspacePath string
	FilesScanned  int
	FilesTotal    int
	CurrentPath   string
	Percentage    float64
	Error         *string
}

// TaskEventData contains data for task-related events.
type TaskEventData struct {
	TaskID   entity.TaskID
	TaskType entity.TaskType
	Status   entity.TaskStatus
	Progress *entity.TaskProgress
	Error    *string
}

// LLMEventData contains data for LLM-related events.
type LLMEventData struct {
	Provider      string
	Model         string
	Operation     string
	ProcessingMs  int64
	TokensUsed    int
	Error         *string
}

// WorkspaceEventData contains data for workspace-related events.
type WorkspaceEventData struct {
	WorkspaceID   entity.WorkspaceID
	WorkspacePath string
	WorkspaceName string
}

// PluginEventData contains data for plugin-related events.
type PluginEventData struct {
	PluginID   string
	PluginName string
	Error      *string
}

// PipelineEventData contains data for pipeline-related events.
type PipelineEventData struct {
	FilePath string
	Stage    string
	Error    *string
}

// MetadataEventData contains data for metadata-related events.
type MetadataEventData struct {
	FileID   string
	FilePath string
	Tag      string
	Context  string
}

var eventCounter uint64

func generateEventID() string {
	eventCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune(eventCounter))
}
