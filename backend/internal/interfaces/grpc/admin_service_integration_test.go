package grpc

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/adapters"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAdminServiceIntegration(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.GRPCAddress = "127.0.0.1:0"
	cfg.DataDir = t.TempDir()
	cfg.PluginDir = filepath.Join(cfg.DataDir, "plugins")

	logger := zerolog.New(io.Discard)
	publisher := event.NewBufferedPublisher(event.NewInMemoryPublisher(), 10)
	defer publisher.Close()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	taskRepo := sqlite.NewTaskRepository(conn)

	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	entry := entity.NewFileEntry(workspace.Path, "notes.txt", 12, time.Now())
	entry.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
	if err := fileRepo.Upsert(ctx, workspace.ID, entry); err != nil {
		t.Fatalf("upsert file: %v", err)
	}

	pendingTask := entity.NewTask(entity.TaskTypeScanWorkspace, entity.TaskPriorityNormal, []byte("payload"))
	pendingTask.WorkspaceID = &workspace.ID
	if err := taskRepo.Create(ctx, pendingTask); err != nil {
		t.Fatalf("create pending task: %v", err)
	}

	completedTask := entity.NewTask(entity.TaskTypeAnalyzeCode, entity.TaskPriorityLow, []byte("payload"))
	completedTask.WorkspaceID = &workspace.ID
	if err := taskRepo.Create(ctx, completedTask); err != nil {
		t.Fatalf("create completed task: %v", err)
	}
	if err := taskRepo.UpdateStatus(ctx, completedTask.ID, entity.TaskStatusCompleted, nil); err != nil {
		t.Fatalf("update completed status: %v", err)
	}

	adminHandler := handlers.NewAdminHandler(handlers.AdminHandlerConfig{
		WorkspaceRepo: workspaceRepo,
		FileRepo:      fileRepo,
		TaskRepo:      taskRepo,
		Version:       "test",
		Logger:        logger,
	})
	adminAdapter := adapters.NewAdminServiceAdapter(adminHandler)

	server := NewServer(cfg, logger, publisher)
	server.RegisterAdminService(adminAdapter)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	connCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	clientConn, err := grpc.DialContext(connCtx, server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	client := cortexv1.NewAdminServiceClient(clientConn)
	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()

	healthResp, err := client.HealthCheck(callCtx, &cortexv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if !healthResp.Healthy || healthResp.Status != "healthy" {
		t.Fatalf("unexpected health: %#v", healthResp)
	}
	if _, ok := healthResp.Components["database"]; !ok {
		t.Fatalf("expected database component in health response")
	}

	statusResp, err := client.GetStatus(callCtx, &cortexv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if statusResp.WorkspaceCount != 1 || statusResp.IndexedFiles != 1 {
		t.Fatalf("unexpected status counts: %#v", statusResp)
	}
	if statusResp.QueueStats == nil || statusResp.QueueStats.Pending != 1 || statusResp.QueueStats.Completed != 1 {
		t.Fatalf("unexpected queue stats: %#v", statusResp.QueueStats)
	}

	metricsResp, err := client.GetMetrics(callCtx, &cortexv1.GetMetricsRequest{})
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	if metricsResp.Counters["files_indexed"] != 1 {
		t.Fatalf("unexpected files_indexed counter: %#v", metricsResp.Counters)
	}
	if metricsResp.Counters["tasks_processed"] != 1 || metricsResp.Counters["tasks_pending"] != 1 {
		t.Fatalf("unexpected task counters: %#v", metricsResp.Counters)
	}

	reloadResp, err := client.Reload(callCtx, &cortexv1.ReloadRequest{})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloadResp.Success {
		t.Fatalf("expected reload success: %#v", reloadResp)
	}

	listCtx, listCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer listCancel()
	stream, err := client.ListWorkspaces(listCtx, &cortexv1.ListWorkspacesRequest{})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	workspaceCount := 0
	for {
		ws, err := stream.Recv()
		if err != nil {
			break
		}
		if ws != nil {
			workspaceCount++
		}
	}
	if workspaceCount != 1 {
		t.Fatalf("expected 1 workspace in stream, got %d", workspaceCount)
	}
}
