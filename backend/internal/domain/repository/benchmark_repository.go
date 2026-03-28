package repository

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// BenchmarkRepository stores and retrieves benchmark results.
type BenchmarkRepository interface {
	// SaveResult stores a benchmark result.
	SaveResult(ctx context.Context, result *entity.BenchmarkMetrics) error

	// GetResultByID retrieves a specific benchmark result.
	GetResultByID(ctx context.Context, id string) (*entity.BenchmarkMetrics, error)

	// GetResultsByType retrieves benchmark results filtered by type.
	GetResultsByType(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, limit int) ([]*entity.BenchmarkMetrics, error)

	// GetResultsByTestSuite retrieves benchmark results for a specific test suite.
	GetResultsByTestSuite(ctx context.Context, workspaceID entity.WorkspaceID, testSuite string, limit int) ([]*entity.BenchmarkMetrics, error)

	// GetLatestByType retrieves the most recent benchmark of a given type.
	GetLatestByType(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType) (*entity.BenchmarkMetrics, error)

	// GetBaseline retrieves the designated baseline benchmark for comparison.
	GetBaseline(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType) (*entity.BenchmarkMetrics, error)

	// SetBaseline designates a benchmark result as the baseline for future comparisons.
	SetBaseline(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, benchmarkID string) error

	// GetResultsInTimeRange retrieves benchmarks within a time range.
	GetResultsInTimeRange(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, start, end time.Time) ([]*entity.BenchmarkMetrics, error)

	// DeleteOlderThan removes benchmark results older than the specified time.
	DeleteOlderThan(ctx context.Context, workspaceID entity.WorkspaceID, before time.Time) (int64, error)

	// GetAggregatedMetrics returns aggregated metrics over a time period.
	GetAggregatedMetrics(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, period time.Duration) (*AggregatedBenchmarkMetrics, error)
}

// AggregatedBenchmarkMetrics contains aggregated benchmark statistics.
type AggregatedBenchmarkMetrics struct {
	MetricType entity.BenchmarkMetricType `json:"metric_type"`
	Period     time.Duration              `json:"period"`
	Count      int                        `json:"count"`

	// Average quality metrics
	AvgPrecision float64 `json:"avg_precision"`
	AvgRecall    float64 `json:"avg_recall"`
	AvgF1Score   float64 `json:"avg_f1_score"`

	// Min/Max ranges
	MinPrecision float64 `json:"min_precision"`
	MaxPrecision float64 `json:"max_precision"`
	MinRecall    float64 `json:"min_recall"`
	MaxRecall    float64 `json:"max_recall"`

	// Performance metrics
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	TotalTokensUsed int    `json:"total_tokens_used"`

	// Trend indicators
	PrecisionTrend string `json:"precision_trend"` // "up", "down", "stable"
	RecallTrend    string `json:"recall_trend"`
	LatencyTrend   string `json:"latency_trend"`
}
