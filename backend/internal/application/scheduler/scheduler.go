// Package scheduler provides task scheduling functionality.
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/queue"
)

// Scheduler manages scheduled tasks.
type Scheduler struct {
	cron        *cron.Cron
	workerPool  *queue.WorkerPool
	tasks       map[string]*scheduledEntry
	logger      zerolog.Logger
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

type scheduledEntry struct {
	task    *entity.ScheduledTask
	entryID cron.EntryID
}

// NewScheduler creates a new scheduler.
func NewScheduler(workerPool *queue.WorkerPool, logger zerolog.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		cron:       cron.New(cron.WithSeconds()),
		workerPool: workerPool,
		tasks:      make(map[string]*scheduledEntry),
		logger:     logger.With().Str("component", "scheduler").Logger(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() error {
	s.logger.Info().Msg("Starting scheduler")
	s.cron.Start()
	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() error {
	s.logger.Info().Msg("Stopping scheduler")
	s.cancel()

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.logger.Info().Msg("Scheduler stopped")
	return nil
}

// AddTask adds a scheduled task.
func (s *Scheduler) AddTask(task *entity.ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing if present
	if existing, ok := s.tasks[task.ID]; ok {
		s.cron.Remove(existing.entryID)
	}

	if !task.Enabled {
		s.logger.Debug().
			Str("task_id", task.ID).
			Str("task_name", task.Name).
			Msg("Scheduled task is disabled, not adding")
		return nil
	}

	// Create job function
	job := s.createJob(task)

	// Schedule with cron
	entryID, err := s.cron.AddFunc(task.CronExpression, job)
	if err != nil {
		return err
	}

	// Store entry
	s.tasks[task.ID] = &scheduledEntry{
		task:    task,
		entryID: entryID,
	}

	// Update next run time
	entry := s.cron.Entry(entryID)
	task.NextRun = &entry.Next

	s.logger.Info().
		Str("task_id", task.ID).
		Str("task_name", task.Name).
		Str("cron", task.CronExpression).
		Time("next_run", entry.Next).
		Msg("Added scheduled task")

	return nil
}

// RemoveTask removes a scheduled task.
func (s *Scheduler) RemoveTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entry.entryID)
		delete(s.tasks, taskID)

		s.logger.Info().
			Str("task_id", taskID).
			Msg("Removed scheduled task")
	}

	return nil
}

// GetTask returns a scheduled task.
func (s *Scheduler) GetTask(taskID string) *entity.ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if entry, ok := s.tasks[taskID]; ok {
		return entry.task
	}
	return nil
}

// ListTasks returns all scheduled tasks.
func (s *Scheduler) ListTasks() []*entity.ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*entity.ScheduledTask, 0, len(s.tasks))
	for _, entry := range s.tasks {
		tasks = append(tasks, entry.task)
	}
	return tasks
}

// UpdateNextRun updates the next run time for all tasks.
func (s *Scheduler) UpdateNextRun() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range s.tasks {
		cronEntry := s.cron.Entry(entry.entryID)
		entry.task.NextRun = &cronEntry.Next
	}
}

func (s *Scheduler) createJob(scheduledTask *entity.ScheduledTask) func() {
	return func() {
		s.logger.Debug().
			Str("task_id", scheduledTask.ID).
			Str("task_name", scheduledTask.Name).
			Msg("Executing scheduled task")

		// Create a new task
		task := entity.NewTask(scheduledTask.TaskType, entity.TaskPriorityNormal, scheduledTask.TaskPayload)
		task.WorkspaceID = scheduledTask.WorkspaceID

		// Submit to worker pool
		if err := s.workerPool.Submit(s.ctx, task); err != nil {
			s.logger.Error().
				Err(err).
				Str("task_id", scheduledTask.ID).
				Msg("Failed to submit scheduled task")
			return
		}

		// Update last run time
		now := time.Now()
		scheduledTask.LastRun = &now

		// Update next run time
		s.mu.RLock()
		if entry, ok := s.tasks[scheduledTask.ID]; ok {
			cronEntry := s.cron.Entry(entry.entryID)
			scheduledTask.NextRun = &cronEntry.Next
		}
		s.mu.RUnlock()
	}
}

// ParseCronExpression validates a cron expression.
func ParseCronExpression(expr string) (string, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	if err != nil {
		return "", err
	}
	return expr, nil
}

// NextRunTime calculates the next run time for a cron expression.
func NextRunTime(expr string) (*time.Time, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(time.Now())
	return &next, nil
}
