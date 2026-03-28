package grpc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
)

func TestServerHealthCheck(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.GRPCAddress = "127.0.0.1:0"
	cfg.DataDir = t.TempDir()
	cfg.PluginDir = cfg.DataDir + "/plugins"

	logger := zerolog.New(io.Discard)
	publisher := event.NewBufferedPublisher(event.NewInMemoryPublisher(), 10)
	defer publisher.Close()

	server := NewServer(cfg, logger, publisher)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	conn, err := grpc.Dial(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	healthCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Check(healthCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("unexpected status: %v", resp.Status)
	}
}
