package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// BenchmarkRepository provides benchmark persistence in SQLite.
type BenchmarkRepository struct {
	conn *Connection
}

// NewBenchmarkRepository creates a new benchmark repository.
func NewBenchmarkRepository(conn *Connection) *BenchmarkRepository {
	return &BenchmarkRepository{conn: conn}
}

// SaveResult stores a benchmark result.
func (r *BenchmarkRepository) SaveResult(ctx context.Context, result *entity.BenchmarkMetrics) error {
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now()
	}

	var detailsJSON *string
	if len(result.Details) > 0 {
		s := string(result.Details)
		detailsJSON = &s
	}

	_, err := r.conn.Exec(ctx, `
		INSERT INTO benchmark_results (
			id,
			workspace_id,
			test_suite,
			metric_type,
			precision,
			recall,
			f1_score,
			accuracy,
			retrieval_hit_rate,
			grounding_accuracy,
			hallucination_rate,
			tokens_used,
			latency_ms,
			model_version,
			details,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		result.ID,
		result.WorkspaceID,
		result.TestSuite,
		result.MetricType,
		nullableFloat(result.Precision),
		nullableFloat(result.Recall),
		nullableFloat(result.F1Score),
		nullableFloat(result.Accuracy),
		nullableFloat(result.RetrievalHitRate),
		nullableFloat(result.GroundingAccuracy),
		nullableFloat(result.HallucinationRate),
		nullableInt(result.TokensUsed),
		nullableInt64(result.LatencyMs),
		nullableString(result.ModelVersion),
		detailsJSON,
		result.CreatedAt.UnixMilli(),
	)
	return err
}

// GetResultByID retrieves a specific benchmark result.
func (r *BenchmarkRepository) GetResultByID(ctx context.Context, id string) (*entity.BenchmarkMetrics, error) {
	row := r.conn.QueryRow(ctx, `
		SELECT id, workspace_id, test_suite, metric_type,
		       precision, recall, f1_score, accuracy,
		       retrieval_hit_rate, grounding_accuracy, hallucination_rate,
		       tokens_used, latency_ms, model_version, details, created_at
		  FROM benchmark_results
		 WHERE id = ?
	`, id)

	result, err := scanBenchmarkResult(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return result, err
}

// GetResultsByType retrieves benchmark results filtered by type.
func (r *BenchmarkRepository) GetResultsByType(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, limit int) ([]*entity.BenchmarkMetrics, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, test_suite, metric_type,
		       precision, recall, f1_score, accuracy,
		       retrieval_hit_rate, grounding_accuracy, hallucination_rate,
		       tokens_used, latency_ms, model_version, details, created_at
		  FROM benchmark_results
		 WHERE workspace_id = ?
		   AND metric_type = ?
		 ORDER BY created_at DESC
		 LIMIT ?
	`, workspaceID, metricType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBenchmarkResults(rows)
}

// GetResultsByTestSuite retrieves benchmark results for a specific test suite.
func (r *BenchmarkRepository) GetResultsByTestSuite(ctx context.Context, workspaceID entity.WorkspaceID, testSuite string, limit int) ([]*entity.BenchmarkMetrics, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, test_suite, metric_type,
		       precision, recall, f1_score, accuracy,
		       retrieval_hit_rate, grounding_accuracy, hallucination_rate,
		       tokens_used, latency_ms, model_version, details, created_at
		  FROM benchmark_results
		 WHERE workspace_id = ?
		   AND test_suite = ?
		 ORDER BY created_at DESC
		 LIMIT ?
	`, workspaceID, testSuite, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBenchmarkResults(rows)
}

// GetLatestByType retrieves the most recent benchmark of a given type.
func (r *BenchmarkRepository) GetLatestByType(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType) (*entity.BenchmarkMetrics, error) {
	row := r.conn.QueryRow(ctx, `
		SELECT id, workspace_id, test_suite, metric_type,
		       precision, recall, f1_score, accuracy,
		       retrieval_hit_rate, grounding_accuracy, hallucination_rate,
		       tokens_used, latency_ms, model_version, details, created_at
		  FROM benchmark_results
		 WHERE workspace_id = ?
		   AND metric_type = ?
		 ORDER BY created_at DESC
		 LIMIT 1
	`, workspaceID, metricType)

	result, err := scanBenchmarkResult(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return result, err
}

// GetBaseline retrieves the designated baseline benchmark for comparison.
func (r *BenchmarkRepository) GetBaseline(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType) (*entity.BenchmarkMetrics, error) {
	row := r.conn.QueryRow(ctx, `
		SELECT br.id, br.workspace_id, br.test_suite, br.metric_type,
		       br.precision, br.recall, br.f1_score, br.accuracy,
		       br.retrieval_hit_rate, br.grounding_accuracy, br.hallucination_rate,
		       br.tokens_used, br.latency_ms, br.model_version, br.details, br.created_at
		  FROM benchmark_results br
		  JOIN benchmark_baselines bb ON br.id = bb.benchmark_id
		 WHERE bb.workspace_id = ?
		   AND bb.metric_type = ?
	`, workspaceID, metricType)

	result, err := scanBenchmarkResult(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return result, err
}

// SetBaseline designates a benchmark result as the baseline for future comparisons.
func (r *BenchmarkRepository) SetBaseline(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, benchmarkID string) error {
	_, err := r.conn.Exec(ctx, `
		INSERT INTO benchmark_baselines (workspace_id, metric_type, benchmark_id, set_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (workspace_id, metric_type) DO UPDATE SET
			benchmark_id = excluded.benchmark_id,
			set_at = excluded.set_at
	`, workspaceID, metricType, benchmarkID, time.Now().UnixMilli())
	return err
}

// GetResultsInTimeRange retrieves benchmarks within a time range.
func (r *BenchmarkRepository) GetResultsInTimeRange(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, start, end time.Time) ([]*entity.BenchmarkMetrics, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, test_suite, metric_type,
		       precision, recall, f1_score, accuracy,
		       retrieval_hit_rate, grounding_accuracy, hallucination_rate,
		       tokens_used, latency_ms, model_version, details, created_at
		  FROM benchmark_results
		 WHERE workspace_id = ?
		   AND metric_type = ?
		   AND created_at >= ?
		   AND created_at <= ?
		 ORDER BY created_at DESC
	`, workspaceID, metricType, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBenchmarkResults(rows)
}

// DeleteOlderThan removes benchmark results older than the specified time.
func (r *BenchmarkRepository) DeleteOlderThan(ctx context.Context, workspaceID entity.WorkspaceID, before time.Time) (int64, error) {
	result, err := r.conn.Exec(ctx, `
		DELETE FROM benchmark_results
		 WHERE workspace_id = ?
		   AND created_at < ?
		   AND id NOT IN (SELECT benchmark_id FROM benchmark_baselines WHERE workspace_id = ?)
	`, workspaceID, before.UnixMilli(), workspaceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetAggregatedMetrics returns aggregated metrics over a time period.
func (r *BenchmarkRepository) GetAggregatedMetrics(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, period time.Duration) (*repository.AggregatedBenchmarkMetrics, error) {
	since := time.Now().Add(-period).UnixMilli()

	row := r.conn.QueryRow(ctx, `
		SELECT
			COUNT(*) as count,
			AVG(precision) as avg_precision,
			AVG(recall) as avg_recall,
			AVG(f1_score) as avg_f1,
			MIN(precision) as min_precision,
			MAX(precision) as max_precision,
			MIN(recall) as min_recall,
			MAX(recall) as max_recall,
			AVG(latency_ms) as avg_latency,
			SUM(tokens_used) as total_tokens
		  FROM benchmark_results
		 WHERE workspace_id = ?
		   AND metric_type = ?
		   AND created_at >= ?
	`, workspaceID, metricType, since)

	var agg repository.AggregatedBenchmarkMetrics
	agg.MetricType = metricType
	agg.Period = period

	var avgPrecision, avgRecall, avgF1, minPrecision, maxPrecision, minRecall, maxRecall, avgLatency sql.NullFloat64
	var totalTokens sql.NullInt64

	if err := row.Scan(
		&agg.Count,
		&avgPrecision,
		&avgRecall,
		&avgF1,
		&minPrecision,
		&maxPrecision,
		&minRecall,
		&maxRecall,
		&avgLatency,
		&totalTokens,
	); err != nil {
		return nil, err
	}

	agg.AvgPrecision = avgPrecision.Float64
	agg.AvgRecall = avgRecall.Float64
	agg.AvgF1Score = avgF1.Float64
	agg.MinPrecision = minPrecision.Float64
	agg.MaxPrecision = maxPrecision.Float64
	agg.MinRecall = minRecall.Float64
	agg.MaxRecall = maxRecall.Float64
	agg.AvgLatencyMs = avgLatency.Float64
	agg.TotalTokensUsed = int(totalTokens.Int64)

	// Calculate trends (comparing first half vs second half of period)
	agg.PrecisionTrend = r.calculateTrend(ctx, workspaceID, metricType, "precision", since)
	agg.RecallTrend = r.calculateTrend(ctx, workspaceID, metricType, "recall", since)
	agg.LatencyTrend = r.calculateTrend(ctx, workspaceID, metricType, "latency_ms", since)

	return &agg, nil
}

func (r *BenchmarkRepository) calculateTrend(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, column string, since int64) string {
	midpoint := since + (time.Now().UnixMilli()-since)/2

	// This is a simplified trend calculation; a real implementation might use linear regression
	row := r.conn.QueryRow(ctx, `
		SELECT
			(SELECT AVG(`+column+`) FROM benchmark_results WHERE workspace_id = ? AND metric_type = ? AND created_at >= ? AND created_at < ?) as first_half,
			(SELECT AVG(`+column+`) FROM benchmark_results WHERE workspace_id = ? AND metric_type = ? AND created_at >= ?) as second_half
	`, workspaceID, metricType, since, midpoint, workspaceID, metricType, midpoint)

	var firstHalf, secondHalf sql.NullFloat64
	if err := row.Scan(&firstHalf, &secondHalf); err != nil {
		return "stable"
	}

	if !firstHalf.Valid || !secondHalf.Valid {
		return "stable"
	}

	diff := secondHalf.Float64 - firstHalf.Float64
	threshold := firstHalf.Float64 * 0.05 // 5% change threshold

	if diff > threshold {
		return "up"
	} else if diff < -threshold {
		return "down"
	}
	return "stable"
}

// Helper functions

func nullableFloat(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}

func nullableInt(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

func nullableInt64(i int64) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

type benchmarkScanner interface {
	Scan(dest ...interface{}) error
}

func scanBenchmarkResult(scanner benchmarkScanner) (*entity.BenchmarkMetrics, error) {
	var result entity.BenchmarkMetrics
	var precision, recall, f1Score, accuracy sql.NullFloat64
	var retrievalHitRate, groundingAccuracy, hallucinationRate sql.NullFloat64
	var tokensUsed sql.NullInt64
	var latencyMs sql.NullInt64
	var modelVersion sql.NullString
	var details sql.NullString
	var createdAt int64

	if err := scanner.Scan(
		&result.ID,
		&result.WorkspaceID,
		&result.TestSuite,
		&result.MetricType,
		&precision,
		&recall,
		&f1Score,
		&accuracy,
		&retrievalHitRate,
		&groundingAccuracy,
		&hallucinationRate,
		&tokensUsed,
		&latencyMs,
		&modelVersion,
		&details,
		&createdAt,
	); err != nil {
		return nil, err
	}

	result.Precision = precision.Float64
	result.Recall = recall.Float64
	result.F1Score = f1Score.Float64
	result.Accuracy = accuracy.Float64
	result.RetrievalHitRate = retrievalHitRate.Float64
	result.GroundingAccuracy = groundingAccuracy.Float64
	result.HallucinationRate = hallucinationRate.Float64
	result.TokensUsed = int(tokensUsed.Int64)
	result.LatencyMs = latencyMs.Int64
	result.ModelVersion = modelVersion.String
	if details.Valid && details.String != "" {
		result.Details = json.RawMessage(details.String)
	}
	result.CreatedAt = time.UnixMilli(createdAt)

	return &result, nil
}

func scanBenchmarkResults(rows *sql.Rows) ([]*entity.BenchmarkMetrics, error) {
	var results []*entity.BenchmarkMetrics
	for rows.Next() {
		result, err := scanBenchmarkResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}
