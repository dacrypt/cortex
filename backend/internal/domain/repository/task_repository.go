package repository

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TaskRepository defines the interface for task persistence.
type TaskRepository interface {
	// CRUD operations
	Create(ctx context.Context, task *entity.Task) error
	Get(ctx context.Context, id entity.TaskID) (*entity.Task, error)
	Update(ctx context.Context, task *entity.Task) error
	Delete(ctx context.Context, id entity.TaskID) error

	// Queue operations
	Enqueue(ctx context.Context, task *entity.Task) error
	Dequeue(ctx context.Context) (*entity.Task, error)
	DequeueByType(ctx context.Context, taskType entity.TaskType) (*entity.Task, error)
	DequeueByPriority(ctx context.Context, minPriority entity.TaskPriority) (*entity.Task, error)
	Peek(ctx context.Context, limit int) ([]*entity.Task, error)

	// Status management
	UpdateStatus(ctx context.Context, id entity.TaskID, status entity.TaskStatus, err *string) error
	UpdateProgress(ctx context.Context, id entity.TaskID, progress entity.TaskProgress) error

	// Queries
	List(ctx context.Context, opts TaskListOptions) ([]*entity.Task, error)
	ListByStatus(ctx context.Context, status entity.TaskStatus, opts TaskListOptions) ([]*entity.Task, error)
	ListByType(ctx context.Context, taskType entity.TaskType, opts TaskListOptions) ([]*entity.Task, error)
	ListByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID, opts TaskListOptions) ([]*entity.Task, error)
	GetStats(ctx context.Context) (*entity.TaskStats, error)
	GetStatsByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.TaskStats, error)

	// Cleanup
	PurgeCompleted(ctx context.Context, olderThan time.Duration) (int, error)
	PurgeFailed(ctx context.Context, olderThan time.Duration) (int, error)
	PurgeCancelled(ctx context.Context, olderThan time.Duration) (int, error)

	// Retry management
	GetRetryableTasks(ctx context.Context, limit int) ([]*entity.Task, error)
	ResetStuckTasks(ctx context.Context, stuckDuration time.Duration) (int, error)
}

// TaskListOptions contains options for listing tasks.
type TaskListOptions struct {
	WorkspaceID *entity.WorkspaceID
	Status      *entity.TaskStatus
	Type        *entity.TaskType
	Offset      int
	Limit       int
}

// DefaultTaskListOptions returns default task list options.
func DefaultTaskListOptions() TaskListOptions {
	return TaskListOptions{
		Offset: 0,
		Limit:  100,
	}
}

// ScheduledTaskRepository defines the interface for scheduled task persistence.
type ScheduledTaskRepository interface {
	// CRUD operations
	Create(ctx context.Context, task *entity.ScheduledTask) error
	Get(ctx context.Context, id string) (*entity.ScheduledTask, error)
	Update(ctx context.Context, task *entity.ScheduledTask) error
	Delete(ctx context.Context, id string) error

	// Queries
	List(ctx context.Context, opts ScheduledTaskListOptions) ([]*entity.ScheduledTask, error)
	ListEnabled(ctx context.Context) ([]*entity.ScheduledTask, error)
	ListByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.ScheduledTask, error)

	// Scheduling
	GetDueTasks(ctx context.Context, now time.Time) ([]*entity.ScheduledTask, error)
	UpdateNextRun(ctx context.Context, id string, nextRun time.Time) error
	UpdateLastRun(ctx context.Context, id string, lastRun time.Time) error
}

// ScheduledTaskListOptions contains options for listing scheduled tasks.
type ScheduledTaskListOptions struct {
	WorkspaceID *entity.WorkspaceID
	EnabledOnly bool
	Offset      int
	Limit       int
}
