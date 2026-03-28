package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestTaskRepositoryFlows(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewTaskRepository(conn)

	taskHigh := entity.NewTask(entity.TaskTypeScanWorkspace, entity.TaskPriorityHigh, []byte("payload-high"))
	taskHigh.WorkspaceID = &workspace.ID
	if err := repo.Create(ctx, taskHigh); err != nil {
		t.Fatalf("create task high: %v", err)
	}

	taskQueued := entity.NewTask(entity.TaskTypeIndexFile, entity.TaskPriorityNormal, []byte("payload-queued"))
	taskQueued.WorkspaceID = &workspace.ID
	if err := repo.Enqueue(ctx, taskQueued); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	taskPendingLow := entity.NewTask(entity.TaskTypeExtractMetadata, entity.TaskPriorityLow, []byte("payload-low"))
	taskPendingLow.WorkspaceID = &workspace.ID
	if err := repo.Create(ctx, taskPendingLow); err != nil {
		t.Fatalf("create task low: %v", err)
	}

	taskCompleted := entity.NewTask(entity.TaskTypeAnalyzeCode, entity.TaskPriorityLow, []byte("payload-complete"))
	taskCompleted.WorkspaceID = &workspace.ID
	if err := repo.Create(ctx, taskCompleted); err != nil {
		t.Fatalf("create task completed: %v", err)
	}
	past := time.Now().Add(-2 * time.Hour)
	taskCompleted.Status = entity.TaskStatusCompleted
	taskCompleted.Result = []byte("done")
	taskCompleted.CompletedAt = &past
	if err := repo.Update(ctx, taskCompleted); err != nil {
		t.Fatalf("update task completed: %v", err)
	}

	taskFailed := entity.NewTask(entity.TaskTypeSuggestTags, entity.TaskPriorityNormal, []byte("payload-failed"))
	taskFailed.WorkspaceID = &workspace.ID
	if err := repo.Create(ctx, taskFailed); err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	taskFailed.RetryCount = 1
	taskFailed.Status = entity.TaskStatusFailed
	errMsg := "boom"
	taskFailed.Error = &errMsg
	taskFailed.CompletedAt = &past
	if err := repo.Update(ctx, taskFailed); err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	pending, err := repo.GetPending(ctx)
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if pending == nil || pending.ID != taskHigh.ID {
		t.Fatalf("expected pending task to be high priority")
	}

	dequeued, err := repo.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if dequeued == nil || dequeued.ID != taskHigh.ID || dequeued.Status != entity.TaskStatusRunning || dequeued.StartedAt == nil {
		t.Fatalf("unexpected dequeued task: %#v", dequeued)
	}

	got, err := repo.Get(ctx, taskHigh.ID)
	if err != nil {
		t.Fatalf("get after dequeue: %v", err)
	}
	if got.Status != entity.TaskStatusRunning {
		t.Fatalf("expected running status, got %s", got.Status)
	}

	countPending, err := repo.CountByStatus(ctx, entity.TaskStatusPending)
	if err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if countPending != 1 {
		t.Fatalf("expected 1 pending task, got %d", countPending)
	}

	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.Pending != 1 || stats.Queued != 1 || stats.Running != 1 || stats.Completed != 1 || stats.Failed != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}

	statsByWorkspace, err := repo.GetStatsByWorkspace(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("get stats by workspace: %v", err)
	}
	if statsByWorkspace.Pending != 1 || statsByWorkspace.Queued != 1 || statsByWorkspace.Running != 1 {
		t.Fatalf("unexpected workspace stats: %#v", statsByWorkspace)
	}

	if err := repo.UpdateStatus(ctx, taskQueued.ID, entity.TaskStatusCompleted, nil); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, err = repo.Get(ctx, taskQueued.ID)
	if err != nil {
		t.Fatalf("get after update status: %v", err)
	}
	if got.Status != entity.TaskStatusCompleted || got.CompletedAt == nil {
		t.Fatalf("expected completed task, got %#v", got)
	}

	retryable, err := repo.GetRetryableTasks(ctx, 10)
	if err != nil {
		t.Fatalf("get retryable: %v", err)
	}
	if len(retryable) != 1 || retryable[0].ID != taskFailed.ID {
		t.Fatalf("unexpected retryable tasks: %#v", retryable)
	}

	purged, err := repo.PurgeCompleted(ctx, time.Hour)
	if err != nil {
		t.Fatalf("purge completed: %v", err)
	}
	if purged != 1 {
		t.Fatalf("expected 1 purged task, got %d", purged)
	}

	list, err := repo.ListByStatus(ctx, entity.TaskStatusCompleted, repository.TaskListOptions{})
	if err != nil {
		t.Fatalf("list by status: %v", err)
	}
	if len(list) != 1 || list[0].ID != taskQueued.ID {
		t.Fatalf("unexpected completed list: %#v", list)
	}
}
