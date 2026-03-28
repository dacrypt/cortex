// Package dto provides Data Transfer Objects for the application layer.
package dto

import (
	"time"
)

// ScanRequest is the request for scanning a workspace.
type ScanRequest struct {
	WorkspaceID string
	Path        string
	Incremental bool
}

// ScanProgress represents scan progress.
type ScanProgress struct {
	Phase          string
	FilesDiscovered int
	FilesProcessed  int
	CurrentFile     string
	Errors          []string
}

// FileDTO represents a file in API responses.
type FileDTO struct {
	ID           string             `json:"id"`
	RelativePath string             `json:"relative_path"`
	AbsolutePath string             `json:"absolute_path"`
	Filename     string             `json:"filename"`
	Extension    string             `json:"extension"`
	FileSize     int64              `json:"file_size"`
	LastModified time.Time          `json:"last_modified"`
	MimeType     string             `json:"mime_type,omitempty"`
	FileType     string             `json:"file_type,omitempty"`
	Category     string             `json:"category,omitempty"`
	LineCount    int                `json:"line_count,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Contexts     []string           `json:"contexts,omitempty"`
}

// MetadataDTO represents file metadata in API responses.
type MetadataDTO struct {
	FileID            string    `json:"file_id"`
	RelativePath      string    `json:"relative_path"`
	Tags              []string  `json:"tags"`
	Contexts          []string  `json:"contexts"`
	SuggestedContexts []string  `json:"suggested_contexts,omitempty"`
	Type              string    `json:"type"`
	Notes             *string   `json:"notes,omitempty"`
	AISummary         *string   `json:"ai_summary,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TagCountDTO represents a tag with its count.
type TagCountDTO struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ContextCountDTO represents a context with its count.
type ContextCountDTO struct {
	Context string `json:"context"`
	Count   int    `json:"count"`
}

// TaskDTO represents a task in API responses.
type TaskDTO struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	Progress    int        `json:"progress"`
	ProgressMax int        `json:"progress_max"`
	Message     string     `json:"message,omitempty"`
	Error       *string    `json:"error,omitempty"`
	WorkspaceID string     `json:"workspace_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// QueueStatsDTO represents task queue statistics.
type QueueStatsDTO struct {
	Pending   int `json:"pending"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Total     int `json:"total"`
}

// ScheduledTaskDTO represents a scheduled task.
type ScheduledTaskDTO struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	TaskType       string     `json:"task_type"`
	Enabled        bool       `json:"enabled"`
	NextRun        *time.Time `json:"next_run,omitempty"`
	LastRun        *time.Time `json:"last_run,omitempty"`
}

// ProviderDTO represents an LLM provider.
type ProviderDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// ModelDTO represents an LLM model.
type ModelDTO struct {
	Name          string   `json:"name"`
	Size          int64    `json:"size,omitempty"`
	ContextLength int64    `json:"context_length,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
}

// WorkspaceDTO represents a workspace.
type WorkspaceDTO struct {
	ID          string     `json:"id"`
	Path        string     `json:"path"`
	Name        string     `json:"name"`
	Active      bool       `json:"active"`
	LastIndexed *time.Time `json:"last_indexed,omitempty"`
}

// PluginDTO represents a loaded plugin.
type PluginDTO struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Type         string   `json:"type"`
	Author       string   `json:"author,omitempty"`
	Description  string   `json:"description,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Healthy      bool     `json:"healthy"`
}

// StatusDTO represents daemon status.
type StatusDTO struct {
	Version    string         `json:"version"`
	Uptime     time.Duration  `json:"uptime"`
	Workspaces int            `json:"workspaces"`
	Files      int            `json:"files"`
	Tasks      QueueStatsDTO  `json:"tasks"`
	LLMActive  bool           `json:"llm_active"`
	Plugins    int            `json:"plugins"`
}

// HealthDTO represents health check response.
type HealthDTO struct {
	Status    string            `json:"status"` // healthy, degraded, unhealthy
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// CreateTaskRequest is the request to create a task.
type CreateTaskRequest struct {
	Type        string                 `json:"type"`
	Priority    int                    `json:"priority"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
}

// ScheduleTaskRequest is the request to schedule a task.
type ScheduleTaskRequest struct {
	Name           string                 `json:"name"`
	CronExpression string                 `json:"cron_expression"`
	TaskType       string                 `json:"task_type"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	Enabled        bool                   `json:"enabled"`
}

// SuggestTagsRequest is the request for AI tag suggestions.
type SuggestTagsRequest struct {
	FileID  string `json:"file_id,omitempty"`
	Content string `json:"content,omitempty"`
	MaxTags int    `json:"max_tags,omitempty"`
}

// SuggestProjectRequest is the request for AI project suggestions.
type SuggestProjectRequest struct {
	FileID           string   `json:"file_id,omitempty"`
	Content          string   `json:"content,omitempty"`
	ExistingProjects []string `json:"existing_projects,omitempty"`
}

// GenerateSummaryRequest is the request for AI summary generation.
type GenerateSummaryRequest struct {
	FileID    string `json:"file_id,omitempty"`
	Content   string `json:"content,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
}

// CompletionRequest is the request for raw LLM completion.
type CompletionRequest struct {
	Prompt      string  `json:"prompt"`
	Model       string  `json:"model,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// CompletionResponse is the response from LLM completion.
type CompletionResponse struct {
	Text             string `json:"text"`
	TokensUsed       int    `json:"tokens_used"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	ProcessingTimeMs int64  `json:"processing_time_ms"`
}
