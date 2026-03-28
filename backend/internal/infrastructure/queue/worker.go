package queue

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
)

// TaskHandler is a function that handles a task.
type TaskHandler func(ctx context.Context, task *entity.Task) error

// WorkerPool manages a pool of workers that process tasks.
type WorkerPool struct {
	queue       Queue
	handlers    map[entity.TaskType]TaskHandler
	publisher   event.Publisher
	logger      zerolog.Logger
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	running     bool
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(queue Queue, publisher event.Publisher, logger zerolog.Logger, workerCount int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		queue:       queue,
		handlers:    make(map[entity.TaskType]TaskHandler),
		publisher:   publisher,
		logger:      logger.With().Str("component", "worker_pool").Logger(),
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterHandler registers a handler for a task type.
func (p *WorkerPool) RegisterHandler(taskType entity.TaskType, handler TaskHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[taskType] = handler
}

// Start starts the worker pool.
func (p *WorkerPool) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = true
	p.mu.Unlock()

	p.logger.Info().Int("workers", p.workerCount).Msg("Starting worker pool")

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return nil
}

// Stop stops the worker pool.
func (p *WorkerPool) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	p.logger.Info().Msg("Stopping worker pool")
	p.cancel()
	p.wg.Wait()
	p.logger.Info().Msg("Worker pool stopped")

	return nil
}

// Pause pauses task processing by pausing the underlying queue.
func (p *WorkerPool) Pause(ctx context.Context) error {
	return p.queue.Pause(ctx)
}

// Resume resumes task processing by resuming the underlying queue.
func (p *WorkerPool) Resume(ctx context.Context) error {
	return p.queue.Resume(ctx)
}

// Drain waits until the queue is empty or the context is canceled.
func (p *WorkerPool) Drain(ctx context.Context) error {
	for {
		size, err := p.queue.Size(ctx)
		if err != nil {
			return err
		}
		if size == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Submit submits a task to the queue.
func (p *WorkerPool) Submit(ctx context.Context, task *entity.Task) error {
	if err := p.queue.Enqueue(ctx, task); err != nil {
		return err
	}

	// Publish event
	if p.publisher != nil {
		evt := event.NewEvent(event.EventTaskCreated, &event.TaskEventData{
			TaskID:   task.ID,
			TaskType: task.Type,
			Status:   task.Status,
		})
		_ = p.publisher.Publish(ctx, evt)
	}

	return nil
}

// worker is the main worker loop.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	logger := p.logger.With().Int("worker_id", id).Logger()
	logger.Debug().Msg("Worker started")

	for {
		select {
		case <-p.ctx.Done():
			logger.Debug().Msg("Worker stopping")
			return
		default:
		}

		// Try to get a task
		task, err := p.queue.Dequeue(p.ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to dequeue task")
			time.Sleep(time.Second)
			continue
		}

		if task == nil {
			// No task available, wait and try again
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Process the task
		p.processTask(p.ctx, task, logger)
	}
}

func (p *WorkerPool) processTask(ctx context.Context, task *entity.Task, logger zerolog.Logger) {
	logger = logger.With().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Logger()

	logger.Debug().Msg("Processing task")

	// Publish start event
	if p.publisher != nil {
		evt := event.NewEvent(event.EventTaskStarted, &event.TaskEventData{
			TaskID:   task.ID,
			TaskType: task.Type,
			Status:   entity.TaskStatusRunning,
		})
		_ = p.publisher.Publish(ctx, evt)
	}

	// Get handler
	p.mu.RLock()
	handler, ok := p.handlers[task.Type]
	p.mu.RUnlock()

	if !ok {
		errMsg := "no handler registered for task type"
		task.Fail(errMsg)
		logger.Error().Msg(errMsg)

		if p.publisher != nil {
			evt := event.NewEvent(event.EventTaskFailed, &event.TaskEventData{
				TaskID:   task.ID,
				TaskType: task.Type,
				Status:   entity.TaskStatusFailed,
				Error:    &errMsg,
			})
			_ = p.publisher.Publish(ctx, evt)
		}
		return
	}

	// Execute handler
	start := time.Now()
	err := handler(ctx, task)
	duration := time.Since(start)

	if err != nil {
		errMsg := err.Error()
		logger.Error().
			Err(err).
			Dur("duration", duration).
			Msg("Task failed")

		// Check if retryable
		if task.CanRetry() {
			task.Retry()
			logger.Info().
				Int("retry_count", task.RetryCount).
				Msg("Retrying task")

			// Re-enqueue
			_ = p.queue.Enqueue(ctx, task)
		} else {
			task.Fail(errMsg)

			if p.publisher != nil {
				evt := event.NewEvent(event.EventTaskFailed, &event.TaskEventData{
					TaskID:   task.ID,
					TaskType: task.Type,
					Status:   entity.TaskStatusFailed,
					Error:    &errMsg,
				})
				_ = p.publisher.Publish(ctx, evt)
			}
		}
		return
	}

	// Success
	task.Complete(nil)
	logger.Debug().
		Dur("duration", duration).
		Msg("Task completed")

	if p.publisher != nil {
		evt := event.NewEvent(event.EventTaskCompleted, &event.TaskEventData{
			TaskID:   task.ID,
			TaskType: task.Type,
			Status:   entity.TaskStatusCompleted,
		})
		_ = p.publisher.Publish(ctx, evt)
	}
}

// Stats returns worker pool statistics.
func (p *WorkerPool) Stats() WorkerPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := WorkerPoolStats{
		WorkerCount: p.workerCount,
		Running:     p.running,
	}

	if queueStats, err := p.queue.Stats(context.Background()); err == nil {
		stats.QueueSize = queueStats.Queued
	}

	return stats
}

// WorkerPoolStats contains worker pool statistics.
type WorkerPoolStats struct {
	WorkerCount int
	Running     bool
	QueueSize   int
}
