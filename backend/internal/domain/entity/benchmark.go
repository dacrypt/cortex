package entity

import (
	"encoding/json"
	"time"
)

// BenchmarkMetricType identifies the type of benchmark measurement.
type BenchmarkMetricType string

const (
	// MetricTypeRAG measures RAG retrieval and answer quality.
	MetricTypeRAG BenchmarkMetricType = "rag"
	// MetricTypeNER measures Named Entity Recognition accuracy.
	MetricTypeNER BenchmarkMetricType = "ner"
	// MetricTypeClassification measures category classification accuracy.
	MetricTypeClassification BenchmarkMetricType = "classification"
	// MetricTypeSummary measures summary generation quality.
	MetricTypeSummary BenchmarkMetricType = "summary"
)

// BenchmarkMetrics represents a benchmark measurement result.
type BenchmarkMetrics struct {
	ID          string              `json:"id"`
	WorkspaceID WorkspaceID         `json:"workspace_id"`
	TestSuite   string              `json:"test_suite"`
	MetricType  BenchmarkMetricType `json:"metric_type"`

	// Quality Metrics
	Precision float64 `json:"precision,omitempty"`
	Recall    float64 `json:"recall,omitempty"`
	F1Score   float64 `json:"f1_score,omitempty"`
	Accuracy  float64 `json:"accuracy,omitempty"`

	// RAG-specific Metrics
	RetrievalHitRate  float64 `json:"retrieval_hit_rate,omitempty"`
	GroundingAccuracy float64 `json:"grounding_accuracy,omitempty"`
	HallucinationRate float64 `json:"hallucination_rate,omitempty"`

	// Cost and Performance Metrics
	TokensUsed   int    `json:"tokens_used,omitempty"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
	ModelVersion string `json:"model_version,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`

	// Additional details as JSON
	Details json.RawMessage `json:"details,omitempty"`
}

// BenchmarkComparison compares two benchmark results.
type BenchmarkComparison struct {
	Baseline BenchmarkMetrics `json:"baseline"`
	Current  BenchmarkMetrics `json:"current"`

	// Deltas (positive = improvement)
	PrecisionDelta         float64 `json:"precision_delta"`
	RecallDelta            float64 `json:"recall_delta"`
	F1ScoreDelta           float64 `json:"f1_score_delta"`
	RetrievalHitRateDelta  float64 `json:"retrieval_hit_rate_delta"`
	GroundingAccuracyDelta float64 `json:"grounding_accuracy_delta"`
	HallucinationRateDelta float64 `json:"hallucination_rate_delta"` // negative = improvement
	LatencyDelta           int64   `json:"latency_delta"`            // negative = improvement
	TokensDelta            int     `json:"tokens_delta"`             // negative = cost reduction

	// Overall assessment
	IsImproved bool   `json:"is_improved"`
	Summary    string `json:"summary"`
}

// RAGTestCase defines a test case for RAG benchmarking.
type RAGTestCase struct {
	ID              string   `json:"id"`
	Query           string   `json:"query"`
	ExpectedDocIDs  []string `json:"expected_doc_ids"`
	ExpectedAnswer  string   `json:"expected_answer,omitempty"`
	RelevanceScores []float64 `json:"relevance_scores,omitempty"` // Expected relevance per doc
}

// RAGTestResult captures the result of a single RAG test case.
type RAGTestResult struct {
	TestCase       RAGTestCase `json:"test_case"`
	RetrievedDocs  []string    `json:"retrieved_docs"`
	GeneratedAnswer string     `json:"generated_answer,omitempty"`
	Precision      float64     `json:"precision"`
	Recall         float64     `json:"recall"`
	NDCG           float64     `json:"ndcg"` // Normalized Discounted Cumulative Gain
	IsGrounded     bool        `json:"is_grounded"`
	LatencyMs      int64       `json:"latency_ms"`
	TokensUsed     int         `json:"tokens_used"`
}

// NERTestCase defines a test case for NER benchmarking.
type NERTestCase struct {
	ID              string       `json:"id"`
	Text            string       `json:"text"`
	ExpectedEntities []NEREntity `json:"expected_entities"`
}

// NEREntity represents an expected named entity.
type NEREntity struct {
	Text     string `json:"text"`
	Type     string `json:"type"` // PERSON, ORG, LOCATION, etc.
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

// NERTestResult captures the result of a single NER test case.
type NERTestResult struct {
	TestCase         NERTestCase `json:"test_case"`
	ExtractedEntities []NEREntity `json:"extracted_entities"`
	TruePositives    int         `json:"true_positives"`
	FalsePositives   int         `json:"false_positives"`
	FalseNegatives   int         `json:"false_negatives"`
	Precision        float64     `json:"precision"`
	Recall           float64     `json:"recall"`
	F1Score          float64     `json:"f1_score"`
	LatencyMs        int64       `json:"latency_ms"`
}

// ClassificationTestCase defines a test case for classification benchmarking.
type ClassificationTestCase struct {
	ID               string  `json:"id"`
	FilePath         string  `json:"file_path"`
	Content          string  `json:"content,omitempty"`
	ExpectedCategory string  `json:"expected_category"`
	ExpectedConfidence float64 `json:"expected_confidence,omitempty"`
}

// ClassificationTestResult captures the result of a single classification test.
type ClassificationTestResult struct {
	TestCase          ClassificationTestCase `json:"test_case"`
	PredictedCategory string                 `json:"predicted_category"`
	Confidence        float64                `json:"confidence"`
	IsCorrect         bool                   `json:"is_correct"`
	LatencyMs         int64                  `json:"latency_ms"`
	TokensUsed        int                    `json:"tokens_used"`
}

// SummaryTestCase defines a test case for summary benchmarking.
type SummaryTestCase struct {
	ID               string `json:"id"`
	FilePath         string `json:"file_path"`
	Content          string `json:"content"`
	ReferenceSummary string `json:"reference_summary"`
}

// SummaryTestResult captures the result of a single summary test.
type SummaryTestResult struct {
	TestCase          SummaryTestCase `json:"test_case"`
	GeneratedSummary  string          `json:"generated_summary"`
	ROUGEScores       ROUGEScores     `json:"rouge_scores"`
	BLEUScore         float64         `json:"bleu_score"`
	LatencyMs         int64           `json:"latency_ms"`
	TokensUsed        int             `json:"tokens_used"`
}

// ROUGEScores contains ROUGE evaluation metrics.
type ROUGEScores struct {
	ROUGE1 float64 `json:"rouge_1"` // Unigram overlap
	ROUGE2 float64 `json:"rouge_2"` // Bigram overlap
	ROUGEL float64 `json:"rouge_l"` // Longest common subsequence
}

// BaselineReport summarizes benchmark results against a baseline.
type BaselineReport struct {
	WorkspaceID   WorkspaceID           `json:"workspace_id"`
	GeneratedAt   time.Time             `json:"generated_at"`
	BaselineDate  time.Time             `json:"baseline_date"`
	CurrentDate   time.Time             `json:"current_date"`
	Comparisons   []BenchmarkComparison `json:"comparisons"`
	OverallStatus string                `json:"overall_status"` // "improved", "degraded", "stable"
	Summary       string                `json:"summary"`
}

// LLMCostMetrics tracks token usage and costs for LLM operations.
type LLMCostMetrics struct {
	ModelID         string    `json:"model_id"`
	ModelVersion    string    `json:"model_version"`
	Provider        string    `json:"provider"`
	Operation       string    `json:"operation"`
	PromptTokens    int       `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens     int       `json:"total_tokens"`
	EstimatedCost   float64   `json:"estimated_cost"` // In USD
	LatencyMs       int64     `json:"latency_ms"`
	Timestamp       time.Time `json:"timestamp"`
}
