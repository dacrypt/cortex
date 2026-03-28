// Package handlers provides gRPC service implementations.
package handlers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/scheduler"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/queue"
)

// TaskHandler handles task-related gRPC requests.
type TaskHandler struct {
	taskRepo   repository.TaskRepository
	workerPool *queue.WorkerPool
	scheduler  *scheduler.Scheduler
	logger     zerolog.Logger
}

// NewTaskHandler creates a new task handler.
func NewTaskHandler(
	taskRepo repository.TaskRepository,
	workerPool *queue.WorkerPool,
	scheduler *scheduler.Scheduler,
	logger zerolog.Logger,
) *TaskHandler {
	return &TaskHandler{
		taskRepo:   taskRepo,
		workerPool: workerPool,
		scheduler:  scheduler,
		logger:     logger.With().Str("handler", "task").Logger(),
	}
}

// CreateTask creates a new task.
func (h *TaskHandler) CreateTask(ctx context.Context, taskType entity.TaskType, priority entity.TaskPriority, payload []byte) (*entity.Task, error) {
	task := entity.NewTask(taskType, priority, payload)

	if err := h.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	// Submit to worker pool
	if err := h.workerPool.Submit(ctx, task); err != nil {
		h.logger.Warn().
			Err(err).
			Str("task_id", string(task.ID)).
			Msg("Failed to submit task to worker pool")
	}

	h.logger.Debug().
		Str("task_id", string(task.ID)).
		Str("type", string(taskType)).
		Msg("Task created")

	return task, nil
}

// GetTask retrieves a task by ID.
func (h *TaskHandler) GetTask(ctx context.Context, id entity.TaskID) (*entity.Task, error) {
	return h.taskRepo.Get(ctx, id)
}

// ListTasks lists tasks with optional filtering.
func (h *TaskHandler) ListTasks(ctx context.Context, opts ListTasksOptions) ([]*entity.Task, error) {
	listOpts := repository.DefaultTaskListOptions()
	if opts.Limit > 0 {
		listOpts.Limit = opts.Limit
	}

	if opts.Status != "" {
		return h.taskRepo.ListByStatus(ctx, opts.Status, listOpts)
	}
	if opts.WorkspaceID != "" {
		return h.taskRepo.ListByWorkspace(ctx, entity.WorkspaceID(opts.WorkspaceID), listOpts)
	}

	// List all pending tasks by default
	return h.taskRepo.ListByStatus(ctx, entity.TaskStatusPending, listOpts)
}

// CancelTask cancels a task.
func (h *TaskHandler) CancelTask(ctx context.Context, id entity.TaskID) error {
	task, err := h.taskRepo.Get(ctx, id)
	if err != nil {
		return err
	}

	task.Status = entity.TaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now

	return h.taskRepo.Update(ctx, task)
}

// RetryTask retries a failed task.
func (h *TaskHandler) RetryTask(ctx context.Context, id entity.TaskID) (*entity.Task, error) {
	task, err := h.taskRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Reset status
	task.Status = entity.TaskStatusPending
	task.Error = nil
	task.StartedAt = nil
	task.CompletedAt = nil
	task.Progress = entity.TaskProgress{}

	if err := h.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}

	// Resubmit to worker pool
	if err := h.workerPool.Submit(ctx, task); err != nil {
		h.logger.Warn().
			Err(err).
			Str("task_id", string(task.ID)).
			Msg("Failed to resubmit task to worker pool")
	}

	return task, nil
}

// GetQueueStats returns task queue statistics.
func (h *TaskHandler) GetQueueStats(ctx context.Context) (*entity.TaskStats, error) {
	return h.taskRepo.GetStats(ctx)
}

// PauseQueue pauses task processing.
func (h *TaskHandler) PauseQueue() {
	_ = h.workerPool.Pause(context.Background())
	h.logger.Info().Msg("Task queue paused")
}

// ResumeQueue resumes task processing.
func (h *TaskHandler) ResumeQueue() {
	_ = h.workerPool.Resume(context.Background())
	h.logger.Info().Msg("Task queue resumed")
}

// DrainQueue waits for all tasks to complete.
func (h *TaskHandler) DrainQueue(ctx context.Context) error {
	return h.workerPool.Drain(ctx)
}

// ScheduleTask creates a scheduled task.
func (h *TaskHandler) ScheduleTask(ctx context.Context, req ScheduleTaskRequest) (*entity.ScheduledTask, error) {
	// Validate cron expression
	if _, err := scheduler.ParseCronExpression(req.CronExpression); err != nil {
		return nil, err
	}

	// Calculate next run time
	nextRun, err := scheduler.NextRunTime(req.CronExpression)
	if err != nil {
		return nil, err
	}

	task := entity.NewScheduledTask(req.Name, req.CronExpression, req.TaskType, req.Payload)
	task.Enabled = req.Enabled
	task.NextRun = nextRun
	if req.WorkspaceID != "" {
		workspaceID := entity.WorkspaceID(req.WorkspaceID)
		task.WorkspaceID = &workspaceID
	}

	// Add to scheduler
	if err := h.scheduler.AddTask(task); err != nil {
		return nil, err
	}

	h.logger.Info().
		Str("task_id", task.ID).
		Str("name", task.Name).
		Str("cron", task.CronExpression).
		Msg("Scheduled task created")

	return task, nil
}

// ListScheduledTasks lists all scheduled tasks.
func (h *TaskHandler) ListScheduledTasks() []*entity.ScheduledTask {
	return h.scheduler.ListTasks()
}

// CancelScheduledTask removes a scheduled task.
func (h *TaskHandler) CancelScheduledTask(ctx context.Context, id string) error {
	return h.scheduler.RemoveTask(id)
}

// UpdateScheduledTask updates a scheduled task.
func (h *TaskHandler) UpdateScheduledTask(ctx context.Context, task *entity.ScheduledTask) error {
	// Remove old and add new
	if err := h.scheduler.RemoveTask(task.ID); err != nil {
		return err
	}
	return h.scheduler.AddTask(task)
}

// CleanupOldTasks removes completed tasks older than the specified duration.
func (h *TaskHandler) CleanupOldTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	count, err := h.taskRepo.PurgeCompleted(ctx, olderThan)
	return int64(count), err
}

// ListTasksOptions contains options for listing tasks.
type ListTasksOptions struct {
	Status      entity.TaskStatus
	WorkspaceID string
	Limit       int
}

// ScheduleTaskRequest is the request to schedule a task.
type ScheduleTaskRequest struct {
	Name           string
	CronExpression string
	TaskType       entity.TaskType
	Payload        []byte
	WorkspaceID    string
	Enabled        bool
}
