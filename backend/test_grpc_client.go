//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to gRPC server
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Println("=== Testing AdminService.HealthCheck ===")
	adminClient := cortexv1.NewAdminServiceClient(conn)
	healthResp, err := adminClient.HealthCheck(ctx, &cortexv1.HealthCheckRequest{})
	if err != nil {
		log.Printf("HealthCheck failed: %v", err)
	} else {
		fmt.Printf("Health: healthy=%v, status=%s\n", healthResp.Healthy, healthResp.Status)
	}

	fmt.Println("\n=== Testing AdminService.GetStatus ===")
	statusResp, err := adminClient.GetStatus(ctx, &cortexv1.GetStatusRequest{})
	if err != nil {
		log.Printf("GetStatus failed: %v", err)
	} else {
		fmt.Printf("Status: version=%s, uptime=%ds, workspaces=%d, files=%d\n",
			statusResp.Version, statusResp.UptimeSeconds, statusResp.WorkspaceCount, statusResp.IndexedFiles)
	}

	fmt.Println("\n=== Testing AdminService.GetMetrics ===")
	metricsResp, err := adminClient.GetMetrics(ctx, &cortexv1.GetMetricsRequest{})
	if err != nil {
		log.Printf("GetMetrics failed: %v", err)
	} else {
		fmt.Printf("Metrics: goroutines=%d, heap=%d bytes\n",
			metricsResp.Resources.GoroutineCount, metricsResp.Resources.MemoryBytes)
	}

	fmt.Println("\n=== All gRPC tests complete! ===")
}
