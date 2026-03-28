package llm

import (
	"sync"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ModelPricing defines the cost per 1000 tokens for a model.
type ModelPricing struct {
	ModelID         string
	Provider        string
	InputCostPer1K  float64 // Cost per 1K input tokens in USD
	OutputCostPer1K float64 // Cost per 1K output tokens in USD
}

// CostCalculator calculates LLM operation costs and tracks usage.
type CostCalculator struct {
	pricing map[string]ModelPricing
	mu      sync.RWMutex
}

// NewCostCalculator creates a new cost calculator with default pricing.
func NewCostCalculator() *CostCalculator {
	calc := &CostCalculator{
		pricing: make(map[string]ModelPricing),
	}
	calc.initDefaultPricing()
	return calc
}

// initDefaultPricing sets up default pricing for common models.
func (c *CostCalculator) initDefaultPricing() {
	defaults := []ModelPricing{
		// OpenAI models
		{ModelID: "gpt-4", Provider: "openai", InputCostPer1K: 0.03, OutputCostPer1K: 0.06},
		{ModelID: "gpt-4-turbo", Provider: "openai", InputCostPer1K: 0.01, OutputCostPer1K: 0.03},
		{ModelID: "gpt-4o", Provider: "openai", InputCostPer1K: 0.005, OutputCostPer1K: 0.015},
		{ModelID: "gpt-4o-mini", Provider: "openai", InputCostPer1K: 0.00015, OutputCostPer1K: 0.0006},
		{ModelID: "gpt-3.5-turbo", Provider: "openai", InputCostPer1K: 0.0005, OutputCostPer1K: 0.0015},

		// Anthropic models
		{ModelID: "claude-3-opus", Provider: "anthropic", InputCostPer1K: 0.015, OutputCostPer1K: 0.075},
		{ModelID: "claude-3-sonnet", Provider: "anthropic", InputCostPer1K: 0.003, OutputCostPer1K: 0.015},
		{ModelID: "claude-3-haiku", Provider: "anthropic", InputCostPer1K: 0.00025, OutputCostPer1K: 0.00125},
		{ModelID: "claude-3.5-sonnet", Provider: "anthropic", InputCostPer1K: 0.003, OutputCostPer1K: 0.015},

		// Ollama models (local, no cost)
		{ModelID: "llama3.2", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "llama3.1", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "llama3", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "mistral", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "mixtral", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "qwen2.5", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "phi3", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},
		{ModelID: "gemma2", Provider: "ollama", InputCostPer1K: 0, OutputCostPer1K: 0},

		// Google models
		{ModelID: "gemini-1.5-pro", Provider: "google", InputCostPer1K: 0.00125, OutputCostPer1K: 0.005},
		{ModelID: "gemini-1.5-flash", Provider: "google", InputCostPer1K: 0.000075, OutputCostPer1K: 0.0003},
		{ModelID: "gemini-2.0-flash-exp", Provider: "google", InputCostPer1K: 0, OutputCostPer1K: 0}, // Free during preview
	}

	for _, p := range defaults {
		c.pricing[p.ModelID] = p
	}
}

// SetPricing adds or updates pricing for a model.
func (c *CostCalculator) SetPricing(pricing ModelPricing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pricing[pricing.ModelID] = pricing
}

// GetPricing retrieves pricing for a model.
func (c *CostCalculator) GetPricing(modelID string) (ModelPricing, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.pricing[modelID]
	return p, ok
}

// CalculateCost computes the cost for a given token usage.
func (c *CostCalculator) CalculateCost(modelID string, inputTokens, outputTokens int) float64 {
	c.mu.RLock()
	pricing, ok := c.pricing[modelID]
	c.mu.RUnlock()

	if !ok {
		// Default to zero cost for unknown models (likely local)
		return 0
	}

	inputCost := (float64(inputTokens) / 1000.0) * pricing.InputCostPer1K
	outputCost := (float64(outputTokens) / 1000.0) * pricing.OutputCostPer1K

	return inputCost + outputCost
}

// CreateCostMetrics creates an LLMCostMetrics record.
func (c *CostCalculator) CreateCostMetrics(
	modelID string,
	modelVersion string,
	provider string,
	operation string,
	promptTokens int,
	completionTokens int,
	latencyMs int64,
) entity.LLMCostMetrics {
	totalTokens := promptTokens + completionTokens
	cost := c.CalculateCost(modelID, promptTokens, completionTokens)

	return entity.LLMCostMetrics{
		ModelID:          modelID,
		ModelVersion:     modelVersion,
		Provider:         provider,
		Operation:        operation,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		EstimatedCost:    cost,
		LatencyMs:        latencyMs,
		Timestamp:        time.Now(),
	}
}

// EstimateTokens provides a rough token estimate for text.
// This is an approximation; actual token counts depend on the model's tokenizer.
func EstimateTokens(text string) int {
	// Rough approximation: ~4 characters per token for English text
	// This varies by model and language
	return (len(text) + 3) / 4
}

// TokenUsage tracks token usage for an operation.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Add combines token usage from multiple operations.
func (u *TokenUsage) Add(other TokenUsage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
}

// CostSummary provides a summary of costs over a period.
type CostSummary struct {
	Period           time.Duration
	TotalCost        float64
	TotalTokens      int
	OperationCounts  map[string]int
	CostByModel      map[string]float64
	CostByOperation  map[string]float64
	TokensByModel    map[string]int
	AvgLatencyMs     float64
}

// NewCostSummary creates an empty cost summary.
func NewCostSummary(period time.Duration) *CostSummary {
	return &CostSummary{
		Period:          period,
		OperationCounts: make(map[string]int),
		CostByModel:     make(map[string]float64),
		CostByOperation: make(map[string]float64),
		TokensByModel:   make(map[string]int),
	}
}

// AddMetrics adds metrics to the summary.
func (s *CostSummary) AddMetrics(m entity.LLMCostMetrics) {
	s.TotalCost += m.EstimatedCost
	s.TotalTokens += m.TotalTokens
	s.OperationCounts[m.Operation]++
	s.CostByModel[m.ModelID] += m.EstimatedCost
	s.CostByOperation[m.Operation] += m.EstimatedCost
	s.TokensByModel[m.ModelID] += m.TotalTokens
}
