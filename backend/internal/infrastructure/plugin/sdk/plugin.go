// Package sdk provides the plugin SDK for Cortex.
package sdk

import (
	"context"
)

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	// Info returns plugin metadata.
	Info() PluginInfo

	// Init initializes the plugin with configuration.
	Init(ctx context.Context, config PluginConfig) error

	// Start starts the plugin (called after Init).
	Start(ctx context.Context) error

	// Stop gracefully stops the plugin.
	Stop(ctx context.Context) error

	// Health returns the health status of the plugin.
	Health(ctx context.Context) HealthStatus
}

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	ID           string
	Name         string
	Version      string
	Author       string
	Description  string
	Type         PluginType
	Capabilities []string
}

// PluginType represents the type of plugin.
type PluginType string

const (
	PluginTypeIndexer   PluginType = "indexer"
	PluginTypeProcessor PluginType = "processor"
	PluginTypeHook      PluginType = "hook"
	PluginTypeLLM       PluginType = "llm_provider"
)

// PluginConfig contains plugin configuration.
type PluginConfig struct {
	Settings map[string]interface{}
	DataDir  string
	Logger   Logger
}

// HealthStatus contains plugin health information.
type HealthStatus struct {
	Healthy bool
	Message string
	Details map[string]interface{}
}

// Logger is the logging interface for plugins.
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// IndexerPlugin defines the interface for custom indexer plugins.
type IndexerPlugin interface {
	Plugin

	// CanHandle returns true if this plugin can process the given file.
	CanHandle(ctx context.Context, file FileInfo) bool

	// Extract extracts custom metadata from the file.
	Extract(ctx context.Context, file FileInfo) (*IndexerResult, error)

	// GetPriority returns the priority for this indexer (higher = earlier execution).
	GetPriority() int
}

// FileInfo contains file information for indexer plugins.
type FileInfo struct {
	RelativePath string
	AbsolutePath string
	Extension    string
	Size         int64
	MimeType     string
}

// IndexerResult contains the result of indexer extraction.
type IndexerResult struct {
	// CustomMetadata is stored as JSON in the database.
	CustomMetadata map[string]interface{}

	// SuggestedTags are tags suggested by this indexer.
	SuggestedTags []string

	// SuggestedContexts are contexts suggested by this indexer.
	SuggestedContexts []string

	// ExtractedText is text content for search indexing.
	ExtractedText *string

	// Warnings encountered during extraction (non-fatal).
	Warnings []string
}

// ProcessorPlugin defines the interface for pipeline stage plugins.
type ProcessorPlugin interface {
	Plugin

	// Stage returns which pipeline stage this processor runs in.
	Stage() PipelineStage

	// Process transforms the file entry, potentially modifying its metadata.
	Process(ctx context.Context, file FileInfo, input ProcessorInput) (*ProcessorOutput, error)
}

// PipelineStage represents a stage in the processing pipeline.
type PipelineStage string

const (
	StagePreBasic     PipelineStage = "pre_basic"
	StagePostBasic    PipelineStage = "post_basic"
	StagePreMime      PipelineStage = "pre_mime"
	StagePostMime     PipelineStage = "post_mime"
	StagePreCode      PipelineStage = "pre_code"
	StagePostCode     PipelineStage = "post_code"
	StagePreDocument  PipelineStage = "pre_document"
	StagePostDocument PipelineStage = "post_document"
	StagePreAI        PipelineStage = "pre_ai"
	StagePostAI       PipelineStage = "post_ai"
	StageFinal        PipelineStage = "final"
)

// ProcessorInput contains input for processor plugins.
type ProcessorInput struct {
	FileContent   []byte
	PreviousStage *ProcessorOutput
	Metadata      map[string]interface{}
}

// ProcessorOutput contains the result of processor execution.
type ProcessorOutput struct {
	Metadata    map[string]interface{}
	CustomData  map[string]interface{}
	Skip        bool   // Skip remaining processors in this stage
	Abort       bool   // Abort entire pipeline for this file
	AbortReason string
}

// HookPlugin defines the interface for event hook plugins.
type HookPlugin interface {
	Plugin

	// SubscribedEvents returns the list of events this hook wants to receive.
	SubscribedEvents() []EventType

	// OnEvent is called when a subscribed event occurs.
	OnEvent(ctx context.Context, evt Event) error

	// IsAsync returns true if this hook should be called asynchronously.
	IsAsync() bool
}

// EventType represents an event type for hooks.
type EventType string

const (
	EventFileCreated        EventType = "file.created"
	EventFileModified       EventType = "file.modified"
	EventFileDeleted        EventType = "file.deleted"
	EventTagAdded           EventType = "metadata.tag.added"
	EventTagRemoved         EventType = "metadata.tag.removed"
	EventContextAdded       EventType = "metadata.context.added"
	EventContextRemoved     EventType = "metadata.context.removed"
	EventIndexingStarted    EventType = "indexing.started"
	EventIndexingCompleted  EventType = "indexing.completed"
	EventScanStarted        EventType = "scan.started"
	EventScanCompleted      EventType = "scan.completed"
	EventPipelineStarted    EventType = "pipeline.started"
	EventPipelineCompleted  EventType = "pipeline.completed"
)

// Event contains event data for hook plugins.
type Event struct {
	Type        EventType
	WorkspaceID string
	Data        interface{}
}
