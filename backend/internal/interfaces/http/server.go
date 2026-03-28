// Package http provides HTTP endpoints for health and metrics.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

// Server provides HTTP endpoints for health and metrics.
type Server struct {
	server      *http.Server
	healthCheck HealthChecker
	metrics     MetricsProvider
	startTime   time.Time
	logger      zerolog.Logger
}

// HealthChecker provides health check functionality.
type HealthChecker interface {
	Check(ctx context.Context) HealthStatus
}

// MetricsProvider provides metrics.
type MetricsProvider interface {
	GetMetrics(ctx context.Context) Metrics
}

// HealthStatus represents health check status.
type HealthStatus struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Metrics represents system metrics.
type Metrics struct {
	Uptime       float64 `json:"uptime_seconds"`
	Goroutines   int     `json:"goroutines"`
	HeapAlloc    uint64  `json:"heap_alloc_bytes"`
	HeapSys      uint64  `json:"heap_sys_bytes"`
	HeapObjects  uint64  `json:"heap_objects"`
	GCCycles     uint32  `json:"gc_cycles"`
	FilesIndexed int     `json:"files_indexed"`
	TasksPending int     `json:"tasks_pending"`
	TasksRunning int     `json:"tasks_running"`
}

// Config holds server configuration.
type Config struct {
	Addr          string
	HealthChecker HealthChecker
	Metrics       MetricsProvider
	Logger        zerolog.Logger
	ExtraHandlers map[string]http.Handler
}

// NewServer creates a new HTTP server.
func NewServer(cfg Config) *Server {
	s := &Server{
		healthCheck: cfg.HealthChecker,
		metrics:     cfg.Metrics,
		startTime:   time.Now(),
		logger:      cfg.Logger.With().Str("component", "http_server").Logger(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/health/live", s.handleLive)
	mux.HandleFunc("/health/ready", s.handleReady)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/version", s.handleVersion)
	for path, handler := range cfg.ExtraHandlers {
		if handler != nil {
			mux.Handle(path, handler)
		}
	}

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info().
		Str("addr", s.server.Addr).
		Msg("Starting HTTP server")

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	return nil
}

// Stop stops the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping HTTP server")
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    make(map[string]string),
	}

	if s.healthCheck != nil {
		status = s.healthCheck.Check(ctx)
	}

	w.Header().Set("Content-Type", "application/json")

	if status.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	// Liveness probe - just check if we're running
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "alive",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ready := true
	if s.healthCheck != nil {
		status := s.healthCheck.Check(ctx)
		ready = status.Status == "healthy"
	}

	w.Header().Set("Content-Type", "application/json")

	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := Metrics{
		Uptime:      time.Since(s.startTime).Seconds(),
		Goroutines:  runtime.NumGoroutine(),
		HeapAlloc:   m.HeapAlloc,
		HeapSys:     m.HeapSys,
		HeapObjects: m.HeapObjects,
		GCCycles:    m.NumGC,
	}

	if s.metrics != nil {
		custom := s.metrics.GetMetrics(ctx)
		metrics.FilesIndexed = custom.FilesIndexed
		metrics.TasksPending = custom.TasksPending
		metrics.TasksRunning = custom.TasksRunning
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version":    "0.1.0",
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	})
}

// SimpleHealthChecker is a basic health checker.
type SimpleHealthChecker struct {
	checks []func(context.Context) (string, error)
}

// NewSimpleHealthChecker creates a simple health checker.
func NewSimpleHealthChecker() *SimpleHealthChecker {
	return &SimpleHealthChecker{
		checks: make([]func(context.Context) (string, error), 0),
	}
}

// AddCheck adds a health check.
func (h *SimpleHealthChecker) AddCheck(name string, check func(context.Context) error) {
	h.checks = append(h.checks, func(ctx context.Context) (string, error) {
		if err := check(ctx); err != nil {
			return name, err
		}
		return name, nil
	})
}

// Check performs all health checks.
func (h *SimpleHealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    make(map[string]string),
	}

	for _, check := range h.checks {
		name, err := check(ctx)
		if err != nil {
			status.Checks[name] = fmt.Sprintf("unhealthy: %v", err)
			status.Status = "unhealthy"
		} else {
			status.Checks[name] = "healthy"
		}
	}

	return status
}

// SimpleMetricsProvider is a basic metrics provider.
type SimpleMetricsProvider struct {
	getFilesIndexed func() int
	getTasksPending func() int
	getTasksRunning func() int
}

// NewSimpleMetricsProvider creates a simple metrics provider.
func NewSimpleMetricsProvider() *SimpleMetricsProvider {
	return &SimpleMetricsProvider{}
}

// SetFilesIndexedFunc sets the function to get files indexed count.
func (m *SimpleMetricsProvider) SetFilesIndexedFunc(f func() int) {
	m.getFilesIndexed = f
}

// SetTasksPendingFunc sets the function to get pending tasks count.
func (m *SimpleMetricsProvider) SetTasksPendingFunc(f func() int) {
	m.getTasksPending = f
}

// SetTasksRunningFunc sets the function to get running tasks count.
func (m *SimpleMetricsProvider) SetTasksRunningFunc(f func() int) {
	m.getTasksRunning = f
}

// GetMetrics returns current metrics.
func (m *SimpleMetricsProvider) GetMetrics(ctx context.Context) Metrics {
	metrics := Metrics{}

	if m.getFilesIndexed != nil {
		metrics.FilesIndexed = m.getFilesIndexed()
	}
	if m.getTasksPending != nil {
		metrics.TasksPending = m.getTasksPending()
	}
	if m.getTasksRunning != nil {
		metrics.TasksRunning = m.getTasksRunning()
	}

	return metrics
}
