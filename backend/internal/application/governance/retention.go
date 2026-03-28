package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// RetentionPolicy defines data retention rules.
type RetentionPolicy struct {
	// Trace data retention
	TraceRetentionDays int

	// Benchmark data retention
	BenchmarkRetentionDays int

	// Model usage data retention
	ModelUsageRetentionDays int

	// Extraction events retention
	ExtractionEventsRetentionDays int

	// Temporary file retention
	TempFileRetentionHours int

	// Access logs retention
	AccessLogRetentionDays int

	// Enabled flags
	EnforceTraceRetention     bool
	EnforceBenchmarkRetention bool
	EnforceModelUsageRetention bool
	EnforceExtractionRetention bool
}

// DefaultRetentionPolicy returns a default retention policy.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		TraceRetentionDays:            30,
		BenchmarkRetentionDays:        90,
		ModelUsageRetentionDays:       60,
		ExtractionEventsRetentionDays: 30,
		TempFileRetentionHours:        24,
		AccessLogRetentionDays:        7,
		EnforceTraceRetention:         true,
		EnforceBenchmarkRetention:     true,
		EnforceModelUsageRetention:    true,
		EnforceExtractionRetention:    true,
	}
}

// RetentionResult contains the result of a retention enforcement run.
type RetentionResult struct {
	StartedAt          time.Time         `json:"started_at"`
	CompletedAt        time.Time         `json:"completed_at"`
	TracesDeleted      int64             `json:"traces_deleted"`
	BenchmarksDeleted  int64             `json:"benchmarks_deleted"`
	ModelUsageDeleted  int64             `json:"model_usage_deleted"`
	ExtractionDeleted  int64             `json:"extraction_deleted"`
	Errors             []string          `json:"errors,omitempty"`
	WorkspaceResults   map[string]int64  `json:"workspace_results"`
}

// RetentionService enforces data retention policies.
type RetentionService struct {
	policy        RetentionPolicy
	benchmarkRepo repository.BenchmarkRepository
	traceRepo     repository.TraceRepository
	logger        zerolog.Logger
}

// NewRetentionService creates a new retention service.
func NewRetentionService(
	policy RetentionPolicy,
	benchmarkRepo repository.BenchmarkRepository,
	traceRepo repository.TraceRepository,
	logger zerolog.Logger,
) *RetentionService {
	return &RetentionService{
		policy:        policy,
		benchmarkRepo: benchmarkRepo,
		traceRepo:     traceRepo,
		logger:        logger.With().Str("component", "retention").Logger(),
	}
}

// EnforceRetention enforces all retention policies.
func (s *RetentionService) EnforceRetention(ctx context.Context) (*RetentionResult, error) {
	result := &RetentionResult{
		StartedAt:        time.Now(),
		WorkspaceResults: make(map[string]int64),
	}

	// Enforce trace retention
	if s.policy.EnforceTraceRetention {
		deleted, err := s.enforceTraceRetention(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("trace retention: %v", err))
			s.logger.Error().Err(err).Msg("Failed to enforce trace retention")
		}
		result.TracesDeleted = deleted
	}

	// Enforce benchmark retention
	if s.policy.EnforceBenchmarkRetention {
		deleted, err := s.enforceBenchmarkRetention(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("benchmark retention: %v", err))
			s.logger.Error().Err(err).Msg("Failed to enforce benchmark retention")
		}
		result.BenchmarksDeleted = deleted
	}

	// Enforce model usage retention
	if s.policy.EnforceModelUsageRetention {
		deleted, err := s.enforceModelUsageRetention(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("model usage retention: %v", err))
			s.logger.Error().Err(err).Msg("Failed to enforce model usage retention")
		}
		result.ModelUsageDeleted = deleted
	}

	// Enforce extraction events retention
	if s.policy.EnforceExtractionRetention {
		deleted, err := s.enforceExtractionRetention(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("extraction retention: %v", err))
			s.logger.Error().Err(err).Msg("Failed to enforce extraction retention")
		}
		result.ExtractionDeleted = deleted
	}

	result.CompletedAt = time.Now()

	s.logger.Info().
		Int64("traces", result.TracesDeleted).
		Int64("benchmarks", result.BenchmarksDeleted).
		Int64("model_usage", result.ModelUsageDeleted).
		Int64("extraction", result.ExtractionDeleted).
		Dur("duration", result.CompletedAt.Sub(result.StartedAt)).
		Msg("Retention enforcement completed")

	return result, nil
}

// EnforceForWorkspace enforces retention for a specific workspace.
func (s *RetentionService) EnforceForWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) (*RetentionResult, error) {
	result := &RetentionResult{
		StartedAt:        time.Now(),
		WorkspaceResults: make(map[string]int64),
	}

	var totalDeleted int64

	// Enforce benchmark retention for workspace
	if s.policy.EnforceBenchmarkRetention {
		cutoff := time.Now().AddDate(0, 0, -s.policy.BenchmarkRetentionDays)
		deleted, err := s.benchmarkRepo.DeleteOlderThan(ctx, workspaceID, cutoff)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("benchmark retention: %v", err))
		}
		totalDeleted += deleted
		result.BenchmarksDeleted = deleted
	}

	result.WorkspaceResults[string(workspaceID)] = totalDeleted
	result.CompletedAt = time.Now()

	return result, nil
}

// GetPolicy returns the current retention policy.
func (s *RetentionService) GetPolicy() RetentionPolicy {
	return s.policy
}

// UpdatePolicy updates the retention policy.
func (s *RetentionService) UpdatePolicy(policy RetentionPolicy) {
	s.policy = policy
	s.logger.Info().
		Int("trace_days", policy.TraceRetentionDays).
		Int("benchmark_days", policy.BenchmarkRetentionDays).
		Int("model_usage_days", policy.ModelUsageRetentionDays).
		Msg("Retention policy updated")
}

// GetRetentionStats returns statistics about data subject to retention.
func (s *RetentionService) GetRetentionStats(ctx context.Context, workspaceID entity.WorkspaceID) (*RetentionStats, error) {
	stats := &RetentionStats{
		WorkspaceID: workspaceID,
		GeneratedAt: time.Now(),
	}

	// Calculate cutoff times
	traceCutoff := time.Now().AddDate(0, 0, -s.policy.TraceRetentionDays)
	benchmarkCutoff := time.Now().AddDate(0, 0, -s.policy.BenchmarkRetentionDays)

	// Get counts of records subject to deletion
	// These would typically be database queries
	stats.TracesSubjectToRetention = 0         // Placeholder
	stats.BenchmarksSubjectToRetention = 0     // Placeholder
	stats.ModelUsageSubjectToRetention = 0     // Placeholder
	stats.ExtractionSubjectToRetention = 0     // Placeholder

	stats.TraceRetentionCutoff = traceCutoff
	stats.BenchmarkRetentionCutoff = benchmarkCutoff

	return stats, nil
}

// Internal methods

func (s *RetentionService) enforceTraceRetention(ctx context.Context) (int64, error) {
	// Trace retention would be implemented by the trace repository
	// For now, return 0
	cutoff := time.Now().AddDate(0, 0, -s.policy.TraceRetentionDays)
	s.logger.Debug().Time("cutoff", cutoff).Msg("Enforcing trace retention")
	return 0, nil
}

func (s *RetentionService) enforceBenchmarkRetention(ctx context.Context) (int64, error) {
	// This would iterate over workspaces and delete old benchmarks
	// For now, return 0
	cutoff := time.Now().AddDate(0, 0, -s.policy.BenchmarkRetentionDays)
	s.logger.Debug().Time("cutoff", cutoff).Msg("Enforcing benchmark retention")
	return 0, nil
}

func (s *RetentionService) enforceModelUsageRetention(ctx context.Context) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -s.policy.ModelUsageRetentionDays)
	s.logger.Debug().Time("cutoff", cutoff).Msg("Enforcing model usage retention")
	return 0, nil
}

func (s *RetentionService) enforceExtractionRetention(ctx context.Context) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -s.policy.ExtractionEventsRetentionDays)
	s.logger.Debug().Time("cutoff", cutoff).Msg("Enforcing extraction events retention")
	return 0, nil
}

// RetentionStats contains statistics about data retention.
type RetentionStats struct {
	WorkspaceID                    entity.WorkspaceID `json:"workspace_id"`
	GeneratedAt                    time.Time          `json:"generated_at"`
	TracesSubjectToRetention       int64              `json:"traces_subject_to_retention"`
	BenchmarksSubjectToRetention   int64              `json:"benchmarks_subject_to_retention"`
	ModelUsageSubjectToRetention   int64              `json:"model_usage_subject_to_retention"`
	ExtractionSubjectToRetention   int64              `json:"extraction_subject_to_retention"`
	TraceRetentionCutoff           time.Time          `json:"trace_retention_cutoff"`
	BenchmarkRetentionCutoff       time.Time          `json:"benchmark_retention_cutoff"`
	NextEnforcementRun             time.Time          `json:"next_enforcement_run"`
	LastEnforcementRun             time.Time          `json:"last_enforcement_run"`
	LastEnforcementResult          *RetentionResult   `json:"last_enforcement_result,omitempty"`
}

// RetentionScheduler schedules periodic retention enforcement.
type RetentionScheduler struct {
	service  *RetentionService
	interval time.Duration
	stopCh   chan struct{}
	logger   zerolog.Logger
}

// NewRetentionScheduler creates a new retention scheduler.
func NewRetentionScheduler(service *RetentionService, interval time.Duration, logger zerolog.Logger) *RetentionScheduler {
	return &RetentionScheduler{
		service:  service,
		interval: interval,
		stopCh:   make(chan struct{}),
		logger:   logger.With().Str("component", "retention-scheduler").Logger(),
	}
}

// Start starts the retention scheduler.
func (s *RetentionScheduler) Start(ctx context.Context) {
	s.logger.Info().Dur("interval", s.interval).Msg("Starting retention scheduler")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("Retention scheduler stopped (context cancelled)")
			return
		case <-s.stopCh:
			s.logger.Info().Msg("Retention scheduler stopped")
			return
		case <-ticker.C:
			result, err := s.service.EnforceRetention(ctx)
			if err != nil {
				s.logger.Error().Err(err).Msg("Scheduled retention enforcement failed")
			} else {
				s.logger.Info().
					Int64("total_deleted", result.TracesDeleted+result.BenchmarksDeleted+result.ModelUsageDeleted+result.ExtractionDeleted).
					Msg("Scheduled retention enforcement completed")
			}
		}
	}
}

// Stop stops the retention scheduler.
func (s *RetentionScheduler) Stop() {
	close(s.stopCh)
}
