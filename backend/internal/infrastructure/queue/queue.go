// Package queue provides task queue implementations.
package queue

import (
	"context"
	"sync"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// Queue defines the interface for a task queue.
type Queue interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, task *entity.Task) error

	// Dequeue removes and returns the next task.
	Dequeue(ctx context.Context) (*entity.Task, error)

	// Peek returns the next task without removing it.
	Peek(ctx context.Context) (*entity.Task, error)

	// Size returns the number of tasks in the queue.
	Size(ctx context.Context) (int, error)

	// Stats returns queue statistics.
	Stats(ctx context.Context) (*entity.TaskStats, error)

	// Pause pauses task processing.
	Pause(ctx context.Context) error

	// Resume resumes task processing.
	Resume(ctx context.Context) error

	// IsPaused returns whether the queue is paused.
	IsPaused() bool
}

// InMemoryQueue is an in-memory implementation of Queue.
type InMemoryQueue struct {
	mu     sync.Mutex
	tasks  []*entity.Task
	paused bool
}

// NewInMemoryQueue creates a new in-memory queue.
func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		tasks: make([]*entity.Task, 0),
	}
}

// Enqueue adds a task to the queue.
func (q *InMemoryQueue) Enqueue(ctx context.Context, task *entity.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.Status = entity.TaskStatusQueued

	// Insert in priority order (higher priority first)
	inserted := false
	for i, t := range q.tasks {
		if task.Priority > t.Priority {
			q.tasks = append(q.tasks[:i], append([]*entity.Task{task}, q.tasks[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.tasks = append(q.tasks, task)
	}

	return nil
}

// Dequeue removes and returns the next task.
func (q *InMemoryQueue) Dequeue(ctx context.Context) (*entity.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.paused || len(q.tasks) == 0 {
		return nil, nil
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	task.Start()

	return task, nil
}

// Peek returns the next task without removing it.
func (q *InMemoryQueue) Peek(ctx context.Context) (*entity.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return nil, nil
	}

	return q.tasks[0], nil
}

// Size returns the number of tasks in the queue.
func (q *InMemoryQueue) Size(ctx context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks), nil
}

// Stats returns queue statistics.
func (q *InMemoryQueue) Stats(ctx context.Context) (*entity.TaskStats, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	stats := &entity.TaskStats{
		Queued: len(q.tasks),
	}

	return stats, nil
}

// Pause pauses task processing.
func (q *InMemoryQueue) Pause(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paused = true
	return nil
}

// Resume resumes task processing.
func (q *InMemoryQueue) Resume(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paused = false
	return nil
}

// IsPaused returns whether the queue is paused.
func (q *InMemoryQueue) IsPaused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

// Clear removes all tasks from the queue.
func (q *InMemoryQueue) Clear(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = make([]*entity.Task, 0)
	return nil
}
