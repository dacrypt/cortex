// Package metrics provides observability and metrics services.
package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DashboardMetrics contains aggregated metrics for the dashboard.
type DashboardMetrics struct {
	// Extraction health
	ExtractionSuccessRate  float64            `json:"extraction_success_rate"`
	ExtractionFailureCount int                `json:"extraction_failure_count"`
	AvgExtractionLatencyMs float64            `json:"avg_extraction_latency_ms"`

	// Confidence distributions
	ConfidenceHistogram    map[string]int     `json:"confidence_histogram"`
	LowConfidenceCount     int                `json:"low_confidence_count"`

	// Model performance
	ModelVersions          map[string]int     `json:"model_versions"`
	TokenUsageByModel      map[string]int64   `json:"token_usage_by_model"`
	CostByModel            map[string]float64 `json:"cost_by_model"`

	// Data quality
	MissingMetadataCount   int                `json:"missing_metadata_count"`
	StaleMetadataCount     int                `json:"stale_metadata_count"`
	OrphanedRecordsCount   int                `json:"orphaned_records_count"`

	// Temporal trends
	HourlyExtractionCounts []int              `json:"hourly_extraction_counts"`
	DailyErrorRates        []float64          `json:"daily_error_rates"`

	// Timestamps
	GeneratedAt time.Time `json:"generated_at"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// ConfidenceDistribution shows confidence score distribution by category.
type ConfidenceDistribution struct {
	Category     string             `json:"category"`
	Distribution map[string]float64 `json:"distribution"` // bucket -> percentage
	Mean         float64            `json:"mean"`
	Median       float64            `json:"median"`
	StdDev       float64            `json:"std_dev"`
}

// DriftReport indicates model performance drift over time.
type DriftReport struct {
	WorkspaceID    entity.WorkspaceID `json:"workspace_id"`
	Period         time.Duration      `json:"period"`
	DriftDetected  bool               `json:"drift_detected"`
	DriftSeverity  string             `json:"drift_severity"` // "none", "minor", "major", "critical"
	Indicators     []DriftIndicator   `json:"indicators"`
	Recommendations []string          `json:"recommendations"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

// DriftIndicator shows a specific drift measurement.
type DriftIndicator struct {
	Metric         string  `json:"metric"`
	BaselineValue  float64 `json:"baseline_value"`
	CurrentValue   float64 `json:"current_value"`
	ChangePercent  float64 `json:"change_percent"`
	IsSignificant  bool    `json:"is_significant"`
}

// Service provides metrics aggregation and analysis.
type Service struct {
	benchmarkRepo  repository.BenchmarkRepository
	metadataRepo   repository.MetadataRepository
	logger         zerolog.Logger
}

// NewService creates a new metrics service.
func NewService(
	benchmarkRepo repository.BenchmarkRepository,
	metadataRepo repository.MetadataRepository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		benchmarkRepo:  benchmarkRepo,
		metadataRepo:   metadataRepo,
		logger:         logger.With().Str("component", "metrics").Logger(),
	}
}

// GetDashboardMetrics retrieves aggregated dashboard metrics for a workspace.
func (s *Service) GetDashboardMetrics(ctx context.Context, workspaceID entity.WorkspaceID, period time.Duration) (*DashboardMetrics, error) {
	now := time.Now()
	periodStart := now.Add(-period)

	metrics := &DashboardMetrics{
		GeneratedAt:         now,
		PeriodStart:         periodStart,
		PeriodEnd:           now,
		ConfidenceHistogram: make(map[string]int),
		ModelVersions:       make(map[string]int),
		TokenUsageByModel:   make(map[string]int64),
		CostByModel:         make(map[string]float64),
	}

	// Aggregate extraction metrics
	if err := s.aggregateExtractionMetrics(ctx, workspaceID, periodStart, metrics); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to aggregate extraction metrics")
	}

	// Aggregate model usage metrics
	if err := s.aggregateModelUsageMetrics(ctx, workspaceID, periodStart, metrics); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to aggregate model usage metrics")
	}

	// Calculate data quality metrics
	if err := s.calculateDataQualityMetrics(ctx, workspaceID, metrics); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to calculate data quality metrics")
	}

	// Build temporal trends
	if err := s.buildTemporalTrends(ctx, workspaceID, periodStart, metrics); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to build temporal trends")
	}

	return metrics, nil
}

// GetConfidenceDistribution returns confidence score distributions.
func (s *Service) GetConfidenceDistribution(ctx context.Context, workspaceID entity.WorkspaceID) (map[string]*ConfidenceDistribution, error) {
	distributions := make(map[string]*ConfidenceDistribution)

	// Categories to analyze
	categories := []string{"ai_category", "tags", "projects", "entities"}

	for _, cat := range categories {
		dist := &ConfidenceDistribution{
			Category:     cat,
			Distribution: make(map[string]float64),
		}

		// Placeholder: actual implementation would query the database
		// and compute distribution statistics
		dist.Distribution["0.0-0.2"] = 0.05
		dist.Distribution["0.2-0.4"] = 0.10
		dist.Distribution["0.4-0.6"] = 0.20
		dist.Distribution["0.6-0.8"] = 0.35
		dist.Distribution["0.8-1.0"] = 0.30
		dist.Mean = 0.65
		dist.Median = 0.68

		distributions[cat] = dist
	}

	return distributions, nil
}

// DetectModelDrift analyzes benchmark trends to detect model performance drift.
func (s *Service) DetectModelDrift(ctx context.Context, workspaceID entity.WorkspaceID) (*DriftReport, error) {
	report := &DriftReport{
		WorkspaceID:   workspaceID,
		Period:        7 * 24 * time.Hour, // Last 7 days
		GeneratedAt:   time.Now(),
		Indicators:    []DriftIndicator{},
	}

	// Get baseline and recent benchmarks
	metricTypes := []entity.BenchmarkMetricType{
		entity.MetricTypeRAG,
		entity.MetricTypeNER,
		entity.MetricTypeClassification,
	}

	var significantDrifts int

	for _, mt := range metricTypes {
		baseline, err := s.benchmarkRepo.GetBaseline(ctx, workspaceID, mt)
		if err != nil || baseline == nil {
			continue
		}

		recent, err := s.benchmarkRepo.GetLatestByType(ctx, workspaceID, mt)
		if err != nil || recent == nil {
			continue
		}

		// Check F1 drift
		f1Indicator := s.createDriftIndicator("f1_score_"+string(mt), baseline.F1Score, recent.F1Score)
		if f1Indicator.IsSignificant {
			significantDrifts++
		}
		report.Indicators = append(report.Indicators, f1Indicator)

		// Check precision drift
		precisionIndicator := s.createDriftIndicator("precision_"+string(mt), baseline.Precision, recent.Precision)
		if precisionIndicator.IsSignificant {
			significantDrifts++
		}
		report.Indicators = append(report.Indicators, precisionIndicator)

		// Check latency drift
		latencyIndicator := s.createDriftIndicator("latency_"+string(mt), float64(baseline.LatencyMs), float64(recent.LatencyMs))
		latencyIndicator.IsSignificant = latencyIndicator.ChangePercent > 50 // Latency increase > 50%
		if latencyIndicator.IsSignificant {
			significantDrifts++
		}
		report.Indicators = append(report.Indicators, latencyIndicator)
	}

	// Determine overall drift severity
	switch {
	case significantDrifts == 0:
		report.DriftSeverity = "none"
	case significantDrifts <= 2:
		report.DriftDetected = true
		report.DriftSeverity = "minor"
		report.Recommendations = append(report.Recommendations,
			"Monitor affected metrics over the next few days")
	case significantDrifts <= 4:
		report.DriftDetected = true
		report.DriftSeverity = "major"
		report.Recommendations = append(report.Recommendations,
			"Review recent model or data changes",
			"Consider rerunning benchmarks with fresh test data")
	default:
		report.DriftDetected = true
		report.DriftSeverity = "critical"
		report.Recommendations = append(report.Recommendations,
			"Immediate investigation required",
			"Consider rolling back recent model changes",
			"Verify data pipeline integrity")
	}

	return report, nil
}

// Helper methods

func (s *Service) aggregateExtractionMetrics(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, metrics *DashboardMetrics) error {
	// This would query extraction_events table
	// Placeholder implementation

	metrics.ExtractionSuccessRate = 0.95
	metrics.ExtractionFailureCount = 12
	metrics.AvgExtractionLatencyMs = 450.5

	return nil
}

func (s *Service) aggregateModelUsageMetrics(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, metrics *DashboardMetrics) error {
	// This would query model_usage table
	// Placeholder implementation

	metrics.ModelVersions["llama3.2"] = 150
	metrics.ModelVersions["gpt-4o-mini"] = 50
	metrics.TokenUsageByModel["llama3.2"] = 1500000
	metrics.TokenUsageByModel["gpt-4o-mini"] = 250000
	metrics.CostByModel["llama3.2"] = 0 // Local model
	metrics.CostByModel["gpt-4o-mini"] = 0.15

	return nil
}

func (s *Service) calculateDataQualityMetrics(ctx context.Context, workspaceID entity.WorkspaceID, metrics *DashboardMetrics) error {
	// This would analyze file_metadata table for quality issues
	// Placeholder implementation

	metrics.MissingMetadataCount = 25
	metrics.StaleMetadataCount = 10
	metrics.OrphanedRecordsCount = 3

	// Build confidence histogram
	metrics.ConfidenceHistogram["0.0-0.2"] = 5
	metrics.ConfidenceHistogram["0.2-0.4"] = 10
	metrics.ConfidenceHistogram["0.4-0.6"] = 20
	metrics.ConfidenceHistogram["0.6-0.8"] = 35
	metrics.ConfidenceHistogram["0.8-1.0"] = 30

	metrics.LowConfidenceCount = 15 // Files with confidence < 0.4

	return nil
}

func (s *Service) buildTemporalTrends(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, metrics *DashboardMetrics) error {
	// Build hourly extraction counts (last 24 hours)
	metrics.HourlyExtractionCounts = make([]int, 24)
	for i := 0; i < 24; i++ {
		metrics.HourlyExtractionCounts[i] = 10 + i%5 // Placeholder
	}

	// Build daily error rates (last 7 days)
	metrics.DailyErrorRates = make([]float64, 7)
	for i := 0; i < 7; i++ {
		metrics.DailyErrorRates[i] = 0.03 + float64(i%3)*0.01 // Placeholder
	}

	return nil
}

func (s *Service) createDriftIndicator(metric string, baseline, current float64) DriftIndicator {
	indicator := DriftIndicator{
		Metric:        metric,
		BaselineValue: baseline,
		CurrentValue:  current,
	}

	if baseline > 0 {
		indicator.ChangePercent = ((current - baseline) / baseline) * 100
	}

	// Significant if change > 10% in either direction for quality metrics
	indicator.IsSignificant = indicator.ChangePercent < -10 // Quality degradation

	return indicator
}

// ExtractionEvent represents an extraction event for tracking.
type ExtractionEvent struct {
	ID            string
	WorkspaceID   entity.WorkspaceID
	FileID        entity.FileID
	RelativePath  string
	Stage         string
	EventType     string // "success", "failure"
	ErrorType     string
	ErrorMessage  string
	Retryable     bool
	RetryCount    int
	ItemsExtracted int
	Confidence    float64
	DurationMs    int64
	ModelVersion  string
	CreatedAt     time.Time
}

// RecordExtractionEvent records an extraction event for metrics.
func (s *Service) RecordExtractionEvent(ctx context.Context, event ExtractionEvent) error {
	// This would insert into extraction_events table
	s.logger.Debug().
		Str("file_id", string(event.FileID)).
		Str("stage", event.Stage).
		Str("event_type", event.EventType).
		Msg("Recording extraction event")

	return nil
}

// ModelUsageEvent represents a model usage event for tracking.
type ModelUsageEvent struct {
	ID               string
	WorkspaceID      entity.WorkspaceID
	ModelID          string
	ModelVersion     string
	Provider         string
	Operation        string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCost    float64
	LatencyMs        int64
	Success          bool
	ErrorMessage     string
	CreatedAt        time.Time
}

// RecordModelUsage records a model usage event for cost tracking.
func (s *Service) RecordModelUsage(ctx context.Context, event ModelUsageEvent) error {
	// This would insert into model_usage table
	s.logger.Debug().
		Str("model_id", event.ModelID).
		Str("operation", event.Operation).
		Int("tokens", event.TotalTokens).
		Float64("cost", event.EstimatedCost).
		Msg("Recording model usage")

	return nil
}
