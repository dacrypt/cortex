package entity

import (
	"time"

	"github.com/google/uuid"
)

// TaskID is a unique identifier for a task.
type TaskID string

// NewTaskID creates a new unique TaskID.
func NewTaskID() TaskID {
	return TaskID(uuid.New().String())
}

// String returns the string representation of TaskID.
func (id TaskID) String() string {
	return string(id)
}

// Task represents a unit of work in the queue.
type Task struct {
	ID          TaskID
	Type        TaskType
	Status      TaskStatus
	Priority    TaskPriority
	Payload     []byte
	Result      []byte
	Error       *string
	RetryCount  int
	MaxRetries  int
	Progress    TaskProgress
	WorkspaceID *WorkspaceID
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// NewTask creates a new task with the given type and priority.
func NewTask(taskType TaskType, priority TaskPriority, payload []byte) *Task {
	return &Task{
		ID:         NewTaskID(),
		Type:       taskType,
		Status:     TaskStatusPending,
		Priority:   priority,
		Payload:    payload,
		MaxRetries: 3,
		Progress:   TaskProgress{},
		CreatedAt:  time.Now(),
	}
}

// Start marks the task as running.
func (t *Task) Start() {
	now := time.Now()
	t.Status = TaskStatusRunning
	t.StartedAt = &now
}

// Complete marks the task as completed with a result.
func (t *Task) Complete(result []byte) {
	now := time.Now()
	t.Status = TaskStatusCompleted
	t.Result = result
	t.CompletedAt = &now
}

// Fail marks the task as failed with an error.
func (t *Task) Fail(err string) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.Error = &err
	t.CompletedAt = &now
}

// Retry increments retry count and resets for retry.
func (t *Task) Retry() bool {
	if t.RetryCount >= t.MaxRetries {
		return false
	}
	t.RetryCount++
	t.Status = TaskStatusRetrying
	t.StartedAt = nil
	t.CompletedAt = nil
	t.Error = nil
	return true
}

// Cancel marks the task as cancelled.
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = TaskStatusCancelled
	t.CompletedAt = &now
}

// CanRetry returns true if the task can be retried.
func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetries
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusCancelled
}

// TaskType enumerates the different types of tasks.
type TaskType string

const (
	TaskTypeScanWorkspace   TaskType = "scan_workspace"
	TaskTypeIndexFile       TaskType = "index_file"
	TaskTypeExtractMetadata TaskType = "extract_metadata"
	TaskTypeExtractDocument TaskType = "extract_document"
	TaskTypeAnalyzeCode     TaskType = "analyze_code"
	TaskTypeGenerateSummary TaskType = "generate_summary"
	TaskTypeSuggestTags     TaskType = "suggest_tags"
	TaskTypeSuggestProject  TaskType = "suggest_project"
	TaskTypeAutoAssign      TaskType = "auto_assign"
	TaskTypePlugin          TaskType = "plugin"
)

// TaskStatus enumerates the different states of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusRetrying  TaskStatus = "retrying"
)

// TaskPriority enumerates task priorities.
type TaskPriority int

const (
	TaskPriorityLow      TaskPriority = 0
	TaskPriorityNormal   TaskPriority = 1
	TaskPriorityHigh     TaskPriority = 2
	TaskPriorityCritical TaskPriority = 3
)

// TaskProgress tracks task execution progress.
type TaskProgress struct {
	Processed  int
	Total      int
	Message    string
	Percentage float64
}

// Update updates the progress with new values.
func (p *TaskProgress) Update(processed, total int, message string) {
	p.Processed = processed
	p.Total = total
	p.Message = message
	if total > 0 {
		p.Percentage = float64(processed) / float64(total) * 100
	}
}

// ScheduledTask represents a recurring task.
type ScheduledTask struct {
	ID             string
	Name           string
	CronExpression string
	TaskType       TaskType
	TaskPayload    []byte
	Enabled        bool
	NextRun        *time.Time
	LastRun        *time.Time
	WorkspaceID    *WorkspaceID
}

// NewScheduledTask creates a new scheduled task.
func NewScheduledTask(name, cronExpr string, taskType TaskType, payload []byte) *ScheduledTask {
	return &ScheduledTask{
		ID:             uuid.New().String(),
		Name:           name,
		CronExpression: cronExpr,
		TaskType:       taskType,
		TaskPayload:    payload,
		Enabled:        true,
	}
}

// TaskStats contains queue statistics.
type TaskStats struct {
	Pending   int
	Queued    int
	Running   int
	Completed int
	Failed    int
	Cancelled int
}
