// Package benchmark provides benchmarking services for AI/RAG quality metrics.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Service provides benchmark execution and analysis capabilities.
type Service struct {
	benchmarkRepo repository.BenchmarkRepository
	logger        zerolog.Logger
}

// NewService creates a new benchmark service.
func NewService(
	benchmarkRepo repository.BenchmarkRepository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		benchmarkRepo: benchmarkRepo,
		logger:        logger.With().Str("component", "benchmark").Logger(),
	}
}

// RunRAGBenchmark executes a RAG benchmark suite and stores the results.
func (s *Service) RunRAGBenchmark(ctx context.Context, workspaceID entity.WorkspaceID, testCases []entity.RAGTestCase, retriever RAGRetriever) (*entity.BenchmarkMetrics, error) {
	s.logger.Info().Int("test_cases", len(testCases)).Msg("Starting RAG benchmark")

	start := time.Now()
	var results []entity.RAGTestResult
	var totalPrecision, totalRecall, totalNDCG float64
	var groundedCount int
	var totalTokens int
	var totalLatency int64

	for _, tc := range testCases {
		result, err := s.runSingleRAGTest(ctx, tc, retriever)
		if err != nil {
			s.logger.Warn().Err(err).Str("test_id", tc.ID).Msg("RAG test case failed")
			continue
		}
		results = append(results, result)
		totalPrecision += result.Precision
		totalRecall += result.Recall
		totalNDCG += result.NDCG
		if result.IsGrounded {
			groundedCount++
		}
		totalTokens += result.TokensUsed
		totalLatency += result.LatencyMs
	}

	n := float64(len(results))
	if n == 0 {
		return nil, fmt.Errorf("all test cases failed")
	}

	metrics := &entity.BenchmarkMetrics{
		ID:                uuid.New().String(),
		WorkspaceID:       workspaceID,
		TestSuite:         "rag_benchmark",
		MetricType:        entity.MetricTypeRAG,
		Precision:         totalPrecision / n,
		Recall:            totalRecall / n,
		F1Score:           calculateF1(totalPrecision/n, totalRecall/n),
		RetrievalHitRate:  totalNDCG / n,
		GroundingAccuracy: float64(groundedCount) / n,
		HallucinationRate: 1.0 - float64(groundedCount)/n,
		TokensUsed:        totalTokens,
		LatencyMs:         time.Since(start).Milliseconds(),
		CreatedAt:         time.Now(),
	}

	// Store detailed results
	detailsBytes, _ := json.Marshal(results)
	metrics.Details = detailsBytes

	if err := s.benchmarkRepo.SaveResult(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to save benchmark result: %w", err)
	}

	s.logger.Info().
		Float64("precision", metrics.Precision).
		Float64("recall", metrics.Recall).
		Float64("grounding_accuracy", metrics.GroundingAccuracy).
		Msg("RAG benchmark completed")

	return metrics, nil
}

// RunNERBenchmark executes a Named Entity Recognition benchmark suite.
func (s *Service) RunNERBenchmark(ctx context.Context, workspaceID entity.WorkspaceID, testCases []entity.NERTestCase, extractor NERExtractor) (*entity.BenchmarkMetrics, error) {
	s.logger.Info().Int("test_cases", len(testCases)).Msg("Starting NER benchmark")

	start := time.Now()
	var results []entity.NERTestResult
	var totalTP, totalFP, totalFN int

	for _, tc := range testCases {
		result, err := s.runSingleNERTest(ctx, tc, extractor)
		if err != nil {
			s.logger.Warn().Err(err).Str("test_id", tc.ID).Msg("NER test case failed")
			continue
		}
		results = append(results, result)
		totalTP += result.TruePositives
		totalFP += result.FalsePositives
		totalFN += result.FalseNegatives
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all test cases failed")
	}

	// Micro-averaged metrics
	precision := float64(totalTP) / float64(totalTP+totalFP)
	recall := float64(totalTP) / float64(totalTP+totalFN)
	if math.IsNaN(precision) {
		precision = 0
	}
	if math.IsNaN(recall) {
		recall = 0
	}

	metrics := &entity.BenchmarkMetrics{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		TestSuite:   "ner_benchmark",
		MetricType:  entity.MetricTypeNER,
		Precision:   precision,
		Recall:      recall,
		F1Score:     calculateF1(precision, recall),
		LatencyMs:   time.Since(start).Milliseconds(),
		CreatedAt:   time.Now(),
	}

	detailsBytes, _ := json.Marshal(results)
	metrics.Details = detailsBytes

	if err := s.benchmarkRepo.SaveResult(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to save benchmark result: %w", err)
	}

	s.logger.Info().
		Float64("precision", metrics.Precision).
		Float64("recall", metrics.Recall).
		Float64("f1", metrics.F1Score).
		Msg("NER benchmark completed")

	return metrics, nil
}

// RunClassificationBenchmark executes a classification benchmark suite.
func (s *Service) RunClassificationBenchmark(ctx context.Context, workspaceID entity.WorkspaceID, testCases []entity.ClassificationTestCase, classifier Classifier) (*entity.BenchmarkMetrics, error) {
	s.logger.Info().Int("test_cases", len(testCases)).Msg("Starting classification benchmark")

	start := time.Now()
	var results []entity.ClassificationTestResult
	var correctCount int
	var totalTokens int

	for _, tc := range testCases {
		result, err := s.runSingleClassificationTest(ctx, tc, classifier)
		if err != nil {
			s.logger.Warn().Err(err).Str("test_id", tc.ID).Msg("Classification test case failed")
			continue
		}
		results = append(results, result)
		if result.IsCorrect {
			correctCount++
		}
		totalTokens += result.TokensUsed
	}

	n := float64(len(results))
	if n == 0 {
		return nil, fmt.Errorf("all test cases failed")
	}

	metrics := &entity.BenchmarkMetrics{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		TestSuite:   "classification_benchmark",
		MetricType:  entity.MetricTypeClassification,
		Accuracy:    float64(correctCount) / n,
		TokensUsed:  totalTokens,
		LatencyMs:   time.Since(start).Milliseconds(),
		CreatedAt:   time.Now(),
	}

	detailsBytes, _ := json.Marshal(results)
	metrics.Details = detailsBytes

	if err := s.benchmarkRepo.SaveResult(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to save benchmark result: %w", err)
	}

	s.logger.Info().
		Float64("accuracy", metrics.Accuracy).
		Msg("Classification benchmark completed")

	return metrics, nil
}

// GenerateBaselineReport compares current benchmarks against baseline.
func (s *Service) GenerateBaselineReport(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.BaselineReport, error) {
	report := &entity.BaselineReport{
		WorkspaceID: workspaceID,
		GeneratedAt: time.Now(),
		CurrentDate: time.Now(),
	}

	metricTypes := []entity.BenchmarkMetricType{
		entity.MetricTypeRAG,
		entity.MetricTypeNER,
		entity.MetricTypeClassification,
		entity.MetricTypeSummary,
	}

	var improvements, degradations int

	for _, mt := range metricTypes {
		baseline, err := s.benchmarkRepo.GetBaseline(ctx, workspaceID, mt)
		if err != nil || baseline == nil {
			continue
		}

		current, err := s.benchmarkRepo.GetLatestByType(ctx, workspaceID, mt)
		if err != nil || current == nil {
			continue
		}

		comparison := s.compareBenchmarks(baseline, current)
		report.Comparisons = append(report.Comparisons, comparison)

		if comparison.IsImproved {
			improvements++
		} else if comparison.F1ScoreDelta < -0.01 || comparison.PrecisionDelta < -0.01 {
			degradations++
		}

		if report.BaselineDate.IsZero() {
			report.BaselineDate = baseline.CreatedAt
		}
	}

	if improvements > degradations {
		report.OverallStatus = "improved"
	} else if degradations > improvements {
		report.OverallStatus = "degraded"
	} else {
		report.OverallStatus = "stable"
	}

	report.Summary = fmt.Sprintf("%d improvements, %d degradations across %d metric types",
		improvements, degradations, len(report.Comparisons))

	return report, nil
}

// SetBaseline designates a benchmark as the baseline for future comparisons.
func (s *Service) SetBaseline(ctx context.Context, workspaceID entity.WorkspaceID, metricType entity.BenchmarkMetricType, benchmarkID string) error {
	return s.benchmarkRepo.SetBaseline(ctx, workspaceID, metricType, benchmarkID)
}

// GetLatestMetrics retrieves the most recent benchmark for each type.
func (s *Service) GetLatestMetrics(ctx context.Context, workspaceID entity.WorkspaceID) (map[entity.BenchmarkMetricType]*entity.BenchmarkMetrics, error) {
	result := make(map[entity.BenchmarkMetricType]*entity.BenchmarkMetrics)

	metricTypes := []entity.BenchmarkMetricType{
		entity.MetricTypeRAG,
		entity.MetricTypeNER,
		entity.MetricTypeClassification,
		entity.MetricTypeSummary,
	}

	for _, mt := range metricTypes {
		metrics, err := s.benchmarkRepo.GetLatestByType(ctx, workspaceID, mt)
		if err != nil {
			return nil, err
		}
		if metrics != nil {
			result[mt] = metrics
		}
	}

	return result, nil
}

// Helper methods

func (s *Service) runSingleRAGTest(ctx context.Context, tc entity.RAGTestCase, retriever RAGRetriever) (entity.RAGTestResult, error) {
	start := time.Now()

	retrieved, answer, tokens, err := retriever.Retrieve(ctx, tc.Query)
	if err != nil {
		return entity.RAGTestResult{}, err
	}

	// Calculate precision and recall
	expectedSet := make(map[string]bool)
	for _, id := range tc.ExpectedDocIDs {
		expectedSet[id] = true
	}

	retrievedSet := make(map[string]bool)
	for _, id := range retrieved {
		retrievedSet[id] = true
	}

	var tp int
	for id := range retrievedSet {
		if expectedSet[id] {
			tp++
		}
	}

	precision := float64(tp) / float64(len(retrieved))
	recall := float64(tp) / float64(len(tc.ExpectedDocIDs))
	if math.IsNaN(precision) {
		precision = 0
	}
	if math.IsNaN(recall) {
		recall = 0
	}

	// Simplified NDCG calculation
	ndcg := calculateNDCG(retrieved, tc.ExpectedDocIDs)

	// Check grounding (simplified: answer must not be empty and at least one doc retrieved)
	isGrounded := answer != "" && len(retrieved) > 0 && tp > 0

	return entity.RAGTestResult{
		TestCase:        tc,
		RetrievedDocs:   retrieved,
		GeneratedAnswer: answer,
		Precision:       precision,
		Recall:          recall,
		NDCG:            ndcg,
		IsGrounded:      isGrounded,
		LatencyMs:       time.Since(start).Milliseconds(),
		TokensUsed:      tokens,
	}, nil
}

func (s *Service) runSingleNERTest(ctx context.Context, tc entity.NERTestCase, extractor NERExtractor) (entity.NERTestResult, error) {
	start := time.Now()

	extracted, err := extractor.Extract(ctx, tc.Text)
	if err != nil {
		return entity.NERTestResult{}, err
	}

	// Match entities by text and type
	expectedMap := make(map[string]bool)
	for _, e := range tc.ExpectedEntities {
		key := fmt.Sprintf("%s:%s", e.Type, e.Text)
		expectedMap[key] = true
	}

	extractedMap := make(map[string]bool)
	for _, e := range extracted {
		key := fmt.Sprintf("%s:%s", e.Type, e.Text)
		extractedMap[key] = true
	}

	var tp, fp, fn int
	for key := range extractedMap {
		if expectedMap[key] {
			tp++
		} else {
			fp++
		}
	}
	for key := range expectedMap {
		if !extractedMap[key] {
			fn++
		}
	}

	precision := float64(tp) / float64(tp+fp)
	recall := float64(tp) / float64(tp+fn)
	if math.IsNaN(precision) {
		precision = 0
	}
	if math.IsNaN(recall) {
		recall = 0
	}

	return entity.NERTestResult{
		TestCase:          tc,
		ExtractedEntities: extracted,
		TruePositives:     tp,
		FalsePositives:    fp,
		FalseNegatives:    fn,
		Precision:         precision,
		Recall:            recall,
		F1Score:           calculateF1(precision, recall),
		LatencyMs:         time.Since(start).Milliseconds(),
	}, nil
}

func (s *Service) runSingleClassificationTest(ctx context.Context, tc entity.ClassificationTestCase, classifier Classifier) (entity.ClassificationTestResult, error) {
	start := time.Now()

	predicted, confidence, tokens, err := classifier.Classify(ctx, tc.Content, tc.FilePath)
	if err != nil {
		return entity.ClassificationTestResult{}, err
	}

	return entity.ClassificationTestResult{
		TestCase:          tc,
		PredictedCategory: predicted,
		Confidence:        confidence,
		IsCorrect:         predicted == tc.ExpectedCategory,
		LatencyMs:         time.Since(start).Milliseconds(),
		TokensUsed:        tokens,
	}, nil
}

func (s *Service) compareBenchmarks(baseline, current *entity.BenchmarkMetrics) entity.BenchmarkComparison {
	comp := entity.BenchmarkComparison{
		Baseline:               *baseline,
		Current:                *current,
		PrecisionDelta:         current.Precision - baseline.Precision,
		RecallDelta:            current.Recall - baseline.Recall,
		F1ScoreDelta:           current.F1Score - baseline.F1Score,
		RetrievalHitRateDelta:  current.RetrievalHitRate - baseline.RetrievalHitRate,
		GroundingAccuracyDelta: current.GroundingAccuracy - baseline.GroundingAccuracy,
		HallucinationRateDelta: current.HallucinationRate - baseline.HallucinationRate, // negative = good
		LatencyDelta:           current.LatencyMs - baseline.LatencyMs,                 // negative = good
		TokensDelta:            current.TokensUsed - baseline.TokensUsed,               // negative = good
	}

	// Determine if overall improved (weighted by importance)
	qualityScore := comp.F1ScoreDelta*0.4 + comp.PrecisionDelta*0.3 + comp.RecallDelta*0.3
	comp.IsImproved = qualityScore > 0.01 // Threshold for improvement

	comp.Summary = fmt.Sprintf("F1: %+.2f%%, Precision: %+.2f%%, Recall: %+.2f%%",
		comp.F1ScoreDelta*100, comp.PrecisionDelta*100, comp.RecallDelta*100)

	return comp
}

// Utility functions

func calculateF1(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0
	}
	return 2 * (precision * recall) / (precision + recall)
}

func calculateNDCG(retrieved, expected []string) float64 {
	if len(expected) == 0 {
		return 0
	}

	expectedSet := make(map[string]int)
	for i, id := range expected {
		expectedSet[id] = i + 1 // Position in ideal ranking
	}

	var dcg float64
	for i, id := range retrieved {
		if _, ok := expectedSet[id]; ok {
			dcg += 1.0 / math.Log2(float64(i+2)) // +2 because log2(1) = 0
		}
	}

	// Calculate ideal DCG
	var idcg float64
	for i := 0; i < len(expected) && i < len(retrieved); i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// Interfaces for test dependencies

// RAGRetriever retrieves documents and generates answers.
type RAGRetriever interface {
	Retrieve(ctx context.Context, query string) (docIDs []string, answer string, tokensUsed int, err error)
}

// NERExtractor extracts named entities from text.
type NERExtractor interface {
	Extract(ctx context.Context, text string) ([]entity.NEREntity, error)
}

// Classifier classifies content into categories.
type Classifier interface {
	Classify(ctx context.Context, content, filePath string) (category string, confidence float64, tokensUsed int, err error)
}
