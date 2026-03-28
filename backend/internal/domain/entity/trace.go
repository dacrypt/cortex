package entity

import "time"

// ProcessingTrace captures detailed processing metadata for a file.
type ProcessingTrace struct {
	WorkspaceID   WorkspaceID
	FileID        FileID
	RelativePath  string
	Stage         string
	Operation     string
	PromptPath    string
	OutputPath    string
	PromptPreview string
	OutputPreview string
	Model         string
	TokensUsed    int
	DurationMs    int64
	Error         *string
	CreatedAt     time.Time
}
