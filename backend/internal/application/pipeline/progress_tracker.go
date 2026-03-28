package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
)

// FileProgress tracks the progress of a single file through the pipeline.
type FileProgress struct {
	FilePath     string
	FileID       entity.FileID
	Status       ProgressStatus
	CurrentStage string
	Stages       map[string]StageProgress
	StartedAt    time.Time
	CompletedAt  *time.Time
	Error        string
	Duration     time.Duration
	mu           sync.RWMutex
}

// StageProgress tracks progress for a single stage.
type StageProgress struct {
	Stage       string
	Status      ProgressStatus
	StartedAt   time.Time
	CompletedAt *time.Time
	Duration    time.Duration
	Error       string
}

// ProgressStatus represents the status of a file or stage.
type ProgressStatus string

const (
	ProgressStatusPending    ProgressStatus = "pending"
	ProgressStatusProcessing ProgressStatus = "processing"
	ProgressStatusCompleted  ProgressStatus = "completed"
	ProgressStatusSkipped    ProgressStatus = "skipped"
	ProgressStatusFailed     ProgressStatus = "failed"
)

// ProgressTracker tracks pipeline progress for all files.
type ProgressTracker struct {
	files   map[string]*FileProgress
	stages  []string // Ordered list of stage names
	mu      sync.RWMutex
	onUpdate func(*FileProgress) // Callback for updates
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(stages []string) *ProgressTracker {
	return &ProgressTracker{
		files:  make(map[string]*FileProgress),
		stages: stages,
	}
}

// SetUpdateCallback sets a callback that's called when file progress updates.
func (pt *ProgressTracker) SetUpdateCallback(callback func(*FileProgress)) {
	pt.mu.Lock()
	pt.onUpdate = callback
}

// OnEvent handles a pipeline event and updates progress.
func (pt *ProgressTracker) OnEvent(ctx context.Context, evt *event.Event) {
	if evt.Type != event.EventPipelineStarted &&
		evt.Type != event.EventPipelineProgress &&
		evt.Type != event.EventPipelineCompleted &&
		evt.Type != event.EventPipelineFailed {
		return
	}

	data, ok := evt.Data.(event.PipelineEventData)
	if !ok {
		return
	}

	pt.mu.Lock()

	filePath := data.FilePath
	progress, exists := pt.files[filePath]
	if !exists {
		progress = &FileProgress{
			FilePath:  filePath,
			Status:    ProgressStatusPending,
			Stages:    make(map[string]StageProgress),
			StartedAt: evt.Timestamp,
		}
		pt.files[filePath] = progress
	}

	progress.mu.Lock()

	switch evt.Type {
	case event.EventPipelineStarted:
		progress.Status = ProgressStatusProcessing
		progress.StartedAt = evt.Timestamp
		progress.CurrentStage = "start"

	case event.EventPipelineProgress:
		stage := data.Stage
		if stage == "" {
			stage = "unknown"
		}

		// Mark previous stage as completed
		if progress.CurrentStage != "" && progress.CurrentStage != stage {
			if prevStage, ok := progress.Stages[progress.CurrentStage]; ok {
				now := evt.Timestamp
				prevStage.CompletedAt = &now
				prevStage.Duration = now.Sub(prevStage.StartedAt)
				prevStage.Status = ProgressStatusCompleted
				progress.Stages[progress.CurrentStage] = prevStage
			}
		}

		// Start new stage
		stageProgress := StageProgress{
			Stage:     stage,
			Status:    ProgressStatusProcessing,
			StartedAt: evt.Timestamp,
		}
		progress.Stages[stage] = stageProgress
		progress.CurrentStage = stage

	case event.EventPipelineCompleted:
		// Mark current stage as completed
		if progress.CurrentStage != "" {
			if stage, ok := progress.Stages[progress.CurrentStage]; ok {
				now := evt.Timestamp
				stage.CompletedAt = &now
				stage.Duration = now.Sub(stage.StartedAt)
				stage.Status = ProgressStatusCompleted
				progress.Stages[progress.CurrentStage] = stage
			}
		}
		progress.Status = ProgressStatusCompleted
		now := evt.Timestamp
		progress.CompletedAt = &now
		progress.Duration = now.Sub(progress.StartedAt)

	case event.EventPipelineFailed:
		stage := data.Stage
		if stage == "" {
			stage = progress.CurrentStage
		}
		if stage == "" {
			stage = "unknown"
		}

		// Mark stage as failed
		if stageProgress, ok := progress.Stages[stage]; ok {
			now := evt.Timestamp
			stageProgress.CompletedAt = &now
			stageProgress.Duration = now.Sub(stageProgress.StartedAt)
			stageProgress.Status = ProgressStatusFailed
			if data.Error != nil {
				stageProgress.Error = *data.Error
			}
			progress.Stages[stage] = stageProgress
		}

		progress.Status = ProgressStatusFailed
		if data.Error != nil {
			progress.Error = *data.Error
		}
		now := evt.Timestamp
		progress.CompletedAt = &now
		progress.Duration = now.Sub(progress.StartedAt)
	}

	progressCopy := &FileProgress{
		FilePath:     progress.FilePath,
		FileID:       progress.FileID,
		Status:       progress.Status,
		CurrentStage: progress.CurrentStage,
		StartedAt:    progress.StartedAt,
		Error:        progress.Error,
		Duration:     progress.Duration,
		Stages:       make(map[string]StageProgress),
	}
	if progress.CompletedAt != nil {
		completedAt := *progress.CompletedAt
		progressCopy.CompletedAt = &completedAt
	}

	for k, v := range progress.Stages {
		stageCopy := v
		if v.CompletedAt != nil {
			completedAt := *v.CompletedAt
			stageCopy.CompletedAt = &completedAt
		}
		progressCopy.Stages[k] = stageCopy
	}

	callback := pt.onUpdate
	progress.mu.Unlock()
	pt.mu.Unlock()

	if callback != nil {
		callback(progressCopy)
	}
}

// GetProgress returns progress for a specific file.
func (pt *ProgressTracker) GetProgress(filePath string) (*FileProgress, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	progress, ok := pt.files[filePath]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid race conditions
	progress.mu.RLock()
	defer progress.mu.RUnlock()

	copy := &FileProgress{
		FilePath:     progress.FilePath,
		FileID:       progress.FileID,
		Status:       progress.Status,
		CurrentStage: progress.CurrentStage,
		StartedAt:    progress.StartedAt,
		Error:        progress.Error,
		Duration:     progress.Duration,
		Stages:       make(map[string]StageProgress),
	}
	if progress.CompletedAt != nil {
		completedAt := *progress.CompletedAt
		copy.CompletedAt = &completedAt
	}

	for k, v := range progress.Stages {
		stageCopy := v
		if v.CompletedAt != nil {
			completedAt := *v.CompletedAt
			stageCopy.CompletedAt = &completedAt
		}
		copy.Stages[k] = stageCopy
	}

	return copy, true
}

// GetAllProgress returns progress for all files.
func (pt *ProgressTracker) GetAllProgress() []*FileProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]*FileProgress, 0, len(pt.files))
	for _, progress := range pt.files {
		progress.mu.RLock()
		copy := &FileProgress{
			FilePath:     progress.FilePath,
			FileID:       progress.FileID,
			Status:       progress.Status,
			CurrentStage: progress.CurrentStage,
			StartedAt:    progress.StartedAt,
			Error:        progress.Error,
			Duration:     progress.Duration,
			Stages:       make(map[string]StageProgress),
		}
		if progress.CompletedAt != nil {
			completedAt := *progress.CompletedAt
			copy.CompletedAt = &completedAt
		}
		for k, v := range progress.Stages {
			stageCopy := v
			if v.CompletedAt != nil {
				completedAt := *v.CompletedAt
				stageCopy.CompletedAt = &completedAt
			}
			copy.Stages[k] = stageCopy
		}
		progress.mu.RUnlock()
		result = append(result, copy)
	}

	return result
}

// GetStages returns the ordered list of stage names.
func (pt *ProgressTracker) GetStages() []string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return append([]string{}, pt.stages...)
}

// Clear clears all progress data.
func (pt *ProgressTracker) Clear() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.files = make(map[string]*FileProgress)
}

// GetStats returns aggregate statistics.
func (pt *ProgressTracker) GetStats() ProgressStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats := ProgressStats{}
	for _, progress := range pt.files {
		progress.mu.RLock()
		stats.Total++
		switch progress.Status {
		case ProgressStatusPending:
			stats.Pending++
		case ProgressStatusProcessing:
			stats.Processing++
		case ProgressStatusCompleted:
			stats.Completed++
		case ProgressStatusFailed:
			stats.Failed++
		}
		progress.mu.RUnlock()
	}
	return stats
}

// ProgressStats contains aggregate statistics.
type ProgressStats struct {
	Total      int
	Pending    int
	Processing int
	Completed  int
	Failed     int
}

// ProgressSnapshot contains a snapshot of all pipeline progress.
type ProgressSnapshot struct {
	Files  []*FileProgress
	Stages []string
	Stats  ProgressStats
}
