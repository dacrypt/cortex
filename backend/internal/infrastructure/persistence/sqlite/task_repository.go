// Package sqlite provides SQLite implementations of repositories.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// TaskRepository implements repository.TaskRepository using SQLite.
type TaskRepository struct {
	conn *Connection
}

// NewTaskRepository creates a new SQLite task repository.
func NewTaskRepository(conn *Connection) *TaskRepository {
	return &TaskRepository{conn: conn}
}

// Create creates a new task.
func (r *TaskRepository) Create(ctx context.Context, task *entity.Task) error {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO tasks (
			id, type, status, priority, payload, retry_count, max_retries,
			progress_processed, progress_total, progress_message,
			workspace_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.conn.db.ExecContext(ctx, query,
		task.ID,
		task.Type,
		task.Status,
		task.Priority,
		payload,
		task.RetryCount,
		task.MaxRetries,
		task.Progress.Processed,
		task.Progress.Total,
		task.Progress.Message,
		task.WorkspaceID,
		task.CreatedAt.UnixMilli(),
	)
	return err
}

// Get retrieves a task by ID.
func (r *TaskRepository) Get(ctx context.Context, id entity.TaskID) (*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks WHERE id = ?`

	row := r.conn.db.QueryRowContext(ctx, query, id)
	return r.scanTask(row)
}

// Update updates an existing task.
func (r *TaskRepository) Update(ctx context.Context, task *entity.Task) error {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	var result []byte
	if task.Result != nil {
		result, err = json.Marshal(task.Result)
		if err != nil {
			return err
		}
	}

	var startedAt, completedAt *int64
	if task.StartedAt != nil {
		ts := task.StartedAt.UnixMilli()
		startedAt = &ts
	}
	if task.CompletedAt != nil {
		ts := task.CompletedAt.UnixMilli()
		completedAt = &ts
	}

	query := `
		UPDATE tasks SET
			status = ?, priority = ?, payload = ?, result = ?, error = ?,
			retry_count = ?, progress_processed = ?, progress_total = ?, progress_message = ?,
			started_at = ?, completed_at = ?
		WHERE id = ?`

	_, err = r.conn.db.ExecContext(ctx, query,
		task.Status,
		task.Priority,
		payload,
		result,
		task.Error,
		task.RetryCount,
		task.Progress.Processed,
		task.Progress.Total,
		task.Progress.Message,
		startedAt,
		completedAt,
		task.ID,
	)
	return err
}

// Delete deletes a task.
func (r *TaskRepository) Delete(ctx context.Context, id entity.TaskID) error {
	_, err := r.conn.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	return err
}

// ListByStatus lists tasks by status.
func (r *TaskRepository) ListByStatus(ctx context.Context, status entity.TaskStatus, opts repository.TaskListOptions) ([]*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks WHERE status = ?
		ORDER BY priority DESC, created_at ASC
		LIMIT ? OFFSET ?`

	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	rows, err := r.conn.db.QueryContext(ctx, query, status, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// ListByWorkspace lists tasks by workspace.
func (r *TaskRepository) ListByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.TaskListOptions) ([]*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks WHERE workspace_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	rows, err := r.conn.db.QueryContext(ctx, query, workspaceID, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetPending gets the next pending task to process.
func (r *TaskRepository) GetPending(ctx context.Context) (*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks
		WHERE status = ?
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`

	row := r.conn.db.QueryRowContext(ctx, query, entity.TaskStatusPending)
	task, err := r.scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return task, err
}

// CountByStatus counts tasks by status.
func (r *TaskRepository) CountByStatus(ctx context.Context, status entity.TaskStatus) (int, error) {
	var count int
	err := r.conn.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tasks WHERE status = ?", status).Scan(&count)
	return count, err
}

// GetStats returns task queue statistics.
func (r *TaskRepository) GetStats(ctx context.Context) (*entity.TaskStats, error) {
	stats := &entity.TaskStats{}

	// Count by status
	rows, err := r.conn.db.QueryContext(ctx,
		"SELECT status, COUNT(*) FROM tasks GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}

		switch entity.TaskStatus(status) {
		case entity.TaskStatusPending:
			stats.Pending = count
		case entity.TaskStatusQueued:
			stats.Queued = count
		case entity.TaskStatusRunning:
			stats.Running = count
		case entity.TaskStatusCompleted:
			stats.Completed = count
		case entity.TaskStatusFailed:
			stats.Failed = count
		case entity.TaskStatusCancelled:
			stats.Cancelled = count
		}
	}

	return stats, nil
}

// GetStatsByWorkspace returns task stats for a workspace.
func (r *TaskRepository) GetStatsByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.TaskStats, error) {
	stats := &entity.TaskStats{}

	rows, err := r.conn.db.QueryContext(ctx,
		"SELECT status, COUNT(*) FROM tasks WHERE workspace_id = ? GROUP BY status", workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}

		switch entity.TaskStatus(status) {
		case entity.TaskStatusPending:
			stats.Pending = count
		case entity.TaskStatusQueued:
			stats.Queued = count
		case entity.TaskStatusRunning:
			stats.Running = count
		case entity.TaskStatusCompleted:
			stats.Completed = count
		case entity.TaskStatusFailed:
			stats.Failed = count
		case entity.TaskStatusCancelled:
			stats.Cancelled = count
		}
	}

	return stats, nil
}

// Enqueue adds a task to the queue.
func (r *TaskRepository) Enqueue(ctx context.Context, task *entity.Task) error {
	task.Status = entity.TaskStatusQueued
	return r.Create(ctx, task)
}

// Dequeue gets and removes the next task from the queue.
func (r *TaskRepository) Dequeue(ctx context.Context) (*entity.Task, error) {
	tx, err := r.conn.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks
		WHERE status IN (?, ?)
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`

	var (
		task          entity.Task
		payload       []byte
		result        []byte
		createdAt     int64
		startedAt     *int64
		completedAt   *int64
		progressProc  int
		progressTotal int
		progressMsg   string
	)

	err = tx.QueryRowContext(ctx, query, entity.TaskStatusPending, entity.TaskStatusQueued).Scan(
		&task.ID, &task.Type, &task.Status, &task.Priority,
		&payload, &result, &task.Error,
		&task.RetryCount, &task.MaxRetries,
		&progressProc, &progressTotal, &progressMsg,
		&task.WorkspaceID, &createdAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Update status to running
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		"UPDATE tasks SET status = ?, started_at = ? WHERE id = ?",
		entity.TaskStatusRunning, now.UnixMilli(), task.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	task.Payload = payload
	task.Result = result
	task.CreatedAt = time.UnixMilli(createdAt)
	task.Status = entity.TaskStatusRunning
	task.StartedAt = &now
	task.Progress = entity.TaskProgress{Processed: progressProc, Total: progressTotal, Message: progressMsg}

	return &task, nil
}

// DequeueByType gets the next task of a specific type.
func (r *TaskRepository) DequeueByType(ctx context.Context, taskType entity.TaskType) (*entity.Task, error) {
	task, err := r.Dequeue(ctx)
	if err != nil || task == nil {
		return task, err
	}
	if task.Type != taskType {
		// Put it back
		task.Status = entity.TaskStatusPending
		task.StartedAt = nil
		r.Update(ctx, task)
		return nil, nil
	}
	return task, nil
}

// DequeueByPriority gets the next task with at least the given priority.
func (r *TaskRepository) DequeueByPriority(ctx context.Context, minPriority entity.TaskPriority) (*entity.Task, error) {
	task, err := r.Dequeue(ctx)
	if err != nil || task == nil {
		return task, err
	}
	if task.Priority < minPriority {
		task.Status = entity.TaskStatusPending
		task.StartedAt = nil
		r.Update(ctx, task)
		return nil, nil
	}
	return task, nil
}

// Peek returns tasks without removing them.
func (r *TaskRepository) Peek(ctx context.Context, limit int) ([]*entity.Task, error) {
	return r.ListByStatus(ctx, entity.TaskStatusPending, repository.TaskListOptions{Limit: limit})
}

// UpdateStatus updates a task's status.
func (r *TaskRepository) UpdateStatus(ctx context.Context, id entity.TaskID, status entity.TaskStatus, errStr *string) error {
	var completedAt *int64
	if status == entity.TaskStatusCompleted || status == entity.TaskStatusFailed || status == entity.TaskStatusCancelled {
		now := time.Now().UnixMilli()
		completedAt = &now
	}
	_, err := r.conn.db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?",
		status, errStr, completedAt, id)
	return err
}

// UpdateProgress updates a task's progress.
func (r *TaskRepository) UpdateProgress(ctx context.Context, id entity.TaskID, progress entity.TaskProgress) error {
	_, err := r.conn.db.ExecContext(ctx,
		"UPDATE tasks SET progress_processed = ?, progress_total = ?, progress_message = ? WHERE id = ?",
		progress.Processed, progress.Total, progress.Message, id)
	return err
}

// List lists tasks with options.
func (r *TaskRepository) List(ctx context.Context, opts repository.TaskListOptions) ([]*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	rows, err := r.conn.db.QueryContext(ctx, query, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// ListByType lists tasks by type.
func (r *TaskRepository) ListByType(ctx context.Context, taskType entity.TaskType, opts repository.TaskListOptions) ([]*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks WHERE type = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	rows, err := r.conn.db.QueryContext(ctx, query, taskType, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// PurgeCompleted removes completed tasks older than the specified duration.
func (r *TaskRepository) PurgeCompleted(ctx context.Context, olderThan time.Duration) (int, error) {
	threshold := time.Now().Add(-olderThan).UnixMilli()
	result, err := r.conn.db.ExecContext(ctx,
		"DELETE FROM tasks WHERE status = ? AND completed_at < ?",
		entity.TaskStatusCompleted, threshold)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// PurgeFailed removes failed tasks older than the specified duration.
func (r *TaskRepository) PurgeFailed(ctx context.Context, olderThan time.Duration) (int, error) {
	threshold := time.Now().Add(-olderThan).UnixMilli()
	result, err := r.conn.db.ExecContext(ctx,
		"DELETE FROM tasks WHERE status = ? AND completed_at < ?",
		entity.TaskStatusFailed, threshold)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// PurgeCancelled removes cancelled tasks older than the specified duration.
func (r *TaskRepository) PurgeCancelled(ctx context.Context, olderThan time.Duration) (int, error) {
	threshold := time.Now().Add(-olderThan).UnixMilli()
	result, err := r.conn.db.ExecContext(ctx,
		"DELETE FROM tasks WHERE status = ? AND completed_at < ?",
		entity.TaskStatusCancelled, threshold)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// GetRetryableTasks returns tasks that can be retried.
func (r *TaskRepository) GetRetryableTasks(ctx context.Context, limit int) ([]*entity.Task, error) {
	query := `
		SELECT id, type, status, priority, payload, result, error,
			retry_count, max_retries, progress_processed, progress_total, progress_message,
			workspace_id, created_at, started_at, completed_at
		FROM tasks
		WHERE status = ? AND retry_count < max_retries
		ORDER BY created_at ASC
		LIMIT ?`

	rows, err := r.conn.db.QueryContext(ctx, query, entity.TaskStatusFailed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// ResetStuckTasks resets tasks that have been running too long.
func (r *TaskRepository) ResetStuckTasks(ctx context.Context, stuckDuration time.Duration) (int, error) {
	threshold := time.Now().Add(-stuckDuration).UnixMilli()
	result, err := r.conn.db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, started_at = NULL WHERE status = ? AND started_at < ?",
		entity.TaskStatusPending, entity.TaskStatusRunning, threshold)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// CleanupOld removes completed tasks older than the specified duration.
func (r *TaskRepository) CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	threshold := time.Now().Add(-olderThan).UnixMilli()

	result, err := r.conn.db.ExecContext(ctx, `
		DELETE FROM tasks
		WHERE status IN (?, ?, ?) AND completed_at < ?`,
		entity.TaskStatusCompleted, entity.TaskStatusFailed, entity.TaskStatusCancelled,
		threshold,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (r *TaskRepository) scanTask(row *sql.Row) (*entity.Task, error) {
	var (
		task          entity.Task
		payload       []byte
		result        []byte
		createdAt     int64
		startedAt     *int64
		completedAt   *int64
		progressProc  int
		progressTotal int
		progressMsg   string
	)

	err := row.Scan(
		&task.ID, &task.Type, &task.Status, &task.Priority,
		&payload, &result, &task.Error,
		&task.RetryCount, &task.MaxRetries,
		&progressProc, &progressTotal, &progressMsg,
		&task.WorkspaceID, &createdAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &task.Payload); err != nil {
			return nil, err
		}
	}

	if len(result) > 0 {
		if err := json.Unmarshal(result, &task.Result); err != nil {
			return nil, err
		}
	}

	task.CreatedAt = time.UnixMilli(createdAt)
	if startedAt != nil {
		t := time.UnixMilli(*startedAt)
		task.StartedAt = &t
	}
	if completedAt != nil {
		t := time.UnixMilli(*completedAt)
		task.CompletedAt = &t
	}

	task.Progress = entity.TaskProgress{
		Processed: progressProc,
		Total:     progressTotal,
		Message:   progressMsg,
	}

	return &task, nil
}

func (r *TaskRepository) scanTasks(rows *sql.Rows) ([]*entity.Task, error) {
	var tasks []*entity.Task

	for rows.Next() {
		var (
			task          entity.Task
			payload       []byte
			result        []byte
			createdAt     int64
			startedAt     *int64
			completedAt   *int64
			progressProc  int
			progressTotal int
			progressMsg   string
		)

		err := rows.Scan(
			&task.ID, &task.Type, &task.Status, &task.Priority,
			&payload, &result, &task.Error,
			&task.RetryCount, &task.MaxRetries,
			&progressProc, &progressTotal, &progressMsg,
			&task.WorkspaceID, &createdAt, &startedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &task.Payload); err != nil {
				return nil, err
			}
		}

		if len(result) > 0 {
			if err := json.Unmarshal(result, &task.Result); err != nil {
				return nil, err
			}
		}

		task.CreatedAt = time.UnixMilli(createdAt)
		if startedAt != nil {
			t := time.UnixMilli(*startedAt)
			task.StartedAt = &t
		}
		if completedAt != nil {
			t := time.UnixMilli(*completedAt)
			task.CompletedAt = &t
		}

		task.Progress = entity.TaskProgress{
			Processed: progressProc,
			Total:     progressTotal,
			Message:   progressMsg,
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// Ensure TaskRepository implements repository.TaskRepository
var _ repository.TaskRepository = (*TaskRepository)(nil)
